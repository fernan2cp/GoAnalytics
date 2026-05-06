# Go Analytics Web SDK

SDK TypeScript para enviar eventos browser a Go Analytics.

```ts
import { createAnalyticsClient } from "@go-analytics/web-sdk";

const analytics = createAnalyticsClient({
  token: trackingToken,
  endpoint: "https://analytics.example.com/v1/events",
  flushIntervalMs: 5000,
  batchSize: 10
});

analytics.page();
analytics.track("product_viewed", {
  product_id: "123",
  category: "calzado"
});
```

## API

- `createAnalyticsClient(options)` crea un cliente aislado del backend principal.
- `track(eventName, properties?, options?)` encola eventos genericos.
- `page(properties?, options?)` envia un evento `page_view` con datos de navegacion.
- `identify(userId)` asocia eventos futuros a un usuario conocido sin enviar secretos.
- `flush()` fuerza el envio del batch pendiente.
- `destroy()` detiene el timer interno y envia lo pendiente.

El transporte por defecto usa `fetch` con `Authorization: Bearer <tracking_jwt>` y `keepalive` para respetar el contrato de ingesta. `sendBeacon` puede habilitarse con `beaconEndpoint` cuando exista un endpoint compatible que no requiera headers personalizados.

## Opciones principales

```ts
type AnalyticsClientOptions = {
  token: string;
  endpoint: string;
  flushIntervalMs?: number;
  batchSize?: number;
  sessionTimeoutMs?: number;
  useBeacon?: boolean;
  beaconEndpoint?: string;
};
```

El SDK genera `event_id`, `anonymous_id` persistente y `session_id` automaticamente. Nunca firma tokens ni conoce secretos.

## Desarrollo e Instalación

### Generar el SDK (Build)

Para compilar el SDK y generar los archivos de distribución:

```bash
cd packages/web-sdk
npm install
npm run build
```

Esto generará la carpeta `dist/` necesaria para que el paquete sea funcional.

### Opciones de Instalación en otros proyectos

#### 1. Uso Local (npm link)
Ideal para desarrollo en la misma máquina:
- En `packages/web-sdk`: `npm link`
- En tu proyecto: `npm link @go-analytics/web-sdk`

#### 2. Instalación desde Git
Puedes instalar el SDK directamente desde el repositorio sin publicarlo en npm:
```bash
npm install github:fernan2cp/GoAnalytics#main:packages/web-sdk
```

#### 3. Publicación en Registro (npm)
Si tienes permisos, puedes publicarlo para uso general:
```bash
npm publish --access public
```
