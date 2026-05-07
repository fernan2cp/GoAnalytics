# Instrucciones mínimas para integrar Go Analytics en el proyecto actual

## Objetivo

Integrar el SDK web de Go Analytics en el frontend del proyecto actual y dejar preparado el backend principal para permitir la hidratación interna de metadata requerida por el microservicio.

Este instructivo se limita a los pasos mínimos de integración:

1. Instalar el SDK.
2. Inicializar el SDK en el frontend.
3. Agregar eventos desde el frontend.
4. Crear una URL interna del backend para hidratación de metadata.

El resto de la configuración, instalación y operación del microservicio queda fuera de este documento y debe consultarse en la documentación propia de Go Analytics.

---

## 1. Instalar el SDK en el frontend

### Opción recomendada para desarrollo local

Si el SDK está disponible localmente dentro del repositorio del microservicio:

```bash
cd packages/web-sdk
npm install
npm run build
npm link
```

Luego, en el proyecto frontend actual:

```bash
npm link @go-analytics/web-sdk
```

### Opción desde repositorio Git

Si se instala directamente desde Git:

```bash
npm install github:fernan2cp/GoAnalytics#main:packages/web-sdk
```

### Opción desde npm

Si el paquete ya fue publicado:

```bash
npm install @go-analytics/web-sdk
```

---

## 2. Inicializar el SDK en el frontend

El frontend debe recibir desde el backend principal un `trackingToken` válido y configurar el endpoint público de ingesta del microservicio.

Ejemplo base:

```ts
import { createAnalyticsClient } from "@go-analytics/web-sdk";

const analytics = createAnalyticsClient({
  token: trackingToken,
  endpoint: import.meta.env.VITE_GO_ANALYTICS_EVENTS_ENDPOINT,
  flushIntervalMs: 5000,
  batchSize: 10,
});
```

El SDK no debe generar ni firmar tokens. El token debe ser entregado por el backend principal.

### Variable de entorno sugerida para el frontend

```env
VITE_GO_ANALYTICS_EVENTS_ENDPOINT=https://analytics.example.com/v1/events
```

---

## 3. Enviar evento de página vista

Cuando el usuario ingresa o cambia de pantalla, enviar un evento `page_view`.

Ejemplo:

```ts
analytics.page();
```

También puede enviarse información adicional de la pantalla:

```ts
analytics.page({
  path: window.location.pathname,
  title: document.title,
});
```

En React, esto puede ejecutarse al montar una página o al detectar cambios de ruta.

Ejemplo simple:

```ts
useEffect(() => {
  analytics.page({
    path: location.pathname,
    title: document.title,
  });
}, [location.pathname]);
```

---

## 4. Agregar eventos personalizados en el frontend

Usar `track` para registrar acciones relevantes del usuario.

Ejemplo:

```ts
analytics.track("product_viewed", {
  product_id: "123",
  category: "calzado",
});
```

Ejemplos de eventos útiles para análisis de comportamiento:

```ts
analytics.track("button_clicked", {
  button_id: "checkout_start",
  screen: "cart",
});
```

```ts
analytics.track("checkout_started", {
  cart_id: cartId,
  total_items: items.length,
});
```

```ts
analytics.track("search_performed", {
  query,
  results_count: results.length,
});
```

```ts
analytics.track("form_submitted", {
  form_id: "contact_form",
  screen: "contact",
});
```

---

## 5. Identificar usuario conocido

Cuando el usuario esté autenticado, se puede asociar la sesión a un identificador de usuario.

```ts
analytics.identify(user.id);
```

No enviar contraseñas, tokens, documentos, emails sensibles ni datos personales innecesarios dentro de los eventos.

---

## 6. Forzar envío de eventos pendientes

En situaciones donde se necesite asegurar el envío inmediato del batch pendiente:

```ts
await analytics.flush();
```

Por ejemplo, antes de cerrar una operación importante o al finalizar un flujo crítico.

---

## 7. Destruir la instancia del SDK

Si se desmonta completamente la aplicación o se necesita detener el cliente:

```ts
analytics.destroy();
```

Esto detiene el timer interno y envía los eventos pendientes.

---

## 8. Generar el `trackingToken` en el backend principal

El `trackingToken` es un JWT (JSON Web Token) firmado con el algoritmo **HS256** utilizando una clave secreta compartida entre el backend principal y Go Analytics.

### Estructura del Token

#### Header
```json
{
  "alg": "HS256",
  "typ": "JWT"
}
```

#### Payload (Claims)
El payload debe contener la siguiente estructura para cumplir con el contrato del puerto de validación (`EventTokenVerifier` en el servicio de ingesta). 

En un **proyecto multitenant**, el backend debe generar este payload de manera **dinámica** por cada solicitud o sesión, inyectando los datos específicos del tenant y sitio actual.

| Campo (Domain) | JSON Key | Tipo | Creación | Descripción |
| :--- | :--- | :--- | :--- | :--- |
| `Issuer` | `iss` | String | Estático | Identificador del emisor (ej. `main-backend`). |
| `Audience` | `aud` | String / Array | Estático | Audiencia esperada por Go Analytics (ej. `analytics-ingest`). |
| `SiteCode` | `site_code` | String | **Dinámico** | El código público del sitio actual. Cambia según el tenant/sitio. |
| `Environment` | `env` | String | Estático | Entorno de ejecución (ej. `production`). |
| `TokenVersion` | `token_version` | Integer | Estático | Versión del formato del token (actualmente `1`). |
| `IssuedAt` | `iat` | Integer | **Dinámico** | Timestamp Unix de creación (en segundos). |
| `NotBefore` | `nbf` | Integer | **Dinámico** | Timestamp Unix desde el cual es válido (en segundos). |
| `ExpiresAt` | `exp` | Integer | **Dinámico** | Timestamp Unix de expiración (en segundos). |
| `TenantHint` | `tenant_hint` | String | **Dinámico** | (Opcional) El ID interno del tenant. Altamente recomendado en multitenant. |
| `SiteHint` | `site_hint` | String | **Dinámico** | (Opcional) El ID interno del sitio. Altamente recomendado en multitenant. |
| `JWTID` | `jti` | String | **Dinámico** | Identificador único obligatorio del token. |

> **Importante para Multitenant:** El uso de `tenant_hint` y `site_hint` permite que Go Analytics procese los eventos de forma mucho más rápida al evitar búsquedas complejas para resolver a qué tenant pertenece el `site_code`.
> 
> **Nota sobre Sincronización de Tiempo:** Asegúrese de que el servidor que genera los tokens esté sincronizado con UTC. El microservicio aplica un margen de tolerancia (leeway) de 1 minuto, pero discrepancias mayores causarán el rechazo del token.

### Ejemplo de Payload JSON Dinámico

Ejemplo de cómo debería verse el JSON generado por el backend para el tenant "123" y sitio "456":

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

### Firma del Token
El backend debe firmar este payload usando el secreto definido en la variable de entorno `GO_ANALYTICS_JWT_SECRET` del microservicio.

---

## 9. Crear URL interna del backend para hidratación

El backend principal debe exponer una URL interna para que el microservicio pueda resolver la metadata de un sitio cuando no exista en Redis o necesite rehidratarla.

### Endpoint requerido

```http
POST /internal/analytics/sites/resolve
Authorization: Bearer <SITE_RESOLVER_TOKEN>
Content-Type: application/json
```

### Request esperado

```json
{
  "site_code": "pub_site_abc123",
  "origin": "https://cliente.com",
  "env": "production"
}
```

### Response esperado cuando el sitio existe

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

### Response esperado cuando el sitio no existe

```json
{
  "error": "site_not_found"
}
```

Código HTTP esperado:

```http
404 Not Found
```

---

## 10. Requerimientos mínimos de la URL interna

La URL interna del backend debe cumplir estas condiciones:

1. Validar el header `Authorization: Bearer <SITE_RESOLVER_TOKEN>`.
2. Recibir `site_code`, `origin` y `env`.
3. Buscar el sitio correspondiente dentro del backend principal.
4. Devolver metadata normalizada para Go Analytics.
5. No devolver stack traces ni detalles internos del sistema.
6. No exponer esta URL públicamente si no es necesario.
7. No acoplar el microservicio a modelos, tablas ni servicios internos del backend principal.

---

## 11. Checklist mínimo de integración

- [ ] SDK instalado en el frontend.
- [ ] SDK inicializado con `trackingToken` y endpoint de ingesta.
- [ ] Evento `page_view` enviado al cambiar de pantalla.
- [ ] Eventos personalizados agregados en acciones importantes.
- [ ] Usuario identificado con `analytics.identify(user.id)` cuando corresponda.
- [ ] Backend principal genera el `trackingToken` (JWT HS256) con los claims correctos.
- [ ] Backend principal entrega el `trackingToken` al frontend.
- [ ] Backend principal expone `/internal/analytics/sites/resolve`.
- [ ] La URL interna valida `SITE_RESOLVER_TOKEN`.
- [ ] La URL interna devuelve metadata compatible con Go Analytics.

---

## 12. Resumen de variables de entorno (Configuración mínima)

Para que la integración funcione correctamente, asegúrese de tener configuradas las siguientes variables en el proyecto principal:

```env
# Secreto compartido para firmar los Tracking Tokens (JWT HS256)
GO_ANALYTICS_JWT_SECRET=change_me_in_production

# Token de seguridad para la comunicación entre el Microservicio y el Backend Principal
SITE_RESOLVER_TOKEN=change_me

# Política de CORS: Orígenes permitidos (separados por coma). Use * para desarrollo local.
CORS_ALLOWED_ORIGINS=*

# Endpoint público del microservicio de ingesta (usado por el SDK en el frontend)
VITE_GO_ANALYTICS_EVENTS_ENDPOINT=http://localhost:8080/v1/events
```

