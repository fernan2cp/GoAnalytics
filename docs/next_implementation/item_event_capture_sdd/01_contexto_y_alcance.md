# Contexto Y Alcance

## Contexto

Go Analytics ya recibe eventos, valida metadata de site, deduplica y persiste eventos validos en `analytics_events`. El nuevo objetivo es conservar detalle estructurado de items para que un proceso externo calcule scoring.

El documento base es `../item_event_capture_design.md`. Este SDD lo traduce en decisiones operativas para implementar sin cambiar la responsabilidad del microservicio.

## Responsabilidad De Go Analytics

Go Analytics debe:

- aceptar eventos relacionados a items desde el contrato publico vigente;
- preservar el evento completo en `analytics_events`;
- extraer datos de items desde `properties`, `metadata` o `items`;
- normalizar esos datos en `analytics_event_items`;
- aplicar deduplicacion general y deduplicacion especifica por tipo de evento;
- dejar auditoria suficiente para reprocesos y revisiones.

## Fuera De Alcance

Go Analytics no debe:

- consultar stock;
- decidir visibilidad, vigencia o elegibilidad comercial;
- calcular scores o posiciones finales;
- ordenar catalogos;
- escribir en bases tenant;
- aplicar diversidad de categoria o clase.

## Dependencias Externas

DragonFullAV y sus tareas Celery seran responsables de:

- leer datos agregables desde Go Analytics;
- cruzar esos datos con catalogo y reglas tenant;
- calcular scoring y ranking;
- materializar resultados en tablas tenant.

El frontend o aplicacion integradora sera responsable de:

- medir visibilidad real para `item_impression`;
- enviar `item_id`, `variant_id` y `sku` sin inferencias del SDK;
- enviar importes y datos de orden cuando el evento los tenga disponibles.

## Compatibilidad

El contrato debe seguir aceptando eventos existentes. Los campos nuevos son opcionales salvo en eventos donde el SDD indique que son necesarios para scoring confiable.

`item_type` puede usar valores como `product`, `service` o `subscription_plan`; esa es la unica excepcion permitida al uso general de nomenclatura `item`.
