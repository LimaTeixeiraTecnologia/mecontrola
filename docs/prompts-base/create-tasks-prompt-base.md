# Prompt canônico — `create-tasks`

Use a skill `create-tasks` para decompor uma spec em tarefas mandatórias, robustas, production-ready, eficientes e econômicas, sem lacunas de cobertura, sem micro-fragmentação artificial e sem alucinação de skills, dependências ou arquivos.

## Gate obrigatório de entrada

Antes de planejar qualquer tarefa:

- Se eu **não** informar o diretório da spec, responda obrigatoriamente com `needs_input` pedindo `.specs/prd-<slug>/`.
- Se não existirem `prd.md` e `techspec.md` nesse diretório, responda `needs_input` e não monte plano parcial.

## Fonte de verdade

Use como base obrigatória:

- `AGENTS.md`
- `.agents/skills/create-tasks/SKILL.md`
- `.specs/prd-<slug>/prd.md`
- `.specs/prd-<slug>/techspec.md`
- o working tree atual como fonte da verdade

## Regras mandatórias

1. Leia `prd.md` e `techspec.md` por completo antes de propor tarefas.
2. Enumere explicitamente todos os IDs `RF-nn` e `REQ-nn` e mantenha a origem de cada um.
3. Nenhum requisito pode ficar implícito; cada ID deve aparecer em pelo menos uma tarefa.
4. Quebre o trabalho em fatias verificáveis, independentes e objetivamente revisáveis.
5. Prefira a ordem `domain -> interfaces/ports -> use cases -> adapters/repositories -> handlers -> integration`, salvo justificativa explícita da techspec.
6. Produza primeiro o plano de alto nível e aguarde aprovação antes de gerar `tasks.md` e `task-*.md`.
7. Não invente skills inexistentes, dependências fictícias, arquivos órfãos, tarefas sem critério de aceitação ou placeholders sem lastro.
8. Preserve os campos canônicos exigidos pela skill para `Status`, `Dependências`, `Paralelizável` e `Skills`.
9. Em toda tarefa que possa resultar em implementação Go, registre de forma explícita nos critérios ou descrição que a execução posterior:
   - deve carregar obrigatoriamente `go-implementation`
   - deve carregar exemplos apenas sob demanda
   - deve verificar `go.mod` antes de usar recursos da linguagem
   - deve partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`
   - não pode usar `internal/platform/runtime` como ponto de partida
   - não pode adicionar comentários em arquivos Go
10. Não escreva tasks de implementação fora do escopo do PRD ou techspec.

## Artefatos obrigatórios

Após aprovação:

1. gerar `.specs/prd-<slug>/tasks.md`
2. gerar exatamente um `task-*.md` por linha da tabela de `tasks.md`
3. sincronizar os `spec-hash` de `tasks.md`
4. validar drift e cobertura antes de encerrar
5. confirmar sincronia entre a coluna `Skills` e a seção `## Skills Necessárias` de cada task file

## Critérios de aceite inegociáveis

Considere o trabalho concluído apenas se:

1. todos os `RF-nn` e `REQ-nn` estiverem enumerados e cobertos
2. o plano de alto nível tiver sido aprovado antes da escrita final
3. `tasks.md` e todos os `task-*.md` estiverem em sincronia
4. hashes e drift estiverem consistentes
5. nenhuma tarefa misturar preocupações desconexas
6. as tarefas deixarem explícitas as regras mandatórias para futuras execuções Go quando aplicável

## Formato de saída esperado

Responda em PT-BR e retorne apenas:

1. `status_final`: `done` ou `needs_input`
2. `spec_alvo`
3. `arquivos_gerados`
4. `requisitos_cobertos`
5. `tarefas_principais`
6. `dependencias_criticas`
7. `paralelismo_seguro`
8. `drifts_ou_bloqueios`
9. `aprovacao_pendente` quando aplicável
