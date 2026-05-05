---
name: go-analytics-standards
description: Estandares obligatorios para trabajar en el proyecto Go Analytics. Usar siempre que Codex cree, modifique o revise codigo, documentacion, comentarios, GoDoc, contratos, README, archivos Markdown o implementaciones Go dentro del repositorio GoAnalytics.
---

# Estandares de Go Analytics

## Idioma

- Escribir todos los comentarios de codigo en espanol.
- Escribir toda la documentacion de proyecto en espanol.
- Mantener nombres de funciones, tipos, paquetes, variables, endpoints y campos tecnicos en ingles cuando sea lo idiomatico o ya forme parte del contrato.
- Evitar mezclar explicaciones en ingles dentro de Markdown, GoDoc, comentarios inline o ejemplos narrativos.

## GoDoc obligatorio

Documentar toda funcion, metodo, tipo exportado, interfaz exportada y constructor con comentarios estilo GoDoc.

Cada comentario GoDoc debe cubrir, de forma concisa:

- Finalidad: que hace y por que existe.
- Entradas: parametros recibidos y significado.
- Salidas: valores devueltos y significado.
- Tipos de datos relevantes: estructuras, interfaces o valores esperados.
- Condiciones de uso: precondiciones, invariantes o contexto donde corresponde usarlo.
- Condiciones de error: cuando devuelve error, cuando no lo hace o como representa fallos.

Para funciones no exportadas, agregar comentario en espanol cuando tengan logica propia, efectos secundarios, errores, I/O, concurrencia, validaciones o decisiones de dominio. Las funciones triviales pueden tener comentarios breves, pero nunca comentarios en ingles.

## Formato recomendado

Usar parrafos breves en lugar de bloques largos. Ejemplo:

```go
// NewIngestEventsUseCase crea el caso de uso de ingesta de eventos.
//
// Recibe puertos de salida para verificar tokens, publicar eventos, aplicar
// limites, obtener tiempo y registrar logs. Devuelve una instancia lista para
// ejecutar la orquestacion de ingesta.
//
// Debe usarse desde bootstrap con adaptadores ya inicializados. No devuelve
// error porque solo asigna dependencias; las fallas de dependencias se reportan
// al ejecutar el caso de uso.
func NewIngestEventsUseCase(...) *IngestEventsUseCase
```

## Revision antes de finalizar

Antes de entregar cambios:

- Buscar comentarios o documentacion en ingles agregados durante la tarea.
- Confirmar que los archivos Markdown nuevos esten en espanol.
- Confirmar que las funciones Go nuevas tengan GoDoc suficiente.
- Confirmar que `domain` y `application` no importen infraestructura concreta.
