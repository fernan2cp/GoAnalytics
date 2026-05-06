# Implementacion de Go Analytics

## Estado general

Go Analytics esta en fase inicial. La prioridad actual es dejar lista la estructura del monorepo, los contratos base y las reglas de arquitectura hexagonal.

## Tabla de fases

| Fase | Objetivo | Estado | Fecha de inicio | Fecha de cierre | Notas |
|---|---|---|---|---|---|
| Fase 0 | Diseno base y contratos | Completada | 2026-05-05 | 2026-05-05 | Estructura inicial, docs, contratos base y convenciones de eventos. |
| Fase 1 | Nucleo hexagonal de ingesta | Completada | 2026-05-05 | 2026-05-05 | Dominio, DTOs, puertos y caso de uso testeable validados con Go en ruta explicita. |
| Fase 2 | API HTTP de ingesta | Completada | 2026-05-05 | 2026-05-05 | Endpoint `POST /v1/events`, JWT HS256, Redis Stream, rate limit Redis y bootstrap conectados. |
| Fase 3 | Nucleo hexagonal del worker | Completada | 2026-05-05 | 2026-05-05 | Procesamiento testeable sin Redis ni PostgreSQL. |
| Fase 4 | Worker con Redis Stream y PostgreSQL | Completada | 2026-05-05 | 2026-05-05 | Consumer Redis Stream, cache/resolver, cooldown, negative cache, repositorios pgx y deduplicacion conectados. |
| Fase 5 | Base PostgreSQL analytics | Completada | 2026-05-05 | 2026-05-05 | Migraciones `up/down`, indices base, repositorios pgx, insercion batch y ejecucion con `golang-migrate` configurados. |
| Fase 6 | SDK TypeScript | Completada | 2026-05-05 | 2026-05-05 | Cliente funcional con `track`, `page`, `identify`, queue, batching, `fetch keepalive`, soporte opcional de `sendBeacon` y tipos exportados. |
| Fase 7 | Docker y entorno local | Completada | 2026-05-05 | 2026-05-06 | Dockerfiles, servicios Go en Compose, healthchecks HTTP y Makefile validados. |
| Fase 8 | Integracion con backend principal | Pendiente |  |  | JWT, hidratacion Redis y resolver interno. |
| Fase 9 | Seguridad y hardening | Parcial | 2026-05-05 |  | Rate limit logico definido en application; faltan adaptador Redis, CORS, Origin/Referer, payload size y bloqueo de secretos. |
| Fase 10 | Observabilidad | Pendiente |  |  | Health, readiness, logs y metricas basicas. |
| Fase 11 | Preparacion produccion | Pendiente |  |  | Secretos, recursos, backups y reinicios. |
| Fase 12 | Publicacion futura SDK npm | Pendiente |  |  | Versionado, README SDK y pipeline futuro. |

## Fase 0 - Diseno base y contratos

- [x] Definir Go Analytics como nombre principal del proyecto.
- [x] Crear estructura base de monorepo.
- [x] Crear `go.work` con servicios Go independientes.
- [x] Crear README inicial.
- [x] Crear `.env.example`.
- [x] Crear documentacion inicial en `docs`.
- [x] Crear archivo de seguimiento `IMPLEMENTACION.md`.
- [x] Completar reglas detalladas de convenciones de eventos.

## Fase 1 - Nucleo hexagonal de ingesta

- [x] Crear reglas de dominio de evento.
- [x] Crear reglas de dominio de token.
- [x] Completar DTOs de aplicacion.
- [x] Completar puerto inbound `IngestEvents`.
- [x] Completar puertos outbound.
- [x] Implementar `IngestEventsUseCase`.
- [x] Agregar tests unitarios con fakes.

## Fase 2 - API HTTP de ingesta

- [x] Implementar carga de configuracion.
- [x] Implementar `POST /v1/events`.
- [x] Implementar middleware de request id.
- [x] Implementar logs estructurados.
- [x] Implementar adaptador JWT HS256.
- [x] Implementar publisher Redis Stream.
- [x] Implementar rate limiter Redis.
- [x] Conectar bootstrap.

## Fase 3 - Nucleo hexagonal del worker

- [x] Completar dominio de site.
- [x] Completar dominio de rechazo.
- [x] Implementar `ProcessEventsUseCase`.
- [x] Implementar validacion de site.
- [x] Implementar rehidratacion como caso de uso.
- [x] Agregar tests unitarios con fakes.

## Fase 4 - Worker con Redis Stream y PostgreSQL

- [x] Implementar consumer Redis Stream.
- [x] Implementar lectura por batches.
- [x] Implementar site cache Redis.
- [x] Implementar resolver HTTP.
- [x] Implementar cooldown de rehidratacion.
- [x] Implementar negative cache.
- [x] Implementar conexion PostgreSQL en bootstrap del worker con `pgxpool`.
- [x] Implementar repositorios PostgreSQL con `pgx`.
- [x] Implementar deduplicacion por `event_id`.

## Fase 5 - Base PostgreSQL analytics

- [x] Crear migracion de `analytics_events`.
- [x] Crear migracion de `analytics_rejected_events`.
- [x] Crear indices base.
- [x] Configurar ejecucion de migraciones con `golang-migrate`.
- [x] Implementar insercion batch.

## Fase 6 - SDK TypeScript

- [x] Configurar TypeScript.
- [x] Configurar build ESM.
- [x] Implementar `createAnalyticsClient`.
- [x] Implementar `track`.
- [x] Implementar `page`.
- [x] Implementar queue y batching.
- [x] Implementar `sendBeacon`.
- [x] Implementar fallback `fetch` con `keepalive`.
- [x] Exportar tipos.

## Fase 7 - Docker y entorno local

- [x] Crear Dockerfile de ingesta.
- [x] Crear Dockerfile de worker.
- [x] Completar `docker-compose.yml` con servicios Go.
- [x] Agregar healthchecks de Redis y PostgreSQL.
- [x] Agregar healthchecks de servicios Go.
- [x] Validar comandos del Makefile.

## Fase 8 - Integracion con backend principal

**Verificación realizada en proyecto de integración según:**
- [Instrucciones de integración mínima](docs/integration/instrucciones-integracion-go-analytics-minimo.md)
- [Agent para backend principal](docs/integration/agent-backend-principal.md)
- [Verificación Fase 8](docs/integration/verificacion-fase-8-backend-principal.md)
- [x] Generar JWT desde backend principal.
- [x] Entregar tracking token al frontend.
- [x] Hidratar Redis con metadata de site.
- [x] Implementar resolver interno compatible.
- [x] Probar rehidratacion automatica.

## Fase 9 - Seguridad y hardening

- [ ] Validar CORS.
- [ ] Validar Origin/Referer.
- [x] Definir rate limit por site en el nucleo de ingesta.
- [x] Definir rate limit por IP en el nucleo de ingesta.
- [x] Implementar rate limit Redis por site.
- [x] Implementar rate limit Redis por IP.
- [ ] Bloquear claves sensibles.
- [ ] Guardar IP hasheada, nunca cruda.

## Fase 10 - Observabilidad

- [ ] Agregar `GET /health`.
- [ ] Agregar `GET /ready`.
- [ ] Agregar logs estructurados.
- [ ] Preparar metricas basicas.
- [ ] Preparar Prometheus futuro.

## Fase 11 - Preparacion produccion

- [ ] Separar configuracion development y production.
- [ ] Configurar secretos seguros.
- [ ] Configurar Redis con password.
- [ ] Configurar usuario restringido de PostgreSQL.
- [ ] Configurar backups.
- [ ] Configurar restart policies.

## Fase 12 - Publicacion futura SDK npm

- [ ] Definir nombre del paquete.
- [ ] Completar README del SDK.
- [ ] Configurar `files` para npm.
- [ ] Generar `.d.ts`.
- [ ] Preparar versionado semantico.
- [ ] Preparar pipeline de publicacion futura.
