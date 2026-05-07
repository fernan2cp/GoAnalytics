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

analytics.checkoutStarted({
  cart_id: "cart_123",
  value: 99.99,
  currency: "ARS",
  items_count: 2
});

analytics.formAttempt({
  form_id: "checkout_address",
  step_id: "address",
  valid_fields: ["country"],
  invalid_fields: ["postal_code"],
  field_errors: { postal_code: "invalid_format" },
  valid_count: 1,
  error_count: 1
});
```

## API

- `createAnalyticsClient(options)` crea un cliente aislado del backend principal.
- `track(eventName, properties?, options?)` encola eventos genericos.
- `page(properties?, options?)` envia un evento `page_view` con datos de navegacion.
- `checkoutStarted(payload, options?)` registra el inicio de checkout con identidad funcional cuando existe `checkout_id`, `cart_id` u `order_draft_id`.
- `formAttempt(payload, options?)` registra un intento de validacion o envio.
- `formCompleted(payload, options?)` registra un formulario completado.
- `formAbandoned(payload, options?)` registra abandono de formulario.
- `formStepAdvanced(payload, options?)` registra avance de paso.
- `formStepViewed(payload, options?)` registra visualizacion de paso.
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
  maxQueueSize?: number;
  maxPayloadBytes?: number;
  useBeacon?: boolean;
  beaconEndpoint?: string;
};
```

El SDK genera `event_id`, `logical_event_id`, `idempotency_key`, `anonymous_id`
persistente, `session_id`, `tab_id` y `sequence` automaticamente. `event_id`
identifica el intento tecnico de envio. `logical_event_id` identifica el evento
logico y se usa como base de `idempotency_key` cuando el consumidor no la
provee manualmente. La clave generada usa:

```text
event_name:session_id:tab_id:logical_event_id
```

`tab_id` y `sequence` se guardan en `sessionStorage` para conservar continuidad
durante la vida de la pestana, incluso ante refresh. `navigation_id` tambien se
guarda en `sessionStorage`, pero cambia cuando cambia el `path` para representar
navegaciones SPA reales. Nunca firma tokens ni conoce secretos.

`page()` evita encolar dos `page_view` identicos durante una ventana local de
1000 ms. Esto reduce duplicados causados por doble inicializacion, React
StrictMode o handlers ejecutados dos veces. Si aun asi dos intentos equivalentes
llegan al backend, comparten `logical_event_id` e `idempotency_key`.

`checkoutStarted()` marca el evento como critico por defecto. Su identidad
funcional usa primero `checkout_id`, luego `cart_id` y finalmente
`order_draft_id`. Si no recibe ninguno de esos campos, el evento se envia con la
politica generica y el backend puede aplicar su fallback semantico.

Los helpers de formulario no aceptan valores reales ingresados por el usuario.
Solo transportan nombres tecnicos de campos, codigos de error y conteos.

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
