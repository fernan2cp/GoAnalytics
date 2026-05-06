$ErrorActionPreference = "Stop"

Write-Host "--- INSTALANDO HERRAMIENTAS DE DESARROLLO ---"

Write-Host "Instalando golang-migrate con soporte PostgreSQL..."
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

Write-Host "--- TOOLS INSTALACION COMPLETA ---"