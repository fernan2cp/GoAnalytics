# Contratos De Eventos Y Resolvers

## Endpoint Público Existente

`POST /v1/events` no cambia. Continúa usando tracking JWT y los requisitos actuales de sesión, navegación y origen. Ingest asigna internamente `event_source=web`.

`subject_impression`, `subject_viewed` y `subject_selected` pueden enviarse por este endpoint cuando representan interacción web. Deben usar `properties.subjects`.

## Endpoint Interno De Eventos

```http
POST /v1/internal/events
Authorization: Bearer <SUBJECT_EVENTS_API_TOKEN>
Content-Type: application/json
```

Acepta objeto único o `{ "events": [...] }`, igual que el mapper público. Aplica el mismo máximo de bytes y máximo 50 eventos por batch.

Campos por evento:

| Campo | Regla |
| --- | --- |
| `event_id` | Obligatorio y estable entre reintentos. |
| `idempotency_key` | Obligatorio para associated y lifecycle. |
| `event_name` | Uno de los eventos de sujetos registrados. |
| `event_version` | `1` en el MVP. |
| `timestamp` | RFC3339 UTC. |
| `site_code` | Obligatorio; determina tenant y site. |
| `properties` | Objeto JSON con `subjects` y datos del evento. |
| `context` | Objeto JSON; puede ser vacío. |

No acepta `tenant_id` como fuente de autoridad. Ignora o rechaza explícitamente cualquier `event_source` provisto: ingest asigna `server`.

Autenticación:

- Comparación constante del Bearer configurado.
- Token ausente o inválido: `401`.
- Capacidad deshabilitada: `404` para no exponerla.
- Token vacío con capacidad habilitada: falla de bootstrap.
- El token nunca aparece en logs.

Respuesta aceptada:

```json
{
  "accepted": 1,
  "rejected": 0,
  "event_ids": ["018f9b8e-0000-7000-a000-000000000301"]
}
```

## Forma Canónica De Sujetos

```json
{
  "subjects": [
    {
      "subject_type": "customer",
      "subject_id": "customer_123",
      "relation": "attendee"
    }
  ],
  "operation": {
    "operation_type": "appointment",
    "operation_id": "845"
  }
}
```

Reglas:

- `subjects` contiene entre 1 y 100 elementos.
- `subject_type`, `subject_id` y `relation` tienen máximo 255 caracteres.
- `subject_type=customer` es el único habilitado.
- `subject_id` se guarda como `external_subject_id` sin inferir formato.
- `relation` se normaliza a cadena vacía cuando se omite.
- Duplicados exactos conservan la primera posición.
- `operation` es opcional; si existe, ambos campos son obligatorios.

## Eventos De Interacción Y Asociación

```text
subject_impression
subject_viewed
subject_selected
subject_associated
```

`subject_associated` solo se emite después del commit de negocio y requiere `idempotency_key`. Su operación debe identificar el hecho confirmado cuando el proyecto principal disponga de esa identidad.

## Eventos Lifecycle

```text
subject_deactivated
subject_reactivated
subject_merged
```

Payload común:

```json
{
  "event_id": "018f9b8e-0000-7000-a000-000000000401",
  "idempotency_key": "customer:123:lifecycle:2026-08-06T12:00:00.123456Z:mutation-77",
  "event_name": "subject_deactivated",
  "event_version": 1,
  "timestamp": "2026-08-06T12:00:01Z",
  "site_code": "pub_site_abc123",
  "properties": {
    "subjects": [
      {
        "subject_type": "customer",
        "subject_id": "customer_123",
        "relation": "lifecycle"
      }
    ],
    "lifecycle": {
      "source_updated_at": "2026-08-06T12:00:00.123456Z",
      "source_mutation_id": "mutation-77",
      "effective_at": "2026-08-06T12:00:00Z"
    }
  },
  "context": {}
}
```

`subject_merged` agrega:

```json
{
  "lifecycle": {
    "source_updated_at": "2026-08-06T12:00:00.123456Z",
    "source_mutation_id": "mutation-78",
    "effective_at": "2026-08-06T12:00:00Z",
    "merged_into_subject_id": "customer_456"
  }
}
```

La precedencia compara primero `source_updated_at` y luego `source_mutation_id` lexicográficamente. Ambos son obligatorios. `effective_at` no participa del orden.

## Extensión Del Resolver De Sites

Request existente extendido:

```json
{
  "site_code": "pub_site_abc123",
  "origin": "https://cliente.com",
  "env": "production",
  "token_version": 1,
  "source": "web"
}
```

Para server:

```json
{
  "site_code": "pub_site_abc123",
  "env": "production",
  "source": "server"
}
```

Reglas:

- Ausencia de `source` equivale a `web`.
- `origin` y `token_version` mantienen reglas actuales para `web`.
- Para `server`, `origin` es opcional y el resolver autentica la llamada interna existente.
- La respuesta agrega `timezone` sin eliminar campos.

```json
{
  "site_code": "pub_site_abc123",
  "tenant_id": "tenant_123",
  "site_id": "site_456",
  "status": "active",
  "tracking_enabled": true,
  "allowed_domains": ["cliente.com"],
  "token_version": 1,
  "sample_rate": 1,
  "schema_version": 2,
  "timezone": "America/Argentina/Buenos_Aires"
}
```

Una zona inválida o faltante permite persistir el evento crudo, pero la normalización queda `timezone_unresolved` y no participa en proyecciones.

## Snapshot De Sujetos

```http
GET /internal/analytics/subjects/snapshot
  ?subject_type=customer
  &limit=500
  &cursor=<opaque>
  &source_revision=<revision>
Authorization: Bearer <SUBJECT_SOURCE_TOKEN>
```

Primera página: `cursor` y `source_revision` ausentes. El servidor fija una revisión. Páginas siguientes: ambos valores recibidos previamente.

```json
{
  "subject_type": "customer",
  "source_revision": "customers:2026-08-06T00:00:00Z:42",
  "next_cursor": "opaque_cursor_2",
  "has_more": true,
  "subjects": [
    {
      "tenant_id": "tenant_123",
      "external_subject_id": "customer_123",
      "lifecycle_status": "active",
      "source_updated_at": "2026-08-05T18:00:00.000000Z",
      "source_mutation_id": "customer-123-rev-9",
      "merged_into_external_subject_id": null
    }
  ]
}
```

Errores:

- `400` parámetros inválidos.
- `401` token inválido.
- `409 snapshot_revision_expired` si no puede mantener la revisión.
- `503` fuente temporalmente indisponible.

No devuelve nombre, email, teléfono, documento ni otra información personal.

## Registro De Contextos

```http
GET /internal/analytics/context-registry
Authorization: Bearer <SUBJECT_CONTEXT_REGISTRY_TOKEN>
```

```json
{
  "schema_version": 1,
  "registry_version": 4,
  "registry_revision": "registry-4-8d2f",
  "contexts": [
    {
      "context_key": "appointments.customer_picker",
      "context_version": 1,
      "context_family": "search",
      "parent_context_key": null,
      "parent_context_version": null,
      "status": "active"
    }
  ]
}
```

`registry_revision` es opaca, no vacía y estable para el mismo payload. GoAnalytics calcula `payload_checksum` sobre los bytes recibidos; reutilizar una revisión con checksum distinto invalida la carga.
