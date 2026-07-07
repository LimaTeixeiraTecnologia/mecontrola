# Generated: 2026-07-06T00:00:00Z

# Evidências RUN_REAL_LLM — Pendência Conversacional (task 8.0)

Data: 2026-07-06
Modelo: openai/gpt-4o-mini via OpenRouter
Flags: `RUN_REAL_LLM=1`, `-tags integration`
Comando: `go test -tags integration -run "TestRealLLM" -v -timeout 600s ./internal/agents/application/agents/`

## Resultado Global

```
Go test: 18 passed in 1 packages
```

Todos os 18 testes RealLLM passaram, incluindo os 5 novos testes de pendência e os 13 existentes.

## Cenários CA Validados

### CA-01 — Clarificação pede UMA pergunta (`TestRealLLM_PendingEntry_CA01_ClarifyAsksOneQuestion`)

- Mensagem: `"Gastei R$ 150,00 no mercado hoje no pix"`
- Tool `register_expense` retorna `outcome=clarify`
- Asserções:
  - `questionMarks <= 1`: PASS (agente faz no máximo uma pergunta)
  - `M-03=0 (não confirma sucesso sem write real)`: PASS
  - `M-02 (não repete valor/pagamento no slot de categoria)`: PASS

### CA-04 — Múltiplos candidatos lista legível (`TestRealLLM_PendingEntry_CA04_MultipleCandidates_ListaLegivel`)

- Tool `classify_category` retorna 3 candidatos (Plano de Saúde, Consultas e Exames, Terapia e Saúde Mental)
- Asserções:
  - Sem duplo asterisco: PASS
  - Lista numerada ou referência aos nomes: PASS

### CA-06 / G7-15 — Erro de ledger, resposta honesta (`TestRealLLM_PendingEntry_CA06_LedgerError_HonestResponse`)

- Tool `register_expense` retorna erro de persistência
- Asserções:
  - `M-03=0 (não afirma sucesso)`: PASS
  - Sem termos falsos de sucesso: PASS

### CA-12 — Formatação WhatsApp (duplo asterisco proibido) (`TestRealLLM_PendingEntry_CA12_DoubleAsteriskProibido`)

- Mensagem: `"Gastei R$ 320,00 no supermercado hoje no pix"`
- Tool retorna `outcome=clarify`
- Asserções:
  - `result.Content` não contém `"**"`: PASS

### NoInfra — Sem termos de infraestrutura na resposta (`TestRealLLM_PendingEntry_NoInfraInResponse`)

- Mensagem: `"Gastei R$ 200,00 no supermercado ontem no débito"`
- Asserções:
  - Sem `"workflow"`, `"pendência interna"`, `"correlação"`, `"thread_id"`, `"resource_id"`, `"correlation_key"`: PASS

## Cenários Existentes Validados

- `TestCA03_HonestConfirmation_ToolErrorNeverSuccessNorEmpty`: PASS — G7-15 / CA-03 / M-03=0
- `TestRealLLM_ToolCalling_RegisterExpense`: PASS — registro básico de despesa
- `TestRealLLM_ToolCalling_QueryMonth`: PASS — consulta de resumo mensal
- `TestRealLLM_Scorer_CategorizationLLMJudged`: PASS — scorer LLM-judged
- `TestRealLLM_OnboardingSummary_UsesWhatsAppFormattingAndEmojis`: PASS — formatação WhatsApp e emojis
- `TestRealLLM_ToolError_ProducesHonestResponse`: PASS — resposta honesta em falha de tool
- `TestRealLLM_CardPurchaseChain_ResolveClassifyRegister` (2 cenários): PASS — chain cartão de crédito
- `TestRealLLM_ClarifyClassifyRegisterChain`: PASS — cadeia clarify→classify→register

## Templates de Resposta Validados

Os templates implementados em `pending_entry_workflow.go`:

| Tipo | Template |
|------|----------|
| Sucesso (despesa) | `Despesa de R$ X,XX registrada em *Raiz > Folha* para hoje no pix ✅` |
| Sucesso (receita) | `Receita de R$ X,XX registrada em *Raiz > Folha* para hoje no pix ✅` |
| Confirmação | `Confirma? R$ X,XX em *Raiz > Folha* para hoje no pix?` |
| Cancelamento | `Tudo certo, o registro foi cancelado.` |
| Expiração | `O registro expirou. Para registrar, envie a informação completa novamente.` |
| Múltiplos candidatos | `Qual se encaixa melhor? 1. Raiz > Folha1 2. Raiz > Folha2` |

## Grep de Infraestrutura (8.3)

Nenhum termo de infraestrutura encontrado nas instruções do agente:

```bash
grep -rn "workflow_id|correlation_key|resource_id|thread_id" .../mecontrola_agent.go
# exit: 1 (vazio - nenhuma ocorrência)
```

## Métricas M-02 / M-03 / M-07

- `M-03=0`: Confirmado — agente nunca afirma sucesso sem write real (CA-01, CA-06, CA-12)
- `M-02`: Confirmado — agente não re-pergunta valor/pagamento já informados (CA-01)
- `M-07=0`: Validado em unit tests (tarefa 7.0) — harness pendente

## Arquivos Alterados

- `internal/agents/application/agents/mecontrola_agent.go` — instruções atualizadas (8.1 + 8.2)
- `internal/agents/application/workflows/pending_entry_workflow.go` — templates de resposta (8.2)
- `internal/agents/application/workflows/transactions_ledger_pending_test.go` — assert R$ 150,00
- `internal/agents/application/agents/mecontrola_agent_test.go` — 7 novos testes de instruções
- `internal/agents/application/agents/mecontrola_agent_e2e_test.go` — fix pre-existing nil arg
- `internal/agents/application/agents/pending_entry_realllm_test.go` — 5 testes RealLLM
