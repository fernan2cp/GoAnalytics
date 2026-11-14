# Migraciones al desplegar

La imagen ejecuta `migrate-up.sh`, construye la URL desde el entorno del propio
contenedor y aplica exclusivamente `up`. No necesita `--env-file` de Compose
para interpolar credenciales ni hereda variables de otros contenedores.

## Producción con dc-msa.yml

1. Actualizar el checkout de GoAnalytics que se usa como contexto de construcción.
2. Respaldar PostgreSQL y el compose antes de activar las migraciones automáticas.
3. Verificar las variables en los archivos `ve_go_analytics.env` y
   `ve_go_analytics_share.env`: `POSTGRES_HOST`, `POSTGRES_USER`,
   `POSTGRES_PASSWORD`, `POSTGRES_DB`; opcionales `POSTGRES_PORT` (5432) y
   `POSTGRES_SSLMODE` (disable, igual que el worker; usar TLS según el entorno).
   El segundo archivo prevalece sobre el primero. No versionar secretos.
4. Conservar el `dc-msa.yml` externo y combinarlo con el overlay versionado:

```bash
docker compose -f "${PROJECT_ROOT}/produccion/dc-msa.yml" \
  -f "${PROJECT_ROOT}/dockers/msa/go-analytics/deploy/migrations/dc-msa.migrate.yml" \
  config --quiet

docker compose -f "${PROJECT_ROOT}/produccion/dc-msa.yml" \
  -f "${PROJECT_ROOT}/dockers/msa/go-analytics/deploy/migrations/dc-msa.migrate.yml" \
  up -d --build
```

`PROJECT_ROOT` debe seguir disponible para Compose como en el despliegue actual;
no es una credencial. PostgreSQL debe estar iniciado y accesible desde `av_net`.
Usar Docker Compose v2, no `docker stack deploy`. El overlay presupone los cuatro
servicios del compose de producción actualizado. También se pueden incorporar
sus bloques directamente al compose externo, sin usar el segundo `-f`.

El migrador termina con código 0 si está actualizado. Los servicios esperan
`service_completed_successfully`; no usar `--no-deps`. Esto ordena el arranque,
pero no detiene servicios que ya estén ejecutándose: para una actualización con
cambios de esquema, detener primero los cuatro procesos durante la ventana de
mantenimiento. Un `docker compose build` solo construye imágenes, no migra datos.

Los errores conservan el código de salida, pero se omite la salida del proveedor
para no filtrar credenciales. Revisar conectividad, permisos y
`SELECT version, dirty FROM schema_migrations;` en una sesión administrativa
segura. No aplicar `force` ni `down` automáticamente. Las migraciones de
DragonFullAV/Alembic son independientes y no se ejecutan con este servicio.

## Sandbox y pruebas

El compose raíz mantiene el perfil opt-in `tools` y usa `.env.example`, solo para
el sandbox. No usar sus credenciales ni `make migrate-down` en producción.

```bash
docker compose --profile tools run --rm --build migrate
docker build -f deploy/migrations/Dockerfile -t goanalytics-migrate:test .
RUN_MIGRATE_CONTAINER_TESTS=1 python -m unittest discover -s deploy/migrations -p 'test_*.py'
```

Las pruebas sustituyen únicamente el comando de migración dentro del contenedor
para verificar codificación URL, validación, propagación de errores y ausencia
de secretos en logs. No utilizan bases reales.

Validación adicional realizada: construcción de imagen, tres pruebas de script,
configuración del compose raíz y aplicación real hasta `008` en PostgreSQL 16
temporal con contraseña que contiene caracteres especiales. Una segunda
ejecución terminó correctamente sin cambios (`version=8`, `dirty=false`). El
PostgreSQL temporal y su volumen se eliminaron tras la prueba. El overlay debe
validarse también en el servidor con sus rutas y archivos de entorno reales.
