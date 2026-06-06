# Prompt canônico — `execute-all-tasks`

Use a skill `execute-all-tasks` para executar integralmente uma spec de forma mandatória, robusta, production-ready, eficiente e econômica, sem desvios, sem fallback mágico e sem falso positivo.

## Gate obrigatório de entrada

Antes de iniciar a orquestração:

- Se eu **não** informar o diretório da spec, responda obrigatoriamente com `needs_input` pedindo `.specs/prd-<slug>/`.
- Se o diretório informado não contiver `prd.md`, `techspec.md` e `tasks.md`, responda `needs_input` e não tente orquestração parcial.

## Escopo de leitura obrigatório

Leia e use como fonte de verdade:

- `AGENTS.md`
- `.agents/skills/execute-all-tasks/SKILL.md`
- `.agents/skills/agent-governance/SKILL.md`
- `.specs/prd-<slug>/prd.md`
- `.specs/prd-<slug>/techspec.md`
- `.specs/prd-<slug>/tasks.md`
- todos os `task-*.md` e ADRs do diretório informado
- `go.mod`
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- o working tree atual

## Regras mandatórias e inegociáveis

1. Use `execute-all-tasks` como orquestrador top-level. Não degrade para execução manual inline do PRD inteiro.
2. Execute o pré-voo completo da skill: hook, profundidade, `ai-spec`, drift, cobertura e integridade da spec.
3. Respeite o DAG de `tasks.md`, o paralelismo permitido e o modelo halt-first da skill.
4. Cada tarefa deve rodar em subagent fresh, com contexto mínimo e estritamente limitado ao seu `task-*.md`.
5. Assuma sempre o working tree atual como fonte da verdade. Se docs antigas divergirem do código real, prevalece o estado atual do workspace e a opção mais segura.
6. Não invente packages, handlers, routers, jobs, consumers, adapters, migrations, interfaces, wiring ou contexto de negócio ausente.
7. Toda implementação Go deve carregar obrigatoriamente `go-implementation`.
8. Em implementação Go, carregue exemplos apenas sob demanda, nunca por reflexo.
9. Em implementação Go, verifique `go.mod` antes de usar APIs ou dependências dependentes de versão.
10. Em implementação Go, parta obrigatoriamente de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nunca de `internal/platform/runtime`.
11. Em implementação Go, mantenha zero comentários adicionados em arquivos Go.
12. Preserve estritamente as fronteiras `infrastructure -> application -> domain`, o padrão obrigatório de módulo em `identity` e `billing`, e os contratos de `internal/platform/httpclient`, outbox e worker.
13. Não marque tarefa como concluída sem evidência física real, `report_path` válido e coerência com `tasks.md`.
14. Não faça reexecução silenciosa, reparo especulativo, placeholder, “quase pronto” nem expansão de escopo.

## Critérios de aceite inegociáveis

Considere a execução correta apenas se:

1. todas as tarefas aplicáveis forem encerradas com status canônico e evidência coerente
2. qualquer bloqueio real gerar `partial`, `failed` ou `needs_input`, nunca “done” aparente
3. toda alteração Go tiver seguido `go-implementation` e exemplos sob demanda
4. nenhum item fora do escopo da spec tiver sido implementado
5. o relatório de orquestração final refletir honestamente o estado das tarefas

## Formato de saída esperado

Responda em PT-BR e retorne apenas:

1. `status_final`: `done`, `partial`, `failed` ou `needs_input`
2. `spec_alvo`
3. `waves_executadas`
4. `tarefas_done`
5. `tarefas_nao_done`
6. `relatorio_orquestracao`
7. `bloqueios_reais`
8. `drifts_registrados`
9. `proximos_passos` apenas se houver dependência externa real
