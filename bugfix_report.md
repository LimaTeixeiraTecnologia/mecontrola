# Relatorio de Bugfix

- Total de bugs no escopo: 2
- Corrigidos: 2
- Testes de regressao adicionados: 8
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: BUG-WA-NONTEXT-DEADLETTER
- Severidade: critical
- Origem: investigacao de producao 2026-08-07 (outbox_events status=4, payload text:"" — 66 dead letters em 2026-08-04)
- Estado: fixed
- Causa raiz: `parseMessage` (`internal/platform/whatsapp/payload/parser.go:30-46`) caia no branch generico para qualquer tipo de mensagem diferente de `audio` e fabricava `Message{Type: MessageTypeText, Text: ""}` para tipos nao suportados (`image`, `sticker`, `video`, `document`, `reaction`, `location`, `contacts`, etc.), cujo campo `text` e nil no webhook. O route `buildWhatsAppAgentRoute` publicava `agents.whatsapp.inbound.v1` com `text: ""`; o consumer (`validateInboundPayload`) rejeitava com `payload incompleto`, esgotava 3 retries e o evento morria na outbox (status=4).
- Arquivos alterados:
  - `internal/platform/whatsapp/payload/parser.go:33-35` — guard `if msg.Type != "text" { return Message{}, false }` apos o branch de audio: tipos nao suportados nao geram `Message`, logo nenhum evento chega a outbox. Comportamento de `text` (inclusive texto vazio com `msg.Text` nil) e `audio` (validacoes de media_id/mime/sha256 em `parseAudioMessage`) inalterado.
  - `internal/platform/whatsapp/payload/parser_test.go:116` — `TestExtractFirstMessage_NilTextBody` ajustado para fixture `type:"text"` sem corpo (preserva a assercao de texto vazio para tipo text; antes usava `type:"image"`, que codificava o comportamento bugado).
  - `internal/platform/whatsapp/payload/parser_test.go:214` — `TestExtractFirstMessage_UnknownMessageType_TreatedAsTextFallback` substituido por `TestExtractFirstMessage_UnsupportedMessageType_Ignored` (o teste anterior codificava o bug).
- Teste de regressao:
  - `TestExtractFirstMessage_UnsupportedMessageType_Ignored` (sticker -> ok=false).
  - `TestExtractMessages_UnsupportedMessageTypes_Ignored` (image, sticker, reaction, video, document, location, contacts -> nenhuma mensagem extraida).
  - `TestExtractMessages_MixedSupportedAndUnsupported_KeepsOnlySupported` (payload misto image+text+reaction -> apenas o text extraido).
  - Comportamento inalterado coberto pelos testes preexistentes: text com body, text sem body, audio valido e audio invalido (media_id/mime/sha256).
- Validacao: mapeamento de callers confirmou seguranca — `ExtractMessages` tem unico caller de producao (`dispatcher.Route`, `internal/platform/whatsapp/dispatcher/dispatcher.go:109`), que trata `len(msgs)==0` como `OutcomeInvalid` sem publicar evento; `ExtractFirstMessage` so tem callers de teste; onboarding consome apenas `payload.MaskMobile` e o tipo `payload.Message`; handler inbound (`internal/platform/whatsapp/handlers/inbound_handler.go:37`) ignora o outcome. `go test -race -count=1 ./internal/platform/whatsapp/... ./internal/onboarding/...` ok; `go build ./...` + `go vet ./...` ok; `golangci-lint run` (binario pinado `.tools/bin/golangci-lint`) no escopo: 0 issues.

- ID: BUG-DESTRUCTIVE-INVALID-UUID
- Severidade: major
- Origem: investigacao de producao 2026-08-07 (platform_runs status=failed — run 9617d47d-83f1-410f-8dd4-23af7dc6c91d em 2026-08-04 09:08 com "parse entry uuid: invalid UUID")
- Estado: fixed
- Causa raiz: o step do workflow `destructive-confirm` (`internal/agents/application/workflows/destructive_manage_workflow.go`) nao validava `state.TargetRef` na entrada: `DestructiveOpDeleteEntry` com TargetRef nao-UUID (extracao LLM, ex.: "mercado") suspendia para confirmacao e falhava em `executeDestructiveManageDeleteEntry` (`uuid.Parse`) apos o "sim" do usuario, marcando o run como failed e expondo erro tecnico; `DestructiveOpDeleteCard` com TargetRef vazio/nao-UUID tinha o mesmo destino em `executeDestructiveManageDeleteCard`.
- Arquivos alterados:
  - `internal/agents/application/workflows/destructive_manage_workflow.go:67` — condicao de `DeleteEntry` alargada de `TargetRef == ""` para `!isDestructiveManageUUIDRef(state.TargetRef)`: UUID invalido passa a ser tratado como ref ausente e roteia para `handleDestructiveEntrySearch` (busca de candidatos, fluxo ja testado). O ponto cobre start e resume porque o step re-executa com `ResumeText` vazio no start.
  - `internal/agents/application/workflows/destructive_manage_workflow.go:70-74` — `DeleteCard` com TargetRef vazio/nao-UUID completa na entrada como `DestructiveManageCancelled` com mensagem de dominio amigavel, sem falhar o run e sem suspender para confirmacao.
  - `internal/agents/application/workflows/destructive_manage_workflow.go:121-124` — helper puro `isDestructiveManageUUIDRef` (uuid.Parse), estilo das funcoes `Decide*` do pacote.
  - `internal/agents/application/messages/transaction_write_messages.go:136-138` — nova mensagem `NoDeleteCardIdentified()` (nenhuma mensagem existente era adequada: `NoDeleteCandidateFound` e especifica de lancamento).
  - `DeleteRecurrence`/`UpdateRecurrence` nao tocados (usam TargetRef string sem parse) — fora do escopo.
- Teste de regressao:
  - `TestFirstEntryDeleteEntry_InvalidUUIDTargetRef_RoutesToCandidateSearch` (TargetRef="mercado" -> SearchEditCandidates chamado, Completed/Cancelled com "Nao encontrei", nunca StepStatusFailed).
  - `TestFirstEntryDeleteEntry_InvalidUUIDTargetRef_WithCandidates_SuspendsForDisambiguation` (TargetRef="mercado" com candidato -> suspend com prompt de confirmacao, TargetRef promovido ao UUID do candidato).
  - `TestFirstEntryDeleteCard_InvalidUUIDTargetRef_CompletesCancelled` (TargetRef="nao-e-uuid" -> Completed/Cancelled com mensagem amigavel, SoftDeleteCard nunca chamado).
  - `TestFirstEntryDeleteCard_EmptyTargetRef_CompletesCancelled` (idem para TargetRef vazio).
  - `TestNoDeleteCardIdentified_NotEmpty` (pacote messages).
  - Fluxos felizes inalterados cobertos pela suite preexistente (confirm/cancel/reprompt/expire para delete_card, delete_entry, delete_recurrence, update_recurrence e busca de candidatos) — suite inteira do pacote verde.
- Validacao: `go test -race -count=1 ./internal/agents/...` (modulo inteiro) ok; `go test -race -count=1 ./internal/agents/application/workflows/... ./internal/agents/application/messages/...` ok; `go build ./...` + `go vet ./...` ok; `gofmt -l` limpo nos arquivos tocados; `golangci-lint run` (pinado) no escopo: 0 issues.

## Comandos Executados

- `source .agents/lib/check-invocation-depth.sh` -> depth OK: 1
- `python3 .agents/skills/design-patterns-mandatory/scripts/select_pattern.py --input -` -> status `reject` (decisao: nao aplicar padrao para ambas as correcoes)
- `gofmt -l internal/platform/whatsapp/payload/ internal/agents/application/workflows/ internal/agents/application/messages/` -> sem saida (formatacao ok)
- `go test -race -count=1 ./internal/platform/whatsapp/payload/...` -> ok 1.485s
- `go test -race -count=1 ./internal/agents/application/workflows/... ./internal/agents/application/messages/...` -> ok 2.029s / ok 1.399s
- `go build ./... && go vet ./...` -> BUILD_VET_OK
- `go test -race -count=1 ./internal/platform/whatsapp/... ./internal/onboarding/...` -> todos ok (consumidores do parser: dispatcher, handlers, onboarding)
- `go test -race -count=1 ./internal/agents/...` -> todos ok (prova de 0 regressao no modulo)
- `./.tools/bin/golangci-lint run ./internal/platform/whatsapp/payload/... ./internal/agents/application/workflows/... ./internal/agents/application/messages/...` -> 0 issues
- `grep -nE '\b_[a-zA-Z][a-zA-Z0-9]*'` nos arquivos de producao tocados -> sem identificador com prefixo `_` (gate R5.26)
- `grep -nE '//'` nos arquivos de producao tocados -> zero comentarios em Go de producao
- `bash .agents/scripts/validate-bugfix-evidence.sh bugfix_report.md` -> aprovada (exit 0)

## Riscos Residuais

- Runs suspensos persistidos antes da correcao com TargetRef invalido em estado suspenso ainda falhariam no resume (a validacao nova atua na entrada do step, antes de suspender; estados ilegais antigos em voo nao sao reparados). Mitigacao operacional: reaper de suspended stale (`BuildDestructiveManageReaper`) expira esses runs. Incidentes de 2026-08-04 ja constam como failed.
- `text` com corpo vazio (msg.Text nil) continua gerando evento com `text: ""` que o consumer rejeita — decisao deliberada mantida do diagnostico (consumer valida), fora do escopo desta correcao.
- Mudancas preexistentes no working tree nao relacionadas (`cmd/worker/worker.go`, `deployment/telemetry/grafana/**`, `internal/platform/llm/credits*.go`, remocao de `.github/workflows/version-drift-check.yml`) nao foram tocadas nem validadas neste bugfix.
- `go test ./...` amplo nao executado (validacao proporcional por escopo: pacotes alterados + consumidores do parser + modulo internal/agents inteiro); nenhuma falha preexistente observada nos escopos rodados.
