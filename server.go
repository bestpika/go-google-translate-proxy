package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	serverReadHeaderTimeout = 10 * time.Second
	serverReadTimeout       = 15 * time.Second
	serverWriteTimeout      = 45 * time.Second
	serverIdleTimeout       = 60 * time.Second
	serverShutdownTimeout   = 10 * time.Second
)

type serverRunner struct {
	server   *http.Server
	listener net.Listener
	done     chan error
	stopOnce sync.Once
	stopErr  error
}

func loadServer() (*http.Server, error) {
	cfg, err := loadRuntimeConfig()
	if err != nil {
		return nil, err
	}
	return newServer(cfg), nil
}

func newServer(cfg config) *http.Server {
	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           newApp(newGoogleTranslator(cfg.GoogleURL, cfg.APIKey, nil)).routes(),
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}
}

func startServer(server *http.Server) (*serverRunner, error) {
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, err
	}
	runner := &serverRunner{server: server, listener: listener, done: make(chan error, 1)}
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		runner.done <- err
	}()
	log.Printf("listening on %s", listener.Addr())
	return runner, nil
}

func (r *serverRunner) stop(ctx context.Context) error {
	r.stopOnce.Do(func() {
		r.stopErr = r.server.Shutdown(ctx)
		if r.stopErr != nil {
			r.stopErr = errors.Join(r.stopErr, r.server.Close())
		}
	})
	return r.stopErr
}

func (r *serverRunner) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	return r.stop(ctx)
}

func runServer(ctx context.Context) error {
	server, err := loadServer()
	if err != nil {
		return err
	}
	return runForeground(ctx, server)
}

func runForeground(ctx context.Context, server *http.Server) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	runner, err := startServer(server)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return runner.shutdown()
	case err := <-runner.done:
		return errors.Join(err, runner.shutdown())
	}
}
