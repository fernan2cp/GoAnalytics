import assert from "node:assert/strict";
import { beforeEach, test } from "node:test";
import { createAnalyticsClient } from "../dist/index.js";

const endpoint = "https://analytics.example.com/v1/events";
let requests;

beforeEach(() => {
  requests = [];
  globalThis.localStorage = createStorage();
  globalThis.sessionStorage = createStorage();
  globalThis.document = {
    referrer: "",
    title: "Tienda"
  };
  globalThis.fetch = async (_url, options) => {
    requests.push(JSON.parse(String(options.body)));
    return { ok: true };
  };
  setLocation("/eshop/catalogue");
});

test("deduplica page_view duplicado dentro de la ventana local", async () => {
  const analytics = createClient();

  analytics.page();
  analytics.page();
  await analytics.flush();
  await analytics.destroy();

  const events = capturedEvents();
  assert.equal(events.length, 1);
  assert.equal(events[0].event_name, "page_view");
  assert.ok(events[0].logical_event_id);
  assert.ok(events[0].idempotency_key);
});

test("genera navigation_id e idempotency_key distintos para rutas SPA distintas", async () => {
  const analytics = createClient();

  analytics.page();
  setLocation("/eshop/checkout");
  analytics.page();
  await analytics.flush();
  await analytics.destroy();

  const events = capturedEvents();
  assert.equal(events.length, 2);
  assert.notEqual(events[0].properties.navigation_id, events[1].properties.navigation_id);
  assert.notEqual(events[0].idempotency_key, events[1].idempotency_key);
});

test("volver a una ruta previa luego de otra navegacion genera nuevo navigation_id", async () => {
  const analytics = createClient();

  analytics.page();
  setLocation("/eshop/checkout");
  analytics.page();
  setLocation("/eshop/catalogue");
  analytics.page();
  await analytics.flush();
  await analytics.destroy();

  const events = capturedEvents();
  assert.equal(events.length, 3);
  assert.notEqual(events[0].properties.navigation_id, events[2].properties.navigation_id);
  assert.notEqual(events[0].idempotency_key, events[2].idempotency_key);
});

test("genera idempotency_key automatica para eventos con logical_event_id", async () => {
  const analytics = createClient();

  analytics.track("product_viewed", { product_id: "sku_123" }, { logicalEventId: "logical_product_123" });
  await analytics.flush();
  await analytics.destroy();

  const [event] = capturedEvents();
  assert.equal(event.logical_event_id, "logical_product_123");
  assert.equal(event.idempotency_key, "product_viewed:ev_sess_mock:ev_tab_mock:logical_product_123");
});

test("checkoutStarted reutiliza identidad para el mismo carrito", async () => {
  const analytics = createClient();

  analytics.checkoutStarted({ cart_id: "cart_123", value: 99.99, currency: "ARS", items_count: 2 });
  analytics.checkoutStarted({ cart_id: "cart_123", value: 99.99, currency: "ARS", items_count: 2 });
  await analytics.flush();
  await analytics.destroy();

  const events = capturedEvents();
  assert.equal(events.length, 2);
  assert.equal(events[0].logical_event_id, events[1].logical_event_id);
  assert.equal(events[0].idempotency_key, events[1].idempotency_key);
});

test("checkoutStarted separa identidades para carritos distintos", async () => {
  const analytics = createClient();

  analytics.checkoutStarted({ cart_id: "cart_123" });
  analytics.checkoutStarted({ cart_id: "cart_456" });
  await analytics.flush();
  await analytics.destroy();

  const events = capturedEvents();
  assert.equal(events.length, 2);
  assert.notEqual(events[0].logical_event_id, events[1].logical_event_id);
  assert.notEqual(events[0].idempotency_key, events[1].idempotency_key);
});

test("respeta logicalEventId e idempotencyKey provistos por el consumidor", async () => {
  const analytics = createClient();

  analytics.track("checkout_started", { cart_id: "cart_123" }, {
    logicalEventId: "custom_logical",
    idempotencyKey: "custom_idempotency"
  });
  await analytics.flush();
  await analytics.destroy();

  const [event] = capturedEvents();
  assert.equal(event.logical_event_id, "custom_logical");
  assert.equal(event.idempotency_key, "custom_idempotency");
});

function createClient() {
  return createAnalyticsClient({
    token: "tracking_token",
    endpoint,
    batchSize: 100,
    flushIntervalMs: 60_000,
    useBeacon: false
  });
}

function capturedEvents() {
  return requests.flatMap((request) => request.events);
}

function setLocation(path) {
  globalThis.location = {
    origin: "https://tienda.example.com",
    href: `https://tienda.example.com${path}`,
    pathname: path,
    search: "",
    hash: ""
  };
}

function createStorage() {
  const values = new Map([
    ["goanalytics:session", JSON.stringify({ id: "ev_sess_mock", updatedAt: Date.now() })],
    ["goanalytics:tab_id", "ev_tab_mock"]
  ]);
  return {
    getItem(key) {
      return values.get(key) ?? null;
    },
    setItem(key, value) {
      values.set(key, String(value));
    },
    removeItem(key) {
      values.delete(key);
    },
    clear() {
      values.clear();
    }
  };
}
