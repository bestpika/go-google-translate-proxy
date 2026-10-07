package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type stubTranslator struct {
	texts      []string
	err        error
	calls      int
	lastSource string
	lastTarget string
	lastTexts  []string
}

func (s *stubTranslator) Translate(_ context.Context, sourceLang, targetLang string, texts []string) ([]string, error) {
	s.calls++
	s.lastSource, s.lastTarget = sourceLang, targetLang
	s.lastTexts = append([]string(nil), texts...)
	return s.texts, s.err
}

type translatorFunc func(context.Context, string, string, []string) ([]string, error)

func (f translatorFunc) Translate(ctx context.Context, source, target string, texts []string) ([]string, error) {
	return f(ctx, source, target, texts)
}

type failedResponseWriter struct{ header http.Header }

func (w *failedResponseWriter) Header() http.Header       { return w.header }
func (w *failedResponseWriter) WriteHeader(int)           {}
func (w *failedResponseWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWriteJSONError(t *testing.T) {
	writer := &failedResponseWriter{header: make(http.Header)}
	writeJSON(writer, 200, map[string]string{"status": "ok"})
	if writer.header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatal("missing JSON content type")
	}
}

func TestRoutes(t *testing.T) {
	for _, tt := range []struct {
		method, path, allow string
		status              int
	}{
		{http.MethodGet, "/healthz", "", 200},
		{http.MethodPost, "/healthz", "GET", 405},
		{http.MethodHead, "/healthz", "GET", 405},
		{http.MethodGet, "/translate", "POST", 405},
		{http.MethodOptions, "/translate", "POST", 405},
		{http.MethodGet, "/missing", "", 404},
		{http.MethodGet, "/healthz/", "", 404},
	} {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			stub := &stubTranslator{}
			rec := httptest.NewRecorder()
			newApp(stub).routes().ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.status || rec.Header().Get("Allow") != tt.allow || stub.calls != 0 {
				t.Fatalf("status=%d allow=%q calls=%d", rec.Code, rec.Header().Get("Allow"), stub.calls)
			}
			if tt.status == 200 && strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
				t.Fatalf("body=%s", rec.Body.String())
			}
		})
	}
}

func TestHandleTranslate(t *testing.T) {
	for _, source := range []string{"", "auto", " en "} {
		t.Run(source, func(t *testing.T) {
			stub := &stubTranslator{texts: []string{"你好", "世界"}}
			body, _ := json.Marshal(translateRequest{SourceLang: source, TargetLang: " zh-TW ", TextList: []string{"Hello", " "}})
			rec := httptest.NewRecorder()
			newApp(stub).routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/translate", bytes.NewReader(body)))
			wantSource := strings.TrimSpace(source)
			if wantSource == "" {
				wantSource = "auto"
			}
			if rec.Code != 200 || stub.lastSource != wantSource || stub.lastTarget != "zh-TW" || !reflect.DeepEqual(stub.lastTexts, []string{"Hello", " "}) {
				t.Fatalf("status=%d source=%q target=%q texts=%v", rec.Code, stub.lastSource, stub.lastTarget, stub.lastTexts)
			}
			var got translateResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			want := translateResponse{Translations: []translation{{wantSource, "你好"}, {wantSource, "世界"}}}
			if !reflect.DeepEqual(got, want) || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatalf("response=%+v", got)
			}
		})
	}
}

func TestHandleTranslateValidation(t *testing.T) {
	for name, body := range map[string]string{
		"invalid": "{", "empty": "", "null": "null", "array": "[]",
		"missing target":   `{"text_list":["Hello"]}`,
		"blank target":     `{"target_lang":"  ","text_list":["Hello"]}`,
		"empty list":       `{"target_lang":"zh-TW","text_list":[]}`,
		"null list":        `{"target_lang":"zh-TW","text_list":null}`,
		"empty text":       `{"target_lang":"zh-TW","text_list":[""]}`,
		"unknown field":    `{"target_lang":"zh-TW","text_list":["Hello"],"extra":true}`,
		"wrong type":       `{"target_lang":"zh-TW","text_list":[1]}`,
		"trailing object":  `{"target_lang":"zh-TW","text_list":["Hello"]}{}`,
		"trailing null":    `{"target_lang":"zh-TW","text_list":["Hello"]}null`,
		"trailing garbage": `{"target_lang":"zh-TW","text_list":["Hello"]}garbage`,
	} {
		t.Run(name, func(t *testing.T) {
			stub := &stubTranslator{}
			rec := httptest.NewRecorder()
			newApp(stub).routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/translate", strings.NewReader(body)))
			if rec.Code != 400 || stub.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, stub.calls, rec.Body.String())
			}
		})
	}
}

func TestHandleTranslateSizeLimit(t *testing.T) {
	base := `{"target_lang":"zh-TW","text_list":["Hello"]}`
	for _, tt := range []struct {
		name, body string
		status     int
	}{
		{"exact limit", base + strings.Repeat(" ", maxRequestBodyBytes-len(base)), 200},
		{"one byte over", base + strings.Repeat(" ", maxRequestBodyBytes-len(base)+1), 413},
		{"large text", `{"target_lang":"zh-TW","text_list":["` + strings.Repeat("a", maxRequestBodyBytes) + `"]}`, 413},
		{"trailing object over limit", base + `{}` + strings.Repeat(" ", maxRequestBodyBytes), 413},
		{"whitespace suffix", base + " \n\t", 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubTranslator{texts: []string{"你好"}}
			rec := httptest.NewRecorder()
			newApp(stub).routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/translate", strings.NewReader(tt.body)))
			if rec.Code != tt.status || (tt.status == 413 && stub.calls != 0) {
				t.Fatalf("status=%d calls=%d", rec.Code, stub.calls)
			}
		})
	}
}

func TestHandleTranslateReadError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/translate", nil)
	req.Body = io.NopCloser(errorReader{})
	rec := httptest.NewRecorder()
	newApp(&stubTranslator{}).routes().ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleTranslateUpstreamErrors(t *testing.T) {
	secret := "private-key-and-translation"
	for _, tt := range []struct {
		name string
		err  error
		want int
		kind string
	}{
		{"configuration", errMissingAPIKey, 500, "configuration"},
		{"unknown", errors.New(secret), 502, "unknown"},
		{"status", &upstreamError{kind: "http_status", status: 403, cause: errors.New(secret)}, 502, "http_status"},
		{"canceled", context.Canceled, 502, "canceled"},
		{"timeout", context.DeadlineExceeded, 502, "timeout"},
		{"count", nil, 502, "response_count"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(&stubTranslator{err: tt.err})
			var logs bytes.Buffer
			a.logger = log.New(&logs, "", 0)
			rec := httptest.NewRecorder()
			a.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/translate", strings.NewReader(`{"target_lang":"zh-TW","text_list":["Hello"]}`)))
			if rec.Code != tt.want || !strings.Contains(logs.String(), "kind="+tt.kind) || strings.Contains(logs.String()+rec.Body.String(), secret) {
				t.Fatalf("status=%d logs=%s body=%s", rec.Code, &logs, rec.Body.String())
			}
		})
	}
}

func TestHandleTranslateContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := newApp(translatorFunc(func(got context.Context, _, _ string, _ []string) ([]string, error) {
		if got != ctx {
			t.Error("request context was not propagated")
		}
		return nil, got.Err()
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/translate", strings.NewReader(`{"target_lang":"zh-TW","text_list":["Hello"]}`)).WithContext(ctx)
	a.routes().ServeHTTP(rec, req)
	if rec.Code != 502 {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTranslateIntegration(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-Goog-API-Key") != "test-key" {
			t.Error("unexpected upstream request")
		}
		io.WriteString(w, `[["你好","世界"],"metadata"]`)
	}))
	defer upstream.Close()
	proxy := httptest.NewServer(newServer(config{Port: "8080", GoogleURL: upstream.URL, APIKey: "test-key"}).Handler)
	defer proxy.Close()
	resp, err := proxy.Client().Post(proxy.URL+"/translate", "application/json", strings.NewReader(`{"target_lang":"zh-TW","text_list":["Hello","World"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got translateResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || len(got.Translations) != 2 || got.Translations[0].Text != "你好" || got.Translations[1].Text != "世界" || got.Translations[0].DetectedSourceLang != "auto" {
		t.Fatalf("status=%d response=%+v", resp.StatusCode, got)
	}
}
