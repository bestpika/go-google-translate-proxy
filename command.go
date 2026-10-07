package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kardianos/service"
)

type usageError struct{ message string }

func (e *usageError) Error() string { return e.message }

type commandDeps struct {
	interactive func() bool
	foreground  func() error
	newService  func() (managedService, *serviceProgram, error)
}

func runCommand(args []string, out io.Writer, deps commandDeps) error {
	command := "run"
	if len(args) == 0 {
		if !deps.interactive() {
			command = serviceRunCommand
		}
	} else {
		command = strings.ToLower(args[0])
	}
	switch command {
	case "run", "install", "uninstall", "start", "stop", "restart", "status", "help", "-h", "--help":
		if len(args) > 1 {
			printUsage(out)
			return &usageError{message: "unexpected arguments"}
		}
	case serviceRunCommand:
		if len(args) > 0 {
			if err := applyServiceRunArgs(args[1:]); err != nil {
				return &usageError{message: err.Error()}
			}
		}
	default:
		printUsage(out)
		return &usageError{message: "unknown command"}
	}
	if command == "help" || command == "-h" || command == "--help" {
		printUsage(out)
		return nil
	}
	if command == "run" {
		return deps.foreground()
	}
	svc, program, err := deps.newService()
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	if command == serviceRunCommand {
		return runService(svc, program)
	}
	if command == "status" {
		status, err := svc.Status()
		if err != nil {
			return fmt.Errorf("service status: %w", err)
		}
		_, err = fmt.Fprintln(out, serviceStatusText(status))
		return err
	}
	controls := map[string]func() error{
		"install": svc.Install, "uninstall": svc.Uninstall, "start": svc.Start,
		"stop": svc.Stop, "restart": svc.Restart,
	}
	if err := controls[command](); err != nil {
		return fmt.Errorf("service %s: %w", command, err)
	}
	_, err = fmt.Fprintf(out, "%s %s ok\n", serviceName, command)
	return err
}

func applyServiceRunArgs(args []string) error {
	path := ""
	for i := 0; i < len(args); i++ {
		if args[i] != serviceWorkDirFlag {
			return fmt.Errorf("unknown argument %q", args[i])
		}
		i++
		if i >= len(args) || strings.TrimSpace(args[i]) == "" {
			return fmt.Errorf("%s requires a path", serviceWorkDirFlag)
		}
		if path != "" {
			return fmt.Errorf("%s may only be specified once", serviceWorkDirFlag)
		}
		path = args[i]
	}
	if path != "" {
		if err := os.Chdir(path); err != nil {
			return fmt.Errorf("set working directory: %w", err)
		}
	}
	return nil
}

func printUsage(out io.Writer) {
	fmt.Fprintf(out, "Usage: %s [run|install|uninstall|start|stop|restart|status|help]\n", serviceName)
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
