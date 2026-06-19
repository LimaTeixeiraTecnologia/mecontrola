package openapi

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type specDefinition struct {
	Module       string
	RelativePath string
}

type operation struct {
	Method  string
	Path    string
	Summary string
	Tag     string
}

type loadedSpec struct {
	Module      string
	Title       string
	Version     string
	Description string
	RawPath     string
	RawYAML     []byte
	Operations  []operation
}

type catalogItem struct {
	Module   string `json:"module"`
	Title    string `json:"title"`
	Version  string `json:"version"`
	RawURL   string `json:"raw_url"`
	DocsURL  string `json:"docs_url"`
	SpecPath string `json:"spec_path"`
}

type docsRouter struct {
	catalog []catalogItem
	specs   map[string]loadedSpec
	tmpl    *template.Template
}

var specDefinitions = []specDefinition{
	{Module: "identity", RelativePath: "internal/identity/openapi.yaml"},
	{Module: "billing", RelativePath: "internal/billing/openapi.yaml"},
	{Module: "categories", RelativePath: "internal/categories/openapi.yaml"},
	{Module: "onboarding", RelativePath: "internal/onboarding/openapi.yaml"},
	{Module: "card", RelativePath: "internal/card/openapi.yaml"},
	{Module: "budgets", RelativePath: "internal/budgets/openapi.yaml"},
	{Module: "transactions", RelativePath: "internal/transactions/openapi.yaml"},
}

const docsTemplate = `<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MeControla OpenAPI Docs</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f3ea;
      --paper: #fffdf8;
      --ink: #1f1a17;
      --muted: #6e6258;
      --line: #dbcdbd;
      --accent: #116466;
      --accent-soft: #e3f1ec;
      --chip: #efe6d8;
      --get: #1d7a55;
      --post: #005f99;
      --put: #8f4b00;
      --patch: #6d3fb2;
      --delete: #a12e2e;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: radial-gradient(circle at top left, #fff7e5, var(--bg) 40%);
      color: var(--ink);
      font-family: "Iowan Old Style", "Palatino Linotype", serif;
    }
    main {
      max-width: 1200px;
      margin: 0 auto;
      padding: 32px 20px 48px;
    }
    h1, h2, h3 { margin: 0; }
    p { margin: 0; line-height: 1.5; }
    a { color: var(--accent); text-decoration: none; }
    a:hover { text-decoration: underline; }
    .hero {
      display: grid;
      gap: 16px;
      margin-bottom: 24px;
      padding: 24px;
      border: 1px solid var(--line);
      border-radius: 20px;
      background: linear-gradient(135deg, rgba(17, 100, 102, 0.08), rgba(255, 253, 248, 0.92));
    }
    .hero p { color: var(--muted); max-width: 72ch; }
    .meta {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      font-size: 14px;
      color: var(--muted);
    }
    .layout {
      display: grid;
      grid-template-columns: 280px 1fr;
      gap: 20px;
    }
    .panel {
      border: 1px solid var(--line);
      border-radius: 20px;
      background: var(--paper);
      overflow: hidden;
    }
    .panel-header {
      padding: 18px 20px 10px;
      border-bottom: 1px solid var(--line);
    }
    .panel-header p {
      color: var(--muted);
      font-size: 14px;
      margin-top: 6px;
    }
    .nav {
      padding: 8px;
      display: grid;
      gap: 8px;
    }
    .nav a {
      display: block;
      padding: 12px 14px;
      border-radius: 14px;
      border: 1px solid transparent;
      color: var(--ink);
    }
    .nav a.active {
      background: var(--accent-soft);
      border-color: rgba(17, 100, 102, 0.15);
    }
    .nav small {
      display: block;
      margin-top: 4px;
      color: var(--muted);
      font-size: 12px;
    }
    .content {
      padding: 20px;
      display: grid;
      gap: 18px;
    }
    .content p.description {
      color: var(--muted);
      white-space: pre-line;
    }
    .links {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      font-size: 14px;
    }
    .links a {
      padding: 10px 12px;
      border-radius: 999px;
      background: var(--chip);
    }
    .ops {
      display: grid;
      gap: 10px;
    }
    .op {
      display: grid;
      gap: 8px;
      padding: 14px;
      border: 1px solid var(--line);
      border-radius: 16px;
    }
    .op-top {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 10px;
    }
    .method {
      min-width: 74px;
      padding: 4px 10px;
      border-radius: 999px;
      color: #fff;
      text-align: center;
      font: 700 12px/1.3 ui-monospace, "SFMono-Regular", monospace;
      letter-spacing: 0.06em;
    }
    .method.GET { background: var(--get); }
    .method.POST { background: var(--post); }
    .method.PUT { background: var(--put); }
    .method.PATCH { background: var(--patch); }
    .method.DELETE { background: var(--delete); }
    code {
      font-family: ui-monospace, "SFMono-Regular", monospace;
      font-size: 13px;
      word-break: break-word;
    }
    .tag {
      color: var(--muted);
      font-size: 13px;
    }
    @media (max-width: 860px) {
      .layout { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <main>
    <section class="hero">
      <h1>MeControla OpenAPI Docs</h1>
      <p>Catálogo local das specs OpenAPI por bounded context HTTP. Esta superfície existe apenas quando o servidor sobe com <code>ENVIRONMENT=local</code>.</p>
      <div class="meta">
        <span>{{len .Catalog}} módulos documentados</span>
        <a href="/__docs/openapi/index.json">index.json</a>
      </div>
    </section>
    <section class="layout">
      <aside class="panel">
        <div class="panel-header">
          <h2>Módulos</h2>
          <p>Selecione uma spec para inspecionar as operações documentadas e baixar o YAML cru.</p>
        </div>
        <nav class="nav">
          {{range .Catalog}}
          <a href="/__docs?module={{.Module}}" class="{{if eq $.Selected.Module .Module}}active{{end}}">
            <strong>{{.Title}}</strong>
            <small>{{.Module}} · {{.Version}}</small>
          </a>
          {{end}}
        </nav>
      </aside>
      <section class="panel">
        <div class="panel-header">
          <h2>{{.Selected.Title}}</h2>
          <p>{{.Selected.Module}} · versão {{.Selected.Version}}</p>
        </div>
        <div class="content">
          <p class="description">{{.Selected.Description}}</p>
          <div class="links">
            <a href="{{.Selected.RawURL}}">Baixar YAML</a>
            <a href="/__docs/openapi/index.json">Ver catálogo JSON</a>
          </div>
          <div class="ops">
            {{range .Selected.Operations}}
            <article class="op">
              <div class="op-top">
                <span class="method {{.Method}}">{{.Method}}</span>
                <code>{{.Path}}</code>
                <span class="tag">{{.Tag}}</span>
              </div>
              <p>{{.Summary}}</p>
            </article>
            {{end}}
          </div>
        </div>
      </section>
    </section>
  </main>
</body>
</html>`

func NewRouterIfEnabled(cfg *configs.Config) (*docsRouter, error) {
	if cfg == nil || cfg.AppConfig.Environment != "local" {
		return nil, nil
	}
	return NewRouter()
}

func NewRouter() (*docsRouter, error) {
	specs, err := loadSpecs()
	if err != nil {
		return nil, err
	}

	catalog := make([]catalogItem, 0, len(specs))
	for _, def := range specDefinitions {
		spec := specs[def.Module]
		catalog = append(catalog, catalogItem{
			Module:   spec.Module,
			Title:    spec.Title,
			Version:  spec.Version,
			RawURL:   "/__docs/openapi/" + spec.Module + ".yaml",
			DocsURL:  "/__docs?module=" + spec.Module,
			SpecPath: def.RelativePath,
		})
	}

	tmpl, err := template.New("openapi-docs").Parse(docsTemplate)
	if err != nil {
		return nil, fmt.Errorf("openapi: parse html template: %w", err)
	}

	return &docsRouter{
		catalog: catalog,
		specs:   specs,
		tmpl:    tmpl,
	}, nil
}

func loadSpecs() (map[string]loadedSpec, error) {
	repoRoot, err := resolveRepoRoot()
	if err != nil {
		return nil, err
	}

	loader := openapi3.NewLoader()
	specs := make(map[string]loadedSpec, len(specDefinitions))

	for _, def := range specDefinitions {
		absPath := filepath.Join(repoRoot, filepath.FromSlash(def.RelativePath))
		raw, readErr := os.ReadFile(absPath)
		if readErr != nil {
			return nil, fmt.Errorf("openapi: read %s: %w", def.RelativePath, readErr)
		}

		doc, loadErr := loader.LoadFromFile(absPath)
		if loadErr != nil {
			return nil, fmt.Errorf("openapi: load %s: %w", def.RelativePath, loadErr)
		}
		if validateErr := doc.Validate(loader.Context); validateErr != nil {
			return nil, fmt.Errorf("openapi: validate %s: %w", def.RelativePath, validateErr)
		}

		specs[def.Module] = loadedSpec{
			Module:      def.Module,
			Title:       doc.Info.Title,
			Version:     doc.Info.Version,
			Description: strings.TrimSpace(doc.Info.Description),
			RawPath:     def.RelativePath,
			RawYAML:     raw,
			Operations:  collectOperations(doc),
		}
	}

	return specs, nil
}

func resolveRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("openapi: runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")), nil
}

func collectOperations(doc *openapi3.T) []operation {
	type operationRef struct {
		method string
		op     *openapi3.Operation
	}

	paths := make([]string, 0, len(doc.Paths.Map()))
	for path := range doc.Paths.Map() {
		paths = append(paths, path)
	}
	slices.Sort(paths)

	operations := make([]operation, 0, len(paths)*2)
	for _, path := range paths {
		pathItem := doc.Paths.Map()[path]
		refs := []operationRef{
			{method: http.MethodGet, op: pathItem.Get},
			{method: http.MethodPost, op: pathItem.Post},
			{method: http.MethodPut, op: pathItem.Put},
			{method: http.MethodPatch, op: pathItem.Patch},
			{method: http.MethodDelete, op: pathItem.Delete},
		}
		for _, ref := range refs {
			if ref.op == nil {
				continue
			}
			tag := ""
			if len(ref.op.Tags) > 0 {
				tag = ref.op.Tags[0]
			}
			operations = append(operations, operation{
				Method:  ref.method,
				Path:    path,
				Summary: ref.op.Summary,
				Tag:     tag,
			})
		}
	}

	return operations
}

func (rt *docsRouter) Register(r chi.Router) {
	r.Route("/__docs", func(sub chi.Router) {
		sub.Get("/", rt.handleHTML)
		sub.Get("/openapi/index.json", rt.handleIndex)
		sub.Get("/openapi/{module}.yaml", rt.handleRawSpec)
	})
}

func (rt *docsRouter) handleHTML(w http.ResponseWriter, r *http.Request) {
	module := r.URL.Query().Get("module")
	if module == "" && len(rt.catalog) > 0 {
		module = rt.catalog[0].Module
	}

	spec, ok := rt.specs[module]
	if !ok {
		http.NotFound(w, r)
		return
	}

	selected := catalogItem{}
	for _, item := range rt.catalog {
		if item.Module == module {
			selected = item
			break
		}
	}

	data := struct {
		Catalog  []catalogItem
		Selected struct {
			Module      string
			Title       string
			Version     string
			Description string
			RawURL      string
			Operations  []operation
		}
	}{
		Catalog: rt.catalog,
	}
	data.Selected.Module = spec.Module
	data.Selected.Title = spec.Title
	data.Selected.Version = spec.Version
	data.Selected.Description = spec.Description
	data.Selected.RawURL = selected.RawURL
	data.Selected.Operations = spec.Operations

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := rt.tmpl.Execute(w, data); err != nil {
		http.Error(w, "failed to render docs", http.StatusInternalServerError)
	}
}

func (rt *docsRouter) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(rt.catalog)
}

func (rt *docsRouter) handleRawSpec(w http.ResponseWriter, r *http.Request) {
	module := chi.URLParam(r, "module")
	spec, ok := rt.specs[module]
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	if _, err := w.Write(spec.RawYAML); err != nil {
		http.Error(w, "failed to write spec", http.StatusInternalServerError)
	}
}
