# Tarefa 5.0: Golden real-LLM de edição de receita + gate

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a suíte golden real-LLM que prova a edição conversacional de receita em linguagem natural: localizar por valor/termo, desambiguar múltiplos candidatos, mostrar nota de impacto, atualizar só após confirmação, distinguir valor novo de critério de busca, mudar data (reusando a resolução determinística da tarefa 1.0), e tratar ausência de candidato / correção inválida. Registrar no `registry.go` e validar o gate ≥ 0,90, 0 falso-sucesso.

<requirements>
- RF-17: reconhecer intenção de editar receita em linguagem natural.
- RF-18: localizar receitas compatíveis por valor/termo quando o id é desconhecido.
- RF-19: >1 candidato apresenta opções; 1 candidato apresenta para confirmação.
- RF-20: atualizar só após confirmação; mostrar nota de impacto.
- RF-21: distinguir valor novo (amountCents) de critério de busca (searchAmountCents).
- RF-23: sucesso de edição oficial; idempotência por wamid/itemSeq.
- RF-24: nenhum candidato → informa sem alterar; correção inválida → não persiste e orienta.
</requirements>

## Subtarefas

- [x] 5.1 Adicionar casos de edição ao arquivo golden (frases: `corrige aquela receita`, `era 150`, `na verdade foi 180`, `recebi mais/menos`, `troca o valor`, `muda a data`, `foi semana passada`).
- [x] 5.2 Casos de desambiguação (mais de um candidato) e de candidato único (nota de impacto + confirmação).
- [x] 5.3 Casos de mudança de data via resolução determinística (`foi semana passada`) assertando data ≠ dia corrente.
- [x] 5.4 Casos de erro: nenhum candidato compatível (sem alteração) e correção inválida (não persiste).
- [x] 5.5 Registrar no `registry.go`; executar gate real-LLM (`RUN_REAL_LLM=1`) até ≥0,90, 0 falso-sucesso.

## Detalhes de Implementação

Ver `techspec.md` seção "Abordagem de Testes → Testes E2E" e ADR-003. A ferramenta `edit_entry` já distingue valor novo (`amountCents`) de critério de busca (`searchAmountCents`) no schema (edit_entry.go:43,50); os casos devem provar essa distinção e o gate de confirmação. Zero comentários em Go de produção.

## Critérios de Sucesso

- Gate golden real-LLM de edição ≥ 0,90, 0 falso-sucesso.
- Casos cobrem localização, desambiguação, mudança de data, confirmação e erros.
- `go build`/`go vet` verdes; suíte compila e registra sem órfãos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — golden cases/scorers e harness real-LLM do agente sobre internal/agents (edição conversacional).

## Testes da Tarefa

- [x] Testes unitários (registro dos casos; compilação da suíte)
- [x] Testes de integração (gate real-LLM `RUN_REAL_LLM=1` — prova E2E de edição)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/golden/cases_income_natural_language.go` (compartilhado com 4.0) ou `cases_income_edit.go`
- `internal/agents/application/golden/registry.go`
- `internal/agents/application/tools/edit_entry.go` (referência do schema)
- `internal/agents/application/golden/harness_realllm_test.go`
