# Perfil De Carga Del MVP

`profile.json` es la fuente reproducible del volumen máximo acordado. La semilla fija permite regenerar las mismas identidades opacas y distribuciones en cada ejecución.

El preflight se ejecuta con:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\customers-analytics-load\validate-profile.ps1
```

Devuelve el checksum SHA-256 y la semilla que deberán acompañar cada reporte.

El runner final se ejecuta contra una base PostgreSQL de referencia aislando su
dataset en un schema temporal:

```powershell
$env:POSTGRES_INTEGRATION_DSN='postgres://analytics:analytics@localhost:5433/analytics?sslmode=disable'
$env:SUBJECT_MVP_VOLUME_TEST='1'
go test ./services/worker/internal/adapters/outbound/postgres -run TestSubjectMVPVolumeProfile -v -count=1
```

La prueba genera 20 tenants y 100.000 sujetos, procesa 500.000 asociaciones,
mide las proyecciones global/contextual y calcula p95 sobre 200 consultas
reales (100 globales y 100 contextuales) con `limit=10`.

La evolución del runner se completó por etapas:

- `TASK-CA-0204`: generación de snapshot y medición de bootstrap.
- `TASK-CA-0304`: población global, proyección y consultas con `limit=10`.
- `TASK-CA-0408`: contexto, recorrido end-to-end y reporte consolidado.

Cada reporte debe guardar commit, hardware, PostgreSQL/Redis, flags, fecha UTC, comando, dataset, p50/p95/p99, throughput, duración y errores. No se aceptan mocks para certificar `AC-CA-029` ni se incluyen datos personales.

El perfil valida hasta 20 tenants, 100.000 sujetos y 500.000 eventos mensuales. Los objetivos son query p95 ≤300 ms, proyección global ≤30 minutos y bootstrap ≤60 minutos. Esta tarea prepara el contrato del entorno; la certificación se realiza cuando estén implementados snapshot, proyector y query API.
