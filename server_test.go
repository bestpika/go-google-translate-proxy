package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

func waitFor[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for test operation")
		var zero T
		return zero
	}
}

func TestNewServer(t *testing.T) {
	t.Chdir(t.TempDir())
	server := newServer(config{Port: "9090", GoogleURL: defaultGoogleURL, APIKey: "key"})
	if server.Addr != ":9090" || server.ReadHeaderTimeout != serverReadHeaderTimeout || server.ReadTimeout != serverReadTimeout || server.WriteTimeout != serverWriteTimeout || server.IdleTimeout != serverIdleTimeout {
		t.Fatalf("server=%+v", server)
	}
	if _, err := os.Stat(".env"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("server construction must not create .env")
	}
}

func TestLoadServerPort(t *testing.T) {
	example, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name            string
		file, env, want string
		external        bool
	}{
		{"external default", "", "", "9009", true},
		{"embedded default", "", "", "9009", false},
		{"existing file", "PORT=8080\n", "", "8080", false},
		{"existing environment", "", "8080", "8080", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PORT", tt.env)
			t.Setenv("GOOGLE_TRANSLATE_URL", "")
			t.Setenv("GOOGLE_TRANSLATE_API_KEY", "")
			wantFile := string(example)
			if tt.external {
				if err := os.WriteFile(".env.example", example, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tt.file != "" {
				wantFile = tt.file
				if err := os.WriteFile(".env", []byte(tt.file), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if server, err := loadServer(); err != nil || server.Addr != ":"+tt.want {
				t.Fatalf("server=%v error=%v want port=%s", server, err, tt.want)
			}
			if content, err := os.ReadFile(".env"); err != nil || string(content) != wantFile {
				t.Fatalf("unexpected .env content=%q error=%v", content, err)
			}
		})
	}
}

func TestServerBindFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server := &http.Server{Addr: listener.Addr().String()}
	if _, err := startServer(server); err == nil {
		t.Fatal("expected occupied port error")
	}
	if err := runForeground(context.Background(), server); err == nil {
		t.Fatal("foreground reported success despite occupied port")
	}
	program := newServiceProgram(func() (*http.Server, error) { return server, nil })
	if err := program.Start(nil); err == nil || program.runner != nil {
		t.Fatal("service reported success despite occupied port")
	}
	if err := program.Stop(nil); err != nil {
		t.Fatal(err)
	}
}

func TestServerStopDrainsRequests(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	finish := func() { releaseOnce.Do(func() { close(release) }) }
	defer finish()
	server := &http.Server{Addr: "127.0.0.1:0", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
			io.WriteString(w, "finished")
		case <-r.Context().Done():
		}
	})}
	runner, err := startServer(server)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { finish(); runner.shutdown() }()
	response := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 4 * time.Second}
		resp, err := client.Get("http://" + runner.listener.Addr().String())
		if err == nil {
			defer resp.Body.Close()
			body, readErr := io.ReadAll(resp.Body)
			err = readErr
			if string(body) != "finished" {
				err = errors.New("in-flight response was interrupted")
			}
		}
		response <- err
	}()
	waitFor(t, entered)
	shutdownStarted := make(chan struct{})
	server.RegisterOnShutdown(func() { close(shutdownStarted) })
	stopped := make(chan error, 1)
	go func() { stopped <- runner.shutdown() }()
	waitFor(t, shutdownStarted)
	select {
	case err := <-stopped:
		t.Fatalf("shutdown finished before handler: %v", err)
	default:
	}
	finish()
	if err := waitFor(t, response); err != nil {
		t.Fatal(err)
	}
	if err := waitFor(t, stopped); err != nil {
		t.Fatal(err)
	}
	if err := runner.shutdown(); err != nil {
		t.Fatal(err)
	}
	if err := waitFor(t, runner.done); err != nil {
		t.Fatal(err)
	}
}

func TestServerStopTimeout(t *testing.T) {
	entered, canceled := make(chan struct{}), make(chan struct{})
	server := &http.Server{Addr: "127.0.0.1:0", Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(canceled)
	})}
	runner, err := startServer(server)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	requestDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 4 * time.Second}
		resp, err := client.Get("http://" + runner.listener.Addr().String())
		if resp != nil {
			resp.Body.Close()
		}
		requestDone <- err
	}()
	waitFor(t, entered)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := runner.stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
	waitFor(t, canceled)
	if err := waitFor(t, requestDone); err == nil {
		t.Fatal("force close did not interrupt connection")
	}
	if err := runner.stop(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("repeated stop error=%v", err)
	}
}

func TestServerConcurrentStop(t *testing.T) {
	runner, err := startServer(&http.Server{Addr: "127.0.0.1:0", Handler: newApp(&stubTranslator{}).routes()})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runner.shutdown(); err != nil {
				t.Errorf("shutdown: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestRunForeground(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	address := make(chan string, 1)
	server := &http.Server{
		Addr: "127.0.0.1:0", Handler: newApp(&stubTranslator{}).routes(),
		BaseContext: func(listener net.Listener) context.Context {
			address <- listener.Addr().String()
			return context.Background()
		},
	}
	done := make(chan error, 1)
	go func() { done <- runForeground(ctx, server) }()
	addr := waitFor(t, address)
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	cancel()
	if err := waitFor(t, done); err != nil {
		t.Fatal(err)
	}
	if err := runForeground(ctx, server); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error=%v", err)
	}
}

func TestServiceProgram(t *testing.T) {
	cause := errors.New("configuration failed")
	program := newServiceProgram(func() (*http.Server, error) { return nil, cause })
	if err := program.Start(nil); !errors.Is(err, cause) {
		t.Fatalf("error=%v", err)
	}
	program = newServiceProgram(func() (*http.Server, error) {
		return &http.Server{Addr: "127.0.0.1:0", Handler: newApp(&stubTranslator{}).routes()}, nil
	})
	if err := program.Start(nil); err != nil {
		t.Fatal(err)
	}
	defer program.Stop(nil)
	if err := program.Start(nil); err == nil {
		t.Fatal("expected duplicate start error")
	}
	for range 2 {
		if err := program.Stop(nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadServerErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PORT", "invalid")
	t.Setenv("GOOGLE_TRANSLATE_URL", "")
	t.Setenv("GOOGLE_TRANSLATE_API_KEY", "")
	if _, err := loadServer(); err == nil {
		t.Fatal("expected configuration error")
	}
	if err := runServer(context.Background()); err == nil {
		t.Fatal("expected foreground configuration error")
	}
	t.Setenv("PORT", "8080")
	if server, err := loadServer(); err != nil || server.Addr != ":8080" {
		t.Fatalf("server=%v error=%v", server, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runServer(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestRunForegroundServeFailure(t *testing.T) {
	listenerReady := make(chan net.Listener, 1)
	server := &http.Server{
		Addr: "127.0.0.1:0",
		BaseContext: func(listener net.Listener) context.Context {
			listenerReady <- listener
			return context.Background()
		},
	}
	done := make(chan error, 1)
	go func() { done <- runForeground(context.Background(), server) }()
	listener := waitFor(t, listenerReady)
	defer server.Close()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitFor(t, done); err == nil {
		t.Fatal("serve failure did not reach foreground")
	}
}
