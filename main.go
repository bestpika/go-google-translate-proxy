package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kardianos/service"
)

const (
	serviceName             = "go-google-translate-proxy"
	serviceDisplayName      = "Google Translate Proxy"
	serviceDescription      = "Google Translate proxy for Immersive Translate custom API."
	serviceRunCommand       = "service-run"
	serviceWorkDirFlag      = "--workdir"
	defaultPort             = "8080"
	defaultGoogleURL        = "https://translate-pa.googleapis.com/v1/translateHtml"
	defaultGoogleAPIKey     = "AIzaSyATBXajvzQLTDHEQbcpq0Ihe0vWDHmO520"
	googleTranslateClient   = "wt_lib"
	maxRequestBodyBytes     = 1 << 20
	maxGoogleResponseBytes  = 1 << 20
	serverReadHeaderTimeout = 10 * time.Second
	serverReadTimeout       = 15 * time.Second
	serverWriteTimeout      = 45 * time.Second
	serverIdleTimeout       = 60 * time.Second
	serverShutdownTimeout   = 10 * time.Second
)

//go:embed .env.example
var embeddedEnvExample string

type config struct {
	Port      string
	GoogleURL string
	APIKey    string
}

type app struct {
	translator translator
}

type serviceProgram struct {
	server *http.Server
}

type translator interface {
	Translate(ctx context.Context, sourceLang string, targetLang string, texts []string) ([]string, error)
}

type googleTranslator struct {
	client *http.Client
	url    string
	apiKey string
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

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == serviceRunCommand {
		if err := applyServiceRunArgs(args[1:]); err != nil {
			log.Fatalf("service args: %v", err)
		}
	}

	program := &serviceProgram{}
	svc, err := service.New(program, newServiceConfig())
	if err != nil {
		log.Fatalf("create service: %v", err)
	}

	if len(args) > 0 {
		runCommand(svc, args[0])
		return
	}

	if !service.Interactive() {
		if err := svc.Run(); err != nil {
			log.Fatalf("run service: %v", err)
		}
		return
	}

	if err := runServer(); err != nil {
		log.Fatalf("listen and serve: %v", err)
	}
}

func newServiceConfig() *service.Config {
	workingDirectory, err := os.Getwd()
	if err != nil {
		log.Fatalf("get working directory: %v", err)
	}

	return &service.Config{
		Name:             serviceName,
		DisplayName:      serviceDisplayName,
		Description:      serviceDescription,
		WorkingDirectory: workingDirectory,
		Arguments:        []string{serviceRunCommand, serviceWorkDirFlag, workingDirectory},
		Option: service.KeyValue{
			"Restart":                "on-failure",
			"OnFailure":              "restart",
			"OnFailureDelayDuration": "5s",
		},
	}
}

func runCommand(svc service.Service, command string) {
	switch strings.ToLower(command) {
	case serviceRunCommand:
		if err := svc.Run(); err != nil {
			log.Fatalf("run service: %v", err)
		}
	case "run":
		if err := runServer(); err != nil {
			log.Fatalf("listen and serve: %v", err)
		}
	case "install", "uninstall", "start", "stop", "restart":
		if err := service.Control(svc, strings.ToLower(command)); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s %s ok\n", serviceName, strings.ToLower(command))
	case "status":
		status, err := svc.Status()
		if err != nil {
			log.Fatalf("service status: %v", err)
		}
		fmt.Println(serviceStatusText(status))
	case "help", "-h", "--help":
		printUsage()
	default:
		printUsage()
		os.Exit(2)
	}
}

func applyServiceRunArgs(args []string) error {
	for i := 0; i < len(args); i++ {
		if args[i] != serviceWorkDirFlag {
			return fmt.Errorf("unknown argument %q", args[i])
		}
		i++
		if i >= len(args) || strings.TrimSpace(args[i]) == "" {
			return fmt.Errorf("%s requires a path", serviceWorkDirFlag)
		}
		if err := os.Chdir(args[i]); err != nil {
			return fmt.Errorf("set working directory: %w", err)
		}
	}
	return nil
}

func printUsage() {
	fmt.Printf("Usage: %s [run|install|uninstall|start|stop|restart|status]\n", serviceName)
}

func serviceStatusText(status service.Status) string {
	switch status {
	case service.StatusRunning:
		return "running"
	case service.StatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

func (p *serviceProgram) Start(service.Service) error {
	server, err := newServer()
	if err != nil {
		return err
	}
	p.server = server

	go func() {
		if err := serve(server); err != nil {
			log.Fatalf("listen and serve: %v", err)
		}
	}()

	return nil
}

func (p *serviceProgram) Stop(service.Service) error {
	if p.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	return p.server.Shutdown(ctx)
}

func runServer() error {
	server, err := newServer()
	if err != nil {
		return err
	}
	return serve(server)
}

func newServer() (*http.Server, error) {
	if err := ensureDotEnv(".env", ".env.example"); err != nil {
		return nil, fmt.Errorf("ensure .env: %w", err)
	}
	if err := loadDotEnv(".env"); err != nil {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	cfg := loadConfig()
	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           newApp(newGoogleTranslator(cfg.GoogleURL, cfg.APIKey, nil)).routes(),
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}, nil
}

func serve(server *http.Server) error {
	log.Printf("listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func loadConfig() config {
	return config{
		Port:      envOrDefault("PORT", defaultPort),
		GoogleURL: envOrDefault("GOOGLE_TRANSLATE_URL", defaultGoogleURL),
		APIKey:    envOrDefault("GOOGLE_TRANSLATE_API_KEY", defaultGoogleAPIKey),
	}
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func ensureDotEnv(path string, examplePath string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	content, err := os.ReadFile(examplePath)
	if errors.Is(err, os.ErrNotExist) {
		content = []byte(embeddedEnvExample)
	} else if err != nil {
		return err
	}

	if len(content) == 0 {
		return errors.New(".env.example is empty")
	}

	return os.WriteFile(path, content, 0o600)
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func newGoogleTranslator(url string, apiKey string, client *http.Client) *googleTranslator {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	return &googleTranslator{
		client: client,
		url:    url,
		apiKey: apiKey,
	}
}

func (t *googleTranslator) Translate(ctx context.Context, sourceLang string, targetLang string, texts []string) ([]string, error) {
	if strings.TrimSpace(t.apiKey) == "" {
		return nil, errMissingAPIKey
	}

	body, err := json.Marshal([]any{
		[]any{texts, sourceLang, targetLang},
		googleTranslateClient,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal google request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create google request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json+protobuf")
	req.Header.Set("X-Goog-API-Key", t.apiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call google translate: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxGoogleResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read google response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google translate returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	translations, err := parseGoogleResponse(respBody)
	if err != nil {
		return nil, err
	}
	if len(translations) != len(texts) {
		return nil, fmt.Errorf("google translate returned %d translations for %d texts", len(translations), len(texts))
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

var errMissingAPIKey = errors.New("missing GOOGLE_TRANSLATE_API_KEY")

func newApp(translator translator) *app {
	return &app{translator: translator}
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.handleHealthz)
	mux.HandleFunc("/translate", a.handleTranslate)
	return mux
}

func (a *app) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *app) handleTranslate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer r.Body.Close()

	var req translateRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, errorResponse{Error: "request body too large"})
			return
		}
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
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "service is not configured"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "translation upstream failed"})
		return
	}

	translations := make([]translation, 0, len(translatedTexts))
	for _, text := range translatedTexts {
		translations = append(translations, translation{
			DetectedSourceLang: req.SourceLang,
			Text:               text,
		})
	}

	writeJSON(w, http.StatusOK, translateResponse{Translations: translations})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write json response: %v", err)
	}
}
