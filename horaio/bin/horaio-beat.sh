#!/bin/sh
# ---------------------------------------------------------------------------
# Latido del sensor de Horaio (ADR-0008).
#
# Lo lanza el cliente en cada interacción -- un hook de Claude Code, el
# equivalente de otra herramienta, o un envoltorio de terminal -- y le dice al
# servidor «en esta carpeta se está trabajando ahora». Nada más: el modelo no
# decide nada y no tiene que acordarse de nada.
#
# QUÉ MANDA: el nombre del repositorio git (o de la carpeta) y el token del
# dispositivo. Nunca lee stdin, así que nunca ve el prompt, la herramienta ni
# su salida, aunque el cliente se los pase.
#
# Configuración, la misma que `horaio connect`: ~/.horaio/config.toml, o las
# variables HORAIO_SERVER_URL y HORAIO_DEVICE_TOKEN.
#
#   horaio-beat.sh [carpeta]     por defecto, la carpeta actual
#
# Sale siempre con 0 y sin escribir nada: un latido perdido cuesta un par de
# minutos, un hook que falla o que imprime interrumpe al agente (lo que un
# hook de UserPromptSubmit escribe en stdout acaba en el contexto del modelo).
# ---------------------------------------------------------------------------
set -u

config="${HORAIO_CONFIG:-$HOME/.horaio/config.toml}"

read_key() {
  [ -r "$config" ] || return 0
  sed -n "s/^$1[[:space:]]*=[[:space:]]*\"\(.*\)\"[[:space:]]*$/\1/p" "$config" | head -n 1
}

token="${HORAIO_DEVICE_TOKEN:-$(read_key device_token)}"
url="${HORAIO_SERVER_URL:-$(read_key server_url)}"
[ -n "$token" ] && [ -n "$url" ] || exit 0

# server_url apunta a /mcp; el latido vive al lado, en /v1/heartbeat.
base="${url%/}"
base="${base%/mcp}"

# El proyecto sale de la carpeta donde se abrió la sesión (CLAUDE_PROJECT_DIR
# en Claude Code), o de su repositorio si lo es: un `cd` del agente a mitad de
# sesión no lo cambia, y así coincide con el nombre que usa el modelo
# (ADR-0011). Sin sesión, la carpeta actual.
dir="${1:-${CLAUDE_PROJECT_DIR:-$PWD}}"
top=$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null) || top="$dir"

# Desde el home o la raíz no hay proyecto que nombrar: vacío es «sin
# especificar» en el servidor (ADR-0007 §2), que es lo honrado.
name=$(basename "$top")
if [ "$top" = "$HOME" ] || [ "$top" = "/" ]; then
  name=""
fi

# Muestreo local: un hook en cada herramienta dispara varias veces por minuto.
# El servidor ya descarta lo que sobra, pero no hace falta ni la petición.
stamp_dir="${TMPDIR:-/tmp}/horaio-beat-$(id -u)"
stamp="$stamp_dir/$(printf '%s' "${name:-_}" | tr -c 'A-Za-z0-9._-' '_')"
now=$(date +%s)
if [ -r "$stamp" ]; then
  last=$(cat "$stamp" 2>/dev/null || echo 0)
  [ $((now - ${last:-0})) -lt "${HORAIO_BEAT_MIN_SECONDS:-60}" ] && exit 0
fi
mkdir -p "$stamp_dir" 2>/dev/null && printf '%s' "$now" >"$stamp" 2>/dev/null

escaped=$(printf '%s' "$name" | sed 's/\\/\\\\/g; s/"/\\"/g')

# El token va por stdin (-H @-) y no en la línea de órdenes, donde lo vería
# cualquiera con `ps`.
printf 'Authorization: Bearer %s\n' "$token" | curl -fsS --max-time 5 \
  -X POST "$base/v1/heartbeat" \
  -H @- \
  -H 'Content-Type: application/json' \
  -d "{\"project_hint\":\"$escaped\"}" >/dev/null 2>&1

exit 0
