# Implementacion de Go Analytics

## Estado general

Go Analytics esta en fase inicial. La prioridad actual es dejar lista la estructura del monorepo, los contratos base y las reglas de arquitectura hexagonal.

## Tabla de fases

| Fase | Objetivo | Estado | Fecha de inicio | Fecha de cierre | Notas |
|---|---|---|---|---|---|
| Fase 0 | Diseno base y contratos | En progreso | 2026-05-05 |  | Estructura inicial, docs y contratos base. |
| Fase 1 | Nucleo hexagonal de ingesta | En progreso | 2026-05-05 |  | Dominio, DTOs, puertos y caso de uso testeable implementados; falta validar con Go disponible en PATH. |
| Fase 2 | API HTTP de ingesta | Pendiente |  |  | Endpoint `POST /v1/events`, JWT y Redis Stream. |
| Fase 3 | Nucleo hexagonal del worker | Pendiente |  |  | Procesamiento testeable sin Redis ni PostgreSQL. |
| Fase 4 | Worker con Redis Stream y PostgreSQL | Pendiente |  |  | Consumer, validacion, rehidratacion y persistencia. |
| Fase 5 | Base PostgreSQL analytics | Pendiente |  |  | Migraciones, indices y repositorios. |
| Fase 6 | SDK TypeScript | Pendiente |  |  | Paquete npm, batching y transporte browser. |
| Fase 7 | Docker y entorno local | Pendiente |  |  | Compose completo, healthchecks y comandos. |
| Fase 8 | Integracion con backend principal | Pendiente |  |  | JWT, hidratacion Redis y resolver interno. |
| Fase 9 | Seguridad y hardening | Pendiente |  |  | CORS, rate limit, payload limits y secretos. |
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
- [ ] Completar reglas detalladas de convenciones de eventos.

## Fase 1 - Nucleo hexagonal de ingesta

- [x] Crear reglas de dominio de evento.
- [x] Crear reglas de dominio de token.
- [x] Completar DTOs de aplicacion.
- [x] Completar puerto inbound `IngestEvents`.
- [x] Completar puertos outbound.
- [x] Implementar `IngestEventsUseCase`.
- [x] Agregar tests unitarios con fakes.

## Fase 2 - API HTTP de ingesta

- [ ] Implementar carga de configuracion.
- [ ] Implementar `POST /v1/events`.
- [ ] Implementar middleware de request id.
- [ ] Implementar logs estructurados.
- [ ] Implementar adaptador JWT HS256.
- [ ] Implementar publisher Redis Stream.
- [ ] Implementar rate limiter Redis.
- [ ] Conectar bootstrap.

## Fase 3 - Nucleo hexagonal del worker

- [ ] Completar dominio de site.
- [ ] Completar dominio de rechazo.
- [ ] Implementar `ProcessEventsUseCase`.
- [ ] Implementar validacion de site.
- [ ] Implementar rehidratacion como caso de uso.
- [ ] Agregar tests unitarios con fakes.

## Fase 4 - Worker con Redis Stream y PostgreSQL

- [ ] Implementar consumer Redis Stream.
- [ ] Implementar lectura por batches.
- [ ] Implementar site cache Redis.
- [ ] Implementar resolver HTTP.
- [ ] Implementar cooldown de rehidratacion.
- [ ] Implementar negative cache.
- [ ] Implementar repositorios PostgreSQL.
- [ ] Implementar deduplicacion por `event_id`.

## Fase 5 - Base PostgreSQL analytics

- [ ] Crear migracion de `analytics_events`.
- [ ] Crear migracion de `analytics_rejected_events`.
- [ ] Crear indices base.
- [ ] Implementar insercion batch.

## Fase 6 - SDK TypeScript

- [ ] Configurar TypeScript.
- [ ] Configurar build ESM.
- [ ] Implementar `createAnalyticsClient`.
- [ ] Implementar `track`.
- [ ] Implementar `page`.
- [ ] Implementar queue y batching.
- [ ] Implementar `sendBeacon`.
- [ ] Implementar fallback `fetch` con `keepalive`.
- [ ] Exportar tipos.

## Fase 7 - Docker y entorno local

- [ ] Crear Dockerfile de ingesta.
- [ ] Crear Dockerfile de worker.
- [ ] Completar `docker-compose.yml` con servicios Go.
- [ ] Agregar healthchecks de servicios.
- [ ] Validar comandos del Makefile.

## Fase 8 - Integracion con backend principal

- [ ] Generar JWT desde backend principal.
- [ ] Entregar tracking token al frontend.
- [ ] Hidratar Redis con metadata de site.
- [ ] Implementar resolver interno compatible.
- [ ] Probar rehidratacion automatica.

## Fase 9 - Seguridad y hardening

- [ ] Validar CORS.
- [ ] Validar Origin/Referer.
- [ ] Aplicar rate limit por site.
- [ ] Aplicar rate limit por IP.
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
