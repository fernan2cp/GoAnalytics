# Verificacion minima de Fase 8 en el backend principal

## Objetivo

Validar que el proyecto principal cumple la integracion minima requerida por
Go Analytics sin modificar codigo del microservicio.

Esta verificacion confirma cinco puntos:

1. El backend principal genera un JWT de tracking valido.
2. El frontend recibe el `trackingToken`.
3. El SDK envia eventos a Go Analytics.
4. El backend principal expone el resolver interno.
5. El worker puede rehidratar metadata de site cuando Redis esta vacio.

## Variables requeridas

Confirmar que el backend principal y Go Analytics usan los mismos valores:

```env
GO_ANALYTICS_JWT_SECRET=change_me_in_production
JWT_ISSUER=main-backend
JWT_AUDIENCE=analytics-ingest
JWT_EXPIRATION_MINUTES=30
SITE_RESOLVER_TOKEN=change_me
SITE_RESOLVER_URL=http://main-backend/internal/analytics/sites/resolve
```

En el frontend confirmar:

```env
VITE_GO_ANALYTICS_EVENTS_ENDPOINT=http://localhost:8080/v1/events
```

Usar la URL real del servicio de ingesta segun el entorno.

## 1. Verificar token en `/site/info`

Hacer una solicitud al endpoint publico que usa el frontend:

```bash
curl -s http://localhost:7000/api/v1/site/info
```

El JSON debe incluir:

```json
{
  "trackingToken": "<jwt>"
}
```

Decodificar el payload del JWT y confirmar estos claims:

```json
{
  "iss": "main-backend",
  "aud": "analytics-ingest",
  "site_code": "pub_site_abc123",
  "env": "production",
  "token_version": 1,
  "iat": 1715012345,
  "nbf": 1715012345,
  "exp": 1715014145,
  "jti": "01HXTRACKINGTOKEN123",
  "tenant_hint": "tenant_123",
  "site_hint": "site_456"
}
```

`jti` es obligatorio para Go Analytics. `tenant_hint` y `site_hint` son
opcionales, pero recomendados para integraciones multitenant.

## 2. Verificar aceptacion del token en Ingest

Enviar un evento minimo usando el token recibido:

```bash
curl -i -X POST http://localhost:8080/v1/events \
  -H "Authorization: Bearer <trackingToken>" \
  -H "Content-Type: application/json" \
  -d "{\"events\":[{\"event_id\":\"evt_manual_1\",\"event_name\":\"page_view\",\"event_version\":1,\"timestamp\":\"2026-05-06T12:00:00Z\",\"anonymous_id\":\"anon_manual\",\"session_id\":\"sess_manual\",\"origin\":\"https://cliente.com\",\"url\":\"https://cliente.com/\",\"path\":\"/\",\"properties\":{},\"context\":{}}]}"
```

Resultado esperado: respuesta exitosa del endpoint de ingesta. Si devuelve
error de autenticacion, revisar firma, `GO_ANALYTICS_JWT_SECRET`, `iss`, `aud`,
`exp`, `nbf` y `jti`.

## 3. Verificar resolver interno

Probar el endpoint interno del backend principal:

```bash
curl -i -X POST http://localhost:7000/internal/analytics/sites/resolve \
  -H "Authorization: Bearer <SITE_RESOLVER_TOKEN>" \
  -H "Content-Type: application/json" \
  -d "{\"site_code\":\"pub_site_abc123\",\"origin\":\"https://cliente.com\",\"env\":\"production\"}"
```

Respuesta esperada para un site existente:

```json
{
  "site_code": "pub_site_abc123",
  "tenant_id": "tenant_123",
  "site_id": "site_456",
  "status": "active",
  "tracking_enabled": true,
  "allowed_domains": ["cliente.com", "www.cliente.com"],
  "token_version": 1,
  "sample_rate": 1,
  "schema_version": 1
}
```

Para un site inexistente debe responder `404` con:

```json
{
  "error": "site_not_found"
}
```

## 4. Verificar hidratacion Redis

Confirmar que el backend principal puede escribir metadata en:

```text
goanalytics:site:public:{site_code}
```

El valor debe ser JSON compatible con el contrato del resolver. Si se usa TTL,
debe ser coherente con `SITE_CACHE_TTL_SECONDS`.

## 5. Verificar rehidratacion automatica

Con Redis accesible desde Go Analytics:

1. Borrar la clave `goanalytics:site:public:{site_code}`.
2. Enviar un evento valido con el SDK o con `curl`.
3. Confirmar que el worker llama a `SITE_RESOLVER_URL`.
4. Confirmar que Redis vuelve a contener `goanalytics:site:public:{site_code}`.
5. Confirmar que el evento queda persistido en `analytics_events`.

Si el site no existe, confirmar que se crea negative cache temporal en:

```text
goanalytics:site:not_found:{site_code}
```

## 6. Verificar frontend

Abrir la aplicacion principal y comprobar en la consola de red:

- El frontend obtiene `trackingToken` desde `/site/info`.
- El SDK se inicializa con `VITE_GO_ANALYTICS_EVENTS_ENDPOINT`.
- Al cambiar de ruta se envia `page_view`.
- Al ver producto se envia `product_viewed`.
- Al agregar al carrito se envia `cart_item_added`.
- Al iniciar checkout se envia `checkout_started`.

## Criterio de cierre

La Fase 8 puede marcarse como completada cuando:

- Ingest acepta el JWT generado por el backend principal.
- El frontend envia eventos reales con el SDK.
- El resolver interno responde metadata valida y valida `SITE_RESOLVER_TOKEN`.
- El worker rehidrata Redis ante cache miss.
- Eventos validos quedan en `analytics_events`.
- Eventos con dominio, token_version o site invalido quedan rechazados.
