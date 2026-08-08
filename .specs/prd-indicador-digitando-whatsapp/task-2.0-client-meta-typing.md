# Tarefa 2.0: Método SendTypingIndicator no client Meta com payload oficial

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar ao client Meta a chamada de mark-as-read com typing indicator, com tipos de payload próprios e fluxo de envio separado de `doSend`, porque a resposta desta chamada é `{"success": true}` e não contém `messages[]`.

<requirements>
- RF-01: payload com `status: "read"`, `message_id` e `typing_indicator` do tipo `text`, sem campo `to`.
- RF-04: erros mapeados pelos sentinelas existentes para tratamento best-effort no consumidor.
- RF-07: esta tarefa produz o artefato técnico (payload exato) que a tarefa 6.0 valida em ambiente real.
- RF-09: operação separada de `SendText`, sem reutilizar `doSend` nem seu contrato de resposta.
</requirements>

## Subtarefas

- [ ] 2.1 Adicionar `markAsReadRequest` e `typingIndicatorPayload` em `internal/onboarding/infrastructure/http/client/meta/models.go`, conforme modelagem da techspec.
- [ ] 2.2 Adicionar `func (c *Client) SendTypingIndicator(ctx context.Context, wamid string) error` em `client.go`, reutilizando o mesmo endpoint `/%s/messages`, headers e `WithoutRetry()`, validando apenas o status HTTP via `checkStatus`.
- [ ] 2.3 Retornar erro com `%w` nos sentinelas existentes (`ErrMetaAuth`, `ErrMetaBadRequest`, `ErrMetaServer`) via `checkStatus`; nenhum sentinela novo.
- [ ] 2.4 Testes httptest em `client_test.go`: assert do JSON exato (campos e ausência de `to`), sucesso 2xx e mapeamento de 401, 400 e 500.

## Detalhes de Implementação

Ver `techspec.md`, seções "Interfaces Chave" e "Modelos de Dados". Contrato oficial da Meta referenciado no PRD (RF-01, restrições). Validação manual em ambiente real fica na tarefa 6.0, não aqui.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/onboarding/infrastructure/http/client/meta/...` verde.
- Payload serializado idêntico ao contrato oficial documentado na techspec.
- Nenhum comentário em Go de produção; `go build ./...` e `go vet ./...` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (httptest com assert de payload e erros)
- [ ] Testes de integração (não aplicável: fronteira real com a Meta é validada no gate 6.0; justificativa registrada)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/infrastructure/http/client/meta/client.go`
- `internal/onboarding/infrastructure/http/client/meta/models.go`
- `internal/onboarding/infrastructure/http/client/meta/errors.go`
- `internal/onboarding/infrastructure/http/client/meta/client_test.go`
