# Contrato de rehidratacion de site

El worker no depende de FastAPI. Solo conoce una URL interna configurable mediante `SITE_RESOLVER_URL`.

## Request

```http
POST /internal/analytics/sites/resolve
Authorization: Bearer <SITE_RESOLVER_TOKEN>
Content-Type: application/json
```

```json
{
  "site_code": "pub_site_abc123",
  "origin": "https://cliente.com",
  "env": "production"
}
```

## Response exitosa

```json
{
  "site_code": "pub_site_abc123",
  "tenant_id": "tenant_123",
  "site_id": "site_456",
  "status": "active",
  "tracking_enabled": true,
  "allowed_domains": ["cliente.com", "www.cliente.com"],
  "token_version": 1,
  "sample_rate": 1,
  "schema_version": 1
}
```

## No encontrado

```json
{
  "error": "site_not_found"
}
```
