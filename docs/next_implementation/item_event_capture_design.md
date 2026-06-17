# Captura De Eventos De Items Para Scoring

## Objetivo

Go Analytics debe capturar, validar, deduplicar y persistir los eventos necesarios para que otro servicio calcule el scoring de items.

El microservicio no debe calcular rankings, no debe aplicar reglas de negocio del catalogo y no debe escribir en bases tenant. Su responsabilidad termina en dejar datos confiables, auditables y consultables.

## Responsabilidades Del Microservicio

- Recibir eventos desde el SDK o integraciones compatibles.
- Validar token, site, tenant, dominio, version y estructura minima.
- Deduplicar eventos con reglas generales y reglas especificas por tipo de evento.
- Persistir el evento principal en `analytics_events`.
- Persistir datos normalizados de items en `analytics_event_items` cuando el evento incluya items.
- Mantener trazabilidad suficiente para que procesos externos puedan recalcular metricas con distintas ventanas o versiones de algoritmo.

No corresponde a Go Analytics:

- decidir si un item es visible, vigente o vendible;
- consultar stock;
- ordenar el catalogo;
- calcular scores finales;
- materializar tablas tenant;
- aplicar diversidad de categoria, clase o marca.

## Eventos Requeridos

Los eventos deben usar nombres estables en `snake_case`.

| Evento | Finalidad |
|---|---|
| `item_impression` | Registrar que un item fue efectivamente visible en una superficie del catalogo. |
| `item_viewed` | Registrar apertura o vista de detalle de un item. |
| `item_image_zoomed` | Registrar interes visual fuerte sobre una imagen del item. |
| `cart_item_added` | Registrar incorporacion de un item al carrito. |
| `checkout_started` | Registrar inicio de checkout con los items presentes. |
| `purchase_completed` | Registrar compra confirmada con los items incluidos. |

`item_impression` es obligatorio para evitar sesgos por volumen de exposicion. Sin impresiones, el scoring no puede distinguir entre buen rendimiento relativo y simple mayor visibilidad.

## Regla Formal De `item_impression`

Una impresion valida debe representar visibilidad real, no solo renderizado tecnico.

Condicion minima recomendada:

```text
visible_ratio >= 50%
AND visible_time_ms >= 1000
AND document.visibilityState == "visible"
AND item real renderizado, no skeleton ni placeholder
```

No debe emitirse `item_impression` por:

- simple render del componente;
- presencia del item en la respuesta del backend;
- pre-carga de datos;
- skeletons o placeholders;
- elementos fuera del viewport;
- pestana en segundo plano;
- re-renderizados del frontend.

El frontend o aplicacion integradora es responsable de medir la visibilidad real. Go Analytics debe validar estructura, persistir la evidencia tecnica y permitir auditoria posterior.

## Datos De Item En Eventos

Los eventos relacionados a items deben incluir, cuando aplique:

- `item_id`: identificador canonico del item en el tenant.
- `variant_id`: identificador de la variante concreta cuando aplique.
- `sku`: codigo concreto de la variante o item cuando exista.
- `item_type`: tipo funcional del item, por ejemplo `product`, `service` o `subscription_plan`.
- `item_class_id`: clase del item cuando exista.
- `category_ids`: categorias asociadas conocidas al momento del evento.
- `surface`: superficie donde ocurrio el evento, por ejemplo `catalog`, `category`, `search`, `home` o `cart`.
- `position`: posicion visible dentro de la superficie.
- `page`: pagina o bloque de paginacion donde fue mostrado.
- `search_term`: busqueda que produjo la exposicion, si corresponde.
- `ranking_run_id`: corrida de ranking que produjo la exposicion, si existe.
- `ranking_version`: version del ranking o del criterio de ordenamiento aplicado.
- `list_instance_id`: instancia logica de lista o grilla que genero la exposicion.
- `impression_batch_id`: lote de impresiones emitido junto.
- `visible_ratio`: proporcion visible del item.
- `visible_time_ms`: tiempo visible acumulado en milisegundos.
- `viewport_width`: ancho del viewport al medir la impresion.
- `viewport_height`: alto del viewport al medir la impresion.
- `rendered_at`: momento en que el item real quedo renderizado.
- `cart_id`: carrito asociado cuando aplique.
- `checkout_id`: checkout asociado cuando aplique.
- `order_id`: orden asociada cuando aplique.
- `order_line_id`: linea de orden asociada cuando aplique.
- `unit_price`: precio unitario registrado como snapshot del evento.
- `currency`: moneda del precio.
- `quantity`: cantidad involucrada.
- `gross_amount`: importe bruto de la linea o item.
- `net_amount`: importe neto de la linea o item.
- `discount_amount`: descuento aplicado a la linea o item.
- `unit_cost`: costo unitario como snapshot opcional.
- `cost_amount`: costo total como snapshot opcional.

Contrato recomendado para variantes:

```text
item_id = ID canonico del item
variant_id = ID de la variante concreta, si aplica
sku = codigo concreto, si aplica
```

El SDK no debe inferir si un identificador corresponde al item canonico o a una variante. La aplicacion integradora debe enviar `item_id` y `variant_id` correctamente.

Los campos pueden recibirse en `properties`, `metadata` o `items`, segun el contrato publico vigente. El worker debe normalizarlos hacia tablas relacionales para consultas eficientes.

## Identidad De Eventos

El esquema actual de `analytics_events` ya usa:

```text
analytics_events.id = PK interna
analytics_events.event_id = ID enviado por el SDK
analytics_events.logical_event_id = ID logico del flujo
```

Para evitar ambiguedad, las tablas nuevas deben usar nombres explicitos:

```text
analytics_event_items.analytics_event_id = FK a analytics_events.id
analytics_event_items.client_event_id = copia opcional de analytics_events.event_id
analytics_event_items.logical_event_id = copia opcional de analytics_events.logical_event_id
```

No se debe usar `event_id` en tablas nuevas para representar la FK interna si eso puede confundirse con el ID generado por el SDK.

## Deduplicacion Por Tipo De Evento

La deduplicacion debe apoyarse en los campos existentes del contrato:

- `event_id`;
- `logical_event_id`;
- `idempotency_key`;
- `tab_id`;
- `sequence`;
- `anonymous_id`;
- `session_id`;
- `user_id`.

Reglas objetivo:

| Tipo | Regla recomendada |
|---|---|
| Eventos generales | `tenant_id + site_id + idempotency_key` cuando exista. |
| Eventos logicos | `tenant_id + site_id + logical_event_id` cuando exista. |
| Impresiones | `tenant_id + site_id + session_id + surface + list_instance_id + item_id + variant_id`. |
| Compras | `tenant_id + site_id + order_id + order_line_id`. |

Para `purchase_completed`, la deduplicacion debe ser estricta. Si falta `order_id` u `order_line_id` en un evento de compra, el evento puede persistirse para auditoria, pero debe quedar marcado como incompleto para scoring confiable.

La deduplicacion semantica del worker debe poder leer campos normalizados desde `properties` o desde la estructura de items para construir claves por tipo de evento.

## Tabla `analytics_event_items`

Se recomienda crear una tabla `analytics_event_items` con relacion 1:N contra `analytics_events`.

Esta decision reemplaza una relacion 1:1 porque algunos eventos pueden contener multiples items, especialmente `checkout_started` y `purchase_completed`. Para eventos de un solo item habra una sola fila asociada.

`analytics_events` sigue siendo la tabla principal de auditoria. `analytics_event_items` es la tabla analitica denormalizada para scoring por item.

Estructura conceptual:

| Campo | Uso |
|---|---|
| `id` | Identificador tecnico de la fila. |
| `analytics_event_id` | FK interna a `analytics_events.id`. |
| `client_event_id` | ID enviado por el SDK. |
| `logical_event_id` | ID logico del flujo, si existe. |
| `tenant_id` | Tenant resuelto por el worker. |
| `site_id` | Site resuelto por el worker. |
| `site_code` | Codigo publico del site. |
| `event_name` | Nombre del evento principal. |
| `event_time` | Tiempo funcional del evento. |
| `received_at` | Momento de recepcion asignado por ingesta. |
| `anonymous_id` | Identidad anonima asociada. |
| `session_id` | Sesion asociada. |
| `user_id` | Usuario asociado, si existe. |
| `item_id` | Identificador canonico del item. |
| `variant_id` | Variante concreta, si aplica. |
| `sku` | Codigo concreto, si aplica. |
| `item_type` | Tipo funcional del item, por ejemplo `product`, `service` o `subscription_plan`. |
| `item_class_id` | Clase del item. |
| `category_ids` | Lista de categorias capturadas. |
| `surface` | Superficie donde ocurrio la interaccion. |
| `position` | Posicion visible. |
| `page` | Pagina o bloque de paginacion. |
| `search_term` | Busqueda asociada. |
| `ranking_run_id` | Corrida de ranking que genero la exposicion. |
| `ranking_version` | Version del ranking usado. |
| `list_instance_id` | Instancia logica de lista o grilla. |
| `impression_batch_id` | Lote de impresiones emitido junto. |
| `visible_ratio` | Proporcion visible medida. |
| `visible_time_ms` | Tiempo visible medido. |
| `viewport_width` | Ancho de viewport registrado. |
| `viewport_height` | Alto de viewport registrado. |
| `rendered_at` | Momento de render real del item. |
| `cart_id` | Carrito asociado. |
| `checkout_id` | Checkout asociado. |
| `order_id` | Orden asociada. |
| `order_line_id` | Linea de orden asociada. |
| `quantity` | Cantidad involucrada. |
| `unit_price` | Precio unitario snapshot. |
| `currency` | Moneda. |
| `gross_amount` | Importe bruto. |
| `net_amount` | Importe neto. |
| `discount_amount` | Descuento aplicado. |
| `unit_cost` | Costo unitario snapshot opcional. |
| `cost_amount` | Costo total snapshot opcional. |
| `metadata` | Datos adicionales utiles para scoring o auditoria acotada. |
| `created_at` | Momento de persistencia. |

No es necesario duplicar todo el evento completo. Datos como `user_agent`, `url`, `referrer`, `context`, `properties` completas e `ip_hash` pueden permanecer en `analytics_events`, salvo que luego se necesiten para segmentacion o reporting avanzado.

## Tabla `analytics_event_orders`

Se recomienda evaluar una tabla `analytics_event_orders` para eventos que representan checkout u orden.

Esta tabla no reemplaza a `analytics_event_items`. Sirve para separar cabecera economica y operativa de las lineas.

Campos conceptuales:

| Campo | Uso |
|---|---|
| `id` | Identificador tecnico. |
| `analytics_event_id` | FK interna a `analytics_events.id`. |
| `client_event_id` | ID enviado por el SDK. |
| `tenant_id` | Tenant resuelto. |
| `site_id` | Site resuelto. |
| `site_code` | Codigo publico del site. |
| `event_name` | Nombre del evento. |
| `event_time` | Tiempo funcional. |
| `cart_id` | Carrito asociado. |
| `checkout_id` | Checkout asociado. |
| `order_id` | Orden asociada. |
| `currency` | Moneda. |
| `subtotal_amount` | Subtotal de la orden o checkout. |
| `discount_amount` | Descuento total. |
| `shipping_amount` | Importe de envio. |
| `tax_amount` | Impuestos. |
| `gross_amount` | Importe bruto. |
| `net_amount` | Importe neto. |
| `cost_amount` | Costo total snapshot opcional. |
| `payment_method_id` | Medio de pago, si existe. |
| `payment_provider` | Proveedor de pago, si existe. |
| `shipping_method_id` | Metodo de envio, si existe. |
| `metadata` | Datos adicionales acotados. |
| `created_at` | Momento de persistencia. |

Puede quedar para una mejora inmediata posterior si el esfuerzo de la primera implementacion debe mantenerse bajo. Si se implementa desde el inicio, mejora auditoria y reporting sin sobrecargar `analytics_event_items`.

## Indices Esperados

Los indices deben favorecer agregaciones por ventana temporal, item y superficie:

- `analytics_events(tenant_id, site_id, event_time DESC)`.
- `analytics_event_items(tenant_id, site_id, item_id, event_time DESC)`.
- `analytics_event_items(tenant_id, site_id, event_name, event_time DESC)`.
- `analytics_event_items(tenant_id, site_id, surface, event_time DESC)`.
- `analytics_event_items(tenant_id, site_id, order_id, order_line_id)` para deduplicacion y auditoria de compras.
- `analytics_event_items(ranking_run_id)` cuando se use trazabilidad de corridas.
- `GIN(category_ids)` si `category_ids` se almacena como array o JSONB en PostgreSQL.

Como alternativa futura, se puede crear `analytics_event_item_categories` como tabla puente si el analisis por categoria requiere joins relacionales mas precisos.

## Contrato Con El Proceso De Scoring

Go Analytics debe exponer los datos de forma que un proceso externo pueda calcular:

- impresiones por item;
- vistas por item;
- zooms de imagen por item;
- agregados al carrito por item;
- inicios de checkout por item;
- compras confirmadas por item;
- revenue por item;
- descuento por item;
- costo snapshot opcional por item;
- reward ponderado por ventana temporal;
- reward normalizado por impresiones;
- distribucion por superficie y categoria.

El proceso externo debe poder leer eventos de los ultimos 90 dias o de otra ventana configurada sin depender de acumulados permanentes.

## Mejoras Futuras No Bloqueantes

No son necesarias para aprobar la primera implementacion, pero deben mantenerse como lineas de evolucion:

- particionado por fecha;
- agregados diarios o vistas materializadas;
- filtrado de bots y staff;
- atribucion avanzada entre impresion, carrito y compra;
- segmentacion por campana o dispositivo;
- pruebas A/B de ranking;
- normalizacion avanzada de `search_term`.

## Criterios De Aceptacion Del Diseno

- Todo evento relacionado a item puede reconstruirse desde `analytics_events` y `analytics_event_items`.
- Los eventos sin item siguen persistiendo solo en `analytics_events`.
- Los eventos multi-item no duplican el evento principal.
- `analytics_event_items.analytics_event_id` representa la FK interna a `analytics_events.id`.
- `analytics_events.event_id` conserva el ID enviado por el SDK.
- Las impresiones incluyen evidencia minima de visibilidad real.
- Las compras incluyen datos suficientes para deduplicacion estricta por orden y linea.
- El microservicio no contiene reglas de elegibilidad del catalogo.
- El microservicio no calcula ni guarda posiciones finales de ranking.
- La nomenclatura publica y persistida usa siempre `item`.
