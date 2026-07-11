# Paquete SDD - Captura De Eventos De Items

## Objetivo

Este paquete convierte el diseño objetivo de `../item_event_capture_design.md` en una guia implementable para Go Analytics.

El alcance es documentar como capturar, normalizar, deduplicar y persistir eventos relacionados a items para que un proceso externo pueda calcular scoring. Go Analytics no calcula scoring y no escribe en bases tenant.

## Orden De Lectura

1. `01_contexto_y_alcance.md`: limites de responsabilidad y dependencias externas.
2. `02_contrato_eventos_items.md`: eventos, campos esperados y reglas de impresion.
3. `03_modelo_datos.md`: tablas, relaciones, claves e indices.
4. `04_flujo_procesamiento.md`: recorrido desde ingesta hasta persistencia.
5. `05_plan_implementacion.md`: fases recomendadas de implementacion.
6. `06_plan_pruebas.md`: pruebas y escenarios de aceptacion.

## Principios

- Usar `item` como nomenclatura principal.
- Permitir `product` solo como valor valido de `item_type`, junto con `service` y `subscription_plan`.
- Mantener `analytics_events.event_id` como ID enviado por el SDK.
- Usar `analytics_event_items.analytics_event_id` como FK interna hacia `analytics_events.id`.
- Tratar `analytics_event_items` como tabla 1:N denormalizada para scoring.
- Documentar `analytics_event_orders` como recomendable y no bloqueante.

## Resultado Esperado

Al finalizar la implementacion derivada de este SDD, Go Analytics debe persistir eventos base y detalle normalizado de items con suficiente contexto para calcular impresiones, interacciones, compras, importes, costos opcionales y rewards fuera del microservicio.
