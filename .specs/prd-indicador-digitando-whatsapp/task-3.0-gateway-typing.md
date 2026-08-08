# Tarefa 3.0: Método SendTypingIndicator no gateway de onboarding

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar a operação de typing no gateway concreto de onboarding, delegando ao client Meta e classificando erros pelo `classifyError` existente. A interface pública `interfaces.WhatsAppGateway` e seus mocks mockery não são alterados (decisão da ADR-002).

<requirements>
- RF-04: erro classificado nos sentinelas de aplicação existentes para o consumer tratar como best-effort.
- RF-09: operação separada de `SendTextMessage`; nenhuma interface pública de onboarding modificada.
</requirements>

## Subtarefas

- [ ] 3.1 Adicionar `func (g *WhatsAppGateway) SendTypingIndicator(ctx context.Context, wamid string) error` em `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go`, delegando a `client.SendTypingIndicator` e usando `classifyError` com operação descritiva em pt-br.
- [ ] 3.2 Teste em `whatsapp_gateway_test.go`: delegação com sucesso e classificação de erro de client (`ErrMetaBadRequest` vira `ErrWhatsAppClientError`; `ErrMetaServer` vira `ErrWhatsAppServerError`).

## Detalhes de Implementação

Ver `techspec.md`, seção "Interfaces Chave", e ADR-002 (alternativa rejeitada: estender a interface pública de onboarding). Confirmar que `interfaces/whatsapp_gateway.go` permanece intacto ao final da tarefa.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/onboarding/infrastructure/gateway/...` verde.
- `git diff` sem alteração em `internal/onboarding/application/interfaces/whatsapp_gateway.go` nem em `internal/onboarding/application/interfaces/mocks/`.
- `go build ./...` e `go vet ./...` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (delegação e classificação de erro)
- [ ] Testes de integração (não aplicável: adapter puro sem fronteira própria; justificativa registrada)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go`
- `internal/onboarding/infrastructure/gateway/whatsapp_gateway_test.go`
