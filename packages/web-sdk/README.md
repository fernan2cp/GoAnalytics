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

analytics.track("page_view");
```

La implementacion funcional del SDK pertenece a la fase 6.
