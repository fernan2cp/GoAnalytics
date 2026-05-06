package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"goanalytics/services/worker/internal/bootstrap"
)

// main inicia el entrypoint del worker de Go Analytics.
//
// No recibe parametros directos; la configuracion se lee desde el entorno
// durante el bootstrap. No devuelve valores porque es el punto de entrada del
// proceso.
//
// Debe usarse solo como binario del servicio `go-analytics-worker`. Si la
// inicializacion devuelve error, escribe el mensaje en stderr y finaliza con
// codigo distinto de cero.
func main() {
	envFile := os.Getenv("ENV_FILE")
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			fmt.Fprintf(os.Stderr, "advertencia: no se pudo cargar env file %s: %v\n", envFile, err)
		}
	}

	postgres_host := os.Getenv("POSTGRES_HOST")
	postgres_db := os.Getenv("POSTGRES_DB")
	if postgres_host == "" {
		fmt.Fprintf(os.Stderr, "Postgres Host NO Inicializado")
	} else if postgres_db == "" {
		fmt.Fprintf(os.Stderr, "DB Name faltante")
	} else {
		fmt.Fprintf(os.Stdout, "postgres_host: %s postgres_db: %s\n", postgres_host, postgres_db)
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "go-analytics-worker: %v\n", err)
		os.Exit(1)
	}
}

// run ejecuta el bootstrap real del worker.
//
// No recibe entradas y devuelve error cuando la carga de configuracion,
// conexiones o consumo fallan. Atiende SIGINT y SIGTERM para cerrar Redis y
// PostgreSQL de forma ordenada.
//
// Debe ejecutarse solo desde main. La logica de negocio queda en application y
// los detalles de infraestructura en bootstrap/adapters.
func run() error {
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	config, err := bootstrap.LoadConfig(os.Getenv("ENV_FILE"))
	if err != nil {
		return err
	}
	container, err := bootstrap.NewContainer(ctx, config)
	if err != nil {
		return err
	}
	defer func() {
		if err := container.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "advertencia: cierre de recursos fallo: %v\n", err)
		}
	}()

	healthServer := &http.Server{
		Addr:              config.HealthAddress(),
		Handler:           container.Health,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() {
		container.Logger.Info(ctx, "servidor operativo del worker iniciado", map[string]any{"addr": config.HealthAddress()})
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		errCh <- container.Consumer.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return healthServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if shutdownErr := healthServer.Shutdown(shutdownCtx); shutdownErr != nil {
			return shutdownErr
		}
		return err
	}
}
