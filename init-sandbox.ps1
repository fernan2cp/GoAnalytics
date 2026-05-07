# Script para inicializar el sandbox de GoAnalytics en PowerShell

# 1. Asegurar que el archivo .env existe
if (-not (Test-Path .env)) {
    Write-Host "📄 El archivo .env no existe. Creándolo desde .env.example..." -ForegroundColor Cyan
    Copy-Item .env.example .env
}

# 2. Levantar los servicios básicos
Write-Host "🚀 Levantando contenedores base..." -ForegroundColor Green
docker compose up -d --build

# 3. Ejecutar migraciones
Write-Host "🔄 Ejecutando migraciones de base de datos..." -ForegroundColor Cyan
docker compose --profile tools up migrate

# 4. Verificar éxito de las migraciones
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Migraciones completadas con éxito." -ForegroundColor Green
} else {
    Write-Host "❌ Error al ejecutar las migraciones." -ForegroundColor Red
    exit $LASTEXITCODE
}

# 5. Abrir el navegador
Write-Host "🌐 Abriendo el sandbox en http://localhost:3000..." -ForegroundColor Green
Start-Process "http://localhost:3000"

Write-Host "✨ Proceso finalizado correctamente." -ForegroundColor Green
