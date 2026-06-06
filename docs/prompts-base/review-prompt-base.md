# Prompt canônico — `review`

Use a skill `review` para revisar um diff de forma mandatória, robusta, production-ready, eficiente e econômica, focando em correção, segurança, regressões, testes faltantes e violações reais de governança.

## Gate obrigatório de entrada

Antes de revisar:

- Se o diff estiver vinculado a uma spec ou tarefa e eu **não** informar o diretório correspondente, responda obrigatoriamente com `needs_input` pedindo `.specs/prd-<slug>/` e, quando existir, o `task-*.md` associado.
- Se eu não fornecer diff, branch ou contexto mínimo revisável, responda `BLOCKED`.

## Fonte de verdade

Use como base:

- `AGENTS.md`
- `.agents/skills/review/SKILL.md`
- o diff real
- o working tree atual
- `.specs/prd-<slug>/prd.md`, `techspec.md`, `tasks.md` e `task-*.md` quando o review estiver ligado a uma spec

## Regras mandatórias

1. Revise apenas o diff pretendido e os arquivos efetivamente necessários.
2. Se houver task ativa, confronte obrigatoriamente cada critério de sucesso ou aceite contra o diff.
3. Priorize bugs, regressões, segurança, contratos quebrados, testes faltantes e ausência de evidência.
4. Não desperdice o review com estilo cosmético sem impacto real.
5. Em diffs Go, trate como achado material qualquer violação relevante de governança, incluindo:
   - ausência de aderência a `go-implementation` quando a mudança é implementação ou correção Go
   - uso de exemplos não demandados ou copiados cegamente
   - falta de verificação de `go.mod` quando a mudança depende de versão
   - ponto de partida incorreto fora de `cmd/server/server.go` e/ou `cmd/worker/worker.go` quando a mudança exige wiring ou bootstrapping
   - uso de `internal/platform/runtime` como ponto de partida
   - comentários adicionados em arquivos Go
6. Assuma o working tree atual como fonte da verdade; se docs antigas divergirem do código real, revise contra o estado atual e registre o drift.
7. Não implemente correções durante a revisão.

## Critérios de aceite inegociáveis

Considere o review completo apenas se:

1. o escopo revisado estiver claro e suficiente
2. os critérios de aceite da task, quando existirem, tiverem sido confrontados contra o diff
3. o veredito final respeitar estritamente a matriz canônica da skill
4. cada achado trouxer severidade, arquivo, impacto e dica de correção
5. violações relevantes de governança Go tiverem sido apontadas como achados, e não tratadas como detalhe cosmético

## Formato de saída esperado

Responda em PT-BR e retorne apenas:

1. `verdict`: `APPROVED`, `APPROVED_WITH_REMARKS`, `REJECTED` ou `BLOCKED`
2. `spec_alvo` quando aplicável
3. `files_reviewed`
4. `refs_loaded`
5. `findings`
6. `residual_risks`
7. `validations_run`
8. `task_criteria_check`
