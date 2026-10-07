package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/kardianos/service"
)

const (
	serviceName        = "go-google-translate-proxy"
	serviceDisplayName = "Google Translate Proxy"
	serviceDescription = "Google Translate proxy for Immersive Translate custom API."
	serviceRunCommand  = "service-run"
	serviceWorkDirFlag = "--workdir"
)

type managedService interface {
	Run() error
	Status() (service.Status, error)
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Restart() error
}

type serviceProgram struct {
	mu              sync.Mutex
	runner          *serverRunner
	newServer       func() (*http.Server, error)
	shutdownTimeout time.Duration
	failures        chan error
}

func newServiceProgram(factory func() (*http.Server, error)) *serviceProgram {
	return &serviceProgram{newServer: factory, shutdownTimeout: serverShutdownTimeout, failures: make(chan error, 1)}
}

func newServiceConfig() (*service.Config, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &service.Config{
		Name:             serviceName,
		DisplayName:      serviceDisplayName,
		Description:      serviceDescription,
		WorkingDirectory: workingDirectory,
		Arguments:        []string{serviceRunCommand, serviceWorkDirFlag, workingDirectory},
		Option: service.KeyValue{
			"Restart": "on-failure", "OnFailure": "restart", "OnFailureDelayDuration": "5s",
		},
	}, nil
}

func createService() (managedService, *serviceProgram, error) {
	cfg, err := newServiceConfig()
	if err != nil {
		return nil, nil, err
	}
	program := newServiceProgram(loadServer)
	svc, err := service.New(program, cfg)
	return svc, program, err
}

func (p *serviceProgram) Start(service.Service) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.runner != nil {
		return errors.New("service is already started")
	}
	server, err := p.newServer()
	if err != nil {
		return err
	}
	runner, err := startServer(server)
	if err != nil {
		return err
	}
	p.runner = runner
	go func() {
		if err := <-runner.done; err != nil {
			p.failures <- err
		}
	}()
	return nil
}

func (p *serviceProgram) Stop(service.Service) error {
	p.mu.Lock()
	runner := p.runner
	p.mu.Unlock()
	if runner == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.shutdownTimeout)
	defer cancel()
	return runner.stop(ctx)
}

func runService(svc managedService, program *serviceProgram) error {
	done := make(chan error, 1)
	go func() { done <- svc.Run() }()
	select {
	case err := <-done:
		return errors.Join(err, program.Stop(nil))
	case err := <-program.failures:
		return errors.Join(err, program.Stop(nil))
	}
}
