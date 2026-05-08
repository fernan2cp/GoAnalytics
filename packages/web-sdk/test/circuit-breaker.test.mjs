import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { createAnalyticsClient } from "../dist/index.js";

const endpoint = "https://analytics.example.com/v1/events";
const originalDateNow = Date.now;
const originalConsoleWarn = console.warn;
let now;
let requests;
let warnings;

beforeEach(() => {
  now = 1_700_000_000_000;
  requests = [];
  warnings = [];
  Date.now = () => now;
  console.warn = (message) => {
    warnings.push(String(message));
  };
  globalThis.localStorage = createStorage();
  globalThis.sessionStorage = createStorage();
  globalThis.document = {
    referrer: "",
    title: "Tienda"
  };
  setLocation("/eshop/catalogue");
});

afterEach(() => {
  Date.now = originalDateNow;
  console.warn = originalConsoleWarn;
});

test("abre el circuito despues de fallos consecutivos de red", async () => {
  let attempts = 0;
  globalThis.fetch = async () => {
    attempts += 1;
    throw new TypeError("net::ERR_CONNECTION_REFUSED");
  };
  const analytics = createClient({ maxFailures: 2, circuitOpenMs: 1_000 });

  analytics.track("product_viewed", { product_id: "sku_123" });
  await analytics.flush();
  await analytics.flush();
  await analytics.flush();
  await analytics.destroy();

  assert.equal(attempts, 2);
  assert.equal(warnings.length, 1);
});

test("omite envios mientras el circuito esta abierto y permite recuperacion tras cooldown", async () => {
  let attempts = 0;
  globalThis.fetch = async (_url, options) => {
    attempts += 1;
    if (attempts <= 2) {
      throw new TypeError("Failed to fetch");
    }
    requests.push(JSON.parse(String(options.body)));
    return { ok: true, status: 202 };
  };
  const analytics = createClient({ maxFailures: 2, circuitOpenMs: 1_000 });

  analytics.track("product_viewed", { product_id: "sku_123" });
  await analytics.flush();
  await analytics.flush();
  await analytics.flush();
  now += 1_001;
  await analytics.flush();

  assert.equal(attempts, 3);
  assert.equal(capturedEvents().length, 1);
  await analytics.destroy();
});

test("un envio exitoso tras cooldown resetea el contador y continua enviando", async () => {
  let attempts = 0;
  globalThis.fetch = async (_url, options) => {
    attempts += 1;
    if (attempts <= 2) {
      throw new TypeError("Failed to fetch");
    }
    requests.push(JSON.parse(String(options.body)));
    return { ok: true, status: 202 };
  };
  const analytics = createClient({ maxFailures: 2, circuitOpenMs: 1_000 });

  analytics.track("product_viewed", { product_id: "sku_123" });
  await analytics.flush();
  await analytics.flush();
  now += 1_001;
  await analytics.flush();
  analytics.track("cart_item_added", { product_id: "sku_123" });
  await analytics.flush();
  await analytics.destroy();

  assert.equal(attempts, 4);
  assert.equal(capturedEvents().length, 2);
});

test("descarta errores 4xx sin reintentar indefinidamente el batch", async () => {
  let attempts = 0;
  globalThis.fetch = async () => {
    attempts += 1;
    return { ok: false, status: 400 };
  };
  const analytics = createClient();

  analytics.track("product_viewed", { product_id: "sku_123" });
  await analytics.flush();
  await analytics.flush();
  await analytics.destroy();

  assert.equal(attempts, 1);
  assert.equal(warnings.length, 1);
});

test("descarta payload demasiado grande sin abrir el circuito", async () => {
  let attempts = 0;
  globalThis.fetch = async (_url, options) => {
    attempts += 1;
    requests.push(JSON.parse(String(options.body)));
    return { ok: true, status: 202 };
  };
  const analytics = createClient({ maxPayloadBytes: 1_000 });

  analytics.track("oversized_event", { payload: "x".repeat(2_000) });
  await analytics.flush();
  analytics.track("small_event", {});
  await analytics.flush();
  await analytics.destroy();

  assert.equal(attempts, 1);
  assert.equal(capturedEvents().length, 1);
  assert.equal(capturedEvents()[0].event_name, "small_event");
  assert.equal(warnings.length, 0);
});

function createClient(overrides = {}) {
  return createAnalyticsClient({
    token: "tracking_token",
    endpoint,
    batchSize: 100,
    flushIntervalMs: 60_000,
    useBeacon: false,
    ...overrides
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
    ["goanalytics:session", JSON.stringify({ id: "ev_sess_mock", updatedAt: now })],
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
