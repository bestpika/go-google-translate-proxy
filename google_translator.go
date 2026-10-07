package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	googleTranslateClient  = "wt_lib"
	maxGoogleResponseBytes = 1 << 20
	googleRequestTimeout   = 30 * time.Second
)

var (
	errMissingAPIKey          = errors.New("missing GOOGLE_TRANSLATE_API_KEY")
	errGoogleResponseTooLarge = errors.New("google response exceeds size limit")
)

type googleTranslator struct {
	client *http.Client
	url    string
	apiKey string
}

// 僅公開固定分類與狀態碼，保留原因供 errors.Is 判斷，不輸出網址或上游內容。
type upstreamError struct {
	kind   string
	status int
	cause  error
}

func (e *upstreamError) Error() string {
	return fmt.Sprintf("google translate failed: kind=%s status=%d", e.kind, e.status)
}

func (e *upstreamError) Unwrap() error { return e.cause }

func newGoogleTranslator(url, apiKey string, client *http.Client) *googleTranslator {
	if client == nil {
		client = &http.Client{Timeout: googleRequestTimeout}
	}
	return &googleTranslator{client: client, url: url, apiKey: apiKey}
}

func (t *googleTranslator) Translate(ctx context.Context, sourceLang, targetLang string, texts []string) ([]string, error) {
	if strings.TrimSpace(t.apiKey) == "" {
		return nil, errMissingAPIKey
	}
	body, err := json.Marshal([]any{[]any{texts, sourceLang, targetLang}, googleTranslateClient})
	if err != nil {
		return nil, &upstreamError{kind: "request", cause: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(body))
	if err != nil {
		return nil, &upstreamError{kind: "request", cause: err}
	}
	req.Header.Set("Content-Type", "application/json+protobuf")
	req.Header.Set("X-Goog-API-Key", t.apiKey)
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, &upstreamError{kind: "transport", cause: err}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxGoogleResponseBytes+1))
	if err != nil {
		return nil, &upstreamError{kind: "response_read", status: resp.StatusCode, cause: err}
	}
	if len(respBody) > maxGoogleResponseBytes {
		return nil, &upstreamError{kind: "response_too_large", status: resp.StatusCode, cause: errGoogleResponseTooLarge}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &upstreamError{kind: "http_status", status: resp.StatusCode}
	}
	translations, err := parseGoogleResponse(respBody)
	if err != nil {
		return nil, &upstreamError{kind: "response_format", status: resp.StatusCode, cause: err}
	}
	if len(translations) != len(texts) {
		return nil, &upstreamError{kind: "response_count", status: resp.StatusCode}
	}
	return translations, nil
}

func parseGoogleResponse(body []byte) ([]string, error) {
	var raw []any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode google response: %w", err)
	}
	if len(raw) == 0 {
		return nil, errors.New("google response is empty")
	}
	items, ok := raw[0].([]any)
	if !ok {
		return nil, errors.New("google response has unexpected format")
	}
	translations := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, errors.New("google response contains non-string translation")
		}
		translations = append(translations, text)
	}
	return translations, nil
}
