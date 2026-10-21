# Horaio · plugins

El sensor de [Horaio](https://app.horaio.com), el fichador automático para
quien trabaja con agentes de IA. Mide cuánto tiempo trabajas con el agente, y
en qué proyecto, sin que el modelo tenga que acordarse de apuntar nada.

Este repositorio es el código que se ejecuta en tu máquina, y es abierto (MIT)
para que puedas comprobar qué hace. El servidor de Horaio no está aquí.

```
.claude-plugin/marketplace.json   marketplace «horaio» para Claude Code
horaio/                           plugin de Claude Code: hooks y lanzador
horaio-contexto/                  plugin opcional: sugiere /clear con mucho contexto
horaio-beat/                      el binario que manda el latido (Go, sin dependencias)
```

## Qué manda y qué no

En cada prompt, cada herramienta que usa el agente y cada fin de turno,
`horaio-beat` manda **un solo dato**: el nombre de la carpeta donde se abrió la
sesión, o de la raíz de su repositorio git; si se abrió en el home, el de la
carpeta actual. Como mucho una vez por minuto y proyecto.

```json
{ "project_hint": "mi-proyecto" }
```

**Nunca** manda el prompt, las respuestas, las herramientas usadas, su salida,
rutas ni contenido de ficheros. El binario no lee lo que Claude Code le pasa
por stdin, y el servidor rechaza cualquier campo distinto de `project_hint`.
El código está en [`horaio-beat/`](horaio-beat) para comprobarlo.

## Instalación en Claude Code

Necesitas una cuenta de Horaio y un token de dispositivo (`crn_...`).

**1. El binario.** Mientras no haya versiones publicadas, se compila con Go
1.25 o posterior:

```bash
git clone https://github.com/Rekodes/horaio-plugins
cd horaio-plugins/horaio-beat
./build.sh 0.2.0
mkdir -p ~/.horaio/bin
cp dist/horaio-beat-darwin-arm64 ~/.horaio/bin/horaio-beat   # elige tu plataforma
```

**2. El token del dispositivo**, una vez:

```bash
~/.horaio/bin/horaio-beat connect --token crn_...
~/.horaio/bin/horaio-beat status
```

**3. El plugin:**

```
/plugin marketplace add Rekodes/horaio-plugins
/plugin install horaio@horaio
```

**4. Las herramientas MCP de Horaio:**

```bash
claude mcp add horaio --scope user --transport http \
  https://app.horaio.com/mcp --header "Authorization: Bearer crn_..."
```

Abre una sesión nueva: `/hooks` debe mostrar los cuatro hooks del plugin. Si
falta el token o el binario, Horaio te lo dice al empezar la sesión.

## Qué instala el plugin

| Evento | Qué hace |
|---|---|
| `SessionStart` | Si falta el token o el binario, una línea de aviso. Si no, nada |
| `UserPromptSubmit`, `PostToolUse`, `Stop` | El latido, en segundo plano: el agente no espera |

El lanzador (`horaio/bin/horaio-beat-hook`) busca `horaio-beat` junto al
plugin, en `~/.horaio/bin/` y en el PATH. Sin binario no hace nada. Nunca
imprime nada al latir ni falla, para no interrumpir al agente.

## Opcional: `horaio-contexto`

Una conversación larga reenvía todo lo anterior en cada turno: cuanto más se
alarga, más tokens gasta cada pregunta. Este segundo plugin te avisa de cuándo
compensa cerrarla y empezar otra. Tras cada actividad registrada, mira en el
transcript de la sesión cuántos tokens ocupa la conversación, en tu máquina y
sin mandarlo a ningún sitio. Por encima de 200k, el agente termina su respuesta
con una línea como esta:

> Contexto: 250k. Si has terminado con esto, puedes hacer /clear; la siguiente
> conversación retoma con lo registrado en Horaio.

Si tu cuenta está en «solo medir», la línea no habla de retomar, porque en ese
modo Horaio no devuelve nada al empezar. Se ajusta con
`HORAIO_HANDOFF_MIN_TOKENS` y `HORAIO_HANDOFF_STEP_TOKENS`; con
`HORAIO_HANDOFF_MIN_TOKENS=0` no avisa.

Va aparte porque, a diferencia de `horaio`, lee lo que Claude Code le pasa por
stdin y el transcript: solo el tamaño, y no manda nada. Se instala después de
`horaio` y necesita `horaio-beat` 0.4.0 o posterior:

```
/plugin install horaio-contexto@horaio
```

## Actualizar

```bash
claude plugin marketplace update horaio
claude plugin update horaio@horaio
```

El binario se actualiza aparte, en `~/.horaio/bin`.

## Desinstalar

```bash
claude plugin uninstall horaio@horaio
rm -rf ~/.horaio
```

## Licencia

MIT. Ver [`LICENSE`](LICENSE).
