# Cambios Recomendados En GoAnalytics

## Principios

- Mantener compatibilidad hacia atras.
- Preferir campos opcionales.
- No introducir valores de dominio como enums obligatorios.
- No bloquear eventos por contexto desconocido.
- Reutilizar `properties` y `context` antes de crear contratos paralelos.
- Mantener la arquitectura hexagonal existente.

## SDK Web

Extensiones recomendadas:

- Tipar un `TrackingContext` generico opcional.
- Agregar helpers para eventos de busqueda.
- Agregar helper generico para `feature_opened` y `feature_action_performed`.
- Mantener helpers de formularios existentes.
- Documentar como pasar `context` sin forzar nombres de dominio.

Ejemplo conceptual:

```ts
client.trackFeatureOpened({
  context: {
    app_area: 'backoffice',
    feature: 'sales',
    surface: 'drawer'
  },
  properties: {
    open_mode: 'drawer'
  }
});
```

Estos helpers deben ser azucar sobre `track`, no un nuevo transporte.

## Validacion De Ingesta

La ingesta debe:

- Aceptar `context` como objeto JSON.
- Aplicar bloqueo de claves sensibles tambien a objetos anidados.
- Validar tamanos maximos de payload.
- No rechazar campos de contexto desconocidos si son JSON seguro.
- Mantener aliases existentes como `metadata -> properties`.

## Persistencia

La tabla `analytics_events` ya conserva `properties` y `context` como JSONB. Para analisis contextual basico, esto es suficiente.

Indices opcionales a evaluar si el volumen lo justifica:

```sql
CREATE INDEX ix_analytics_events_context_feature_time
ON analytics_events (tenant_id, site_id, ((context->>'feature')), event_time DESC);

CREATE INDEX ix_analytics_events_context_surface_time
ON analytics_events (tenant_id, site_id, ((context->>'surface')), event_time DESC);
```

Estos indices deben introducirse solo con evidencia de consultas frecuentes. Para v1 puede bastar con persistencia JSONB y agregacion offline.

## Normalizacion De Items

`analytics_event_items` ya normaliza eventos de items. Para soportar rankings por contexto, hay dos opciones:

1. Leer `context` desde `analytics_events` al agregar metricas.
2. Copiar campos contextuales seleccionados a `analytics_event_items`.

La opcion 1 evita migracion y mantiene flexibilidad. La opcion 2 mejora performance para consultas frecuentes, pero debe mantener campos nullable y genericos.

Campos candidatos si se normalizan:

- `app_area`
- `feature`
- `surface_context`
- `entry_point`
- `component_id`
- `entity_type`

No usar `surface` si ya existe con semantica de item; preferir `surface_context` o documentar una unica semantica clara.

## Agregados Opcionales

Decision v1: los agregados contextuales quedan fuera de GoAnalytics. El servicio conserva eventos crudos y detalles normalizados existentes; los integradores pueden leer/exportar datos en modo server-to-server y materializar agregados propios.

No crear en v1:

- migracion `analytics_context_item_aggregates`;
- job periodico de scoring contextual;
- endpoint `GET /v1/aggregates/items/top`;
- credencial `query_jwt` para agregados online.

Fase posterior opcional:

Si GoAnalytics implementa agregados, deben ejecutarse fuera de ingesta:

- Job periodico.
- Worker batch.
- Vista materializada refrescada.
- Export hacia data warehouse.

El algoritmo de score debe versionarse con `algorithm_version`.

## Seguridad

- No almacenar valores de formularios.
- No permitir claves sensibles en `properties` ni `context`.
- No exponer endpoints de agregados con tracking token publico si devuelven datos internos.
- Documentar credencial server-to-server para consultas.

## Documentacion Publica

Actualizar o extender:

- `docs/event-contract.md`
- `docs/event-conventions.md`
- `packages/web-sdk/README.md`

La documentacion debe presentar ejemplos genericos. Los integradores especificos pueden vivir en guias separadas, no en el contrato base.
