# Suíte Production-Proof — Validação Completa Pós-Wipe (WhatsApp)

Data: 2026-07-29 (pós-wipe do banco + deploy sha `7959ec5`, run 30485218565)
Número de teste: `+5511986896322` (usuário será recriado no onboarding)
Substitui e consolida: `suite-fogo-real-fluxos-nao-cobertos.md` + incidentes de `resultados-e2e-agente-2026-07-28.txt` (RODADAs 1-23)

## Como usar

1. Envie as frases EXATAS, na ordem indicada na coluna "Você envia", aguardando a resposta do bot entre elas.
2. "Resposta variável" = valide a SEMÂNTICA (intenção, dados, valores), não o texto exato.
3. O `user_id` mudou após o wipe. Resolva uma vez por sessão de validação:

```bash
ssh mecontrola-vps "docker exec -i mecontrola_postgres.1.uvxevski7nrklemzf2gnusg75 psql -U mecontrola -d mecontrola_db -t -A -c \"SELECT id FROM mecontrola.users WHERE whatsapp_number='+5511986896322';\""
```

Todas as queries abaixo usam `'<USER_ID>'` — substitua pelo id resolvido.

4. **FAIL imediato global (0 falso positivo)**: bot diz "Prontinho! ✅"/"atualizei"/"removi" sem efeito no banco; valor/categoria/cartão/dia diferente do confirmado; mensagem duplicada; pergunta de cartão em pagamento que não é crédito; a palavra "cartão" substituída por emoji 💳 no texto do bot.
5. Registre cada resultado na tabela final + evidência (run id, query).

---

## Bloco 1 — Onboarding (banco zerado)

### 1.1 Fluxo completo informando cartão (fluxo REAL do onboarding)

O onboarding NÃO tem etapa de recorrência de despesas — a pergunta de recorrência do onboarding é sobre repetir o **orçamento** todo mês, não sobre criar templates de lançamento. Templates de recorrência são criados depois, pelo agente principal (cenários 4.x).

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `oi` | Boas-vindas + pergunta do nome de tratamento |
| 2 | `Jailton` | Pergunta do objetivo financeiro |
| 3 | `economizar para uma viagem` | Pergunta do orçamento mensal |
| 4 | `4000` | Proposta de distribuição do orçamento por categoria |
| 5 | `sim` | Pergunta: repetir o orçamento automaticamente todo mês? |
| 6 | `sim` | Pergunta sobre cartões de crédito |
| 7 | `nubank vencimento dia 10` | "💳 Cartão registrado com sucesso ✅ Quer registrar algum outro?" |
| 8 | `não` | **Resumo final de onboarding** (objetivo, orçamento, distribuição, cartões, recorrência do orçamento) |

Checks:

```sql
SELECT id, whatsapp_number, display_name, status FROM mecontrola.users WHERE whatsapp_number='+5511986896322';
SELECT competence, total_cents, state FROM mecontrola.budgets WHERE user_id='<USER_ID>';
SELECT root_slug, planned_cents FROM mecontrola.budgets_allocations WHERE budget_id IN (SELECT id FROM mecontrola.budgets WHERE user_id='<USER_ID>');
SELECT nickname, due_day, closing_day FROM mecontrola.cards WHERE user_id='<USER_ID>';
SELECT role, left(content,80) FROM mecontrola.platform_messages WHERE resource_id='<USER_ID>' ORDER BY created_at DESC LIMIT 3;
```

PASS: usuário ACTIVE; budget 400000 cents com alocações somando o total; cartão nubank due_day=10; **resumo final presente em platform_messages como role=assistant** (fix RODADA 25 — antes a FinalMessage não era persistida); NENHUM template em transactions_recurring_templates neste ponto. FAIL: loop de etapa, dado errado, onboarding concluído sem dados, resumo final ausente.

---

## Bloco 2 — Despesas sem cartão (pix, vale, débito, dinheiro)

### 2.1 Pix com categoria automática (sem nenhuma pergunta de cartão)

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 45 na farmácia no pix` | Confirmação: R$ 45,00, categoria resolvida (Medicamentos e Farmácia ou equivalente), pagamento pix. **NUNCA perguntar cartão** |
| 2 | `sim` | "Prontinho! ✅" |

Check:

```sql
SELECT description, amount_cents, payment_method, subcategory_name_snapshot, occurred_at::date
FROM mecontrola.transactions WHERE user_id='<USER_ID>' ORDER BY created_at DESC LIMIT 1;
```

PASS: 4500 cents, pix, categoria coerente, data de hoje. FAIL: perguntou cartão; valor/categoria errados.

### 2.2 Vale-refeição com categoria automática

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 30 no almoço no vale` | Confirmação: R$ 30,00, Prazeres > Restaurantes (ou listagem — responder o número), vale-refeição |
| 2 | `sim` | "Prontinho! ✅" |

Check: mesma query de 2.1 — payment_method=`vale_refeicao`, 3000 cents.

### 2.3 Categoria desconhecida → listagem numerada → recusa

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 77 em teste recusa` | Lista de categorias raiz numerada |
| 2 | `2` | Lista de subcategorias de Custo Fixo |
| 3 | `1` | Pergunta forma de pagamento (ou confirmação direta) |
| 4 | `pix` | Confirmação: R$ 77,00, Custo Fixo > Açougue, pix |
| 5 | `não` | "Tudo certo, o registro foi cancelado." |

PASS: **zero** transação de 7700 no banco. FAIL: persistiu após "não".

### 2.4 Reprompt de confirmação ("talvez")

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 33 em teste reprompt` | Lista de categorias |
| 2 | `2` | Lista de subcategorias |
| 3 | `2` | Pergunta forma de pagamento |
| 4 | `pix` | Confirmação: R$ 33,00, Custo Fixo > Água, pix |
| 5 | `talvez` | "Não entendi. Por favor, responda apenas sim ou não para confirmar." |
| 6 | `talvez` | Reprompt OU cancelamento — registrar o que ocorreu: ____ |
| 7 | `cancelar` | "Tudo certo, o registro foi cancelado." |

PASS: zero transação de 3300. FAIL: registrou; ou aceitou "talvez" como confirmação.

### 2.5 Cancelar na etapa de categoria

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 99 em teste cancelamento` | Lista de categorias |
| 2 | `cancelar` | "Tudo certo, o registro foi cancelado." |

PASS: zero transação de 9900.

### 2.6 Multi-item na mesma mensagem

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 30 no ônibus e 15 no café` | "Percebi mais de um lançamento na mesma mensagem... registro um de cada vez" |

PASS: **nenhum** lançamento criado. FAIL: registrou 1 ou 2 lançamentos.

### 2.7 Débito e dinheiro

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `paguei 35 no dentista no débito` | Confirmação: R$ 35,00, Odontologia (ou listagem), débito |
| 2 | `sim` | "Prontinho! ✅" |
| 3 | `gastei 20 na padaria em dinheiro` | Confirmação (ou listagem de categoria — responder) |
| 4 | `sim` | "Prontinho! ✅" |

Check: payment_method `debit_card` (3500) e `cash` (2000).

---

## Bloco 3 — Crédito e cartões

### 3.1 Crédito com apelido na frase (atalho determinístico)

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 45 no cartão nubank` | Confirmação direta: R$ 45,00, categoria, crédito — **sem perguntar apelido** (ou listagem de categoria — responder) |
| 2 | `sim` | "Prontinho! ✅" |

Check: 4500 cents, payment_method=`credit_card`, card_id do nubank.

### 3.2 Crédito SEM apelido → pergunta com texto correto

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `comprei 50 no crédito` | "Qual é o apelido do cartão que você usou?" (ou listagem de categoria antes/depois) |
| 2 | `nubank` | Confirmação (ou listagem de categoria — responder) |
| 3 | `sim` | "Prontinho! ✅" |

**Exigência dura**: a pergunta contém a palavra "cartão" por extenso. FAIL se 💳 substituir a palavra (ex.: "preciso saber qual 💳 você quer usar").

### 3.3 Parcelado + fatura por mês

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `quanto está minha fatura do cartão nubank?` | Valor baseline (anotar: ____) |
| 2 | `comprei 600 de eletrônico em 3x no crédito` | Confirmação (apelido/categoria se pedir: `nubank` / número) |
| 3 | `sim` | "Prontinho! ✅" |
| 4 | `quanto está minha fatura do cartão nubank?` | Baseline + **R$ 200,00** (não R$ 600,00) |

Checks:

```sql
SELECT id, amount_cents, installments_total, card_id FROM mecontrola.transactions
WHERE user_id='<USER_ID>' ORDER BY created_at DESC LIMIT 1;
SELECT i.ref_month, it.installment_index, it.amount_cents
FROM mecontrola.transactions_card_invoice_items it
JOIN mecontrola.transactions_card_invoices i ON i.id=it.invoice_id
WHERE it.transaction_id='<ID_DA_TRANSACAO>' ORDER BY it.installment_index;
```

PASS: 1 transação 60000 com installments_total=3; 3 itens de 20000 em ref_months consecutivos. FAIL: fatura subiu 60000 de uma vez.

### 3.4 Listar cartões / atualizar vencimento

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `quais cartões eu tenho?` | Lista com nubank (sem cartão alucinado) |
| 2 | `muda o vencimento do nubank para dia 15` | Confirmação: 10 → 15 |
| 3 | `sim` | Confirmação de atualização |

Check: `SELECT nickname, due_day FROM mecontrola.cards WHERE user_id='<USER_ID>';` → due_day=15 só após confirmação.

---

## Bloco 4 — Receitas

### 4.1 Receita avulsa

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `recebi 500 de freelance` | Confirmação: R$ 500,00, Origem: freelance |
| 2 | `sim` | Mensagem de receita registrada |

Check:

```sql
SELECT direction, payment_method, amount_cents, description FROM mecontrola.transactions
WHERE user_id='<USER_ID>' AND direction=1 ORDER BY created_at DESC LIMIT 1;
```

(direction de income: confirmar valor na 1ª execução — registrar: ____)

### 4.2 Receita recorrente — frase exata do incidente R$ 5,00

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `todo dia 5 eu recebo R$ 13.874,40 de salário` | Confirmação de **recorrência** DIRETA: **R$ 13.874,40**, dia 5, salário — NUNCA R$ 5,00; NÃO pergunta forma de pagamento (income usa pix default) |
| 2 | `sim` | Confirmação de recorrência criada |

Check:

```sql
SELECT description, amount_cents, day_of_month, direction FROM mecontrola.transactions_recurring_templates
WHERE user_id='<USER_ID>' ORDER BY created_at DESC LIMIT 1;
```

PASS: 1387440 cents, dia 5, income, resolução determinística, payment_method='pix' (default de income). FAIL: R$ 5,00; lançamento avulso; pergunta de valor ou de pagamento.

### 4.3 Receita recorrente sem dia ("todo mês" — slot determinístico recurrence_day)

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `todo mês eu recebo 800 de pensão` | Pergunta o dia do mês: "Em qual dia do mês ele se repete? 📅 (ex.: 5)" (slot determinístico; NÃO confirma lançamento avulso; dias 1–31 aceitos — 29/30/31 materializam no último dia em meses curtos) |
| 2 | `dia 10` | Confirmação de recorrência: R$ 800,00, dia 10 |
| 3 | `sim` | Recorrência criada |

PASS: template 80000 dia 10. FAIL: lançamento avulso de R$ 800 em `transactions`; dia assumido sem perguntar; reprompt infinito ao responder `dia 10`.

### 4.4 Despesa recorrente com pagamento na frase (aluguel — 1ª criação)

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `todo dia 5 pago 1500 de aluguel no pix` | Confirmação de recorrência: R$ 1.500,00, Custo Fixo > Aluguel, dia 5, pix |
| 2 | `sim` | Recorrência criada |

```sql
SELECT description, amount_cents, day_of_month, direction, payment_method FROM mecontrola.transactions_recurring_templates
WHERE user_id='<USER_ID>' AND description ILIKE '%aluguel%' AND deleted_at IS NULL;
```

PASS: 1 template, 150000 cents, dia 5, expense, pix. FAIL: "Não consegui registrar"; lançamento avulso; template sem dia.

### 4.5 Despesa recorrente SEM pagamento na frase (novo caminho pós-fix)

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `todo dia 10 pago 200 de internet` | Pergunta a forma de pagamento (slot; NÃO falha com "Não consegui registrar") |
| 2 | `pix` | Confirmação de recorrência: R$ 200,00, dia 10, pix |
| 3 | `sim` | Recorrência criada |

```sql
SELECT description, amount_cents, day_of_month, payment_method FROM mecontrola.transactions_recurring_templates
WHERE user_id='<USER_ID>' AND description ILIKE '%internet%' AND deleted_at IS NULL;
```

PASS: 20000 cents, dia 10, pix. FAIL: invoke da tool falhar por `paymentMethod` obrigatório; fallback "Não consegui registrar. Tente novamente em breve."

---

## Bloco 5 — Edição, correção e exclusão de lançamentos

### 5.1 Correção de valor com desambiguação

Preparação:

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 30 no uber no pix` | Confirmação (categoria se pedir) |
| 2 | `sim` | Registrado |
| 3 | `gastei 30 no uber no pix` | Confirmação (categoria se pedir) |
| 4 | `sim` | Registrado (2º uber) |

Cenário:

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `edita o lançamento do uber` | "Qual alteração você gostaria de fazer...?" |
| 2 | `quero alterar o valor para 35` | Lista 2 candidatos numerados |
| 3 | `1` | Confirmação: atual R$ 30,00 → novo R$ 35,00 |
| 4 | `sim` | "Prontinho, atualizei! ✅" |

```sql
SELECT id, description, amount_cents, version, deleted_at FROM mecontrola.transactions
WHERE user_id='<USER_ID>' AND description ILIKE '%uber%' ORDER BY created_at;
```

PASS: MESMO id com 3500 e version+1; o outro uber intacto com 3000. FAIL: criou lançamento novo.

### 5.2 Correção por frase completa ("o valor certo é X e não Y")

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `no lançamento da farmácia o valor certo é 50 e não 45` | Candidato(s) ou confirmação direta 45 → 50 |
| 2 | `1` (se listar) | Confirmação |
| 3 | `sim` | "Prontinho, atualizei! ✅" |

PASS: farmácia 4500 → 5000, mesmo id.

### 5.3 Edição de lançamento inexistente (sem fabricar)

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `edita o lançamento de 999 do circo` | "Não encontrei um lançamento compatível..." (ou pede detalhes) — **NUNCA** "Prontinho!" |
| 2 | `cancelar` | "Tudo certo, o registro foi cancelado." |

PASS: zero efeito no banco. FAIL: confirmou/fabricou edição inexistente.

### 5.4 Cancelar no meio da edição

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `quero corrigir um lançamento aí` | "Qual lançamento você gostaria de corrigir?" |
| 2 | `o lançamento do uber` | "Qual correção...?" |
| 3 | `quero alterar o valor para 40` | Candidato(s)/confirmação |
| 4 | `cancelar` | "Tudo certo, o registro foi cancelado." |

PASS: valor do uber inalterado no banco.

### 5.5 Editar categoria

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `o lançamento do uber não é essa categoria, coloca em transporte` | Candidato(s) ou nova categoria proposta |
| 2 | `1` (se listar) | Confirmação mostrando categoria atual → nova |
| 3 | `sim` | "Prontinho, atualizei! ✅" |

PASS: category_id muda, MESMO id, version+1.

### 5.6 Excluir lançamento

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `apaga o lançamento de 30 do uber` | Candidato(s) numerado(s) |
| 2 | `1` | Confirmação de remoção |
| 3 | `sim` | Confirma remoção |

PASS: `deleted_at` preenchido só após confirmação; demais uber intactos.

---

## Bloco 6 — Consultas

### 6.1 Quanto gastei hoje (valores e decimais exatos)

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `quanto gastei hoje?` | Total + itens **centavo a centavo** iguais ao banco (regressão histórica: R$ 300 aparecia como R$ 30) |

```sql
SELECT description, amount_cents FROM mecontrola.transactions
WHERE user_id='<USER_ID>' AND direction=2 AND deleted_at IS NULL
AND occurred_at >= date_trunc('day', now() AT TIME ZONE 'America/Sao_Paulo') AT TIME ZONE 'America/Sao_Paulo'
ORDER BY created_at;
```

(direction de despesa: confirmar valor na 1ª execução — registrar: ____)

### 6.2 Quanto gastei ontem

Preparação:

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 25 ontem no lanche no pix` | Confirmação com data de ontem |
| 2 | `sim` | Registrado |

Cenário:

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `quanto gastei ontem?` | R$ 25,00 — lanche (janela 00:00–23:59 de ontem; regressão histórica: retornava "sem lançamentos") |

### 6.3 Pergunta fora de contexto no meio de fluxo

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 15 no café no pix` | Lista de categorias (fluxo suspenso) |
| 2 | `que dia é hoje?` | Data corrente correta |
| 3 | `5` (ou número da categoria) | Fluxo retoma: subcategoria/confirmação |
| 4 | `cancelar` | "Tudo certo, o registro foi cancelado." |

PASS: a pergunta não quebra o fluxo; retoma corretamente. FAIL: perdeu o fluxo ou registrou errado.

### 6.4 Fatura do cartão

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `quanto está minha fatura do cartão nubank?` | Valor, vencimento e itens batendo com `transactions_card_invoice_items` do ref_month corrente |

---

## Bloco 7 — Recorrências: CRUD + dedup

### 7.1 Listar recorrências

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `quais são minhas recorrências?` | Lista: aluguel (4.4), salário (4.2), pensão (4.3), internet (4.5) — valores/dias corretos, sem alucinação |

### 7.2 Dedup — criar a mesma recorrência 2x (fix B-23)

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `todo dia 5 pago 1500 de aluguel no pix` | Confirmação de recorrência (valor/dia/categoria) |
| 2 | `sim` | **Bloqueio amigável**: "Você já tem uma recorrência igual ativa... Não criei outra para não lançar em dobro..." |

```sql
SELECT count(*) FROM mecontrola.transactions_recurring_templates
WHERE user_id='<USER_ID>' AND description ILIKE '%aluguel%' AND deleted_at IS NULL;
```

PASS: count=1. FAIL: 2 templates; ou "Prontinho!" com template novo.

### 7.3 Editar recorrência

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `muda o aluguel para 1600` | Confirmação: R$ 1.500,00 → R$ 1.600,00 |
| 2 | `sim` | Atualizado |

Check: amount_cents=160000, version+1, mesmo id.

### 7.4 Excluir recorrência

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `cancela a recorrência da pensão` | Confirmação de remoção |
| 2 | `sim` | Removida |

Check: `deleted_at` preenchido no template da pensão; demais intactas.

### 7.5 Materialização (janela real, se aplicável)

Preparação (se amanhã for dia D):

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `todo dia <D> pago 10 de teste materialização no pix` | Confirmação |
| 2 | `sim` | Recorrência criada |

No dia seguinte:

```sql
SELECT t.description, m.ref_month, m.materialized_transaction_id, m.materialized_at
FROM mecontrola.transactions_recurring_materializations m
JOIN mecontrola.transactions_recurring_templates t ON t.id=m.template_id
WHERE t.user_id='<USER_ID>';
```

PASS: 1 lançamento materializado de 1000 cents no dia. FAIL: duplicado, retroativo ou valor errado. Depois excluir o template e o lançamento de teste.

---

## Bloco 8 — Orçamento

### 8.1 Consultar orçamento

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `como está meu orçamento?` | Planejado x gasto do mês batendo com `budgets` + soma real das transações |

### 8.2 Alterar total com reescalonamento

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `quero mudar meu orçamento total para 5000` | Aviso de reescalonamento + confirmação |
| 2 | `sim` | Confirmado |

Check: total_cents=500000; alocações somando 500000.

---

## Bloco 9 — Resiliência e idempotência

### 9.1 Duplo "sim" (reenvio da mesma mensagem)

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `gastei 12 no teste duplo sim no pix` | Confirmação |
| 2 | `sim` (2x seguidas, rápido) | 1 único "Prontinho! ✅"; 2º "sim" recebe resposta segura |

PASS: exatamente 1 transação de 1200. FAIL: 2 transações ou erro.

### 9.2 Re-entrega de evento (técnico)

```sql
SELECT id, event_type, status FROM mecontrola.outbox_events
WHERE event_type LIKE 'agents.whatsapp.inbound%' ORDER BY created_at DESC LIMIT 3;
UPDATE mecontrola.outbox_events SET status=1, attempts=0, locked_at=NULL, locked_by=NULL, published_at=NULL
WHERE id='<ID_DO_EVENTO_JA_PROCESSADO>';
```

Esperado: consumer ignora com outcome=deduplicated (métrica `agents_whatsapp_inbound_total` + log), zero mensagem/lançamento duplicado.

### 9.3 Mensagem sem sentido

| # | Você envia | Bot responde (esperado) |
|---|-----------|------------------------|
| 1 | `asdfgh` | Resposta segura/orientação — sem criar nada, sem "Prontinho!" |

---

## Registro de execução

| Cenário | Data | Resultado | Evidência |
|---------|------|-----------|-----------|
| 1.1 Onboarding completo | | | |
| 2.1 Pix auto-categoria | | | |
| 2.2 Vale-refeição | | | |
| 2.3 Listagem + recusa | | | |
| 2.4 Reprompt "talvez" | | | |
| 2.5 Cancelar na categoria | | | |
| 2.6 Multi-item | | | |
| 2.7 Débito + dinheiro | | | |
| 3.1 Crédito com apelido | | | |
| 3.2 Pergunta apelido (texto sem 💳) | | | |
| 3.3 Parcelado + fatura | | | |
| 3.4 Listar/atualizar cartão | | | |
| 4.1 Receita avulsa | | | |
| 4.2 Salário recorrente (caso R$ 5) | | | |
| 4.3 "Todo mês" sem dia | | | |
| 4.4 Aluguel recorrente (com pix) | | | |
| 4.5 Internet recorrente (sem pix) | | | |
| 5.1 Correção com desambiguação | | | |
| 5.2 "O valor certo é X" | | | |
| 5.3 Edição inexistente | | | |
| 5.4 Cancelar edição | | | |
| 5.5 Editar categoria | | | |
| 5.6 Excluir lançamento | | | |
| 6.1 Gasto hoje (decimais) | | | |
| 6.2 Gasto ontem (janela) | | | |
| 6.3 Pergunta fora de contexto | | | |
| 6.4 Fatura | | | |
| 7.1 Listar recorrências | | | |
| 7.2 Dedup | | | |
| 7.3 Editar recorrência | | | |
| 7.4 Excluir recorrência | | | |
| 7.5 Materialização | | | |
| 8.1 Consultar orçamento | | | |
| 8.2 Alterar total | | | |
| 9.1 Duplo "sim" | | | |
| 9.2 Re-entrega de evento | | | |
| 9.3 Mensagem sem sentido | | | |
