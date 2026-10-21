package beat

import (
	"encoding/json"
	"io"
	"net/url"
	"strings"
)

// WorkspaceDir saca la carpeta de trabajo del JSON que Antigravity pasa a sus
// hooks por stdin. Hace falta porque Antigravity los ejecuta desde la carpeta
// de hooks.json (~/.gemini/config), no desde el proyecto. Solo decodifica
// `workspacePaths`: el resto del JSON (argumentos de herramientas, rutas del
// transcript) se descarta sin mirarlo. Sin carpeta devuelve "".
func WorkspaceDir(r io.Reader) string {
	var payload struct {
		WorkspacePaths []string `json:"workspacePaths"`
	}
	if err := json.NewDecoder(r).Decode(&payload); err != nil || len(payload.WorkspacePaths) == 0 {
		return ""
	}
	dir := payload.WorkspacePaths[0]
	if strings.HasPrefix(dir, "file://") {
		if u, err := url.Parse(dir); err == nil {
			return u.Path
		}
		return ""
	}
	return dir
}
