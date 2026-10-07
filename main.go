package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kardianos/service"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	err := runCommand(os.Args[1:], os.Stdout, commandDeps{
		interactive: service.Interactive,
		foreground:  func() error { return runServer(ctx) },
		newService:  createService,
	})
	if err != nil {
		log.Printf("proxy: %v", err)
		var usage *usageError
		if errors.As(err, &usage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
