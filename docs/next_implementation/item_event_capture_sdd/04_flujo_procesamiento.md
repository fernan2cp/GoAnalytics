# Flujo De Procesamiento

## Ingesta HTTP

La API de ingesta debe seguir aceptando el contrato actual de eventos. Los datos de items pueden llegar dentro de `properties`, `metadata` o `items`.

La ingesta no debe validar reglas profundas de scoring. Solo debe:

- decodificar el payload;
- preservar campos compatibles;
- enriquecer con datos del token y request;
- publicar el evento crudo en Redis Stream.

## Stream Interno

El stream debe conservar los campos existentes:

- `event_id`
- `logical_event_id`
- `idempotency_key`
- `tab_id`
- `sequence`
- `previous_logical_event_id`
- datos de navegacion y usuario
- `properties`
- `context`

No se requiere un nuevo stream para items. El worker debe extraerlos desde el evento crudo.

## Worker

El worker debe:

1. Validar campos base del evento.
2. Validar site y tenant.
3. Aplicar deduplicacion exacta y fuerte.
4. Extraer items desde `properties`, `metadata` o `items`.
5. Normalizar cada item hacia una estructura interna.
6. Aplicar deduplicacion especifica por tipo de evento cuando corresponda.
7. Persistir `analytics_events`.
8. Persistir `analytics_event_items` y opcionalmente `analytics_event_orders`.
9. Marcar claves de deduplicacion despues de persistir correctamente.

## Normalizacion De Items

La normalizacion debe producir cero, una o muchas filas por evento.

Reglas:

- evento sin item: solo persiste `analytics_events`;
- evento con un item: persiste una fila en `analytics_event_items`;
- evento multi-item: persiste una fila por item;
- evento de compra incompleto: persiste con marca de incompletitud en `metadata`;
- importes y costos se guardan como snapshots, sin recalculo.

## Deduplicacion

Reglas objetivo:

- general por `tenant_id + site_id + idempotency_key`;
- logica por `tenant_id + site_id + logical_event_id`;
- impresion por `tenant_id + site_id + session_id + surface + list_instance_id + item_id + variant_id`;
- compra por `tenant_id + site_id + order_id + order_line_id`.

Para `purchase_completed`, la deduplicacion debe ser estricta. Si faltan datos de orden o linea, no debe bloquear auditoria, pero si debe impedir que el registro se considere confiable para scoring hasta que se corrija la integracion.

## Persistencia

La interfaz de repositorio debe evolucionar para persistir el evento base y sus detalles normalizados en una misma operacion logica.

El adaptador PostgreSQL debe encapsular:

- insercion de `analytics_events`;
- obtencion de `analytics_events.id`;
- insercion batch de `analytics_event_items`;
- insercion opcional de `analytics_event_orders`;
- manejo idempotente de conflictos.

`domain` y `application` no deben importar SQL, pgx ni detalles de migracion.

## Errores

Si falla la persistencia del evento base, no se debe persistir detalle de item.

Si falla la persistencia de detalle para un evento que contiene items validos, la operacion debe fallar completa para evitar perdida silenciosa de datos de scoring.
