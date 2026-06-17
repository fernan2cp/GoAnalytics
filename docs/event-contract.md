# Contrato de eventos

Las reglas de nombres, versionado, propiedades y claves sensibles estan en `docs/event-conventions.md`.

## Endpoint de ingesta

```http
POST /v1/events
Authorization: Bearer <tracking_jwt>
Content-Type: application/json
```

## Payload

El microservicio es flexible en la recepcion de datos. Permite enviar un array
de eventos o un unico evento suelto. Tambien soporta los siguientes alias para
facilitar la integracion desde diversos SDKs:

- `metadata` -> mapea a `properties`
- `event_type` -> mapea a `event_name`
- `page_url` -> mapea a `url`

```json
{
  "events": [
    {
      "event_id": "018f9b8e-0000-7000-a000-000000000001",
      "logical_event_id": "page_view:sess_xyz:tab_abc:nav_123:/items/123",
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
      "url": "https://cliente.com/items/123",
      "path": "/items/123",
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

## Eventos De Items

Go Analytics captura eventos relacionados a items para analisis y scoring
externo. El microservicio no calcula scoring, no genera rankings y no escribe
resultados en bases tenant.

Eventos soportados para scoring de items:

- `item_impression`
- `item_viewed`
- `item_image_zoomed`
- `cart_item_added`
- `checkout_started`
- `purchase_completed`

Campos principales para eventos relacionados a items:

- `item_id`: identificador canonico del item.
- `variant_id`: variante concreta, si aplica.
- `sku`: codigo concreto, si aplica.
- `item_type`: tipo funcional. Valores esperados: `product`, `service` o
  `subscription_plan`.
- `item_class_id`: clase del item, si existe.
- `category_ids`: categorias conocidas al momento del evento.
- `surface`: superficie donde ocurrio el evento.
- `position`: posicion visible dentro de la superficie.
- `page`: pagina o bloque de paginacion.
- `search_term`: busqueda asociada.
- `ranking_run_id`: corrida de ranking que genero la exposicion.
- `ranking_version`: version del ranking aplicado.
- `list_instance_id`: instancia logica de lista o grilla.
- `impression_batch_id`: lote de impresiones emitido junto.
- `visible_ratio`: proporcion visible del item.
- `visible_time_ms`: tiempo visible acumulado en milisegundos.
- `viewport_width` y `viewport_height`: tamano del viewport al medir.
- `rendered_at`: momento en que el item real quedo renderizado.
- `cart_id`, `checkout_id`, `order_id` y `order_line_id`: referencias de flujo
  comercial cuando apliquen.
- `quantity`, `unit_price`, `currency`, `gross_amount`, `net_amount`,
  `discount_amount`, `unit_cost` y `cost_amount`: snapshots economicos cuando
  apliquen.

### Ejemplo De `item_impression`

```json
{
  "event_id": "018f9b8e-0000-7000-a000-000000000101",
  "logical_event_id": "item_impression:sess_xyz:list_1:100",
  "event_name": "item_impression",
  "event_version": 1,
  "timestamp": "2026-05-05T12:01:00.000Z",
  "anonymous_id": "anon_abc",
  "session_id": "sess_xyz",
  "origin": "https://cliente.com",
  "url": "https://cliente.com/catalogo",
  "path": "/catalogo",
  "properties": {
    "item_id": "100",
    "item_type": "product",
    "category_ids": ["cat_1"],
    "surface": "catalog",
    "position": 3,
    "page": 1,
    "list_instance_id": "list_1",
    "impression_batch_id": "batch_1",
    "ranking_run_id": "rank_20260505_01",
    "ranking_version": "relevant_v1",
    "visible_ratio": 75,
    "visible_time_ms": 1500,
    "viewport_width": 1366,
    "viewport_height": 768,
    "rendered_at": "2026-05-05T12:00:59.000Z"
  },
  "context": {}
}
```

`item_impression` debe representar visibilidad real, no solo renderizado
tecnico. La aplicacion integradora debe evitar emitirlo por placeholders,
pre-carga de datos, elementos fuera del viewport o pestanas en segundo plano.

### Ejemplo De `purchase_completed`

```json
{
  "event_id": "018f9b8e-0000-7000-a000-000000000201",
  "idempotency_key": "purchase:ord_1",
  "event_name": "purchase_completed",
  "event_version": 1,
  "timestamp": "2026-05-05T12:10:00.000Z",
  "anonymous_id": "anon_abc",
  "session_id": "sess_xyz",
  "user_id": "user_1",
  "origin": "https://cliente.com",
  "url": "https://cliente.com/checkout/gracias",
  "path": "/checkout/gracias",
  "properties": {
    "cart_id": "cart_1",
    "checkout_id": "chk_1",
    "order_id": "ord_1",
    "currency": "ARS",
    "subtotal_amount": 10000,
    "discount_amount": 1000,
    "shipping_amount": 500,
    "tax_amount": 0,
    "gross_amount": 10500,
    "net_amount": 9500,
    "items": [
      {
        "item_id": "100",
        "variant_id": "100-red-m",
        "sku": "SKU-100-RED-M",
        "item_type": "product",
        "category_ids": ["cat_1"],
        "order_line_id": "line_1",
        "quantity": 2,
        "unit_price": 5000,
        "currency": "ARS",
        "gross_amount": 10000,
        "net_amount": 9000,
        "discount_amount": 1000
      }
    ]
  },
  "context": {}
}
```

Para deduplicacion estricta de compras, cada linea debe incluir `order_id` en
la cabecera del evento y `order_line_id` en la linea. Si esos datos faltan, el
evento puede conservarse para auditoria, pero no debe considerarse completo
para scoring confiable.

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
