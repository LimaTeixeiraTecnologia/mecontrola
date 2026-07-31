# Tarefa 4.0: Golden real-LLM de lançamento (grupos + valor + data) + gate

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a suíte golden real-LLM que prova a compreensão ampla de lançamento de receita: uma ocorrência por grupo de intenção (nove grupos), formas de valor (numérico/BR/gíria já suportados + por-extenso simples e composto) e datas novas (`semana passada`, `mês passado`, `dia 10`). Preservar os casos-armadilha de produção. Registrar no `registry.go` e validar o gate ≥ 0,90 por grupo, 0 falso-sucesso.

<requirements>
- RF-01: reconhecer intenção de receita nas nove famílias.
- RF-02: origem como termo literal.
- RF-03: valor numérico/BR/gíria.
- RF-04: valor por-extenso simples e composto.
- RF-08: pedir apenas o dado ausente com pergunta oficial.
- RF-09: clarify de categoria/subcategoria de receita (comportamento existente) provado.
- RF-11: confirmação obrigatória antes de gravar; grava 1x no "sim".
- RF-12: mensagem de sucesso oficial de receita.
- RF-13: idempotência por wamid/itemSeq (replay).
- RF-14: valor inválido/origem vazia não grava.
- RF-15: receita não captura forma de pagamento (canal reconhecido, não persistido).
- RF-25: ≥1 caso golden por grupo + armadilhas; ≥0,90, 0 falso-sucesso.
- RF-27: métricas com cardinalidade controlada (sem user_id/category_id como label) — gate de verificação.
</requirements>

## Subtarefas

- [ ] 4.1 Criar `cases_income_natural_language.go` com ≥1 caso por grupo (recebimentos, trabalho, autônomos, comércio, produção artesanal, comissões/bônus, salário, aluguéis, investimentos).
- [ ] 4.2 Adicionar casos de valor (por-extenso simples/composto, gíria) e de data nova (`semana passada`/`mês passado`/`dia 10`) assertando data resolvida ≠ dia corrente.
- [ ] 4.3 Adicionar casos de confirmação (sucesso oficial), replay (idempotência), valor inválido e canal reconhecido sem persistir pagamento; preservar armadilhas (falso múltiplo, paráfrase de origem).
- [ ] 4.4 Registrar os casos no `registry.go`; executar o gate real-LLM (`RUN_REAL_LLM=1`) e ajustar até ≥0,90 por grupo, 0 falso-sucesso.
- [ ] 4.5 Executar o gate de cardinalidade de métricas (grep R-TXN-004) confirmando ausência de `user_id`/`category_id` como label.

## Detalhes de Implementação

Ver `techspec.md` seção "Abordagem de Testes → Testes E2E" e ADR-003 (`adr-003-gate-golden-por-grupo.md`). Seguir o padrão de `cases_expense_income.go` (Case com ToolSubset/ExpectedTool/ExpectedArgs/ResponseProperty). Zero comentários em Go de produção; casos golden são código de teste versionado. Validação com mocks não substitui o gate real-LLM (política do projeto).

## Critérios de Sucesso

- Gate golden real-LLM ≥ 0,90 por grupo, 0 falso-sucesso; armadilhas verdes.
- Cardinalidade de métricas confirmada sem labels proibidos.
- `go build`/`go vet` verdes; suíte golden compila e registra sem órfãos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — golden cases/scorers e harness real-LLM do agente sobre internal/agents (evals do MeControlaAgent).

## Testes da Tarefa

- [ ] Testes unitários (registro dos casos; compilação da suíte)
- [ ] Testes de integração (gate real-LLM `RUN_REAL_LLM=1` — prova E2E de comprensão)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/golden/cases_income_natural_language.go` (novo)
- `internal/agents/application/golden/registry.go`
- `internal/agents/application/golden/cases_expense_income.go` (referência de padrão)
- `internal/agents/application/golden/harness_realllm_test.go`
