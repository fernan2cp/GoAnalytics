$ErrorActionPreference = "Stop"

Write-Host "Instalando Dependencias ingest..."
Set-Location "services/ingest"
go mod download
go mod tidy

Write-Host "Instalando Dependencias worker..."
Set-Location "../worker"
go mod download
go mod tidy

Set-Location "../.."

Write-Host "Dependencias GO - INSTALACIÓN COMPLETA."
