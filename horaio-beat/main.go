// horaio-beat manda el latido del sensor de Horaio (ADR-0008, ADR-0010).
//
//	horaio-beat [carpeta]                       manda el latido; lo llaman los hooks
//	horaio-beat connect --token crn_... [--server URL]
//	                                            guarda el token de este dispositivo
//	horaio-beat status                          muestra la configuración y si el servidor responde
//	horaio-beat mcp-headers                     cabecera de autenticación para el servidor MCP
//	horaio-beat session-start                   una línea de aviso si falta configurar; si no, nada
//	horaio-beat antigravity-hook                el hook de Antigravity: lee su JSON y contesta {}
//	horaio-beat handoff-hook                    tras record_activity, sugiere /clear si hay mucho contexto
//	horaio-beat version                         imprime la versión
//
// Al latir no escribe nada en stdout y sale siempre con 0: lo que imprime un
// hook de UserPromptSubmit acaba en el contexto del modelo, y un hook que
// falla interrumpe al agente. Con HORAIO_BEAT_DEBUG=1, los errores van a stderr.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/Rekodes/horaio/packages/horaio-beat/internal/beat"
)

// version la fija el build con -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Println(version)
			return
		case "connect":
			os.Exit(connect(os.Args[2:]))
		case "status":
			status()
			return
		case "mcp-headers":
			os.Exit(mcpHeaders())
		case "session-start":
			sessionStart()
			return
		case "antigravity-hook":
			antigravityHook()
			return
		case "handoff-hook":
			handoffHook()
			return
		case "help", "--help", "-h":
			fmt.Println("uso: horaio-beat [carpeta] | connect --token crn_... [--server URL] | status | mcp-headers | session-start | antigravity-hook | handoff-hook | version")
			return
		}
	}
	beatOnce(firstArg())
}

func beatOnce(dir string) {
	err := beat.Run(context.Background(), beat.SystemEnv(dir, version))
	if err != nil && os.Getenv("HORAIO_BEAT_DEBUG") != "" {
		fmt.Fprintln(os.Stderr, "horaio-beat:", err)
	}
	os.Exit(0)
}

func connect(args []string) int {
	flags := flag.NewFlagSet("connect", flag.ContinueOnError)
	token := flags.String("token", "", "token de dispositivo (crn_...)")
	server := flags.String("server", beat.DefaultServerURL, "URL del servidor MCP")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	path := beat.ConfigPath(os.Getenv, home())
	if err := beat.Connect(path, *server, *token); err != nil {
		fmt.Fprintln(os.Stderr, "horaio-beat:", err)
		return 1
	}
	fmt.Println("guardado en", path)
	status()
	return 0
}

func status() {
	cfg := beat.LoadConfig(os.Getenv, home())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Second}
	for _, line := range beat.Status(ctx, client, cfg, beat.ConfigPath(os.Getenv, home())) {
		fmt.Println(line)
	}
}

// sessionStart lo llama el hook de SessionStart: lo que imprime llega al
// contexto del modelo, así que solo habla cuando hay algo que hacer.
func sessionStart() {
	if line := beat.SessionNotice(beat.LoadConfig(os.Getenv, home())); line != "" {
		fmt.Println(line)
	}
}

// antigravityHook lo llama el hook de Antigravity. Antigravity espera un
// objeto JSON en stdout y para al agente mientras el hook corre, así que
// contesta `{}` enseguida y el latido lo manda un proceso aparte, que sigue
// cuando este sale. Sin carpeta de trabajo no late: la actual es la de
// hooks.json, no la del proyecto.
func antigravityHook() {
	dir := beat.WorkspaceDir(os.Stdin)
	fmt.Println("{}")
	if dir == "" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if err := exec.Command(exe, dir).Start(); err != nil && os.Getenv("HORAIO_BEAT_DEBUG") != "" {
		fmt.Fprintln(os.Stderr, "horaio-beat:", err)
	}
}

// handoffHook lo llama el PostToolUse de record_activity (ADR-0035). Lo que
// imprime llega al contexto del modelo: por debajo del umbral, nada.
func handoffHook() {
	if out := beat.HandoffHook(os.Stdin, os.Getenv, beat.StateDir()); out != "" {
		fmt.Println(out)
	}
}

func mcpHeaders() int {
	out, err := beat.MCPHeaders(beat.LoadConfig(os.Getenv, home()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "horaio-beat:", err)
		return 1
	}
	fmt.Println(string(out))
	return 0
}

func firstArg() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	return ""
}

func home() string {
	dir, _ := os.UserHomeDir()
	return dir
}
