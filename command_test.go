package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kardianos/service"
)

type fakeService struct {
	command string
	status  service.Status
	err     error
	run     func() error
}

func (s *fakeService) Run() error {
	if s.run != nil {
		return s.run()
	}
	return s.err
}

func (s *fakeService) Status() (service.Status, error) { return s.status, s.err }
func (s *fakeService) Install() error                  { s.command = "install"; return s.err }
func (s *fakeService) Uninstall() error                { s.command = "uninstall"; return s.err }
func (s *fakeService) Start() error                    { s.command = "start"; return s.err }
func (s *fakeService) Stop() error                     { s.command = "stop"; return s.err }
func (s *fakeService) Restart() error                  { s.command = "restart"; return s.err }

func TestRunCommandForegroundAndHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"run"}, {"RUN"}, {"help"}, {"-h"}, {"--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			called := false
			var out bytes.Buffer
			err := runCommand(args, &out, commandDeps{
				interactive: func() bool { return true },
				foreground:  func() error { called = true; return nil },
				newService: func() (managedService, *serviceProgram, error) {
					t.Fatal("foreground/help must not create a service")
					return nil, nil, nil
				},
			})
			wantRun := len(args) == 0 || strings.EqualFold(args[0], "run")
			if err != nil || called != wantRun || (!wantRun && !strings.Contains(out.String(), "Usage:")) {
				t.Fatalf("called=%v output=%q error=%v", called, out.String(), err)
			}
		})
	}
	cause := errors.New("foreground failed")
	if err := runCommand([]string{"run"}, io.Discard, commandDeps{foreground: func() error { return cause }}); !errors.Is(err, cause) {
		t.Fatalf("error=%v", err)
	}
}

func TestRunCommandUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"run", "extra"}, {"help", "extra"}, {serviceRunCommand, "bad"}, {serviceRunCommand, serviceWorkDirFlag}} {
		var out bytes.Buffer
		err := runCommand(args, &out, commandDeps{})
		var usage *usageError
		if !errors.As(err, &usage) || usage.Error() == "" {
			t.Fatalf("args=%v error=%v", args, err)
		}
	}
}

func TestRunCommandServices(t *testing.T) {
	for _, command := range []string{"install", "uninstall", "start", "stop", "restart", "status", serviceRunCommand, "SERVICE-RUN", ""} {
		t.Run(command, func(t *testing.T) {
			svc := &fakeService{status: service.StatusRunning}
			deps := commandDeps{
				interactive: func() bool { return false },
				newService: func() (managedService, *serviceProgram, error) {
					return svc, newServiceProgram(nil), nil
				},
			}
			var args []string
			if command != "" {
				args = []string{command}
			}
			var out bytes.Buffer
			if err := runCommand(args, &out, deps); err != nil {
				t.Fatal(err)
			}
			if command == "status" {
				if out.String() != "running\n" {
					t.Fatalf("output=%q", out.String())
				}
			} else if command != "" && !strings.EqualFold(command, serviceRunCommand) {
				if svc.command != command || out.String() != serviceName+" "+command+" ok\n" {
					t.Fatalf("command=%q output=%q", svc.command, out.String())
				}
			}
		})
	}
	for _, command := range []string{"install", "status", serviceRunCommand} {
		cause := errors.New("service error")
		deps := commandDeps{newService: func() (managedService, *serviceProgram, error) {
			return &fakeService{err: cause}, newServiceProgram(nil), nil
		}}
		if err := runCommand([]string{command}, io.Discard, deps); !errors.Is(err, cause) {
			t.Fatalf("command=%s error=%v", command, err)
		}
	}
	cause := errors.New("factory error")
	if err := runCommand([]string{"status"}, io.Discard, commandDeps{newService: func() (managedService, *serviceProgram, error) {
		return nil, nil, cause
	}}); !errors.Is(err, cause) {
		t.Fatalf("error=%v", err)
	}
}

func TestApplyServiceRunArgs(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, args := range [][]string{{"bad"}, {serviceWorkDirFlag}, {serviceWorkDirFlag, " "}, {serviceWorkDirFlag, dir, "bad"}, {serviceWorkDirFlag, dir, serviceWorkDirFlag, dir}, {serviceWorkDirFlag, "missing"}} {
		if err := applyServiceRunArgs(args); err == nil {
			t.Fatalf("args=%v expected error", args)
		}
		if got, _ := os.Getwd(); got != cwd {
			t.Fatalf("invalid arguments changed cwd to %q", got)
		}
	}
	if err := applyServiceRunArgs(nil); err != nil {
		t.Fatal(err)
	}
	if err := applyServiceRunArgs([]string{serviceWorkDirFlag, dir}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.Stat(".")
	want, _ := os.Stat(dir)
	if !os.SameFile(got, want) {
		t.Fatal("working directory not applied")
	}
}

func TestServiceConfiguration(t *testing.T) {
	cfg, err := newServiceConfig()
	if err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if cfg.Name != serviceName || cfg.WorkingDirectory != cwd || !reflect.DeepEqual(cfg.Arguments, []string{serviceRunCommand, serviceWorkDirFlag, cwd}) || cfg.Option["Restart"] != "on-failure" {
		t.Fatalf("config=%+v", cfg)
	}
	for status, want := range map[service.Status]string{service.StatusRunning: "running", service.StatusStopped: "stopped", service.StatusUnknown: "unknown"} {
		if got := serviceStatusText(status); got != want {
			t.Fatalf("status=%d text=%q", status, got)
		}
	}
}

func TestCreateService(t *testing.T) {
	svc, program, err := createService()
	if err != nil || svc == nil || program == nil || program.newServer == nil {
		t.Fatalf("service=%v program=%v error=%v", svc, program, err)
	}
}

func TestRunServiceFailure(t *testing.T) {
	program := newServiceProgram(func() (*http.Server, error) {
		return &http.Server{Addr: "127.0.0.1:0", Handler: newApp(&stubTranslator{}).routes()}, nil
	})
	if err := program.Start(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { program.Stop(nil) })
	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) })
	svc := &fakeService{run: func() error { <-blocked; return nil }}
	done := make(chan error, 1)
	go func() { done <- runService(svc, program) }()
	if err := program.runner.listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitFor(t, done); err == nil {
		t.Fatal("serve failure did not reach the entry point")
	}
}
