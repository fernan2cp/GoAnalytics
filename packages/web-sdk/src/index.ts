export type AnalyticsClientOptions = {
  token: string;
  endpoint: string;
  flushIntervalMs?: number;
  batchSize?: number;
};

export type AnalyticsClient = {
  track(eventName: string, properties?: Record<string, unknown>): void;
  flush(): Promise<void>;
};

export function createAnalyticsClient(options: AnalyticsClientOptions): AnalyticsClient {
  void options;

  return {
    track() {
      // La implementacion del SDK comienza en la fase dedicada.
    },
    async flush() {
      // La implementacion del SDK comienza en la fase dedicada.
    }
  };
}
