# Tarefa 7.0: Harness Determinístico — G7+G10+G12 + Integração Postgres (CA-01..CA-17)

<critical>Ler prd.md, techspec.md e scenarios.md desta pasta — tarefa invalidada se pulado</critical>

## Visão Geral

Implementar o harness determinístico como gate primário de aceite funcional (RF-33, RF-34, CA-12). Cobre os grupos G7 (controle de estado, 20 cenários), G10 (gates de segurança, 4 cenários) e G12 (confirmação/edição/seleção, 6 cenários) do `scenarios.md` com doubles de `CategoriesReader`, `TransactionsLedger`, `CardManager` e workflow store em memória. Implementar testes de integração Postgres validando `workflow_runs`, `workflow_steps`, `agents_write_ledger` e `transactions` com banco real. Grupos G1–G6 e G8–G9 são cobertos por testes unitários table-driven de decisão nesta tarefa. O turno de confirmação obrigatório (spec-version 3 do scenarios.md) é asserido em todo cenário com escrita (CA-13..CA-17).

<requirements>
- Harness determinístico: provider fake para mensagens do agente; doubles tipados de CategoriesReader, TransactionsLedger, CardManager, workflow store; sem rede real, sem OpenRouter
- Toda assertiva de harness deve verificar: estado final (PendingStatus), tool calls (quais foram invocadas e com qual ToolOutcome), resposta ao usuário, presença/ausência de escrita real, RunStatus, ToolOutcome, workflow_runs e agents_write_ledger, não-duplicidade
- Cenários G7-01..G7-20 cobrindo: substituição (G7-01, G7-02), raiz sem folha (G7-03), cancelamentos (G7-04..G7-06), "sim e pix" incompatível (G7-07), expiração (G7-08), replay (G7-09), múltiplos candidatos (G7-10, G7-11), correção de descrição (G7-12), ambiguidade (G7-13, G7-14), erro de ledger (G7-15), cartão (G7-16, G7-17), pagamento pendente (G7-18), data pendente (G7-19), fluxo completo com confirmação (G7-20)
- Cenários G10-01..G10-04: raiz sem folha income, ID LLM inválido rejeitado, sucesso simulado proibido, dados preservados (M-02=100%)
- Cenários G12-01..G12-06: confirmação obrigatória no caminho inequívoco (CA-13), recusa no gate (CA-05), confirmação ambígua com reprompt único (CA-14), seleção por número ou nome (CA-15), recorrência via CreateRecurringTemplate com confirmação (CA-16), edição via UpdateTransaction respeitando TargetVersion (CA-17)
- Confirmação obrigatória (spec-version 3): todo cenário com escrita deve assertar AwaitingSlot=confirmation e `expectConfirmationBeforeWrite=true` antes de write; M-07=0 em todos os cenários
- Testes de integração Postgres (build tag integration): workflow_runs, workflow_steps, agents_write_ledger, transactions; expiração com SuspendedAt antigo; substituição sem escrita da pendência antiga
- Grupos G1–G6: unit tests table-driven de decisão com double — um test case por subcategoria representativa, cobrindo candidato único, resolve direto, resposta curta, múltiplos candidatos
- Zero falso positivo: M-01=100%, M-02=100%, M-03=0, M-04=0, M-06=0, M-07=0 em todos os cenários do harness
</requirements>

## Subtarefas

- [ ] 7.1 Criar doubles/fakes: `fakeCategoriesReader`, `fakeTransactionsLedger`, `fakeCardManager`, `inMemoryWorkflowStore` em `internal/agents/application/` (ou pasta de testes compartilhada)
- [ ] 7.2 Implementar harness base: runner que aceita sequência de turnos `{actor, text, messageId}` e asserções `{expectPendingStatus, expectAwaitingSlot, expectWrite, expectNoWrite, expectToolCalls, expectRunStatus, expectNoRepeat}`
- [ ] 7.3 Implementar cenários G7-01..G7-10 no harness (substituição, raiz sem folha, cancelamentos, expiração, replay, múltiplos candidatos) — incluindo turno de confirmação implícito onde write é esperado
- [ ] 7.4 Implementar cenários G7-11..G7-20 no harness (ResolveForWrite falha, correção de descrição, ambiguidade, erro de ledger, cartão, pagamento, data, fluxo completo com confirmação)
- [ ] 7.5 Implementar cenários G10-01..G10-04 no harness (raiz sem folha income, ID LLM inválido, sucesso simulado proibido, dados preservados)
- [ ] 7.5b Implementar cenários G12-01..G12-06 no harness (CA-13 confirmação no caminho inequívoco, CA-05 recusa no gate, CA-14 confirmação ambígua → reprompt único → cancela, CA-15 seleção por número e por nome resolvem o mesmo par, CA-16 recorrência via CreateRecurringTemplate com confirmação, CA-17 edição via UpdateTransaction respeitando TargetVersion) com `expectConfirmationBeforeWrite=true`
- [ ] 7.6 Implementar testes de integração Postgres (`integration` build tag): start real → resume real → write real em `transactions`; expiração com snapshot antigo; substituição sem escrita da pendência antiga; replay idempotente com banco
- [ ] 7.7 Implementar unit tests table-driven G1–G6: cobrir ao menos 1 cenário por subcategoria raiz (custo-fixo, conhecimento, prazeres, metas, liberdade-financeira para expense; salario, renda-variavel, investimentos para income) — verificar que DecidePendingResume + DecideCategoryChoice + ResolveForWrite produzem o par root+sub correto

## Detalhes de Implementação

Ver `techspec.md` seção **"Harness Determinístico"** e `scenarios.md` **Grupo 11** (payloads de harness) e **"Convenção Global de Confirmação (spec-version 3)"**.

Payload mínimo por cenário do harness (Grupo 11 do scenarios.md):

```json
{
  "scenario": "G7-20",
  "turns": [
    { "actor": "user", "text": "Gastei R$ 150,00 no mercado hoje, no pix", "messageId": "wamid-001" },
    { "actor": "agent", "expectPendingStatus": "active", "expectAwaitingSlot": "category" },
    { "actor": "user", "text": "supermercado", "messageId": "wamid-002" },
    { "actor": "agent", "expectPendingStatus": "active", "expectAwaitingSlot": "confirmation" },
    { "actor": "user", "text": "sim", "messageId": "wamid-003" },
    { "actor": "agent",
      "expectWrite": { "amountCents": 15000, "paymentMethod": "pix",
        "rootCategoryId": "66cb85a0-...", "rootSlug": "custo-fixo",
        "subcategoryId": "97fa4b86-...", "subcategorySlug": "supermercado",
        "categorySource": "user_selected_candidate" },
      "expectPendingStatus": "completed", "expectRunStatus": "succeeded",
      "expectNoRepeat": ["amountCents", "paymentMethod"] }
  ]
}
```

`expectNoRepeat`: campos que não devem ser re-perguntados ao usuário após o primeiro turno — validado verificando que nenhuma mensagem do agente nos turnos posteriores solicita esses slots novamente.

Confirmação obrigatória (spec-version 3): inserir turno `{actor: "agent", expectAwaitingSlot: "confirmation"}` entre fechamento do último slot e `expectWrite` em todo cenário com escrita. Sem esse turno no harness, a assertiva falhará com "write sem confirmação prévia — M-07 > 0".

Integração Postgres: usar build tag `//go:build integration` e banco de testes (variável `TEST_DATABASE_URL`). Verificar tabelas `workflow_runs`, `workflow_steps` com `status='succeeded'` após write; `transactions` com `category_source='user_selected_candidate'`; `agents_write_ledger` com `origin_operation='pending_entry_register'`.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/agents/...` verde (unitários + harness sem banco)
- `go test -race -count=1 -tags integration ./internal/agents/...` verde (com banco real)
- Todos os 30 cenários de harness (G7-01..G7-20 + G10-01..G10-04 + G12-01..G12-06) passam
- M-01=100%: todos os turnos de resposta curta corretamente associados à pendência
- M-02=100%: zero re-pergunta de valor/pagamento/descrição já informados
- M-03=0: zero resposta de sucesso sem write real comprovado
- M-04=0: zero escrita sem par root+sub canônico validado por ResolveForWrite
- M-06=0: zero resposta aplicada a pendência errada
- M-07=0: zero escrita sem turno de confirmação prévia (spec-version 3)
- CA-01..CA-17: todos passam no harness com assertivas de Run auditável

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — harness determinístico valida o substrato agent da plataforma (Thread→Run→workflow→write); doubles de CategoriesReader/TransactionsLedger/CardManager são contratos do consumidor internal/agents

## Testes da Tarefa

- [ ] G7-01..G7-20: 20 cenários de harness cobrindo todos os estados de pendência
- [ ] G10-01..G10-04: 4 cenários de gates de segurança
- [ ] G12-01..G12-06: 6 cenários de confirmação/edição/seleção/recorrência (CA-13, CA-05, CA-14, CA-15, CA-16, CA-17)
- [ ] G1–G6 representativos: ao menos 8 cenários de unit test (1 por categoria raiz de expense + 3 de income) validando par root+sub canônico
- [ ] Integração Postgres: start→resume→write→transactions; expiração; substituição; replay idempotente
- [ ] CA-01 através CA-17: cada critério de aceite do PRD passa como caso de harness

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/agents/pending_entry_harness_test.go` (novo — harness G7+G10)
- `internal/agents/application/agents/pending_entry_decision_g1g6_test.go` (novo — unit tests G1–G6)
- `internal/agents/infrastructure/binding/pending_entry_integration_test.go` (novo — integração Postgres)
- `internal/agents/application/workflows/pending_entry_state.go` (de 1.0)
- `internal/agents/application/workflows/pending_entry_decisions.go` (de 1.0)
- `internal/agents/application/workflows/pending_entry_workflow.go` (de 2.0)
- `internal/agents/application/usecases/pending_entry_continuer.go` (de 6.0)
- `.specs/prd-conversa-agentiva-fluida/techspec.md` (seção "Harness Determinístico", "Gates de Validação")
- `.specs/prd-conversa-agentiva-fluida/scenarios.md` (Grupo 7, Grupo 10, Grupo 11, Grupo 12, Convenção Global de Confirmação)
