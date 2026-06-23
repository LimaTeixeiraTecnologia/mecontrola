# Tarefa 9.0: [agent] Tools + scripts — closing_day, objective_profile, copy literal

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ajustar, em `internal/agent`, o catálogo de tools e os scripts de onboarding: tool
`save_onboarding_card` passa a exigir `closing_day` (1..31); tool `save_onboarding_objective` ganha
enum opcional `objective_profile` (preenchido pelo LLM no parse, forwardado ao onboarding); copy dos
scripts atualizado ("dia de fechamento") e contrato literal das mensagens (5 categorias, progresso,
resumo enxuto, 1ª transação).

<requirements>
- RF-09: sem confirmações supérfluas. RF-10: 5 categorias em 1 mensagem. RF-11: indicador de progresso.
- RF-12: tool de cartão por `closing_day`. RF-16: resumo enxuto. RF-17: não encerrar → 1ª transação.
- RF-18: priorizar atrito/ativação.
- ADR-004 (enum objective_profile no parse), ADR-005 (closing_day).
</requirements>

## Subtarefas

- [ ] 9.1 Tool `save_onboarding_card`: `required: [nickname, closing_day]` (1..31), remover `due_day`.
- [ ] 9.2 Tool `save_onboarding_objective`: enum opcional `objective_profile` (`payoff_debt|emergency_fund|invest|specific_goal|organize_spending`).
- [ ] 9.3 Scripts: `scriptCardQuestion`/`scriptCards` "dia de fechamento"; revisar copy das 5 categorias, progresso, resumo e `scriptFirstTx` conforme contrato.
- [ ] 9.4 Dispatcher: ler `closing_day`; resposta "fecha dia %d".
- [ ] 9.5 Testes (schema das tools; fidelidade de narração contra constantes).

## Detalhes de Implementação

Ver techspec.md → "Scripts e Copy (contrato — RF-09/10/11/16/17)" e "Cartão por dia de fechamento".
Tool fina (R-AGENT-WF-001.2); LLM só no parse.

## Critérios de Sucesso

- Tool de cartão não aceita mais `due_day`; objetivo aceita o enum opcional.
- Copy bate com o contrato (5 categorias/progresso/resumo); sem perguntas de confirmação supérfluas.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — altera tool catalog/Descriptor e copy do agente de onboarding (padrão Workflow/Tool).

go-implementation (linguagem, auto) e agent-governance (governança, auto) também se aplicam.

## Testes da Tarefa

- [ ] Testes unitários (schema/required das tools; mapeamento closing_day; fidelidade de copy)
- [ ] Testes de integração (T12 — diálogo e2e com fidelidade)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] Tools e scripts atualizados; `due_day` ausente do onboarding.
- [ ] Zero comentários no `.go`; sem regra/SQL nas tools.
- [ ] `go build ./internal/agent/...` e `go test ./internal/agent/application/usecases/... -run Onboarding` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/agent/... && \
go test ./internal/agent/... -run "Onboarding|Script|Tool" -count=1
grep -rn "due_day\|vencimento" internal/agent/application/usecases/onboarding_tool_catalog.go internal/agent/application/usecases/onboarding_scripts.go && echo "REVISAR" || echo OK
```

## Arquivos Relevantes
- `internal/agent/application/usecases/onboarding_tool_catalog.go` (modificado)
- `internal/agent/application/usecases/onboarding_scripts.go` (modificado)
- `internal/agent/infrastructure/onboarding/onboarding_tool_dispatcher.go` (modificado)
