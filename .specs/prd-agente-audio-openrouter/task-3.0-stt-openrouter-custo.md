# Tarefa 3.0: Porta STT OpenRouter com custo pre e pos-STT

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar porta de transcrição STT em `internal/platform/llm` usando OpenRouter como provider único, com timeout, modelo configurável, formato `ogg`, linguagem `pt`, custo pre-STT por duração/modelo e gate pós-STT por `usage.cost`.

<requirements>
- Cobrir RF-09, RF-10, RF-11, RF-12, RF-36 e RF-45.
- Usar somente OpenRouter.
- Não criar fallback chain, provider paralelo ou chamada a chat completions para áudio.
- Falhar fechado para modelo indisponível, timeout, formato não suportado, resposta inválida, truncamento e custo acima do teto.
- Permitir suite real por `RUN_REAL_STT=1`.
</requirements>

## Subtarefas

- [ ] 3.1 Ler `internal/platform/llm/provider.go` e `openrouter.go` reais antes de alterar contratos.
- [ ] 3.2 Adicionar porta STT pequena e separada de `Complete`, `Stream` e `Embed`.
- [ ] 3.3 Implementar OpenRouter `/api/v1/audio/transcriptions` com JSON base64, `language=pt`, `temperature=0` e timeout 20s.
- [ ] 3.4 Implementar preflight de custo usando duração extraída, `AGENT_AUDIO_MAX_COST_MICROUSD` e modelo escolhido.
- [ ] 3.5 Implementar gate pós-STT por `usage.cost`, resposta vazia, erro HTTP, timeout, formato inválido e modelo indisponível.
- [ ] 3.6 Criar testes unitários com server fake e teste real skipado por flag.

## Detalhes de Implementação

Referenciar `techspec.md` nas seções `Contrato STT OpenRouter`, `Benchmark Obrigatorio Antes das Tasks de Codigo` e `OpenRouter STT`.

Evidências de codebase a respeitar:
- `internal/platform/llm/provider.go:5`
- `internal/platform/llm/openrouter.go:22`
- `.specs/prd-agente-audio-openrouter/benchmark-stt.md`
- `.specs/prd-agente-audio-openrouter/stt-lote-30-results.json`

## Critérios de Sucesso

- `llm.Transcriber` não altera comportamento de chat/embeddings.
- Modelo vem de config, não hardcoded em código de aplicação.
- Preflight impede gasto previsivelmente acima do teto antes de chamada externa.
- Pós-STT impede dispatch quando custo medido exceder teto.
- Teste real não roda sem `RUN_REAL_STT=1` e `OPENROUTER_API_KEY`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — STT deve consumir o provider OpenRouter da plataforma sem criar runtime paralelo.
- `domain-modeling-production` — Outcomes de STT e erros tipados alimentam decisões fechadas.
- `design-patterns-mandatory` — A solução deve permanecer porta/adapter simples, sem fallback ou pattern formal novo.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/platform/llm`
- [ ] Teste fake para HTTP 200, 400, 401/403, 402, 429, 5xx, timeout, JSON inválido e texto vazio.
- [ ] Teste de preflight de custo acima do teto antes da chamada.
- [ ] `RUN_REAL_STT=1 go test -count=1 ./internal/platform/llm -run Real`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/llm/provider.go`
- `internal/platform/llm/openrouter.go`
- `internal/platform/llm/*_test.go`
- `configs/config.go`
