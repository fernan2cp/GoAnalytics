# Variables de entorno

Las variables base estan documentadas en `.env.example`.

## Convenciones

- `APP_NAME=go-analytics`.
- El stream principal es `goanalytics:events:raw`.
- Las claves Redis deben usar el prefijo `goanalytics:*`.
- Los secretos reales no deben commitearse.

## Redis

```text
goanalytics:site:public:{site_code}
goanalytics:rehydrate:last_attempt:{site_code}
goanalytics:site:not_found:{site_code}
goanalytics:dedup:exact:{event_id}
goanalytics:dedup:logical:{tenant_id}:{site_id}:{logical_event_id}
goanalytics:dedup:idempotency:{tenant_id}:{site_id}:{idempotency_key}
goanalytics:dedup:semantic:{hash}:{bucket}
goanalytics:ratelimit:site:{site_code}:{minute}
goanalytics:ratelimit:ip:{ip_hash}:{minute}
```

## Deduplicacion semantica

`EVENT_SEMANTIC_DEDUP_RULES_JSON` permite declarar reglas explicitas del worker.
Debe usarse solo como respaldo cuando no exista `logical_event_id` ni
`idempotency_key`. Las ventanas deben ser bajas, por ejemplo:

```json
[{"event_name":"page_view","window_ms":200,"fields":["session_id","tab_id","path"]}]
```
