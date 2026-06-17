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
}, {
  logicalEventId: "product_viewed:123"
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
  maxFailures?: number;
  circuitOpenMs?: number;
  useBeacon?: boolean;
  beaconEndpoint?: string;
};
```

`maxFailures` define cuantos fallos consecutivos de red o respuestas `5xx`
tolera el SDK antes de abrir el circuito. Por defecto usa `5`. `circuitOpenMs`
define cuanto tiempo queda pausado el envio antes de probar recuperacion; por
defecto usa `60000` ms.

Cuando el circuito esta abierto, `flush()` no realiza requests y los eventos
nuevos siguen entrando a la cola hasta `maxQueueSize`, respetando la politica
existente de eventos criticos. Al terminar el cooldown, el SDK permite un envio
de prueba: si funciona, resetea el contador y vuelve a operar normalmente; si
falla, pausa otro intervalo.

Los errores `4xx` de ingesta se consideran rechazos no recuperables para ese
batch y no se reintentan indefinidamente. Los fallos de red, timeouts del
navegador, `ERR_CONNECTION_REFUSED` y respuestas `5xx` cuentan para abrir el
circuito. Esto evita saturar la consola, la pestana Network y la aplicacion host
cuando el microservicio de ingesta esta caido.

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

Para eventos que involucran ítems o productos, el SDK permite enviar los datos en las propiedades del evento (`properties`). Esto soporta tanto el flujo de un solo ítem (pasando los campos directamente en el objeto de propiedades) como el flujo de múltiples ítems (enviando un arreglo `items`).

### Eventos de Ítems Soportados

| Evento | Propósito | Payload Recomendado |
|---|---|---|
| `item_impression` | Ítem visible realmente en pantalla. | Campos de visibilidad (`visible_ratio`, `visible_time_ms`), `item_id`, `surface`, `list_instance_id`. |
| `item_viewed` | Visualización del detalle del ítem. | `item_id`, `variant_id` (opcional), `sku` (opcional). |
| `item_image_zoomed`| Zoom de imagen del ítem. | `item_id`, `variant_id` (si aplica a la imagen). |
| `cart_item_added` | Ítem agregado al carrito. | `item_id`, `quantity`, `unit_price`, `currency`, `cart_id`. |
| `checkout_started` | Inicio de proceso de pago. | Lista de `items` (cada uno con `item_id`, `quantity`, etc.) y `checkout_id`. |
| `purchase_completed`| Compra confirmada. | Lista de `items` (cada uno con `item_id`, `order_line_id`, `quantity`) y `order_id`. |

### Regla Estricta para `item_impression`

Para evitar sesgos y asegurar un cálculo de scoring confiable en el backend, no debes emitir `item_impression` por render técnico, pre-cargas o skeletons. Solo debe emitirse cuando el ítem sea visible realmente:
- El ítem tiene al menos un **50% de visibilidad** en el viewport (`visible_ratio >= 0.5`).
- El tiempo visible acumulado es de al menos **1000 ms** (`visible_time_ms >= 1000`).
- La pestaña está activa (`document.visibilityState === "visible"`).

### Ejemplos de Implementación

#### 1. Registrar una Impresión Real (`item_impression`)
```ts
analytics.track("item_impression", {
  item_id: "prod_987",
  variant_id: "var_blue_large",
  sku: "SKU-987-BL",
  item_type: "product",
  surface: "search_results",
  position: 3,
  page: 1,
  list_instance_id: "list_search_results_abc",
  impression_batch_id: "batch_xyz",
  visible_ratio: 0.85,
  visible_time_ms: 1200,
  viewport_width: 1920,
  viewport_height: 1080,
  rendered_at: new Date().toISOString()
}, {
  // logicalEventId específico para deduplicación semántica de impresiones
  logicalEventId: `item_impression:session_123:search_results:prod_987:var_blue_large`
});
```

#### 2. Registrar Vista de Detalle (`item_viewed`)
```ts
analytics.track("item_viewed", {
  item_id: "prod_987",
  variant_id: "var_blue_large",
  sku: "SKU-987-BL",
  item_type: "product",
  surface: "product_detail",
  category_ids: ["category_shoes", "category_running"]
}, {
  logicalEventId: `item_viewed:prod_987:var_blue_large`
});
```

#### 3. Agregar al Carrito (`cart_item_added`)
```ts
analytics.track("cart_item_added", {
  cart_id: "cart_abc123",
  item_id: "prod_987",
  variant_id: "var_blue_large",
  sku: "SKU-987-BL",
  quantity: 2,
  unit_price: 4500.00,
  currency: "ARS"
}, {
  logicalEventId: `cart_item_added:cart_abc123:prod_987:var_blue_large`
});
```

#### 4. Inicio de Checkout (Helper Dedicado)
Para registrar el inicio del checkout con múltiples ítems, utiliza el helper `checkoutStarted` pasando la estructura con el arreglo `items`:
```ts
analytics.checkoutStarted({
  checkout_id: "check_555",
  cart_id: "cart_abc123",
  value: 9000.00,
  currency: "ARS",
  items_count: 2,
  items: [
    {
      item_id: "prod_987",
      variant_id: "var_blue_large",
      sku: "SKU-987-BL",
      quantity: 2,
      unit_price: 4500.00,
      currency: "ARS"
    }
  ]
});
```

#### 5. Compra Completada (`purchase_completed` - Multi-ítem)
Para asegurar que el scoring reciba datos de compras auditables, es requerido incluir el identificador de orden (`order_id`) y la línea de la orden (`order_line_id`) de cada ítem:
```ts
analytics.track("purchase_completed", {
  order_id: "order_xyz789",
  currency: "ARS",
  gross_amount: 9000.00,
  net_amount: 8500.00,
  discount_amount: 500.00,
  items: [
    {
      item_id: "prod_987",
      variant_id: "var_blue_large",
      sku: "SKU-987-BL",
      order_line_id: "line_0",
      quantity: 2,
      unit_price: 4500.00,
      gross_amount: 9000.00,
      discount_amount: 500.00
    }
  ]
}, {
  logicalEventId: `purchase_completed:order_xyz789`
});
```

### Notas sobre Deduplicación e Idempotencia

Para eventos personalizados o disparados desde componentes que pueden montarse más de una vez (ej. `useEffect` en React StrictMode), es altamente recomendable pasar el `logicalEventId` de forma manual en las opciones. Esto evita que se generen duplicados técnicos en la base de datos de analítica.

Los helpers de formulario no aceptan valores reales ingresados por el usuario. Solo transportan nombres técnicos de campos, códigos de error y conteos.

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
