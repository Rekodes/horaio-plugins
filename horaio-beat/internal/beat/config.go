package beat

import (
	"bufio"
	"os"
	"strings"
)

// Config es a dónde latir y con qué credencial.
type Config struct {
	ServerURL string
	Token     string
}

// Complete dice si hay lo mínimo para mandar un latido.
func (c Config) Complete() bool {
	return c.ServerURL != "" && c.Token != ""
}

// LoadConfig lee ~/.horaio/config.toml (o HORAIO_CONFIG), el mismo fichero
// que escribe `horaio connect`. HORAIO_SERVER_URL y HORAIO_DEVICE_TOKEN tienen
// prioridad. Un fichero que no existe no es un error: queda sin configurar.
func LoadConfig(getenv func(string) string, home string) Config {
	values := readConfigFile(ConfigPath(getenv, home))

	cfg := Config{ServerURL: values["server_url"], Token: values["device_token"]}
	if v := getenv("HORAIO_SERVER_URL"); v != "" {
		cfg.ServerURL = v
	}
	if v := getenv("HORAIO_DEVICE_TOKEN"); v != "" {
		cfg.Token = v
	}
	return cfg
}

// readConfigFile entiende solo `clave = "valor"`, que es todo lo que escribe
// `horaio connect`. No hace falta un parser de TOML completo.
func readConfigFile(path string) map[string]string {
	values := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return values
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value := strings.TrimSpace(raw)
		if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
			continue
		}
		values[strings.TrimSpace(key)] = value[1 : len(value)-1]
	}
	return values
}

// HeartbeatURL deriva el endpoint del latido de la URL de MCP guardada:
// `https://app.horaio.com/mcp` -> `https://app.horaio.com/v1/heartbeat`.
func HeartbeatURL(serverURL string) string {
	base := strings.TrimSuffix(serverURL, "/")
	base = strings.TrimSuffix(base, "/mcp")
	return base + "/v1/heartbeat"
}
