# Registro De Contextos

## Propiedad Y Publicación

El proyecto principal mantiene el JSON canónico, lo valida en CI y lo sirve completo mediante el endpoint interno. GoAnalytics no edita el registro.

Familias MVP:

```text
search
selection
transaction
service
support
content
navigation
```

Agregar una familia requiere cambio de contrato, pruebas y despliegue coordinado; no existe `other`.

## Validaciones

- `schema_version=1`.
- `registry_version` entero positivo y no decreciente.
- `registry_revision` no vacía y estable.
- Clave y versión únicas.
- `context_version > 0`.
- Familia en allowlist.
- Estado `active` o `inactive`.
- Padre existente por clave y versión.
- Cero o un padre.
- Sin ciclos.
- Padre e hijo de la misma familia en el MVP.
- Definición previamente publicada inmutable.
- Misma revisión con payload distinto rechazada.

El archivo machine-readable previsto es `contracts/context-registry/v1.schema.json`. La validación Go combina decode tipado y reglas de dominio; no depende de lógica del proyecto principal.

## Persistencia Atómica

Una carga válida se persiste en una transacción:

1. Insertar o verificar revisión y checksum.
2. Insertar definiciones nuevas.
3. Comparar definiciones existentes para garantizar inmutabilidad.
4. Insertar entries y estados.
5. Validar padres y ciclos.
6. Activar revisión.
7. Desactivar la revisión activa anterior sin borrar sus filas.

Una falla revierte todo. La última revisión válida continúa activa.

## Orden De Carga

1. Snapshot en memoria.
2. Revisión activa en Redis.
3. Revisión activa en PostgreSQL.
4. Endpoint del proyecto principal.

PostgreSQL es el fallback durable. Redis almacena revisión y payload validado con TTL de 86.400 segundos.

## Lock Distribuido

Clave:

```text
goanalytics:subject-context-registry:rehydrate-lock
```

Adquisición con `SET NX PX` y token aleatorio. Liberación mediante Lua que elimina solo si el token coincide. TTL inicial 30 segundos y timeout HTTP 2 segundos.

Si otro proceso tiene el lock, se usa la última revisión válida. No se espera bloqueando el procesamiento de eventos.

## Resolución Durante Normalización

La clave es `context_key + context_version` en la revisión activa.

- Entry active: `registered` y persiste definition/revision.
- Entry inactive: `inactive_context`.
- Definición ausente: `unregistered` y upsert de contador pendiente.
- Familia declarada distinta: conservar claimed, usar resolved y emitir métrica.
- Sin contexto: no crea agregado contextual y no es error.

La resolución queda congelada para ese evento. Publicar otra revisión no reinterpreta automáticamente historia.

## Resolución Inicial De Fase 4

Comando:

```powershell
subject-jobs resolve-contexts --registry-revision <revision>
```

Procesa filas `registry_disabled`, `unregistered` o `family_unresolved` por `id`, en lotes de 5.000. Solo actualiza columnas derivadas de contexto y `updated_at`; no modifica `analytics_events`.

El comando es idempotente y reanudable mediante checkpoint propio. Al completarse, se ejecuta `project-context` para materializar los días afectados.

No implementa conciliación continua. Una revisión futura que registre un contexto previamente desconocido requerirá ejecución manual hasta la Fase 5.

## Revisión Inválida

- No se publica en memoria ni Redis.
- Se registra checksum, motivo y `validation_status=invalid` cuando sea seguro hacerlo.
- No desplaza la revisión activa.
- Eventos nuevos continúan con la revisión válida anterior.
- Se emiten log, contador y alerta si no existe ninguna revisión válida.
