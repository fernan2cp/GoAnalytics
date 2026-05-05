# Arquitectura de Go Analytics

Go Analytics se compone de una API de ingesta, un worker de procesamiento, Redis, PostgreSQL y un SDK TypeScript.

```text
Frontend -> SDK -> go-analytics-ingest -> Redis Stream -> go-analytics-worker -> PostgreSQL
                                      worker -> Site Resolver interno
```

La ingesta debe responder rapido y no escribir directamente en la base final. El worker concentra la validacion contra metadata real del site y la persistencia.

La decision de persistencia esta documentada en `docs/persistence.md`: PostgreSQL es el almacenamiento inicial, `pgx/pgxpool` vive en adaptadores outbound del worker y las migraciones se administran con `golang-migrate`.

## Componentes

- Backend principal: genera JWT, hidrata Redis y expone una URL interna de rehidratacion.
- SDK: crea eventos, ids de usuario anonimo y sesion, y envia batches.
- Ingest: valida token y estructura basica, aplica rate limit y publica en stream.
- Worker: consume stream, valida site, deduplica y persiste.
- Redis: stream, cache de site, cooldown, negative cache y rate limit.
- PostgreSQL: almacenamiento inicial de eventos validos y rechazados, accesible solo desde adaptadores de infraestructura del worker.
