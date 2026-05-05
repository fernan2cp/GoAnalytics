package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goanalytics/services/ingest/internal/bootstrap"

	"github.com/joho/godotenv"
)

// main inicia el entrypoint del servicio de ingesta de Go Analytics.
//
// No recibe parametros directos; la configuracion futura se leera desde el
// entorno durante el bootstrap. No devuelve valores porque es el punto de
// entrada del proceso.
//
// Debe usarse solo como binario del servicio `go-analytics-ingest`. Si la
// inicializacion devuelve error, escribe el mensaje en stderr y finaliza con
// codigo distinto de cero.
func main() {
	envFile := os.Getenv("ENV_FILE")
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load env file %s: %v\n", envFile, err)
		}
	}
	
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "go-analytics-ingest: %v\n", err)
		os.Exit(1)
	}
}

// run ejecuta la inicializacion completa del servicio de ingesta.
//
// No recibe entradas y devuelve error cuando falla la configuracion, la
// conexion con Redis o el servidor HTTP. Lee `.env`, ensambla adaptadores y
// mantiene el proceso activo hasta recibir una senal de cierre.
func run() error {
	config, err := bootstrap.LoadConfig(".env")
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	container, err := bootstrap.NewContainer(ctx, config)
	if err != nil {
		return err
	}
	defer container.Close()

	server := &http.Server{
		Addr:              config.Address(),
		Handler:           container.Handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		container.Logger.Info(ctx, "servicio de ingesta iniciado", map[string]any{"addr": config.Address()})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
