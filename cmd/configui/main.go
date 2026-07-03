package main

import (
	"cmp"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	configFile  = "deployment/config/prod.env"
	secretsFile = "deployment/config/prod.secrets.env"
)

var (
	cfg              settings
	staticPassword   string
	tmplContentCache string
)

type settings struct {
	RepoDir      string
	ListenAddr   string
	PasswordHash string
}

func loadSettings() settings {
	return settings{
		RepoDir:      cmp.Or(os.Getenv("CONFIG_UI_REPO_DIR"), "."),          //nolint:forbidigo // bootstrap de cmd entrypoint
		ListenAddr:   cmp.Or(os.Getenv("CONFIG_UI_ADDR"), "127.0.0.1:8080"), //nolint:forbidigo // bootstrap de cmd entrypoint
		PasswordHash: os.Getenv("CONFIG_UI_PASSWORD_HASH"),                  //nolint:forbidigo // bootstrap de cmd entrypoint
	}
}

// envLine é uma discriminated union (sealed interface) representando uma linha
// de um arquivo .env: comentário, linha em branco ou variável. DMMF Princípio 3.
type envLine interface {
	isEnvLine()
}

type commentLine struct{ raw string }

func (commentLine) isEnvLine() {}

type blankLine struct{}

func (blankLine) isEnvLine() {}

type variable struct {
	key   string
	value string
}

func (variable) isEnvLine() {}

// newVariable é o smart constructor para variáveis de ambiente.
// Rejeita chave vazia, garantindo estado ilegal irrepresentável.
func newVariable(key, value string) (variable, error) {
	k := strings.TrimSpace(key)
	if k == "" {
		return variable{}, errors.New("chave de variável não pode ser vazia")
	}
	return variable{key: k, value: strings.TrimSpace(value)}, nil
}

func main() {
	cfg = loadSettings()

	hashPassword := flag.Bool("hash-password", false, "gerar hash bcrypt de uma senha e sair")
	flag.Parse()

	if *hashPassword {
		generatePasswordHash()
		return
	}

	if cfg.PasswordHash == "" {
		setupRandomPassword()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", withAuth(handleIndex))
	mux.HandleFunc("POST /save-config", withAuth(handleSaveConfig))
	mux.HandleFunc("POST /save-secrets", withAuth(handleSaveSecrets))

	slog.Info("configui iniciado", "addr", cfg.ListenAddr, "repo", cfg.RepoDir)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		slog.Error("servidor encerrou", "error", err)
		os.Exit(1)
	}
}

func generatePasswordHash() {
	fmt.Print("Senha: ")
	var pwd string
	if _, err := fmt.Scanln(&pwd); err != nil {
		fmt.Fprintln(os.Stderr, "erro ao ler senha:", err)
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao gerar hash:", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}

func setupRandomPassword() {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		fmt.Fprintln(os.Stderr, "CONFIG_UI_PASSWORD_HASH não definido e não foi possível gerar senha temporária")
		os.Exit(1)
	}
	staticPassword = hex.EncodeToString(b)
	hash, err := bcrypt.GenerateFromPassword([]byte(staticPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao gerar hash temporário:", err)
		os.Exit(1)
	}
	cfg.PasswordHash = string(hash)
	fmt.Fprintf(os.Stderr, "\n========================================================\n")
	fmt.Fprintf(os.Stderr, "SENHA TEMPORÁRIA DO CONFIGUI: %s\n", staticPassword)
	fmt.Fprintf(os.Stderr, "Defina CONFIG_UI_PASSWORD_HASH para evitar esta mensagem.\n")
	fmt.Fprintf(os.Stderr, "========================================================\n\n")
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || !authOK(user, pass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="configui"`)
			http.Error(w, "autenticação necessária", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func authOK(user, pass string) bool {
	if user == "" || pass == "" {
		return false
	}
	if staticPassword != "" {
		return subtle.ConstantTimeCompare([]byte(user), []byte("admin")) == 1 &&
			subtle.ConstantTimeCompare([]byte(pass), []byte(staticPassword)) == 1
	}
	err := bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(pass))
	return err == nil && user == "admin"
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	configEntries, configErr := loadConfig()
	secretsEntries, secretsErr := loadSecrets()

	data := struct {
		Config       []viewRow
		Secrets      []viewRow
		ConfigError  string
		SecretsError string
		SaveMessage  string
		SaveError    string
	}{
		Config:      toViewRows(configEntries),
		Secrets:     toViewRows(secretsEntries),
		ConfigError: errMsg(configErr),
	}

	if secretsErr != nil {
		data.SecretsError = secretsErr.Error()
	}
	if msg := r.URL.Query().Get("msg"); msg != "" {
		data.SaveMessage = msg
	}
	if err := r.URL.Query().Get("err"); err != "" {
		data.SaveError = err
	}

	render(w, data)
}

func handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		redirectWith(w, r, "/", "", "erro ao parsear formulário")
		return
	}
	entries, err := loadConfig()
	if err != nil {
		redirectWith(w, r, "/", "", "erro ao carregar config: "+err.Error())
		return
	}
	entries, err = applyForm(entries, r.Form)
	if err != nil {
		redirectWith(w, r, "/", "", "erro ao aplicar alterações: "+err.Error())
		return
	}
	path := filepath.Join(cfg.RepoDir, configFile)
	if err := writeEntries(path, entries); err != nil {
		redirectWith(w, r, "/", "", "erro ao salvar config: "+err.Error())
		return
	}
	redirectWith(w, r, "/", "Config salva com sucesso", "")
}

func handleSaveSecrets(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		redirectWith(w, r, "/", "", "erro ao parsear formulário")
		return
	}
	entries, err := loadSecrets()
	if err != nil {
		redirectWith(w, r, "/", "", "erro ao carregar secrets: "+err.Error())
		return
	}
	entries, err = applyForm(entries, r.Form)
	if err != nil {
		redirectWith(w, r, "/", "", "erro ao aplicar alterações: "+err.Error())
		return
	}
	path := filepath.Join(cfg.RepoDir, secretsFile)
	if err := writeSecrets(path, entries); err != nil {
		redirectWith(w, r, "/", "", "erro ao salvar secrets: "+err.Error())
		return
	}
	redirectWith(w, r, "/", "Secrets criptografados e salvos", "")
}

func redirectWith(w http.ResponseWriter, r *http.Request, path, msg, err string) {
	q := r.URL.Query()
	q.Set("msg", msg)
	q.Set("err", err)
	http.Redirect(w, r, path+"?"+q.Encode(), http.StatusSeeOther)
}

func loadConfig() ([]envLine, error) {
	path := filepath.Join(cfg.RepoDir, configFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseEnv(string(data))
}

func loadSecrets() ([]envLine, error) {
	path := filepath.Join(cfg.RepoDir, secretsFile)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out, err := runSOPS("--decrypt", path)
	if err != nil {
		return nil, fmt.Errorf("sops decrypt: %w", err)
	}
	return parseEnv(out)
}

func runSOPS(args ...string) (string, error) {
	cmd := exec.Command("sops", args...)
	cmd.Dir = cfg.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func writeEntries(path string, entries []envLine) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(serialize(entries)), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeSecrets(path string, entries []envLine) error {
	backup := path + ".bak"
	if err := copyFile(path, backup); err != nil {
		return fmt.Errorf("backup do arquivo criptografado falhou: %w", err)
	}
	defer func() { _ = os.Remove(backup) }()

	if err := os.WriteFile(path, []byte(serialize(entries)), 0600); err != nil {
		_ = os.Rename(backup, path)
		return fmt.Errorf("escrita do plaintext falhou: %w", err)
	}

	if _, err := runSOPS("--encrypt", "--in-place", path); err != nil {
		_ = os.Rename(backup, path)
		return fmt.Errorf("sops encrypt falhou: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

func applyForm(entries []envLine, form map[string][]string) ([]envLine, error) {
	var result []envLine
	seen := make(map[string]bool)

	for _, e := range entries {
		switch l := e.(type) {
		case commentLine, blankLine:
			result = append(result, e)
		case variable:
			if form["del_"+l.key] != nil {
				continue
			}
			v := l.value
			if values := form["val_"+l.key]; len(values) > 0 {
				v = strings.TrimSpace(values[0])
			}
			result = append(result, variable{key: l.key, value: v})
			seen[l.key] = true
		}
	}

	newKeys := form["new_keys"]
	newVals := form["new_values"]
	for i, k := range newKeys {
		k = strings.TrimSpace(k)
		if k == "" || seen[k] {
			continue
		}
		v := ""
		if i < len(newVals) {
			v = strings.TrimSpace(newVals[i])
		}
		nv, err := newVariable(k, v)
		if err != nil {
			return nil, err
		}
		result = append(result, nv)
		seen[k] = true
	}

	return result, nil
}

func parseEnv(content string) ([]envLine, error) {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var entries []envLine
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			entries = append(entries, blankLine{})
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			entries = append(entries, commentLine{raw: line})
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			entries = append(entries, commentLine{raw: line})
			continue
		}
		v, err := newVariable(key, value)
		if err != nil {
			return nil, fmt.Errorf("linha %q: %w", line, err)
		}
		entries = append(entries, v)
	}
	return entries, nil
}

func serialize(entries []envLine) string {
	var b strings.Builder
	for _, e := range entries {
		switch l := e.(type) {
		case commentLine:
			b.WriteString(l.raw)
		case blankLine:
			// nada a serializar
		case variable:
			b.WriteString(l.key)
			b.WriteString("=")
			b.WriteString(l.value)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type viewRow struct {
	Key       string
	Value     string
	IsComment bool
	IsBlank   bool
	Raw       string
}

func toViewRows(entries []envLine) []viewRow {
	rows := make([]viewRow, 0, len(entries))
	for _, e := range entries {
		switch l := e.(type) {
		case commentLine:
			rows = append(rows, viewRow{IsComment: true, Raw: l.raw})
		case blankLine:
			rows = append(rows, viewRow{IsBlank: true})
		case variable:
			rows = append(rows, viewRow{Key: l.key, Value: l.value})
		}
	}
	return rows
}

func render(w http.ResponseWriter, data any) {
	tmpl, err := template.New("configui").Parse(pageTemplate())
	if err != nil {
		http.Error(w, "erro no template: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}

func pageTemplate() string {
	if tmplContentCache != "" {
		return tmplContentCache
	}
	if path := os.Getenv("CONFIG_UI_TEMPLATE"); path != "" { //nolint:forbidigo // bootstrap de cmd entrypoint
		if data, err := os.ReadFile(path); err == nil {
			tmplContentCache = string(data)
			return tmplContentCache
		}
	}
	tmplContentCache = defaultTemplate
	return tmplContentCache
}

const defaultTemplate = `<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MeControla Config UI</title>
  <style>
    :root { color-scheme: light dark; }
    body { font-family: system-ui, -apple-system, sans-serif; margin: 2rem; line-height: 1.5; }
    h1, h2 { margin-top: 2rem; }
    table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
    th, td { text-align: left; padding: .5rem; border-bottom: 1px solid #ccc; }
    input[type="text"] { width: 100%; box-sizing: border-box; font-family: monospace; }
    button { margin-top: 1rem; padding: .6rem 1.2rem; cursor: pointer; }
    .msg { padding: .8rem; border-radius: .4rem; margin-top: 1rem; }
    .msg.ok { background: #d4edda; color: #155724; }
    .msg.err { background: #f8d7da; color: #721c24; }
    .hint { color: #666; font-size: .9rem; margin-top: .5rem; }
    .add-row td { padding-top: 1rem; }
  </style>
</head>
<body>
  <h1>MeControla — Configuração e Secrets</h1>
  {{if .SaveMessage}}<div class="msg ok">{{.SaveMessage}}</div>{{end}}
  {{if .SaveError}}<div class="msg err">{{.SaveError}}</div>{{end}}

  <h2>Configuração (prod.env)</h2>
  {{if .ConfigError}}<div class="msg err">{{.ConfigError}}</div>{{end}}
  <form method="post" action="/save-config">
    <table>
      <thead><tr><th>Chave</th><th>Valor</th><th>Remover</th></tr></thead>
      <tbody>
        {{range .Config}}
          {{if .IsComment}}
            <tr><td colspan="3"><em>{{.Raw}}</em></td></tr>
          {{else if .IsBlank}}
            <tr><td colspan="3">&nbsp;</td></tr>
          {{else}}
            <tr>
              <td>{{.Key}}</td>
              <td><input type="text" name="val_{{.Key}}" value="{{.Value}}"></td>
              <td><input type="checkbox" name="del_{{.Key}}" value="1"></td>
            </tr>
          {{end}}
        {{end}}
        <tr class="add-row"><td><input type="text" name="new_keys" placeholder="nova_chave"></td><td><input type="text" name="new_values" placeholder="valor"></td><td>—</td></tr>
      </tbody>
    </table>
    <button type="submit">Salvar prod.env</button>
  </form>

  <h2>Secrets (prod.secrets.env)</h2>
  {{if .SecretsError}}<div class="msg err">{{.SecretsError}}</div>{{end}}
  <form method="post" action="/save-secrets">
    <table>
      <thead><tr><th>Chave</th><th>Valor</th><th>Remover</th></tr></thead>
      <tbody>
        {{range .Secrets}}
          {{if .IsComment}}
            <tr><td colspan="3"><em>{{.Raw}}</em></td></tr>
          {{else if .IsBlank}}
            <tr><td colspan="3">&nbsp;</td></tr>
          {{else}}
            <tr>
              <td>{{.Key}}</td>
              <td><input type="text" name="val_{{.Key}}" value="{{.Value}}"></td>
              <td><input type="checkbox" name="del_{{.Key}}" value="1"></td>
            </tr>
          {{end}}
        {{end}}
        <tr class="add-row"><td><input type="text" name="new_keys" placeholder="nova_chave"></td><td><input type="text" name="new_values" placeholder="valor"></td><td>—</td></tr>
      </tbody>
    </table>
    <button type="submit">Salvar e criptografar prod.secrets.env</button>
    <p class="hint">Ao salvar, o arquivo será re-criptografado com SOPS + age usando as regras de .sops.yaml.</p>
  </form>
</body>
</html>`
