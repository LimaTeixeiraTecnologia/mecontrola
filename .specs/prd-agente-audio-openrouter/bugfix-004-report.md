# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 1
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: BUG-004
- Severidade: minor (achado original da review, nivel `low` no schema de 4 niveis)
- Origem: finding de review de codigo (R-DTO-VALIDATE-001, `.claude/rules/input-dto-validate.md`), arquivo `internal/agents/application/usecases/process_audio_inbound.go`
- Estado: fixed
- Causa raiz: `handleSTTError` (caminho de erro STT) construia `input.AudioDecisionInput` e chamava `DecideAudioTranscription` diretamente, sem chamar `.Validate()` no DTO antes de usa-lo — diferente do caminho de sucesso STT em `transcribeAndDecide`, que sempre chama `decisionInput.Validate()` logo apos construir o DTO (R-DTO-VALIDATE-001 / R-DTO-002). O comportamento era inofensivo em producao porque `STTModel` e `MinConfidence` vem de `ProcessAudioInboundConfig` ja validada no startup, mas violava a aplicacao consistente do gate de validacao do DTO nos dois caminhos que constroem o mesmo tipo.
- Arquivos alterados:
  - `internal/agents/application/usecases/process_audio_inbound.go` — `handleSTTError` passou a construir `decisionInput` numa variavel local e chamar `decisionInput.Validate()` imediatamente apos a construcao, retornando erro tecnico interno (`fmt.Errorf("agents.usecase.process_audio_inbound: decision input invalido: %w", validateErr)`) em caso de falha, replicando exatamente o padrao ja usado em `transcribeAndDecide` (linhas ~309-311 antes da mudanca). `DecideAudioTranscription` so e chamado apos a validacao passar.
  - `internal/agents/application/usecases/process_audio_inbound_test.go` — novo cenario `"deve falhar tecnicamente sem persistir quando decision input do caminho de erro stt e invalido"` na tabela `TestExecute`, cobrindo o novo gate.
- Teste de regressao: o novo cenario forca `cfg.STTModel = ""` (unico jeito de tornar `AudioDecisionInput.Validate()` invalido no caminho de erro, ja que `Text`/`Confidence` nao sao usados por `handleSTTError` e `MinConfidence`/`Confidence` vem em range valido a partir de `baseCfg()`), com o transcriber retornando `llm.ErrSTTUpstream` (entra em `handleSTTError`). O `auditMock` so expira `FindByWAMID` (sem `InsertTerminal`), provando que a funcao retorna erro tecnico ANTES de chegar em `p.terminal(...)`. O `expect` verifica `s.Error(err)`, `s.Contains(err.Error(), "decision input invalido")` e `s.Equal(AudioInboundResult{}, result)`.
  - Nao foi possivel, dentro do fluxo real de producao (config ja validada em `NewProcessAudioInbound`/bootstrap), simular um `AudioDecisionInput` invalido "naturalmente" sem forcar `STTModel` vazio via `cfg` no teste — por isso o cenario manipula `cfg` diretamente para exercitar o gate, e nao um dado de producao plausivel. Isso e esperado e documentado aqui: o objetivo do teste e comprovar que o gate de validacao existe e bloqueia corretamente, nao que ele dispare em producao.
- Validacao:
  - Leitura completa de `process_audio_inbound.go`, comparando o caminho de sucesso STT (`transcribeAndDecide`, linhas 301-312) com o caminho de erro STT (`handleSTTError`, linhas 338-365) antes da mudanca — confirmado que so o caminho de sucesso chamava `.Validate()`.
  - Leitura de `internal/agents/application/dtos/input/audio_decision_input.go` — confirmado que `Validate()` so checa `Model != ""` e ranges de `MinConfidence`/`Confidence` (nil-safe), portanto seguro chamar em ambos os caminhos sem quebrar o fluxo de sucesso existente (STTError nao e validado, e Text/Language/Truncated nao sao exigidos).
  - Leitura de `decide_audio_transcription.go` (via subagente Explore) confirmando a assinatura de `DecideAudioTranscription` e o uso de `in.STTError` como primeiro branch, sem dependencia de `Text`.
  - Leitura de `process_audio_inbound_test.go` para replicar exatamente o estilo canonico testify/suite table-driven (`dependencies` com IIFE por mock, cfg por cenario, SUT instanciado dentro de `s.Run`).

## Comandos Executados
- `gofmt -l internal/agents/application/usecases/process_audio_inbound.go internal/agents/application/usecases/process_audio_inbound_test.go` -> vazio apos `gofmt -w` no teste (indentacao do novo cenario corrigida automaticamente)
- `go build ./internal/agents/...` -> sem erros
- `go vet ./internal/agents/...` -> sem erros
- `go test -race -count=1 ./internal/agents/application/usecases/... ./internal/agents/...` -> todos os pacotes `ok`, incluindo `internal/agents/application/usecases` (4.369s) com o novo cenario passando
- `grep -n "^[[:space:]]*//" internal/agents/application/usecases/process_audio_inbound.go | grep -Ev "(//go:|//nolint:|// Code generated)"` -> vazio (R-ADAPTER-001.1 zero comentarios preservado)

## Riscos Residuais
- Nenhum. A mudanca e estritamente aditiva ao gate de validacao (mesmo padrao ja usado no caminho de sucesso), nao altera comportamento observavel em producao (config valida sempre passa no novo `Validate()`), e toda a suite de testes do modulo `internal/agents` permanece verde com `-race`.
