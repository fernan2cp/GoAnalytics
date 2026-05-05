package main

import (
	"fmt"
	"os"
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
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "go-analytics-ingest: %v\n", err)
		os.Exit(1)
	}
}

// run ejecuta la inicializacion minima del servicio de ingesta.
//
// No recibe entradas y devuelve un error solo cuando la inicializacion del
// servicio falla. En esta fase inicial no crea conexiones ni adaptadores, por
// lo que siempre devuelve nil.
//
// Debe reemplazarse en fases posteriores por el bootstrap real de HTTP,
// configuracion, adaptadores y casos de uso.
func run() error {
	fmt.Println("Go Analytics ingest service bootstrap pending")
	return nil
}
