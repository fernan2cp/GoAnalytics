# Contexto Y Alcance Generico

## Contexto

`GoAnalytics` ya soporta ingesta multitenant, SDK web, deduplicacion, persistencia de eventos y normalizacion de eventos de items. La siguiente evolucion es permitir que distintos productos midan comportamiento interno de uso con un contrato comun.

Los integradores necesitan responder preguntas como:

- Que features se usan mas.
- Desde que superficies se usan.
- Que formularios generan friccion.
- Donde se abandonan flujos.
- Que acciones indican valor.
- Que items, entidades o resultados son mas seleccionados por contexto.

Estas preguntas no son exclusivas de un dominio. El contrato debe servir para cualquier producto con areas, features, superficies, formularios, busquedas y entidades.

## Responsabilidades De GoAnalytics

GoAnalytics debe:

- Aceptar campos de contexto opcionales en eventos existentes.
- Validar que `properties` y `context` sean JSON seguro.
- Mantener bloqueo de claves sensibles.
- Persistir eventos crudos completos para auditoria.
- Normalizar detalles cuando exista un modelo generico justificado.
- Exponer contratos documentados para agregados opcionales.
- Mantener compatibilidad hacia atras.

## Responsabilidades Del Integrador

Cada integrador debe:

- Definir su taxonomia de areas, features y superficies.
- Decidir que eventos emite y en que momentos.
- Evitar datos sensibles o valores libres de usuarios.
- Generar `logical_event_id` o `idempotency_key` cuando tenga identidad funcional estable.
- Consumir agregados o exportar datos segun sus necesidades.

## Limites

GoAnalytics no debe:

- Requerir valores especificos de `app_area`, `feature`, `surface` o `entry_point`.
- Validar reglas de negocio del integrador.
- Depender de tablas tenant del integrador.
- Escribir resultados directamente en bases de datos externas.
- Calcular rankings obligatorios en el path de ingesta.

## Compatibilidad

Todos los campos nuevos deben ser opcionales. Los eventos actuales deben seguir funcionando con el mismo payload:

- `event_name`
- `event_version`
- `timestamp`
- `anonymous_id`
- `session_id`
- `user_id`
- `origin`
- `url`
- `path`
- `properties`
- `context`

Agregar propiedades opcionales es compatible. Cambiar significado o tipo de campos existentes requiere nueva version de evento o documento de migracion.
