package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
)

const maxRequestBodyBytes = 1 << 20

type translator interface {
	Translate(ctx context.Context, sourceLang, targetLang string, texts []string) ([]string, error)
}

type app struct {
	translator translator
	logger     *log.Logger
}

type translateRequest struct {
	SourceLang string   `json:"source_lang"`
	TargetLang string   `json:"target_lang"`
	TextList   []string `json:"text_list"`
}

type translateResponse struct {
	Translations []translation `json:"translations"`
}

type translation struct {
	DetectedSourceLang string `json:"detected_source_lang"`
	Text               string `json:"text"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func newApp(translator translator) *app {
	return &app{translator: translator, logger: log.Default()}
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.handleHealthz)
	mux.HandleFunc("/translate", a.handleTranslate)
	return mux
}

func (a *app) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *app) handleTranslate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, errorResponse{Error: "request body too large"})
		} else {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		}
		return
	}
	var req translateRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	req.SourceLang = strings.TrimSpace(req.SourceLang)
	if req.SourceLang == "" {
		req.SourceLang = "auto"
	}
	req.TargetLang = strings.TrimSpace(req.TargetLang)
	if req.TargetLang == "" || len(req.TextList) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "target_lang and text_list are required"})
		return
	}
	for _, text := range req.TextList {
		if text == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "text_list cannot contain empty text"})
			return
		}
	}
	translatedTexts, err := a.translator.Translate(r.Context(), req.SourceLang, req.TargetLang, req.TextList)
	if errors.Is(err, errMissingAPIKey) {
		a.logger.Printf("translation failed: kind=configuration status=0")
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "service is not configured"})
		return
	}
	if err != nil || len(translatedTexts) != len(req.TextList) {
		a.logUpstreamError(err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "translation upstream failed"})
		return
	}
	translations := make([]translation, 0, len(translatedTexts))
	for _, text := range translatedTexts {
		translations = append(translations, translation{DetectedSourceLang: req.SourceLang, Text: text})
	}
	writeJSON(w, http.StatusOK, translateResponse{Translations: translations})
}

func (a *app) logUpstreamError(err error) {
	kind, status := "unknown", 0
	var upstream *upstreamError
	switch {
	case err == nil:
		kind = "response_count"
	case errors.Is(err, context.Canceled):
		kind = "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		kind = "timeout"
	case errors.As(err, &upstream):
		kind, status = upstream.kind, upstream.status
	}
	a.logger.Printf("translation upstream failed: kind=%s status=%d", kind, status)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Print("write json response failed")
	}
}
