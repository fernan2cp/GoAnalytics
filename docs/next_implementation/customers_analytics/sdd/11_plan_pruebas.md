# Plan De Pruebas

## Comandos Base

Desde la raíz:

```powershell
make test
```

```powershell
npm test --prefix packages/web-sdk
```

Validación focal:

```powershell
go test ./... -count=1
```

Ejecutar el comando anterior dentro de `services/ingest` y `services/worker`.

Las pruebas PostgreSQL y Redis usan servicios efímeros o el compose de test definido durante implementación. No deben depender de datos locales permanentes.

## Fase 0

### Compatibilidad

- Evento web actual aceptado sin `event_source` enviado.
- Aliases y respuestas actuales sin cambios.
- Flags deshabilitados mantienen flujo y esquema funcional previo.

### Redis

- Reclamar mensaje pendiente después de superar min idle.
- Procesar reclamado antes de mensajes nuevos.
- No reclamar mensaje todavía activo.
- Reintentar hasta delivery 5.
- Persistir `stream_retry_exhausted` antes de `XACK`.
- Dejar pendiente si falla el repositorio de rechazos.
- Un payload inválido no bloquea mensajes válidos del mismo read.

### PostgreSQL

- Duplicado exacto clasificado sin error.
- Duplicado lógico e idempotente después de simular expiración Redis.
- Conflicto contradictorio auditable.
- Un duplicado no revierte otro evento del lote.
- Error desconocido no confirma Redis.

## Fase 1

### Endpoint Interno

- Token válido, ausente e inválido.
- Capacidad deshabilitada devuelve 404.
- Batch único y múltiple.
- Payload demasiado grande y más de 50 eventos.
- Campos de navegación ausentes aceptados para server.
- Los mismos campos siguen requeridos para web.
- `event_source` del cliente no puede falsificarse.

### Sujetos

- Uno y varios elementos en `subjects`.
- Duplicados dentro del payload.
- Tipo no soportado y sujeto incompleto conservan evento crudo.
- IDs iguales en dos tenants no colisionan.
- IDs iguales en dos sites del mismo tenant convergen.
- Asociación idéntica entre sites incrementa una sola vez.
- Transacción por evento conserva fila cruda y detalles atómicamente.

### Resolver Y Zona

- Request web actual compatible sin `source`.
- Request server sin origin.
- Zona IANA válida propagada a dominio y caché.
- Zona inválida produce `timezone_unresolved`.
- Cambio de schema de caché compatible con entradas previas.

### Lifecycle

- Updated at mayor aplica.
- Updated at menor se ignora.
- Timestamp igual desempata por mutation ID.
- Mismo timestamp y mutation ID es idempotente.
- `effective_at` distinto no altera precedencia.
- Merge entre tenants o hacia sí mismo rechazado semánticamente.
- Reactivación posterior a merge limpia destino.

## Fase 2

- Primera página fija revisión.
- Páginas siguientes exigen la misma revisión.
- Cursor y conteos se confirman juntos.
- Reanudación continúa desde último cursor.
- `409` marca failed sin mezclar revisiones.
- `--force-new-snapshot` inicia ejecución nueva.
- Backfill incluye active, inactive y merged.
- Destino de merge fuera de página se crea `unknown`.
- Backfill concurrente con evento usa la tupla más nueva.
- Dos procesos no ejecutan bootstrap simultáneamente.
- 100.000 sujetos completan en 60 minutos o menos.

## Fase 3

- Primera proyección corre sin checkpoint.
- Ejecución no vencida no se repite.
- Advisory lock evita concurrencia.
- Watermark no omite filas con mismo timestamp.
- Cancelación antes de commit no avanza checkpoint.
- Repetir lote no duplica conteos.
- Evento tardío recalcula día antiguo.
- Cada tipo de evento incrementa solo su contador.
- `last_association_at` usa máximo correcto.
- Lifecycle no incrementa señales.
- Sujetos no active se excluyen al consultar.
- Proyección de perfil MVP finaliza en 30 minutos o menos.

### Query Global

- Autenticación y parámetros inválidos.
- Orden y desempate deterministas.
- Ventana global de 90 días.
- Sin datos devuelve 200 vacío.
- Sin proyección devuelve 503.
- Stale permitido y rechazado según flag.
- Aislamiento con IDs iguales entre tenants.
- Respuesta sin ID interno o PII.
- p95 menor o igual a 300 ms para limit 10.

## Fase 4

### Registro

- Siete familias válidas y familia desconocida rechazada.
- Clave/versión duplicada.
- Padre ausente, familia distinta y ciclo.
- Definición publicada modificada.
- Misma revisión con checksum diferente.
- Revisión inválida no reemplaza activa.
- Fallback memoria, Redis, PostgreSQL y HTTP.
- Lock con token no elimina lock ajeno.

### Resolución Y Agregado

- Contexto active queda registered.
- Inactive queda inactive_context.
- Desconocido queda unregistered y cuenta global.
- Familia declarada distinta usa la resuelta.
- Resolución inicial es reanudable e idempotente.
- Cambio de detalle actualiza watermark contextual.
- Proyección contextual no incluye no registrados.

### Query Contextual

- Exact context usa 30 días.
- Completa con global de 90 días.
- No duplica sujetos.
- Conserva conteo del source level.
- Contexto inactivo o desconocido usa global e informa estado.
- No existe fallback por familia.

## Contratos Del Proyecto Principal

- Resolver responde timezone y conserva compatibilidad web.
- Snapshot no contiene PII y mantiene revisión.
- Registry endpoint entrega payload validado.
- Retry reutiliza IDs y respeta 250/1000 ms.
- `400/401` no se reintentan; `429/5xx/network` sí.
- Fallo final no altera la operación de negocio.
- Consumo de sugerencias recupera clientes dentro del tenant.
- Caída de GoAnalytics conserva comportamiento previo.

## Migraciones

- Up `001` a `008` desde base vacía.
- Up `005` a `008` sobre esquema en `004` con datos.
- Defaults preservan eventos existentes como web.
- Down `008` a `004` en ambiente desechable.
- FKs, checks, índices parciales y unicidades ejercitados.

## Evidencia Por Criterio

Cada `AC-CA-*` debe registrar comando, test, fixture, salida y fecha en la evidencia de su `TASK-CA-*`. Rendimiento debe indicar hardware, dataset y configuración.

### Mapa De Validación Objetiva

| Criterio | Evidencia mínima exigida |
|---|---|
| `AC-CA-001` | Regresión completa de ingest, worker y SDK con flags apagados; migración sobre `004` con eventos existentes. |
| `AC-CA-002` | Integración Redis que mate el consumidor después de leer y demuestre recuperación con `XAUTOCLAIM` y confirmación posterior. |
| `AC-CA-003` | Lote mixto válido/inválido, rechazo durable, ausencia de bloqueo y prueba de fallo del repositorio de rechazos. |
| `AC-CA-004` | Integración PostgreSQL concurrente con conflictos exacto, lógico y funcional junto con un evento válido. |
| `AC-CA-005` | Suite HTTP comparativa para endpoint web e interno, autenticación, campos requeridos, respuesta y publicación al stream. |
| `AC-CA-006` | Pruebas de contrato y caché para source/timezone válida, ausente, inválida y entrada legacy. |
| `AC-CA-007` | Fixtures versionados de asociación/lifecycle con uno, varios, duplicados y formas ambiguas rechazadas. |
| `AC-CA-008` | Carrera de inserciones con misma identidad, tenants distintos y tipo no habilitado. |
| `AC-CA-009` | Matriz de origen/evento/estado que compruebe creación y elegibilidad de unknown, active, inactive y merged. |
| `AC-CA-010` | Tabla dirigida de lifecycle fuera de orden, timestamp empatado, duplicado, effective_at divergente y merge. |
| `AC-CA-011` | Mismo hecho enviado por dos sites del mismo tenant y verificación de una sola asociación funcional. |
| `AC-CA-012` | Contract tests del snapshot multipágina, límite 500, revisión fija y error `409 snapshot_revision_expired`. |
| `AC-CA-013` | Bootstrap interrumpido/reanudado y carrera entre snapshot, lazy creation y evento lifecycle más nuevo. |
| `AC-CA-014` | Dos procesos jobs concurrentes, reloj acelerado y restart con ejecución vigente/vencida. |
| `AC-CA-015` | Filas con timestamp empatado, evento tardío, crash antes del commit y repetición del lote. |
| `AC-CA-016` | Dataset conocido para ventanas 30/90 días, elegibilidad, conteos y máximo de asociación. |
| `AC-CA-017` | Fixtures del catálogo válido y revisiones con ciclo, padre ausente, familia inválida y mutación inmutable. |
| `AC-CA-018` | Caídas sucesivas de cada capa de caché, contención de lock y publicación de revisión inválida. |
| `AC-CA-019` | Asociaciones active/inactive/desconocidas y dos ejecuciones de resolución histórica con la segunda no-op. |
| `AC-CA-020` | Arranque independiente de query API y pruebas de Bearer ausente, inválido y válido. |
| `AC-CA-021` | Tabla HTTP de parámetros faltantes/inválidos y límites 1, 10, 100 y 101. |
| `AC-CA-022` | Dataset con empates completos, contexto con resultados parciales y verificación de fallback solo global. |
| `AC-CA-023` | Snapshots JSON sin campos prohibidos y casos empty, unavailable, stale permitido y stale rechazado. |
| `AC-CA-024` | Datos espejo en dos tenants, pruebas cruzadas de API/caché/locks y revisión de planes SQL tenant-first. |
| `AC-CA-025` | Simulación post-commit con respuestas 400, 401, 429, 5xx y red; conteo de intentos y clave estable. |
| `AC-CA-026` | Contract test consumidor que hidrate IDs, filtre cliente inactivo y degrade ante timeout/lista vacía. |
| `AC-CA-027` | Pruebas de health/readiness y captura de métricas, logs y alertas por cada flujo sin secretos ni PII. |
| `AC-CA-028` | Ensayo del orden de rollout y reversión por flags/detención de binarios, manteniendo ingestión web activa. |
| `AC-CA-029` | Reporte reproducible del perfil máximo MVP con p95, duración de proyección y duración de backfill. |
| `AC-CA-030` | Prueba end-to-end de un tenant desde emisión hasta hidratación contextual y repetición con flags apagados. |
