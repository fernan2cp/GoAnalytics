# Plan De Pruebas De Contrato

## Objetivo

Validar que las extensiones de system tracking sean seguras, compatibles y reutilizables.

## Pruebas De Compatibilidad

- Eventos existentes sin `context` siguen aceptandose.
- Eventos con `context` vacio siguen aceptandose.
- Eventos con campos nuevos opcionales no rompen persistencia.
- Aliases actuales (`metadata`, `event_type`, `page_url`) siguen funcionando.
- Eventos de items actuales siguen normalizandose igual.

## Pruebas De Seguridad

- Rechazar o sanitizar claves sensibles en `properties`.
- Rechazar o sanitizar claves sensibles en `context`.
- Validar bloqueo en objetos anidados.
- Verificar que eventos de formularios no acepten claves como `password`, `token`, `document`, `card_number`.
- Verificar limites de tamano para payloads con contexto grande.

## Pruebas De Contexto

- Aceptar `app_area`, `feature`, `surface`, `entry_point`, `component_id`, `flow_id`, `entity_type`, `entity_id`.
- Aceptar valores desconocidos sin requerir enums.
- Persistir `context` completo en `analytics_events`.
- Mantener `properties.surface` para items cuando se usa normalizacion actual.

## Pruebas De Eventos De Comportamiento

Cubrir payloads validos para:

- `feature_opened`
- `feature_action_performed`
- `search_performed`
- `search_result_selected`
- `search_empty_result`
- `search_abandoned`
- `form_validation_attempt`
- `form_completed`
- `form_abandoned`
- `rage_click_detected`
- `dead_click_detected`
- `flow_abandoned`

Para cada evento:

- Acepta payload minimo.
- Acepta contexto opcional.
- Persiste `event_name`, `properties` y `context`.
- No requiere campos especificos de un integrador.

## Pruebas De SDK

Si se agregan helpers:

- Cada helper llama internamente a `track` con `event_name` correcto.
- El helper no rompe si faltan campos opcionales.
- El helper permite pasar `logicalEventId` y `idempotencyKey`.
- El helper respeta sanitizacion de formularios.
- El SDK no lanza errores hacia la aplicacion host.

## Pruebas De Agregados Opcionales

Si se implementa top items o agregados contextuales:

- No mezcla tenants ni sites.
- Agrupa por contexto exacto.
- Soporta filtros parciales.
- Respeta `limit`.
- Devuelve `fallback_level` cuando aplica.
- Versiona el algoritmo.
- Excluye eventos incompletos o degradados segun reglas documentadas.

## Pruebas De No Acoplamiento

- No hay enums obligatorios con nombres de un integrador especifico.
- La documentacion base usa ejemplos genericos.
- Los tests no dependen de dominios como ERP, CRM, POS o CMS para pasar.
- Un evento con `app_area=mobile_app` y `feature=onboarding` es valido.

## Criterios De Aceptacion

- La suite existente de ingesta y worker sigue pasando.
- Los nuevos contratos estan cubiertos por tests unitarios.
- Las pruebas de seguridad cubren `properties` y `context`.
- La documentacion publica describe compatibilidad y campos opcionales.
- Cualquier endpoint de consulta nuevo usa credencial de lectura adecuada.
