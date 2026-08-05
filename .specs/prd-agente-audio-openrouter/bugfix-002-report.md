# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 1
- Pendentes: nenhum
- Estado final: done

## Bugs
- ID: BUG-002
- Severidade: minor (contexto de review original: medium)
- Origem: finding de review de codigo (task 6.0/6.1, PRD prd-agente-audio-openrouter, mapeamento de audio no route WhatsApp -> outbox)
- Estado: fixed
- Causa raiz: `buildWhatsAppAgentRoute` mapeia `msg.Audio` (MediaID/MimeType/SHA256/Voice) para `whatsAppInboundPayload` quando `audioEnabled=true` e `msg.Type == wapayload.MessageTypeAudio`, mas `TestWhatsAppAgentRouteSuite` em `module_test.go` so exercitava cenarios com `audioEnabled=false`. Nenhum teste cobria o caminho de audio, deixando o mapeamento sem rede de seguranca de regressao.
- Arquivos alterados:
  - `internal/agents/module_test.go` (novo cenario de teste `TestBuildWhatsAppAgentRoute_AudioEnabled_MapsAudioMetadataToPayload` na suite `WhatsAppAgentRouteSuite`; import `encoding/json` adicionado)
- Teste de regressao: `TestBuildWhatsAppAgentRoute_AudioEnabled_MapsAudioMetadataToPayload` chama `buildWhatsAppAgentRoute(publisherMock, o11y, true)` com `wapayload.Message{Type: MessageTypeAudio, Audio: &wapayload.Audio{MediaID, MimeType, SHA256, Voice}}`, captura o `outbox.Event` publicado via `mock.MatchedBy`, desserializa `capturedEvent.Payload` em `whatsAppInboundPayload` e asserta `MessageType`, `AudioMediaID`, `AudioMimeType`, `AudioSHA256`, `AudioVoice`.
- Validacao: `go build ./internal/agents/...` (sem erros); `go vet ./internal/agents/...` (sem erros); `go test -race -count=1 ./internal/agents/ -run TestWhatsAppAgentRouteSuite -v` (5/5 subtestes PASS, incluindo o novo); `go test -race -count=1 ./internal/agents/...` (todos os pacotes OK, sem regressao).

## Comandos Executados
- `go build ./internal/agents/...` -> sem output, build OK
- `go vet ./internal/agents/...` -> sem output, vet OK
- `go test -race -count=1 ./internal/agents/ -run TestWhatsAppAgentRouteSuite -v` -> PASS (5/5 subtestes, incluindo `TestBuildWhatsAppAgentRoute_AudioEnabled_MapsAudioMetadataToPayload`)
- `go test -race -count=1 ./internal/agents/...` -> ok em todos os pacotes do modulo `internal/agents` (sem regressao)
- `grep -n "^[[:space:]]*//" internal/agents/module_test.go | grep -Ev "(//go:|//nolint:|// Code generated)"` -> vazio (sem comentarios introduzidos)

## Riscos Residuais
- Nenhum identificado para este bug pontual. O mapeamento de audio em `buildWhatsAppAgentRoute` (linhas ~565-571 de `internal/agents/module.go`) nao foi alterado, apenas coberto por teste de regressao.
