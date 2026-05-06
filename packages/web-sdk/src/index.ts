export type JsonPrimitive = string | number | boolean | null;
export type JsonValue = JsonPrimitive | JsonValue[] | { [key: string]: JsonValue };
export type EventProperties = Record<string, JsonValue>;

export type AnalyticsClientOptions = {
  token: string;
  endpoint: string;
  flushIntervalMs?: number;
  batchSize?: number;
  sessionTimeoutMs?: number;
  useBeacon?: boolean;
  beaconEndpoint?: string;
};

export type TrackOptions = {
  eventId?: string;
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
  origin?: string;
  url?: string;
  path?: string;
  referrer?: string;
};

export type AnalyticsClient = {
  track(eventName: string, properties?: EventProperties, options?: TrackOptions): void;
  page(properties?: EventProperties, options?: PageOptions): void;
  identify(userId: string | null): void;
  flush(): Promise<void>;
  destroy(): Promise<void>;
};

type AnalyticsEventPayload = {
  event_id: string;
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

const SDK_NAME = "@go-analytics/web-sdk";
const SDK_VERSION = "0.1.0";
const DEFAULT_FLUSH_INTERVAL_MS = 5000;
const DEFAULT_BATCH_SIZE = 10;
const DEFAULT_SESSION_TIMEOUT_MS = 30 * 60 * 1000;
const ANONYMOUS_ID_KEY = "goanalytics:anonymous_id";
const SESSION_KEY = "goanalytics:session";

export function createAnalyticsClient(options: AnalyticsClientOptions): AnalyticsClient {
  const config = normalizeOptions(options);
  const storage = createBrowserStorage();
  let anonymousId = storage.get(ANONYMOUS_ID_KEY);
  let userId: string | null = null;
  let queue: AnalyticsEventPayload[] = [];
  let flushInFlight: Promise<void> | null = null;

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
      const event = buildEvent(eventName, properties, {
        ...eventOptions,
        anonymousId: eventOptions.anonymousId ?? persistedAnonymousId,
        sessionId,
        userId: eventOptions.userId ?? userId
      });

      queue.push(event);
      if (queue.length >= config.batchSize) {
        void flush();
      }
    } catch {
      // El SDK no debe romper la aplicacion host si falla la preparacion local.
    }
  }

  function page(properties: EventProperties = {}, pageOptions: PageOptions = {}): void {
    const locationInfo = browserLocation();
    const pageProperties: EventProperties = {
      title: pageOptions.title ?? browserTitle(),
      ...properties
    };

    track("page_view", pageProperties, {
      ...pageOptions,
      origin: pageOptions.origin ?? locationInfo.origin,
      url: pageOptions.url ?? locationInfo.url,
      path: pageOptions.path ?? locationInfo.path,
      referrer: pageOptions.referrer ?? browserReferrer()
    });
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
      } catch {
        queue = batch.concat(queue).slice(0, Math.max(config.batchSize * 3, batch.length));
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
    identify,
    flush,
    destroy
  };
}

function normalizeOptions(options: AnalyticsClientOptions): Required<Pick<AnalyticsClientOptions, "token" | "endpoint" | "flushIntervalMs" | "batchSize" | "sessionTimeoutMs" | "useBeacon">> & Pick<AnalyticsClientOptions, "beaconEndpoint"> {
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
    useBeacon: options.useBeacon ?? true,
    beaconEndpoint: options.beaconEndpoint
  };
}

function buildEvent(eventName: string, properties: EventProperties, options: TrackOptions): AnalyticsEventPayload {
  const locationInfo = browserLocation();
  const timestamp = options.timestamp ?? new Date();

  return {
    event_id: options.eventId ?? createId(),
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
    keepalive: body.length <= 64 * 1024
  });

  if (!response.ok) {
    throw new Error(`Go Analytics rechazo el batch con estado ${response.status}.`);
  }
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

function safeLocalStorage(): Storage | null {
  try {
    return globalThis.localStorage ?? null;
  } catch {
    return null;
  }
}
