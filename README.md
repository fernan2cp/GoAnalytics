# Go Analytics

Go Analytics es un proyecto de analítica de eventos web multitenant de alto rendimiento, estructurado como un monorepo con servicios independientes en Go, un SDK en TypeScript y contratos documentados para integrarse con cualquier backend principal.

La arquitectura del proyecto prioriza el diseño hexagonal: la lógica de dominio y aplicación es completamente pura y desacoplada de detalles técnicos externos como HTTP, Redis, PostgreSQL, JWT o variables de entorno, los cuales se implementan a través de adaptadores.

---

## Funcionalidades Principales

- **Ingesta de Eventos Multitenant:** API HTTP optimizada para la recepción de batches de eventos firmados con tokens JWT.
- **Procesamiento Asíncrono en Tiempo Real:** Worker de procesamiento que consume eventos desde un Redis Stream.
- **Captura y Normalización de Ítems para Scoring:**
  - Captura estructurada de interacciones con ítems (impresiones, vistas, zooms de imagen, adiciones a carrito, checkouts y compras).
  - Normalización en la tabla relacional 1:N `analytics_event_items` con soporte de índices avanzados (incluyendo índices GIN para categorías) para agregaciones eficientes.
  - Almacenamiento opcional de cabeceras de órdenes en `analytics_event_orders`.
- **Deduplicación Avanzada:** Reglas de deduplicación exacta (por `event_id`), lógica (por `logical_event_id`), semántica e idempotente por tipo de evento (por ejemplo, deduplicación de impresiones por sesión y superficie, o compras por ID de orden y línea).
- **Hardening y Hard-limit:** Limitadores de tasa (Rate Limiting) basados en Redis por IP y por sitio, validaciones estrictas y exclusión de claves sensibles.

---

## Requisitos del Sistema

Para poder utilizar y desarrollar en este proyecto, asegúrate de cumplir con los siguientes requisitos:

### Con Docker (Recomendado)
- **Docker Engine:** Versión 20.10 o superior.
- **Docker Compose:** Versión v2.0 o superior.

### Para Desarrollo y Pruebas Locales (Sin Docker)
- **Go:** Versión 1.22 o superior (para los servicios `ingest` y `worker`).
- **Node.js:** Versión 18 o superior (para compilar y probar el `packages/web-sdk`).
- **PostgreSQL:** Versión 16 o superior (con soporte para arreglos y tipo de datos `JSONB`).
- **Redis:** Versión 7 o superior (con soporte para Streams).
- **Make:** Opcional (recomendado para facilitar la ejecución de comandos comunes).

---

## Estructura del Proyecto

```text
services/
  ingest/        API Go de ingesta de eventos (servidor HTTP / Redis Stream)
  worker/        Worker Go de procesamiento y persistencia (PostgreSQL)
packages/
  web-sdk/       SDK TypeScript cliente preparado para distribución npm
migrations/
  postgres/      Migraciones SQL (golang-migrate) de la base de datos de analítica
docs/            Arquitectura, convenciones de eventos, contratos y especificaciones (SDD)
sandbox/         Entorno interactivo local de pruebas rápidas y demostración
IMPLEMENTACION.md Registro detallado del estado de fases de desarrollo
```

---

## Aseguramiento de las Migraciones

Las migraciones de la base de datos PostgreSQL se gestionan mediante `golang-migrate` y aplican automáticamente el esquema inicial y las nuevas tablas para ítems/órdenes.

Ej:
```bash
migrate -path migrations/postgres -database "postgres://user:pass@localhost:5432/go_analytics?sslmode=disable" up
```

### 1. Ejecución Automática (Recomendado)
Al levantar el entorno sandbox mediante los scripts provistos, la base de datos se migrará automáticamente a la última versión:
```bash
# En sistemas basados en Unix (Linux / macOS):
./init-sandbox.sh

# En Windows (PowerShell):
.\init-sandbox.ps1
```

### 2. Ejecución Manual con Make
Puedes controlar las migraciones de forma manual utilizando las herramientas del `Makefile`:
```bash
# Aplicar todas las migraciones pendientes
make migrate-up

# Revertir la última migración aplicada (paso a paso)
make migrate-down

# Forzar una versión específica en caso de conflicto en el esquema
make migrate-force VERSION=<version_number>
```

### 3. Ejecución Directa con Docker Compose
Si no dispones de `make`, ejecuta la herramienta de migración directamente a través de Docker Compose activando el perfil `tools`:
```bash
docker compose --profile tools run --rm migrate -path=/migrations -database "postgres://analytics:analytics@postgres_analytics:5432/analytics?sslmode=disable" up
```

---

## Comandos Útiles

```bash
make test             # Ejecuta la suite de pruebas unitarias en Ingest y Worker
make tidy             # Ejecuta go mod tidy en ambos servicios y sincroniza el go.work
make up               # Inicia los servicios base en segundo plano (ingest, worker, db, redis)
make down             # Detiene y remueve todos los contenedores del entorno local
make logs-follow      # Muestra los logs en tiempo real de los servicios
```

---

## Sandbox (Demostración)

El sandbox es un entorno interactivo web premium diseñado para ver el flujo completo en tiempo real (SDK -> Ingest -> Redis Stream -> Worker -> Postgres):

1. Inicializa el sandbox con `./init-sandbox.sh` o `.\init-sandbox.ps1`.
2. Accede a la interfaz interactiva en [http://localhost:3000](http://localhost:3000).
3. Podrás interactuar enviando eventos de prueba y observar cómo se validan, deduplican y persisten inmediatamente en PostgreSQL.

Consulta la [Guía del Sandbox](docs/sandbox/README.md) para más información.

---

## Documentación Detallada

La documentación técnica y de diseño se organiza en la carpeta `docs/`:

- **Arquitectura y Diseño Hexagonal:** [docs/architecture.md](docs/architecture.md) y [docs/hexagonal-architecture.md](docs/hexagonal-architecture.md)
- **Captura de Eventos de Ítems (SDD):** [docs/next_implementation/_complete/item_event_capture/item_event_capture_design.md](docs/next_implementation/_complete/item_event_capture/item_event_capture_design.md) y la guía detallada [docs/next_implementation/_complete/item_event_capture/item_event_capture_sdd/README.md](docs/next_implementation/_complete/item_event_capture/item_event_capture_sdd/README.md)
- **Convenciones y Contratos de Eventos:** [docs/event-conventions.md](docs/event-conventions.md) y [docs/event-contract.md](docs/event-contract.md)
- **Configuración y Seguridad:** [docs/env.md](docs/env.md) y [docs/jwt-contract.md](docs/jwt-contract.md)
- **Persistencia de Datos:** [docs/persistence.md](docs/persistence.md)

