# Plan De Tareas

## 1. Reglas De Ejecución

- Todas las tareas comienzan en estado `pending`.
- Una tarea solo puede pasar a `done` cuando sus criterios `AC-CA-*` estén satisfechos y la evidencia quede enlazada en este documento y en `15_estado_implementacion.md`.
- Los cambios deben preservar `POST /v1/events` y permanecer desactivados por defecto hasta el paso de rollout correspondiente.
- Las tareas del proyecto principal describen comportamiento y contrato; sus rutas internas se decidirán en ese repositorio.
- Las migraciones se ejecutan en orden y no se revierten en producción como mecanismo operativo.

## 2. Fase 0 — Confiabilidad Y Contratos

### TASK-CA-0001 — Recuperar Pendientes De Redis De Forma Segura

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-006`, `REQ-CA-007`, `REQ-CA-008`, `REQ-CA-010`.
- **Criterios:** `AC-CA-002`, `AC-CA-003`.
- **Dependencias:** ninguna.
- **Acciones:**
  1. Incorporar recuperación con `XAUTOCLAIM`, umbral configurable de inactividad y cursor de continuación.
  2. Alternar recuperación de pendientes con lectura de mensajes nuevos sin provocar starvation.
  3. Procesar y confirmar cada mensaje por separado después de obtener resultado persistente.
  4. Persistir payloads inválidos como rechazo durable con causa observable.
  5. Cubrir caída entre lectura, persistencia y `XACK`, además del agotamiento de reintentos.
- **Evidencia:**
  - `go test ./... -count=1` en `services/worker`: suite completa aprobada.
  - `go vet ./...` en `services/worker`: análisis estático aprobado.
  - `REDIS_INTEGRATION_ADDR=localhost:6379 go test ./internal/adapters/inbound/redisstream -run Integration -count=1 -v`: recuperación de un mensaje abandonado y protección de un mensaje activo aprobadas contra Redis real.
  - Pruebas unitarias cubren lote mixto, aislamiento por mensaje, fallo del repositorio de rechazos, cursor de `XAUTOCLAIM`, delivery count y agotamiento en la quinta entrega.
  - Los rechazos de transporte guardan ID de stream, delivery count, longitud y SHA-256; no persisten el payload crudo potencialmente sensible.

### TASK-CA-0002 — Aislar Persistencia E Idempotencia Por Evento

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-004`, `REQ-CA-009`, `REQ-CA-010`.
- **Criterios:** `AC-CA-001`, `AC-CA-004`.
- **Dependencias:** `TASK-CA-0001`.
- **Acciones:**
  1. Reemplazar el fallo transaccional de lote por resultados individuales clasificables.
  2. Distinguir duplicado exacto, duplicado lógico, duplicado funcional y error no recuperable.
  3. Confirmar como éxito idempotente los conflictos esperables y preservar los demás eventos del lote.
  4. Mantener sin cambios el alcance de los índices existentes y preparar la unicidad tenant-first para sujetos.
- **Evidencia:** `go test ./... -count=1` y `go vet ./...` aprobados en `services/worker`; integración PostgreSQL real aprobada para duplicados exactos, lógicos y funcionales, contradicción de idempotencia junto con un evento válido y conflicto lógico concurrente; `make test` aprobó ingest y worker, y `npm test --prefix packages/web-sdk` aprobó compilación y 16 pruebas. Los índices y las migraciones existentes permanecen sin cambios.

### TASK-CA-0003 — Agregar Flags, Configuración Y Migración `005`

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-001`, `REQ-CA-002`, `REQ-CA-003`, `REQ-CA-005`, `REQ-CA-015`.
- **Criterios:** `AC-CA-001`, `AC-CA-006`, `AC-CA-028`.
- **Dependencias:** `TASK-CA-0002`.
- **Acciones:**
  1. Crear la migración aditiva `005` con `event_source`, defaults compatibles y ajustes de idempotencia persistente.
  2. Incorporar flags separados para endpoint interno, normalización, backfill, proyecciones, registro y API.
  3. Mantener todos los flags apagados por defecto y validar combinaciones inválidas al iniciar.
  4. Implementar `down` simétrico para desarrollo con advertencia explícita de pérdida de datos.
- **Evidencia:** migración `005` validada con `migrate` sobre PostgreSQL 16 desde base vacía y desde versión `004` con un evento legacy; el default produjo `event_source=web`, el `CHECK` rechazó un origen inválido y ambos `down` regresaron a versión `004` conservando el evento. Las pruebas de bootstrap verifican los seis flags apagados por defecto, booleanos inválidos y dependencias de normalización/proyección. `make test`, ambos `go vet ./...` y las 16 pruebas del SDK aprobaron.

### TASK-CA-0004 — Extender El Resolver De Sites

- **Estado:** `done`
- **Responsables:** GoAnalytics y proyecto principal.
- **Requisitos:** `REQ-CA-013`, `REQ-CA-014`, `REQ-CA-015`.
- **Criterios:** `AC-CA-005`, `AC-CA-006`.
- **Dependencias:** `TASK-CA-0003`.
- **Acciones:**
  1. Extender de forma aditiva solicitud, respuesta, dominio y caché con `source` y `timezone` IANA.
  2. Permitir resolución server-to-server sin `origin`; conservarla para tráfico web.
  3. Validar zona horaria y definir error explícito cuando falte en capacidades que la requieran.
  4. Mantener compatibilidad con resolvers que todavía no devuelvan los campos nuevos mientras los flags estén apagados.
- **Evidencia:** GoAnalytics propaga `source`, conserva source vacío como web, permite server sin origin/token de tracking, mantiene las claves Redis web legacy, aísla claves server y conserva payloads cacheados sin timezone. `ValidateTimezone` distingue zona IANA válida de `timezone_unresolved`. En DragonFullAV, commit `4bcf56d13` extiende el resolver, lee `site.timezone` con degradación segura y aporta 5 pruebas focales aprobadas. `make test`, `go vet ./...` del worker y las 16 pruebas del SDK aprobaron. La suite backend general se ejecutó con `--maxfail=1`: 12 pruebas pasaron y se detuvo por un fallo ajeno en `test_expenses_repository.py`, cuyo fixture SQLite omite la columna preexistente `users.avatar_url`.

### TASK-CA-0005 — Preparar Operación, Métricas Y Objetivos De Rendimiento

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-064`, `REQ-CA-065`, `REQ-CA-066`.
- **Criterios:** `AC-CA-027`, `AC-CA-028`, `AC-CA-029`.
- **Dependencias:** `TASK-CA-0003`.
- **Acciones:**
  1. Definir métricas, logs estructurados y dimensiones tenant-safe de los nuevos flujos.
  2. Separar health de proceso y readiness de dependencias.
  3. Preparar targets de build y despliegue independientes para worker, jobs y query API.
  4. Construir el entorno de referencia y scripts repetibles para las metas de volumen del MVP.
- **Evidencia:** el worker expone `/health`, `/ready` y `/metrics` por rutas separadas; el puerto de métricas y los adaptadores no-op/Prometheus mantienen domain libre de infraestructura y rechazan nombres/labels fuera del catálogo. Se instrumentaron PEL, `XAUTOCLAIM`, retry agotado y resultados de persistencia. El dashboard Grafana, 6 reglas validadas por `promtool`, runbook y perfil determinista de 20 tenants/100.000 sujetos/500.000 eventos quedaron versionados. El Dockerfile parametrizable construyó `worker` y dejó targets independientes para los futuros comandos. `make test`, `go vet ./...` y 16 pruebas SDK aprobaron. Esta tarea prepara `AC-CA-027` y `AC-CA-029`; la certificación de flujos y carga permanece trazada a `TASK-CA-0204`, `TASK-CA-0304`, `TASK-CA-0305`, `TASK-CA-0407` y `TASK-CA-0408`.

## 3. Fase 1 — Identidad, Normalización Y Lifecycle

### TASK-CA-0101 — Crear Modelo De Sujetos Y Migración `006`

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-003`, `REQ-CA-018`, `REQ-CA-019`, `REQ-CA-020`, `REQ-CA-021`.
- **Criterios:** `AC-CA-008`, `AC-CA-009`.
- **Dependencias:** `TASK-CA-0003`.
- **Acciones:**
  1. Crear `analytics_subjects`, `analytics_event_subjects` y `analytics_subject_bootstrap`.
  2. Aplicar identidad única `tenant_id + subject_type + external_subject_id` e índices tenant-first.
  3. Incorporar estados, precedencia lifecycle, estado/motivo de normalización, revisión y timezone aplicadas.
  4. Incorporar unicidades de asociación necesarias para convergencia y reprocesamiento.
  5. Limitar el tipo habilitado a `customer` sin impedir extensiones aditivas futuras.
- **Evidencia:** la migración `006` se aplicó con `golang-migrate` sobre PostgreSQL 16 desde base vacía y desde versión `005`; su `down` eliminó simétricamente las tres tablas en ambos recorridos. Dos inserciones concurrentes con `ON CONFLICT` convergieron en una identidad tenant-first. PostgreSQL rechazó tipo distinto de `customer`, merge sin destino y una segunda asociación con igual idempotency key recibida desde otro site del tenant. El dominio define identidad, estados lifecycle y validaciones estables; `go test ./...` aprobó toda la suite del worker, incluidas sus pruebas focales.

### TASK-CA-0102 — Crear `POST /v1/internal/events`

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-011`, `REQ-CA-012`, `REQ-CA-013`, `REQ-CA-016`, `REQ-CA-017`.
- **Criterios:** `AC-CA-005`, `AC-CA-007`, `AC-CA-020`.
- **Dependencias:** `TASK-CA-0004`, `TASK-CA-0101`.
- **Acciones:**
  1. Agregar endpoint server-to-server con Bearer dedicado y respuesta `202`.
  2. Definir perfiles canónicos no ambiguos para asociación y lifecycle con uno o varios sujetos.
  3. No exigir sesión, navegación ni `origin`; exigir los campos del perfil server.
  4. Publicar al mismo Redis Stream con `event_source=server` y sin alterar el endpoint web.
  5. Aplicar límites de tamaño, validación estricta y errores estables.
- **Evidencia:** `POST /v1/internal/events` quedó registrado únicamente con `SUBJECT_INTERNAL_EVENTS_ENABLED=true`, exige `SUBJECT_EVENTS_API_TOKEN` separado y compara su digest en tiempo constante. El mapper acepta objeto o batch de hasta 50 eventos, rechaza campos desconocidos —incluidos navegación y `event_source` aportado— y valida el catálogo versionado `event_version=1`, uno a cien sujetos, asociación, lifecycle y merge. El caso de uso publica `event_source=server` mediante el mismo puerto Redis sin inventar sesión, origin, URL ni path; el flujo público fija `event_source=web`. Pruebas HTTP cubren `202`, `401`, `404`, límite de bytes y errores JSON estables; `make test` aprobó ingest y worker sin cambios incompatibles en `POST /v1/events`.

### TASK-CA-0103 — Propagar Origen, Zona Horaria Y Trazabilidad

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-012`, `REQ-CA-014`, `REQ-CA-015`, `REQ-CA-016`.
- **Criterios:** `AC-CA-005`, `AC-CA-006`, `AC-CA-007`.
- **Dependencias:** `TASK-CA-0102`.
- **Acciones:**
  1. Propagar `event_source`, tenant, site, timezone y claves de idempotencia por ingesta, stream y worker.
  2. Guardar timezone y revisión aplicadas como snapshot reproducible.
  3. Evitar que datos server relajen la validación de tráfico web.
  4. Agregar trazas correlacionables sin registrar tokens ni identificadores personales innecesarios.
- **Evidencia:** ingest publica `event_source` tanto en el envelope como en el JSON del stream; el consumer interpreta mensajes legacy sin source como `web` y conserva `server`. La validación del worker mantiene token, identidad de sesión, origin, URL y path obligatorios para web, mientras server usa una clave de caché aislada y no exige navegación. Tras resolver el site, el evento validado transporta tenant, site, source y timezone; el contexto persistido guarda el snapshot reservado `_goanalytics` con source, timezone y revisión de registro aplicada —vacía mientras el registro está deshabilitado—. PostgreSQL persiste y usa `event_source` en la huella de idempotencia. Integraciones reales aprobaron envelope/payload Redis y consulta de `analytics_events.event_source`; pruebas de aplicación aprobaron server sin navegación y rechazo web sin origin. `make test` y `go vet ./...` aprobaron ingest y worker.

### TASK-CA-0104 — Implementar Normalización 1:N Tenant-First

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-004`, `REQ-CA-016`, `REQ-CA-018`, `REQ-CA-019`, `REQ-CA-025`.
- **Criterios:** `AC-CA-007`, `AC-CA-008`, `AC-CA-011`.
- **Dependencias:** `TASK-CA-0101`, `TASK-CA-0103`.
- **Acciones:**
  1. Normalizar cero, uno o varios sujetos por evento manteniendo una sola fila cruda.
  2. Crear sujetos desconocidos por interacción web y resolver asociaciones server-to-server.
  3. Aplicar idempotencia funcional en todo el tenant aunque el hecho llegue desde distintos sites.
  4. Persistir `normalization_status/reason` sin impedir el guardado del evento crudo.
  5. Registrar revisión de contexto y timezone utilizadas.
- **Evidencia:** con `SUBJECT_ANALYTICS_ENABLED=false` el worker no genera escrituras de sujetos; al activarlo, `properties.subjects` produce cero, uno o varios detalles conservando la primera posición de duplicados exactos. El normalizador calcula fecha local desde el snapshot IANA, conserva operación y contexto declarado, crea `unknown` para web y `active` para `subject_associated` server. Formas inválidas, tipos no soportados y timezone ausente se guardan con `normalization_status/reason` sin impedir el evento crudo. El repositorio persiste evento, identidad y detalles en una transacción por evento; `ON CONFLICT` converge identidades concurrentes y transforma la segunda contribución funcional entre sites en `idempotency_conflict`. La integración PostgreSQL real verificó tres eventos crudos preservados, una sola identidad promovida a active, una asociación normalizada, un conflicto funcional y un fallo semántico; otra prueba concurrente produjo una identidad y dos detalles. `make test` y `go vet ./...` aprobaron.

### TASK-CA-0105 — Implementar Lifecycle Monotónico Y Merge

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-020`, `REQ-CA-021`, `REQ-CA-022`, `REQ-CA-023`, `REQ-CA-024`.
- **Criterios:** `AC-CA-009`, `AC-CA-010`.
- **Dependencias:** `TASK-CA-0104`.
- **Acciones:**
  1. Ordenar mutaciones por `source_updated_at + source_mutation_id`.
  2. Tratar `effective_at` como dato informativo.
  3. Registrar como no-op auditable eventos antiguos o duplicados.
  4. Conservar historia en el sujeto fusionado sin transferir conteos en el MVP.
  5. Mantener solo `active` como estado elegible.
- **Evidencia:** el normalizador extrae la tupla UTC `source_updated_at + source_mutation_id`, conserva `effective_at` solo como dato y rechaza lifecycle ambiguo o merge consigo mismo sin perder el evento crudo. PostgreSQL aplica la transición mediante un `UPDATE` condicional; el desempate textual usa collation binaria `C`, por lo que una tupla igual, anterior o con mutation ID menor queda `lifecycle_ignored` en el detalle sin modificar la identidad. Deactivate limpia merge y deja inactive; reactivate deja active; merge crea o reutiliza el destino unknown del mismo tenant/tipo, conserva la historia del origen y no mueve conteos. La integración real verificó evento antiguo ignorado, empate resuelto por mutation ID, reactivación y merge final con FK al destino. El dominio prueba orden temporal independiente de `effective_at` y define elegibilidad exclusiva de active. `make test` y `go vet ./...` aprobaron.

### TASK-CA-0106 — Emitir Asociaciones Y Lifecycle Desde El Proyecto Principal

- **Estado:** `done`
- **Responsable:** proyecto principal.
- **Requisitos:** `REQ-CA-060`, `REQ-CA-061`.
- **Criterios:** `AC-CA-025`.
- **Dependencias:** `TASK-CA-0102`, `TASK-CA-0105`.
- **Acciones:**
  1. Emitir solo después del commit de negocio.
  2. Reutilizar una `idempotency_key` estable durante tres intentos con timeout y backoff exponencial.
  3. No revertir la operación de negocio ante el fallo final.
  4. Exponer métricas, log y correlación para reintento o análisis operativo.
  5. Generar `source_mutation_id` monotónico y estable dentro de la fuente.
- **Evidencia:** DragonFullAV `2747c0fdf` incorpora un emisor opt-in desactivado por defecto, conectado después del commit de creación de turnos y conversión de clientes. Conserva `event_id` e `idempotency_key` entre intentos, usa timeout de un segundo, backoff de 250/1000 ms, reintenta transporte/429/5xx y no reintenta 4xx de contrato. El adaptador absorbe el fallo final, registra correlación y contadores sin PII, y genera la tupla lifecycle estable desde la mutación de origen. Las 4 pruebas específicas y 30 pruebas compatibles de turnos aprobaron; 6 pruebas ampliadas conservaron un fallo preexistente ajeno en `crud/erp/sale_order.py:625` por el fixture SQLite. `py_compile`, `git diff --check` y el escaneo UTF-8 aprobaron. `AC-CA-025` queda cubierto para emisión global; su extensión contextual permanece en `TASK-CA-0407`.

## 4. Fase 2 — Backfill Y Elegibilidad

### TASK-CA-0201 — Publicar Snapshot Revisionado En El Proyecto Principal

- **Estado:** `done`
- **Responsable:** proyecto principal.
- **Requisitos:** `REQ-CA-026`, `REQ-CA-027`, `REQ-CA-028`, `REQ-CA-029`.
- **Criterios:** `AC-CA-012`.
- **Dependencias:** `TASK-CA-0106`.
- **Acciones:**
  1. Implementar `GET /internal/analytics/subjects/snapshot` con Bearer dedicado.
  2. Devolver activos, inactivos y fusionados con cursor opaco y hasta 500 sujetos por página.
  3. Mantener `source_revision` estable durante toda la lectura.
  4. Responder `409 snapshot_revision_expired` cuando ya no pueda continuar la revisión.
  5. Incluir lifecycle y precedencia suficiente para converger con eventos en vivo.
- **Evidencia:** DragonFullAV `923a7d221` publica `GET /internal/analytics/subjects/snapshot` bajo flag apagado por defecto y Bearer dedicado. Materializa un corte inmutable de todos los tenants activos durante dos horas, ordena por tenant e ID externo, limita páginas a 500 y firma cursores opacos vinculados a la revisión. El payload contiene exclusivamente identidad técnica, lifecycle y precedencia; el contrato admite active, inactive y merged, mientras la fuente actual materializa active/inactive porque el modelo principal aún no registra fusiones. Revisión vencida produce `409 snapshot_revision_expired`; cursor inválido produce `400`. Nueve pruebas aprobaron paginación, estabilidad, manipulación, expiración, autenticación y regresión del resolver; `py_compile`, `git diff --check` y UTF-8 aprobaron.

### TASK-CA-0202 — Crear Cliente De Snapshot Y Bootstrap En `subject-jobs`

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-026`, `REQ-CA-027`, `REQ-CA-028`, `REQ-CA-030`, `REQ-CA-031`.
- **Criterios:** `AC-CA-012`, `AC-CA-013`.
- **Dependencias:** `TASK-CA-0201`, `TASK-CA-0101`.
- **Acciones:**
  1. Implementar cliente autenticado, cursor, revisión y límites del snapshot.
  2. Crear modo bootstrap del binario `cmd/subject-jobs`.
  3. Persistir progreso por tenant y revisión para reanudar sin duplicar.
  4. Reiniciar de forma controlada ante `snapshot_revision_expired`.
  5. Aplicar timeouts, reintentos acotados y métricas de avance.
- **Evidencia:** `cmd/subject-jobs -mode bootstrap` se construye como proceso separado y exige la cadena opt-in completa. El cliente HTTP usa Bearer, timeout configurable, páginas de hasta 500 y tres intentos acotados ante transporte, 429 o 5xx; valida de forma estricta revision, lifecycle y timestamps. El caso de uso reanuda cursor/revisión desde `analytics_subject_bootstrap`, aplica cada página y checkpoint en una transacción, reinicia como máximo dos veces ante `snapshot_revision_expired` y conserva el error final. El repositorio converge identidades tenant-first con precedencia monotónica y soporta merged mediante destino técnico. Pruebas con servidor simulado cubren auth, cursor, contrato, retry y 409; fakes cubren reanudación, reinicio y agotamiento. `go test ./...`, `go vet ./...`, `git diff --check` y UTF-8 aprobaron.

### TASK-CA-0203 — Hacer Converger Backfill, Lazy Creation Y Lifecycle

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-018`, `REQ-CA-020`, `REQ-CA-022`, `REQ-CA-023`, `REQ-CA-024`, `REQ-CA-029`, `REQ-CA-030`, `REQ-CA-031`.
- **Criterios:** `AC-CA-009`, `AC-CA-010`, `AC-CA-013`.
- **Dependencias:** `TASK-CA-0105`, `TASK-CA-0202`.
- **Acciones:**
  1. Resolver concurrencia mediante la identidad única y actualización monotónica.
  2. Evitar degradar un estado más nuevo con datos atrasados del snapshot.
  3. Registrar filas procesadas, no-op, errores recuperables y errores definitivos.
  4. Verificar estados active, inactive y merged y la elegibilidad resultante.
- **Evidencia:** el repositorio de bootstrap reutiliza la identidad única `tenant_id + subject_type + external_subject_id` y aplica lifecycle únicamente cuando la tupla `source_updated_at + source_mutation_id` supera la existente. Cada página y su checkpoint se confirman en una sola transacción; una falla conserva el checkpoint previo, marca la ejecución failed y emite resultado de página. La integración PostgreSQL real verificó creación lazy concurrente con backfill sin duplicados y resultado active, además de lifecycle inactive más nuevo que no fue degradado por un snapshot atrasado. Active, inactive y merged comparten la misma ruta; merged crea/reutiliza su destino técnico antes del origen. Las pruebas focales aprobaron contra `go-analytics-postgres` y la suite unitaria conserva reanudación, errores y revisión expirada.

### TASK-CA-0204 — Ejecutar Y Certificar El Backfill Inicial

- **Estado:** `done`
- **Responsables:** GoAnalytics y proyecto principal.
- **Requisitos:** `REQ-CA-030`, `REQ-CA-031`, `REQ-CA-064`, `REQ-CA-066`.
- **Criterios:** `AC-CA-013`, `AC-CA-027`, `AC-CA-029`.
- **Dependencias:** `TASK-CA-0203`, `TASK-CA-0005`.
- **Acciones:**
  1. Ejecutar en entorno de referencia con hasta 100.000 sujetos.
  2. Verificar completitud por estado, tenant y revisión.
  3. Confirmar reanudación después de una interrupción y ausencia de duplicados.
  4. Registrar duración, throughput, errores y decisión de habilitación de la fase siguiente.
- **Evidencia:** el 2026-08-11 se ejecutó `TestSubjectBootstrapVolume100k` contra el contenedor local `go-analytics-postgres`: 100.000 sujetos en 1m11,514s, throughput 1.398,33 sujetos/s y margen amplio frente al SLO de 60 minutos. La prueba interrumpió y reconstruyó el repositorio al llegar a 50.000; `BeginOrResume` recuperó cursor, revisión y conteo exactos. El resultado final tuvo 100.000 identidades distintas, 90.000 active, 10.000 inactive, cero duplicados y checkpoint completed en 100.000. `make test`, vet de ingest/worker y las 16 pruebas del SDK aprobaron. El gate Fase 2 → 3 queda habilitado con flags aún apagados por defecto.

## 5. Fase 3 — Proyección Diaria Y Ranking Global

### TASK-CA-0301 — Crear Migración `007` Y Repositorios De Proyección

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-003`, `REQ-CA-032`, `REQ-CA-035`, `REQ-CA-036`, `REQ-CA-037`, `REQ-CA-038`, `REQ-CA-039`.
- **Criterios:** `AC-CA-014`, `AC-CA-015`, `AC-CA-016`.
- **Dependencias:** `TASK-CA-0204`.
- **Acciones:**
  1. Crear ejecuciones, checkpoints y agregado diario global con claves `BIGSERIAL` y FKs explícitas.
  2. Definir unicidades para reemplazo idempotente por tenant, día y sujeto.
  3. Crear índices tenant-first para watermark, ventanas y ranking.
  4. Incluir `association_count` y `last_association_at`.
- **Evidencia:** la migración `007_create_subject_global_projections` crea runs, checkpoint compuesto y agregado diario global con FK cascade, PK tenant-first, restricciones de estados/conteos y unicidad parcial de una ejecución running. Los índices cubren historial de runs, ranking diario y ventana por tenant/versión/fecha. El repositorio PostgreSQL registra complete/failed, carga checkpoint cero, reemplaza agregados absolutos y avanza el watermark en la misma transacción. La cadena 001→007, down 007 y up 007 aprobaron en una base temporal aislada; la base fue eliminada después. La integración real verificó run, agregado, `last_association_at`, checkpoint y finalización; `go test ./...` y `go vet ./...` aprobaron.

### TASK-CA-0302 — Implementar Proyector Global Incremental

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-033`, `REQ-CA-034`, `REQ-CA-035`, `REQ-CA-036`, `REQ-CA-037`, `REQ-CA-038`, `REQ-CA-039`.
- **Criterios:** `AC-CA-014`, `AC-CA-015`, `AC-CA-016`.
- **Dependencias:** `TASK-CA-0301`.
- **Acciones:**
  1. Tomar lock asesor PostgreSQL por ejecución compatible con una sola instancia efectiva.
  2. Detectar cambios mediante watermark compuesto `updated_at + id`.
  3. Recalcular días completos afectados y reemplazarlos atómicamente.
  4. Agregar ventana global de 90 días solo para sujetos elegibles.
  5. Avanzar checkpoint únicamente después de publicar agregados y ejecución exitosa.
- **Evidencia:** `ProjectGlobal` obtiene un advisory lock dedicado por nombre+versión, recupera el checkpoint `updated_at + id`, lee lotes ordenados, deduplica claves diarias y publica recomputación absoluta junto con el nuevo checkpoint en una transacción. El repositorio elimina la fila anterior antes del `INSERT SELECT`, por lo que también limpia agregados que quedan en cero o pierden elegibilidad; solo identidades active participan. Un run interrumpido se marca failed al recuperar el lock y recomienza desde el último checkpoint durable; lock ocupado registra `skipped_locked`. Pruebas unitarias cubren lotes, fallo previo al checkpoint y lock omitido. PostgreSQL real verificó repetición no-op, evento tardío incorporado sin doble conteo y exclusión entre dos conexiones. Suite worker y vet aprobaron.

### TASK-CA-0303 — Programar Proyección En `subject-jobs`

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-032`, `REQ-CA-033`, `REQ-CA-034`, `REQ-CA-064`.
- **Criterios:** `AC-CA-014`, `AC-CA-027`.
- **Dependencias:** `TASK-CA-0302`.
- **Acciones:**
  1. Incorporar modos bootstrap, resolución y proyección al binario separado.
  2. Ejecutar cada 86.400 segundos y al iniciar si no existe ejecución vigente.
  3. Exponer health, readiness, estado de última ejecución y cierre ordenado.
  4. Evitar ejecuciones duplicadas ante múltiples réplicas mediante el lock PostgreSQL.
- **Evidencia:** `subject-jobs` admite `bootstrap`, `project-global` y `serve`; el contenedor construye solo las capacidades habilitadas y no requiere Redis para proyección. `serve` expone `/health`, `/ready`, `/metrics` y `/status`, ejecuta al iniciar cuando no hay éxito o está vencido, y reevalúa cada 60 segundos por defecto. Intervalo 86.400s, stale 172.800s, batch, versión y puerto son configurables y validados. El cierre por señal detiene HTTP en hasta 10s; el advisory lock mantiene una sola ejecución efectiva entre réplicas. Pruebas aceleradas cubren inicio sin run y omisión con run fresco; suite completa y vet del worker aprobaron. El Dockerfile parametrizable ya construye la imagen independiente con `WORKER_COMMAND=subject-jobs`.

### TASK-CA-0304 — Crear Query API Y Ranking Global

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-050`, `REQ-CA-051`, `REQ-CA-052`, `REQ-CA-053`, `REQ-CA-054`, `REQ-CA-055`, `REQ-CA-056`, `REQ-CA-057`, `REQ-CA-058`, `REQ-CA-059`.
- **Criterios:** `AC-CA-020`, `AC-CA-021`, `AC-CA-022`, `AC-CA-023`, `AC-CA-024`.
- **Dependencias:** `TASK-CA-0302`, `TASK-CA-0005`.
- **Acciones:**
  1. Crear `cmd/query-api` y `GET /v1/internal/subjects/suggestions` con Bearer dedicado.
  2. Validar tenant, `subject_type`, límites y parámetros de contexto.
  3. Ordenar por conteo, última asociación e ID externo; devolver solo IDs externos y trazabilidad.
  4. Aplicar estados unavailable, stale, empty y aislamiento por tenant.
  5. Servir stale por defecto dentro del contrato acordado y permitir desactivarlo.
- **Evidencia:** `cmd/query-api` inicia como proceso independiente sin Redis Stream y falla cerrado si flag/token no están configurados. Expone health/readiness/metrics y `GET /v1/internal/subjects/suggestions` con Bearer comparado en tiempo constante, query estricta, tenant/tipo obligatorios, contexto coherente, default 10 y máximo 100. El caso de uso distingue unavailable, stale servido y stale rechazado; vacío serializa `[]`. PostgreSQL consulta solo agregado diario, une tenant en agregado/identidad, filtra active y suma ventana local de 90 días. Orden: association desc, last association desc nulls last e ID externo asc; respuesta solo contiene ID externo y trazabilidad global. Pruebas HTTP cubren auth, parámetros y límites; unitarias cubren frescura/vacío; PostgreSQL real verificó orden y ausencia de fuga entre tenants. Suite y vet aprobaron.

### TASK-CA-0305 — Integrar Sugerencias Globales En El Proyecto Principal

- **Estado:** `done`
- **Responsable:** proyecto principal.
- **Requisitos:** `REQ-CA-062`, `REQ-CA-063`, `REQ-CA-064`, `REQ-CA-065`, `REQ-CA-066`.
- **Criterios:** `AC-CA-026`, `AC-CA-027`, `AC-CA-028`, `AC-CA-029`.
- **Dependencias:** `TASK-CA-0304`.
- **Acciones:**
  1. Consultar sugerencias globales bajo flag y con timeout acotado.
  2. Recuperar datos reales del cliente en el proyecto principal y descartar sujetos ya no elegibles.
  3. Definir experiencia degradada ante timeout, unavailable o lista vacía.
  4. Activar progresivamente por tenant y medir latencia, stale y errores.
- **Evidencia:** DragonFullAV `d0216f882` agrega cliente opt-in con Bearer, timeout de un segundo y allowlist progresiva por tenant. El endpoint POS `/customers/suggestions` consulta solo IDs externos, hidrata en la sesión tenant actual, exige `type=client` e `is_active`, preserva el orden y descarta IDs ausentes. Ante flag apagado, contexto tenant ausente, timeout/503, payload inválido o lista vacía, reutiliza la lista de clientes actual sin afectar el flujo. Registra contadores de accepted/empty/degraded sin tenant ni PII. Tres pruebas específicas cubren auth/tenant/orden, 503 y descarte; junto con la regresión de búsqueda aprobaron 21 tests y 1 skip de entorno. `py_compile`, `git diff --check` y UTF-8 aprobaron.

## 6. Fase 4 — Registro Y Ranking Contextual

### TASK-CA-0401 — Publicar El Registro Desde El Proyecto Principal

- **Estado:** `done`
- **Responsable:** proyecto principal.
- **Requisitos:** `REQ-CA-040`, `REQ-CA-041`, `REQ-CA-042`, `REQ-CA-043`.
- **Criterios:** `AC-CA-017`.
- **Dependencias:** `TASK-CA-0305`.
- **Acciones:**
  1. Implementar `GET /internal/analytics/context-registry` con revisiones estables.
  2. Publicar las siete familias MVP y definiciones inmutables por clave y versión.
  3. Asociar estado activo/inactivo a una revisión concreta.
  4. Rechazar ciclos, referencias inexistentes y mutaciones de una definición publicada.
- **Evidencia:** DragonFullAV `1c576f810` versiona un JSON canónico con siete definiciones, una por familia MVP, identidades `context_key + context_version`, padres y estado. El cargador valida forma estricta, duplicados, familias completas, referencias, ciclos e inmutabilidad semántica entre revisiones; solo status puede variar para una definición publicada y registry_version debe avanzar. `registry_revision` deriva de SHA-256 canónico y es estable para el mismo payload. `GET /internal/analytics/context-registry` es opt-in, usa Bearer dedicado y devuelve 503 si el archivo desplegado es inválido. Diez pruebas aprobaron catálogo, revisión, padre inexistente, ciclo, mutación, auth y regresión de analytics interno; sintaxis, diff y UTF-8 aprobaron.

### TASK-CA-0402 — Crear Migración `008` Y Repositorio Revisionado

- **Estado:** `done`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-003`, `REQ-CA-040`, `REQ-CA-041`, `REQ-CA-042`, `REQ-CA-043`, `REQ-CA-044`, `REQ-CA-045`, `REQ-CA-046`.
- **Criterios:** `AC-CA-017`, `AC-CA-018`.
- **Dependencias:** `TASK-CA-0401`.
- **Acciones:**
  1. Crear revisiones, definiciones, pendientes no registrados y agregado diario contextual.
  2. Aplicar inmutabilidad, unicidades e índices tenant-first.
  3. Implementar carga memoria → Redis → PostgreSQL → proyecto principal.
  4. Proteger refresh con lock Redis tokenizado y liberación compare-and-delete.
  5. Conservar la última revisión válida cuando una nueva sea inválida.
- **Evidencia:** la migración `008` crea revisiones, definiciones inmutables, entradas por revisión, pendientes no registrados y agregado contextual con claves e índices tenant-first; el recorrido aislado `001 up` a `008 up`, `008 down` y `008 up` aprobó. El dominio valida catálogo, padres, ciclos y checksum; PostgreSQL conserva una única revisión activa y rechaza mutaciones publicadas. La carga sigue memoria → Redis → PostgreSQL → fuente HTTP, conserva el último registro válido y usa lock Redis con token y compare-and-delete. La suite completa y `go vet ./...` aprobaron; la integración contra Redis real confirmó exclusión y liberación propietaria. `AC-CA-017` y `AC-CA-018` quedan cubiertos.

### TASK-CA-0403 — Resolver Contextos Durante La Normalización

- **Estado:** `pending`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-016`, `REQ-CA-047`, `REQ-CA-048`.
- **Criterios:** `AC-CA-007`, `AC-CA-019`.
- **Dependencias:** `TASK-CA-0104`, `TASK-CA-0402`.
- **Acciones:**
  1. Resolver por `context_key + context_version` contra una revisión válida.
  2. Guardar revisión y resultado de resolución en la asociación normalizada.
  3. Enviar contextos desconocidos a pendientes y mantener su aporte global.
  4. Excluir contextos desconocidos o inactivos del agregado contextual.
- **Evidencia:** pendiente: pruebas de revisión histórica, desconocidos e inactivos.

### TASK-CA-0404 — Resolver Históricos Iniciales De Forma Idempotente

- **Estado:** `pending`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-035`, `REQ-CA-036`, `REQ-CA-049`.
- **Criterios:** `AC-CA-015`, `AC-CA-019`.
- **Dependencias:** `TASK-CA-0403`.
- **Acciones:**
  1. Incorporar modo de resolución inicial a `cmd/subject-jobs`.
  2. Seleccionar solo filas previas sin revisión resuelta y procesarlas por páginas.
  3. Persistir checkpoint reanudable y resultado idempotente.
  4. Marcar días afectados para recomputación sin iniciar conciliación general.
- **Evidencia:** pendiente: pruebas de reanudación, segunda ejecución no-op y conteos afectados.

### TASK-CA-0405 — Implementar Proyección Contextual

- **Estado:** `pending`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-032`, `REQ-CA-033`, `REQ-CA-034`, `REQ-CA-035`, `REQ-CA-036`, `REQ-CA-037`, `REQ-CA-038`, `REQ-CA-039`, `REQ-CA-047`, `REQ-CA-048`, `REQ-CA-049`.
- **Criterios:** `AC-CA-014`, `AC-CA-015`, `AC-CA-016`, `AC-CA-019`.
- **Dependencias:** `TASK-CA-0404`, `TASK-CA-0303`.
- **Acciones:**
  1. Calcular agregado contextual diario con ventana de 30 días.
  2. Reutilizar lock, watermark, recomputación diaria y publicación atómica.
  3. Aplicar elegibilidad y revisión histórica de cada asociación.
  4. Mantener el agregado global independiente y disponible.
- **Evidencia:** pendiente: pruebas de agregados, tardíos, cambios de revisión y rendimiento.

### TASK-CA-0406 — Habilitar Ranking Contextual Y Fallback

- **Estado:** `pending`
- **Responsable:** GoAnalytics.
- **Requisitos:** `REQ-CA-052`, `REQ-CA-053`, `REQ-CA-054`, `REQ-CA-055`, `REQ-CA-056`, `REQ-CA-057`, `REQ-CA-058`, `REQ-CA-059`.
- **Criterios:** `AC-CA-021`, `AC-CA-022`, `AC-CA-023`, `AC-CA-024`.
- **Dependencias:** `TASK-CA-0405`, `TASK-CA-0304`.
- **Acciones:**
  1. Exigir `context_version` cuando exista `context_key`.
  2. Resolver contexto exacto y aplicar como único fallback el ranking global del tenant.
  3. Conservar orden determinista, límites, frescura y privacidad.
  4. Exponer trazabilidad de fuente de resultado sin claves internas.
- **Evidencia:** pendiente: pruebas exactas, fallback, sin datos, stale y aislamiento.

### TASK-CA-0407 — Integrar Contextos Y Sugerencias En El Proyecto Principal

- **Estado:** `pending`
- **Responsable:** proyecto principal.
- **Requisitos:** `REQ-CA-060`, `REQ-CA-061`, `REQ-CA-062`, `REQ-CA-063`, `REQ-CA-064`, `REQ-CA-065`.
- **Criterios:** `AC-CA-025`, `AC-CA-026`, `AC-CA-027`, `AC-CA-028`.
- **Dependencias:** `TASK-CA-0401`, `TASK-CA-0406`.
- **Acciones:**
  1. Emitir claves/versiones de contexto publicadas en las asociaciones.
  2. Consultar sugerencias contextuales bajo flag y realizar hidratación local.
  3. Aplicar fallback funcional del consumidor cuando GoAnalytics no esté disponible.
  4. Activar por tenant, observar y documentar reversión mediante flags.
- **Evidencia:** pendiente: recorrido end-to-end y aprobación de usuarios internos.

### TASK-CA-0408 — Certificar El MVP Y Cerrar El Rollout

- **Estado:** `pending`
- **Responsables:** GoAnalytics y proyecto principal.
- **Requisitos:** `REQ-CA-001`, `REQ-CA-002`, `REQ-CA-004`, `REQ-CA-005`, `REQ-CA-064`, `REQ-CA-065`, `REQ-CA-066`.
- **Criterios:** `AC-CA-001`, `AC-CA-027`, `AC-CA-028`, `AC-CA-029`, `AC-CA-030`.
- **Dependencias:** `TASK-CA-0407` y todas las tareas anteriores.
- **Acciones:**
  1. Ejecutar regresión completa de ingest, worker y SDK.
  2. Ejecutar pruebas de migración, contrato, integración, concurrencia, recuperación y carga.
  3. Verificar que todos los flags continúen permitiendo desactivar el MVP sin afectar contratos existentes.
  4. Completar runbooks, alertas, evidencias y matriz final de trazabilidad.
  5. Aprobar el gate del MVP o registrar explícitamente criterios pendientes.
- **Evidencia:** pendiente: informe de aceptación, trazabilidad completa y acta de habilitación.

## 7. Secuencia Crítica

La ruta crítica mínima es:

`TASK-CA-0001` → `TASK-CA-0002` → `TASK-CA-0003` → `TASK-CA-0101` → `TASK-CA-0102` → `TASK-CA-0104` → `TASK-CA-0105` → `TASK-CA-0201` → `TASK-CA-0202` → `TASK-CA-0203` → `TASK-CA-0204` → `TASK-CA-0301` → `TASK-CA-0302` → `TASK-CA-0304` → `TASK-CA-0305` → `TASK-CA-0401` → `TASK-CA-0402` → `TASK-CA-0403` → `TASK-CA-0404` → `TASK-CA-0405` → `TASK-CA-0406` → `TASK-CA-0407` → `TASK-CA-0408`.

La preparación operativa puede avanzar en paralelo después de `TASK-CA-0003`, pero cada gate exige su evidencia antes de habilitar la fase siguiente.
