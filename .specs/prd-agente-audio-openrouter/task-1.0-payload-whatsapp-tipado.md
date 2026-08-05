# Tarefa 1.0: Payload WhatsApp tipado e regressão textual

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender o parser e o contrato de payload WhatsApp para representar texto e áudio como mensagens tipadas, preservando 100% do comportamento textual atual. A tarefa fecha a entrada estrutural para que áudio não vire `text=""` nem perca `media_id`, MIME, SHA-256, timestamp ou `voice`.

<requirements>
- Cobrir RF-01, RF-02, RF-03, RF-04, RF-05, RF-40 e RF-41.
- Preservar assinatura, timestamp, dedup, principal autenticado e rate limit antes de qualquer processamento de áudio.
- Usar fixture real sanitizada de `whatsapp-audio-payload-evidence-2026-08-04.md`.
- Rejeitar payload de áudio sem campos obrigatórios confirmados.
- Não publicar áudio bruto, base64, telefone ou transcrição em logs/test fixtures.
</requirements>

## Subtarefas

- [x] 1.1 Ler `internal/platform/whatsapp/payload/types.go`, `parser.go`, dispatcher e handler WhatsApp reais.
- [x] 1.2 Modelar `MessageTypeText` e `MessageTypeAudio` como tipos fechados, com zero value inválido quando aplicável.
- [x] 1.3 Adicionar `Audio` ao contrato público `payload.Message` com `MediaID`, `MimeType`, `SHA256` e `Voice`.
- [x] 1.4 Ajustar parser para texto continuar idêntico e áudio sair tipado sem preencher texto vazio como mensagem textual.
- [x] 1.5 Criar testes de parser para texto atual, áudio real sanitizado, áudio sem media id, payload desconhecido e regressão de timestamp/WAMID.
- [x] 1.6 Rodar gates direcionados do pacote e grep de vazamento de áudio bruto/base64 em fixtures.

## Detalhes de Implementação

Referenciar `techspec.md` nas seções `Payload WhatsApp`, `Fluxo de Dados` e `Mapeamento RF -> Decisao -> Gate`.

Evidências de codebase a respeitar:
- `internal/platform/whatsapp/payload/types.go:29`
- `internal/platform/whatsapp/payload/types.go:41`
- `internal/platform/whatsapp/payload/parser.go:19`
- `internal/platform/whatsapp/dispatcher/dispatcher.go:127`
- `internal/platform/whatsapp/dispatcher/dispatcher.go:132`
- `internal/platform/whatsapp/dispatcher/dispatcher.go:141`
- `internal/platform/whatsapp/dispatcher/dispatcher.go:154`

## Critérios de Sucesso

- Mensagens textuais existentes continuam com o mesmo output público e testes verdes.
- Mensagens de áudio reais sanitizadas produzem `MessageTypeAudio` com metadados obrigatórios.
- Áudio inválido é rejeitado de forma tipada antes de download/STT.
- Nenhuma regra financeira, chamada STT ou download de mídia é introduzido nesta tarefa.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — O payload WhatsApp é a entrada do consumidor agentivo `internal/agents`.
- `domain-modeling-production` — A tarefa introduz estados fechados de modalidade e rejeição técnica.
- `design-patterns-mandatory` — A tarefa deve confirmar `nao aplicar padrao` e evitar abstração desnecessária.

## Testes da Tarefa

- [x] `go test -race -count=1 ./internal/platform/whatsapp/payload`
- [x] `go test -race -count=1 ./internal/platform/whatsapp/...`
- [x] Teste com fixture real sanitizada de áudio.
- [x] Teste de regressão textual existente.
- [x] Grep para impedir áudio bruto/base64 em fixture versionada.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/whatsapp/payload/types.go`
- `internal/platform/whatsapp/payload/parser.go`
- `internal/platform/whatsapp/payload/*_test.go`
- `.specs/prd-agente-audio-openrouter/whatsapp-audio-payload-evidence-2026-08-04.md`
