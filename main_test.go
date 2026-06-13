package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type stubTranslator struct {
	texts      []string
	err        error
	lastSource string
	lastTarget string
	lastTexts  []string
}

func (s *stubTranslator) Translate(_ context.Context, sourceLang string, targetLang string, texts []string) ([]string, error) {
	s.lastSource = sourceLang
	s.lastTarget = targetLang
	s.lastTexts = append([]string(nil), texts...)
	if s.err != nil {
		return nil, s.err
	}
	return s.texts, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestHandleHealthz(t *testing.T) {
	app := newApp(&stubTranslator{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", got["status"])
	}
}

func TestHandleTranslate(t *testing.T) {
	stub := &stubTranslator{texts: []string{"你好", "世界"}}
	app := newApp(stub)
	req := httptest.NewRequest(http.MethodPost, "/translate", bytes.NewBufferString(`{
		"target_lang":"zh-TW",
		"text_list":["Hello","World"]
	}`))
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if stub.lastSource != "auto" {
		t.Fatalf("source = %q, want auto", stub.lastSource)
	}
	if stub.lastTarget != "zh-TW" {
		t.Fatalf("target = %q, want zh-TW", stub.lastTarget)
	}
	if !reflect.DeepEqual(stub.lastTexts, []string{"Hello", "World"}) {
		t.Fatalf("texts = %#v", stub.lastTexts)
	}

	var got translateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := translateResponse{Translations: []translation{
		{DetectedSourceLang: "auto", Text: "你好"},
		{DetectedSourceLang: "auto", Text: "世界"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response = %#v, want %#v", got, want)
	}
}

func TestHandleTranslateValidation(t *testing.T) {
	app := newApp(&stubTranslator{})
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{`},
		{name: "missing target", body: `{"text_list":["Hello"]}`},
		{name: "empty text list", body: `{"target_lang":"zh-TW","text_list":[]}`},
		{name: "empty text", body: `{"target_lang":"zh-TW","text_list":[""]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/translate", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()

			app.routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandleTranslateUpstreamErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "missing api key", err: errMissingAPIKey, want: http.StatusInternalServerError},
		{name: "upstream failed", err: errors.New("upstream failed"), want: http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newApp(&stubTranslator{err: tt.err})
			req := httptest.NewRequest(http.MethodPost, "/translate", bytes.NewBufferString(`{"target_lang":"zh-TW","text_list":["Hello"]}`))
			rec := httptest.NewRecorder()

			app.routes().ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestGoogleTranslatorTranslate(t *testing.T) {
	var gotContentType string
	var gotAPIKey string
	var gotBody []any
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotContentType = req.Header.Get("Content-Type")
		gotAPIKey = req.Header.Get("X-Goog-API-Key")

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewBufferString(`[["你好","世界"]]`)),
			Header:     make(http.Header),
		}, nil
	})}
	translator := newGoogleTranslator("https://example.test/translate", "test-key", client)

	got, err := translator.Translate(context.Background(), "en", "zh-TW", []string{"Hello", "World"})
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}

	if !reflect.DeepEqual(got, []string{"你好", "世界"}) {
		t.Fatalf("translations = %#v", got)
	}
	if gotContentType != "application/json+protobuf" {
		t.Fatalf("content-type = %q", gotContentType)
	}
	if gotAPIKey != "test-key" {
		t.Fatalf("api key = %q", gotAPIKey)
	}

	wantBody := []any{[]any{[]any{"Hello", "World"}, "en", "zh-TW"}, googleTranslateClient}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Fatalf("body = %#v, want %#v", gotBody, wantBody)
	}
}

func TestGoogleTranslatorErrors(t *testing.T) {
	t.Run("missing api key", func(t *testing.T) {
		translator := newGoogleTranslator("https://example.test/translate", "", nil)
		_, err := translator.Translate(context.Background(), "en", "zh-TW", []string{"Hello"})
		if !errors.Is(err, errMissingAPIKey) {
			t.Fatalf("error = %v, want errMissingAPIKey", err)
		}
	})

	t.Run("bad status", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Status:     "403 Forbidden",
				Body:       io.NopCloser(bytes.NewBufferString(`forbidden`)),
				Header:     make(http.Header),
			}, nil
		})}
		translator := newGoogleTranslator("https://example.test/translate", "test-key", client)
		_, err := translator.Translate(context.Background(), "en", "zh-TW", []string{"Hello"})
		if err == nil {
			t.Fatal("error = nil, want error")
		}
	})

	t.Run("unexpected response", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewBufferString(`{"unexpected":true}`)),
				Header:     make(http.Header),
			}, nil
		})}
		translator := newGoogleTranslator("https://example.test/translate", "test-key", client)
		_, err := translator.Translate(context.Background(), "en", "zh-TW", []string{"Hello"})
		if err == nil {
			t.Fatal("error = nil, want error")
		}
	})
}

func TestLoadDotEnv(t *testing.T) {
	t.Setenv("DOTENV_EXISTING", "from-env")
	dir := t.TempDir()
	path := dir + "/.env"
	content := []byte("DOTENV_VALUE=from-file\nDOTENV_EXISTING=from-file\n# comment\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv returned error: %v", err)
	}

	if got := os.Getenv("DOTENV_VALUE"); got != "from-file" {
		t.Fatalf("DOTENV_VALUE = %q", got)
	}
	if got := os.Getenv("DOTENV_EXISTING"); got != "from-env" {
		t.Fatalf("DOTENV_EXISTING = %q", got)
	}
}

func TestEnsureDotEnv(t *testing.T) {
	t.Run("copies example file", func(t *testing.T) {
		dir := t.TempDir()
		envPath := filepath.Join(dir, ".env")
		examplePath := filepath.Join(dir, ".env.example")
		want := []byte("GOOGLE_TRANSLATE_API_KEY=test-key\nPORT=8080\n")

		if err := os.WriteFile(examplePath, want, 0o600); err != nil {
			t.Fatalf("write .env.example: %v", err)
		}
		if err := ensureDotEnv(envPath, examplePath); err != nil {
			t.Fatalf("ensureDotEnv returned error: %v", err)
		}

		got, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("read .env: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf(".env = %q, want %q", got, want)
		}
	})

	t.Run("keeps existing env file", func(t *testing.T) {
		dir := t.TempDir()
		envPath := filepath.Join(dir, ".env")
		examplePath := filepath.Join(dir, ".env.example")
		want := []byte("GOOGLE_TRANSLATE_API_KEY=existing\n")

		if err := os.WriteFile(envPath, want, 0o600); err != nil {
			t.Fatalf("write .env: %v", err)
		}
		if err := os.WriteFile(examplePath, []byte("GOOGLE_TRANSLATE_API_KEY=example\n"), 0o600); err != nil {
			t.Fatalf("write .env.example: %v", err)
		}
		if err := ensureDotEnv(envPath, examplePath); err != nil {
			t.Fatalf("ensureDotEnv returned error: %v", err)
		}

		got, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("read .env: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf(".env = %q, want %q", got, want)
		}
	})

	t.Run("uses embedded example when file is missing", func(t *testing.T) {
		dir := t.TempDir()
		envPath := filepath.Join(dir, ".env")
		examplePath := filepath.Join(dir, ".env.example")

		if err := ensureDotEnv(envPath, examplePath); err != nil {
			t.Fatalf("ensureDotEnv returned error: %v", err)
		}

		got, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("read .env: %v", err)
		}
		if !bytes.Contains(got, []byte("GOOGLE_TRANSLATE_API_KEY=")) {
			t.Fatalf("embedded .env does not contain GOOGLE_TRANSLATE_API_KEY: %q", got)
		}
	})
}
