# Tarefa 7.0: Configuração, métricas e logs de áudio

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar configuração production-safe, métricas e logs sanitizados para áudio/STT. A tarefa deve permitir operar custo, latência, erro, incerteza, tamanho, duração e dispatch sem labels de alta cardinalidade.

<requirements>
- Cobrir RF-28, RF-29 e RF-45.
- Adicionar `AGENT_AUDIO_ENABLED=false` por default.
- Validar produção quando áudio estiver habilitado.
- Emitir métricas com labels permitidos na techspec.
- Não logar áudio bruto, base64, token, URL temporária, WAMID em métrica, telefone ou transcrição completa em INFO.
- Usar alertas iniciais da techspec, incluindo `audio_cost_microusd_1h > 120000`.
</requirements>

## Subtarefas

- [x] 7.1 Adicionar campos de config e parsing de env em `configs/config.go`.
- [x] 7.2 Validar production quando `AudioEnabled=true`: modelo, bytes, duração, custo, timeout e replies.
- [x] 7.3 Integrar config no wiring de server/worker sem alterar default textual.
- [x] 7.4 Instrumentar métricas de áudio com labels de baixa cardinalidade.
- [x] 7.5 Instrumentar logs sanitizados por outcome/reason sem transcrição completa.
- [x] 7.6 Adicionar testes de config, labels e grep de vazamento.

## Detalhes de Implementação

Referenciar `techspec.md` nas seções `Configuracao`, `Monitoramento e Observabilidade` e `Gates de Validacao para Merge`.

Evidências de codebase a respeitar:
- `configs/config.go:180`
- `configs/config.go:541`
- `configs/config.go:1034`
- `cmd/worker/worker.go:290`
- `internal/platform/observability/`

## Critérios de Sucesso

- Áudio desabilitado por default.
- Produção falha startup/config validation quando áudio habilitado sem limites obrigatórios.
- Métricas não contêm `user_id`, `resource_id`, `thread_id`, `wamid`, telefone, media id ou conteúdo.
- Logs INFO não contêm transcrição completa nem audio/base64.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — A configuração afeta o consumidor agentivo e o caminho até `AgentRuntime`.
- `domain-modeling-production` — Outcomes/reasons instrumentados devem permanecer fechados.
- `design-patterns-mandatory` — A instrumentação deve evitar decorators/patterns novos sem necessidade.
- `golden-signals-otel-standards` — A tarefa define métricas, cardinalidade, latência, erro, tráfego e custo.

## Testes da Tarefa

- [x] `go test -race -count=1 ./configs/...`
- [x] `go test -race -count=1 ./internal/agents/... ./internal/platform/...`
- [x] Grep de labels proibidos em métricas.
- [x] Teste de config production com áudio habilitado/desabilitado.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `configs/config.go`
- `cmd/server/`
- `cmd/worker/`
- `internal/platform/observability/`
- `internal/agents/`
