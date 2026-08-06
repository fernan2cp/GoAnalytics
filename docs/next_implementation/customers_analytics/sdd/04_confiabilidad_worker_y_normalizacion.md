# Confiabilidad Del Worker Y Normalización

## Flujo Objetivo

```text
HTTP público o interno
        ↓
Redis Stream con event_source
        ↓
lectura nueva o XAUTOCLAIM
        ↓
mapeo individual
        ↓
validación de site según source
        ↓
deduplicación y normalización
        ↓
transacción por evento
        ↓
resultado durable
        ↓
XACK
```

## Recuperación De Pendientes

Configuración inicial:

```env
WORKER_PENDING_CLAIM_ENABLED=true
WORKER_PENDING_MIN_IDLE_MS=60000
WORKER_PENDING_CLAIM_INTERVAL_MS=30000
WORKER_PENDING_CLAIM_BATCH_SIZE=100
WORKER_MAX_DELIVERIES=5
```

Al iniciar y después de cada intervalo, el consumer ejecuta `XAUTOCLAIM` desde `0-0` hasta agotar el lote disponible. Los mensajes reclamados se procesan antes de nuevas lecturas para evitar starvation de pendientes.

Si la entrega alcanza `WORKER_MAX_DELIVERIES`:

1. Persistir rechazo `stream_retry_exhausted` con payload seguro y delivery count.
2. Emitir log y métrica.
3. Ejecutar `XACK` solamente si el rechazo fue durable.
4. No ejecutar `XDEL`; el trimming del stream conserva su política actual.

## Aislamiento Por Mensaje

`mapMessages` deja de fallar como operación indivisible. Devuelve elementos válidos y errores asociados a IDs concretos.

Un mensaje sin `payload` o con JSON inválido:

- se registra como `invalid_stream_payload`;
- no bloquea otros mensajes;
- se confirma después del rechazo durable;
- permanece pendiente si PostgreSQL no está disponible.

## Persistencia Idempotente

El puerto de eventos devuelve un resultado por entrada:

```text
inserted
duplicate_exact
duplicate_logical
duplicate_idempotency
conflict
```

El adaptador usa `ON CONFLICT DO NOTHING RETURNING id`. Cuando no obtiene fila, consulta el registro conflictivo por prioridad: `event_id`, `logical_event_id`, `idempotency_key`.

- Duplicado exacto o funcional: resultado estable, sin reemplazar detalles existentes.
- Misma clave con payload funcionalmente contradictorio: `conflict`, rechazo auditable.
- Error PostgreSQL no clasificado: aborta el evento y no confirma Redis.

Los eventos del lote no comparten una transacción global. Cada evento conserva atomicidad entre fila cruda, sujeto, detalles de sujeto, items y orden. Esto impide que un duplicado revierta eventos no relacionados.

## Perfil De Validación

`RawEvent` y `ValidatedEvent` agregan `EventSource`.

Perfil `web`:

- reglas actuales completas;
- JWT, identidad anónima o sesión, navegación, origen y dominio.

Perfil `server`:

- `event_id`, `event_name`, versión, tiempo, site y token interno ya validado;
- no exige navegación o actor;
- resuelve site con `source=server`;
- conserva payload safety, límites y deduplicación.

## Normalización 1:N

El caso de uso reconoce solo nombres y versión 1 registrados. Extrae `properties.subjects`, deduplica elementos y construye detalles sin conocer SQL.

Por cada elemento:

1. Validar forma, tamaños y `subject_type`.
2. Resolver o crear `analytics_subjects` mediante upsert tenant-global.
3. Determinar estado inicial según source y evento.
4. Resolver zona IANA y `local_date`.
5. Copiar operación y contexto opaco.
6. Aplicar idempotencia tenant-global.
7. Persistir `normalization_status` y `normalization_reason`.

La ausencia o error de un sujeto no elimina otros sujetos válidos del mismo evento.

## Contexto Antes De Fase 4

Durante Fases 1 a 3, la normalización conserva `context_key`, `context_version` y familia declarada, pero usa:

```text
context_resolution_status = registry_disabled
context_definition_id = NULL
context_registry_revision_id = NULL
```

Esto permite la resolución inicial de Fase 4 sin releer o modificar `analytics_events`.

## Deducción De Estado Inicial

- `subject_associated` con `event_source=server`: crea `active` si no existe.
- Cualquier interacción web: crea `unknown`.
- Lifecycle: crea `unknown` y luego aplica la transición si su orden es válido.
- Un sujeto existente nunca retrocede de estado por la regla de creación.

## Fallos

- Zona faltante: detalle `timezone_unresolved`; evento crudo confirmado.
- Tipo no permitido: `unsupported_subject_type`; evento crudo confirmado.
- Sujeto incompleto: `invalid_subject`; evento crudo confirmado.
- Contradicción idempotente: `idempotency_conflict`; no cuenta.
- Infraestructura no disponible: no `XACK`, reintento posterior.

## Fronteras Hexagonales

Application define puertos para persistencia de eventos, sujetos, contextos, métricas y resolución. Domain contiene validaciones y transiciones. Redis, PostgreSQL, HTTP, Prometheus y variables de entorno permanecen en adapters/bootstrap.
