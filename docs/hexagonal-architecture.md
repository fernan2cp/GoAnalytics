# Arquitectura hexagonal

Go Analytics usa Ports & Adapters desde la estructura inicial.

```text
adapters -> application -> domain
```

## Reglas

- `domain` contiene entidades, value objects y reglas puras.
- `application` contiene casos de uso, DTOs y puertos.
- `adapters` implementa detalles tecnicos como HTTP, Redis, PostgreSQL y JWT.
- `bootstrap` lee configuracion, crea conexiones e inyecta dependencias.
- La persistencia PostgreSQL se implementa como adaptador outbound del worker, con `pgx/pgxpool` y migraciones fuera del nucleo.

## Prohibido

- `domain` importando Redis, PostgreSQL, HTTP o JWT.
- `application` importando drivers o variables de entorno.
- `application` importando `pgx`, `pgxpool` o queries SQL.
- La API de ingesta escribiendo directamente en PostgreSQL.
- Handlers HTTP con logica de negocio.
- Adaptadores definiendo reglas de aceptacion de eventos.
