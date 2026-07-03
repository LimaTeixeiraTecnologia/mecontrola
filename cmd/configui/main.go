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
	AgeKeyFile   string
}

func loadSettings() settings {
	s := settings{
		RepoDir:      cmp.Or(os.Getenv("CONFIG_UI_REPO_DIR"), "."),          //nolint:forbidigo // bootstrap de cmd entrypoint
		ListenAddr:   cmp.Or(os.Getenv("CONFIG_UI_ADDR"), "127.0.0.1:8080"), //nolint:forbidigo // bootstrap de cmd entrypoint
		PasswordHash: os.Getenv("CONFIG_UI_PASSWORD_HASH"),                  //nolint:forbidigo // bootstrap de cmd entrypoint
	}
	s.AgeKeyFile = resolveAgeKeyFile(s.RepoDir)
	return s
}

func resolveAgeKeyFile(repoDir string) string {
	if p := os.Getenv("SOPS_AGE_KEY_FILE"); p != "" { //nolint:forbidigo // bootstrap de cmd entrypoint
		return p
	}

	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(repoDir, "key.txt"),
		filepath.Join(repoDir, ".sops", "age", "key.txt"),
	}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, ".config", "sops", "age", "key.txt"))
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

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
		Config        []viewRow
		Secrets       []viewRow
		ConfigError   string
		SecretsError  string
		SaveMessage   string
		SaveError     string
		AgeKeyMissing bool
	}{
		Config:        toViewRows(configEntries),
		Secrets:       toViewRows(secretsEntries),
		ConfigError:   errMsg(configErr),
		AgeKeyMissing: !ageKeyConfigured() && cfg.AgeKeyFile == "",
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

func ageKeyConfigured() bool {
	for _, key := range []string{"SOPS_AGE_KEY", "SOPS_AGE_KEY_FILE", "SOPS_AGE_KEY_CMD", "SOPS_AGE_SSH_PRIVATE_KEY_FILE", "SOPS_AGE_SSH_PRIVATE_KEY_CMD"} {
		if os.Getenv(key) != "" { //nolint:forbidigo // bootstrap de cmd entrypoint
			return true
		}
	}
	return false
}

func runSOPS(args ...string) (string, error) {
	cmd := exec.Command("sops", args...)
	cmd.Dir = cfg.RepoDir
	cmd.Env = os.Environ()

	if !ageKeyConfigured() && cfg.AgeKeyFile != "" {
		cmd.Env = append(cmd.Env, "SOPS_AGE_KEY_FILE="+cfg.AgeKeyFile)
	}

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
<html lang="pt-BR" class="dark">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MeControla — Config UI</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <script>
    tailwind.config = { darkMode: 'class' };
  </script>
  <style>
    [data-row][hidden] { display: none !important; }
  </style>
</head>
<body class="bg-slate-950 text-slate-200 min-h-screen">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <header class="mb-8 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold text-white tracking-tight">MeControla</h1>
        <p class="text-slate-400 text-sm mt-1">Gestão de variáveis de ambiente e secrets de produção</p>
      </div>
      <div class="flex items-center gap-3">
        <span class="inline-flex items-center rounded-full bg-emerald-500/10 px-3 py-1 text-xs font-medium text-emerald-400 ring-1 ring-inset ring-emerald-500/20">Produção</span>
        <span class="inline-flex items-center rounded-full bg-slate-800 px-3 py-1 text-xs font-medium text-slate-300 ring-1 ring-inset ring-slate-700">SOPS + age</span>
      </div>
    </header>

    {{if .SaveMessage}}<div class="mb-6 rounded-lg bg-emerald-500/10 border border-emerald-500/20 p-4 text-emerald-300 text-sm">{{.SaveMessage}}</div>{{end}}
    {{if .SaveError}}<div class="mb-6 rounded-lg bg-rose-500/10 border border-rose-500/20 p-4 text-rose-300 text-sm">{{.SaveError}}</div>{{end}}
    {{if .AgeKeyMissing}}<div class="mb-6 rounded-lg bg-amber-500/10 border border-amber-500/20 p-4 text-amber-300 text-sm">Chave age não encontrada. Coloque <code>key.txt</code> na raiz do repositório ou defina <code>SOPS_AGE_KEY_FILE</code>.</div>{{end}}

    <div class="flex flex-col gap-6">
      <!-- Config -->
      <section class="rounded-xl bg-slate-900 border border-slate-800 shadow-sm">
        <div class="p-5 border-b border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <h2 class="text-lg font-semibold text-white">Configuração</h2>
            <p class="text-xs text-slate-400 mt-0.5">deployment/config/prod.env</p>
          </div>
          <input type="text" data-filter="config" placeholder="Buscar chave..." class="w-full sm:w-56 rounded-md bg-slate-950 border border-slate-700 px-3 py-1.5 text-sm text-white placeholder-slate-500 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500">
        </div>
        {{if .ConfigError}}<div class="mx-5 mt-5 rounded-lg bg-rose-500/10 border border-rose-500/20 p-3 text-rose-300 text-sm">{{.ConfigError}}</div>{{end}}
        <form method="post" action="/save-config" class="p-5">
          <div class="overflow-x-auto rounded-lg border border-slate-800">
            <table class="w-full text-sm">
              <thead class="bg-slate-950 text-slate-400 text-left">
                <tr>
                  <th class="px-4 py-2.5 font-medium">Chave</th>
                  <th class="px-4 py-2.5 font-medium w-full">Valor</th>
                  <th class="px-4 py-2.5 font-medium text-center">Remover</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-800">
                {{range .Config}}
                  {{if .IsComment}}
                    <tr><td colspan="3" class="px-4 py-2 text-slate-500 italic bg-slate-950/50">{{.Raw}}</td></tr>
                  {{else if .IsBlank}}
                    <tr><td colspan="3" class="h-2 bg-slate-950/30"></td></tr>
                  {{else}}
                    <tr data-row data-key="{{.Key}}" class="hover:bg-slate-800/50 transition-colors">
                      <td class="px-4 py-2 align-top"><code class="text-indigo-300 text-xs">{{.Key}}</code></td>
                      <td class="px-4 py-2 align-top">
                        <input type="text" name="val_{{.Key}}" value="{{.Value}}" class="w-full rounded-md bg-slate-950 border border-slate-700 px-2.5 py-1.5 text-xs font-mono text-white focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500">
                      </td>
                      <td class="px-4 py-2 align-middle text-center">
                        <input type="checkbox" name="del_{{.Key}}" value="1" class="h-4 w-4 rounded border-slate-700 bg-slate-950 text-rose-500 focus:ring-rose-500 focus:ring-offset-slate-900">
                      </td>
                    </tr>
                  {{end}}
                {{end}}
                <tr class="bg-slate-950/40">
                  <td class="px-4 py-2 align-top">
                    <input type="text" name="new_keys" placeholder="nova_chave" class="w-full rounded-md bg-slate-950 border border-slate-700 px-2.5 py-1.5 text-xs text-white placeholder-slate-500 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500">
                  </td>
                  <td class="px-4 py-2 align-top">
                    <input type="text" name="new_values" placeholder="valor" class="w-full rounded-md bg-slate-950 border border-slate-700 px-2.5 py-1.5 text-xs font-mono text-white placeholder-slate-500 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500">
                  </td>
                  <td class="px-4 py-2 align-middle text-center text-slate-500 text-xs">novo</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="mt-4 flex justify-end">
            <button type="submit" class="inline-flex items-center rounded-md bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 transition-colors">Salvar prod.env</button>
          </div>
        </form>
      </section>

      <!-- Secrets -->
      <section class="rounded-xl bg-slate-900 border border-slate-800 shadow-sm">
        <div class="p-5 border-b border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <h2 class="text-lg font-semibold text-white">Secrets</h2>
            <p class="text-xs text-slate-400 mt-0.5">deployment/config/prod.secrets.env</p>
          </div>
          <div class="flex items-center gap-3">
            <label class="inline-flex items-center gap-2 text-xs text-slate-300 cursor-pointer select-none">
              <input type="checkbox" id="toggle-secrets" checked class="h-4 w-4 rounded border-slate-700 bg-slate-950 text-indigo-500 focus:ring-indigo-500 focus:ring-offset-slate-900">
              Mostrar valores
            </label>
            <input type="text" data-filter="secrets" placeholder="Buscar chave..." class="w-full sm:w-56 rounded-md bg-slate-950 border border-slate-700 px-3 py-1.5 text-sm text-white placeholder-slate-500 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500">
          </div>
        </div>
        {{if .SecretsError}}
        <div class="mx-5 mt-5 rounded-lg bg-rose-500/10 border border-rose-500/20 p-3 text-rose-300 text-sm">{{.SecretsError}}</div>
        <div class="mx-5 mt-2 text-xs text-slate-500">Dica: coloque <code>key.txt</code> na raiz do repositório ou defina <code>SOPS_AGE_KEY_FILE</code>.</div>
        {{end}}
        <form method="post" action="/save-secrets" class="p-5">
          <div class="overflow-x-auto rounded-lg border border-slate-800">
            <table class="w-full text-sm">
              <thead class="bg-slate-950 text-slate-400 text-left">
                <tr>
                  <th class="px-4 py-2.5 font-medium">Chave</th>
                  <th class="px-4 py-2.5 font-medium w-full">Valor</th>
                  <th class="px-4 py-2.5 font-medium text-center">Remover</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-800">
                {{range .Secrets}}
                  {{if .IsComment}}
                    <tr><td colspan="3" class="px-4 py-2 text-slate-500 italic bg-slate-950/50">{{.Raw}}</td></tr>
                  {{else if .IsBlank}}
                    <tr><td colspan="3" class="h-2 bg-slate-950/30"></td></tr>
                  {{else}}
                    <tr data-row data-key="{{.Key}}" class="hover:bg-slate-800/50 transition-colors">
                      <td class="px-4 py-2 align-top"><code class="text-amber-300 text-xs">{{.Key}}</code></td>
                      <td class="px-4 py-2 align-top">
                        <div class="relative">
                          <input type="text" name="val_{{.Key}}" value="{{.Value}}" data-secret class="w-full rounded-md bg-slate-950 border border-slate-700 px-2.5 py-1.5 text-xs font-mono text-white focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 pr-8">
                        </div>
                      </td>
                      <td class="px-4 py-2 align-middle text-center">
                        <input type="checkbox" name="del_{{.Key}}" value="1" class="h-4 w-4 rounded border-slate-700 bg-slate-950 text-rose-500 focus:ring-rose-500 focus:ring-offset-slate-900">
                      </td>
                    </tr>
                  {{end}}
                {{end}}
                <tr class="bg-slate-950/40">
                  <td class="px-4 py-2 align-top">
                    <input type="text" name="new_keys" placeholder="nova_chave" class="w-full rounded-md bg-slate-950 border border-slate-700 px-2.5 py-1.5 text-xs text-white placeholder-slate-500 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500">
                  </td>
                  <td class="px-4 py-2 align-top">
                    <input type="text" name="new_values" placeholder="valor" class="w-full rounded-md bg-slate-950 border border-slate-700 px-2.5 py-1.5 text-xs font-mono text-white placeholder-slate-500 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500">
                  </td>
                  <td class="px-4 py-2 align-middle text-center text-slate-500 text-xs">novo</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="mt-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <p class="text-xs text-slate-500">Ao salvar, o arquivo será re-criptografado com SOPS + age usando as regras de <code>.sops.yaml</code>.</p>
            <button type="submit" class="inline-flex items-center rounded-md bg-amber-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-amber-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-amber-600 transition-colors">Salvar e criptografar secrets</button>
          </div>
        </form>
      </section>
    </div>

    <footer class="mt-8 text-center text-xs text-slate-600">
      Não exponha esta interface em rede pública. Use apenas em <code>127.0.0.1</code>.
    </footer>
  </div>

  <script>
    function filterRows(input) {
      const scope = input.closest('section');
      const term = input.value.trim().toLowerCase();
      scope.querySelectorAll('[data-row]').forEach(row => {
        const key = (row.dataset.key || '').toLowerCase();
        row.hidden = term && !key.includes(term);
      });
    }

    document.querySelectorAll('[data-filter]').forEach(input => {
      input.addEventListener('input', () => filterRows(input));
    });

    const toggle = document.getElementById('toggle-secrets');
    if (toggle) {
      toggle.addEventListener('change', () => {
        document.querySelectorAll('[data-secret]').forEach(input => {
          input.type = toggle.checked ? 'text' : 'password';
        });
      });
    }
  </script>
</body>
</html>`
