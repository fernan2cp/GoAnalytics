# GoAnalytics Sandbox

El entorno Sandbox está diseñado para facilitar pruebas locales y demostraciones del funcionamiento end-to-end del microservicio Go Analytics. Incluye un backend simulado en Go y un frontend interactivo en HTML/JS.

## Propósito

El Sandbox permite:
1. **Verificar la Ingestión:** Comprobar que los eventos enviados desde el frontend se procesan y almacenan correctamente en PostgreSQL.
2. **Simular el Backend Principal:** Generar tokens JWT válidos para inicializar el SDK de forma automática e interceptar las peticiones de resolución de sitios (hidratación de metadatos) que realiza el microservicio de ingesta internamente.
3. **Explorar el SDK:** Proveer una interfaz visual con botones preconfigurados para lanzar eventos estandarizados y personalizados.

## Instalación y Ejecución

Para iniciar el Sandbox junto con los servicios del core (Ingest, Worker, Redis, Postgres), ejecuta en la raíz del proyecto:

```bash
docker compose up --build
```

Esto levantará los siguientes componentes de forma automatizada:
- **Sandbox Frontend/Backend:** Expuesto en el puerto `3000`.
- **PostgreSQL:** Expuesto en el puerto local `5433` (para evitar conflictos con tu DB local).
- **GoAnalytics Ingest, Worker y Redis:** Ejecutándose en la red interna de Docker, sin exponer puertos externos por defecto.

## Uso del Sandbox

1. **Acceder a la Interfaz:**
   Abre tu navegador web y navega a [http://localhost:3000](http://localhost:3000).

2. **Probar Eventos:**
   En la pantalla principal, utiliza los botones disponibles para enviar distintos tipos de eventos (`page_view`, `button_click`, `checkout_started`, e identificar usuarios).

3. **Verificar Eventos:**
   Haz clic en "Ver Eventos Registrados" en el menú de navegación para abrir la tabla que lee directamente de PostgreSQL. Los eventos que generes aparecerán allí tras unos pocos segundos.

## Notas sobre Puertos y Acceso Directo

Para evitar conflictos en entornos locales de desarrollo, los puertos predeterminados del microservicio en el archivo `docker-compose.yml` han sido **comentados**. Esto significa que no podrás acceder directamente al microservicio de ingestión desde tu navegador (por ejemplo a `http://localhost:8080`).

Si deseas realizar pruebas directas contra los microservicios sin utilizar el Sandbox (ej: usando cURL, Postman o desarrollo frontend local), debes editar el archivo `docker-compose.yml` y descomentar los bloques `ports:` correspondientes a los servicios que necesites.

Por ejemplo, para exponer el Ingest:
```yaml
  analytics-ingest:
    # ...
    ports:
      - "8080:8080"
```
Luego de realizar este cambio, reinicia los contenedores:
```bash
docker compose up -d
```
