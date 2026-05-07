# Convenciones de eventos

Este documento define las reglas base para nombrar, versionar y transportar eventos en Go Analytics. Aplica al SDK, a la API de ingesta, al stream interno y al worker.

## Identidad

- `event_id` debe ser unico por evento y debe generarse antes de enviar el batch.
- `logical_event_id` identifica el evento logico cuando el SDK puede derivarlo
  de forma estable. Para `page_view` debe basarse en una identidad de
  navegacion, no solo en `path`.
- `idempotency_key` debe usarse para eventos de negocio cuando exista una clave
  funcional confiable del dominio.
- `tab_id` identifica una pestana dentro de la sesion y `sequence` ordena los
  eventos emitidos desde esa pestana.
- `event_name` debe usar `snake_case`, por ejemplo `page_view`, `product_viewed` o `checkout_started`.
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
- `path` debe representar la ruta sin dominio, por ejemplo `/productos/123`.
- `referrer` puede ser vacio cuando no existe o no se permite capturarlo.

## Propiedades

- `properties` contiene datos especificos del evento.
- `context` contiene datos de entorno, dispositivo, pagina, SDK o runtime.
- Ambos campos deben serializarse como objeto JSON; si no hay datos, deben enviarse como `{}`.
- Las propiedades deben usar claves `snake_case`.
- Los valores deben ser JSON simples: string, number, boolean, null, array u objeto.
- No se deben enviar secretos, credenciales, tokens, cookies, documentos personales ni tarjetas.
- En eventos de formularios no se deben enviar valores ingresados por el usuario.
  Solo se permiten nombres tecnicos de campos, codigos de error y metricas
  agregadas.

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
- `item_viewed`: vista de item.
- `search_performed`: busqueda ejecutada.
- `checkout_started`: inicio de checkout.
- `purchase_completed`: compra completada.
- `signup_started`: inicio de registro.
- `signup_completed`: registro completado.
- `form_validation_attempt`: intento de validacion o envio de formulario.
- `form_completed`: formulario completado.
- `form_abandoned`: formulario abandonado.
- `form_step_advanced`: avance de paso.
- `form_step_viewed`: visualizacion de paso.

## Compatibilidad

- No cambiar el significado de un `event_name` existente sin subir `event_version`.
- Agregar propiedades opcionales es compatible.
- Renombrar o cambiar el tipo de una propiedad principal requiere nueva version.
- El SDK debe evitar romper la aplicacion host si falla el envio de eventos.
