# Variables de entorno

Las variables base estan documentadas en `.env.example`.

## Convenciones

- `APP_NAME=go-analytics`.
- El stream principal es `goanalytics:events:raw`.
- Las claves Redis deben usar el prefijo `goanalytics:*`.
- Los secretos reales no deben commitearse.

## Redis

```text
goanalytics:site:public:{site_public_id}
goanalytics:rehydrate:last_attempt:{site_public_id}
goanalytics:site:not_found:{site_public_id}
goanalytics:ratelimit:site:{site_public_id}:{minute}
goanalytics:ratelimit:ip:{ip_hash}:{minute}
```
