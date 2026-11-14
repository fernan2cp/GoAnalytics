# Baseline Fase 0

## Estado Actual Confirmado

GoAnalytics está compuesto por dos módulos Go, un SDK TypeScript, Redis y PostgreSQL. `go.work` incluye `services/ingest` y `services/worker`; no existen módulos o binarios para consulta analítica o jobs de sujetos.

La ingesta pública registra únicamente `POST /v1/events`, `GET /health` y `GET /ready`. Valida tracking JWT HS256, batch, payload y rate limiting antes de publicar en Redis Stream. El dominio exige identidad anónima o sesión, `origin`, `url` y `path`, por lo que no sirve sin cambios para eventos server-to-server.

El worker consume `goanalytics:events:raw` mediante `XREADGROUP` con selector `>`, valida site, seguridad de payload y deduplicación, y confirma con `XACK` después de persistir. No usa `XAUTOCLAIM`, no recupera mensajes pendientes y mapea el batch de forma indivisible: un payload de stream inválido impide procesar los demás mensajes leídos con él.

La deduplicación Redis distingue estrategias exacta, lógica, idempotente y semántica. Las claves lógica e idempotente se acotan por `tenant_id + site_id`. PostgreSQL posee índices únicos equivalentes, pero `EventRepository.SaveBatch` usa una transacción para todo el batch y solo resuelve conflicto por `event_id`; otro conflicto único puede revertir eventos no relacionados.

`analytics_events` conserva evento, tenant, site, navegación, `properties` y `context`. Las migraciones `003` y `004` agregan idempotencia y normalización 1:N de items y órdenes. Este patrón es reutilizable para sujetos.

`SiteConfig`, el resolver HTTP y la caché Redis contienen tenant, site, estado, dominios, token, sampling y schema version. No contienen `source` ni zona horaria.

El worker ya sigue puertos hexagonales para consumer, persistencia, resolver, caché, deduplicación, reloj y logger. Los componentes nuevos deben conservar esta frontera y no importar infraestructura desde domain o application.

`docker-compose.yml` despliega ingest, worker, Redis, PostgreSQL, migraciones y sandbox. El Dockerfile del worker compila un único `cmd/worker`; deberá permitir imágenes o targets separados para jobs y query API.

## Capacidades Reutilizables

- Contrato genérico `properties`/`context` y alias públicos.
- Publicación asíncrona en Redis Stream.
- Validación JWT pública sin reglas de negocio.
- Resolución y caché de sites.
- Persistencia transaccional con `pgx`.
- Esquema de eventos crudos y detalles 1:N.
- Configuración desde entorno y health HTTP.
- Logger estructurado y reloj inyectable.
- Migraciones con `golang-migrate`.
- Suite Go y SDK existente.

## Gaps Bloqueantes

- Recuperación de PEL de Redis y política para mensajes irrecuperables.
- Resultado de persistencia por evento ante conflictos únicos.
- Perfil de validación distinto para eventos server-to-server.
- `event_source` persistido y transportado.
- Zona horaria IANA en resolver, dominio y caché.
- Tablas, dominio y casos de uso de sujetos.
- Contrato fuente de snapshot y registro de contextos.
- Jobs, checkpoints, locks y API de consulta.
- Métricas y alertas de procesos asíncronos.

## Decisiones Cerradas

- El MVP incluye las Fases 0 a 4 completas.
- La integración es end-to-end, aunque el proyecto principal no está en este workspace.
- La identidad de cliente es única por tenant, no por site.
- `customer` es el único `subject_type` habilitado.
- El endpoint interno usa Bearer dedicado y el mismo stream.
- El proyecto principal usa retry directo, no outbox.
- Lifecycle se ordena por `source_updated_at + source_mutation_id`.
- Backfill incluye activos, inactivos y fusionados.
- La proyección es diaria y sirve stale hasta 48 horas por defecto.
- Query API y jobs son binarios separados dentro del módulo worker.
- Las familias iniciales son `search`, `selection`, `transaction`, `service`, `support`, `content` y `navigation`.
- El fallback es contexto exacto a global.
- La capacidad está apagada por defecto y usa migraciones aditivas.

## Supuestos De Dimensionamiento

- Hasta 20 tenants.
- Hasta 100.000 clientes totales.
- Hasta 500.000 eventos de sujetos por mes.
- `limit` normal de consulta igual a 10 y máximo 100.
- Backfill máximo de 500 sujetos por página.
- Una instancia sirve a un solo proyecto consumidor.

## Riesgos Aceptados

- Retry directo puede perder una señal si fallan todos los intentos y el proceso termina; se exige métrica, log y procedimiento de recuperación manual.
- El desempate lexicográfico de lifecycle es determinista, pero no prueba causalidad cuando dos mutaciones comparten timestamp.
- Un merge no transfiere conteos y puede reducir temporalmente la relevancia del sujeto canónico.
- Una corrección de asociación cuenta sujeto anterior y nuevo.
- Datos globales pueden tener hasta 24 horas de demora; después de 48 horas se consideran stale.

## Restricción De Alcance Documental

La creación de este SDD no modifica código, migraciones, contratos públicos ni la guía fuente. Las rutas del proyecto principal se describen por comportamiento porque ese repositorio no está disponible.
