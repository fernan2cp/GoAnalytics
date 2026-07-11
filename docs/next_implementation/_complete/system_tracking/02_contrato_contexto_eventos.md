# Contrato De Contexto De Eventos

## Objetivo

Definir una convencion flexible para describir donde y por que ocurre un evento, sin acoplar el microservicio a un dominio especifico.

El contexto puede enviarse en `context` para analisis transversal. Cuando un campo ya exista en un contrato especifico, por ejemplo `surface` en eventos de items, tambien puede mantenerse en `properties` para normalizacion especializada.

## Campo `context`

Ejemplo:

```json
{
  "context": {
    "app_area": "operations",
    "feature": "checkout",
    "surface": "drawer",
    "entry_point": "calendar_detail",
    "component_id": "item_search",
    "flow_id": "flow_01J...",
    "entity_type": "appointment",
    "entity_id": "123",
    "runtime": {
      "viewport_width": 1366,
      "viewport_height": 768
    }
  }
}
```

## Campos Recomendados

| Campo | Tipo | Requerido | Descripcion |
|---|---|---:|---|
| `app_area` | string | No | Area de producto o aplicacion. |
| `feature` | string | No | Capacidad funcional donde ocurre el evento. |
| `surface` | string | No | Superficie de interaccion. |
| `entry_point` | string | No | Origen funcional desde donde se abrio la superficie. |
| `component_id` | string | No | Componente logico reutilizable. |
| `flow_id` | string | No | Correlacion de una tarea del usuario. |
| `entity_type` | string | No | Tipo de entidad de negocio no sensible. |
| `entity_id` | string | No | Identificador tecnico no sensible. |
| `runtime` | object | No | Datos tecnicos de entorno no sensibles. |

## Reglas De Formato

- Claves en `snake_case`.
- Valores string de longitud razonable, recomendada hasta 120 caracteres.
- `entity_id` debe ser string si se envia.
- Los valores no deben contener datos personales, secretos ni texto libre sensible.
- El integrador puede agregar campos propios en `context`, siempre que cumplan las reglas de seguridad.

## Semantica Flexible

GoAnalytics no debe imponer un catalogo cerrado de valores. Puede validar tipo y seguridad, pero no debe rechazar un evento porque `app_area`, `feature` o `surface` tengan valores desconocidos.

Los integradores pueden usar valores como:

- `app_area`: `admin`, `public_site`, `backoffice`, `mobile_app`.
- `feature`: `checkout`, `catalog`, `content_editor`, `support_inbox`.
- `surface`: `fullscreen`, `drawer`, `modal`, `embedded_panel`, `grid`.
- `entry_point`: `main_nav`, `search_result`, `detail_page`, `notification`.

Estos ejemplos no son normativos.

## Relacion Con `properties`

`context` describe el entorno. `properties` describe el hecho observado.

Ejemplo para busqueda:

```json
{
  "event_name": "search_performed",
  "properties": {
    "search_id": "search_123",
    "query_length": 4,
    "results_count": 12,
    "filters": { "item_type": "service" }
  },
  "context": {
    "app_area": "backoffice",
    "feature": "sales",
    "surface": "drawer",
    "component_id": "item_search"
  }
}
```

## Deduplicacion

El integrador debe usar `logical_event_id` cuando pueda derivar una identidad estable:

```text
search_performed:{session_id}:{component_id}:{search_id}
form_validation_attempt:{session_id}:{form_id}:{step_id}:{attempt_number}
feature_opened:{session_id}:{feature}:{surface}:{flow_id}
```

Para operaciones de negocio idempotentes, debe usar `idempotency_key`.
