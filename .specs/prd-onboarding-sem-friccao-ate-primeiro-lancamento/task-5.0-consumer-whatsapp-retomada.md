# Tarefa 5.0: Atualizar consumer WhatsApp e prioridade de retomada

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Preservar e, quando necessário, ajustar a ordem de retomada do consumer WhatsApp para que onboarding, `pending-entry`, confirmação destrutiva e criação de cartão sejam priorizados antes do agente geral, mantendo a identidade do usuário autenticado pelo próprio WhatsApp.

<requirements>
- RF-28: cadeia de retomada do WhatsApp prioriza pendências antes do agente geral, especialmente `pending-entry`, confirmação destrutiva, criação de 💳, criação de orçamento e onboarding.
- RF-29: correções preservam identidade inbound do usuário ativo e autenticado pelo próprio WhatsApp.
- RF-30: fluxo não cria novo canal, novo endpoint HTTP público nem nova política de autorização.
</requirements>

## Subtarefas

- [ ] 5.1 Revisar a ordem de prioridade no consumer WhatsApp inbound.
- [ ] 5.2 Validar que `pending-entry` ativo é retomado antes do agente geral.
- [ ] 5.3 Validar que onboarding suspenso é retomado quando aplicável.
- [ ] 5.4 Garantir que resposta do onboarding/pending-entry seja enviada como uma única mensagem outbound.
- [ ] 5.5 Preservar autenticação e identidade do usuário sem novo endpoint ou política.

## Detalhes de Implementação

Ver `techspec.md` — seções **Pontos de Integração / WhatsApp inbound** e **Testes de Integração**. O consumer em `whatsapp_inbound_consumer.go:190` já prioriza retomadas; a tarefa foca em testes de integração e possíveis ajustes mínimos para garantir o comportamento com as novas etapas.

## Critérios de Sucesso

- Teste de integração confirma que consumer retoma onboarding e envia primeira resposta combinada como uma única mensagem.
- Teste de integração confirma que consumer retoma `pending-entry` antes do agente geral quando há pendência ativa.
- Teste de integração confirma que fluxo de cartão no onboarding cria um único cartão ativo e não entra em loop.
- `go test -race -count=1 ./internal/agents/...` passa.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — consumer de inbound WhatsApp e retomada de workflow/agente.

## Testes da Tarefa

- [ ] Testes de integração do consumer WhatsApp.
- [ ] Testes de prioridade de retomada.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`
