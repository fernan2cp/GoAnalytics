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
    title: "Workspace"
  };
  globalThis.fetch = async (_url, options) => {
    requests.push(JSON.parse(String(options.body)));
    return { ok: true, status: 202 };
  };
  setLocation("/workspace/search");
});

test("feature helpers delegan en track con contexto e idempotencia", async () => {
  const analytics = createClient();

  analytics.featureOpened({ open_reason: "user_action" }, {
    logicalEventId: "feature_opened:workspace:catalog_search",
    idempotencyKey: "open:catalog_search",
    context: {
      app_area: "backoffice",
      feature: "catalog_search",
      surface: "drawer",
      entry_point: "navigation_menu",
      component_id: "item_search"
    }
  });
  analytics.featureActionPerformed({ action: "primary_button_clicked", result: "accepted" });
  await analytics.flush();
  await analytics.destroy();

  const events = capturedEvents();
  assert.equal(events.length, 2);
  assert.equal(events[0].event_name, "feature_opened");
  assert.equal(events[0].logical_event_id, "feature_opened:workspace:catalog_search");
  assert.equal(events[0].idempotency_key, "open:catalog_search");
  assert.equal(events[0].properties.open_reason, "user_action");
  assert.equal(events[0].context.feature, "catalog_search");
  assert.equal(events[0].context.sdk_name, "@go-analytics/web-sdk");
  assert.equal(events[1].event_name, "feature_action_performed");
  assert.equal(events[1].properties.action, "primary_button_clicked");
});

test("search helpers emiten eventos genericos con payload minimo", async () => {
  const analytics = createClient();

  analytics.searchPerformed({ search_id: "search_1", query_length: 4, result_count: 2 }, { context: { component_id: "item_search" } });
  analytics.searchResultSelected({ search_id: "search_1", result_type: "item", result_id: "item_100", position: 1 });
  analytics.searchEmptyResult({ search_id: "search_2", query_length: 8 });
  analytics.searchAbandoned({ search_id: "search_3", elapsed_ms: 1500 });
  await analytics.flush();
  await analytics.destroy();

  const names = capturedEvents().map((event) => event.event_name);
  assert.deepEqual(names, ["search_performed", "search_result_selected", "search_empty_result", "search_abandoned"]);
  assert.equal(capturedEvents()[0].context.component_id, "item_search");
  assert.equal(capturedEvents()[1].properties.result_id, "item_100");
});

test("frustration y flow helpers emiten eventos esperados", async () => {
  const analytics = createClient();

  analytics.rageClickDetected({ target_id: "save_button", clicks_count: 5, window_ms: 1200 });
  analytics.deadClickDetected({ target_id: "disabled_action" });
  analytics.flowAbandoned({ flow_id: "flow_1", step_id: "step_2", reason: "navigation_away" });
  await analytics.flush();
  await analytics.destroy();

  const names = capturedEvents().map((event) => event.event_name);
  assert.deepEqual(names, ["rage_click_detected", "dead_click_detected", "flow_abandoned"]);
  assert.equal(capturedEvents()[2].properties.flow_id, "flow_1");
});

test("form helpers sanitizan valores y campos inseguros", async () => {
  const analytics = createClient();

  analytics.formAttempt({
    form_id: "settings_form",
    step_id: "profile",
    valid_fields: ["display_name", "email", "profile.timezone"],
    invalid_fields: ["password", "profile-name"],
    field_errors: {
      display_name: "required",
      email: "invalid",
      password: "required"
    },
    error_count: 2,
    value: "raw user value",
    token: "secret"
  });
  await analytics.flush();
  await analytics.destroy();

  const [event] = capturedEvents();
  assert.equal(event.event_name, "form_validation_attempt");
  assert.deepEqual(event.properties.valid_fields, ["display_name", "profile.timezone"]);
  assert.deepEqual(event.properties.invalid_fields, ["profile-name"]);
  assert.deepEqual(event.properties.field_errors, { display_name: "required" });
  assert.equal(event.properties.value, undefined);
  assert.equal(event.properties.token, undefined);
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
    origin: "https://workspace.example.com",
    href: `https://workspace.example.com${path}`,
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