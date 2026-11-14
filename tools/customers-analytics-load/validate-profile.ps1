param(
    [string]$ProfilePath = (Join-Path $PSScriptRoot "profile.json")
)

$resolvedProfile = Resolve-Path -LiteralPath $ProfilePath -ErrorAction Stop
$profile = Get-Content -LiteralPath $resolvedProfile -Raw -Encoding UTF8 | ConvertFrom-Json

$expected = @{
    tenants = 20
    subjects = 100000
    subject_events_per_month = 500000
    query_limit = 10
}

foreach ($entry in $expected.GetEnumerator()) {
    if ($profile.($entry.Key) -ne $entry.Value) {
        throw "Perfil inválido: $($entry.Key) debe ser $($entry.Value)"
    }
}

if ($profile.objectives.query_p95_ms -ne 300 -or
    $profile.objectives.global_projection_max_minutes -ne 30 -or
    $profile.objectives.bootstrap_max_minutes -ne 60) {
    throw "Perfil inválido: los objetivos no coinciden con AC-CA-029"
}

$checksum = (Get-FileHash -LiteralPath $resolvedProfile -Algorithm SHA256).Hash.ToLowerInvariant()
[ordered]@{
    status = "valid"
    profile = $resolvedProfile.Path
    sha256 = $checksum
    seed = $profile.seed
    validated_at_utc = [DateTime]::UtcNow.ToString("o")
} | ConvertTo-Json
