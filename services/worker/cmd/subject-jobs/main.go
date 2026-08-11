// Command subject-jobs ejecuta procesos reanudables de estadísticas de clientes.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"goanalytics/services/worker/internal/bootstrap"
)

func main() {
	mode := flag.String("mode", "bootstrap", "modo a ejecutar: bootstrap")
	flag.Parse()
	if err := run(*mode); err != nil {
		fmt.Fprintf(os.Stderr, "subject-jobs: %v\n", err)
		os.Exit(1)
	}
}

func run(mode string) error {
	if mode != "bootstrap" {
		return fmt.Errorf("modo no soportado: %s", mode)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	config, err := bootstrap.LoadConfig(os.Getenv("ENV_FILE"))
	if err != nil {
		return err
	}
	container, err := bootstrap.NewSubjectJobsContainer(ctx, config)
	if err != nil {
		return err
	}
	defer container.Close()
	return container.Bootstrap.Run(ctx)
}
