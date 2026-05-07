#!/bin/bash

# Script para inicializar el sandbox de GoAnalytics

# 1. Asegurar que el archivo .env existe
if [ ! -f .env ]; then
    echo "📄 El archivo .env no existe. Creándolo desde .env.example..."
    cp .env.example .env
fi

# 2. Levantar los servicios básicos (ingest, worker, redis, postgres, sandbox)
echo "🚀 Levantando contenedores base..."
docker compose up -d --build

# 3. Esperar a que Postgres esté saludable y ejecutar migraciones
# Docker Compose esperará por la condición 'service_healthy' definida en el YAML
echo "🔄 Ejecutando migraciones de base de datos..."
docker compose --profile tools up migrate

# 4. Verificar si las migraciones terminaron con éxito
if [ $? -eq 0 ]; then
    echo "✅ Migraciones completadas con éxito."
else
    echo "❌ Error al ejecutar las migraciones."
    exit 1
fi

# 5. Abrir el navegador en localhost:3000
echo "🌐 Abriendo el sandbox en http://localhost:3000..."

case "$OSTYPE" in
  darwin*)  open "http://localhost:3000" ;; 
  linux*)   xdg-open "http://localhost:3000" || echo "Por favor, abre manualmente: http://localhost:3000" ;;
  msys*)    start "http://localhost:3000" ;;
  cygwin*)  start "http://localhost:3000" ;;
  *)        echo "Por favor, abre manualmente: http://localhost:3000" ;;
esac

echo "✨ Proceso finalizado correctamente."
