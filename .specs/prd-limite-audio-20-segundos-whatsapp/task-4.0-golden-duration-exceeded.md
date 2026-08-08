# Tarefa 4.0: Golden: cenário duration_exceeded com prova de zero chamada ao STT

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar à suíte golden `AudioNegativePipelineSuite` o cenário negativo de duração excedida, provando de ponta a ponta (com doubles determinísticos) que um áudio acima do limite é rejeitado antes de qualquer chamada ao STT, com outcome, reason e mensagem dedicada corretos.

<requirements>
- RF-01 e RF-02: aceite até 20s e rejeição acima, provados no pipeline completo com configuração de 20s.
- RF-03: `ReplyText` do cenário golden igual à mensagem dedicada de duração.
- Objetivo primário do PRD: 100% dos áudios acima do limite rejeitados antes do STT, provado por `transcriber.Calls == 0`.
</requirements>

## Subtarefas

- [ ] 4.1 Novo cenário em `internal/agents/application/golden/audio_pipeline_test.go`: config com `MaxDuration: 20s` e `DurationExceededReply` preenchida, fixture Ogg sintética acima de 20s (mesma técnica de `validAudioOggFixture`, `:181-183`, com granule maior).
- [ ] 4.2 Asserções: `Outcome = AudioOutcomeRejected`, `Reason = AudioReasonDurationExceeded`, `ReplyText` igual à mensagem configurada, `transcriber.Calls == 0` e auditoria persistida com `duration_ms` preenchido.
- [ ] 4.3 Confirmar que os cenários existentes (fixtures de 1-2s) seguem verdes sem alteração de expectativa, provando não regressão do caminho feliz.
- [ ] 4.4 Não alterar os gates reais (`RUN_REAL_STT`, `STT_REAL_AUDIO_FIXTURE`) nem `harness_audio_realllm_test.go`.

## Detalhes de Implementação

Ver `techspec.md` seção `Abordagem de Testes` (Testes E2E e Golden, primeiro bullet). A suíte usa doubles existentes (`SimulatedTranscriber`, `simulatedMediaClient`, `inMemoryAudioAuditRepository`); nenhum mock novo.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/agents/application/golden/...` verde com o cenário novo.
- O cenário falha se a rejeição deixar de ocorrer antes do STT (asserção `transcriber.Calls == 0` é a prova do guardrail de custo).
- Nenhum diff nas fixtures e doubles existentes além do acréscimo do cenário.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — a suíte golden cobre o pipeline do consumer agentivo; o cenário novo deve preservar o fluxo canônico inbound -> ProcessAudioInbound -> auditoria -> resposta, sem atalhos que furem o substrato.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/golden/audio_pipeline_test.go`
- `internal/agents/application/usecases/process_audio_inbound.go` (somente leitura)
- `.specs/prd-limite-audio-20-segundos-whatsapp/techspec.md`
