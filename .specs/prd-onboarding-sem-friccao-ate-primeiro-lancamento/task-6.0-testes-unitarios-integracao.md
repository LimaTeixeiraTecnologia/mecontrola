# Tarefa 6.0: Adicionar testes unitários e de integração obrigatórios

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar e/ou atualizar testes unitários e de integração que cubram primeira mensagem combinada, copy de categorias, cadastro/reuso/recusa/falha de cartão, pending-entry para pix sem cartão e persistência em `transactions`.

<requirements>
- RF-35: atualizar testes unitários do onboarding para primeira mensagem combinada, copy de categorias, cadastro/reuso/recusa de 💳 e falha de parsing de 💳.
- RF-36: cobrir `pending-entry` para pix sem 💳, garantindo que só `credit_card` exige cartão.
- RF-37: cobrir agente financeiro para receita simples com valor BRL usando separador de milhar, impedindo falso múltiplo lançamento.
- RF-38: cobrir integração do consumer WhatsApp para ordem de retomada e envio de resposta quando onboarding ou pending-entry tratam a mensagem.
- RF-39: incluir verificação de persistência em `transactions` para receita simples e despesa pix confirmadas.
</requirements>

## Subtarefas

- [ ] 6.1 Adicionar/atualizar testes unitários de onboarding para RF-35.
- [ ] 6.2 Adicionar testes unitários/integração de pending-entry para RF-36 e RF-39.
- [ ] 6.3 Adicionar testes de agente/tools para RF-37.
- [ ] 6.4 Adicionar testes de integração do consumer WhatsApp para RF-38.
- [ ] 6.5 Garantir que todos os testes rodem com `-race -count=1`.

## Detalhes de Implementação

Ver `techspec.md` — seção **Abordagem de Testes / Testes Unitários e Testes de Integração**. Usar os harnesses existentes; testes que exigem banco real devem manter build tag de integração conforme padrão local.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/agents/application/workflows/...` passa.
- `go test -race -count=1 ./internal/agents/application/agents/...` passa.
- `go test -race -count=1 ./internal/agents/application/tools/...` passa.
- Testes de integração do consumer cobrem retomada de onboarding e `pending-entry`.
- Testes verificam persistência em `transactions` para pix e receita confirmadas.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — testes no consumidor agentivo, workflows, tools e guards.

## Testes da Tarefa

- [ ] Testes unitários.
- [ ] Testes de integração.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go`
- `internal/agents/application/workflows/pending_entry_workflow_test.go`
- `internal/agents/application/agents/guards/card_provenance_test.go`
- `internal/agents/application/tools/financial_tools_test.go`
- `internal/agents/application/tools/register_expense_integration_test.go`
