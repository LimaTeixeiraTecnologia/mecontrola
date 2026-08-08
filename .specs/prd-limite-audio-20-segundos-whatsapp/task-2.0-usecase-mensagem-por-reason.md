# Tarefa 2.0: Use case: seleção de mensagem por reason com testes unitários e de fronteira

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender a seleção de mensagem de rejeição em `ProcessAudioInbound.replyFor` para decidir também por `AudioReason`, retornando a mensagem dedicada de duração excedida quando `reason == AudioReasonDurationExceeded`, sem tocar o consumer e sem alterar os demais motivos (ADR-002).

<requirements>
- RF-01: fronteira inclusiva de 20s provada por teste (fixture de exatamente 20s aprovada).
- RF-02: rejeição acima de 20s antes do STT provada por teste.
- RF-03: mensagem específica do limite para `duration_exceeded`, com fallback para constante default.
- RF-04: demais motivos de rejeição mantêm `RejectedReply` e `UncertainReply` inalterados, provado por teste de não regressão.
</requirements>

## Subtarefas

- [ ] 2.1 Adicionar constante `defaultAudioDurationExceededReply` no bloco `process_audio_inbound.go:22-32`, com texto idêntico ao `SetDefault` da tarefa 1.0.
- [ ] 2.2 Adicionar campo `DurationExceededReply string` em `ProcessAudioInboundConfig` (`:39-47`).
- [ ] 2.3 Estender `resultFromRecord` (`:471-484`) para repassar `record.Reason` e `replyFor` (`:486-493`) para assinatura `(outcome AudioOutcome, reason AudioReason)`, com caso dedicado para `AudioReasonDurationExceeded` via `firstNonEmpty(p.cfg.DurationExceededReply, defaultAudioDurationExceededReply)`; único call-site é `:482`.
- [ ] 2.4 Não alterar `decide_audio_transcription.go`, o consumer, nem `media.CheckMaxDuration`.
- [ ] 2.5 Testes em `process_audio_inbound_test.go`: no cenário de duração excedida (`:486-518`), assertar `result.ReplyText` igual à mensagem configurada; variante sem config caindo no default; não regressão de `media_unavailable` (`:474-483`) mantendo `RejectedReply`; incerto mantendo `UncertainReply`.
- [ ] 2.6 Testes de fronteira: fixture sintética de exatamente 20s aprovada com `MaxDuration: 20s`; fixture de 21s rejeitada com `Reason = AudioReasonDurationExceeded` (reusar `buildOggOpusFixture`, `:414`).
- [ ] 2.7 Testes em `internal/platform/whatsapp/media/duration_test.go` (`:91-98`): casos 20s passa e 21s falha contra max 20s, com `errors.Is(err, media.ErrAudioDurationExceeded)`.

## Detalhes de Implementação

Ver `techspec.md` seções `Design de Implementação` (Interfaces Chave, com o snippet de `replyFor`) e `Abordagem de Testes` (itens 2 e 3). A decisão `não aplicar padrão` está registrada na ADR-003 com saída do seletor determinístico; manter a solução direta.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/agents/application/usecases/... ./internal/platform/whatsapp/media/...` verde.
- Asserções novas provam: mensagem dedicada no motivo de duração, fallback no default e mensagens antigas intactas nos demais motivos.
- Nenhum diff em `whatsapp_inbound_consumer.go`, `decide_audio_transcription.go` ou nos parsers de mídia.
- `golangci-lint run` no escopo dos pacotes alterados sem findings novos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — o use case pertence ao fluxo Thread-Run do consumer agentivo `internal/agents`; a decisão por motivo deve permanecer no use case com o consumer adapter fino, sem reimplementar primitivos de plataforma.
- `design-patterns-mandatory` — gate de desenho já resolvido com `reject` na ADR-003; reexecutar o seletor apenas se a implementação revelar sinal estrutural novo.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/usecases/process_audio_inbound.go`
- `internal/agents/application/usecases/process_audio_inbound_test.go`
- `internal/platform/whatsapp/media/duration_test.go`
- `internal/agents/application/usecases/decide_audio_transcription.go` (somente leitura)
- `.specs/prd-limite-audio-20-segundos-whatsapp/adr-002-selecao-de-mensagem-por-reason-no-usecase.md`
- `.specs/prd-limite-audio-20-segundos-whatsapp/adr-003-nao-aplicar-design-pattern.md`
