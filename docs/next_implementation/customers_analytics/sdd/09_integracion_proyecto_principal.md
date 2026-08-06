# Integración Del Proyecto Principal

## Responsabilidades

El proyecto principal:

- extiende el resolver con `source` y `timezone`;
- publica snapshot de clientes;
- publica registro de contextos;
- emite asociaciones y lifecycle;
- consulta sugerencias;
- recupera datos reales por ID externo;
- aplica elegibilidad comercial adicional.

Este SDD no inventa rutas internas de código porque el repositorio consumidor no está disponible. Los contratos HTTP y comportamientos son obligatorios.

## Emisión Después Del Commit

`subject_associated` se emite únicamente después de confirmar la operación de negocio. Lifecycle se emite después de persistir el nuevo estado del cliente.

Política de retry directo:

```text
intento 1: inmediato
intento 2: espera 250 ms
intento 3: espera 1000 ms
timeout HTTP por intento: 1000 ms
```

Reglas:

- Reutilizar el mismo `event_id` e `idempotency_key` en los tres intentos.
- Reintentar timeouts, errores de red, `429` y `5xx`.
- No reintentar `400` o `401`; registrar error de contrato o configuración.
- Un `202` termina la entrega.
- El fracaso final no revierte ni cambia la respuesta de la operación de negocio.
- Emitir contador y log sin PII; no lanzar excepción fuera del adaptador analítico.

Riesgo aceptado: una caída después del commit o después del último intento puede perder la señal. Lifecycle se recupera ejecutando nuevo snapshot; asociaciones no se recuperan automáticamente en el MVP.

## Idempotency Keys

Formato recomendado:

```text
subject_associated:{tenant_id}:{operation_type}:{operation_id}:{subject_id}:{relation}
lifecycle:{tenant_id}:{subject_id}:{source_updated_at}:{source_mutation_id}
```

Los componentes se escapan o hashean si contienen separadores. La clave no incluye PII y permanece estable para el mismo hecho.

## Snapshot

El backend crea una vista consistente o mecanismo equivalente para fijar `source_revision`. Todas las páginas de una revisión deben reflejar el mismo corte lógico.

Orden de cursor recomendado:

```text
tenant_id ASC, external_subject_id ASC
```

El cursor es opaco para GoAnalytics. La revisión debe permanecer disponible durante al menos 2 horas, suficiente para el objetivo de backfill de 100.000 sujetos.

## Registro De Contextos

El archivo canónico vive en control de versiones del proyecto principal. CI valida schema, claves, versiones, familias, padres, ciclos e inmutabilidad respecto de la última versión publicada.

Publicación:

1. Aprobar cambio por revisión de código.
2. Validar contrato.
3. Desplegar endpoint con nueva revisión.
4. Ejecutar validación/rehidratación en GoAnalytics.
5. Activar contextos en emisores después de que la revisión sea válida.

No reutilizar silenciosamente una versión para cambiar semántica.

## Consumo De Sugerencias

Flujo:

1. El backend consulta GoAnalytics con tenant y contexto exactos.
2. Recibe IDs externos y evidencia agregada.
3. Busca clientes en su propia base, siempre dentro del tenant.
4. Excluye clientes inexistentes, no autorizados o comercialmente inelegibles.
5. Conserva el orden de GoAnalytics entre resultados supervivientes.
6. Si faltan resultados, aplica su comportamiento actual; no consulta tablas internas.

La integración debe tolerar `503`, timeout y respuesta vacía sin impedir el flujo principal. Las sugerencias son mejora opcional, no dependencia crítica.

## Activación En El Proyecto Principal

- La emisión puede habilitarse antes que la lectura.
- La lectura se habilita solo después de backfill y una proyección exitosa.
- El ranking contextual se habilita después de registro y resolución inicial.
- Debe existir flag local para volver inmediatamente al comportamiento anterior.

## Datos Prohibidos

No enviar nombre, email, teléfono, documento, dirección, texto libre, notas ni payload de cliente. Solo IDs técnicos, estado, timestamps, mutación, relación, operación y contexto controlado.
