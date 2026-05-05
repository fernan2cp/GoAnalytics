# Go Analytics

Go Analytics es un proyecto de analitica de eventos web multitenant, pensado como monorepo con servicios Go independientes, un SDK TypeScript y contratos documentados para integrarse con cualquier backend principal.

La base inicial prioriza arquitectura hexagonal: la logica de dominio y aplicacion no depende de HTTP, Redis, PostgreSQL, librerias JWT ni variables de entorno. Los detalles externos viven en adaptadores.

## Estructura

```text
services/
  ingest/        API Go de ingesta de eventos
  worker/        Worker Go de procesamiento
packages/
  web-sdk/       SDK TypeScript preparado para npm
migrations/
  postgres/      Migraciones iniciales de analytics
docs/            Arquitectura, contratos y entorno
IMPLEMENTACION.md
```

## Servicios

- `go-analytics-ingest`: recibe batches desde el SDK, valida JWT, aplica limites basicos y publica en `goanalytics:events:raw`.
- `go-analytics-worker`: consume eventos, valida metadata de site, rehidrata cache si falta y persiste eventos validos.
- `packages/web-sdk`: cliente browser para generar ids, agrupar eventos y enviarlos con `Authorization: Bearer <tracking_jwt>`.

## Comandos

```bash
make test
make tidy
make up
make down
```

## Documentacion

- `docs/architecture.md`
- `docs/hexagonal-architecture.md`
- `docs/jwt-contract.md`
- `docs/event-contract.md`
- `docs/resolver-contract.md`
- `docs/env.md`

## Estado

El seguimiento de fases vive en `IMPLEMENTACION.md`.
