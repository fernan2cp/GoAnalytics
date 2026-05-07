export type JsonPrimitive = string | number | boolean | null;
export type JsonValue = JsonPrimitive | JsonValue[] | { [key: string]: JsonValue };
export type EventProperties = Record<string, JsonValue>;

export type AnalyticsClientOptions = {
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

export type TrackOptions = {
  eventId?: string;
  logicalEventId?: string;
  idempotencyKey?: string;
  tabId?: string;
  sequence?: number;
  previousLogicalEventId?: string;
  critical?: boolean;
  eventVersion?: number;
  timestamp?: Date;
  anonymousId?: string;
  sessionId?: string;
  userId?: string | null;
  origin?: string;
  url?: string;
  path?: string;
  referrer?: string;
  context?: EventProperties;
};

export type PageOptions = Omit<TrackOptions, "origin" | "url" | "path" | "referrer"> & {
  title?: string;
  navigationId?: string;
  origin?: string;
  url?: string;
  path?: string;
  referrer?: string;
};

export type AnalyticsClient = {
  track(eventName: string, properties?: EventProperties, options?: TrackOptions): void;
  page(properties?: EventProperties, options?: PageOptions): void;
  formAttempt(payload: FormAttemptPayload, options?: TrackOptions): void;
  formCompleted(payload: FormCompletedPayload, options?: TrackOptions): void;
  formAbandoned(payload: FormAbandonedPayload, options?: TrackOptions): void;
  formStepAdvanced(payload: FormStepPayload, options?: TrackOptions): void;
  formStepViewed(payload: FormStepPayload, options?: TrackOptions): void;
  identify(userId: string | null): void;
  flush(): Promise<void>;
  destroy(): Promise<void>;
};

type AnalyticsEventPayload = {
  event_id: string;
  logical_event_id?: string;
  idempotency_key?: string;
  tab_id?: string;
  sequence?: number;
  previous_logical_event_id?: string;
  event_name: string;
  event_version: number;
  timestamp: string;
  anonymous_id: string;
  session_id: string;
  user_id: string | null;
  origin: string;
  url: string;
  path: string;
  referrer: string;
  properties: EventProperties;
  context: EventProperties;
};

export type FormFieldErrors = Record<string, string>;

export type FormAttemptPayload = {
  form_id: string;
  step_id?: string;
  valid_fields?: string[];
  invalid_fields?: string[];
  field_errors?: FormFieldErrors;
  valid_count?: number;
  error_count?: number;
  attempt_number?: number;
};

export type FormCompletedPayload = {
  form_id: string;
  step_id?: string;
  valid_count?: number;
  attempt_number?: number;
};

export type FormAbandonedPayload = {
  form_id: string;
  step_id?: string;
  valid_fields?: string[];
  invalid_fields?: string[];
  valid_count?: number;
  error_count?: number;
  attempt_number?: number;
};

export type FormStepPayload = {
  form_id: string;
  step_id: string;
  attempt_number?: number;
};

const SDK_NAME = "@go-analytics/web-sdk";
const SDK_VERSION = "0.1.0";
const DEFAULT_FLUSH_INTERVAL_MS = 5000;
const DEFAULT_BATCH_SIZE = 10;
const DEFAULT_SESSION_TIMEOUT_MS = 30 * 60 * 1000;
const DEFAULT_MAX_QUEUE_SIZE = 100;
const DEFAULT_MAX_PAYLOAD_BYTES = 64 * 1024;
const ANONYMOUS_ID_KEY = "goanalytics:anonymous_id";
const SESSION_KEY = "goanalytics:session";
const TAB_ID_KEY = "goanalytics:tab_id";
const SEQUENCE_KEY = "goanalytics:sequence";
const NAVIGATION_ID_KEY = "goanalytics:navigation_id";
const PREVIOUS_LOGICAL_EVENT_ID_KEY = "goanalytics:previous_logical_event_id";
const FORM_EVENT_NAMES = new Set(["form_validation_attempt", "form_completed", "form_abandoned", "form_step_advanced", "form_step_viewed"]);
const BLOCKED_FORM_KEYS = new Set(["value", "values", "text", "message", "email", "phone", "address", "document", "dni", "card", "password", "token"]);

export function createAnalyticsClient(options: AnalyticsClientOptions): AnalyticsClient {
  const config = normalizeOptions(options);
  const storage = createBrowserStorage();
  const tabStorage = createTabStorage();
  let anonymousId = storage.get(ANONYMOUS_ID_KEY);
  let userId: string | null = null;
  let queue: AnalyticsEventPayload[] = [];
  let flushInFlight: Promise<void> | null = null;
  const tabId = currentTabId(tabStorage);
  let sequence = currentSequence(tabStorage);
  let previousLogicalEventId = tabStorage.get(PREVIOUS_LOGICAL_EVENT_ID_KEY) ?? "";

  if (!anonymousId) {
    anonymousId = `anon_${createId()}`;
    storage.set(ANONYMOUS_ID_KEY, anonymousId);
  }
  const persistedAnonymousId = anonymousId;

  const timer = setFlushTimer(config.flushIntervalMs, () => {
    void flush();
  });

  function track(eventName: string, properties: EventProperties = {}, eventOptions: TrackOptions = {}): void {
    try {
      const sessionId = eventOptions.sessionId ?? currentSessionId(storage, config.sessionTimeoutMs);
      const nextSequence = eventOptions.sequence ?? nextSequenceValue(tabStorage, sequence);
      sequence = nextSequence;
      const event = buildEvent(eventName, properties, {
        ...eventOptions,
        anonymousId: eventOptions.anonymousId ?? persistedAnonymousId,
        sessionId,
        userId: eventOptions.userId ?? userId,
        tabId: eventOptions.tabId ?? tabId,
        sequence: nextSequence,
        previousLogicalEventId: eventOptions.previousLogicalEventId ?? previousLogicalEventId,
        logicalEventId: eventOptions.logicalEventId ?? logicalEventIdForEvent(eventName, sessionId, eventOptions.tabId ?? tabId, nextSequence, properties)
      });

      previousLogicalEventId = event.logical_event_id ?? previousLogicalEventId;
      if (event.logical_event_id) {
        tabStorage.set(PREVIOUS_LOGICAL_EVENT_ID_KEY, event.logical_event_id);
      }
      queue = enqueueEvent(queue, event, config, eventOptions.critical ?? isCriticalEvent(eventName));
      if (queue.length >= config.batchSize) {
        void flush();
      }
    } catch {
      // El SDK no debe romper la aplicacion host si falla la preparacion local.
    }
  }

  function page(properties: EventProperties = {}, pageOptions: PageOptions = {}): void {
    const locationInfo = browserLocation();
    const navigationId = pageOptions.navigationId ?? currentNavigationId();
    const pageProperties: EventProperties = {
      navigation_id: navigationId,
      title: pageOptions.title ?? browserTitle(),
      ...properties
    };

    track("page_view", pageProperties, {
      ...pageOptions,
      logicalEventId: pageOptions.logicalEventId ?? pageLogicalEventId(navigationId, sessionIdFromStorage(storage, config.sessionTimeoutMs), tabId, locationInfo.path),
      origin: pageOptions.origin ?? locationInfo.origin,
      url: pageOptions.url ?? locationInfo.url,
      path: pageOptions.path ?? locationInfo.path,
      referrer: pageOptions.referrer ?? browserReferrer()
    });
  }

  function formAttempt(payload: FormAttemptPayload, eventOptions: TrackOptions = {}): void {
    track("form_validation_attempt", sanitizeFormPayload(payload), { ...eventOptions, critical: eventOptions.critical ?? true });
  }

  function formCompleted(payload: FormCompletedPayload, eventOptions: TrackOptions = {}): void {
    track("form_completed", sanitizeFormPayload(payload), { ...eventOptions, critical: eventOptions.critical ?? true });
  }

  function formAbandoned(payload: FormAbandonedPayload, eventOptions: TrackOptions = {}): void {
    track("form_abandoned", sanitizeFormPayload(payload), { ...eventOptions, critical: eventOptions.critical ?? true });
  }

  function formStepAdvanced(payload: FormStepPayload, eventOptions: TrackOptions = {}): void {
    track("form_step_advanced", sanitizeFormPayload(payload), { ...eventOptions, critical: eventOptions.critical ?? true });
  }

  function formStepViewed(payload: FormStepPayload, eventOptions: TrackOptions = {}): void {
    track("form_step_viewed", sanitizeFormPayload(payload), eventOptions);
  }

  function identify(nextUserId: string | null): void {
    userId = nextUserId;
  }

  async function flush(): Promise<void> {
    if (flushInFlight) {
      return flushInFlight;
    }
    if (queue.length === 0) {
      return;
    }

    flushInFlight = flushQueuedBatches()
      .finally(() => {
        flushInFlight = null;
      });

    return flushInFlight;
  }

  async function flushQueuedBatches(): Promise<void> {
    while (queue.length > 0) {
      const batch = queue.slice(0, config.batchSize);
      queue = queue.slice(batch.length);

      try {
        await sendBatch(config, batch);
      } catch (error) {
        if (isPayloadTooLargeError(error)) {
          continue;
        }
        queue = batch.concat(queue).slice(0, config.maxQueueSize);
        return;
      }
    }
  }

  async function destroy(): Promise<void> {
    clearFlushTimer(timer);
    await flush();
  }

  return {
    track,
    page,
    formAttempt,
    formCompleted,
    formAbandoned,
    formStepAdvanced,
    formStepViewed,
    identify,
    flush,
    destroy
  };
}

function normalizeOptions(options: AnalyticsClientOptions): Required<Pick<AnalyticsClientOptions, "token" | "endpoint" | "flushIntervalMs" | "batchSize" | "sessionTimeoutMs" | "maxQueueSize" | "maxPayloadBytes" | "useBeacon">> & Pick<AnalyticsClientOptions, "beaconEndpoint"> {
  if (!options.token || !options.token.trim()) {
    throw new Error("Go Analytics requiere un token de tracking.");
  }
  if (!options.endpoint || !options.endpoint.trim()) {
    throw new Error("Go Analytics requiere un endpoint de ingesta.");
  }

  return {
    token: options.token,
    endpoint: options.endpoint,
    flushIntervalMs: positiveNumber(options.flushIntervalMs, DEFAULT_FLUSH_INTERVAL_MS),
    batchSize: positiveNumber(options.batchSize, DEFAULT_BATCH_SIZE),
    sessionTimeoutMs: positiveNumber(options.sessionTimeoutMs, DEFAULT_SESSION_TIMEOUT_MS),
    maxQueueSize: positiveNumber(options.maxQueueSize, DEFAULT_MAX_QUEUE_SIZE),
    maxPayloadBytes: positiveNumber(options.maxPayloadBytes, DEFAULT_MAX_PAYLOAD_BYTES),
    useBeacon: options.useBeacon ?? true,
    beaconEndpoint: options.beaconEndpoint
  };
}

function buildEvent(eventName: string, properties: EventProperties, options: TrackOptions): AnalyticsEventPayload {
  const locationInfo = browserLocation();
  const timestamp = options.timestamp ?? new Date();

  return {
    event_id: options.eventId ?? createId(),
    logical_event_id: nonEmpty(options.logicalEventId),
    idempotency_key: nonEmpty(options.idempotencyKey),
    tab_id: nonEmpty(options.tabId),
    sequence: positiveIntegerOrUndefined(options.sequence),
    previous_logical_event_id: nonEmpty(options.previousLogicalEventId),
    event_name: eventName,
    event_version: options.eventVersion ?? 1,
    timestamp: timestamp.toISOString(),
    anonymous_id: options.anonymousId ?? "",
    session_id: options.sessionId ?? "",
    user_id: options.userId ?? null,
    origin: options.origin ?? locationInfo.origin,
    url: options.url ?? locationInfo.url,
    path: options.path ?? locationInfo.path,
    referrer: options.referrer ?? browserReferrer(),
    properties,
    context: {
      sdk_name: SDK_NAME,
      sdk_version: SDK_VERSION,
      ...options.context
    }
  };
}

async function sendBatch(config: ReturnType<typeof normalizeOptions>, events: AnalyticsEventPayload[]): Promise<void> {
  const body = JSON.stringify({ events });
  if (body.length > config.maxPayloadBytes) {
    throw new Error("payload_too_large");
  }

  if (config.useBeacon && config.beaconEndpoint && trySendBeacon(config.beaconEndpoint, body)) {
    return;
  }

  const response = await fetch(config.endpoint, {
    method: "POST",
    headers: {
      "Authorization": `Bearer ${config.token}`,
      "Content-Type": "application/json"
    },
    body,
    keepalive: body.length <= config.maxPayloadBytes
  });

  if (!response.ok) {
    throw new Error(`Go Analytics rechazo el batch con estado ${response.status}.`);
  }
}

function isPayloadTooLargeError(error: unknown): boolean {
  return error instanceof Error && error.message === "payload_too_large";
}

function trySendBeacon(endpoint: string, body: string): boolean {
  const navigatorRef = globalThis.navigator;
  if (!navigatorRef?.sendBeacon) {
    return false;
  }

  const payload = new Blob([body], { type: "application/json" });
  return navigatorRef.sendBeacon(endpoint, payload);
}

function currentSessionId(storage: StorageAdapter, timeoutMs: number): string {
  const now = Date.now();
  const stored = storage.get(SESSION_KEY);
  if (stored) {
    try {
      const parsed = JSON.parse(stored) as { id?: string; updatedAt?: number };
      if (parsed.id && parsed.updatedAt && now - parsed.updatedAt <= timeoutMs) {
        storage.set(SESSION_KEY, JSON.stringify({ id: parsed.id, updatedAt: now }));
        return parsed.id;
      }
    } catch {
      storage.remove(SESSION_KEY);
    }
  }

  const sessionId = `sess_${createId()}`;
  storage.set(SESSION_KEY, JSON.stringify({ id: sessionId, updatedAt: now }));
  return sessionId;
}

function sessionIdFromStorage(storage: StorageAdapter, timeoutMs: number): string {
  return currentSessionId(storage, timeoutMs);
}

function currentTabId(storage: StorageAdapter): string {
  const stored = storage.get(TAB_ID_KEY);
  if (stored) {
    return stored;
  }
  const tabId = `tab_${createId()}`;
  storage.set(TAB_ID_KEY, tabId);
  return tabId;
}

function currentSequence(storage: StorageAdapter): number {
  const stored = Number(storage.get(SEQUENCE_KEY));
  return Number.isFinite(stored) && stored > 0 ? Math.floor(stored) : 0;
}

function nextSequenceValue(storage: StorageAdapter, current: number): number {
  const next = current + 1;
  storage.set(SEQUENCE_KEY, String(next));
  return next;
}

function logicalEventIdForEvent(eventName: string, sessionId: string, tabId: string, sequence: number, properties: EventProperties): string {
  if (eventName === "page_view") {
    const navigationId = typeof properties.navigation_id === "string" ? properties.navigation_id : "";
    return pageLogicalEventId(navigationId, sessionId, tabId, browserLocation().path);
  }
  if (FORM_EVENT_NAMES.has(eventName)) {
    const formId = typeof properties.form_id === "string" ? properties.form_id : "";
    const stepId = typeof properties.step_id === "string" ? properties.step_id : "";
    return stableLogicalEventId([eventName, sessionId, tabId, formId, stepId, String(sequence)]);
  }
  return stableLogicalEventId([eventName, sessionId, tabId, String(sequence)]);
}

function pageLogicalEventId(navigationId: string | undefined, sessionId: string, tabId: string, path: string): string {
  return stableLogicalEventId(["page_view", sessionId, tabId, navigationId || currentNavigationId(), path]);
}

function currentNavigationId(): string {
  const storage = createTabStorage();
  const timeOrigin = String(globalThis.performance?.timeOrigin ?? Date.now());
  const stored = storage.get(NAVIGATION_ID_KEY);
  if (stored) {
    try {
      const parsed = JSON.parse(stored) as { id?: string; timeOrigin?: string };
      if (parsed.id && parsed.timeOrigin === timeOrigin) {
        return parsed.id;
      }
    } catch {
      if (stored.startsWith("nav_")) {
        return stored;
      }
    }
  }
  const navigationId = `nav_${createId()}`;
  storage.set(NAVIGATION_ID_KEY, JSON.stringify({ id: navigationId, timeOrigin }));
  return navigationId;
}

function stableLogicalEventId(parts: string[]): string {
  return parts.map((part) => encodeURIComponent(part || "")).join(":");
}

function enqueueEvent(queue: AnalyticsEventPayload[], event: AnalyticsEventPayload, config: ReturnType<typeof normalizeOptions>, critical: boolean): AnalyticsEventPayload[] {
  if (queue.length < config.maxQueueSize) {
    return queue.concat(event);
  }
  if (!critical) {
    return queue;
  }
  const removableIndex = queue.findIndex((item) => !isCriticalEvent(item.event_name));
  if (removableIndex < 0) {
    return queue;
  }
  const next = queue.slice();
  next.splice(removableIndex, 1);
  next.push(event);
  return next;
}

function isCriticalEvent(eventName: string): boolean {
  return eventName === "purchase_completed" || eventName === "checkout_started" || eventName === "form_completed" || eventName === "form_validation_attempt";
}

function sanitizeFormPayload(payload: object): EventProperties {
  const sanitized: EventProperties = {};
  for (const [key, value] of Object.entries(payload)) {
    if (BLOCKED_FORM_KEYS.has(key)) {
      continue;
    }
    if (key === "valid_fields" || key === "invalid_fields") {
      sanitized[key] = Array.isArray(value) ? value.filter(isSafeFieldName) : [];
      continue;
    }
    if (key === "field_errors") {
      sanitized[key] = sanitizeFieldErrors(value);
      continue;
    }
    if (typeof value === "string" && (key === "form_id" || key === "step_id")) {
      sanitized[key] = value;
      continue;
    }
    if (typeof value === "number" && Number.isFinite(value)) {
      sanitized[key] = value;
    }
  }
  return sanitized;
}

function sanitizeFieldErrors(value: unknown): EventProperties {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {};
  }
  const sanitized: EventProperties = {};
  for (const [field, code] of Object.entries(value as Record<string, unknown>)) {
    if (isSafeFieldName(field) && typeof code === "string") {
      sanitized[field] = code;
    }
  }
  return sanitized;
}

function isSafeFieldName(value: unknown): value is string {
  return typeof value === "string" && /^[a-zA-Z0-9_.-]{1,80}$/.test(value) && !BLOCKED_FORM_KEYS.has(value.toLowerCase());
}

function nonEmpty(value: string | undefined): string | undefined {
  return value && value.trim() ? value : undefined;
}

function positiveIntegerOrUndefined(value: number | undefined): number | undefined {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? Math.floor(value) : undefined;
}

function createId(): string {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID();
  }

  const random = Math.random().toString(36).slice(2);
  return `${Date.now().toString(36)}_${random}`;
}

function browserLocation(): { origin: string; url: string; path: string } {
  const locationRef = globalThis.location;
  if (!locationRef) {
    return { origin: "", url: "", path: "" };
  }

  return {
    origin: locationRef.origin,
    url: locationRef.href,
    path: `${locationRef.pathname}${locationRef.search}${locationRef.hash}`
  };
}

function browserReferrer(): string {
  return globalThis.document?.referrer ?? "";
}

function browserTitle(): string {
  return globalThis.document?.title ?? "";
}

function positiveNumber(value: number | undefined, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : fallback;
}

function setFlushTimer(intervalMs: number, callback: () => void): ReturnType<typeof setInterval> | null {
  if (typeof setInterval === "undefined") {
    return null;
  }
  return setInterval(callback, intervalMs);
}

function clearFlushTimer(timer: ReturnType<typeof setInterval> | null): void {
  if (timer) {
    clearInterval(timer);
  }
}

type StorageAdapter = {
  get(key: string): string | null;
  set(key: string, value: string): void;
  remove(key: string): void;
};

function createBrowserStorage(): StorageAdapter {
  const memory = new Map<string, string>();
  const localStorageRef = safeLocalStorage();

  return {
    get(key: string): string | null {
      return localStorageRef?.getItem(key) ?? memory.get(key) ?? null;
    },
    set(key: string, value: string): void {
      memory.set(key, value);
      localStorageRef?.setItem(key, value);
    },
    remove(key: string): void {
      memory.delete(key);
      localStorageRef?.removeItem(key);
    }
  };
}

function createTabStorage(): StorageAdapter {
  const memory = new Map<string, string>();
  const sessionStorageRef = safeSessionStorage();

  return {
    get(key: string): string | null {
      return sessionStorageRef?.getItem(key) ?? memory.get(key) ?? null;
    },
    set(key: string, value: string): void {
      memory.set(key, value);
      sessionStorageRef?.setItem(key, value);
    },
    remove(key: string): void {
      memory.delete(key);
      sessionStorageRef?.removeItem(key);
    }
  };
}

function safeLocalStorage(): Storage | null {
  try {
    return globalThis.localStorage ?? null;
  } catch {
    return null;
  }
}

function safeSessionStorage(): Storage | null {
  try {
    return globalThis.sessionStorage ?? null;
  } catch {
    return null;
  }
}
