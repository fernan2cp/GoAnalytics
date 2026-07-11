# Propuesta De Evolucion Futura: Agregados Y Consulta

## Estado Del Documento

Este documento es una propuesta de mejora futura. No forma parte de la implementacion de `system_tracking` v1 y no modifica el alcance tecnico actualmente adoptado.

La propuesta debera evaluarse nuevamente cuando la evolucion del negocio, el volumen de datos o necesidades reales de los integradores demuestren que la persistencia y exportacion de eventos ya no son suficientes.

## Resumen Ejecutivo

`GoAnalytics` fue concebido como un microservicio generico, agnostico e independiente, enfocado en recibir, validar y persistir datos de analitica. La implementacion actual respeta esa esencia: conserva `properties` y `context`, normaliza eventos de items definidos y deja los agregados contextuales fuera del servicio.

Una evolucion futura podria agregar proyecciones, agregados y consultas genericas para reducir el trabajo repetido de los integradores. Esta capacidad tendria valor operativo, pero tambien cambiaria la responsabilidad principal del microservicio: dejaria de ser solamente un recolector y pasaria a participar en el procesamiento y consumo analitico.

La recomendacion es considerar esta evolucion como una capacidad opt-in y posterior, condicionada por evidencia. No se recomienda comenzar con un endpoint especifico de `top items`, scoring online ni reglas de recomendacion embebidas en GoAnalytics.

## Relacion Con La V1

La decision vigente de v1 se mantiene:

- La ingesta permanece enfocada en aceptar eventos seguros y compatibles.
- `analytics_events` sigue siendo la fuente de verdad de los eventos recibidos.
- `properties` representa los datos del hecho observado.
- `context` representa el entorno funcional, tecnico o de ubicacion logica.
- La normalizacion existente de items se conserva sin incorporar rankings.
- No se crea `analytics_context_item_aggregates`.
- No se expone `GET /v1/aggregates/items/top`.
- No se ejecuta scoring contextual online.

La documentacion publica de referencia para la situacion actual es `docs/integration/system-tracking-contracts.md`.

## Motivos Para Evaluar La Extension

La extension podria resultar conveniente si aparecen necesidades comunes entre varios integradores:

- Consultar rankings o metricas sin leer directamente las tablas internas.
- Evitar que cada integrador construya su propio pipeline de agregacion.
- Centralizar aislamiento por tenant y site.
- Aplicar politicas homogeneas de ventanas, deduplicacion y seguridad.
- Responder consultas operativas sin recorrer continuamente `analytics_events`.
- Exponer resultados reproducibles mediante definiciones y algoritmos versionados.

El valor de la extension no debe medirse por la cantidad de endpoints agregados, sino por la reduccion de duplicacion y el beneficio compartido entre integradores.

## Cambio De Responsabilidad

La extension modificaria la frontera original del servicio:

```text
V1:       ingesta -> validacion -> persistencia

Futuro:   ingesta -> persistencia -> proyeccion -> agregado -> consulta
```

Este cambio implica que GoAnalytics deberia asumir decisiones y responsabilidades adicionales:

- Procesamiento batch o casi en tiempo real.
- Consistencia eventual y frescura de resultados.
- Reprocesamiento de eventos tardios.
- Versionado de definiciones y algoritmos.
- Costos de almacenamiento e indexacion.
- Autorizacion de consultas analiticas.
- Observabilidad de jobs y materializaciones.

Por ese motivo, no debe considerarse una extension puramente tecnica. Es una evolucion de producto y arquitectura.

## Beneficios Esperados

- Menor duplicacion de pipelines entre proyectos consumidores.
- Consultas operativas mas previsibles y eficientes.
- Mayor control centralizado sobre aislamiento multi-tenant y multi-site.
- Resultados trazables mediante `definition_id`, `algorithm_version`, `computed_at` y ventana temporal.
- Posibilidad de reconstruir agregados desde los eventos crudos.
- Aprovechamiento del contexto generico ya almacenado en `analytics_events.context`.

## Costos Y Riesgos

- Mayor complejidad operativa, de almacenamiento y de mantenimiento.
- Necesidad de definir pesos, ventanas, atribucion y tratamiento de eventos tardios.
- Riesgo de acoplar el microservicio a comercio, catalogos, recomendaciones o un producto especifico.
- Cardinalidad explosiva si se permiten dimensiones dinamicas sin control.
- Resultados eventualmente consistentes que pueden no coincidir inmediatamente con la ingesta.
- Mayor superficie de seguridad al exponer metricas internas por API.
- Necesidad de pruebas de carga, aislamiento, reconstruccion y consistencia.
- Posible divergencia entre el significado de un evento y su uso como senal de negocio.

## Principios De Diseño Futuro

Si la extension se aprueba, deberia respetar los siguientes principios:

1. Mantener `analytics_events` como fuente de verdad y permitir reconstruir las proyecciones.
2. Ejecutar agregacion fuera del camino critico de ingesta.
3. Usar contratos genericos y tratar identificadores de dominio como valores opacos.
4. Evitar catalogos obligatorios y enums de negocio en el nucleo.
5. Separar hechos observados de recomendaciones o decisiones comerciales.
6. Versionar la definicion del agregado y el algoritmo de calculo.
7. Controlar explicitamente dimensiones, cardinalidad, limites y ventanas.
8. Usar credenciales de lectura separadas del tracking JWT.
9. Mantener aislamiento estricto entre tenants y sites.
10. Preservar compatibilidad hacia atras con el contrato de ingesta existente.

## Arquitectura Objetivo

La primera arquitectura recomendable es una proyeccion batch incremental y reconstruible:

```text
Eventos aceptados
       |
       v
analytics_events
       |
       v
Proyector batch incremental
       |
       v
Agregados versionados
       |
       v
API de consulta autenticada
```

La ingesta no deberia esperar por la ejecucion del proyector ni actualizar directamente las tablas de agregados. El proyector deberia registrar checkpoints, soportar reintentos idempotentes y permitir recalcular una ventana temporal.

## Modelo Generico Recomendado

El modelo no deberia basarse exclusivamente en `item`. Conviene utilizar un sujeto analitico generico:

- `subject_type`: tipo opaco, por ejemplo `item`, `feature`, `result` o `article`.
- `subject_id`: identificador opaco enviado por el integrador.
- `metric`: metrica calculada.
- `dimensions`: contexto permitido para agrupar.
- `window_start` y `window_end`: ventana analizada.
- `score` y `event_count`: resultado cuantitativo.
- `definition_id` y `algorithm_version`: trazabilidad del calculo.
- `computed_at`: momento de materializacion.

El caso de uso de `top items` podria existir como una configuracion o alias, pero no deberia definir la estructura fundamental del servicio.

## Definiciones Versionadas

Los pesos y eventos fuente no deberian quedar embebidos en codigo especifico de un integrador. Una definicion podria declarar conceptualmente:

```json
{
  "definition_id": "contextual_engagement",
  "version": 2,
  "subject_type": "item",
  "sources": [
    {"event_name": "item_viewed", "weight": 1},
    {"event_name": "cart_item_added", "weight": 4},
    {"event_name": "purchase_completed", "weight": 10}
  ],
  "dimensions": ["feature", "surface", "component_id"],
  "windows": ["7d", "30d", "90d"]
}
```

La configuracion debe ser administrada y validada separadamente del payload de tracking. GoAnalytics debe calcular segun una definicion, pero el integrador debe decidir que significa una conversion y que pesos representan su negocio.

## Dimensiones Y Cardinalidad

`context` puede continuar siendo abierto en la ingesta, siempre que cumpla las reglas de seguridad y limites de v1. Sin embargo, las dimensiones utilizables para agregacion deben ser controladas por una allowlist configurable.

Como candidatos iniciales pueden considerarse:

- `app_area`.
- `feature`.
- `surface`.
- `entry_point`.
- `component_id`.
- `entity_type`.
- `runtime`.

Cada dimension deberia definir tipo esperado, longitud maxima, politica para valores ausentes y limites de cardinalidad. No deberian habilitarse por defecto URLs completas, identificadores de sesion, timestamps ni valores libres de alta variabilidad.

## Consulta Y Fallback

Una API futura deberia ser generica, por ejemplo:

```http
GET /v1/aggregates/rankings
  ?subject_type=item
  &metric=engagement
  &dimension.feature=catalog_search
  &window=30d
  &limit=20
```

La respuesta deberia informar, como minimo:

- Dimensiones solicitadas y aplicadas.
- Nivel de fallback utilizado.
- Ventana temporal.
- Frescura del resultado.
- `definition_id` y `algorithm_version`.
- Metricas agregadas, sin datos personales ni payload crudo.

El fallback debe ser una politica declarada. Conviene permitir `none` para coincidencia exacta y una politica generica versionada para relajar dimensiones progresivamente. No deberia aceptarse una jerarquia arbitraria en cada consulta.

## Separacion De Recomendaciones

GoAnalytics puede devolver evidencia agregada, pero no deberia decidir por si solo que elemento mostrar al usuario final. Disponibilidad, permisos, inventario, precio, reglas comerciales, diversidad y priorizacion pertenecen al integrador.

Esta separacion permite que GoAnalytics sea util para analitica y senales de ranking sin convertirse en un motor de recomendacion dependiente de un dominio concreto.

## Seguridad Y Operacion

Una implementacion futura deberia contemplar:

- Credenciales separadas para lectura de agregados.
- Scopes de acceso por tenant y site.
- Limites de `limit`, ventanas y frecuencia de consulta.
- Rate limiting independiente del endpoint de ingesta.
- Respuestas sin propiedades crudas ni datos personales.
- Logs de eventos procesados, duracion, filas afectadas y errores.
- Metricas de frescura, atraso del watermark y fallos de reconstruccion.
- Pruebas de aislamiento entre tenants y sites.
- Backfill controlado y reproducible.

## Evolucion Por Fases

### Fase A: Exportacion Estable

Formalizar exportacion o lectura server-to-server incremental de eventos y detalles normalizados. Esta fase permite validar necesidades reales con bajo riesgo y sin introducir una nueva API de agregados.

### Fase B: Proyecciones Internas

Implementar uno o dos agregados batch para casos de uso demostrados. Medir volumen, cardinalidad, costo, frescura, tasa de eventos tardios y frecuencia de reconstruccion.

### Fase C: API Generica De Consulta

Exponer rankings o metricas agregadas con credenciales de lectura, dimensiones controladas, definiciones versionadas y contratos de frescura.

### Fase D: Capacidades Avanzadas

Evaluar procesamiento casi en tiempo real, politicas de fallback mas complejas, retencion diferenciada o scoring avanzado solo si existe evidencia suficiente y un caso de negocio concreto.

## Criterios Para Reabrir La Decision

La implementacion deberia revaluarse cuando se cumplan varias de estas condiciones:

- Existen al menos dos integradores con necesidades equivalentes.
- La exportacion actual genera duplicacion operativa relevante.
- Hay requisitos concretos de frescura, volumen y disponibilidad.
- Las dimensiones de agrupacion son estables y de cardinalidad conocida.
- Existe una politica clara de aislamiento, retencion y autorizacion.
- Se dispone de capacidad operativa para jobs, almacenamiento y observabilidad.
- El beneficio esperado supera el costo de mantener la capacidad dentro de GoAnalytics.

## Decision Recomendada

Mantener la restriccion de diseño de v1 en el estado actual. La evolucion hacia agregados y consultas es valida y puede aportar valor, pero debe tratarse como una decision futura de producto y arquitectura, no como una continuacion automatica del SDD ya implementado.

Si las necesidades aparecen, la primera opcion recomendada es una proyeccion batch generica, versionada y reconstruible, seguida por una API de consulta con autenticacion separada. No se recomienda comenzar con `top items`, scoring online ni reglas de recomendacion embebidas.

El objetivo debe ser ampliar la capacidad analitica sin perder la independencia, reutilizacion y neutralidad que justifican la existencia de GoAnalytics.
