# horaio-beat

El latido del sensor de Horaio. Lo lanza el cliente de IA (un hook de Claude
Code, Cursor, Codex…) en cada interacción y le dice al servidor «en este
repositorio se está trabajando ahora». Es lo que hace que la duración sea
real y no dependa de que el modelo se acuerde de llamar.

Funciona en macOS, Linux y Windows sin depender de `sh`, `curl` ni `git`. Solo usa la librería estándar
de Go.

## Qué manda y qué no

| Manda | No manda nunca |
|---|---|
| El nombre del repositorio o de la carpeta (`{"project_hint": "horaio"}`) | El prompt, las herramientas usadas ni su salida |
| El token del dispositivo, en la cabecera `Authorization` | Rutas completas, nombres de fichero, contenido |
| `User-Agent: horaio-beat/<versión>` | Nada de lo que el cliente le pase por stdin |

**No lee stdin.** Claude Code le pasa el prompt y la salida de cada
herramienta, y el binario nunca los abre. El servidor, además, rechaza
cualquier campo que no sea `project_hint`.

## Uso

```
horaio-beat [carpeta]                         manda el latido (lo llaman los hooks)
horaio-beat connect --token crn_... [--server URL]
                                              guarda el token de este dispositivo
horaio-beat status                            configuración y si el servidor responde
horaio-beat mcp-headers                       cabecera Authorization para el servidor MCP
horaio-beat version                           imprime la versión
```

`connect` escribe `~/.horaio/config.toml` con permisos 600, en el mismo
formato que `horaio connect`; `--server` es opcional y por defecto apunta a
producción. Es provisional hasta `horaio-beat login` con código de dispositivo `status` no enseña el token entero y no manda ningún latido: solo
comprueba `/health`. `mcp-headers` imprime `{"Authorization": "Bearer …"}` para
que el plugin autentique el servidor MCP sin guardar el token en ningún otro
sitio.

Al latir no escribe nada y **sale siempre con 0**: lo que imprime un hook de
`UserPromptSubmit` acaba en el contexto del modelo, y un hook que falla
interrumpe al agente. Para ver por qué no late, `HORAIO_BEAT_DEBUG=1`
manda el error a stderr:

```
$ HORAIO_BEAT_DEBUG=1 horaio-beat
horaio-beat: el servidor respondió 401
```

## Cómo decide el proyecto

Una sesión, un nombre, el mismo que se le pide al modelo en
`start_work`:

1. La carpeta: la del argumento, si no la carpeta donde se abrió la sesión
   (`CLAUDE_PROJECT_DIR`), y si no la actual. Un `cd` del agente a mitad de
   sesión no cambia el proyecto.
2. Si esa carpeta está dentro de un repositorio git, el nombre de la raíz del
   repositorio, subiendo hasta encontrar `.git` (carpeta o fichero, para los
   worktrees). No necesita tener `git` instalado.
3. Desde el home o la raíz manda un nombre vacío, que el servidor guarda como
   «sin especificar».

## Configuración

| Variable | Qué es | Por defecto |
|---|---|---|
| `HORAIO_SERVER_URL` | URL del servidor MCP; el latido va a `<base>/v1/heartbeat` | `server_url` de `~/.horaio/config.toml` |
| `HORAIO_DEVICE_TOKEN` | Token de dispositivo (`crn_...`) | `device_token` de `~/.horaio/config.toml` |
| `HORAIO_CONFIG` | Otro fichero de configuración | `~/.horaio/config.toml` |
| `HORAIO_BEAT_MIN_SECONDS` | Muestreo local: como mucho un envío por proyecto cada N segundos | `60` |
| `HORAIO_BEAT_DEBUG` | Con cualquier valor, los errores van a stderr | vacío |

`~/.horaio/config.toml` es el que escribe `horaio connect`:

```toml
server_url = "https://app.horaio.com/mcp"
device_token = "crn_..."
```

Sin URL o sin token no hace nada. En la fase 2 el token pasará al llavero del
sistema, con `horaio-beat login`.

## Muestreo

Un hook en cada herramienta dispara varias veces por minuto. El binario
guarda por proyecto la hora del último envío en
`$TMPDIR/horaio-beat-<uid>/` y no vuelve a llamar hasta pasado el intervalo.
El formato es el mismo que el del script, así que los dos comparten sellos. El
servidor además guarda como mucho un latido cada 2 minutos por proyecto.

Medido en un Mac M-series contra producción: unos 260 ms cuando llama al
servidor, casi todo red, y unos 40 ms cuando el muestreo lo descarta. El hook
lo lanza en segundo plano (`async: true`), así que el agente no espera.

## Instalación

Con el plugin de Claude Code, siguiendo el [README del repositorio](../README.md).
A mano, en cualquier cliente con hooks:

```bash
./build.sh 0.2.0
mkdir -p ~/.horaio/bin
cp dist/horaio-beat-darwin-arm64 ~/.horaio/bin/horaio-beat   # tu plataforma
~/.horaio/bin/horaio-beat connect --token crn_...
```

Y el hook del cliente llama a `$HOME/.horaio/bin/horaio-beat` en cada prompt,
herramienta y fin de turno, siempre como comando (nunca como hook HTTP, que
mandaría el prompt entero al servidor).

## Desarrollo

Con Go 1.25 o posterior:

```bash
gofmt -l . && go vet ./... && go test ./...
./build.sh 0.2.0                 # las cinco plataformas y SHA256SUMS en dist/
```

| Plataforma | Fichero | Tamaño |
|---|---|---|
| macOS Apple Silicon | `horaio-beat-darwin-arm64` | 6,0 MB |
| macOS Intel | `horaio-beat-darwin-amd64` | 6,5 MB |
| Linux x86-64 | `horaio-beat-linux-amd64` | 6,3 MB |
| Linux ARM64 | `horaio-beat-linux-arm64` | 5,8 MB |
| Windows x86-64 | `horaio-beat-windows-amd64.exe` | 6,5 MB |

Los binarios son estáticos (`CGO_ENABLED=0`) y **reproducibles**: la misma
versión de Go y el mismo código dan los mismos bytes, así que cualquiera puede
compilar y comparar con `SHA256SUMS`.

```
main.go                   argumentos; nunca imprime al latir
internal/beat/
  config.go               ~/.horaio/config.toml y variables de entorno
  project.go              repositorio o carpeta de trabajo
  throttle.go             muestreo local por proyecto
  client.go               POST /v1/heartbeat
  run.go                  orquestación
  beat_test.go            tests, incluido uno contra un servidor HTTP falso
build.sh                  compilación cruzada
```

