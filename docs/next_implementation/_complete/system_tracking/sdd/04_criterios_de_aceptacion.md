# Criterios De Aceptacion

- `AC-ST-001`: Eventos existentes sin `context`, con `context` vacio y con alias actuales siguen siendo aceptados. Requisitos: `REQ-ST-002`. Tareas: `TASK-ST-0101`, `TASK-ST-0601`.
- `AC-ST-002`: La documentacion publica describe `context` generico opcional, campos recomendados y separacion con `properties`. Requisitos: `REQ-ST-001`, `REQ-ST-003`, `REQ-ST-004`, `REQ-ST-026`. Tareas: `TASK-ST-0101`.
- `AC-ST-003`: La documentacion publica incluye payloads validos para eventos de feature, busqueda, seleccion contextual, formularios y frustracion. Requisitos: `REQ-ST-005`, `REQ-ST-006`, `REQ-ST-007`, `REQ-ST-008`, `REQ-ST-009`, `REQ-ST-026`. Tareas: `TASK-ST-0102`.
- `AC-ST-004`: Los eventos de formularios no aceptan ni documentan valores ingresados por usuarios; solo campos tecnicos, conteos y codigos de error. Requisitos: `REQ-ST-007`, `REQ-ST-012`, `REQ-ST-016`. Tareas: `TASK-ST-0102`, `TASK-ST-0303`.
- `AC-ST-005`: Claves sensibles en `properties` y `context`, incluyendo objetos anidados, producen rechazo auditable en worker. Requisitos: `REQ-ST-010`, `REQ-ST-012`. Tareas: `TASK-ST-0201`, `TASK-ST-0601`.
- `AC-ST-006`: Campos desconocidos de `context` se aceptan cuando son JSON seguro y cumplen limites documentados. Requisitos: `REQ-ST-011`, `REQ-ST-013`. Tareas: `TASK-ST-0201`, `TASK-ST-0202`.
- `AC-ST-007`: Cada helper SDK nuevo delega en `track` y genera `event_name`, `properties` y `context` esperados. Requisitos: `REQ-ST-014`, `REQ-ST-015`, `REQ-ST-017`. Tareas: `TASK-ST-0301`, `TASK-ST-0302`.
- `AC-ST-008`: El SDK no lanza errores hacia la aplicacion host ante payloads opcionales faltantes o formularios sanitizados. Requisitos: `REQ-ST-016`, `REQ-ST-028`. Tareas: `TASK-ST-0302`, `TASK-ST-0303`.
- `AC-ST-009`: `analytics_events` persiste `properties` y `context` completos para eventos genericos validos, y los eventos sin `context` quedan con objeto vacio. Requisitos: `REQ-ST-018`, `REQ-ST-028`. Tareas: `TASK-ST-0401`.
- `AC-ST-010`: La decision sobre `item_selected_for_context` y agregados queda implementada o documentada como fase posterior sin ambiguedad. Requisitos: `REQ-ST-009`, `REQ-ST-020`, `REQ-ST-021`, `REQ-ST-022`. Tareas: `TASK-ST-0402`, `TASK-ST-0501`.
- `AC-ST-011`: La documentacion base usa ejemplos genericos y no depende de ERP, CRM, POS, CMS ni integradores especificos. Requisitos: `REQ-ST-026`, `REQ-ST-027`. Tareas: `TASK-ST-0101`, `TASK-ST-0102`, `TASK-ST-0601`.
- `AC-ST-012`: La suite existente y los tests nuevos de ingesta, worker y SDK pasan. Requisitos: `REQ-ST-028`. Tareas: `TASK-ST-0201`, `TASK-ST-0202`, `TASK-ST-0301`, `TASK-ST-0302`, `TASK-ST-0303`, `TASK-ST-0401`, `TASK-ST-0601`.
- `AC-ST-013`: Si se implementan agregados, corren fuera de ingesta, separan tenant/site, versionan algoritmo y usan credencial de lectura. Requisitos: `REQ-ST-022`, `REQ-ST-023`, `REQ-ST-024`, `REQ-ST-025`. Tareas: `TASK-ST-0501`.
- `AC-ST-014`: Existe una referencia de contratos para integradores que permite emitir eventos contra el microservicio sin consultar el SDD interno. Requisitos: `REQ-ST-026`, `REQ-ST-029`. Tareas: `TASK-ST-0601`.
