# Prompt canônico — `bugfix`

Use a skill `bugfix` para corrigir bugs pela causa raiz de forma mandatória, robusta, production-ready, eficiente e econômica, com testes de regressão obrigatórios, evidência real e sem reparo especulativo.

## Gate obrigatório de entrada

Antes de editar qualquer arquivo:

- Se o bug estiver ligado a uma spec ou tarefa e eu **não** informar o diretório correspondente, responda obrigatoriamente com `needs_input` pedindo `.specs/prd-<slug>/`.
- Se eu não fornecer a lista de bugs no formato canônico `{ id, severity, file, line, reproduction, expected, actual }`, responda `needs_input` pedindo exatamente os campos faltantes.
- Se o contexto incluir arquivo JSON de bugs, valide o formato antes de prosseguir.

## Fonte de verdade

Use como base:

- `AGENTS.md`
- `.agents/skills/bugfix/SKILL.md`
- `.agents/skills/agent-governance/SKILL.md`
- a lista canônica de bugs
- `.specs/prd-<slug>/prd.md`, `techspec.md`, `tasks.md` e `task-*.md` quando aplicável
- o working tree atual

## Regras mandatórias

1. Corrija apenas o escopo de bugs acordado.
2. Diagnostique a causa raiz antes de editar.
3. Não aplique patch superficial quando a causa raiz ainda estiver obscura.
4. Crie um teste de regressão para cada bug corrigido.
5. Em qualquer correção que toque Go, carregue obrigatoriamente `go-implementation`.
6. Em qualquer correção Go, carregue exemplos apenas sob demanda, nunca por cópia cega.
7. Em qualquer correção Go, verifique `go.mod` antes de usar recursos dependentes de versão.
8. Em qualquer correção Go, parta obrigatoriamente de `cmd/server/server.go` e/ou `cmd/worker/worker.go` quando a correção exigir wiring ou bootstrapping; nunca use `internal/platform/runtime` como ponto de partida.
9. Em qualquer correção Go, mantenha zero comentários adicionados em arquivos Go.
10. Preserve as fronteiras arquiteturais do repositório e não invente handlers, adapters, jobs, consumers, migrations ou interfaces ausentes.
11. Se houver bloqueio externo real, marque `blocked` em vez de mascarar com workaround especulativo.
12. Não invoque review novamente quando já estiver dentro de um ciclo review -> bugfix se isso violar o limite de profundidade.

## Evidência obrigatória

1. Registre para cada bug: causa raiz, arquivos alterados, teste de regressão e estado final.
2. Registre a origem do bug (`RF`, task, finding de review ou issue).
3. Gere o relatório no path canônico da skill.
4. Use somente estados finais canônicos: `fixed`, `blocked`, `skipped` ou `failed`.

## Critérios de aceite inegociáveis

Considere o trabalho concluído apenas se:

1. cada bug elegível tiver diagnóstico de causa raiz
2. cada bug corrigido tiver teste de regressão correspondente
3. toda correção Go tiver seguido `go-implementation` e exemplos sob demanda
4. o relatório final estiver salvo no path correto e consistente com o escopo tratado
5. nenhum bug bloqueado ou falho for reportado como resolvido

## Formato de saída esperado

Responda em PT-BR e retorne apenas:

1. `status_final`: `done`, `blocked`, `needs_input` ou `failed`
2. `spec_alvo` quando aplicável
3. `bugs_no_escopo`
4. `bugs_corrigidos`
5. `bugs_pendentes`
6. `testes_de_regressao`
7. `relatorio_bugfix`
8. `bloqueios_reais`
9. `drifts_registrados`
