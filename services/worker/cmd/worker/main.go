package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// main inicia el entrypoint del worker de Go Analytics.
//
// No recibe parametros directos; la configuracion futura se leera desde el
// entorno durante el bootstrap. No devuelve valores porque es el punto de
// entrada del proceso.
//
// Debe usarse solo como binario del servicio `go-analytics-worker`. Si la
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
		fmt.Fprintf(os.Stderr, "go-analytics-worker: %v\n", err)
		os.Exit(1)
	}
}

// run ejecuta la inicializacion minima del worker.
//
// No recibe entradas y devuelve un error solo cuando la inicializacion del
// worker falla. En esta fase inicial no consume Redis Stream ni abre conexiones
// a PostgreSQL, por lo que siempre devuelve nil.
//
// Debe reemplazarse en fases posteriores por el bootstrap real del consumer,
// configuracion, adaptadores y casos de uso.
func run() error {
	fmt.Println("Go Analytics worker service bootstrap pending")
	return nil
}
