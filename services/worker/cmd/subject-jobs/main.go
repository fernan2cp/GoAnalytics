// Command subject-jobs ejecuta procesos reanudables de estadísticas de clientes.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goanalytics/services/worker/internal/bootstrap"
)

func main() {
	mode := flag.String("mode", "serve", "modo: serve, bootstrap, project-global, project-context, refresh-registry o resolve-initial-contexts")
	forceNew := flag.Bool("force-new-snapshot", false, "iniciar un snapshot nuevo (solo modo bootstrap)")
	flag.Parse()
	if err := run(*mode, *forceNew); err != nil {
		fmt.Fprintf(os.Stderr, "subject-jobs: %v\n", err)
		os.Exit(1)
	}
}

func run(mode string, forceNew ...bool) error {
	force := len(forceNew) > 0 && forceNew[0]
	if force && mode != "bootstrap" {
		return fmt.Errorf("force-new-snapshot requiere modo bootstrap")
	}
	if mode != "bootstrap" && mode != "project-global" && mode != "project-context" && mode != "refresh-registry" && mode != "resolve-initial-contexts" && mode != "serve" {
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
	switch mode {
	case "bootstrap":
		if container.Bootstrap == nil {
			return fmt.Errorf("bootstrap de sujetos deshabilitado")
		}
		return container.Bootstrap.Run(ctx, force)
	case "project-global":
		if container.ProjectGlobal == nil {
			return fmt.Errorf("proyección global deshabilitada")
		}
		return container.ProjectGlobal.Run(ctx)
	case "project-context":
		if container.ProjectContext == nil {
			return fmt.Errorf("proyección contextual deshabilitada")
		}
		return container.ProjectContext.Run(ctx)
	case "refresh-registry":
		if container.Registry == nil {
			return fmt.Errorf("registro de contextos deshabilitado")
		}
		_, err := container.Registry.Refresh(ctx)
		return err
	case "resolve-initial-contexts":
		if container.ResolveInitial == nil {
			return fmt.Errorf("resolución inicial deshabilitada")
		}
		return container.ResolveInitial.Run(ctx)
	default:
		if container.Scheduler == nil {
			return fmt.Errorf("scheduler de proyección deshabilitado")
		}
		return serve(ctx, config, container)
	}
}

func serve(ctx context.Context, config bootstrap.Config, container *bootstrap.SubjectJobsContainer) error {
	server := &http.Server{Addr: config.SubjectJobsHealthAddress(), Handler: container.Health, ReadHeaderTimeout: 5 * time.Second}
	errors := make(chan error, 2)
	go func() {
		err := server.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		errors <- err
	}()
	go func() { errors <- container.Scheduler.Serve(ctx) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errors:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return err
	}
}
