# Estrategia de estadísticas de clientes en GoAnalytics

## Estado del documento

**Estado:** estrategia alineada con el código existente; requiere SDD por fase antes de implementar.
**Proyecto:** GoAnalytics.  
**Alcance:** captura, normalización, agregación y consulta de estadísticas de sujetos analíticos, comenzando por clientes.  
**Código contrastado:** servicios de ingesta y worker, SDK web, migraciones PostgreSQL y contratos vigentes.
**Fecha de consolidación:** 2026-08-05.

Este documento reúne las decisiones definidas durante las iteraciones de diseño y las contrasta con el repositorio real. No describe funcionalidad ya disponible salvo cuando se indica expresamente en el baseline. Las decisiones todavía no cerradas se enumeran al final y no deben interpretarse como requisitos aprobados.

La solución completa es factible sobre la arquitectura actual, pero no es una ampliación pequeña: incorpora identidad analítica, contratos server-to-server, proyecciones, consulta y operación de jobs. Se conservará el diseño completo y se implementará, si se aprueba, en fases incrementales protegidas por configuración.

---

# 0. Baseline del código real

## 0.1 Capacidades existentes y reutilizables

| Capacidad | Estado real | Reutilización prevista |
| --- | --- | --- |
| `POST /v1/events` con tracking JWT | Implementado en `services/ingest` | Se conserva para tracking web sin cambios incompatibles. |
| Eventos genéricos con `properties` y `context` | Implementado en ingesta, stream y worker | Permite agregar eventos y campos opcionales. |
| Redis Stream | Implementado entre ingesta y worker | Continúa como transporte asíncrono. |
| Resolución de site y tenant | Implementada por HTTP interno, caché Redis y cooldown | Se extiende de forma aditiva para zona horaria. |
| Validación de site y aislamiento de persistencia | Implementados en el worker | El tenant real se sigue derivando de metadata confiable. |
| Deduplicación exacta, lógica, idempotente y semántica | Implementada con Redis e índices PostgreSQL | Se conserva; sujetos agrega idempotencia propia a nivel tenant. |
| `analytics_events` como evento crudo | Implementado en PostgreSQL | Continúa como fuente de verdad auditable. |
| Normalización 1:N de items | Implementada en `analytics_event_items` | Sirve como precedente técnico para normalizar sujetos. |
| Persistencia transaccional con `pgx` | Implementada | Se extiende sin acoplar application a SQL. |
| Migraciones con `golang-migrate` | Implementadas | Los cambios nuevos serán aditivos y reversibles. |
| Arquitectura hexagonal y bootstrap | Implementados | Los casos de uso nuevos conservarán puertos y adaptadores. |
| Pruebas de ingesta, worker y SDK | Implementadas y verdes al realizar este análisis | Constituyen el baseline de regresión. |

## 0.2 Capacidades existentes que deben extenderse

- `SiteConfig`, el contrato del resolver y el payload Redis no contienen zona horaria.
- `ValidatedEvent` y el repositorio solo conocen detalles normalizados de items y órdenes.
- La deduplicación funcional existente usa alcance `tenant_id + site_id`; no cubre por sí sola una asociación única en todo el tenant.
- La configuración del worker no incluye flags, ventanas ni parámetros de estadísticas de sujetos.
- El HTTP del worker solo expone health y readiness; no constituye una API analítica.
- La observabilidad actual se apoya principalmente en logs y health checks.

## 0.3 Capacidades que deben crearse desde cero

- identidad y lifecycle de sujetos;
- endpoint interno de eventos server-to-server;
- registro persistente y versionado de contextos;
- normalización de sujetos;
- backfill y conciliaciones;
- proyecciones, checkpoints y ejecuciones auditables;
- API interna de sugerencias;
- scheduler o comandos operativos oficiales;
- métricas de atraso, frescura, fallos y pendientes.

## 0.4 Desajustes y prerrequisitos detectados

1. La zona horaria todavía no se obtiene ni se historiza.
2. Asociaciones y lifecycle emitidos por backend no encajan limpiamente en el endpoint actual, que exige sesión y navegación.
3. El consumidor Redis lee mensajes nuevos con `>` y no recupera pendientes después de una caída mediante `XAUTOCLAIM` o mecanismo equivalente.
4. Un conflicto persistente de `logical_event_id` o `idempotency_key` puede fallar el batch completo después de vencer la deduplicación Redis.
5. No existen scheduler, CLI operativa, API de consultas ni snapshot persistente del registro.

Los puntos 3 y 4 deben resolverse antes de apoyar nuevas proyecciones sobre garantías de procesamiento que hoy no están completas.

## 0.5 Compatibilidad obligatoria

Toda capacidad nueva será opt-in y estará desactivada por defecto.

- `POST /v1/events` conserva validaciones, tracking JWT, respuesta y semántica actuales.
- Los campos nuevos son opcionales y las migraciones son aditivas.
- Con estadísticas de sujetos desactivadas, el worker conserva el flujo actual.
- Un fallo semántico de normalización no impide guardar el evento crudo válido.
- Un fallo de infraestructura transaccional sí provoca reintento y no debe confirmar parcialmente el mensaje Redis.
- Endpoint interno, API de consulta y credenciales nuevas son independientes de contratos existentes.
- Ningún consumidor actual debe configurar esta capacidad para seguir usando GoAnalytics.

Flags conceptuales:

```env
SUBJECT_ANALYTICS_ENABLED=false
SUBJECT_CONTEXT_REGISTRY_ENABLED=false
SUBJECT_PROJECTION_ENABLED=false
SUBJECT_QUERY_API_ENABLED=false
SUBJECT_BOOTSTRAP_ENABLED=false
```

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

Las tablas de sujetos, contextos y agregados son proyecciones derivadas y deben poder reconstruirse a partir de eventos crudos y snapshots persistidos de la revisión de contextos, la zona horaria aplicada, la versión del proyector y los eventos de ciclo de vida.

La metadata hidratada actual no es histórica. Por eso no alcanza con consultar nuevamente el resolver al reconstruir: la normalización debe conservar la revisión del registro y la zona horaria IANA utilizadas.

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
- garantizar que el ID externo de cliente sea canónico en todo el tenant;
- proporcionar una versión monotónica para lifecycle;
- exponer snapshots paginados y revisionados para backfill y conciliación;
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

Este listado expresa el diseño completo, no un único incremento de entrega. La evolución por fases se define en la sección 23 y permitirá decidir después hasta qué fase llega el MVP.

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

La clave `id` es interna a PostgreSQL y no forma parte del contrato de consulta. La API devuelve `external_subject_id` y no expone la implementación de la clave primaria.

Para clientes:

```text
subject_type = customer
```

La primera implementación habilita únicamente `customer`. Cada nuevo `subject_type` requiere habilitación explícita para evitar cardinalidad accidental.

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

La paginación debe usar cursor opaco sobre un snapshot con `source_revision` estable. No se considera suficiente paginar por offset sobre una colección mutable. El contrato fuente debe devolver estado actual, `lifecycle_version`, destino de fusión y fin inequívoco del snapshot.

No debe repetirse completamente cada vez que se recrea el container.

Modelo conceptual del seguimiento:

```text
analytics_subject_bootstrap
- id
- subject_type
- tenant_id
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

Un sujeto creado por interacción web comienza como `unknown`. Una asociación confirmada por el endpoint interno puede crearlo como `active`. Backfill y lifecycle corrigen posteriormente el estado.

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

## 7.0 Canal server-to-server

Asociaciones confirmadas y eventos de lifecycle ingresarán por un endpoint interno nuevo:

```http
POST /v1/internal/events
Authorization: Bearer <SUBJECT_EVENTS_API_TOKEN>
Content-Type: application/json
```

El endpoint publicará en el mismo stream y convergerá en el mismo worker, pero tendrá autenticación y validación propias. No exigirá `anonymous_id`, `session_id`, `origin`, `url`, `path` ni otros campos de navegación requeridos por el tracking web.

Requerirá al menos `event_id`, `event_name`, `event_version`, `timestamp`, `site_code`, `properties` y `context`. Los eventos funcionales y lifecycle deberán incluir `idempotency_key`. El tenant nunca se aceptará como autoridad desde el payload: se derivará del site resuelto.

Este contrato es aditivo y no modifica `POST /v1/events`.

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
    "subjects": [
      {
        "subject_type": "customer",
        "subject_id": "customer_123",
        "relation": "selected"
      }
    ]
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
  "event_id": "018f9b8e-0000-7000-a000-000000000301",
  "event_name": "subject_associated",
  "event_version": 1,
  "idempotency_key": "appointment:845:customer:123",
  "timestamp": "2026-08-05T22:30:00Z",
  "site_code": "pub_site_abc123",
  "properties": {
    "subjects": [
      {
        "subject_type": "customer",
        "subject_id": "customer_123",
        "relation": "attendee"
      }
    ],
    "operation": {
      "operation_type": "appointment",
      "operation_id": "845"
    }
  },
  "context": {
    "context_key": "appointments.customer_assignment",
    "context_version": 1
  }
}
```

## 7.4 Forma canónica e idempotencia

Todo evento nuevo usa `properties.subjects`, incluso cuando contiene un solo sujeto. `subject_id` es el identificador externo y se normaliza como `external_subject_id`. No se admitirán formas alternativas para evitar ambigüedad contractual.

Dentro de un evento se eliminan duplicados exactos por `subject_type + subject_id + relation`. Un elemento inválido queda marcado como normalización fallida sin eliminar el evento crudo ni otros sujetos válidos.

Los índices existentes de `analytics_events` conservan su alcance `tenant_id + site_id`. La normalización agrega idempotencia propia a nivel tenant para que una asociación no incremente dos veces aunque llegue desde sites distintos:

```text
tenant_id
+ event_name
+ idempotency_key
+ subject_type
+ external_subject_id
+ relation
```

Un uso contradictorio de la misma clave se registra como conflicto auditable y no incrementa agregados.

## 7.5 Responsabilidad semántica

GoAnalytics no decide qué acción representa una selección o una asociación.

El proyecto consumidor debe:

- emitir `subject_selected` cuando corresponda;
- emitir `subject_associated` cuando la operación se confirme;
- proporcionar `logical_event_id` o `idempotency_key`;
- evitar duplicar señales funcionales.

## 7.6 Cambios de asociación

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

Si las correcciones de asociaciones son frecuentes, esta mejora debe adelantarse: contar al sujeto anterior y al nuevo puede sesgar materialmente el ranking.

---

# 8. Ciclo de vida del sujeto

## 8.1 Eventos definidos

Se emitirán:

- `subject_deactivated`;
- `subject_reactivated`;
- `subject_merged`.

Opcionalmente, una evolución futura podrá agregar `subject_deleted` u otros estados.

Estados iniciales:

```text
unknown
active
inactive
merged
```

`deleted` no forma parte del contrato inicial mientras el origen no pueda representarlo inequívocamente. Solo `active` es elegible. Un sujeto creado por interacción web comienza como `unknown`; una asociación confirmada por el endpoint interno puede crearlo como `active` por provenir de una fuente autorizada.

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

La disponibilidad real de esta versión monotónica todavía debe confirmarse y bloquea la fase de lifecycle. No se utilizará `updated_at` sin garantía de orden como sustituto silencioso. Si el origen no dispone de versión, deberá acordarse una secuencia de integración o una regla determinista documentada.

## 8.4 Fusión

Cuando un cliente se fusiona:

- el sujeto anterior queda en estado `merged`;
- `merged_into_subject_id` apunta al sujeto canónico;
- los eventos históricos permanecen en el sujeto anterior;
- los eventos futuros deben emitirse con el cliente canónico;
- no se transfieren conteos automáticamente en la primera versión.

La falta de consolidación reduce la influencia histórica del sujeto canónico. Debe adelantarse una consolidación lógica versionada si las fusiones son frecuentes, sin reescribir eventos crudos.

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

Las propiedades semánticas de una definición publicada son inmutables. Cambiar significado, familia o padre requiere nueva `context_version`. El estado operativo puede cambiar entre revisiones; por eso cada evento normalizado conserva la revisión concreta usada.

## 9.5 Modelo persistente del registro

Redis será caché compartida, no la única copia recuperable.

```text
analytics_context_registry_revisions
- id
- schema_version
- registry_version
- registry_revision
- payload
- validation_status
- loaded_at
- activated_at

analytics_context_definitions
- id
- context_key
- context_version
- context_family
- parent_definition_id
- created_at

analytics_context_registry_entries
- registry_revision_id
- context_definition_id
- status
```

Restricciones conceptuales:

```sql
UNIQUE (registry_revision)
UNIQUE (context_key, context_version)
UNIQUE (registry_revision_id, context_definition_id)
```

Esta separación mantiene una identidad estable de definición y permite registrar su estado por revisión sin reescribir historia.

## 9.6 Familias genéricas

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

## 9.7 Clave y versión

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

La fila normalizada conserva además `context_registry_revision_id` para reproducir la decisión tomada.

## 9.8 Jerarquía

La jerarquía es opcional.

Reglas:

- cero o un padre;
- mismo registro global;
- sin ciclos;
- cambiar el padre requiere una nueva versión;
- no se permiten relaciones M2M;
- la familia del hijo debe ser compatible con la del padre.

En la primera versión se almacena y valida, pero no se utiliza para roll-up ni fallback.

## 9.9 Contextos no registrados

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

## 9.10 Contextos inactivos

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
Authorization: Bearer <SUBJECT_CONTEXT_REGISTRY_TOKEN>
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

1. GoAnalytics consulta memoria local.
2. Si no existe, intenta cargar la revisión activa desde Redis.
3. Si no existe, intenta cargar el último snapshot válido desde PostgreSQL.
4. Si corresponde, consulta al proyecto consumidor.
5. Valida el registro completo.
6. Guarda el snapshot persistente de forma atómica.
7. Publica la revisión en Redis y memoria.
8. Continúa operando con esa revisión.

## 10.4 Hidratación perezosa

Durante la operación:

- se resuelve desde memoria o Redis;
- si la revisión no está disponible, se rehidrata;
- se aplica cooldown;
- se evita la descarga simultánea por múltiples workers;
- se utiliza lock distribuido o patrón `singleflight`.

El registro se obtiene completo y de manera atómica. No se resuelven contextos individualmente.

El cooldown Redis existente sirve como precedente, pero no es por sí solo un lock distribuido completo. La implementación deberá combinar exclusión local y coordinación entre procesos.

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
SUBJECT_CONTEXT_REGISTRY_ENABLED=false
SUBJECT_CONTEXT_REGISTRY_URL=http://backend/internal/analytics/context-registry
SUBJECT_CONTEXT_REGISTRY_TOKEN=change_me
SUBJECT_CONTEXT_REGISTRY_CACHE_TTL_SECONDS=86400
SUBJECT_CONTEXT_REGISTRY_REHYDRATE_COOLDOWN_SECONDS=300
```

Los nombres finales deberán alinearse con las convenciones actuales del repositorio.

---

# 11. Normalización de eventos de sujetos

Modelo conceptual:

```text
analytics_event_subjects
- id
- analytics_event_id
- subject_ordinal
- client_event_id
- logical_event_id
- idempotency_key
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
- context_registry_revision_id
- timezone_name
- local_date
- normalization_status
- normalization_reason
- metadata
- updated_at
- created_at
```

Relación:

```text
analytics_events 1:N analytics_event_subjects
```

Esto permite eventos con varios sujetos, reconstrucción, consultas eficientes, mantener el evento principal una sola vez y extender el modelo a entidades distintas de clientes.

Unicidades mínimas:

- `analytics_event_id + subject_ordinal` para reprocesar sin duplicar filas;
- identidad lógica del sujeto por tenant;
- idempotencia funcional de asociaciones a nivel tenant.

La normalización solo se ejecuta para nombres y versiones registrados. Los demás eventos continúan como eventos crudos sin alterar el comportamiento vigente.

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
- timezone_name
- impression_count
- view_count
- selection_count
- association_count
- last_association_at
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
- timezone_name
- impression_count
- view_count
- selection_count
- association_count
- last_association_at
- projection_version
- computed_at
```

## 12.2 Zona horaria

Los períodos se calculan usando la zona horaria del tenant.

La zona horaria todavía no forma parte de `SiteConfig`, del contrato del resolver ni del payload Redis. Debe agregarse de manera aditiva como nombre IANA, por ejemplo `America/Argentina/Buenos_Aires`.

El proyecto consumidor debe garantizar que todos los sites de un tenant resuelvan la misma zona horaria canónica.

No debe obtenerse del evento individual.

La normalización conserva `timezone_name` y la proyección conserva la zona aplicada. Los cambios se aplican prospectivamente; no se reescribe historia automáticamente. Una reconstrucción usa el snapshot guardado en la fila normalizada.

No existe fallback silencioso. Si falta una zona válida, el evento crudo se guarda y la normalización temporal queda pendiente. Un fallback UTC solo podrá habilitarse explícitamente y deberá quedar trazado.

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
SUBJECT_PROJECTION_ENABLED=false
SUBJECT_PROJECTION_INTERVAL_SECONDS=86400
SUBJECT_PROJECTION_LOOKBACK_SECONDS=172800
SUBJECT_PROJECTION_BATCH_SIZE=5000
```

El `lookback` permite reprocesar eventos tardíos.

El job debe ser incremental, idempotente, reanudable, versionado, observable y reconstruible.

El `lookback` no vuelve a incrementar contadores: recalcula y reemplaza atómicamente los días afectados desde las filas normalizadas.

Modelos operativos requeridos:

```text
analytics_projection_runs
- id
- projection_name
- projection_version
- status
- started_at
- completed_at
- source_from
- source_to
- rows_read
- rows_written
- last_error

analytics_projection_checkpoints
- projection_name
- projection_version
- cursor
- updated_at
```

El repositorio no posee actualmente scheduler ni CLI para estos jobs. Deben ejecutarse fuera del camino crítico de ingesta y poder desplegarse o programarse de forma independiente.

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
2. Ordenar por `association_count DESC`, `last_association_at DESC` y `external_subject_id ASC`.
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

Será una capacidad nueva separada del HTTP operativo del worker, para que carga o fallos de consulta no detengan el consumo de eventos.

```env
SUBJECT_QUERY_API_ENABLED=false
SUBJECT_QUERY_API_TOKEN=change_me
```

No se reutilizará el tracking JWT, el token del resolver de sites, el token de eventos server-to-server ni el token del registro de contextos.

En la instancia actual para un único proyecto consumidor, el token puede consultar cualquier tenant, pero toda lectura filtra obligatoriamente por `tenant_id`. Una futura instancia multiproyecto deberá incorporar scopes antes de compartir la API.

## 16.2 Endpoint conceptual

```http
GET /v1/internal/subjects/suggestions
Authorization: Bearer <SUBJECT_QUERY_API_TOKEN>
```

Parámetros:

```text
tenant_id
subject_type
context_key
context_version
limit
```

`tenant_id` y `subject_type` son obligatorios. `context_version` es obligatorio cuando existe `context_key`. `limit` tendrá default 10 y máximo configurable. Sin contexto se consulta solamente el ranking global.

## 16.3 Respuesta conceptual

```json
{
  "tenant_id": "tenant_123",
  "subject_type": "customer",
  "context": {
    "context_key": "appointments.customer_picker",
    "context_version": 1,
    "resolution_status": "registered"
  },
  "window": {
    "context_days": 30,
    "global_days": 90
  },
  "computed_at": "2026-08-05T02:00:00Z",
  "projection_version": 1,
  "is_stale": false,
  "results": [
    {
      "external_subject_id": "customer_123",
      "association_count": 14,
      "last_association_at": "2026-08-04T20:15:00Z",
      "source_level": "exact_context"
    },
    {
      "external_subject_id": "customer_456",
      "association_count": 25,
      "last_association_at": "2026-08-03T13:20:00Z",
      "source_level": "tenant_global"
    }
  ]
}
```

La API devuelve identidad externa, conteo, último uso, nivel de origen, ventana, versión de proyección, frescura y fecha de cálculo. No expone la clave interna de PostgreSQL.

No devuelve nombre, email, teléfono, documento, información personal ni payload crudo.

## 16.4 Elegibilidad

La API excluye:

- sujetos inactivos;
- sujetos fusionados;
- sujetos con estado no elegible.

El backend consumidor puede aplicar filtros adicionales.

## 16.5 Ausencia y frescura

- Sin datos válidos: `200` con `results: []`.
- Sin ninguna proyección completada: `503 projection_unavailable`.
- Proyección fuera del umbral: puede servirse con `is_stale: true` si la política configurada lo permite.
- Si servir datos desactualizados está deshabilitado: `503 projection_stale`.

Con un contexto desconocido o inactivo se aplica fallback global y la respuesta informa el estado de resolución.

---

# 17. Conciliación de contextos

## 17.1 Alcance global

El registro es único para todo el proyecto consumidor.

El comando principal debe reconciliar todos los tenants:

```bash
goanalytics reconcile-contexts   --all-pending   --registry-revision sha256:8d2f...
```

Los comandos de esta sección son interfaces conceptuales. El repositorio no contiene hoy un binario CLI `goanalytics`; deberá crearse una capacidad operativa oficial en lugar de un script SQL desechable.

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

Los nombres son conceptuales, pero las duraciones usan sufijos explícitos como las configuraciones actuales del repositorio.

```env
# Capacidades opt-in
SUBJECT_ANALYTICS_ENABLED=false
SUBJECT_CONTEXT_REGISTRY_ENABLED=false
SUBJECT_PROJECTION_ENABLED=false
SUBJECT_QUERY_API_ENABLED=false
SUBJECT_BOOTSTRAP_ENABLED=false

# Eventos server-to-server
SUBJECT_EVENTS_API_TOKEN=change_me

# Registro de contextos
SUBJECT_CONTEXT_REGISTRY_URL=http://backend/internal/analytics/context-registry
SUBJECT_CONTEXT_REGISTRY_TOKEN=change_me
SUBJECT_CONTEXT_REGISTRY_CACHE_TTL_SECONDS=86400
SUBJECT_CONTEXT_REGISTRY_REHYDRATE_COOLDOWN_SECONDS=300

# Proyecciones
SUBJECT_PROJECTION_INTERVAL_SECONDS=86400
SUBJECT_PROJECTION_LOOKBACK_SECONDS=172800
SUBJECT_PROJECTION_BATCH_SIZE=5000

# Sugerencias
SUBJECT_SUGGESTION_CONTEXT_WINDOW_DAYS=30
SUBJECT_SUGGESTION_GLOBAL_WINDOW_DAYS=90
SUBJECT_SUGGESTION_DEFAULT_LIMIT=10
SUBJECT_SUGGESTION_MAX_LIMIT=100
SUBJECT_SUGGESTION_STALE_AFTER_SECONDS=172800
SUBJECT_SUGGESTION_SERVE_STALE=true

# API de consulta
SUBJECT_QUERY_API_TOKEN=change_me
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

## 22.7 Recuperación del stream

Antes de agregar proyecciones debe incorporarse recuperación de mensajes pendientes del consumer group, ownership entre workers, límite de reintentos y tratamiento auditable de mensajes irrecuperables. Leer solamente mensajes nuevos con `>` no garantiza recuperación después de una caída.

## 22.8 Conflictos persistentes

Los conflictos esperables de `event_id`, `logical_event_id` e `idempotency_key` deben convertirse en resultados idempotentes por evento. Una fila repetida no debe impedir persistir el resto de un batch válido.

---

# 23. Evolución incremental recomendada

## Fase 0 — Confiabilidad y contratos

- Recuperar pendientes Redis y aislar mensajes problemáticos.
- Endurecer la idempotencia persistente por evento.
- Cerrar endpoint interno, zona horaria y contratos fuente.
- Incorporar flags independientes apagados por defecto.

## Fase 1 — Identidad, normalización y lifecycle

- Crear sujetos y normalización 1:N.
- Admitir eventos server-to-server.
- Implementar idempotencia de asociaciones a nivel tenant.
- Aplicar lifecycle solo cuando exista versión monotónica.

## Fase 2 — Backfill y elegibilidad

- Consumir snapshots paginados y revisionados.
- Inicializar estados de clientes existentes.
- Probar concurrencia entre backfill, creación perezosa y lifecycle.

## Fase 3 — Proyección diaria y ranking global

- Crear ejecuciones, checkpoints y agregados tenant/día.
- Aplicar zona horaria versionada y lookback idempotente.
- Exponer una consulta global opcional.

## Fase 4 — Registro y ranking contextual

- Persistir revisiones y definiciones.
- Resolver contexto exacto durante normalización.
- Agregar proyección contextual y fallback global.

## Fase 5 — Conciliaciones y endurecimiento operativo

- Incorporar comandos oficiales, `dry-run`, auditoría y métricas.
- Conciliar contextos y sujetos.
- Validar carga, frescura, aislamiento y recuperación.

## Fase 6 — Evoluciones basadas en evidencia

- fallback por familia;
- scoring versionado;
- corrección de asociaciones;
- consolidación lógica de fusiones;
- roll-up jerárquico;
- automatización de conciliaciones;
- scopes y rate limiting avanzados.

Las pruebas de contratos, migraciones, concurrencia, deduplicación, orden de lifecycle, contextos inválidos, aislamiento, zona horaria, frescura, backfill, conciliación y eventos tardíos forman parte de la fase que introduce cada comportamiento, no de una etapa final separada.

La decisión de corte del MVP se toma después de estimar las fases. El diseño completo se conserva para evitar callejones sin salida, pero ninguna fase obliga a habilitar las siguientes.

---

# 24. Criterios de aceptación preliminares

1. Una instalación con la capacidad desactivada conserva contratos y comportamiento actuales.
2. IDs iguales en tenants distintos generan sujetos distintos.
3. El mismo ID usado por sites de un tenant converge en un único sujeto.
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
18. La API devuelve el ID externo y no expone la clave interna de PostgreSQL.
19. La API no devuelve datos personales.
20. La API valida un token dedicado.
21. La conciliación global procesa todos los tenants.
22. La conciliación es idempotente y reanudable.
23. Los filtros por tenant son opcionales y operativos.
24. Los eventos históricos sobreviven a cambios de estado.
25. Una fusión no transfiere conteos en la primera versión.
26. El backfill y la creación perezosa no duplican sujetos.
27. Los mensajes pendientes vuelven a procesarse después de una caída.
28. Un conflicto idempotente no bloquea el resto de un batch válido.
29. El lifecycle fuera de orden no reemplaza un estado más nuevo.
30. El `lookback` recalcula sin duplicar conteos.
31. El ranking desempata de forma determinista.
32. Una proyección ausente o desactualizada se representa explícitamente.

---

# 25. Decisiones todavía pendientes

No hace falta cerrar todas estas decisiones para comenzar la Fase 0. Cada una debe resolverse antes del SDD de la fase que la consume.

## 25.1 Versión monotónica de lifecycle

Debe confirmarse si el backend consumidor dispone de:

```text
customer.version
customer.updated_sequence
```

Si no existe, deberá definirse una alternativa basada en `updated_at`, desempate determinista o secuencia generada por integración.

La alternativa recomendada es una secuencia generada por integración. `updated_at` solo será aceptable si el consumidor puede demostrar orden total y desempate estable.

## 25.2 Alcance del backfill

Debe definirse si incluirá:

- solo clientes activos;
- activos e inactivos;
- fusionados;

Un estado `deleted` solo podrá agregarse si se aprueba un contrato futuro que lo represente inequívocamente.

La recomendación preliminar es incluir todos los estados disponibles.

También deben cerrarse cursor opaco, `source_revision`, consistencia del snapshot, fin de paginación y representación de fusiones. Estas definiciones son obligatorias antes de la Fase 2.

## 25.3 Base diaria

Se adopta que los agregados diarios serán la fuente para semana, mes, trimestre, año y ventanas móviles. Un SDD podrá cambiar esta decisión solo con evidencia de volumen o precisión que lo justifique.

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

Debe cerrarse el umbral de frescura y si `SUBJECT_SUGGESTION_SERVE_STALE` estará habilitado en producción.

## 25.5 Familias iniciales

La lista definitiva debe revisarse para evitar demasiadas familias, conceptos superpuestos, familias ligadas al negocio o una categoría residual permanente.

## 25.6 Cambios de zona horaria

Se adopta aplicación prospectiva: la fila normalizada conserva la zona IANA usada y no se reescribe historia automáticamente. Falta confirmar que el consumidor puede suministrar una zona canónica consistente para todos los sites del tenant.

## 25.7 Proceso de publicación del registro

Antes de la Fase 4 deben definirse responsable de aprobación, validación CI, mecanismo de publicación, rollback y familias iniciales.

## 25.8 Volumen, frescura y SLO

Antes de dimensionar proyecciones y API deben estimarse eventos diarios de sujetos, cantidad de tenants, sujetos por tenant, cardinalidad de contextos, frecuencia de consulta, latencia y frescura aceptables.

## 25.9 Topología de ejecución

Debe definirse qué proceso alojará los jobs y cuál alojará la API. Ambos deben permanecer desacoplados del consumer crítico aunque inicialmente compartan módulo Go o imagen.

## 25.10 Decisiones cerradas durante el alineamiento

- Se conserva el diseño completo dividido en fases incrementales.
- Los eventos backend usan un endpoint interno nuevo.
- El identificador externo de cliente es canónico en todo el tenant.
- Toda capacidad nueva es opt-in y está desactivada por defecto.
- La base temporal es diaria.
- El fallback inicial es contexto exacto a global.
- La API no expone claves internas ni datos personales.

---

# 26. Riesgos, impacto y momento recomendado

| Punto débil | Impacto | Momento recomendado | Tratamiento |
| --- | --- | --- | --- |
| Mensajes Redis pendientes no recuperados | Crítico | Ahora, antes de nuevas proyecciones | Implementar recuperación, ownership, límite de reintentos y tratamiento de mensajes irrecuperables. |
| Conflictos persistentes de idempotencia pueden fallar un batch | Crítico | Ahora | Convertir conflictos esperables en resultados idempotentes por evento y evitar que una fila bloquee todo el batch. |
| Eventos backend forzados al contrato web | Alto | Antes de asociaciones | Crear el endpoint interno con autenticación y validación propias. |
| Zona horaria ausente y no historizada | Alto | Antes de proyección diaria | Extender resolver y caché; guardar snapshot IANA aplicado. |
| Lifecycle sin versión monotónica | Alto | Antes de elegibilidad por eventos | Acordar secuencia; no usar timestamps ambiguos silenciosamente. |
| Backfill sin snapshot estable | Alto | Antes del backfill | Exigir cursor opaco y `source_revision`. |
| Funcionalidad activada accidentalmente | Alto | Todas las fases | Flags independientes apagados por defecto y migraciones aditivas. |
| Registro disponible solo en Redis | Alto | Antes de ranking contextual | Persistir revisiones válidas en PostgreSQL. |
| Reasociaciones cuentan sujeto anterior y nuevo | Medio | Según tasa de corrección | Adelantar evento de cambio o proyección por operación si el sesgo es material. |
| Fusiones reducen influencia histórica del canónico | Medio | Según frecuencia de fusiones | Evaluar consolidación lógica versionada sin reescribir eventos. |
| Token global de consulta alcanza todos los tenants | Medio | Al cambiar la topología monocliente | Agregar scopes por tenant o proyecto antes de multiproyecto. |
| Proyección diaria desactualizada | Medio | Antes de exponer API | Definir SLO, umbral y política explícita de datos stale. |
| Cardinalidad de tipos y contextos | Medio | Antes de habilitar cada tipo | Allowlist de `subject_type`, registro controlado y límites. |
| Scoring, jerarquía o fallback por familia prematuros | Bajo en v1 | Futuro | Implementar solo con evidencia y definición versionada. |
| Identidad cruzada entre tenants | Fuera de alcance | No abordar | Requiere una iniciativa independiente de privacidad y gobernanza. |

## 26.1 Trabajos previos obligatorios

1. Recuperación confiable de pendientes Redis.
2. Tratamiento idempotente de conflictos PostgreSQL sin bloquear batches completos.
3. Contrato del endpoint interno de eventos.
4. Extensión aditiva del resolver con zona horaria IANA.
5. Contrato paginado y revisionado de sujetos antes de backfill.

No es necesario resolver scoring, identidad cruzada, roll-up, fallback por familia ni automatización de conciliaciones para comenzar.

---

# 27. Resultado esperado del diseño completo

Al completar las fases necesarias, GoAnalytics deberá:

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
