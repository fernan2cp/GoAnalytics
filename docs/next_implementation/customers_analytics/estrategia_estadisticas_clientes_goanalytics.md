# Estrategia de estadísticas de clientes en GoAnalytics

## Estado del documento

**Estado:** estrategia de producto y arquitectura parcialmente cerrada.  
**Proyecto:** GoAnalytics.  
**Alcance:** captura, normalización, agregación y consulta de estadísticas de sujetos analíticos, comenzando por clientes.  
**Fecha de consolidación:** 2026-08-05.

Este documento reúne las decisiones definidas durante las iteraciones de diseño. Las decisiones todavía no cerradas se enumeran al final y no deben interpretarse como requisitos aprobados.

---

# 1. Objetivo

Extender GoAnalytics para capturar y explotar estadísticas relacionadas con clientes de manera:

- agnóstica respecto del dominio de negocio;
- reutilizable entre distintos proyectos;
- compatible con el modelo multi-tenant existente;
- reconstruible desde eventos crudos;
- preparada para recomendaciones y análisis de comportamiento;
- flexible sin depender de texto libre no controlado;
- desacoplada de las reglas comerciales del proyecto consumidor.

La primera necesidad funcional es sugerir clientes utilizados frecuentemente dentro de un tenant, globalmente y según contexto.

En etapas posteriores, la misma base deberá permitir analizar comportamiento por día de la semana, día del mes, mes, trimestre, año, navegación, turnos, servicios, ventas, productos, pagos y otras operaciones definidas por el proyecto consumidor.

GoAnalytics debe conservar datos crudos suficientes para análisis posteriores, pero no debe incorporar reglas específicas de agenda, POS, ecommerce, pagos u otros dominios.

---

# 2. Principios de diseño

## 2.1 Fuente de verdad

`analytics_events` continúa siendo la fuente de verdad auditable.

Las tablas de sujetos, contextos y agregados son proyecciones derivadas y deben poder reconstruirse a partir de eventos crudos, el registro versionado de contextos, la metadata hidratada de tenants y los eventos de ciclo de vida de sujetos.

El evento crudo no debe modificarse durante conciliaciones o reprocesamientos.

## 2.2 Separación de responsabilidades

GoAnalytics será responsable de:

- recibir eventos;
- validar contratos;
- resolver tenant y contexto;
- deduplicar;
- persistir eventos crudos;
- normalizar sujetos analíticos;
- mantener el estado analítico de los sujetos;
- generar agregados genéricos;
- exponer sugerencias mediante una API interna;
- conservar trazabilidad de cálculos y conciliaciones.

El proyecto consumidor será responsable de:

- decidir cuándo ocurrió un evento de negocio;
- decidir cuándo una selección o asociación es válida;
- mantener el archivo JSON canónico de contextos;
- informar cambios de estado de clientes;
- emitir identificadores externos correctos;
- recuperar los datos reales del cliente;
- aplicar elegibilidad adicional antes de mostrar una sugerencia;
- decidir cómo se utiliza la información analítica.

## 2.3 Agnosticismo

GoAnalytics no debe conocer conceptos como turno, orden, venta de mostrador, alumno, paciente, socio, reserva o medio de pago específico.

Debe trabajar con conceptos genéricos:

- evento;
- actor;
- sujeto;
- relación;
- contexto;
- familia de contexto;
- operación;
- dimensión temporal.

## 2.4 Identificadores opacos

Los identificadores provenientes del proyecto consumidor deben tratarse como valores opacos.

GoAnalytics no debe inferir semántica a partir del formato, prefijo, longitud, secuencia o coincidencias entre tenants.

---

# 3. Alcance inicial

La primera versión incluirá:

1. Identidad analítica interna de clientes.
2. Contextos globales definidos por el proyecto consumidor.
3. Familias genéricas obligatorias.
4. Captura de eventos relacionados con sujetos.
5. Conteos separados por tipo de señal.
6. Agregados diarios.
7. Ranking contextual con fallback global por tenant.
8. Ventanas configurables de 30 y 90 días por defecto.
9. API interna de sugerencias.
10. Backfill inicial de clientes.
11. Hidratación perezosa de sujetos.
12. Eventos de ciclo de vida.
13. Conciliación manual de contextos y sujetos.
14. Token Bearer para la API interna.

No se incluirá inicialmente:

- correlación de personas entre tenants;
- datos personales como email o teléfono;
- scoring ponderado;
- recomendaciones por operador;
- recomendaciones por sucursal;
- mezcla entre ranking contextual y global;
- fallback por familia;
- roll-up automático de jerarquías;
- automatización periódica de conciliaciones;
- identificación probabilística;
- reglas de negocio para elegibilidad;
- exposición pública de la API;
- acceso directo del consumidor a las tablas internas.

---

# 4. Modelo de identidad analítica

## 4.1 Unicidad

Un cliente es único dentro de un tenant.

La identidad lógica se define mediante:

```text
tenant_id + subject_type + external_subject_id
```

Restricción recomendada:

```sql
UNIQUE (tenant_id, subject_type, external_subject_id)
```

## 4.2 Identidad interna

Se utilizará una identidad analítica interna independiente de la clave externa.

Modelo conceptual:

```text
analytics_subjects
- id
- tenant_id
- subject_type
- external_subject_id
- lifecycle_status
- lifecycle_version
- merged_into_subject_id
- lifecycle_effective_at
- status_updated_at
- created_at
- updated_at
```

Para clientes:

```text
subject_type = customer
```

## 4.3 Sin correlación entre tenants

En la primera versión:

- una identidad analítica pertenece exactamente a un tenant;
- no se comparan clientes entre tenants;
- no se vinculan identidades;
- no se almacenan emails, teléfonos ni nicknames;
- IDs externos iguales en tenants diferentes representan sujetos diferentes.

Toda capacidad de identidad global o análisis cruzado de personas queda diferida.

## 4.4 Proyecto consumidor único

Una instancia de GoAnalytics servirá siempre a un único proyecto consumidor.

Esto permite utilizar un único registro semántico de contextos para todos los tenants sin introducir `project_id` o `integration_id`.

La arquitectura deberá documentar esta restricción. Si en el futuro una instancia admite múltiples proyectos consumidores, será obligatorio agregar aislamiento por proyecto.

---

# 5. Creación y precarga de sujetos

## 5.1 Backfill inicial

El proyecto consumidor podrá precargar los clientes existentes.

El backfill debe ser independiente del arranque crítico, no bloqueante, paginado, reanudable, idempotente, auditable y ejecutado una vez por instalación o cuando se solicite explícitamente.

No debe repetirse completamente cada vez que se recrea el container.

Modelo conceptual del seguimiento:

```text
analytics_subject_bootstrap
- id
- subject_type
- status
- cursor
- source_revision
- started_at
- completed_at
- processed_count
- failed_count
- last_error
```

## 5.2 Operación normal

Durante la operación normal se utilizará hidratación perezosa.

Cuando llegue un evento para un sujeto inexistente:

1. Se busca por `tenant_id + subject_type + external_subject_id`.
2. Si no existe, se crea.
3. La creación debe soportar concurrencia.
4. El evento continúa utilizando el sujeto existente o recién creado.

La restricción única evita duplicados entre backfill, eventos concurrentes, reintentos y múltiples workers.

---

# 6. Actor, sujeto y operación

## 6.1 Actor

El actor es quien origina la interacción:

- usuario autenticado;
- operador;
- visitante anónimo;
- proceso del backend.

Se representa con los campos existentes como `user_id`, `anonymous_id` y `session_id`.

## 6.2 Sujeto

El sujeto es la entidad sobre la cual se registra el comportamiento:

```text
subject_type
subject_id
```

Ejemplos posibles:

- customer;
- item;
- service;
- article;
- feature.

La primera implementación se concentra en `customer`, pero el modelo no debe quedar atado a clientes.

## 6.3 Operación

Una operación es una entidad funcional opaca vinculada al evento:

```text
operation_type
operation_id
```

GoAnalytics no interpreta su significado.

---

# 7. Contratos de eventos de sujetos

## 7.1 Eventos de interacción

Se mantendrán contratos separados.

### `subject_impression`

El sujeto fue mostrado efectivamente.

Sirve para entender qué está mostrando el sistema, exposición, frecuencia de aparición y calidad futura de una recomendación.

No es la señal principal para sugerencias.

### `subject_viewed`

El usuario abrió o inspeccionó el sujeto.

Sirve para analizar interés y comportamiento de navegación.

No es la señal principal para sugerencias.

### `subject_selected`

Hubo una selección explícita en una interfaz o flujo.

Debe utilizarse cuando el proyecto consumidor determina que realmente ocurrió una selección.

No debe inferirse por renderizado, click técnico o cambio de foco.

### `subject_associated`

El sujeto quedó asociado a una operación confirmada.

Será la señal principal para las recomendaciones iniciales.

Ejemplos:

- cliente asociado a una orden confirmada;
- cliente asociado a un turno creado;
- cliente asociado a una inscripción confirmada.

La decisión de emitirlo pertenece al proyecto consumidor.

## 7.2 Ejemplo de `subject_selected`

```json
{
  "event_name": "subject_selected",
  "event_version": 1,
  "properties": {
    "subject_type": "customer",
    "subject_id": "customer_123"
  },
  "context": {
    "context_key": "appointments.customer_picker",
    "context_version": 1
  }
}
```

## 7.3 Ejemplo de `subject_associated`

```json
{
  "event_name": "subject_associated",
  "event_version": 1,
  "idempotency_key": "appointment:845:customer:123",
  "properties": {
    "subject_type": "customer",
    "subject_id": "customer_123",
    "relation": "attendee",
    "operation_type": "appointment",
    "operation_id": "845"
  },
  "context": {
    "context_key": "appointments.customer_assignment",
    "context_version": 1
  }
}
```

## 7.4 Responsabilidad semántica

GoAnalytics no decide qué acción representa una selección o una asociación.

El proyecto consumidor debe:

- emitir `subject_selected` cuando corresponda;
- emitir `subject_associated` cuando la operación se confirme;
- proporcionar `logical_event_id` o `idempotency_key`;
- evitar duplicar señales funcionales.

## 7.5 Cambios de asociación

En la primera versión:

- los cambios no eliminan eventos anteriores;
- se mantiene el comportamiento histórico;
- el nuevo sujeto genera su propia asociación;
- el sujeto anterior conserva la asociación histórica.

Mejora futura:

```text
subject_association_changed
```

Este contrato podrá registrar sujeto anterior, sujeto nuevo, relación, operación y motivo del cambio.

---

# 8. Ciclo de vida del sujeto

## 8.1 Eventos definidos

Se emitirán:

- `subject_deactivated`;
- `subject_reactivated`;
- `subject_merged`.

Opcionalmente, una evolución futura podrá agregar `subject_deleted` u otros estados.

## 8.2 Separación entre historial y elegibilidad

Los eventos históricos no se eliminan ni se transfieren automáticamente.

El estado actual determina si el sujeto puede aparecer en sugerencias.

```text
Agregados históricos
+
Estado actual elegible
=
Resultado de sugerencias
```

## 8.3 Orden de eventos

Los eventos pueden llegar fuera de orden.

Se recomienda que el proyecto consumidor envíe una versión monotónica:

```text
lifecycle_version
```

Ejemplo:

```json
{
  "event_name": "subject_reactivated",
  "idempotency_key": "customer:123:lifecycle:8",
  "properties": {
    "subject_type": "customer",
    "subject_id": "customer_123",
    "lifecycle_version": 8,
    "effective_at": "2026-08-05T19:30:00-03:00"
  }
}
```

Regla prevista:

```text
incoming.lifecycle_version > current.lifecycle_version
```

La disponibilidad real de esta versión monotónica todavía debe confirmarse.

## 8.4 Fusión

Cuando un cliente se fusiona:

- el sujeto anterior queda en estado `merged`;
- `merged_into_subject_id` apunta al sujeto canónico;
- los eventos históricos permanecen en el sujeto anterior;
- los eventos futuros deben emitirse con el cliente canónico;
- no se transfieren conteos automáticamente en la primera versión.

---

# 9. Sistema de contextos

## 9.1 Objetivo

Los contextos deben permitir diferenciar el uso de un mismo sujeto sin incorporar reglas de negocio en GoAnalytics.

Separación conceptual:

```text
event_name
Qué ocurrió.

subject
Sobre qué entidad ocurrió.

context_family
Categoría genérica comparable.

context_key
Dónde o para qué ocurrió dentro del proyecto consumidor.
```

## 9.2 Registro global

Habrá un único registro de contextos para todo el proyecto consumidor.

No existirán definiciones independientes por tenant.

Consecuencias:

- una clave significa lo mismo en todos los tenants;
- se facilita la comparación entre tenants;
- se evita una capa posterior de homologación;
- un error en el registro puede afectar transversalmente a todos los tenants.

## 9.3 Archivo JSON canónico

La primera versión utilizará un archivo JSON versionado.

```json
{
  "schema_version": 1,
  "registry_version": 4,
  "contexts": [
    {
      "context_key": "appointments.customer_picker",
      "context_version": 1,
      "context_family": "search",
      "parent_context_key": null,
      "status": "active"
    },
    {
      "context_key": "pos.customer_picker",
      "context_version": 1,
      "context_family": "search",
      "parent_context_key": null,
      "status": "active"
    }
  ]
}
```

## 9.4 Reglas del registro

El archivo debe validar:

- JSON Schema;
- versión de esquema;
- revisión;
- claves únicas;
- versiones únicas;
- familias existentes;
- padres existentes;
- ausencia de ciclos;
- estados válidos;
- inmutabilidad de versiones publicadas;
- compatibilidad hacia atrás.

La rigidez depende de validación en CI, validación al cargar, revisión inmutable, aplicación atómica y prohibición de modificar silenciosamente versiones utilizadas.

## 9.5 Familias genéricas

Todo contexto debe mapearse a una familia genérica.

Las familias deben ser pocas, estables, controladas, independientes del negocio y mantenidas como contrato de GoAnalytics.

Ejemplos preliminares:

- search;
- selection;
- transaction;
- service;
- support;
- content;
- navigation.

La lista definitiva todavía debe cerrarse durante el SDD.

No se recomienda una familia permanente `other`.

Cuando un contexto no pueda conciliarse con ninguna familia:

- se marca `family_unresolved`;
- se genera log estructurado;
- se incrementa una métrica;
- se informa para evaluar una nueva familia;
- no se fuerza una clasificación incorrecta.

## 9.6 Clave y versión

Cada evento envía:

```text
context_key
context_version
```

La definición se resuelve mediante:

```text
context_key + context_version
```

El evento puede declarar una familia, pero se considera no verificada hasta resolver la definición:

```text
claimed_context_family
resolved_context_family
```

Los agregados confiables utilizan `resolved_context_family`.

## 9.7 Jerarquía

La jerarquía es opcional.

Reglas:

- cero o un padre;
- mismo registro global;
- sin ciclos;
- cambiar el padre requiere una nueva versión;
- no se permiten relaciones M2M;
- la familia del hijo debe ser compatible con la del padre.

En la primera versión se almacena y valida, pero no se utiliza para roll-up ni fallback.

## 9.8 Contextos no registrados

Si llega un `context_key` desconocido:

- el evento crudo se guarda;
- se conserva el `context_key`;
- se conserva la versión;
- se conserva la familia declarada;
- se marca como `unregistered`;
- se excluye del agregado contextual;
- puede contribuir al ranking global si el evento y el sujeto son válidos;
- se incorpora a conciliación pendiente.

Modelo conceptual:

```text
analytics_unregistered_contexts
- context_key
- context_version
- claimed_context_family
- first_seen_at
- last_seen_at
- event_count
- reconciliation_status
- last_reconciliation_attempt_at
- last_error
```

## 9.9 Contextos inactivos

Una definición desactivada afecta solo a eventos futuros.

Eventos futuros con una definición inactiva:

- se persisten;
- se marcan `inactive_context`;
- se excluyen de agregados contextuales;
- pueden contribuir al ranking global si son válidos.

Los eventos históricos conservan su significado y participación.

---

# 10. Hidratación del registro de contextos

## 10.1 Fuente canónica

El proyecto consumidor mantiene el archivo JSON.

GoAnalytics no debe leer una copia independiente ni compartir un `.env` con el backend.

El `.env` indica cómo obtener el registro, no contiene las definiciones.

## 10.2 Endpoint interno

El proyecto consumidor expondrá un endpoint conceptual:

```http
GET /internal/analytics/context-registry
Authorization: Bearer <CONTEXT_REGISTRY_TOKEN>
```

Respuesta:

```json
{
  "schema_version": 1,
  "registry_version": 4,
  "registry_revision": "sha256:8d2f...",
  "contexts": []
}
```

## 10.3 Carga inicial

Al iniciar:

1. GoAnalytics intenta cargar la última revisión válida desde Redis.
2. Si no existe, consulta al proyecto consumidor.
3. Valida el registro completo.
4. Guarda un snapshot persistente.
5. Publica la revisión en Redis.
6. Continúa operando con esa revisión.

## 10.4 Hidratación perezosa

Durante la operación:

- se resuelve desde memoria o Redis;
- si la revisión no está disponible, se rehidrata;
- se aplica cooldown;
- se evita la descarga simultánea por múltiples workers;
- se utiliza lock distribuido o patrón `singleflight`.

El registro se obtiene completo y de manera atómica. No se resuelven contextos individualmente.

## 10.5 Revisión inválida

Si el JSON es inválido:

- no se reemplaza la última revisión válida;
- Redis conserva la revisión anterior;
- la ingesta continúa;
- los nuevos contextos desconocidos se registran como `unregistered`;
- se emiten logs y métricas;
- se marca el registro como desactualizado;
- no se permite reconciliar con la revisión inválida.

## 10.6 Variables de entorno conceptuales

```env
CONTEXT_REGISTRY_ENABLED=true
CONTEXT_REGISTRY_RESOLVER_URL=http://backend/internal/analytics/context-registry
CONTEXT_REGISTRY_TOKEN=change_me
CONTEXT_REGISTRY_CACHE_TTL=24h
CONTEXT_REGISTRY_REHYDRATE_COOLDOWN_SECONDS=300
```

Los nombres finales deberán alinearse con las convenciones actuales del repositorio.

---

# 11. Normalización de eventos de sujetos

Modelo conceptual:

```text
analytics_event_subjects
- id
- analytics_event_id
- client_event_id
- logical_event_id
- tenant_id
- site_id
- event_name
- event_time
- received_at
- analytics_subject_id
- subject_type
- external_subject_id
- relation
- operation_type
- operation_id
- context_definition_id
- context_key
- context_version
- claimed_context_family
- resolved_context_family
- context_resolution_status
- metadata
- created_at
```

Relación:

```text
analytics_events 1:N analytics_event_subjects
```

Esto permite eventos con varios sujetos, reconstrucción, consultas eficientes, mantener el evento principal una sola vez y extender el modelo a entidades distintas de clientes.

---

# 12. Agregados temporales

## 12.1 Base diaria

La estrategia recomendada es materializar agregados diarios y derivar períodos superiores.

```text
analytics_subject_context_daily
- tenant_id
- analytics_subject_id
- context_definition_id nullable
- resolved_context_family nullable
- local_date
- impression_count
- view_count
- selection_count
- association_count
- projection_version
- computed_at
- created_at
- updated_at
```

Agregado global:

```text
analytics_subject_tenant_daily
- tenant_id
- analytics_subject_id
- local_date
- impression_count
- view_count
- selection_count
- association_count
- projection_version
- computed_at
```

## 12.2 Zona horaria

Los períodos se calculan usando la zona horaria del tenant.

La zona horaria proviene de la metadata ya hidratada por el resolver existente.

No debe obtenerse del evento individual.

La proyección debe conservar suficiente información para evitar reinterpretaciones silenciosas si cambia la zona horaria del tenant.

El tratamiento exacto de cambios históricos de zona horaria deberá definirse en el SDD.

## 12.3 Períodos derivados

Desde agregados diarios se podrán obtener:

- día de la semana;
- día del mes;
- semana;
- mes;
- trimestre;
- año;
- ventanas móviles.

No se crearán inicialmente tablas separadas para cada período.

## 12.4 Frecuencia de cálculo

La frecuencia será configurable mediante variables de entorno.

Valor inicial aceptable:

- una ejecución diaria;
- posibilidad futura de ejecutar cada varias horas.

```env
SUBJECT_PROJECTION_ENABLED=true
SUBJECT_PROJECTION_INTERVAL=24h
SUBJECT_PROJECTION_LOOKBACK=48h
SUBJECT_PROJECTION_BATCH_SIZE=5000
```

El `lookback` permite reprocesar eventos tardíos.

El job debe ser incremental, idempotente, reanudable, versionado, observable y reconstruible.

---

# 13. Conteos y señales

En la primera versión no se implementará scoring ponderado.

Se almacenarán conteos separados:

```text
impression_count
view_count
selection_count
association_count
```

La señal principal para sugerencias será:

```text
association_count
```

Los demás conteos servirán para analizar exposición, navegación, comparar lo mostrado con lo utilizado, preparar futuros algoritmos y detectar problemas de relevancia.

No se recomienda guardar una columna `score` sin definición.

Una versión futura podrá agregar definición de cálculo, pesos, ventanas, versión de algoritmo, score, normalización, recencia, frecuencia y ratios.

---

# 14. Ventanas de sugerencias

```env
SUBJECT_SUGGESTION_CONTEXT_WINDOW_DAYS=30
SUBJECT_SUGGESTION_GLOBAL_WINDOW_DAYS=90
```

Valores por defecto:

- contexto exacto: 30 días;
- ranking global del tenant: 90 días.

Las ventanas serán configurables.

No se utilizará inicialmente un acumulado histórico ilimitado para sugerencias.

---

# 15. Estrategia de fallback

La primera versión utilizará:

```text
contexto exacto
→ ranking global del tenant
```

Proceso:

1. Buscar sujetos frecuentes en el contexto exacto.
2. Ordenar por `association_count`.
3. Completar hasta el límite con el ranking global.
4. Eliminar duplicados.
5. Excluir sujetos no elegibles.

No se implementará inicialmente:

```text
contexto exacto
→ familia
→ global
```

El fallback por familia se registrará como mejora futura.

No se mezclan scores contextuales y globales. Cada resultado conserva su nivel de origen.

---

# 16. API interna de sugerencias

## 16.1 Acceso

La API estará dentro de una red privada, pero utilizará autenticación mediante token Bearer.

```env
QUERY_API_AUTH_ENABLED=true
QUERY_API_TOKEN=change_me
```

No se reutilizará el tracking JWT, el token del resolver de sites ni el token del registro de contextos.

## 16.2 Endpoint conceptual

```http
GET /v1/internal/subjects/suggestions
Authorization: Bearer <QUERY_API_TOKEN>
```

Parámetros:

```text
tenant_id
subject_type
context_key
context_version
limit
```

## 16.3 Respuesta conceptual

```json
{
  "tenant_id": "tenant_123",
  "subject_type": "customer",
  "context": {
    "context_key": "appointments.customer_picker",
    "context_version": 1
  },
  "window": {
    "context_days": 30,
    "global_days": 90
  },
  "computed_at": "2026-08-05T02:00:00Z",
  "projection_version": 1,
  "results": [
    {
      "analytics_subject_id": "01J...",
      "external_subject_id": "customer_123",
      "association_count": 14,
      "source_level": "exact_context"
    },
    {
      "analytics_subject_id": "01K...",
      "external_subject_id": "customer_456",
      "association_count": 25,
      "source_level": "tenant_global"
    }
  ]
}
```

La API devuelve identidad analítica, identidad externa, conteo, nivel de origen, ventana, versión de proyección y fecha de cálculo.

No devuelve nombre, email, teléfono, documento, información personal ni payload crudo.

## 16.4 Elegibilidad

La API excluye:

- sujetos inactivos;
- sujetos eliminados;
- sujetos fusionados;
- sujetos con estado no elegible.

El backend consumidor puede aplicar filtros adicionales.

---

# 17. Conciliación de contextos

## 17.1 Alcance global

El registro es único para todo el proyecto consumidor.

El comando principal debe reconciliar todos los tenants:

```bash
goanalytics reconcile-contexts   --all-pending   --registry-revision sha256:8d2f...
```

El filtro por tenant será opcional y operativo:

```bash
goanalytics reconcile-contexts   --tenant tenant_123   --dry-run
```

También podrán existir filtros por `context_key`, `context_version`, checkpoint, límite o rango de IDs.

## 17.2 Requisitos

La conciliación debe:

1. Usar una revisión válida y específica.
2. Recorrer todo el histórico pendiente.
3. No limitarse a la ventana de sugerencias.
4. Resolver por clave y versión.
5. Modificar solo proyecciones derivadas.
6. Mantener intacto el evento crudo.
7. Registrar filas procesadas.
8. Registrar errores.
9. Ser idempotente.
10. Ser reanudable.
11. Recalcular agregados afectados.
12. Permitir `dry-run`.

## 17.3 Segmentación interna

Aunque la ejecución sea global, el progreso puede segmentarse por:

```text
tenant_id
context_key
context_version
```

Esto permite reintentos parciales, evitar bloqueo global, estado `partially_completed`, diagnóstico y recuperación.

## 17.4 Operación inicial

La conciliación será manual mediante una capacidad oficial del proyecto.

No debe ser SQL manual ni un script desechable.

En una versión futura podrá automatizarse periódicamente, al publicar un registro, bajo demanda o mediante una cola.

## 17.5 Observabilidad

Métricas previstas:

```text
unregistered_contexts_count
oldest_unregistered_context_age
contexts_pending_reconciliation
context_reconciliation_failures
```

---

# 18. Conciliación de sujetos

## 18.1 Objetivo

Detectar y reparar diferencias entre el estado del sujeto en GoAnalytics y el estado actual en el proyecto consumidor.

Casos:

```text
missing_in_source
source_inactive
source_deleted
source_merged
conflicting_state
```

## 18.2 Modelo conceptual

```text
analytics_subject_reconciliation
- id
- analytics_subject_id
- reason
- status
- detected_at
- resolved_at
- resolution
- metadata
```

## 18.3 Operación inicial

La conciliación será manual:

```bash
goanalytics reconcile-subjects --all --dry-run
```

```bash
goanalytics reconcile-subjects --all
```

A futuro podrá automatizarse de forma periódica o a demanda.

## 18.4 Eventos como mecanismo principal

La conciliación no sustituye a los eventos de ciclo de vida.

```text
Eventos de lifecycle
→ actualización rápida

Conciliación
→ reparación de inconsistencias y datos históricos
```

## 18.5 Ausencia en origen

La ausencia de un sujeto en una consulta no debe interpretarse automáticamente como eliminación.

Puede representar error, paginación incompleta, inconsistencia temporal, fallo de integración o sujeto realmente eliminado.

La resolución debe ser explícita y auditable.

---

# 19. Mejoras futuras registradas

## 19.1 Fallback por familia

```text
contexto exacto
→ familia genérica
→ global del tenant
```

No forma parte de la primera versión.

## 19.2 Scoring versionado

Posible evolución:

- pesos por evento;
- recencia;
- frecuencia;
- normalización;
- ratios;
- definiciones versionadas;
- algoritmo versionado.

## 19.3 Jerarquía analítica

Posible evolución:

- roll-up a padres;
- fallback jerárquico;
- agregados por árbol;
- análisis comparativo por niveles.

## 19.4 Automatización de conciliación

Posible evolución:

- job periódico;
- ejecución al publicar registro;
- reconciliación por demanda;
- alertas;
- autocorrección controlada.

## 19.5 Identidad cruzada

Queda expresamente fuera del alcance actual.

Cualquier futura implementación deberá tratar consentimiento, privacidad, aislamiento, HMAC de identificadores, errores de coincidencia, auditoría, autorización y gobernanza de datos.

## 19.6 Asociación cambiada

Evento futuro:

```text
subject_association_changed
```

Permitirá analizar correcciones, reasignaciones, errores de selección y cambios manuales o automáticos.

---

# 20. Variables de entorno previstas

Los nombres son conceptuales y deberán alinearse con las convenciones del repositorio.

```env
# Registro de contextos
CONTEXT_REGISTRY_ENABLED=true
CONTEXT_REGISTRY_RESOLVER_URL=http://backend/internal/analytics/context-registry
CONTEXT_REGISTRY_TOKEN=change_me
CONTEXT_REGISTRY_CACHE_TTL=24h
CONTEXT_REGISTRY_REHYDRATE_COOLDOWN_SECONDS=300

# Proyecciones
SUBJECT_PROJECTION_ENABLED=true
SUBJECT_PROJECTION_INTERVAL=24h
SUBJECT_PROJECTION_LOOKBACK=48h
SUBJECT_PROJECTION_BATCH_SIZE=5000

# Sugerencias
SUBJECT_SUGGESTION_CONTEXT_WINDOW_DAYS=30
SUBJECT_SUGGESTION_GLOBAL_WINDOW_DAYS=90

# API de consulta
QUERY_API_AUTH_ENABLED=true
QUERY_API_TOKEN=change_me
```

No se debe utilizar `.env` para almacenar el catálogo de contextos.

---

# 21. Restricciones de seguridad

1. No almacenar datos personales innecesarios.
2. No utilizar email o teléfono para correlación.
3. No exponer payload crudo mediante la API de sugerencias.
4. Aislar consultas por tenant.
5. Utilizar credencial separada para la API.
6. No confiar únicamente en la red privada.
7. Mantener auditoría de conciliaciones.
8. Mantener versiones de registro.
9. No modificar eventos crudos.
10. No permitir que un contexto inválido detenga la ingesta.
11. No permitir acceso directo del consumidor a tablas internas.
12. Aplicar rate limiting futuro a la API si el volumen lo requiere.

---

# 22. Consideraciones operativas

## 22.1 Fallo del registro de contextos

GoAnalytics continúa con la última revisión válida.

## 22.2 Fallo del backend consumidor

La ingesta continúa mientras pueda resolver tenant y utilizar cachés existentes.

Los contextos desconocidos se almacenan como pendientes.

## 22.3 Fallo de proyección

La fuente cruda permanece intacta.

El job puede reanudarse desde checkpoint.

## 22.4 Eventos tardíos

Se reprocesan mediante una ventana de `lookback`.

## 22.5 Duplicados

Los eventos funcionales deben utilizar `logical_event_id`, `idempotency_key`, identificadores de operación y versión de lifecycle cuando corresponda.

## 22.6 Concurrencia

Las creaciones perezosas y los backfills deben converger mediante restricciones únicas.

---

# 23. Plan de implementación preliminar

## Fase 1 — Contratos y documentación

- Definir sujetos analíticos.
- Definir eventos de sujetos.
- Definir ciclo de vida.
- Definir contrato del registro JSON.
- Definir familias iniciales.
- Definir endpoint de registro.
- Definir API de sugerencias.
- Definir variables de entorno.
- Crear JSON Schema.
- Crear criterios de aceptación.

## Fase 2 — Modelo de datos

- Crear `analytics_subjects`.
- Crear `analytics_event_subjects`.
- Crear tablas de contextos.
- Crear tabla de contextos no registrados.
- Crear estado de bootstrap.
- Crear tablas de agregados diarios.
- Crear tablas de conciliación.
- Crear índices.
- Crear migraciones down simétricas.

## Fase 3 — Registro de contextos

- Implementar carga desde backend.
- Implementar validación.
- Implementar revisión.
- Implementar snapshot local.
- Implementar caché Redis.
- Implementar cooldown.
- Implementar exclusión mutua.
- Implementar fallback a última revisión válida.

## Fase 4 — Normalización de sujetos

- Crear sujetos de forma perezosa.
- Normalizar eventos.
- Persistir relación 1:N.
- Implementar deduplicación.
- Implementar eventos de lifecycle.
- Implementar estados de elegibilidad.

## Fase 5 — Backfill inicial

- Crear endpoint paginado en el consumidor.
- Implementar job independiente.
- Implementar cursor.
- Implementar reanudación.
- Implementar métricas.
- Probar concurrencia con creación perezosa.

## Fase 6 — Agregados

- Implementar proyección diaria.
- Aplicar zona horaria del tenant.
- Implementar contexto exacto.
- Implementar agregado global.
- Implementar lookback.
- Implementar checkpoints.
- Implementar reconstrucción.

## Fase 7 — API

- Implementar autenticación Bearer.
- Implementar aislamiento por tenant.
- Implementar fallback.
- Excluir sujetos no elegibles.
- Devolver IDs interno y externo.
- Devolver trazabilidad de cálculo.

## Fase 8 — Conciliación

- Implementar conciliación manual de contextos.
- Implementar `dry-run`.
- Implementar conciliación global.
- Implementar filtros operativos.
- Implementar conciliación de sujetos.
- Implementar auditoría y métricas.

## Fase 9 — Pruebas

- Contratos.
- Migraciones.
- Concurrencia.
- Deduplicación.
- Eventos fuera de orden.
- Contextos inválidos.
- Registro desactualizado.
- Fallback.
- Aislamiento.
- Zona horaria.
- Reconciliación.
- Backfill.
- Eventos tardíos.

---

# 24. Criterios de aceptación preliminares

1. Un sujeto se identifica de forma única dentro de un tenant.
2. IDs iguales en tenants distintos generan sujetos distintos.
3. El backfill y la creación perezosa no duplican sujetos.
4. El evento crudo permanece intacto.
5. Todo contexto válido se resuelve contra una revisión conocida.
6. Un contexto desconocido se registra como `unregistered`.
7. El `context_key` desconocido permanece auditable.
8. Una revisión inválida no reemplaza la última válida.
9. Todo contexto pertenece a una familia.
10. La jerarquía no admite ciclos.
11. `subject_associated` incrementa el conteo correcto.
12. Los conteos se mantienen separados.
13. Los agregados diarios utilizan la zona horaria del tenant.
14. La consulta contextual utiliza la ventana configurada.
15. El fallback completa con el ranking global.
16. No se repiten sujetos en la respuesta.
17. Los sujetos no elegibles no se sugieren.
18. La API devuelve IDs interno y externo.
19. La API no devuelve datos personales.
20. La API valida un token dedicado.
21. La conciliación global procesa todos los tenants.
22. La conciliación es idempotente y reanudable.
23. Los filtros por tenant son opcionales y operativos.
24. Los eventos históricos sobreviven a cambios de estado.
25. Una fusión no transfiere conteos en la primera versión.

---

# 25. Decisiones todavía pendientes

Estas decisiones no están cerradas y deberán resolverse antes del SDD final.

## 25.1 Versión monotónica de lifecycle

Debe confirmarse si el backend consumidor dispone de:

```text
customer.version
customer.updated_sequence
```

Si no existe, deberá definirse una alternativa basada en `updated_at`, desempate determinista o secuencia generada por integración.

## 25.2 Alcance del backfill

Debe definirse si incluirá:

- solo clientes activos;
- activos e inactivos;
- fusionados;
- eliminados representables.

La recomendación preliminar es incluir todos los estados disponibles.

## 25.3 Base diaria

Debe confirmarse formalmente que los agregados diarios serán la fuente para semana, mes, trimestre y año.

## 25.4 Datos desactualizados

Debe decidirse si la API puede servir la última proyección válida cuando el job diario falle.

Recomendación preliminar:

```json
{
  "computed_at": "2026-08-05T02:00:00Z",
  "is_stale": true,
  "projection_version": 1
}
```

## 25.5 Familias iniciales

La lista definitiva debe revisarse para evitar demasiadas familias, conceptos superpuestos, familias ligadas al negocio o una categoría residual permanente.

## 25.6 Cambios de zona horaria

Debe definirse cómo se tratan agregados históricos si cambia la zona horaria de un tenant.

---

# 26. Resultado esperado de la primera versión

Al finalizar la primera implementación, GoAnalytics deberá:

- capturar relaciones entre eventos y clientes;
- identificar clientes mediante una identidad analítica interna;
- manejar todos los tenants en una única base;
- resolver contextos desde un registro global;
- conservar contextos desconocidos para conciliación;
- mantener conteos separados;
- generar agregados diarios;
- sugerir clientes por contexto;
- completar resultados con un ranking global;
- respetar la zona horaria del tenant;
- excluir clientes no elegibles;
- exponer una API interna segura;
- permitir backfill y conciliación manual;
- conservar eventos crudos suficientes para evoluciones futuras.

La solución deberá mejorar las capacidades de análisis sin convertir GoAnalytics en un sistema de gestión de clientes ni en un motor de reglas de negocio.
