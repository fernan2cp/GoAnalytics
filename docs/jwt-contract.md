# Contrato JWT

En Fase 1 Go Analytics usa JWT firmado sin cifrado. El algoritmo inicial es HS256, detras del puerto `EventTokenVerifier`, para permitir RS256 o ES256 mas adelante.

## Claims obligatorios

```json
{
  "iss": "main-backend",
  "aud": "analytics-ingest",
  "site_public_id": "pub_site_abc123",
  "env": "production",
  "token_version": 1,
  "iat": 1710000000,
  "nbf": 1710000000,
  "exp": 1710001800,
  "jti": "01HV..."
}
```

## Claims opcionales

```json
{
  "tenant_hint": "tenant_123",
  "site_hint": "site_456"
}
```

La expiracion inicial recomendada es de 30 minutos y se configura con `JWT_EXPIRATION_MINUTES`.
