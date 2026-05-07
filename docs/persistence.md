# Persistencia y base de datos

Go Analytics usa PostgreSQL como almacenamiento inicial de eventos procesados. La integracion con la base debe respetar arquitectura hexagonal: el nucleo de dominio y aplicacion no conoce PostgreSQL, `pgx`, pools de conexiones, SQL ni migraciones.

## Decision tecnica

- Base inicial: PostgreSQL.
- Driver Go: `pgx`.
- Pool de conexiones: `pgxpool`.
- Migraciones: `golang-migrate`.
- Ubicacion de migraciones: `migrations/postgres`.
- Adaptadores PostgreSQL: solo en infraestructura del worker.

## Responsabilidades por servicio

La API de ingesta no escribe directamente en PostgreSQL. Su tarea es recibir eventos, validar JWT, aplicar reglas basicas, enriquecer el evento crudo y publicarlo en Redis Stream.

El worker es el unico componente que debe abrir conexion real a PostgreSQL. Consume eventos desde Redis Stream, valida metadata real del site, aplica reglas de procesamiento, deduplica y persiste eventos validos o rechazados.

## Arquitectura hexagonal

La aplicacion trabaja contra interfaces:

```text
ProcessEventsUseCase -> EventRepository
ProcessEventsUseCase -> RejectedEventRepository
```

Los repositorios PostgreSQL implementan esos puertos desde adaptadores outbound. Deben encapsular:

- `pgxpool.Pool`.
- Queries SQL.
- Transacciones o batches.
- Conversion entre tipos de dominio/DTO y filas SQL.
- Errores propios de PostgreSQL.

`domain` y `application` no deben importar `pgx`, `pgxpool`, drivers SQL, variables de entorno ni archivos de migracion.

## Migraciones

Las migraciones viven fuera del nucleo y se ejecutan como tarea de infraestructura con `golang-migrate`.

Las migraciones deben crear y evolucionar el esquema `analytics_events` y `analytics_rejected_events` sin introducir logica de negocio. Los cambios de esquema deben ser compatibles con los repositorios del worker.

Los archivos usan el formato esperado por `golang-migrate`:

```text
migrations/postgres/001_create_analytics_events.up.sql
migrations/postgres/001_create_analytics_events.down.sql
migrations/postgres/002_create_rejected_events.up.sql
migrations/postgres/002_create_rejected_events.down.sql
migrations/postgres/003_event_idempotency_sequence.up.sql
migrations/postgres/003_event_idempotency_sequence.down.sql
```

En desarrollo local, las migraciones se ejecutan con:

```bash
make migrate-up
```

Para revertir la ultima migracion aplicada:

```bash
make migrate-down
```

## Preparacion para cambios futuros

PostgreSQL es el adaptador inicial, no una dependencia del nucleo. Si mas adelante se reemplaza por ClickHouse u otra base, el cambio esperado es crear nuevos adaptadores que implementen los mismos puertos y ajustar bootstrap, sin reescribir dominio ni casos de uso.
