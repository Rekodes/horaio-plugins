package beat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// ------------------------------------------------------------ configuración

func TestLoadConfigLeeElFicheroDeHoraioConnect(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".horaio", "config.toml"),
		"# comentario\nserver_url = \"https://app.horaio.com/mcp\"\ndevice_token = \"crn_abc\"\n")

	cfg := LoadConfig(envFrom(nil), home)

	if cfg.ServerURL != "https://app.horaio.com/mcp" || cfg.Token != "crn_abc" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadConfigLasVariablesDeEntornoMandan(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".horaio", "config.toml"),
		"server_url = \"https://fichero/mcp\"\ndevice_token = \"crn_fichero\"\n")

	cfg := LoadConfig(envFrom(map[string]string{
		"HORAIO_SERVER_URL":   "https://entorno/mcp",
		"HORAIO_DEVICE_TOKEN": "crn_entorno",
	}), home)

	if cfg.ServerURL != "https://entorno/mcp" || cfg.Token != "crn_entorno" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadConfigSinFicheroQuedaSinConfigurar(t *testing.T) {
	if LoadConfig(envFrom(nil), t.TempDir()).Complete() {
		t.Fatal("sin fichero ni entorno no debería estar completa")
	}
}

func TestHeartbeatURLSaleDeLaURLDeMCP(t *testing.T) {
	cases := map[string]string{
		"https://app.horaio.com/mcp":  "https://app.horaio.com/v1/heartbeat",
		"https://app.horaio.com/mcp/": "https://app.horaio.com/v1/heartbeat",
		"http://127.0.0.1:8011":       "http://127.0.0.1:8011/v1/heartbeat",
	}
	for in, want := range cases {
		if got := HeartbeatURL(in); got != want {
			t.Errorf("HeartbeatURL(%q) = %q, quiero %q", in, got, want)
		}
	}
}

// ----------------------------------------------------------------- proyecto

func TestSessionDirArgumentoLuegoSesionLuegoActual(t *testing.T) {
	if got := SessionDir("/arg", "/sesion", "/actual"); got != "/arg" {
		t.Errorf("con argumento: %q", got)
	}
	if got := SessionDir("", "/sesion", "/actual"); got != "/sesion" {
		t.Errorf("sin argumento: %q", got)
	}
	if got := SessionDir("", "", "/actual"); got != "/actual" {
		t.Errorf("sin sesión: %q", got)
	}
}

func TestProjectNameEsElRepositorioAunqueSeEsteEnUnaSubcarpeta(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "tienda-online")
	writeFile(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/master\n")
	sub := filepath.Join(repo, "src", "deep")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}

	if got := ProjectName(sub, t.TempDir()); got != "tienda-online" {
		t.Fatalf("got %q", got)
	}
}

func TestProjectNameReconoceWorktreesConGitComoFichero(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "horaio")
	writeFile(t, filepath.Join(repo, ".git"), "gitdir: /otra/parte\n")

	if got := ProjectName(repo, t.TempDir()); got != "horaio" {
		t.Fatalf("got %q", got)
	}
}

func TestProjectNameFueraDeRepoEsLaCarpeta(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "casos-2026")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	if got := ProjectName(dir, t.TempDir()); got != "casos-2026" {
		t.Fatalf("got %q", got)
	}
}

func TestProjectNameDesdeElHomeEsSinEspecificar(t *testing.T) {
	home := t.TempDir()
	if got := ProjectName(home, home); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestSessionProjectAbiertaEnElHomeSigueLaCarpetaActual(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "claude-config")
	writeFile(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")

	if got := SessionProject("", home, repo, home); got != "claude-config" {
		t.Errorf("abierta en el home, ahora en un repo: %q", got)
	}
	if got := SessionProject("", home, home, home); got != "" {
		t.Errorf("abierta y sigue en el home: %q", got)
	}
	if got := SessionProject(home, "", repo, home); got != "" {
		t.Errorf("el argumento explícito manda: %q", got)
	}
}

func TestSessionProjectAbiertaEnUnRepoNoSigueAlCd(t *testing.T) {
	home := t.TempDir()
	launch := filepath.Join(home, "phd-projects")
	writeFile(t, filepath.Join(launch, ".git", "HEAD"), "ref: refs/heads/main\n")
	other := filepath.Join(home, "ocean-app")
	writeFile(t, filepath.Join(other, ".git", "HEAD"), "ref: refs/heads/main\n")

	if got := SessionProject("", launch, other, home); got != "phd-projects" {
		t.Fatalf("got %q", got)
	}
}

// ---------------------------------------------------------------- muestreo

func TestShouldSendUnoPorIntervaloYProyecto(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1_790_000_000, 0)

	if !ShouldSend(dir, "tienda-online", now, time.Minute) {
		t.Fatal("el primero se envía")
	}
	if ShouldSend(dir, "tienda-online", now.Add(59*time.Second), time.Minute) {
		t.Fatal("dentro del intervalo no se envía")
	}
	if !ShouldSend(dir, "api-reservas", now.Add(time.Second), time.Minute) {
		t.Fatal("otro proyecto no queda silenciado")
	}
	if !ShouldSend(dir, "tienda-online", now.Add(time.Minute), time.Minute) {
		t.Fatal("pasado el intervalo se envía")
	}
}

func TestStampNameNoSeSaleDeLaCarpeta(t *testing.T) {
	if got := stampName("../../etc/passwd"); strings.ContainsAny(got, "/\\") {
		t.Fatalf("got %q", got)
	}
	if stampName("") != "_" {
		t.Fatal("sin proyecto, un nombre fijo")
	}
}

// -------------------------------------------------------------- de punta a punta

type recorded struct {
	auth, contentType, userAgent string
	body                         map[string]any
}

func fakeServer(t *testing.T, status int) (*httptest.Server, *[]recorded) {
	t.Helper()
	var calls []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/heartbeat" || r.Method != http.MethodPost {
			t.Errorf("petición inesperada: %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("cuerpo no es JSON: %s", raw)
		}
		calls = append(calls, recorded{
			r.Header.Get("Authorization"), r.Header.Get("Content-Type"), r.Header.Get("User-Agent"), body,
		})
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func testEnv(t *testing.T, serverURL, cwd string) Env {
	t.Helper()
	return Env{
		Getenv: envFrom(map[string]string{
			"HORAIO_SERVER_URL":   serverURL + "/mcp",
			"HORAIO_DEVICE_TOKEN": "crn_prueba",
		}),
		Cwd:      cwd,
		Home:     t.TempDir(),
		StampDir: t.TempDir(),
		Now:      time.Unix(1_790_000_000, 0),
		Client:   &http.Client{Timeout: time.Second},
		Version:  "1.2.3",
	}
}

func TestRunMandaSoloElNombreDelProyecto(t *testing.T) {
	srv, calls := fakeServer(t, http.StatusAccepted)
	repo := filepath.Join(t.TempDir(), "horaio")
	writeFile(t, filepath.Join(repo, ".git", "HEAD"), "x")

	if err := Run(context.Background(), testEnv(t, srv.URL, repo)); err != nil {
		t.Fatal(err)
	}

	if len(*calls) != 1 {
		t.Fatalf("llamadas = %d", len(*calls))
	}
	got := (*calls)[0]
	if len(got.body) != 1 || got.body["project_hint"] != "horaio" {
		t.Fatalf("el contrato es un solo campo; body = %v", got.body)
	}
	if got.auth != "Bearer crn_prueba" || got.contentType != "application/json" {
		t.Fatalf("cabeceras = %+v", got)
	}
	if got.userAgent != "horaio-beat/1.2.3" {
		t.Fatalf("user-agent = %q", got.userAgent)
	}
}

func TestRunUnCdDentroDeLaSesionNoCambiaElProyecto(t *testing.T) {
	srv, calls := fakeServer(t, http.StatusAccepted)
	workspace := filepath.Join(t.TempDir(), "mis-proyectos")
	repo := filepath.Join(workspace, "tienda-online")
	writeFile(t, filepath.Join(repo, ".git", "HEAD"), "x")
	env := testEnv(t, srv.URL, repo)
	env.Getenv = envFrom(map[string]string{
		"HORAIO_SERVER_URL":   srv.URL + "/mcp",
		"HORAIO_DEVICE_TOKEN": "crn_prueba",
		"CLAUDE_PROJECT_DIR":  workspace,
	})

	if err := Run(context.Background(), env); err != nil {
		t.Fatal(err)
	}

	if got := (*calls)[0].body["project_hint"]; got != "mis-proyectos" {
		t.Fatalf("la sesión se abrió en mis-proyectos; got %v", got)
	}
}

func TestRunRespetaElMuestreo(t *testing.T) {
	srv, calls := fakeServer(t, http.StatusAccepted)
	env := testEnv(t, srv.URL, t.TempDir())

	_ = Run(context.Background(), env)
	env.Now = env.Now.Add(10 * time.Second)
	_ = Run(context.Background(), env)

	if len(*calls) != 1 {
		t.Fatalf("llamadas = %d, quiero 1", len(*calls))
	}
}

func TestRunSinConfiguracionNoLlamaANadie(t *testing.T) {
	srv, calls := fakeServer(t, http.StatusAccepted)
	env := testEnv(t, srv.URL, t.TempDir())
	env.Getenv = envFrom(nil)

	if err := Run(context.Background(), env); err != ErrNotConfigured {
		t.Fatalf("err = %v", err)
	}
	if len(*calls) != 0 {
		t.Fatal("no debería haber llamado")
	}
}

func TestRunDevuelveErrorSiElServidorRechaza(t *testing.T) {
	srv, _ := fakeServer(t, http.StatusUnauthorized)

	if err := Run(context.Background(), testEnv(t, srv.URL, t.TempDir())); err == nil {
		t.Fatal("un 401 tiene que llegar como error, para HORAIO_BEAT_DEBUG")
	}
}

// ---------------------------------------------------------------- connect

func TestConnectGuardaElTokenSoloParaElUsuario(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".horaio", "config.toml")

	if err := Connect(path, "", "crn_abc123"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos = %v", info.Mode().Perm())
	}
	cfg := LoadConfig(envFrom(map[string]string{"HORAIO_CONFIG": path}), t.TempDir())
	if cfg.ServerURL != DefaultServerURL || cfg.Token != "crn_abc123" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestConnectRechazaLoQueNoEsUnToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, token := range []string{"", "abc", "crn_\"inyectado", "crn_a\nserver_url = \"x\""} {
		if err := Connect(path, "", token); err == nil {
			t.Errorf("aceptó %q", token)
		}
	}
}

func TestMCPHeadersDaLaCabeceraConElToken(t *testing.T) {
	out, err := MCPHeaders(Config{ServerURL: DefaultServerURL, Token: "crn_abc"})
	if err != nil {
		t.Fatal(err)
	}
	var headers map[string]string
	if err := json.Unmarshal(out, &headers); err != nil {
		t.Fatal(err)
	}
	if len(headers) != 1 || headers["Authorization"] != "Bearer crn_abc" {
		t.Fatalf("headers = %v", headers)
	}
}

func TestMCPHeadersSinTokenEsUnObjetoVacio(t *testing.T) {
	out, _ := MCPHeaders(Config{})
	if string(out) != "{}" {
		t.Fatalf("out = %s", out)
	}
}

func TestStatusNoEnsenaElTokenEntero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("status solo debe mirar /health, no %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	cfg := Config{ServerURL: srv.URL + "/mcp", Token: "crn_secreto_largo_123"}

	lines := strings.Join(Status(context.Background(), srv.Client(), cfg, "/x"), "\n")

	if strings.Contains(lines, "secreto_largo") {
		t.Fatalf("enseña el token: %s", lines)
	}
	if !strings.Contains(lines, "servidor accesible") {
		t.Fatalf("lines = %s", lines)
	}
}

func TestSessionNoticeSoloHablaSiFaltaConfigurar(t *testing.T) {
	if SessionNotice(Config{ServerURL: DefaultServerURL, Token: "crn_x"}) != "" {
		t.Fatal("configurado, no debe decir nada")
	}
	if !strings.Contains(SessionNotice(Config{}), "horaio-beat connect") {
		t.Fatal("sin configurar, tiene que decir cómo arreglarlo")
	}
}

// --------------------------------------------------------------- antigravity

func TestWorkspaceDirSaleDelJSONDelHook(t *testing.T) {
	payload := `{"toolCall":{"name":"run_command","args":{"CommandLine":"npm test"}},` +
		`"stepIdx":19,"conversationId":"ec33","workspacePaths":["/Users/ana/tienda","/Users/ana/otra"],"modelName":"auto"}`

	if got := WorkspaceDir(strings.NewReader(payload)); got != "/Users/ana/tienda" {
		t.Fatalf("WorkspaceDir = %q", got)
	}
}

func TestWorkspaceDirAceptaURIsDeFichero(t *testing.T) {
	payload := `{"workspacePaths":["file:///Users/ana/mi%20tienda"]}`

	if got := WorkspaceDir(strings.NewReader(payload)); got != "/Users/ana/mi tienda" {
		t.Fatalf("WorkspaceDir = %q", got)
	}
}

func TestWorkspaceDirSinCarpetaEsVacio(t *testing.T) {
	for _, payload := range []string{`{"workspacePaths":[]}`, `{"stepIdx":3}`, `no es json`, ``} {
		if got := WorkspaceDir(strings.NewReader(payload)); got != "" {
			t.Fatalf("WorkspaceDir(%q) = %q", payload, got)
		}
	}
}

// ------------------------------------------------------------------ handoff

func transcript(t *testing.T, contexts ...int) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(`{"type":"user","message":{"content":"hola"}}` + "\n")
	for _, tokens := range contexts {
		fmt.Fprintf(&b, `{"type":"assistant","message":{"usage":{"input_tokens":10,"cache_read_input_tokens":%d,"cache_creation_input_tokens":0}}}`+"\n", tokens-10)
	}
	// Un subagente con más contexto no es el de esta conversación.
	b.WriteString(`{"type":"assistant","isSidechain":true,"message":{"usage":{"input_tokens":900000}}}` + "\n")
	path := filepath.Join(t.TempDir(), "sesion.jsonl")
	writeFile(t, path, b.String())
	return path
}

func hookEvent(path, toolResponse string) io.Reader {
	return strings.NewReader(`{"session_id":"s-1","transcript_path":"` + path + `","tool_response":` + toolResponse + `}`)
}

func handoffContext(t *testing.T, out string) string {
	t.Helper()
	var parsed struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("salida no es JSON: %q", out)
	}
	if parsed.HookSpecificOutput.HookEventName != "PostToolUse" {
		t.Fatalf("hookEventName = %q", parsed.HookSpecificOutput.HookEventName)
	}
	return parsed.HookSpecificOutput.AdditionalContext
}

func TestContextTokensEsElUltimoMensajeDelAsistente(t *testing.T) {
	got, err := ContextTokens(transcript(t, 120_000, 250_000))
	if err != nil || got != 250_000 {
		t.Fatalf("ContextTokens = %d, %v", got, err)
	}
}

func TestHandoffHookCallaPorDebajoDelUmbral(t *testing.T) {
	out := HandoffHook(hookEvent(transcript(t, 150_000), `"{}"`), envFrom(nil), t.TempDir())
	if out != "" {
		t.Fatalf("no debía avisar: %q", out)
	}
}

func TestHandoffHookAvisaUnaVezPorTramo(t *testing.T) {
	state := t.TempDir()
	env := envFrom(nil)

	first := HandoffHook(hookEvent(transcript(t, 250_000), `"{\"stored\":true}"`), env, state)
	if msg := handoffContext(t, first); !strings.Contains(msg, "250k") || !strings.Contains(msg, "retoma") {
		t.Fatalf("aviso = %q", msg)
	}
	if again := HandoffHook(hookEvent(transcript(t, 300_000), `"{}"`), env, state); again != "" {
		t.Fatalf("no debía repetir antes de otros 100k: %q", again)
	}
	if later := HandoffHook(hookEvent(transcript(t, 360_000), `"{}"`), env, state); later == "" {
		t.Fatal("tenía que volver a avisar pasados 100k más")
	}
}

func TestHandoffHookEnSoloMedirNoPrometeRetomar(t *testing.T) {
	for _, response := range []string{
		`"{\"stored\":true,\"agent_memory\":false}"`,
		`[{"type":"text","text":"{\"stored\":true,\"agent_memory\":false}"}]`,
		`{"content":[{"type":"text","text":"{\"stored\":true,\"agent_memory\":false}"}]}`,
	} {
		msg := handoffContext(t, HandoffHook(hookEvent(transcript(t, 250_000), response), envFrom(nil), t.TempDir()))
		if strings.Contains(msg, "retoma") || strings.Contains(msg, "record_open_loop") {
			t.Fatalf("con la memoria apagada no hay handoff que prometer: %q", msg)
		}
	}
}

func TestHandoffHookSeApagaConUmbralCero(t *testing.T) {
	env := envFrom(map[string]string{"HORAIO_HANDOFF_MIN_TOKENS": "0"})
	if out := HandoffHook(hookEvent(transcript(t, 900_000), `"{}"`), env, t.TempDir()); out != "" {
		t.Fatalf("apagado, no debía avisar: %q", out)
	}
}

func TestHandoffHookSinTranscriptNoDiceNada(t *testing.T) {
	for _, payload := range []string{`no es json`, `{}`, `{"transcript_path":"/no/existe.jsonl"}`} {
		if out := HandoffHook(strings.NewReader(payload), envFrom(nil), t.TempDir()); out != "" {
			t.Fatalf("HandoffHook(%q) = %q", payload, out)
		}
	}
}
