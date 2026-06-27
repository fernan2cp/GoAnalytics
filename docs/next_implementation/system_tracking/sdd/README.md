# SDD System Tracking Generico

## Proposito

Este paquete convierte las guias de `docs/next_implementation/system_tracking/*` en un plan SDD implementable para extender `GoAnalytics` con tracking generico de uso, busqueda, seleccion, friccion y abandono.

El objetivo es que la implementacion mantenga el contrato publico estable, acepte contexto funcional opcional, refuerce seguridad de payloads y documente agregados contextuales opcionales sin convertir la ingesta en un motor de recomendacion online.

## Fuente

- `docs/next_implementation/system_tracking/README.md`
- `docs/next_implementation/system_tracking/01_contexto_y_alcance_generico.md`
- `docs/next_implementation/system_tracking/02_contrato_contexto_eventos.md`
- `docs/next_implementation/system_tracking/03_contrato_eventos_comportamiento.md`
- `docs/next_implementation/system_tracking/04_contrato_agregados_y_consulta.md`
- `docs/next_implementation/system_tracking/05_cambios_recomendados_goanalytics.md`
- `docs/next_implementation/system_tracking/06_plan_pruebas_contrato.md`

## Mapa De Documentos

- `00_baseline_fase_0.md`: estado actual confirmado en el repo, decisiones cerradas, gaps y riesgos.
- `01_requisitos.md`: requisitos trazables de contrato, seguridad, SDK, persistencia, agregados y documentacion.
- `02_diseno_tecnico.md`: diseno de la evolucion por componentes y flujos.
- `03_plan_de_tareas.md`: fases de implementacion con tareas, requisitos y criterios asociados.
- `04_criterios_de_aceptacion.md`: criterios verificables de salida.
- `05_validacion_objetivo.md`: comandos e inspecciones esperadas.

## Regla De Trazabilidad

Cada requisito usa `REQ-ST-*`, cada tarea usa `TASK-ST-*` y cada criterio usa `AC-ST-*`.

Cada `TASK-ST-*` debe referenciar al menos un `REQ-ST-*` y un `AC-ST-*`. Cada `REQ-ST-*` debe aparecer en al menos una tarea y un criterio de aceptacion. Cada `AC-ST-*` debe tener una validacion objetiva en `05_validacion_objetivo.md`.

## Alcance

- Aceptar `context` generico opcional en eventos actuales y nuevos.
- Mantener compatibilidad con `metadata -> properties`, `event_type -> event_name` y `page_url -> url`.
- Agregar o documentar helpers SDK para eventos genericos de feature, busqueda, seleccion, formularios y frustracion.
- Validar seguridad de `properties` y `context`, incluyendo claves sensibles anidadas.
- Persistir el evento crudo completo en `analytics_events`.
- Normalizar solo detalles genericos cuando exista modelo estable y no rompa compatibilidad.
- Documentar agregados contextuales opcionales fuera del path critico de ingesta.
- Actualizar documentacion publica del contrato.
- Crear una referencia de contratos para proyectos integradores en `docs/integration/system-tracking-contracts.md`.

## No Alcance

- No crear taxonomias obligatorias de `app_area`, `feature`, `surface` ni `entry_point`.
- No introducir enums con valores de un integrador especifico.
- No escribir en bases de datos externas.
- No calcular rankings en el path de ingesta.
- No reemplazar los contratos de items existentes.
- No exponer agregados con tracking token publico.
