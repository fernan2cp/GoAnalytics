# Contrato de eventos

Las reglas de nombres, versionado, propiedades y claves sensibles estan en `docs/event-conventions.md`.

## Endpoint de ingesta

```http
POST /v1/events
Authorization: Bearer <tracking_jwt>
Content-Type: application/json
```

## Payload

```json
{
  "events": [
    {
      "event_id": "018f9b8e-0000-7000-a000-000000000001",
      "event_name": "page_view",
      "event_version": 1,
      "timestamp": "2026-05-05T12:00:00.000Z",
      "anonymous_id": "anon_abc",
      "session_id": "sess_xyz",
      "user_id": null,
      "origin": "https://cliente.com",
      "url": "https://cliente.com/productos/123",
      "path": "/productos/123",
      "referrer": "https://google.com",
      "properties": {},
      "context": {}
    }
  ]
}
```

## Respuesta

La respuesta esperada para eventos aceptados para procesamiento es `204 No Content`.

## Stream interno

Los eventos aceptados se publican en `goanalytics:events:raw` enriquecidos con claims del JWT, `received_at`, `user_agent` e `ip_hash`.
