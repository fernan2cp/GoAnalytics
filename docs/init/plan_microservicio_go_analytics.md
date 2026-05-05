# Plan de implementación — Microservicio de analítica multitenant con arquitectura hexagonal

## 1. Objetivo general

Crear un repositorio independiente para un sistema de analítica de eventos web, compuesto por:

1. Un microservicio de ingesta desarrollado en Go.
2. Un worker de procesamiento desarrollado en Go.
3. Un SDK JavaScript/TypeScript preparado para publicarse luego en npm.
4. Integración con Redis para validación de sitios, rate limit, cache y rehidratación.
5. Persistencia inicial en PostgreSQL, con diseño preparado para migrar a ClickHouse en una fase posterior.
6. Validación mediante JWT firmado sin cifrado a través de un adaptador de infraestructura; Fase 1 define el puerto y las reglas de claims.
7. Expiración configurable del token, con valor inicial recomendado de 30 minutos.
8. Estructura interna basada en arquitectura hexagonal desde la primera versión.

El sistema debe ser genérico, reutilizable en futuros proyectos y desacoplado del backend principal. El backend integrador puede ser FastAPI, Django, Node.js u otro, siempre que cumpla el contrato de generación de token y rehidratación.

---

## 2. Principios de arquitectura

El proyecto debe respetar una arquitectura hexagonal, también conocida como Ports & Adapters.

La idea central es que la lógica del sistema no dependa directamente de:

- HTTP.
- Redis.
- PostgreSQL.
- Kafka.
- FastAPI.
- Variables de entorno.
- Frameworks externos.
- Librerías concretas de infraestructura.

El núcleo debe definir reglas, casos de uso y contratos. Los adaptadores externos implementan esos contratos.

---

## 3. Objetivos de arquitectura hexagonal

La estructura debe permitir:

1. Reemplazar Redis Streams por Kafka o Redpanda sin reescribir la lógica de negocio.
2. Reemplazar PostgreSQL por ClickHouse sin modificar los casos de uso principales.
3. Reemplazar JWT HS256 por RS256/ES256 sin modificar los handlers HTTP.
4. /* Cambiar el backend de rehidratación, hoy FastAPI, por otro sistema futuro. */ Este punto no es así, la hidratación se deberá manejar siempre como una api interna, independientemente de quien recibe esa api, que en este caso será fastapi, pero desde el punto de vista del microservicio, solo es una llamado a una url interna que dispara la hidratación y devuelve un 200 cuando la hidratación finaliza.
5. Testear casos de uso sin levantar Redis, PostgreSQL ni HTTP.
6. Mantener servicios Go independientes y escalables.
7. Evitar que el SDK quede acoplado al backend principal o al microservicio.
8. Mantener el repo preparado para publicar el SDK en npm.

---

## 4. Arquitectura objetivo de Fase 1

```text
Backend principal
  ├─ genera JWT firmado
  ├─ hidrata Redis con configuración del site
  └─ expone endpoint interno de rehidratación

Frontend del proyecto
  └─ inicializa SDK con tracking token

SDK JavaScript/TypeScript
  ├─ genera event_id
  ├─ genera anonymous_id
  ├─ genera session_id
  ├─ agrupa eventos
  └─ envía batches al microservicio Go

Go Ingestion API
  ├─ recibe eventos
  ├─ valida JWT
  ├─ valida estructura básica
  ├─ aplica rate limit básico
  ├─ publica evento en stream/queue
  └─ devuelve 204

Go Worker
  ├─ consume eventos
  ├─ valida site contra Redis
  ├─ rehidrata Redis si falta metadata
  ├─ valida dominio, estado y token_version
  ├─ guarda eventos válidos en DB
  └─ registra eventos rechazados/sospechosos
```

---

## 5. Modelo hexagonal general

```text
external world
  ↓
adapters/inbound
  - HTTP handlers
  - worker consumers
  - CLI commands
  ↓
application
  - use cases
  - orchestration
  - transaction boundaries
  ↓
domain
  - entities
  - value objects
  - domain services
  - validation rules
  ↓
ports/outbound
  - EventPublisher
  - EventRepository
  - SiteConfigRepository
  - SiteResolver
  - TokenVerifier
  - RateLimiter
  ↓
adapters/outbound
  - Redis Stream
  - PostgreSQL
  - HTTP resolver
  - JWT library
  - Redis cache
```

---

## 6. Stack inicial recomendado

### Backend del microservicio

```text
Lenguaje: Go
API HTTP: net/http, chi, fiber o gin
Config: variables de entorno
Validación JWT: librería Go JWT detrás de un puerto TokenVerifier
Redis: go-redis detrás de puertos de cache, queue y rate limit
DB inicial: PostgreSQL detrás de EventRepository
Driver PostgreSQL: pgx
Pool de conexiones: pgxpool
Migraciones: golang-migrate
Logs: slog o zerolog detrás de interfaz simple
Tests: go test
Contenedores: Docker + docker-compose
```

### SDK

```text
Lenguaje: TypeScript
Build: Vite library mode o tsup
Salida:
  - ESM para npm
  - UMD/IIFE para script browser futuro
Tipos:
  - .d.ts para TypeScript
Publicación futura:
  - npm público
```

---

## 7. Estructura inicial del repositorio

```text
go-analytics/
  README.md
  LICENSE
  .gitignore
  .env.example
  docker-compose.yml
  Makefile

  services/
    ingest/
      cmd/
        ingest/
          main.go

      internal/
        domain/
          event/
            entity.go
            value_objects.go
            validation.go
          site/
            entity.go
            value_objects.go
          token/
            claims.go

        application/
          usecases/
            ingest_events.go
          dto/
            ingest_request.go
            ingest_response.go
          ports/
            inbound/
              ingest_events.go
            outbound/
              event_publisher.go
              token_verifier.go
              rate_limiter.go
              clock.go
              id_generator.go
              logger.go

        adapters/
          inbound/
            http/
              router.go
              handlers.go
              middleware.go
              request_mapper.go

          outbound/
            jwt/
              hs256_verifier.go
            redis/
              event_stream_publisher.go
              rate_limiter.go
            logger/
              slog_logger.go
            system/
              clock.go
              id_generator.go

        bootstrap/
          config.go
          container.go

      Dockerfile
      go.mod
      go.sum

    worker/
      cmd/
        worker/
          main.go

      internal/
        domain/
          event/
            entity.go
            validation.go
          site/
            entity.go
            validation.go
          rejection/
            entity.go

        application/
          usecases/
            process_events.go
            validate_site.go
            rehydrate_site.go
          dto/
            raw_event.go
          ports/
            inbound/
              event_consumer.go
            outbound/
              event_consumer.go
              event_repository.go
              rejected_event_repository.go
              site_cache.go
              site_resolver.go
              deduplicator.go
              logger.go
              clock.go

        adapters/
          inbound/
            redisstream/
              consumer.go

          outbound/
            postgres/
              event_repository.go
              rejected_event_repository.go
            redis/
              site_cache.go
              deduplicator.go
            httpresolver/
              site_resolver.go
            logger/
              slog_logger.go
            system/
              clock.go

        bootstrap/
          config.go
          container.go

      Dockerfile
      go.mod
      go.sum

  packages/
    web-sdk/
      src/
        index.ts
        client.ts
        transport.ts
        queue.ts
        session.ts
        storage.ts
        types.ts
        utils.ts
      package.json
      tsconfig.json
      vite.config.ts
      README.md

  migrations/
    postgres/
      001_create_analytics_events.sql
      002_create_rejected_events.sql

  docs/
    architecture.md
    hexagonal-architecture.md
    env.md
    jwt-contract.md
    sdk-integration.md
    resolver-contract.md
    event-contract.md
    event-conventions.md
    deployment.md
    security.md
```

---

## 8. Regla principal de dependencias

Las dependencias deben apuntar hacia adentro.

```text
adapters → application → domain
```

Permitido:

```text
HTTP handler importa application/usecases
Use case importa domain y ports
Redis adapter implementa ports/outbound
Postgres adapter implementa ports/outbound
Worker bootstrap crea `pgxpool.Pool` e inyecta repositorios PostgreSQL
```

Prohibido:

```text
domain importando Redis
domain importando PostgreSQL
domain importando HTTP
application importando go-redis directamente
application importando driver de PostgreSQL directamente
domain leyendo variables .env
use case leyendo variables .env
```

---

## 9. Capas del microservicio

### 9.1 Domain

Contiene las reglas más estables.

Debe incluir:

- Entidades.
- Value objects.
- Validaciones puras.
- Reglas de aceptación o rechazo de eventos.
- Definición de estados válidos.
- Normalización básica independiente de infraestructura.

Ejemplos:

```text
domain/event
domain/site
domain/token
domain/rejection
```

No debe tener:

- Redis.
- SQL.
- HTTP.
- JWT library concreta.
- Variables de entorno.

---

### 9.2 Application

Contiene casos de uso.

Ejemplos:

```text
IngestEventsUseCase
ProcessEventsUseCase
ValidateSiteUseCase
RehydrateSiteUseCase
```

Responsabilidades:

- Orquestar reglas de dominio.
- Usar puertos de salida.
- Decidir el flujo general.
- Manejar errores de aplicación.
- Definir DTOs internos.
- No conocer detalles de infraestructura.

Ejemplo conceptual:

```go
type IngestEventsUseCase struct {
    tokenVerifier EventTokenVerifier
    publisher     EventPublisher
    rateLimiter   RateLimiter
    clock         Clock
    logger        Logger
}
```

---

### 9.3 Ports

Contratos que definen lo que la aplicación necesita.

Ejemplos:

```go
type EventPublisher interface {
    Publish(ctx context.Context, events []RawEvent) error
}

type EventRepository interface {
    SaveBatch(ctx context.Context, events []ValidatedEvent) error
}

type SiteCache interface {
    GetByPublicID(ctx context.Context, sitePublicID string) (*SiteConfig, error)
    Set(ctx context.Context, site SiteConfig, ttl time.Duration) error
}

type SiteResolver interface {
    Resolve(ctx context.Context, input ResolveSiteInput) (*SiteConfig, error)
}

type EventTokenVerifier interface {
    Verify(ctx context.Context, token string) (*TrackingClaims, error)
}

type RateLimiter interface {
    Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}
```

---

### 9.4 Adapters

Implementan los puertos.

Ejemplos:

```text
adapters/outbound/redis
adapters/outbound/postgres
adapters/outbound/jwt
adapters/outbound/httpresolver
adapters/inbound/http
adapters/inbound/redisstream
```

Los adaptadores pueden conocer librerías externas.

Permitido en adapters:

- go-redis.
- pgx.
- JWT library.
- net/http.
- slog.
- JSON encoding.
- Drivers.

---

### 9.5 Bootstrap

Responsable de:

- Leer `.env`.
- Crear conexiones.
- Instanciar adaptadores.
- Inyectar dependencias.
- Crear router HTTP.
- Crear consumer worker.

El bootstrap puede conocer todas las piezas porque es la capa de ensamblaje.

En Go Analytics, solo el bootstrap del worker debe crear la conexion real a PostgreSQL. La API de ingesta no abre conexion a la base ni ejecuta queries.

---

## 10. Responsabilidades por componente

### 10.1 Backend principal integrador

El backend principal no forma parte obligatoria de este repo, pero debe cumplir estos contratos:

1. Generar JWT firmado.
2. Hidratar Redis con metadata del site.
3. Exponer una URL interna para rehidratación.
4. Validar o renovar el token si el SDK lo necesita.
5. Mantener la relación real entre:
   - tenant
   - site
   - dominios permitidos
   - estado del site
   - token_version
   - tracking_enabled

---

### 10.2 SDK web

Responsabilidades:

```text
- Inicializarse con un tracking token.
- Enviar eventos al endpoint de ingesta.
- Generar event_id si no viene provisto.
- Generar anonymous_id persistente.
- Generar session_id.
- Hacer batching.
- Usar sendBeacon cuando sea posible.
- Usar fetch con keepalive como fallback.
- No romper nunca la aplicación host.
- No conocer secretos.
- No firmar tokens.
```

Ejemplo de uso futuro:

```ts
import { createAnalyticsClient } from "@go-analytics/web-sdk";

const analytics = createAnalyticsClient({
  token: trackingToken,
  endpoint: "https://analytics.midominio.com/v1/events",
  flushIntervalMs: 5000,
  batchSize: 10,
});

analytics.track("product_viewed", {
  product_id: "123",
  category: "calzado",
  price: 25000,
  currency: "ARS",
});
```

---

### 10.3 Go Ingestion API

Responsabilidades:

```text
- Recibir requests del SDK.
- Validar JWT.
- Validar claims mínimos.
- Validar tamaño del payload.
- Validar cantidad máxima de eventos por batch.
- Aplicar rate limit básico por site_code, IP o token.
- Publicar evento enriquecido en Redis Stream.
- Responder 204 rápidamente.
```

No debe:

```text
- Hacer consultas pesadas.
- Escribir directamente en la DB final.
- Hacer cálculos analíticos.
- Depender del backend principal para cada evento.
- Abrir conexiones PostgreSQL, usar `pgx` o ejecutar migraciones.
```

---

### 10.4 Go Worker

Responsabilidades:

```text
- Leer eventos desde Redis Stream.
- Validar metadata real del site contra Redis.
- Rehidratar metadata si falta.
- Validar dominio/origin contra allowed_domains.
- Validar token_version.
- Validar tracking_enabled.
- Deduplicar por event_id.
- Guardar eventos válidos en PostgreSQL.
- Registrar eventos rechazados o sospechosos.
- Confirmar procesamiento en Redis Stream.
- Abrir la conexion PostgreSQL mediante adaptadores outbound basados en `pgx/pgxpool`.
```

---

## 11. Decisión de seguridad para Fase 1

### 11.1 Token JWT firmado sin cifrado

En Fase 1 se usará JWT firmado, no cifrado.

El objetivo es:

```text
- Evitar manipulación del token.
- Transportar identidad tentativa del site.
- No exponer secretos.
- Mantener bajo el costo de implementación.
```

El token puede ser leído por el frontend, pero no puede ser modificado sin invalidar la firma.

---

### 11.2 Algoritmo recomendado

Para Fase 1 se aceptan dos opciones.

#### Opción simple inicial

```text
HS256
```

Ventaja:

```text
- Más simple.
- Backend principal y microservicio comparten un secreto.
```

Desventaja:

```text
- Ambos servicios conocen el mismo secreto.
```

#### Opción recomendada para repo público y futuro productivo

```text
RS256 o ES256
```

Ventaja:

```text
- El backend principal firma con clave privada.
- El microservicio valida con clave pública.
- El microservicio no necesita conocer la clave privada.
- Mejor separación entre proyectos.
```

Para Fase 1 se puede implementar primero HS256 por simplicidad, pero dejando preparada la interfaz `EventTokenVerifier` para soportar RS256/ES256 luego.

---

## 12. Claims del JWT

### Claims obligatorios

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

### Claims opcionales

```json
{
  "tenant_hint": "tenant_123",
  "site_hint": "site_456"
}
```

Recomendación:

```text
No exponer tenant_id y site_id internos como campos principales.
Usar site_code como identificador público.
Resolver tenant_id y site_id reales desde Redis.
```

---

## 13. Expiración del token

La expiración inicial será de 30 minutos.

Debe ser configurable por `.env`.

```env
JWT_EXPIRATION_MINUTES=30
```

El microservicio Go no genera el token, pero sí valida `exp`.

El backend principal debe generar el token respetando ese mismo valor o su propia configuración equivalente.

---

## 14. Variables de entorno

### 14.1 Variables comunes

```env
APP_ENV=development
APP_NAME=go-analytics
LOG_LEVEL=info
```

---

### 14.2 Ingestion API

```env
INGEST_HTTP_HOST=0.0.0.0
INGEST_HTTP_PORT=8080

JWT_ALGORITHM=HS256
JWT_SECRET=change_me_in_production
JWT_ISSUER=main-backend
JWT_AUDIENCE=analytics-ingest
JWT_EXPIRATION_MINUTES=30

REDIS_ADDR=redis:6379
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_DB=0

EVENT_STREAM_NAME=goanalytics:events:raw
EVENT_STREAM_MAXLEN=1000000

RATE_LIMIT_ENABLED=true
RATE_LIMIT_EVENTS_PER_MINUTE_PER_SITE=3000
RATE_LIMIT_EVENTS_PER_MINUTE_PER_IP=1000

MAX_EVENTS_PER_BATCH=50
MAX_EVENT_PAYLOAD_BYTES=65536
MAX_PROPERTY_KEYS=50
MAX_PROPERTY_DEPTH=5

CORS_ALLOWED_ORIGINS=*
```

---

### 14.3 Worker

```env
WORKER_NAME=analytics-worker-1
WORKER_CONSUMER_GROUP=analytics-workers
WORKER_BATCH_SIZE=500
WORKER_POLL_INTERVAL_MS=1000

REDIS_ADDR=redis:6379
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_DB=0

EVENT_STREAM_NAME=goanalytics:events:raw
REJECTED_STREAM_NAME=goanalytics:events:rejected
UNRESOLVED_STREAM_NAME=goanalytics:events:unresolved

POSTGRES_HOST=postgres_analytics
POSTGRES_PORT=5432
POSTGRES_USER=analytics
POSTGRES_PASSWORD=analytics
POSTGRES_DB=analytics
POSTGRES_SSLMODE=disable

SITE_RESOLVER_URL=http://main-backend/internal/analytics/sites/resolve
SITE_RESOLVER_TOKEN=change_me
SITE_RESOLVER_TIMEOUT_MS=300
SITE_REHYDRATE_COOLDOWN_SECONDS=300
SITE_NEGATIVE_CACHE_TTL_SECONDS=300

SITE_CACHE_TTL_SECONDS=3600
```

---

## 15. Estructura esperada en Redis

### 15.1 Metadata de site

Clave:

```text
goanalytics:site:public:{site_code}
```

Valor:

```json
{
  "site_code": "pub_site_abc123",
  "tenant_id": "tenant_123",
  "site_id": "site_456",
  "status": "active",
  "tracking_enabled": true,
  "allowed_domains": [
    "cliente.com",
    "www.cliente.com"
  ],
  "token_version": 1,
  "sample_rate": 1,
  "schema_version": 1,
  "updated_at": "2026-05-05T12:00:00Z"
}
```

---

### 15.2 Cooldown de rehidratación

```text
goanalytics:rehydrate:last_attempt:{site_code}
```

TTL:

```text
SITE_REHYDRATE_COOLDOWN_SECONDS
```

---

### 15.3 Negative cache

```text
goanalytics:site:not_found:{site_code}
```

TTL:

```text
SITE_NEGATIVE_CACHE_TTL_SECONDS
```

---

### 15.4 Rate limit

```text
goanalytics:ratelimit:site:{site_code}:{minute}
goanalytics:ratelimit:ip:{ip_hash}:{minute}
```

---

## 16. Contrato del endpoint de rehidratación

El microservicio no debe estar acoplado a FastAPI.

Debe llamar a una URL configurable:

```env
SITE_RESOLVER_URL=http://main-backend/internal/analytics/sites/resolve
```

### Request

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

### Response exitosa

```json
{
  "site_code": "pub_site_abc123",
  "tenant_id": "tenant_123",
  "site_id": "site_456",
  "status": "active",
  "tracking_enabled": true,
  "allowed_domains": [
    "cliente.com",
    "www.cliente.com"
  ],
  "token_version": 1,
  "sample_rate": 1,
  "schema_version": 1
}
```

### Response no encontrada

```json
{
  "error": "site_not_found"
}
```

---

## 17. Contrato del SDK hacia Ingestion API

### Endpoint

```http
POST /v1/events
Authorization: Bearer <tracking_jwt>
Content-Type: application/json
```

### Payload

```json
{
  "events": [
    {
      "event_id": "018f9b8e-0000-7000-a000-000000000001",
      "event_name": "page_view",
      "event_version": 1,
      "timestamp": "2026-05-05T12:00:00.000Z",
      "anonymous_id": "anon_abc",
      "session_id": "sess_xyz",
      "user_id": null,
      "origin": "https://cliente.com",
      "url": "https://cliente.com/productos/123",
      "path": "/productos/123",
      "referrer": "https://google.com",
      "properties": {
        "product_id": "123",
        "category": "calzado"
      },
      "context": {
        "device_type": "desktop",
        "language": "es-AR"
      }
    }
  ]
}
```

### Respuesta esperada

```http
204 No Content
```

También se puede usar:

```http
202 Accepted
```

Pero para analytics se recomienda `204` para mantener bajo overhead.

---

## 18. Modelo de evento interno

Después de validar el JWT, Go Ingestion debe enriquecer el evento con datos del token.

```json
{
  "event_id": "018f9b8e-0000-7000-a000-000000000001",
  "site_code": "pub_site_abc123",
  "env": "production",
  "token_version": 1,
  "jwt_id": "01HV...",
  "event_name": "page_view",
  "event_version": 1,
  "event_time": "2026-05-05T12:00:00.000Z",
  "received_at": "2026-05-05T12:00:01.000Z",
  "anonymous_id": "anon_abc",
  "session_id": "sess_xyz",
  "user_id": null,
  "origin": "https://cliente.com",
  "url": "https://cliente.com/productos/123",
  "path": "/productos/123",
  "referrer": "https://google.com",
  "user_agent": "...",
  "ip_hash": "...",
  "sdk_name": "av-analytics-js",
  "sdk_version": "0.1.0",
  "properties": {},
  "context": {}
}
```

---

## 19. Base de datos inicial

### 19.1 Tabla de eventos válidos

```sql
CREATE TABLE analytics_events (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL,

    tenant_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    site_code TEXT NOT NULL,

    env TEXT NOT NULL DEFAULT 'production',

    event_name TEXT NOT NULL,
    event_version INTEGER NOT NULL DEFAULT 1,

    event_time TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    anonymous_id TEXT,
    user_id TEXT,
    session_id TEXT,

    origin TEXT,
    url TEXT,
    path TEXT,
    referrer TEXT,

    user_agent TEXT,
    ip_hash TEXT,

    sdk_name TEXT,
    sdk_version TEXT,

    jwt_id TEXT,
    token_version INTEGER,

    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    context JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX ux_analytics_events_event_id
ON analytics_events(event_id);

CREATE INDEX ix_analytics_events_tenant_site_time
ON analytics_events(tenant_id, site_id, event_time DESC);

CREATE INDEX ix_analytics_events_name_time
ON analytics_events(event_name, event_time DESC);

CREATE INDEX ix_analytics_events_session
ON analytics_events(session_id);

CREATE INDEX ix_analytics_events_user
ON analytics_events(user_id);
```

---

### 19.2 Tabla de eventos rechazados o sospechosos

```sql
CREATE TABLE analytics_rejected_events (
    id BIGSERIAL PRIMARY KEY,

    event_id TEXT,
    site_code TEXT,
    env TEXT,

    reason TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'warning',

    origin TEXT,
    url TEXT,
    ip_hash TEXT,
    user_agent TEXT,

    raw_payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_analytics_rejected_events_site_time
ON analytics_rejected_events(site_code, created_at DESC);

CREATE INDEX ix_analytics_rejected_events_reason
ON analytics_rejected_events(reason);
```

---

## 20. Validaciones de seguridad

### 20.1 Validaciones en Ingestion API

```text
- JWT presente.
- Firma válida.
- Algoritmo permitido.
- aud correcto.
- iss permitido.
- exp vigente.
- nbf válido.
- site_code presente.
- token_version presente.
- event_name válido.
- Cantidad de eventos dentro del límite.
- Payload dentro del tamaño máximo.
- properties y context dentro del límite de profundidad.
- Bloqueo de claves sensibles.
```

Claves sensibles a bloquear:

```text
password
token
access_token
refresh_token
authorization
cookie
secret
private_key
credit_card
card_number
cvv
dni
document
```

---

### 20.2 Validaciones en Worker

```text
- site_code existe en Redis.
- Si no existe, intentar rehidratación.
- tenant_id y site_id reales existen en metadata.
- status = active.
- tracking_enabled = true.
- token_version del JWT coincide con Redis.
- origin/domain está en allowed_domains.
- event_id no fue procesado previamente.
```

---

## 21. Manejo de eventos no válidos

### 21.1 Evento inválido por payload

Ejemplos:

```text
- JSON inválido.
- event_name vacío.
- payload demasiado grande.
- properties con claves prohibidas.
```

Acción:

```text
Rechazar o registrar en goanalytics:events:rejected.
```

---

### 21.2 Evento con JWT inválido

Ejemplos:

```text
- Firma inválida.
- Token expirado.
- Audiencia incorrecta.
```

Acción:

```text
No publicar en stream principal.
Responder 204 o 401 según modo.
```

Para producción pública se recomienda responder `204` o `202` para no dar pistas, pero registrar internamente.

---

### 21.3 Evento con site no encontrado

Acción:

```text
1. Verificar negative cache.
2. Si no hay negative cache, llamar a resolver interno.
3. Si no existe, registrar como unresolved o rejected.
4. Crear negative cache temporal.
```

---

### 21.4 Evento con dominio no permitido

Acción:

```text
Registrar como suspicious/rejected.
No guardar como evento válido.
```

---

## 22. Fases de implementación

---

## Fase 0 — Diseño base y contratos

### Objetivo

Dejar definido el diseño del sistema antes de escribir la lógica principal.

### Tareas

```text
1. Crear repo go-analytics.
2. Definir estructura de carpetas.
3. Crear README inicial.
4. Crear .env.example.
5. Documentar arquitectura en docs/architecture.md.
6. Documentar arquitectura hexagonal en docs/hexagonal-architecture.md.
7. Documentar contrato del JWT en docs/jwt-contract.md.
8. Documentar contrato del evento en docs/event-contract.md.
9. Documentar contrato del resolver en docs/resolver-contract.md.
10. Definir convenciones de nombres de eventos.
```

### Criterios de aceptación

```text
- El repo explica claramente cómo se integran SDK, ingesta, worker, Redis y DB.
- Existe .env.example con todas las variables iniciales.
- Existe documentación de contrato para integradores futuros.
- La estructura separa domain, application, ports, adapters y bootstrap.
```

---

## Fase 1 — Núcleo hexagonal común de ingesta

### Objetivo

Crear el núcleo de dominio y aplicación del servicio de ingesta antes de implementar HTTP o Redis.

### Tareas

```text
1. Crear domain/event.
2. Crear domain/token.
3. Crear DTOs de aplicación.
4. Crear puerto inbound IngestEvents.
5. Crear puertos outbound EventPublisher, TokenVerifier, RateLimiter, Clock, Logger.
6. Crear caso de uso IngestEventsUseCase.
7. Crear tests unitarios del caso de uso con mocks/fakes.
```

### Criterios de aceptación

```text
- El caso de uso puede testearse sin HTTP.
- El caso de uso puede testearse sin Redis.
- El caso de uso puede testearse sin JWT real.
- La lógica principal queda aislada de infraestructura.
```

---

## Fase 2 — Microservicio Go de ingesta HTTP

### Objetivo

Implementar la API HTTP que recibe eventos, valida JWT y publica eventos crudos en Redis Stream.

### Tareas

```text
1. Crear módulo Go para services/ingest.
2. Implementar carga de configuración desde .env en bootstrap.
3. Implementar endpoint POST /v1/events.
4. Implementar middleware de request_id.
5. Implementar logs estructurados.
6. Implementar adaptador JWT HS256.
7. Implementar adaptador Redis Stream Publisher.
8. Implementar adaptador Redis RateLimiter.
9. Conectar adaptadores al caso de uso en bootstrap/container.go.
10. Responder 204 si el evento fue aceptado para procesamiento.
```

### Criterios de aceptación

```text
- POST /v1/events recibe batch de eventos.
- Rechaza internamente tokens inválidos.
- Publica eventos válidos en Redis Stream.
- No escribe directamente en DB.
- Devuelve respuesta rápida.
- Configuración completa desde .env.
- El handler HTTP no contiene lógica de negocio.
```

---

## Fase 3 — Núcleo hexagonal del worker

### Objetivo

Crear el núcleo del procesamiento de eventos desacoplado de Redis y PostgreSQL.

### Tareas

```text
1. Crear domain/site.
2. Crear domain/rejection.
3. Crear caso de uso ProcessEventsUseCase.
4. Crear caso de uso ValidateSiteUseCase.
5. Crear caso de uso RehydrateSiteUseCase.
6. Crear puertos outbound:
   - EventRepository.
   - RejectedEventRepository.
   - SiteCache.
   - SiteResolver.
   - Deduplicator.
   - Logger.
   - Clock.
7. Crear tests unitarios del worker con fakes.
```

### Criterios de aceptación

```text
- El procesamiento puede testearse sin Redis.
- El procesamiento puede testearse sin PostgreSQL.
- La validación de site/domain/token_version queda en application/domain.
- Los adaptadores solo implementan detalles técnicos.
```

---

## Fase 4 — Worker Go con Redis Stream y PostgreSQL

### Objetivo

Implementar el consumer que lee desde Redis Stream, valida contra Redis/backend y persiste eventos válidos.

### Tareas

```text
1. Implementar adaptador inbound Redis Stream Consumer.
2. Implementar lectura por batches.
3. Implementar adaptador SiteCache con Redis.
4. Implementar adaptador SiteResolver con HTTP.
5. Implementar cooldown de rehidratación.
6. Implementar negative cache.
7. Implementar adaptador PostgreSQL EventRepository con `pgx`.
8. Implementar adaptador PostgreSQL RejectedEventRepository con `pgx`.
9. Crear `pgxpool.Pool` en bootstrap del worker e inyectarlo en repositorios.
10. Implementar deduplicación por event_id.
11. Implementar XACK luego de persistencia exitosa.
```

### Criterios de aceptación

```text
- El worker procesa eventos desde Redis.
- Si el site existe y está activo, guarda en DB.
- Si falta cache, intenta rehidratar.
- Si el dominio no coincide, rechaza el evento.
- Si token_version no coincide, rechaza el evento.
- Los eventos inválidos no frenan el procesamiento.
```

---

## Fase 5 — Base de datos analytics

### Objetivo

Crear almacenamiento inicial en PostgreSQL separado de las bases tenant.

### Tareas

```text
1. Crear container postgres_analytics.
2. Crear migraciones administradas con `golang-migrate`.
3. Crear tabla analytics_events.
4. Crear tabla analytics_rejected_events.
5. Crear índices base.
6. Implementar repository en Go con `pgx`.
7. Implementar inserción batch.
8. Mantener PostgreSQL aislado en infraestructura para permitir ClickHouse u otra base futura.
```

### Criterios de aceptación

```text
- PostgreSQL analytics corre en container separado.
- Las migraciones se ejecutan correctamente.
- El worker guarda eventos válidos.
- Existe separación clara respecto a las DB tenant.
- El nucleo de dominio y aplicacion no importa `pgx`, `pgxpool` ni SQL.
```

---

## Fase 6 — SDK TypeScript preparado para npm

### Objetivo

Crear el SDK web como paquete independiente y publicable.

### Tareas

```text
1. Crear packages/web-sdk.
2. Configurar TypeScript.
3. Configurar build para ESM.
4. Preparar package.json para publicación npm.
5. Implementar createAnalyticsClient.
6. Implementar track.
7. Implementar page.
8. Implementar identify opcional.
9. Implementar queue interna.
10. Implementar batching.
11. Implementar flushIntervalMs.
12. Implementar sendBeacon.
13. Implementar fallback con fetch keepalive.
14. Implementar anonymous_id persistente.
15. Implementar session_id.
16. Exportar tipos TypeScript.
17. Documentar integración.
```

### Criterios de aceptación

```text
- El SDK se puede importar desde TypeScript.
- El SDK no depende del backend principal.
- El SDK no conoce secretos.
- El SDK envía Authorization: Bearer <tracking_jwt>.
- El SDK puede generar eventos genéricos.
- El package queda listo para publicarse luego en npm.
```

---

## Fase 7 — Docker y entorno local

### Objetivo

Levantar todo el sistema localmente con Docker Compose.

### Servicios mínimos

```text
- analytics-ingest
- analytics-worker
- redis
- postgres_analytics
```

### Tareas

```text
1. Crear Dockerfile para ingest.
2. Crear Dockerfile para worker.
3. Crear docker-compose.yml.
4. Agregar healthchecks.
5. Agregar volúmenes para Redis y PostgreSQL.
6. Crear Makefile con comandos útiles.
```

### Comandos sugeridos

```bash
make up
make down
make logs
make test
make migrate-up
make migrate-down
```

### Criterios de aceptación

```text
- El entorno local levanta con un solo comando.
- Redis queda accesible para ingest y worker.
- PostgreSQL analytics queda separado.
- Ingest y worker cargan configuración desde .env.
```

---

## Fase 8 — Integración con backend principal

### Objetivo

Conectar un backend real, inicialmente FastAPI, sin acoplar el microservicio a FastAPI.

### Tareas

```text
1. Implementar generación de JWT en FastAPI.
2. Implementar endpoint para entregar tracking token al frontend.
3. Implementar hidratación de Redis desde FastAPI.
4. Implementar resolver interno compatible con contrato genérico.
5. Configurar SITE_RESOLVER_URL en worker.
6. Probar rehidratación automática.
```

### Criterios de aceptación

```text
- El backend principal genera token válido.
- El SDK usa ese token.
- Go Ingest valida el token.
- Worker valida metadata real desde Redis.
- Si Redis no tiene metadata, worker rehidrata desde backend.
```

---

## Fase 9 — Seguridad y hardening

### Objetivo

Reducir superficie de ataque y abuso externo.

### Tareas

```text
1. Validar CORS.
2. Validar Origin/Referer.
3. Implementar rate limit por site_code.
4. Implementar rate limit por IP.
5. Implementar límite de tamaño de payload.
6. Implementar límite de eventos por batch.
7. Implementar bloqueo de keys sensibles.
8. Implementar logs de eventos sospechosos.
9. Implementar token_version.
10. Implementar rotación manual de token_version.
11. Implementar hash de IP.
12. Evitar guardar IP cruda.
```

### Criterios de aceptación

```text
- Un token vencido no es aceptado.
- Un token con aud inválido no es aceptado.
- Un dominio no permitido se marca como sospechoso.
- Un site deshabilitado no registra eventos válidos.
- El rate limit evita abuso básico.
```

---

## Fase 10 — Observabilidad

### Objetivo

Poder monitorear estado, errores y volumen de eventos.

### Métricas iniciales

```text
- eventos recibidos por minuto
- eventos aceptados
- eventos rechazados
- eventos unresolved
- eventos guardados
- errores de JWT
- errores de Redis
- errores de DB
- latencia de ingesta
- lag del stream
- cantidad de mensajes pendientes
```

### Tareas

```text
1. Agregar endpoint GET /health.
2. Agregar endpoint GET /ready.
3. Agregar logs estructurados.
4. Agregar métricas básicas.
5. Preparar integración futura con Prometheus.
```

### Criterios de aceptación

```text
- Se puede verificar si ingest está vivo.
- Se puede verificar si worker conecta a Redis y DB.
- Los logs permiten rastrear errores por request_id.
```

---

## Fase 11 — Preparación para producción

### Objetivo

Dejar el sistema listo para despliegue inicial.

### Tareas

```text
1. Separar .env de development y production.
2. Configurar HTTPS detrás de proxy.
3. Configurar CORS de producción.
4. Configurar secretos seguros.
5. Configurar Redis con contraseña.
6. Configurar PostgreSQL con usuario restringido.
7. Configurar backups de PostgreSQL analytics.
8. Configurar límites de recursos Docker.
9. Configurar restart policies.
10. Configurar logs persistentes o centralizados.
```

### Criterios de aceptación

```text
- El sistema no depende de valores hardcodeados.
- No hay secretos en el repo.
- Los servicios reinician automáticamente.
- La DB analytics tiene volumen persistente.
- Redis tiene configuración controlada.
```

---

## Fase 12 — Publicación futura del SDK en npm

### Objetivo

Dejar el SDK preparado para publicación pública.

### Tareas

```text
1. Definir nombre del paquete.
2. Completar package.json.
3. Agregar README específico del SDK.
4. Agregar licencia.
5. Agregar examples.
6. Agregar build ESM.
7. Agregar generación de .d.ts.
8. Agregar npm ignore o files.
9. Agregar versionado semántico.
10. Preparar GitHub Actions para publish futuro.
```

### Package sugerido

```json
{
  "name": "@go-analytics/web-sdk",
  "version": "0.1.0",
  "description": "Generic web analytics SDK for event tracking",
  "type": "module",
  "main": "./dist/index.cjs",
  "module": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "files": [
    "dist",
    "README.md",
    "LICENSE"
  ],
  "keywords": [
    "analytics",
    "events",
    "tracking",
    "sdk"
  ]
}
```

### Criterios de aceptación

```text
- El SDK puede instalarse localmente como paquete.
- El SDK puede compilarse sin depender del microservicio.
- El SDK tiene tipos TypeScript.
- El SDK está listo para publicarse en npm cuando se decida.
```

---

## 23. Evolución futura

### Fase futura A — Migración de Redis Stream a Kafka/Redpanda

El diseño debe permitir reemplazar el backend de cola.

Crear interfaces:

```text
EventPublisher
EventConsumer
```

Implementaciones:

```text
RedisStreamPublisher
RedisStreamConsumer
KafkaPublisher
KafkaConsumer
RedpandaPublisher
RedpandaConsumer
```

---

### Fase futura B — Migración de PostgreSQL analytics a ClickHouse

Mantener desde el inicio un modelo de datos compatible:

```text
columnas fijas para filtros frecuentes
properties JSON para campos flexibles
context JSON para entorno
event_version para migración de eventos
```

---

### Fase futura C — SDK browser script

Además del paquete npm, generar un script:

```html
<script
  src="https://analytics.midominio.com/sdk/v1/av-analytics.js"
  data-token="...">
</script>
```

Esto permitiría integrarlo en sitios donde no hay build npm.

---

### Fase futura D — Separar librerías internas compartidas

Si ingest y worker comienzan a duplicar demasiado código, extraer un módulo Go compartido dentro del mismo repo:

```text
services/
  shared/
    domain/
    application/
    ports/
```

No hacerlo prematuramente en Fase 1 si complica el desarrollo.

---

## 24. Criterios globales de aceptación

El proyecto se considera correctamente estructurado cuando:

```text
1. Existe repo independiente.
2. El microservicio Go recibe eventos.
3. El JWT se valida correctamente.
4. La expiración de 30 minutos es configurable desde .env.
5. Redis se usa para cache de site, rate limit y stream inicial.
6. Worker valida contra Redis antes de guardar.
7. Worker puede rehidratar metadata desde URL configurable.
8. PostgreSQL analytics está separado.
9. SDK está en carpeta propia y preparado para npm.
10. No hay secretos hardcodeados.
11. Todo puede levantarse con Docker Compose.
12. El diseño permite migrar luego a Kafka/Redpanda y ClickHouse.
13. La lógica de negocio no depende de Redis, PostgreSQL, HTTP ni JWT concreto.
14. Los casos de uso principales tienen tests unitarios con mocks/fakes.
15. Los adaptadores pueden reemplazarse sin reescribir el dominio.
```

---

## 25. Prioridad recomendada de desarrollo

```text
1. Repo y documentación base.
2. Estructura hexagonal.
3. Contrato de eventos.
4. Contrato JWT.
5. Núcleo de dominio y aplicación de ingesta.
6. Ingestion API HTTP.
7. Redis Stream Publisher.
8. Núcleo de dominio y aplicación del worker.
9. Worker consumer.
10. PostgreSQL analytics con adaptadores `pgx/pgxpool` en el worker.
11. SDK mínimo.
12. Integración con backend principal.
13. Seguridad/rate limit.
14. Docker producción.
15. Preparación npm.
```

---

## 26. Decisión final de Fase 1

La primera versión completa del sistema debe implementar:

```text
- Arquitectura hexagonal desde el inicio.
- JWT firmado sin cifrado mediante adaptador de infraestructura.
- Expiración inicial de 30 minutos configurable.
- site_code como identificador público.
- Redis como cache de validación en fases de infraestructura.
- Redis Stream como cola inicial en fases de infraestructura.
- Worker Go para persistencia en fases de worker.
- PostgreSQL analytics separado en fases de base de datos.
- SDK TypeScript preparado para npm en la fase dedicada del SDK.
- URL de rehidratación configurable.
- Credenciales de Redis y DB desde .env.
```

No debe implementar todavía:

```text
- Kafka/Redpanda obligatorio.
- ClickHouse obligatorio.
- JWE/cifrado.
- Dashboard analítico avanzado.
- Motor complejo de schemas.
```

La meta es dejar una base sólida, productiva, testeable y evolutiva, sin sobredimensionar la primera versión.
