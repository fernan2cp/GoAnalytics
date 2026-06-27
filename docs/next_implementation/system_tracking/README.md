# System Tracking Generico

## Objetivo

Este paquete documenta extensiones genericas para que `GoAnalytics` pueda recibir, validar, persistir y exponer eventos de comportamiento de uso en productos distintos, sin acoplarse a un sistema integrador particular.

La guia esta pensada como insumo para un SDD posterior dentro del proyecto `GoAnalytics`.

## Alcance

El microservicio debe conservar sus responsabilidades principales:

- Ingestar eventos por contrato publico estable.
- Validar tokens, limites y seguridad basica.
- Publicar eventos aceptados para procesamiento asincrono.
- Persistir eventos base y detalles normalizados cuando aplique.
- Mantener compatibilidad con integradores existentes.

Las extensiones propuestas agregan convenciones genericas para:

- Contexto funcional de uso.
- Eventos de comportamiento.
- Busquedas y seleccion de resultados.
- Friccion y abandono de formularios.
- Agregados opcionales para rankings o sugerencias.

## No Objetivos

`GoAnalytics` no debe conocer nombres internos de ningun integrador. No debe asumir ERP, CMS, CRM, POS, turnos, suscripciones ni ofertas como conceptos obligatorios. Esos valores pueden aparecer como datos enviados por un integrador, pero el contrato debe seguir siendo generico.

Tampoco debe transformarse obligatoriamente en motor de recomendacion online. Puede almacenar eventos normalizados y, si se implementa, exponer agregados genericos opcionales.

## Orden De Lectura

1. `01_contexto_y_alcance_generico.md`
2. `02_contrato_contexto_eventos.md`
3. `03_contrato_eventos_comportamiento.md`
4. `04_contrato_agregados_y_consulta.md`
5. `05_cambios_recomendados_goanalytics.md`
6. `06_plan_pruebas_contrato.md`
