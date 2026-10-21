package beat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultHandoffMinTokens  = 200_000
	defaultHandoffStepTokens = 100_000
)

// HandoffHook es el PostToolUse de record_activity (ADR-0035). Si la
// conversación lleva mucho contexto, devuelve el JSON que pide a Claude
// sugerir /clear; si no, "". Vuelve a avisar solo cuando el contexto crece
// otro tramo. Todo es local: lee el transcript para medir y no manda nada.
func HandoffHook(r io.Reader, getenv func(string) string, stateDir string) string {
	var event struct {
		SessionID      string          `json:"session_id"`
		TranscriptPath string          `json:"transcript_path"`
		ToolResponse   json.RawMessage `json:"tool_response"`
	}
	if err := json.NewDecoder(r).Decode(&event); err != nil || event.TranscriptPath == "" {
		return ""
	}
	minTokens := intFromEnv(getenv, "HORAIO_HANDOFF_MIN_TOKENS", defaultHandoffMinTokens)
	if minTokens <= 0 {
		return ""
	}
	tokens, err := ContextTokens(event.TranscriptPath)
	if err != nil || tokens < minTokens {
		return ""
	}

	state := filepath.Join(stateDir, "handoff-"+stampName(event.SessionID))
	if raw, err := os.ReadFile(state); err == nil {
		warnedAt, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
		if warnedAt > 0 && tokens < warnedAt+intFromEnv(getenv, "HORAIO_HANDOFF_STEP_TOKENS", defaultHandoffStepTokens) {
			return ""
		}
	}
	if err := os.MkdirAll(stateDir, 0o700); err == nil {
		_ = os.WriteFile(state, []byte(strconv.Itoa(tokens)), 0o600)
	}

	out, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]string{
			"hookEventName":     "PostToolUse",
			"additionalContext": handoffMessage(tokens, !memoryOff(event.ToolResponse)),
		},
	})
	return string(out)
}

// handoffMessage solo promete que la conversación siguiente retoma cuando la
// cuenta guarda memoria: en «solo medir» start_work no devuelve nada.
func handoffMessage(tokens int, memory bool) string {
	k := (tokens + 500) / 1000
	if !memory {
		return fmt.Sprintf("Esta conversación lleva unos %dk tokens de contexto. Termina tu "+
			"respuesta con esta línea para el usuario, sin más explicación: "+
			"«Contexto: %dk. Si has terminado con esto, puedes hacer /clear.»", k, k)
	}
	return fmt.Sprintf("Esta conversación lleva unos %dk tokens de contexto. Si de esta "+
		"tarea quedó algo a medias, regístralo con record_open_loop (siguiente "+
		"paso, lo ya probado y descartado, cómo comprobarlo). Termina tu "+
		"respuesta con esta línea para el usuario, sin más explicación: "+
		"«Contexto: %dk. Si has terminado con esto, puedes hacer /clear; "+
		"la siguiente conversación retoma con lo registrado en Horaio.»", k, k)
}

// ContextTokens es la entrada del último mensaje del asistente en el
// transcript de Claude Code (JSONL): lo que ocupa ahora la conversación, no
// la suma del fichero. Los mensajes de subagentes no cuentan.
func ContextTokens(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	last := 0
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadBytes('\n')
		// Solo se decodifican las líneas con uso de tokens: las demás pueden
		// ocupar megas (salidas de herramientas) y no hacen falta.
		if bytes.Contains(line, []byte(`"usage"`)) {
			var entry struct {
				Type        string `json:"type"`
				IsSidechain bool   `json:"isSidechain"`
				Message     struct {
					Usage struct {
						Input         int `json:"input_tokens"`
						CacheRead     int `json:"cache_read_input_tokens"`
						CacheCreation int `json:"cache_creation_input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(line, &entry) == nil && entry.Type == "assistant" && !entry.IsSidechain {
				usage := entry.Message.Usage
				if total := usage.Input + usage.CacheRead + usage.CacheCreation; total > 0 {
					last = total
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return last, nil
		}
		if err != nil {
			return 0, err
		}
	}
}

// memoryOff dice si la respuesta de record_activity trae `agent_memory:
// false`. Se acepta el texto JSON de la herramienta, sus bloques de contenido
// y el resultado MCP con `content`. Sin el campo, hay memoria.
func memoryOff(raw json.RawMessage) bool {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		raw = json.RawMessage(text)
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		for _, block := range blocks {
			if memoryOff(json.RawMessage(block.Text)) {
				return true
			}
		}
		return false
	}
	var result struct {
		AgentMemory *bool           `json:"agent_memory"`
		Content     json.RawMessage `json:"content"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return false
	}
	if result.AgentMemory == nil && len(result.Content) > 0 {
		return memoryOff(result.Content)
	}
	return result.AgentMemory != nil && !*result.AgentMemory
}

func intFromEnv(getenv func(string) string, key string, fallback int) int {
	if value, err := strconv.Atoi(getenv(key)); err == nil {
		return value
	}
	return fallback
}
