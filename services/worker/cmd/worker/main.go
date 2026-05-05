package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	return container.Consumer.Run(ctx)
}
