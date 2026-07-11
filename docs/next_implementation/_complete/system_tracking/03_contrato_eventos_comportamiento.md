# Contrato De Eventos De Comportamiento

## Objetivo

Definir eventos genericos para medir uso, busqueda, seleccion, friccion, abandono y frustracion en cualquier producto integrador.

Todos los eventos son opcionales y pueden coexistir con eventos especificos del integrador.

## Eventos De Uso

### `feature_opened`

Indica que una capacidad o superficie fue abierta por el usuario.

Propiedades sugeridas:

```json
{
  "open_mode": "route",
  "initial_state": "empty"
}
```

Campos:

- `open_mode`: `route`, `drawer`, `modal`, `embedded`, `programmatic`.
- `initial_state`: estado tecnico no sensible.

### `feature_action_performed`

Indica que el usuario ejecuto una accion funcional.

```json
{
  "action_id": "save",
  "action_group": "primary_actions",
  "result": "succeeded",
  "error_code": null
}
```

Campos:

- `action_id`: nombre tecnico estable.
- `action_group`: agrupador opcional.
- `result`: `started`, `succeeded`, `failed`, `cancelled`.
- `error_code`: codigo tecnico sin datos sensibles.

## Eventos De Busqueda

### `search_performed`

```json
{
  "search_id": "search_123",
  "query_length": 5,
  "search_term": "abcde",
  "filters": { "type": "item" },
  "results_count": 8
}
```

`search_term` solo debe enviarse cuando el integrador pueda garantizar que no contiene datos personales ni texto sensible. Si hay duda, enviar solo `query_length`.

### `search_result_selected`

```json
{
  "search_id": "search_123",
  "result_type": "item",
  "result_id": "456",
  "position": 2,
  "source": "manual_search"
}
```

`source` recomendado:

- `suggestion`
- `manual_search`
- `advanced_search`
- `recent`
- `fallback`

### `search_empty_result`

```json
{
  "search_id": "search_123",
  "query_length": 5,
  "filters": { "type": "item" }
}
```

### `search_abandoned`

```json
{
  "search_id": "search_123",
  "query_length": 5,
  "results_count": 8,
  "time_open_ms": 12000,
  "last_interaction": "viewed_results"
}
```

## Eventos De Formularios

GoAnalytics ya reconoce estos eventos recomendados:

- `form_validation_attempt`
- `form_completed`
- `form_abandoned`
- `form_step_advanced`
- `form_step_viewed`

Contrato de payload:

```json
{
  "form_id": "checkout_form",
  "step_id": "payment",
  "valid_fields": ["payment_method"],
  "invalid_fields": ["installments"],
  "field_errors": {
    "installments": "required"
  },
  "valid_count": 1,
  "error_count": 1,
  "attempt_number": 2,
  "time_open_ms": 45000
}
```

Reglas:

- No enviar valores ingresados por el usuario.
- `field_errors` debe contener codigos, no mensajes libres.
- Los nombres de campos deben ser tecnicos y estables.

## Eventos De Frustracion

### `rage_click_detected`

```json
{
  "target_id": "submit_button",
  "click_count": 5,
  "window_ms": 2000
}
```

### `dead_click_detected`

```json
{
  "target_id": "summary_total",
  "target_role": "text"
}
```

### `flow_abandoned`

```json
{
  "flow_name": "checkout",
  "step_id": "payment",
  "time_open_ms": 90000,
  "dirty_state": true,
  "reason": "close"
}
```

`reason` recomendado:

- `navigation`
- `close`
- `timeout`
- `session_end`
- `unknown`

## Eventos De Items

Los eventos de items existentes siguen siendo el contrato principal:

- `item_impression`
- `item_viewed`
- `item_image_zoomed`
- `cart_item_added`
- `checkout_started`
- `purchase_completed`

Para seleccion de item en contextos que no son carrito, el integrador puede usar un evento generico:

```json
{
  "event_name": "item_selected_for_context",
  "properties": {
    "item_id": "123",
    "item_type": "service",
    "selection_source": "suggestion",
    "position": 1
  },
  "context": {
    "feature": "generic_feature",
    "surface": "drawer",
    "component_id": "item_search"
  }
}
```

Si GoAnalytics decide normalizar este evento en `analytics_event_items`, debe tratarlo como extension opcional del contrato de items.
