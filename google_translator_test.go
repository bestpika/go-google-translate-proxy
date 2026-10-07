package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type trackingBody struct {
	io.Reader
	closed bool
	read   int
}

func (b *trackingBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err
}

func (b *trackingBody) Close() error { b.closed = true; return nil }

func TestGoogleTranslatorTranslate(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.Header.Get("Content-Type") != "application/json+protobuf" || req.Header.Get("X-Goog-API-Key") != "test-key" {
			t.Fatal("unexpected request method or headers")
		}
		var body []any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		want := []any{[]any{[]any{"Hello", "World"}, "en", "zh-TW"}, googleTranslateClient}
		if !reflect.DeepEqual(body, want) {
			t.Fatalf("body=%#v", body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[["你好","世界"]]`))}, nil
	})}
	got, err := newGoogleTranslator("https://example.test/translate", "test-key", client).Translate(context.Background(), "en", "zh-TW", []string{"Hello", "World"})
	if err != nil || !reflect.DeepEqual(got, []string{"你好", "世界"}) {
		t.Fatalf("translations=%v error=%v", got, err)
	}
	if got := newGoogleTranslator(defaultGoogleURL, "key", nil).client.Timeout; got != googleRequestTimeout {
		t.Fatalf("timeout=%v", got)
	}
}

func TestGoogleTranslatorResponses(t *testing.T) {
	base := `[["你好"]]`
	for _, tt := range []struct {
		name, body, kind string
		status           int
	}{
		{"success", base, "", 200},
		{"exact limit", base + strings.Repeat(" ", maxGoogleResponseBytes-len(base)), "", 200},
		{"one byte over", base + strings.Repeat(" ", maxGoogleResponseBytes-len(base)+1), "response_too_large", 200},
		{"large response", strings.Repeat("x", maxGoogleResponseBytes*2), "response_too_large", 200},
		{"forbidden", "private-key-and-translation", "http_status", 403},
		{"server error", "private-key-and-translation", "http_status", 500},
		{"object", `{}`, "response_format", 200},
		{"malformed", `[`, "response_format", 200},
		{"empty", `[]`, "response_format", 200},
		{"null", `null`, "response_format", 200},
		{"wrong items", `[{}]`, "response_format", 200},
		{"number item", `[[1]]`, "response_format", 200},
		{"null item", `[[null]]`, "response_format", 200},
		{"count mismatch", `[["a","b"]]`, "response_count", 200},
		{"no translations", `[[]]`, "response_count", 200},
		{"trailing json", base + `{}`, "response_format", 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackingBody{Reader: strings.NewReader(tt.body)}
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tt.status, Body: body}, nil
			})}
			_, err := newGoogleTranslator("https://example.test/translate", "test-key", client).Translate(context.Background(), "en", "zh-TW", []string{"Hello"})
			var upstream *upstreamError
			if tt.kind == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.As(err, &upstream) || upstream.kind != tt.kind || upstream.status != tt.status {
				t.Fatalf("error=%v, want kind=%s status=%d", err, tt.kind, tt.status)
			}
			if !body.closed || body.read > maxGoogleResponseBytes+1 {
				t.Fatalf("closed=%v bytes read=%d", body.closed, body.read)
			}
			if tt.kind == "response_too_large" && !errors.Is(err, errGoogleResponseTooLarge) {
				t.Fatal("size error was not preserved")
			}
			if err != nil && strings.Contains(err.Error(), "private-key-and-translation") {
				t.Fatal("error leaked upstream contents")
			}
		})
	}
}

func TestGoogleTranslatorFailures(t *testing.T) {
	t.Run("missing key", func(t *testing.T) {
		_, err := newGoogleTranslator(defaultGoogleURL, " ", nil).Translate(context.Background(), "en", "zh-TW", []string{"Hello"})
		if !errors.Is(err, errMissingAPIKey) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("invalid URL", func(t *testing.T) {
		_, err := newGoogleTranslator(":invalid-private-key", "key", nil).Translate(context.Background(), "en", "zh-TW", []string{"Hello"})
		var upstream *upstreamError
		if !errors.As(err, &upstream) || upstream.kind != "request" || strings.Contains(err.Error(), "private-key") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("transport", func(t *testing.T) {
		cause := errors.New("private-key-and-translation")
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, cause })}
		_, err := newGoogleTranslator("https://example.test/private-key", "key", client).Translate(context.Background(), "en", "zh-TW", []string{"Hello"})
		if !errors.Is(err, cause) || strings.Contains(err.Error(), "private-key") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("read", func(t *testing.T) {
		body := &trackingBody{Reader: errorReader{}}
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: body}, nil
		})}
		_, err := newGoogleTranslator("https://example.test", "key", client).Translate(context.Background(), "en", "zh-TW", []string{"Hello"})
		if !errors.Is(err, io.ErrUnexpectedEOF) || !body.closed {
			t.Fatalf("error=%v closed=%v", err, body.closed)
		}
	})
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "canceled", true: "client timeout"}[timeout], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if !timeout {
					cancel()
				}
				<-req.Context().Done()
				return nil, req.Context().Err()
			})}
			want := context.Canceled
			if timeout {
				client.Timeout = 20 * time.Millisecond
				want = context.DeadlineExceeded
			}
			_, err := newGoogleTranslator("https://example.test", "key", client).Translate(ctx, "en", "zh-TW", []string{"Hello"})
			if !errors.Is(err, want) {
				t.Fatalf("error=%v want=%v", err, want)
			}
		})
	}
}

func TestParseGoogleResponse(t *testing.T) {
	got, err := parseGoogleResponse([]byte(`[["你好",""],{"metadata":true}]`))
	if err != nil || !reflect.DeepEqual(got, []string{"你好", ""}) {
		t.Fatalf("translations=%v error=%v", got, err)
	}
}
