# Instructivo resumido de configuración del MVP

Este instructivo resume la configuración necesaria para activar las estadísticas de clientes en producción. La capacidad es opcional: si los flags permanecen en `false`, el flujo web existente continúa funcionando sin cambios.

## 1. Prerrequisitos

- GoAnalytics y DragonFullAV deben poder resolverse y conectarse por HTTP desde sus respectivos procesos.
- GoAnalytics debe tener PostgreSQL con las migraciones hasta la versión `008` y Redis con el stream configurado.
- Redis debe soportar `XAUTOCLAIM` (versión 6.2 o posterior). No es necesario cambiar la versión de PostgreSQL únicamente por este MVP; usar la versión soportada por el despliegue existente.
- Reconstruir las imágenes de worker, jobs y query desde el Dockerfile actualizado: ahora instala `tzdata` para resolver zonas IANA como `America/Argentina/Buenos_Aires`.
- DragonFullAV debe tener la revisión tenant `custanalytics01` aplicada antes de habilitar snapshot o emisión. La captura SQL queda apagada por defecto: activar en cada tenant que publique el snapshot con `SELECT analytics_customer_lifecycle_configure(true);`. La función sincroniza el estado inicial y conserva versiones monotónicas; no ejecutarla sobre un tenant ajeno al rollout.
- El snapshot MVP conserva sus revisiones en memoria durante dos horas: usar un único worker para ese endpoint o afinidad de instancia durante toda la paginación. Tras reiniciar la fuente, un corte expirado requiere un bootstrap nuevo explícito.
- El proyecto principal debe exponer los endpoints internos de snapshot y registro de contextos.
- Los tenants habilitados deben tener sujetos disponibles en el snapshot.
- El endpoint de resolución de sites debe responder dentro del timeout configurado. En desarrollo se observó una latencia cercana a 4,7 s; por eso el entorno usa `SITE_RESOLVER_TIMEOUT_MS=10000`. En producción medir la latencia real y definir el valor con margen, sin ocultar una degradación persistente del resolver.
- Deben generarse cuatro secretos independientes. No reutilizar el secreto JWT, el token del resolver de sites ni credenciales de base de datos.

Los archivos `.env`, `.env.devel` y cualquier archivo que contenga secretos no deben versionarse.

Generar cuatro tokens independientes con `openssl rand -hex 32` (una ejecución por token) e inyectarlos mediante el gestor de secretos. Eventos, snapshot, registro y query usan respectivamente `SUBJECT_EVENTS_API_TOKEN`, `SUBJECT_SOURCE_TOKEN`, `SUBJECT_CONTEXT_REGISTRY_TOKEN` y `SUBJECT_QUERY_API_TOKEN`; cada par emisor/receptor debe recibir el mismo valor para su capacidad.

## 2. Variables de GoAnalytics

Configurar en el entorno del ingest, worker, `subject-jobs` y `query-api`:

```dotenv
SUBJECT_INTERNAL_EVENTS_ENABLED=true
SUBJECT_EVENTS_API_TOKEN=<token-eventos-internos>
SUBJECT_ANALYTICS_ENABLED=true
SUBJECT_BOOTSTRAP_ENABLED=true
SUBJECT_PROJECTION_ENABLED=true
SUBJECT_CONTEXT_REGISTRY_ENABLED=true
SUBJECT_QUERY_API_ENABLED=true

SUBJECT_SNAPSHOT_URL=https://<dragon-host>/api/v1/internal/analytics/subjects/snapshot
SUBJECT_SOURCE_TOKEN=<token-snapshot>
SUBJECT_CONTEXT_REGISTRY_URL=https://<dragon-host>/api/v1/internal/analytics/context-registry
SUBJECT_CONTEXT_REGISTRY_TOKEN=<token-registro>
SUBJECT_QUERY_API_TOKEN=<token-query>

SUBJECT_JOBS_HEALTH_HTTP_HOST=0.0.0.0
SUBJECT_JOBS_HEALTH_HTTP_PORT=8080
SUBJECT_QUERY_HTTP_HOST=0.0.0.0
SUBJECT_QUERY_HTTP_PORT=8080
```

En despliegues separados, publicar el query API detrás de una red interna y configurar en DragonFullAV la URL resultante. No exponerlo a Internet.

Entre contenedores de la misma red usar DNS del servicio y puerto interno (`http://go-ingest:8080`, `http://go-query-api:8080`, `http://back:8000`). Desde el host usar los puertos publicados; desde Docker hacia un backend en el host usar un nombre o IP alcanzable desde Docker, no `localhost`. Los puertos locales 8082 (query) y 8083 (jobs) son ejemplos de desarrollo; producción debe configurar sus propias URLs privadas o proxies TLS.

## 3. Variables de DragonFullAV

Configurar en el entorno efectivo del backend:

```dotenv
SUBJECT_ANALYTICS_EMISSION_ENABLED=true
GO_ANALYTICS_INTERNAL_EVENTS_URL=https://<go-analytics-host>/v1/internal/events
SUBJECT_EVENTS_API_TOKEN=<token-eventos-internos>

SUBJECT_SNAPSHOT_ENABLED=true
SUBJECT_SOURCE_TOKEN=<token-snapshot>
SUBJECT_CONTEXT_REGISTRY_ENABLED=true
SUBJECT_CONTEXT_REGISTRY_TOKEN=<token-registro>

SUBJECT_ANALYTICS_SUGGESTIONS_ENABLED=true
GO_ANALYTICS_SUGGESTIONS_URL=https://<go-analytics-host>/v1/internal/subjects/suggestions
SUBJECT_QUERY_API_TOKEN=<token-query>
SUBJECT_ANALYTICS_SUGGESTION_TENANTS=<tenant-1>,<tenant-2>
```

La lista `SUBJECT_ANALYTICS_SUGGESTION_TENANTS` debe ser explícita en producción. Una lista vacía habilita todos los tenants y solo debe usarse si existe una decisión operativa documentada.

## 4. Procesos y orden de activación

1. Aplicar las migraciones aditivas hasta `008`.
   Aplicar `custanalytics01` en los tenants fuente y activar su captura SQL explícitamente antes de habilitar los endpoints. Los flags de aplicación siguen apagados hasta completar esta preparación.
2. Crear o verificar el stream Redis y el grupo del worker.
3. Levantar ingest y worker.
4. Levantar `subject-jobs` en modo `serve` y `query-api`.
5. Verificar snapshot y registro de contextos con sus Bearer dedicados.
6. Ejecutar una vez `subject-jobs --mode refresh-registry`.
7. Ejecutar `subject-jobs --mode bootstrap`.
8. Ejecutar `subject-jobs --mode resolve-initial-contexts`.
9. Esperar o ejecutar la primera proyección global y contextual.
10. Activar la emisión de asociaciones/lifecycle en DragonFullAV.
11. Activar el consumo de sugerencias para los tenants habilitados.

En Docker Compose de desarrollo, los servicios equivalentes son `analytics-subject-jobs-devel` y `analytics-query-api-devel`. En producción deben conservar el mismo contrato de procesos, healthchecks y variables, aunque la orquestación sea distinta.

`docker-compose-devel.yml` es local y está ignorado por Git: sus cambios no llegan al servidor mediante el merge. Actualizar el compose efectivo del servidor agregando jobs (`build.args.WORKER_COMMAND=subject-jobs`, comando `--mode serve`) y query (`WORKER_COMMAND=query-api`), ambos con `services/worker/Dockerfile`, el entorno real y la misma red privada. El `docker-compose.yml` versionado de la raíz es un sandbox con `.env.example` y no incluye estos procesos nuevos.

Para ejecutar un job puntual sin colisionar con la IP fija del servicio de desarrollo ya iniciado:

```powershell
docker compose -f docker-compose-devel.yml exec analytics-subject-jobs-devel /app/go-analytics-worker --mode refresh-registry
docker compose -f docker-compose-devel.yml exec analytics-subject-jobs-devel /app/go-analytics-worker --mode bootstrap
docker compose -f docker-compose-devel.yml exec analytics-subject-jobs-devel /app/go-analytics-worker --mode resolve-initial-contexts
docker compose -f docker-compose-devel.yml exec analytics-subject-jobs-devel /app/go-analytics-worker --mode project-global
docker compose -f docker-compose-devel.yml exec analytics-subject-jobs-devel /app/go-analytics-worker --mode project-context
```

Un bootstrap completado es no-op. Un fallo de red reanuda el checkpoint. Solo ante una revisión expirada o un nuevo corte intencional agregar `--force-new-snapshot` al modo `bootstrap`. No usar `compose run` simultáneamente con el servicio de IP fija: puede colisionar aunque no publique puertos.

## 5. Smoke tests

Comprobar salud y autenticación:

```bash
curl -fsS https://<go-analytics-host>/health
curl -fsS https://<go-analytics-host>:8083/ready
curl -fsS https://<go-analytics-host>:8082/ready

curl -i https://<dragon-host>/api/v1/internal/analytics/subjects/snapshot
curl -i -H "Authorization: Bearer ${SUBJECT_SOURCE_TOKEN}" \
  'https://<dragon-host>/api/v1/internal/analytics/subjects/snapshot?subject_type=customer&limit=10'

curl -i -H "Authorization: Bearer ${SUBJECT_QUERY_API_TOKEN}" \
  'https://<go-analytics-host>:8082/v1/internal/subjects/suggestions?tenant_id=<tenant>&subject_type=customer&context_key=search&context_version=1&limit=10'
```

El primer snapshot sin Bearer debe devolver `401`. Un snapshot válido debe devolver `200`; una configuración desactivada debe devolver `404`.

Después de crear una asociación real en el proyecto principal, verificar:

```sql
SELECT count(*) FROM analytics_events WHERE event_source = 'server';
SELECT count(*) FROM analytics_subjects;
SELECT count(*) FROM analytics_event_subjects;
SELECT projection_name, status, completed_at
FROM analytics_projection_runs
ORDER BY id DESC
LIMIT 10;
```

Los logs esperados son una emisión aceptada en DragonFullAV, un evento `event_source=server` aceptado por ingest y una normalización persistida por el worker.

### Contrato corregido y verificado

El worker normaliza la versión ausente del perfil server a `1` al consultar el resolver. El payload interno no admite `token_version`: no agregar ese campo desde el emisor. Los cooldowns web/server están separados y la caché de contextos conserva IDs técnicos. La certificación del 2026-11-13 verifica asociación y lifecycle hasta persistencia, consulta e hidratación. Aun así, un `202` solo confirma encolado: comprobar normalización y motivos de rechazo durante cada rollout.

## 6. Rollback

1. Desactivar `SUBJECT_ANALYTICS_SUGGESTIONS_ENABLED` en DragonFullAV.
2. Desactivar `SUBJECT_ANALYTICS_EMISSION_ENABLED`.
3. Detener `query-api` y `subject-jobs`.
4. Mantener ingest y worker legacy activos con las capacidades de sujetos apagadas si se necesita conservar la ingesta web.
5. No eliminar tablas en producción.
6. Usar migraciones `down` únicamente con autorización explícita y respaldo de PostgreSQL.

Para suspender también la captura en la fuente ejecutar `SELECT analytics_customer_lifecycle_configure(false);` en cada tenant habilitado. Las tablas se conservan. Una futura reactivación con `true` resincroniza la elegibilidad actual antes del bootstrap.

Para cambios del compose de desarrollo, conservar el backup previo fuera del repositorio. Restaurar el archivo completo desde el backup solo después de verificar la ruta y el hash.

```powershell
# Usar la ruta exacta registrada antes de editar y verificar su SHA-256.
Get-FileHash -LiteralPath '<backup-validado>' -Algorithm SHA256
Copy-Item -LiteralPath '<backup-validado>' -Destination 'C:\Proy_Python\GoAnalytics\docker-compose-devel.yml'
docker compose -f docker-compose-devel.yml config --quiet
```

## 7. Checklist de liberación

- [ ] Tokens generados, independientes y almacenados en el gestor seguro.
- [ ] Variables configuradas en ambos proyectos y sin secretos en Git.
- [ ] Migraciones verificadas desde la versión anterior.
- [ ] Endpoints internos protegidos con Bearer y accesibles solo por red privada.
- [ ] Snapshot, registro y query API responden según contrato.
- [ ] Bootstrap y resolución inicial completados.
- [ ] Proyecciones global y contextual completadas.
- [ ] Asociación real validada de extremo a extremo.
- [ ] Aislamiento multi-tenant verificado.
- [ ] Rollback probado en un entorno no productivo.
- [ ] Métricas, alertas y runbook operativo disponibles.
