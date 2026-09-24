// horaio-beat manda el latido del sensor de Horaio (ADR-0008, ADR-0010).
//
//	horaio-beat [carpeta]                       manda el latido; lo llaman los hooks
//	horaio-beat connect --token crn_... [--server URL]
//	                                            guarda el token de este dispositivo
//	horaio-beat status                          muestra la configuración y si el servidor responde
//	horaio-beat mcp-headers                     cabecera de autenticación para el servidor MCP
//	horaio-beat session-start                   una línea de aviso si falta configurar; si no, nada
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
	"time"

	"github.com/Rekodes/horario/packages/horaio-beat/internal/beat"
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
		case "help", "--help", "-h":
			fmt.Println("uso: horaio-beat [carpeta] | connect --token crn_... [--server URL] | status | mcp-headers | session-start | version")
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
