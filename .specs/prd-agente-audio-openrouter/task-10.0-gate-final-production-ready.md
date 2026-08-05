# Tarefa 10.0: Gate final production-ready RF-01..RF-46

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Executar revisão final de readiness confrontando cada RF com evidência real de código, testes, golden set, observabilidade, migrations e operação. Esta tarefa não substitui implementação; ela impede falso positivo antes de declarar produção pronta.

<requirements>
- Cobrir RF-01 a RF-46.
- Confirmar uso obrigatório de `mastra`, `domain-modeling-production`, `design-patterns-mandatory` e `go-implementation`.
- Confirmar decisão `nao aplicar padrao` ou reexecutar seletor se alguma task introduzir abstração estrutural nova.
- Confirmar ausência de agente paralelo, workflow kernel de áudio, fallback chain e provider STT fora do OpenRouter.
- Confirmar 0 regressão textual, 0 falso-sucesso mutacional, 0 tool call em `TranscriptionUncertain` e 0 áudio bruto persistido.
</requirements>

## Subtarefas

- [x] 10.1 Gerar matriz RF-01..RF-46 com evidência `path:line`, teste executado ou artefato operacional para cada RF.
- [x] 10.2 Rodar build, vet, race tests e lint nos escopos definidos pela techspec.
- [x] 10.3 Rodar golden set texto/audio e suite real por flag quando credenciais existirem.
- [x] 10.4 Rodar gates heurísticos: sem `init()`, sem `_prefixado`, sem comentários Go proibidos, sem labels proibidos.
- [x] 10.5 Revisar diff final com foco em regressão, segurança, custo, idempotência, privacidade e Mastra.
- [x] 10.6 Registrar estado final como `merge-ready` ou `production-ready`; se qualquer evidência faltar, marcar `blocked` ou `needs_input`.

## Detalhes de Implementação

Referenciar `techspec.md` nas seções `Gates de Validacao para Merge`, `Conformidade com Skills e Regras` e `Mapeamento RF -> Decisao -> Gate`.

Critério de verdade:
- Report autodeclarado não substitui teste executado, evidência `path:line`, payload real mascarado ou métrica/alerta versionado.
- Se `golangci-lint` falhar por toolchain incompatível, registrar comando, versão e escopo; não mascarar como aprovado.

## Critérios de Sucesso

- `ai-spec check-spec-drift .specs/prd-agente-audio-openrouter` sem drift.
- Todos os RFs têm evidência objetiva.
- Gates globais definidos pela techspec executados ou bloqueio explicitamente registrado.
- VPS/deploy só pode ser considerado production-ready após deploy controlado e monitoramento inicial conforme runbook.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — O gate final precisa validar runtime, consumer, tools, workflows e OpenRouter no padrão do agente.
- `domain-modeling-production` — O gate final precisa validar estados fechados, invariantes e outcomes.
- `design-patterns-mandatory` — O gate final precisa validar a decisão de pattern e ausência de overengineering.

## Testes da Tarefa

- [x] `gofmt -w <arquivos-go-alterados>`
- [x] `go build ./internal/platform/... ./internal/agents/... ./configs/... ./cmd/server/... ./cmd/worker/...`
- [x] `go vet ./internal/platform/... ./internal/agents/... ./configs/... ./cmd/server/... ./cmd/worker/...`
- [x] `go test -race -count=1 ./internal/platform/... ./internal/agents/... ./configs/...`
- [x] `RUN_REAL_LLM=1 RUN_REAL_STT=1 STT_REAL_AUDIO_FIXTURE=<audio-ptbr-real> go test -tags=integration -count=1 ./internal/agents/application/golden/...`
- [x] `golangci-lint run ./internal/platform/... ./internal/agents/... ./configs/... ./cmd/server/... ./cmd/worker/...`
- [x] Gates grep da techspec.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.specs/prd-agente-audio-openrouter/prd.md`
- `.specs/prd-agente-audio-openrouter/techspec.md`
- `.specs/prd-agente-audio-openrouter/tasks.md`
- `.specs/prd-agente-audio-openrouter/task-*.md`
- `internal/platform/`
- `internal/agents/`
- `configs/`
- `cmd/server/`
- `cmd/worker/`
