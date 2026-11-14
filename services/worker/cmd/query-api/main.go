// Command query-api sirve sugerencias internas de sujetos.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goanalytics/services/worker/internal/bootstrap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "query-api: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	config, err := bootstrap.LoadConfig(os.Getenv("ENV_FILE"))
	if err != nil {
		return err
	}
	if !config.SubjectQueryAPIEnabled {
		return fmt.Errorf("query API deshabilitada")
	}
	container, err := bootstrap.NewQueryAPIContainer(ctx, config)
	if err != nil {
		return err
	}
	defer container.Close()
	server := &http.Server{Addr: config.SubjectQueryAddress(), Handler: container.Handler, ReadHeaderTimeout: config.SubjectQueryReadTimeout, ReadTimeout: config.SubjectQueryReadTimeout, WriteTimeout: config.SubjectQueryWriteTimeout}
	errors := make(chan error, 1)
	go func() {
		serveErr := server.ListenAndServe()
		if serveErr == http.ErrServerClosed {
			serveErr = nil
		}
		errors <- serveErr
	}()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	case err = <-errors:
		return err
	}
}
