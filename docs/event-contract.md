# Contrato de eventos

Las reglas de nombres, versionado, propiedades y claves sensibles estan en `docs/event-conventions.md`.

## Endpoint de ingesta

```http
POST /v1/events
Authorization: Bearer <tracking_jwt>
Content-Type: application/json
```

## Payload

El microservicio es flexible en la recepción de datos. Permite enviar un array de eventos o un único evento suelto. También soporta los siguientes alias para facilitar la integración desde diversos SDKs:
- `metadata` -> mapea a `properties`
- `event_type` -> mapea a `event_name`
- `page_url` -> mapea a `url`

```json
{
  "events": [
    {
      "event_id": "018f9b8e-0000-7000-a000-000000000001",
      "logical_event_id": "page_view:sess_xyz:tab_abc:nav_123:/productos/123",
      "idempotency_key": null,
      "tab_id": "tab_abc",
      "sequence": 1,
      "previous_logical_event_id": null,
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

Los campos `logical_event_id`, `idempotency_key`, `tab_id`, `sequence` y
`previous_logical_event_id` son opcionales y compatibles hacia atras.
`logical_event_id` identifica un evento logico generado por el SDK.
`idempotency_key` identifica una operacion funcional del dominio cuando existe.
`tab_id` y `sequence` permiten reconstruir recorridos dentro de una pestana.

## Respuesta

La respuesta esperada para una solicitud aceptada para procesamiento es `202 Accepted`. El cuerpo de la respuesta informa sobre el resultado de la ingesta inicial:

```json
{
  "accepted": 1,
  "rejected": 0,
  "event_ids": ["018f9b8e-0000-7000-a000-000000000001"]
}
```

## Stream interno

Los eventos aceptados se publican en `goanalytics:events:raw` enriquecidos con claims del JWT, `received_at`, `user_agent` e `ip_hash`.
