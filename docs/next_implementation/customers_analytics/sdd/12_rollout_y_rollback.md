# Rollout Y Rollback

## Principios

- Migrations primero, capacidades después.
- Todos los flags nuevos comienzan en `false`.
- Emisión antes que lectura.
- Global antes que contextual.
- Rollback operativo sin borrar datos.
- Ningún fallo analítico bloquea negocio del proyecto principal.

## Etapa 1: Fase 0

1. Desplegar recuperación Redis e idempotencia por evento.
2. Mantener funciones de sujetos deshabilitadas.
3. Observar pendientes, duplicados y rechazo por al menos un ciclo operativo.

Gate: cero regresiones y recuperación verificada en entorno integrado.

Rollback: restaurar binario anterior si no hay mensajes cuyo manejo dependa del nuevo formato. La migración `005` puede permanecer aplicada.

## Etapa 2: Contratos Fuente

1. Desplegar resolver con `source` opcional y timezone.
2. Desplegar snapshot y registro en proyecto principal.
3. Configurar secretos separados.
4. Validar contratos desde GoAnalytics sin habilitar normalización.

Gate: contract tests y zona válida para todos los sites activos.

## Etapa 3: Fase 1

1. Aplicar `006`.
2. Desplegar endpoint interno y worker actualizado con flags false.
3. Habilitar `SUBJECT_ANALYTICS_ENABLED` en un entorno no productivo.
4. Habilitar emisión del proyecto principal.
5. Verificar normalización y lifecycle.

Rollback: apagar emisión y flag. Eventos crudos y tablas permanecen.

## Etapa 4: Fase 2

1. Ejecutar backfill en dry validation contractual.
2. Ejecutar bootstrap real.
3. Confirmar conteos por estado y ausencia de PII.
4. Corregir errores y repetir con snapshot nuevo si corresponde.

Gate: bootstrap completed, 100% de páginas procesadas y sin conflictos no explicados.

Rollback: detener job. No revertir upserts; un snapshot posterior puede corregir estados.

## Etapa 5: Fase 3

1. Aplicar `007`.
2. Desplegar `subject-jobs` y `query-api` separados.
3. Ejecutar proyección global manual.
4. Validar frescura, latencia e aislamiento.
5. Habilitar consulta global en el proyecto principal detrás de flag.

Rollback: apagar flag consumidor y query API; detener scheduler. Conservar agregados.

## Etapa 6: Fase 4

1. Aplicar `008`.
2. Publicar primera revisión del registro.
3. Habilitar loader y validar revisión.
4. Ejecutar `resolve-contexts`.
5. Ejecutar `project-context`.
6. Habilitar consulta contextual en proyecto principal.

Gate: revisión válida, resolución completa, proyección completed y fallback probado.

Rollback: volver a ranking global y apagar contexto. No desactivar normalización global.

## Rollback De Migraciones

Los archivos down existen y se prueban, pero no forman parte del rollback operativo productivo. Ejecutarlos elimina datos derivados y requiere:

- backup verificado;
- ventana de mantenimiento;
- confirmación explícita del alcance;
- consumidores y procesos detenidos;
- aplicación en orden `008`, `007`, `006`, `005`.

## Compatibilidad De Caché

El payload Redis de site acepta ausencia de timezone en entradas antiguas. Al habilitar sujetos, una entrada sin timezone se considera miss funcional y se rehidrata; no se interpreta como UTC.

## Rotación De Secretos

El MVP usa un token activo por función. La rotación se hace desplegando primero el consumidor del token nuevo y luego el productor. Una evolución puede admitir token actual/anterior si la operación lo requiere.

## Criterio De Finalización MVP

El MVP se considera disponible cuando Fases 0–4 tienen sus criterios cumplidos, el proyecto principal usa sugerencias contextuales detrás de flag y existe evidencia de rollback global sin afectar la operación principal.
