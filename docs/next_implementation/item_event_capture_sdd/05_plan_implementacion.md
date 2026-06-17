# Plan De Implementacion

## Fase 1 - Contratos Y Tipos Internos

- Definir estructuras internas para item normalizado y cabecera de orden.
- Documentar ejemplos de payload para cada evento.
- Agregar helpers de extraccion desde `properties`, `metadata` e `items`.
- Mantener compatibilidad con eventos sin items.

## Fase 2 - Migraciones

- Crear migracion para `analytics_event_items`.
- Agregar indices minimos y GIN para `category_ids`.
- Evaluar si `analytics_event_orders` entra en la misma fase o queda para la siguiente.
- Crear migraciones `down` simetricas.

## Fase 3 - Worker Y Persistencia

- Extender el caso de uso de procesamiento para construir detalles normalizados.
- Evolucionar el puerto de repositorio para guardar evento base y detalles.
- Actualizar el adaptador PostgreSQL para usar una transaccion logica.
- Mantener `ON CONFLICT` idempotente para el evento base.

## Fase 4 - Deduplicacion Especifica

- Agregar reglas para `item_impression` y `purchase_completed`.
- Permitir claves semanticas construidas con datos de item.
- Marcar eventos incompletos en `metadata` cuando falten datos criticos.
- Verificar que compras duplicadas no generen doble detalle.

## Fase 5 - Documentacion Y Contrato Publico

- Actualizar `docs/event-conventions.md` con eventos de items.
- Actualizar `docs/event-contract.md` con ejemplos de payload.
- Documentar que `item_type` puede ser `product`, `service` o `subscription_plan`.
- Explicar que Go Analytics no calcula scoring.

## Riesgos

- Perder detalle de items si la persistencia del evento base no devuelve la PK interna.
- Duplicar compras si faltan `order_id` u `order_line_id`.
- Emitir impresiones falsas si el frontend no respeta la regla de visibilidad.
- Sobrecargar `analytics_events.properties` si no se normalizan datos de item.

## Dependencias

- El frontend o integrador debe enviar campos de visibilidad, variantes e importes.
- DragonFullAV/Celery debe consumir las tablas analiticas, no pedir scoring al microservicio.
- La decision de incluir `analytics_event_orders` en v1 debe tomarse antes de crear migraciones.

## Resultado De La Implementacion

La implementacion queda completa cuando un evento relacionado a items produce:

- una fila auditable en `analytics_events`;
- cero, una o muchas filas en `analytics_event_items`;
- cabecera opcional en `analytics_event_orders` si el evento representa checkout u orden y esa tabla fue incluida.
