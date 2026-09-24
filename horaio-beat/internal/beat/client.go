package beat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// payload es el contrato completo de POST /v1/heartbeat (ADR-0008): el
// servidor rechaza cualquier otro campo, así que no hay dónde colar contenido.
type payload struct {
	ProjectHint string `json:"project_hint"`
}

// Send manda un latido. Un 2xx es éxito, también `stored: false`, que solo
// significa que el servidor ya tenía un evento reciente de ese proyecto.
func Send(ctx context.Context, client *http.Client, url, token, project, userAgent string) error {
	body, err := json.Marshal(payload{ProjectHint: project})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("el servidor respondió %d", resp.StatusCode)
	}
	return nil
}
