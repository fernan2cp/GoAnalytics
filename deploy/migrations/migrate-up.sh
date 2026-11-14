#!/bin/sh
# Aplica únicamente migraciones pendientes usando las variables del contenedor.
set -eu

# Codifica todos los bytes para admitir credenciales y nombres con caracteres URL.
encode_uri_component() {
    printf '%s' "$1" | od -An -v -tx1 | tr -d ' \n' | sed 's/../%&/g'
}

for variable in POSTGRES_HOST POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB; do
    # Los nombres provienen exclusivamente de la lista fija anterior.
    eval 'value=${'"$variable"':-}'
    if [ -z "$value" ]; then
        printf 'Falta la variable obligatoria %s.\n' "$variable" >&2
        exit 2
    fi
done

port=${POSTGRES_PORT:-5432}
case "$port" in
    ''|*[!0-9]*) echo 'POSTGRES_PORT debe ser numérico.' >&2; exit 2 ;;
esac
if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
    echo 'POSTGRES_PORT está fuera de rango.' >&2
    exit 2
fi

sslmode=${POSTGRES_SSLMODE:-disable}
case "$sslmode" in
    disable|allow|prefer|require|verify-ca|verify-full) ;;
    *) echo 'POSTGRES_SSLMODE no es válido.' >&2; exit 2 ;;
esac

host=$POSTGRES_HOST
case "$host" in
    *'/'*|*'?'*|*'#'*|*'@'*) echo 'POSTGRES_HOST no es válido.' >&2; exit 2 ;;
    \[*\]) ;; # Conserva un literal IPv6 ya delimitado.
    *:*) host="[$host]" ;;
esac

url="postgres://$(encode_uri_component "$POSTGRES_USER"):$(encode_uri_component "$POSTGRES_PASSWORD")@$host:$port/$(encode_uri_component "$POSTGRES_DB")?sslmode=$sslmode"

# No se publica la salida del proveedor: un error podría incluir la URL secreta.
echo 'Aplicando migraciones pendientes de GoAnalytics...'
if migrate -path="${MIGRATIONS_PATH:-/migrations}" -database="$url" up >/dev/null 2>&1; then
    echo 'Base de GoAnalytics actualizada.'
else
    result=$?
    printf 'Falló la migración (código %s). Revisar conectividad, permisos y schema_migrations; no ejecutar force automáticamente.\n' "$result" >&2
    exit "$result"
fi
