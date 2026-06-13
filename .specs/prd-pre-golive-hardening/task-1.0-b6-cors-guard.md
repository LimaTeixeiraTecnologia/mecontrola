# Tarefa 1.0: B6 — CORS guard em Config.Validate

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adiciona validação obrigatória em `Config.Validate()` que falha o boot em `Environment="production"` quando `CORS_ALLOWED_ORIGINS` está vazio ou contém wildcard `*`. Eliminação inegociável de configuração permissiva silenciosa.

<requirements>
- RF-18: erro de boot em production quando lista vazia OU contém `*`
- RF-19: `.env.example` documenta formato esperado
- RF-20: 4 testes unitários (production vazio, production `*`, production lista válida, development qualquer)
- RF-32: skill `go-implementation` carregada
- RF-33: `task lint && task test && task vulncheck` verde
- RF-34: zero nova dependência em `go.mod`
- Zero comentário em `.go` produção
</requirements>

## Subtarefas

- [ ] 1.1 Localizar `Config.Validate()` em `configs/config.go` e identificar campo `cfg.HTTP.CORSAllowedOrigins`.
- [ ] 1.2 Adicionar guard em production: vazio → `fmt.Errorf("CORS_ALLOWED_ORIGINS obrigatorio em production")`; contém `*` → `fmt.Errorf("CORS_ALLOWED_ORIGINS=* proibido em production")`.
- [ ] 1.3 Usar `slices.Contains` (Go 1.21+, R7.3) para checar wildcard.
- [ ] 1.4 Atualizar `.env.example` com `CORS_ALLOWED_ORIGINS=https://app.mecontrola.com.br,https://checkout.mecontrola.com.br`.
- [ ] 1.5 Testes unitários cobrindo os 4 cenários da matriz RF-20.

## Detalhes de Implementação

Ver techspec seção "Modelos de Dados > Config" e PRD seção B6. Snippet inicial no plano-fonte seção 8.3.

## Critérios de Sucesso

- `go test ./configs/... -run "Validate.*CORS" -v` PASS para os 4 casos.
- `go vet ./...` PASS.
- `task lint && task test && task vulncheck` PASS.
- `.env.example` atualizado.
- Inspeção: 0 comentários novos em `.go`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários cobrindo os 4 cenários
- [ ] Smoke local: rodar app com `ENVIRONMENT=production CORS_ALLOWED_ORIGINS=` → boot falha com mensagem clara

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `configs/config.go` (modificado)
- `configs/config_test.go` (modificado)
- `.env.example` (modificado)
