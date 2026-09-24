package beat

import (
	"os"
	"path/filepath"
)

// SessionDir es la carpeta que da nombre al proyecto: la que se pase como
// argumento, si no la carpeta donde se abrió la sesión (CLAUDE_PROJECT_DIR),
// y si no la actual. Un `cd` del agente a mitad de sesión no cambia el
// proyecto: así coincide con el nombre que usa el modelo en `start_work`.
func SessionDir(arg, launch, cwd string) string {
	switch {
	case arg != "":
		return arg
	case launch != "":
		return launch
	default:
		return cwd
	}
}

// ProjectName es el nombre del repositorio que contiene dir, o el de dir si
// no está en ninguno. Desde el home o la raíz devuelve "": el servidor lo
// guarda como «sin especificar».
func ProjectName(dir, home string) string {
	top := repoRoot(dir)
	if top == "" {
		top = filepath.Clean(dir)
	}
	if top == filepath.Clean(home) || isFilesystemRoot(top) {
		return ""
	}
	return filepath.Base(top)
}

// repoRoot sube desde dir hasta encontrar `.git`, carpeta o fichero (los
// worktrees usan un fichero). Sin llamar a `git`, que puede no estar instalado.
func repoRoot(dir string) string {
	if dir == "" {
		return ""
	}
	current := filepath.Clean(dir)
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func isFilesystemRoot(path string) bool {
	return filepath.Dir(path) == path
}
