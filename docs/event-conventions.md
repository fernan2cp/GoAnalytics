# Convenciones de eventos

Este documento define las reglas base para nombrar, versionar y transportar eventos en Go Analytics. Aplica al SDK, a la API de ingesta, al stream interno y al worker.

## Identidad

- `event_id` debe ser unico por evento y debe generarse antes de enviar el batch.
- `logical_event_id` identifica el evento logico cuando el SDK puede derivarlo
  de forma estable. Para `page_view` debe basarse en una identidad de
  navegacion, no solo en `path`.
- `idempotency_key` debe usarse para eventos de negocio cuando exista una clave
  funcional confiable del dominio.
- En eventos personalizados emitidos con `track`, el consumidor debe pasar
  `logicalEventId` cuando pueda derivar una identidad funcional estable. Esto
  evita duplicados por doble montaje, React StrictMode, reintentos o doble
  handler. Ejemplos: `item_viewed:{item_id}:{path}`,
  `cart_item_added:{cart_id}:{item_id}` o `checkout_started:{cart_id}`.
- `tab_id` identifica una pestana dentro de la sesion y `sequence` ordena los
  eventos emitidos desde esa pestana.
- `event_name` debe usar `snake_case`, por ejemplo `page_view`, `item_viewed` o `checkout_started`.
- `event_name` debe describir una accion o hecho observable, no una pantalla tecnica ni un metodo interno.
- `event_version` empieza en `1` y aumenta cuando cambia el significado del evento o de sus propiedades principales.
- `anonymous_id` identifica el navegador o dispositivo cuando no hay usuario autenticado.
- `session_id` identifica una sesion de navegacion.
- Cada evento debe incluir al menos `anonymous_id` o `session_id`.

## Tiempo

- `timestamp` debe venir en UTC con formato RFC3339.
- `received_at` lo asigna el servicio de ingesta y no debe enviarlo el SDK.
- El worker debe usar `event_time` para analisis temporal y `received_at` para auditoria de llegada.

## Navegacion

- `origin` debe incluir esquema y host, por ejemplo `https://cliente.com`.
- `url` debe representar la URL completa visible para el usuario cuando aplica.
- `path` debe representar la ruta sin dominio, por ejemplo `/items/123`.
- `referrer` puede ser vacio cuando no existe o no se permite capturarlo.

## Propiedades Y Contexto

- `properties` contiene datos especificos del evento: accion, resultado, item, busqueda, formulario o importes asociados al hecho observado.
- `context` contiene datos de entorno y ubicacion logica reutilizables: area de aplicacion, feature, superficie, punto de entrada, componente, flujo, entidad no sensible, dispositivo, pagina, SDK o runtime.
- Ambos campos deben serializarse como objeto JSON; si no hay datos, deben enviarse como `{}`.
- Las claves deben usar `snake_case`.
- Los valores deben ser JSON simples: string, number, boolean, null, array u objeto.
- Los campos desconocidos son validos cuando son JSON seguro y respetan las reglas de datos sensibles.
- `context` no tiene catalogo cerrado. Las claves recomendadas para system tracking son `app_area`, `feature`, `surface`, `entry_point`, `flow_id`, `component_id`, `entity_type` y `entity_id`.
- `properties` debe transportar el detalle variable del evento. Por ejemplo, en busquedas: `search_id`, `query_length`, `result_count`, `result_type`, `result_id` y `position`.
- No se deben enviar secretos, credenciales, tokens, cookies, documentos personales ni tarjetas.
- En eventos de formularios no se deben enviar valores ingresados por el usuario. Solo se permiten nombres tecnicos de campos, codigos de error y metricas agregadas.
- `search_term` es opt-in: solo debe enviarse cuando el integrador confirma que no contiene datos personales, secretos ni texto libre sensible. Si no es seguro, usar `query_length`, `has_query` y filtros agregados.
- Cada objeto `properties` y `context` debe mantenerse por debajo de 64 KiB serializado y con profundidad maxima 16. Eventos que superen esos limites pueden ser rechazados por el worker como `payload_too_large`.

## Claves sensibles bloqueadas

Estas claves no deben aparecer en `properties` ni `context`, sin importar mayusculas o minusculas:

```text
password
token
access_token
refresh_token
authorization
cookie
secret
private_key
credit_card
card_number
cvv
dni
document
```

## Eventos recomendados

- `page_view`: vista de pagina o cambio de ruta.
- `feature_opened`: apertura de una feature o superficie funcional.
- `feature_action_performed`: accion del usuario dentro de una feature.
- `item_impression`: impresion real de item visible en una superficie.
- `item_viewed`: vista de item.
- `item_image_zoomed`: zoom o ampliacion de imagen de item.
- `cart_item_added`: incorporacion de item al carrito.
- `search_performed`: busqueda ejecutada.
- `search_result_selected`: seleccion de un resultado.
- `search_empty_result`: busqueda sin resultados.
- `search_abandoned`: busqueda abandonada sin seleccion.
- `item_selected_for_context`: seleccion de item asociada a contexto generico.
- `checkout_started`: inicio de checkout.
- `purchase_completed`: compra completada.
- `signup_started`: inicio de registro.
- `signup_completed`: registro completado.
- `form_validation_attempt`: intento de validacion o envio de formulario.
- `form_completed`: formulario completado.
- `form_abandoned`: formulario abandonado.
- `form_step_advanced`: avance de paso.
- `form_step_viewed`: visualizacion de paso.
- `flow_abandoned`: abandono explicito o inferido de una tarea.
- `rage_click_detected`: secuencia de clicks repetidos sobre una superficie.
- `dead_click_detected`: click sin efecto observable esperado.

## Eventos de items para scoring

Go Analytics captura datos de items para que procesos externos calculen
scoring, ranking o agregados analiticos. El microservicio no calcula scoring,
no ordena catalogos y no escribe resultados en bases tenant.

Los eventos relacionados a items deben usar `item_id` como identificador
canonico y, cuando corresponda, `variant_id` y `sku` para describir la variante
concreta. `item_type` define el tipo funcional del item y puede tomar valores
como `product`, `service` o `subscription_plan`.

`item_impression` debe emitirse solo cuando exista visibilidad real del item.
Como referencia minima, la aplicacion integradora debe medir `visible_ratio`,
`visible_time_ms`, `surface` y `list_instance_id`, evitando emitir impresiones
por renderizados tecnicos, placeholders o elementos fuera del viewport.

Para eventos con multiples items, como `checkout_started` o
`purchase_completed`, las lineas deben enviarse en `properties.items`. Cada
linea puede incluir `order_line_id`, `quantity`, `unit_price`, `currency`,
importes y costos opcionales. La cabecera del evento puede incluir `cart_id`,
`checkout_id`, `order_id`, moneda e importes totales.

## Compatibilidad

- No cambiar el significado de un `event_name` existente sin subir `event_version`.
- Agregar propiedades opcionales es compatible.
- Renombrar o cambiar el tipo de una propiedad principal requiere nueva version.
- El SDK debe evitar romper la aplicacion host si falla el envio de eventos.
