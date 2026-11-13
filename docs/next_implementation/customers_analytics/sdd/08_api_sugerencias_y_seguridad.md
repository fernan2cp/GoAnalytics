# API De Sugerencias Y Seguridad

## Proceso

`services/worker/cmd/query-api` es un binario independiente. Comparte domain, application y adaptadores PostgreSQL del módulo worker, pero no inicializa Redis Stream ni el consumer.

Expone:

```text
GET /health
GET /ready
GET /metrics
GET /v1/internal/subjects/suggestions
```

Readiness requiere conexión PostgreSQL y existencia de migraciones hasta la versión necesaria para el modo habilitado.

## Configuración

```env
SUBJECT_QUERY_API_ENABLED=false
SUBJECT_QUERY_API_TOKEN=change_me
SUBJECT_QUERY_HTTP_HOST=0.0.0.0
SUBJECT_QUERY_HTTP_PORT=8080
SUBJECT_QUERY_READ_TIMEOUT_MS=3000
SUBJECT_QUERY_WRITE_TIMEOUT_MS=3000
SUBJECT_SUGGESTION_DEFAULT_LIMIT=10
SUBJECT_SUGGESTION_MAX_LIMIT=100
SUBJECT_SUGGESTION_CONTEXT_WINDOW_DAYS=30
SUBJECT_SUGGESTION_GLOBAL_WINDOW_DAYS=90
SUBJECT_SUGGESTION_STALE_AFTER_SECONDS=172800
SUBJECT_SUGGESTION_SERVE_STALE=true
```

Habilitar con token vacío impide iniciar el binario.

## Autenticación

- Bearer dedicado y comparación constante.
- `401` para token ausente o inválido.
- No acepta tracking JWT, token de resolver ni token de eventos.
- No registra Authorization ni query completa en logs.
- TLS termina en el proxy o red interna del despliegue.

## Request

```http
GET /v1/internal/subjects/suggestions
  ?tenant_id=tenant_123
  &subject_type=customer
  &context_key=appointments.customer_picker
  &context_version=1
  &limit=10
Authorization: Bearer <SUBJECT_QUERY_API_TOKEN>
```

Validación:

- `tenant_id`: obligatorio, no vacío, máximo 255.
- `subject_type`: obligatorio y exactamente `customer`.
- `context_key`: opcional, máximo 255.
- `context_version`: entero positivo y obligatorio con context key; prohibido sin key.
- `limit`: entero 1–100; default 10.
- Parámetro desconocido: `400 invalid_query` para detectar errores de integración.

## Respuesta

```json
{
  "tenant_id": "tenant_123",
  "subject_type": "customer",
  "context": {
    "context_key": "appointments.customer_picker",
    "context_version": 1,
    "resolution_status": "registered"
  },
  "window": {
    "context_days": 30,
    "global_days": 90
  },
  "computed_at": "2026-08-06T02:00:00Z",
  "projection_version": 1,
  "is_stale": false,
  "results": [
    {
      "external_subject_id": "customer_123",
      "association_count": 14,
      "last_association_at": "2026-08-05T20:15:00Z",
      "source_level": "exact_context"
    }
  ]
}
```

No devuelve `analytics_subject_id`, nombre, email, teléfono, documento, metadata ni evento crudo.

## Consulta Y Fallback

1. Validar que existe una proyección global completada.
2. Determinar frescura desde el último run exitoso.
3. Si se solicitó contexto, resolver definición contra la revisión activa.
4. Consultar ranking exacto de 30 días para sujetos `active`.
5. Completar con ranking global de 90 días para sujetos `active`.
6. Deduplicar por sujeto y limitar.

Sin contexto se omiten pasos 3 y 4. Contexto desconocido o inactivo informa ese estado y usa solo global.

No se mezclan conteos. Cada resultado conserva el conteo de su `source_level`.

Cuando se completa una lista contextual parcial con sujetos globales, la trazabilidad de la respuesta indica `source_level=mixed` y `fallback_applied=true`; cada resultado mantiene `context` o `global`. La frescura usa la más antigua de las proyecciones efectivamente utilizadas. Un contexto inactivo informa `inactive_context` y usa global.

## Estados HTTP

| Estado | Condición |
| --- | --- |
| `200` | Resultado válido, incluso lista vacía o stale permitido. |
| `400` | Query inválida. |
| `401` | Credencial inválida. |
| `404` | Capacidad deshabilitada. |
| `503 projection_unavailable` | Nunca completó la proyección requerida. |
| `503 projection_stale` | Vencida y `serve_stale=false`. |
| `503` | PostgreSQL no disponible. |

## Aislamiento

El handler entrega `tenant_id` al caso de uso. El repositorio exige tenant en todas sus firmas y SQL. Los joins entre agregados y sujetos incluyen tenant, aunque exista FK interna. Las pruebas insertan IDs iguales en tenants distintos y verifican ausencia de fuga.

## Rendimiento

- Query p95 menor o igual a 300 ms para `limit=10` bajo el perfil MVP.
- Timeout de repositorio menor al timeout HTTP.
- No ejecutar scans sobre `analytics_events` en línea.
- No devolver más de 100 resultados.
