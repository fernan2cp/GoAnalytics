# Contrato JWT

Go Analytics usa JWT firmado sin cifrado. Fase 1 define el puerto `EventTokenVerifier`, el DTO de claims y las reglas de validacion de claims que el nucleo de ingesta necesita.

El algoritmo inicial de infraestructura es HS256 y se implementa en Fase 2 detras de `EventTokenVerifier`, para permitir RS256 o ES256 mas adelante sin cambiar el caso de uso.

## Claims obligatorios

```json
{
  "iss": "main-backend",
  "aud": "analytics-ingest",
  "site_code": "pub_site_abc123",
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

El secreto compartido para HS256 se configura con `GO_ANALYTICS_JWT_SECRET`.

La expiracion inicial recomendada es de 30 minutos y se configura con `JWT_EXPIRATION_MINUTES`.
