package beat

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	defaultInterval = 60 * time.Second
	requestTimeout  = 5 * time.Second
)

// ErrNotConfigured: no hay URL o token. No se manda nada.
var ErrNotConfigured = errors.New("sin configurar: falta HORAIO_SERVER_URL o HORAIO_DEVICE_TOKEN")

// Env es todo lo que Run lee del sistema, para poder probarlo sin tocarlo.
type Env struct {
	Getenv   func(string) string
	Arg      string // carpeta pasada como argumento; vacío si no hay
	Cwd      string
	Home     string
	StampDir string
	Now      time.Time
	Client   *http.Client
	Version  string
}

// SystemEnv construye Env desde el proceso real.
func SystemEnv(arg, version string) Env {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	return Env{
		Getenv:   os.Getenv,
		Arg:      arg,
		Cwd:      cwd,
		Home:     home,
		StampDir: stampDir(),
		Now:      time.Now(),
		Client:   &http.Client{Timeout: requestTimeout},
		Version:  version,
	}
}

// Run manda un latido si hay configuración y el muestreo lo permite. Nunca
// lee stdin: el cliente le pasa el prompt y la salida de las herramientas, y
// no tiene por qué verlos.
func Run(ctx context.Context, env Env) error {
	cfg := LoadConfig(env.Getenv, env.Home)
	if !cfg.Complete() {
		return ErrNotConfigured
	}

	dir := SessionDir(env.Arg, env.Getenv("CLAUDE_PROJECT_DIR"), env.Cwd)
	project := ProjectName(dir, env.Home)

	if !ShouldSend(env.StampDir, project, env.Now, interval(env.Getenv)) {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	return Send(ctx, env.Client, HeartbeatURL(cfg.ServerURL), cfg.Token, project, "horaio-beat/"+env.Version)
}

func interval(getenv func(string) string) time.Duration {
	if seconds, err := strconv.Atoi(getenv("HORAIO_BEAT_MIN_SECONDS")); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return defaultInterval
}

// stampDir es por usuario, como el de horaio-beat.sh, para compartir sellos.
// En Windows os.TempDir ya es del usuario y Getuid devuelve -1.
func stampDir() string {
	name := "horaio-beat"
	if uid := os.Getuid(); uid >= 0 {
		name += "-" + strconv.Itoa(uid)
	}
	return filepath.Join(os.TempDir(), name)
}
