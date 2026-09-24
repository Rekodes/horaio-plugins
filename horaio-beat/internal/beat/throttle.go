package beat

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ShouldSend aplica el muestreo local: un hook en cada herramienta dispara
// varias veces por minuto, y basta con un latido por intervalo y proyecto. El
// sello es un fichero por proyecto con la hora Unix del último envío, el mismo
// formato que usa horaio-beat.sh. Si no se puede leer o escribir, se envía.
func ShouldSend(stampDir, project string, now time.Time, interval time.Duration) bool {
	stamp := filepath.Join(stampDir, stampName(project))
	if raw, err := os.ReadFile(stamp); err == nil {
		if last, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil {
			if now.Sub(time.Unix(last, 0)) < interval {
				return false
			}
		}
	}
	if err := os.MkdirAll(stampDir, 0o700); err == nil {
		_ = os.WriteFile(stamp, []byte(strconv.FormatInt(now.Unix(), 10)), 0o600)
	}
	return true
}

// stampName deja solo caracteres seguros en un nombre de fichero.
func stampName(project string) string {
	if project == "" {
		return "_"
	}
	var b strings.Builder
	for _, r := range project {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
