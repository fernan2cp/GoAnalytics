# GoAnalytics Sandbox

El entorno Sandbox está diseñado para facilitar pruebas locales y demostraciones del funcionamiento end-to-end del microservicio Go Analytics. Incluye un backend simulado en Go y un frontend interactivo en HTML/JS.

## Propósito

El Sandbox permite:
1. **Verificar la Ingestión:** Comprobar que los eventos enviados desde el frontend se procesan y almacenan correctamente en PostgreSQL.
2. **Simular el Backend Principal:** Generar tokens JWT válidos para inicializar el SDK de forma automática e interceptar las peticiones de resolución de sitios (hidratación de metadatos) que realiza el microservicio de ingesta internamente.
3. **Explorar el SDK:** Proveer una interfaz visual con botones preconfigurados para lanzar eventos estandarizados y personalizados.

## Instalación y Ejecución Rápida

Para facilitar la puesta en marcha, se han incluido scripts de automatización en la raíz del proyecto que se encargan de configurar el entorno, ejecutar migraciones y abrir el navegador.

**Desde Windows (PowerShell):**
```powershell
.\init-sandbox.ps1
```

**Desde Git Bash / Linux / macOS:**
```bash
chmod +x init-sandbox.sh
./init-sandbox.sh
```

Esto levantará los siguientes componentes:
- **Sandbox Frontend/Backend:** Expuesto en el puerto `3000`.
- **GoAnalytics Ingest:** Expuesto en el puerto local `8081` (mapeado al 8080 interno).
- **PostgreSQL:** Expuesto en el puerto local `5433`.
- **Worker y Redis:** Ejecutándose en la red interna de Docker.

## Uso del Sandbox

1. **Acceder a la Interfaz:**
   Abre tu navegador en [http://localhost:3000](http://localhost:3000).

2. **Probar Eventos:**
   Utiliza los botones disponibles para enviar eventos (`page_view`, `button_click`, etc.). El SDK en el navegador está configurado para apuntar a `localhost:8081`.

3. **Verificar Eventos:**
   Navega a "Ver Eventos Registrados" para consultar la tabla de PostgreSQL en tiempo real.

## Desarrollo y Personalización

El archivo `index.html` del sandbox está mapeado como un **volumen** en el `docker-compose.yml`. Esto significa que puedes editar el código en `sandbox/public/index.html` y ver los cambios en el navegador simplemente refrescando la página, sin necesidad de reiniciar Docker.
