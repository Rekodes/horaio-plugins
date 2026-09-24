package beat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DefaultServerURL es el servidor de Horaio en producción.
const DefaultServerURL = "https://app.horaio.com/mcp"

// ConfigPath es el fichero que leen el latido y `mcp-headers`: HORAIO_CONFIG o
// ~/.horaio/config.toml.
func ConfigPath(getenv func(string) string, home string) string {
	if path := getenv("HORAIO_CONFIG"); path != "" {
		return path
	}
	return filepath.Join(home, ".horaio", "config.toml")
}

// Connect guarda el token del dispositivo, en el mismo formato que
// `horaio connect`. Solo el usuario puede leerlo (0600). Provisional hasta el
// login de dispositivo con el token en el llavero (ADR-0010).
func Connect(path, serverURL, token string) error {
	if !strings.HasPrefix(token, "crn_") {
		return errors.New("el token de dispositivo empieza por crn_")
	}
	if strings.ContainsAny(token+serverURL, "\"\n\r") {
		return errors.New("el token o la URL tienen caracteres no válidos")
	}
	if serverURL == "" {
		serverURL = DefaultServerURL
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	content := fmt.Sprintf("server_url = %q\ndevice_token = %q\n", serverURL, token)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// Status resume la configuración sin enseñar el token entero y comprueba que
// el servidor responde. No manda ningún latido.
func Status(ctx context.Context, client *http.Client, cfg Config, path string) []string {
	lines := []string{"configuración: " + path}
	if !cfg.Complete() {
		return append(lines, "estado: sin configurar. Ejecuta: horaio-beat connect --token crn_...")
	}
	lines = append(lines,
		"servidor: "+cfg.ServerURL,
		"token: "+tokenPrefix(cfg.Token)+"…",
		"latido: "+HeartbeatURL(cfg.ServerURL),
	)
	if err := checkHealth(ctx, client, cfg.ServerURL); err != nil {
		return append(lines, "estado: el servidor no responde ("+err.Error()+")")
	}
	return append(lines, "estado: servidor accesible")
}

// MCPHeaders es la salida de `mcp-headers`: la cabecera de autenticación del
// servidor MCP, para que el token viva en un solo sitio y no en el plugin.
func MCPHeaders(cfg Config) ([]byte, error) {
	headers := map[string]string{}
	if cfg.Token != "" {
		headers["Authorization"] = "Bearer " + cfg.Token
	}
	return json.Marshal(headers)
}

// SessionNotice es el aviso del arranque de sesión: vacío si está configurado.
func SessionNotice(cfg Config) string {
	if cfg.Complete() {
		return ""
	}
	return "Horaio no mide esta sesión: falta el token de este equipo. Díselo al usuario: " +
		"se genera en https://app.horaio.com/conectar y se guarda con `horaio-beat connect --token crn_...`."
}

func tokenPrefix(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8]
}

func checkHealth(ctx context.Context, client *http.Client, serverURL string) error {
	base := strings.TrimSuffix(strings.TrimSuffix(serverURL, "/"), "/mcp")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("respondió %d", resp.StatusCode)
	}
	return nil
}
