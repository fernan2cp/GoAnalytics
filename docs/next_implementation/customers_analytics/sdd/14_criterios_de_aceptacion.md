# Criterios De Aceptación

## 1. Convenciones

- Cada criterio es binario: `passed` o `failed`; `pending` solo representa que todavía no se ejecutó.
- La evidencia debe identificar entorno, versión, fecha, comando o caso ejecutado y resultado.
- Los criterios de rendimiento se miden en el entorno de referencia definido antes de comenzar la implementación.
- Ningún criterio permite sustituir la regresión de contratos existentes por una revisión manual.

## 2. Compatibilidad Y Confiabilidad

### AC-CA-001 — Retrocompatibilidad Y Opt-In

- **Requisitos:** `REQ-CA-001`, `REQ-CA-002`, `REQ-CA-003`, `REQ-CA-004`, `REQ-CA-005`.
- **Tareas:** `TASK-CA-0002`, `TASK-CA-0003`, `TASK-CA-0408`.
- **Aceptación:** con todos los flags nuevos apagados, los contratos, validación, persistencia y semántica actuales de ingest, worker y SDK pasan sin cambios; las migraciones son aditivas y un error de normalización no impide persistir el evento crudo.

### AC-CA-002 — Recuperación De Pendientes

- **Requisitos:** `REQ-CA-006`, `REQ-CA-010`.
- **Tareas:** `TASK-CA-0001`.
- **Aceptación:** un mensaje leído y no confirmado antes de una caída es reclamado con `XAUTOCLAIM`, procesado una sola vez de forma efectiva y confirmado solo después del resultado durable.

### AC-CA-003 — Aislamiento De Mensajes Inválidos

- **Requisitos:** `REQ-CA-007`, `REQ-CA-008`, `REQ-CA-010`.
- **Tareas:** `TASK-CA-0001`.
- **Aceptación:** un payload inválido produce un rechazo persistente y observable, no impide procesar mensajes vecinos y no queda reintentándose indefinidamente.

### AC-CA-004 — Conflictos Persistentes Por Evento

- **Requisitos:** `REQ-CA-009`, `REQ-CA-010`.
- **Tareas:** `TASK-CA-0002`.
- **Aceptación:** conflictos exactos, lógicos o funcionales esperables devuelven resultado idempotente individual y jamás revierten eventos válidos del mismo lote.

## 3. Contratos, Identidad Y Lifecycle

### AC-CA-005 — Endpoint Interno Aislado

- **Requisitos:** `REQ-CA-011`, `REQ-CA-012`, `REQ-CA-013`.
- **Tareas:** `TASK-CA-0004`, `TASK-CA-0102`, `TASK-CA-0103`.
- **Aceptación:** `POST /v1/internal/events` exige su Bearer, acepta perfiles server sin sesión/navegación/origin, responde `202`, publica al stream existente y no cambia `POST /v1/events`.

### AC-CA-006 — Resolver Aditivo Y Timezone Reproducible

- **Requisitos:** `REQ-CA-014`, `REQ-CA-015`.
- **Tareas:** `TASK-CA-0003`, `TASK-CA-0004`, `TASK-CA-0103`.
- **Aceptación:** resolver y caché transportan `source` y timezone IANA; los eventos que requieren calendario guardan la zona aplicada y una zona ausente o inválida falla de forma explícita sin afectar tráfico legacy.

### AC-CA-007 — Contrato Canónico De Sujetos

- **Requisitos:** `REQ-CA-016`, `REQ-CA-017`.
- **Tareas:** `TASK-CA-0102`, `TASK-CA-0103`, `TASK-CA-0104`, `TASK-CA-0403`.
- **Aceptación:** asociación y lifecycle tienen esquemas versionados no ambiguos para uno o varios sujetos, con validación por origen y trazabilidad de idempotencia.

### AC-CA-008 — Identidad Canónica

- **Requisitos:** `REQ-CA-018`, `REQ-CA-019`.
- **Tareas:** `TASK-CA-0101`, `TASK-CA-0104`.
- **Aceptación:** dos operaciones concurrentes con la misma terna tenant/tipo/ID externo convergen en un sujeto, y el MVP rechaza tipos distintos de `customer` con un error estable.

### AC-CA-009 — Estados Y Elegibilidad

- **Requisitos:** `REQ-CA-020`, `REQ-CA-021`.
- **Tareas:** `TASK-CA-0101`, `TASK-CA-0105`, `TASK-CA-0203`.
- **Aceptación:** web crea `unknown`, asociación server puede crear `active`, lifecycle soporta `inactive` y `merged`, y solo `active` participa en resultados.

### AC-CA-010 — Precedencia Lifecycle Determinista

- **Requisitos:** `REQ-CA-022`, `REQ-CA-023`, `REQ-CA-024`.
- **Tareas:** `TASK-CA-0105`, `TASK-CA-0203`.
- **Aceptación:** la precedencia usa exclusivamente `source_updated_at + source_mutation_id`; eventos antiguos son no-op auditables y merge conserva historia sin transferir conteos.

### AC-CA-011 — Idempotencia Funcional Tenant-First

- **Requisitos:** `REQ-CA-025`.
- **Tareas:** `TASK-CA-0104`.
- **Aceptación:** el mismo hecho funcional recibido por sites diferentes del mismo tenant produce una sola contribución normalizada sin modificar índices legacy site-scoped.

## 4. Backfill Y Proyecciones

### AC-CA-012 — Snapshot Consistente Y Paginado

- **Requisitos:** `REQ-CA-026`, `REQ-CA-027`, `REQ-CA-028`, `REQ-CA-029`.
- **Tareas:** `TASK-CA-0201`, `TASK-CA-0202`.
- **Aceptación:** el snapshot entrega hasta 500 sujetos por página, todos los estados MVP, cursor opaco y revisión estable; una revisión no disponible devuelve exactamente `409 snapshot_revision_expired`.

### AC-CA-013 — Bootstrap Reanudable Y Convergente

- **Requisitos:** `REQ-CA-030`, `REQ-CA-031`.
- **Tareas:** `TASK-CA-0202`, `TASK-CA-0203`, `TASK-CA-0204`.
- **Aceptación:** una interrupción puede reanudarse sin duplicados, lazy creation y backfill convergen, y ningún dato atrasado degrada lifecycle más reciente.

### AC-CA-014 — Ejecución Diaria Exclusiva

- **Requisitos:** `REQ-CA-032`, `REQ-CA-033`, `REQ-CA-034`.
- **Tareas:** `TASK-CA-0301`, `TASK-CA-0302`, `TASK-CA-0303`, `TASK-CA-0405`.
- **Aceptación:** `subject-jobs` ejecuta cada 86.400 segundos y al iniciar si corresponde; dos instancias concurrentes producen una sola ejecución efectiva mediante lock PostgreSQL.

### AC-CA-015 — Watermark Y Recomposición Atómica

- **Requisitos:** `REQ-CA-035`, `REQ-CA-036`.
- **Tareas:** `TASK-CA-0301`, `TASK-CA-0302`, `TASK-CA-0404`, `TASK-CA-0405`.
- **Aceptación:** el watermark `updated_at + id` no omite empates; eventos tardíos recalculan días completos, el reemplazo es atómico y el checkpoint no avanza si falla la publicación.

### AC-CA-016 — Agregados Y Orden Global

- **Requisitos:** `REQ-CA-037`, `REQ-CA-038`, `REQ-CA-039`.
- **Tareas:** `TASK-CA-0301`, `TASK-CA-0302`, `TASK-CA-0405`.
- **Aceptación:** el global usa 90 días, el contextual 30, ambos excluyen no elegibles y preservan conteo y `last_association_at` para ordenar de forma reproducible.

## 5. Registro De Contextos

### AC-CA-017 — Registro Revisionado E Inmutable

- **Requisitos:** `REQ-CA-040`, `REQ-CA-041`, `REQ-CA-042`, `REQ-CA-043`.
- **Tareas:** `TASK-CA-0401`, `TASK-CA-0402`.
- **Aceptación:** el catálogo contiene las siete familias, una definición publicada no puede mutar, activo/inactivo pertenece a una revisión y ciclos o referencias inválidas impiden publicar esa revisión.

### AC-CA-018 — Carga Y Caché Seguras

- **Requisitos:** `REQ-CA-044`, `REQ-CA-045`, `REQ-CA-046`.
- **Tareas:** `TASK-CA-0402`.
- **Aceptación:** la carga sigue memoria → Redis → PostgreSQL → fuente, el refresh usa lock tokenizado con liberación segura y una revisión inválida nunca reemplaza la última válida.

### AC-CA-019 — Resolución Histórica De Contextos

- **Requisitos:** `REQ-CA-047`, `REQ-CA-048`, `REQ-CA-049`.
- **Tareas:** `TASK-CA-0403`, `TASK-CA-0404`, `TASK-CA-0405`.
- **Aceptación:** cada asociación conserva revisión y resultado; desconocidos aportan solo al global, e históricos previos se resuelven una vez de forma reanudable e idempotente.

## 6. API, Seguridad E Integración

### AC-CA-020 — Proceso Y Autenticación Independientes

- **Requisitos:** `REQ-CA-050`, `REQ-CA-051`.
- **Tareas:** `TASK-CA-0102`, `TASK-CA-0304`.
- **Aceptación:** query API se despliega como proceso separado, rechaza Bearer ausente o inválido y no reutiliza autenticación ni contratos públicos existentes.

### AC-CA-021 — Validación Y Límites De Consulta

- **Requisitos:** `REQ-CA-052`, `REQ-CA-053`.
- **Tareas:** `TASK-CA-0304`, `TASK-CA-0406`.
- **Aceptación:** tenant y tipo son obligatorios, versión es obligatoria con clave de contexto, el límite por defecto es 10, el máximo 100 y entradas inválidas producen errores estables.

### AC-CA-022 — Ranking Y Fallback Deterministas

- **Requisitos:** `REQ-CA-054`, `REQ-CA-055`.
- **Tareas:** `TASK-CA-0304`, `TASK-CA-0406`.
- **Aceptación:** el orden es conteo descendente, última asociación descendente e ID externo ascendente; el único fallback es contexto exacto → global del tenant.

### AC-CA-023 — Privacidad, Vacío Y Frescura

- **Requisitos:** `REQ-CA-056`, `REQ-CA-057`, `REQ-CA-058`.
- **Tareas:** `TASK-CA-0304`, `TASK-CA-0406`.
- **Aceptación:** la respuesta no contiene IDs internos ni PII, sin datos devuelve `200 []`, sin proyección devuelve `503 projection_unavailable` y una proyección >48 h sigue el flag stale acordado.

### AC-CA-024 — Aislamiento Multi-Tenant

- **Requisitos:** `REQ-CA-059`.
- **Tareas:** `TASK-CA-0304`, `TASK-CA-0406`.
- **Aceptación:** pruebas cruzadas y planes SQL demuestran que ninguna consulta, caché, lock o agregado puede mezclar tenants.

### AC-CA-025 — Emisión Post-Commit Con Retry Directo

- **Requisitos:** `REQ-CA-060`, `REQ-CA-061`.
- **Tareas:** `TASK-CA-0106`, `TASK-CA-0407`.
- **Aceptación:** la fuente emite después del commit, realiza como máximo tres intentos con la misma clave, timeout y backoff, y el fallo final observable no revierte negocio.

### AC-CA-026 — Hidratación Y Elegibilidad En El Consumidor

- **Requisitos:** `REQ-CA-062`, `REQ-CA-063`.
- **Tareas:** `TASK-CA-0305`, `TASK-CA-0407`.
- **Aceptación:** el proyecto principal hidrata IDs externos con datos propios, descarta sujetos ya no elegibles y degrada de forma definida ante ausencia o fallo de sugerencias.

## 7. Operación, Rollout Y Rendimiento

### AC-CA-027 — Observabilidad Operable

- **Requisitos:** `REQ-CA-064`.
- **Tareas:** `TASK-CA-0005`, `TASK-CA-0204`, `TASK-CA-0303`, `TASK-CA-0305`, `TASK-CA-0407`, `TASK-CA-0408`.
- **Aceptación:** health, readiness, métricas, logs y alertas distinguen stream, normalización, bootstrap, registro, proyección y query sin filtrar secretos ni PII.

### AC-CA-028 — Rollout Y Rollback Por Flags

- **Requisitos:** `REQ-CA-065`.
- **Tareas:** `TASK-CA-0003`, `TASK-CA-0305`, `TASK-CA-0407`, `TASK-CA-0408`.
- **Aceptación:** cada capacidad se activa por fase/tenant y puede desactivarse o detener su binario sin eliminar tablas ni afectar ingestión legacy.

### AC-CA-029 — Metas De Volumen Del MVP

- **Requisitos:** `REQ-CA-066`.
- **Tareas:** `TASK-CA-0005`, `TASK-CA-0204`, `TASK-CA-0304`, `TASK-CA-0305`, `TASK-CA-0408`.
- **Aceptación:** con hasta 20 tenants, 100.000 clientes y 500.000 eventos mensuales, query p95 es ≤300 ms para limit 10, proyección diaria ≤30 min y backfill de 100.000 sujetos ≤60 min.

### AC-CA-030 — Recorrido MVP End-To-End

- **Requisitos:** `REQ-CA-001`, `REQ-CA-005`, `REQ-CA-011`, `REQ-CA-018`, `REQ-CA-026`, `REQ-CA-032`, `REQ-CA-040`, `REQ-CA-050`, `REQ-CA-060`, `REQ-CA-065`.
- **Tareas:** `TASK-CA-0408`.
- **Aceptación:** un tenant opt-in completa asociación, lifecycle, backfill, proyección global, registro, proyección contextual, consulta e hidratación; al desactivarlo conserva el servicio legacy sin pérdida del evento crudo.

## 8. Resultado Final

Recertificado el 2026-11-13 en `TASK-CA-0501` a `TASK-CA-0506`. La evidencia vigente reemplaza el cierre anterior basado en pruebas parciales del recorrido. Los comandos y límites del entorno se encuentran en `11_plan_pruebas.md` y `15_estado_implementacion.md`.

| Criterios | Estado | Evidencia principal |
|---|---|---|
| `AC-CA-001` a `AC-CA-011` | `passed` | regresión, Redis/PostgreSQL real, contratos internos, normalización y lifecycle |
| `AC-CA-012` a `AC-CA-019` | `passed` | snapshot/backfill, migraciones, locks, watermarks, registro y resolución histórica |
| `AC-CA-020` a `AC-CA-026` | `passed` | query API, aislamiento, ranking/fallback, emisión e hidratación DragonFullAV |
| `AC-CA-027` y `AC-CA-028` | `passed` | métricas, health/readiness, alertas, flags, imágenes y rollback operativo |
| `AC-CA-029` | `passed` | Dataset distribuido: 100.000 sujetos/500.000 eventos/20 tenants; proyección 4m20,496s, API HTTP p95 23,785 ms y backfill 59,732 s |
| `AC-CA-030` | `passed` | `scripts/certify_subject_mvp.py`: emisor → HTTP → Redis → resolver → normalización → proyecciones → query → hidratación SQL; lifecycle y rollback con persistencia web |
