# Suíte Production-Proof — Validação Completa Pós-Wipe (WhatsApp)

Data: 2026-07-29 (pós-wipe do banco + deploy sha `7959ec5`, run 30485218565)
Número de teste: `+5511986896322` (usuário será recriado no onboarding)
Substitui e consolida: `suite-fogo-real-fluxos-nao-cobertos.md` + incidentes de `resultados-e2e-agente-2026-07-28.txt` (RODADAs 1-23)

## Como usar

1. Envie as frases EXATAS no WhatsApp, na ordem, aguardando a resposta do bot entre elas.
2. "Resposta variável" = valide a SEMÂNTICA (intenção, dados, valores), não o texto exato.
3. O `user_id` mudou após o wipe. Resolva uma vez por sessão de validação:

```bash
ssh mecontrola-vps "docker exec -i mecontrola_postgres.1.uvxevski7nrklemzf2gnusg75 psql -U mecontrola -d mecontrola_db -t -A -c \"SELECT id FROM mecontrola.users WHERE whatsapp_number='+5511986896322';\""
```

Todas as queries abaixo usam `'<USER_ID>'` — substitua pelo id resolvido.

4. **FAIL imediato global (0 falso positivo)**: bot diz "Prontinho! ✅"/"atualizei"/"removi" sem efeito no banco; valor/categoria/cartão/dia diferente do confirmado; mensagem duplicada; pergunta de cartão em pagamento que não é crédito; a palavra "cartão" substituída por emoji 💳 no texto do bot (exigência explícita do usuário: o texto deve ser "Qual é o apelido do cartão que você usou?").
5. Registre cada resultado na tabela final + evidência (run id, query).

---

## Bloco 1 — Onboarding (banco zerado)

### 1.1 Fluxo completo informando cartão e recorrência

Frases (na ordem das etapas do workflow; aguardar cada pergunta):

1. `oi` → boas-vindas + pergunta do nome de tratamento
2. `Jailton` → pergunta do orçamento mensal
3. `4000` → pergunta sobre cartões de crédito
4. `nubank vencimento dia 10` (resposta variável do bot pedindo confirmação) → confirmar com `sim` se pedir
5. (proposta de distribuição do orçamento por categoria) `sim`
6. (despesas recorrentes) `aluguel 1500 dia 5`
7. (confirmação/resumo final) `sim`

Esperado: cada etapa suspende e retoma sem loop; ao final, usuário ACTIVE com budget de R$ 4.000, alocações somando o total, cartão "nubank" (due_day=10) e template de recorrência aluguel R$ 1.500 dia 5.

Checks:

```sql
SELECT id, whatsapp_number, display_name, status FROM mecontrola.users WHERE whatsapp_number='+5511986896322';
SELECT competence, total_cents, state FROM mecontrola.budgets WHERE user_id='<USER_ID>';
SELECT root_slug, planned_cents FROM mecontrola.budgets_allocations WHERE budget_id IN (SELECT id FROM mecontrola.budgets WHERE user_id='<USER_ID>');
SELECT nickname, due_day, closing_day FROM mecontrola.cards WHERE user_id='<USER_ID>';
SELECT description, amount_cents, day_of_month, frequency FROM mecontrola.transactions_recurring_templates WHERE user_id='<USER_ID>';
```

PASS: tudo persistido coerente com as respostas; soma das alocações = total. FAIL: loop de etapa, dado errado, onboarding concluído sem dados, cartão criado sem confirmação.

---

## Bloco 2 — Despesas sem cartão (pix, vale, débito, dinheiro)

### 2.1 Pix com categoria automática (sem nenhuma pergunta de cartão)

Frases:

1. `gastei 45 na farmácia no pix`
2. (se pedir categoria, responder número/nome)
3. `sim`

Esperado: confirmação com valor R$ 45,00, categoria resolvida (Medicamentos e Farmácia ou equivalente), pagamento pix. **NUNCA perguntar "Qual cartão foi utilizado?"** (regressão histórica RODADA inicial).

Check:

```sql
SELECT description, amount_cents, payment_method, subcategory_name_snapshot, occurred_at::date
FROM mecontrola.transactions WHERE user_id='<USER_ID>' ORDER BY created_at DESC LIMIT 1;
```

PASS: 4500 cents, pix, categoria coerente, data de hoje. FAIL: perguntou cartão; valor/categoria errados.

### 2.2 Vale-refeição com categoria automática

Frases: `gastei 30 no almoço no vale` → (confirmação) `sim`

Esperado: pagamento `vale_refeicao`, categoria Prazeres > Restaurantes (ou listagem se não resolver — responder). Check: mesma query — payment_method=`vale_refeicao`, 3000 cents.

### 2.3 Categoria desconhecida → listagem numerada → subcategoria

Frases:

1. `gastei 77 em teste recusa`
2. (lista de raízes) `2` (Custo Fixo)
3. (lista de subcategorias) `1` (Açougue)
4. (forma de pagamento, se perguntar) `pix`
5. (confirmação) `não`

Esperado: após `não`, "Tudo certo, o registro foi cancelado." e **zero transação nova**. Check: query de 2.1 — o lançamento de 7700 NÃO pode existir.

### 2.4 Reprompt de confirmação ("talvez")

Frases:

1. `gastei 33 em teste reprompt`
2. (categoria) `2` → (subcategoria) `2` → (pagamento) `pix`
3. (confirmação) `talvez` → esperado: "Não entendi. Por favor, responda apenas sim ou não para confirmar."
4. `talvez` de novo → comportamento documentado: reprompt OU cancelamento — registrar o que ocorreu: ____
5. `cancelar` → cancelado, zero transação.

### 2.5 Cancelar na etapa de categoria

Frases: `gastei 99 em teste cancelamento` → (lista de categorias) `cancelar`

Esperado: "Tudo certo, o registro foi cancelado." Zero transação de 9900.

### 2.6 Multi-item na mesma mensagem

Frase: `gastei 30 no ônibus e 15 no café`

Esperado: orientação para registrar um de cada vez ("Por segurança, registro um de cada vez..."). **Nenhum** lançamento criado. Check: zero transações novas.

### 2.7 Débito e dinheiro

Frases: `paguei 35 no dentista no débito` → `sim`; depois `gastei 20 no padaria em dinheiro` → (categoria se pedir) → `sim`

Esperado: payment_method `debit_card` (3500) e `cash` (2000). Check query 2.1 para cada um.

---

## Bloco 3 — Crédito e cartões

### 3.1 Crédito com apelido na frase (atalho determinístico)

Frases: `gastei 45 no cartão nubank` → (categoria se pedir) → `sim`

Esperado: **sem pergunta de apelido** (nubank já veio na frase); confirmação com "crédito" e categoria. Check: transação 4500 payment_method=`credit_card`, card_id do nubank.

### 3.2 Crédito SEM apelido → pergunta com texto correto

Frases: `comprei 50 no crédito` → bot pergunta o apelido → `nubank` → (categoria) → `sim`

Esperado (exigência dura do usuário): o texto da pergunta é **"Qual é o apelido do cartão que você usou?"** (ou variação que contenha a palavra "cartão" por extenso). FAIL se a palavra "cartão" for substituída pelo emoji 💳 (ex.: "preciso saber qual 💳 você quer usar").

### 3.3 Parcelado + fatura por mês

Frases:

1. `quanto está minha fatura do cartão nubank?` (anotar baseline: ____)
2. `comprei 600 de eletrônico em 3x no crédito` → (apelido se pedir) `nubank` → (categoria) → `sim`
3. `quanto está minha fatura do cartão nubank?`

Esperado: 1 transação de 60000 cents com installments_total=3; fatura do mês sobe **R$ 200,00** (não R$ 600,00); parcelas 2 e 3 nas faturas dos 2 meses seguintes.

Checks:

```sql
SELECT id, amount_cents, installments_total, card_id FROM mecontrola.transactions
WHERE user_id='<USER_ID>' ORDER BY created_at DESC LIMIT 1;
SELECT i.ref_month, it.installment_index, it.amount_cents
FROM mecontrola.transactions_card_invoice_items it
JOIN mecontrola.transactions_card_invoices i ON i.id=it.invoice_id
WHERE it.transaction_id='<ID_DA_TRANSACAO>' ORDER BY it.installment_index;
```

PASS: 3 itens de 20000 em ref_months consecutivos. FAIL: fatura subiu 60000 de uma vez.

### 3.4 Listar cartões / atualizar vencimento

Frases: `quais cartões eu tenho?` → confere nubank. Depois: `muda o vencimento do nubank para dia 15` → `sim`

Check: `SELECT nickname, due_day FROM mecontrola.cards WHERE user_id='<USER_ID>';` → due_day=15 só após confirmação.

---

## Bloco 4 — Receitas

### 4.1 Receita avulsa

Frases: `recebi 500 de freelance` → `sim`

Esperado: direção income, 50000 cents, Origem: freelance. Check:

```sql
SELECT direction, payment_method, amount_cents, description FROM mecontrola.transactions
WHERE user_id='<USER_ID>' AND direction=1 ORDER BY created_at DESC LIMIT 1;
```

(direction: confirmar valor de income na 1ª execução — registrar: ____)

### 4.2 Receita recorrente — frase exata do incidente R$ 5,00

Frases: `todo dia 5 eu recebo R$ 13.874,40 de salário` → (confirmação) `sim`

Esperado (fix B-23): confirmação de **recorrência** com valor **R$ 13.874,40** — NUNCA R$ 5,00 — dia 5, descrição salário. Resolução determinística (sem LLM).

Check:

```sql
SELECT description, amount_cents, day_of_month, direction FROM mecontrola.transactions_recurring_templates
WHERE user_id='<USER_ID>' ORDER BY created_at DESC LIMIT 1;
```

PASS: 1387440 cents, dia 5, income. FAIL: R$ 5,00; lançamento avulso em vez de recorrência; qualquer pergunta de valor.

### 4.3 Receita recorrente sem dia ("todo mês" — caminho LLM protegido)

Frases: `todo mês eu recebo 800 de pensão` → seguir o fluxo (bot deve perguntar o dia) → `dia 10` → (confirmação) `sim`

Esperado: bot NÃO registra avulso; pergunta o dia; cria recorrência dia 10, 80000 cents. Se em qualquer etapa aparecer confirmação de lançamento avulso de R$ 800: FAIL.

---

## Bloco 5 — Edição, correção e exclusão de lançamentos

### 5.1 Correção de valor com desambiguação

Preparação: `gastei 30 no uber no pix` → (categoria) → `sim`. Repetir: `gastei 30 no uber no pix` → (categoria, pode diferir) → `sim`.

Frases:

1. `edita o lançamento do uber` → bot pergunta o que mudar
2. `quero alterar o valor para 35` → bot lista 2 candidatos numerados
3. `1`
4. (confirmação mostrando 30 → 35) `sim`

Esperado: MESMO id atualizado para 3500, version incrementada. A outra de 3000 intacta.

```sql
SELECT id, description, amount_cents, version, deleted_at FROM mecontrola.transactions
WHERE user_id='<USER_ID>' AND description ILIKE '%uber%' ORDER BY created_at;
```

### 5.2 Correção por frase completa ("o valor certo é X e não Y")

Frases: `no lançamento da farmácia o valor certo é 50 e não 45` → (candidato se listar) `1` → `sim`

Esperado: a farmácia de 4500 vira 5000. Check: query de 5.1 com '%farmácia%'.

### 5.3 Edição de lançamento inexistente (sem fabricar)

Frases: `edita o lançamento de 999 do circo` → esperado: bot informa que não encontrou e pede detalhes OU pede confirmação do que corrigir — **NUNCA** "Prontinho!" nem cria lançamento. Depois `cancelar`.

PASS: zero efeito no banco. FAIL: confirmou/fabricou edição inexistente.

### 5.4 Cancelar no meio da edição

Frases: `quero corrigir um lançamento aí` → `o lançamento do uber` → `quero alterar o valor para 40` → (confirmação) `cancelar`

Esperado: "Tudo certo, o registro foi cancelado." Valor do uber inalterado no banco.

### 5.5 Editar categoria

Frases: `o lançamento do uber não é essa categoria, coloca em transporte` → (candidato) `1` → (nova categoria se listar) → `sim`

Check: category_id muda, MESMO id, version+1.

### 5.6 Excluir lançamento

Frases: `apaga o lançamento de 30 do uber` → (candidato) `1` → (confirmação) `sim`

Esperado: soft delete (`deleted_at` preenchido) só após confirmação; o outro uber intacto.

---

## Bloco 6 — Consultas

### 6.1 Quanto gastei hoje (valores e decimais exatos)

Frase: `quanto gastei hoje?`

Esperado: total e itens **batendo centavo a centavo** com o banco (regressão histórica: R$ 300 aparecia como R$ 30). Conferir com:

```sql
SELECT description, amount_cents FROM mecontrola.transactions
WHERE user_id='<USER_ID>' AND direction=2 AND deleted_at IS NULL
AND occurred_at >= date_trunc('day', now() AT TIME ZONE 'America/Sao_Paulo') AT TIME ZONE 'America/Sao_Paulo'
ORDER BY created_at;
```

(direction de despesa: confirmar valor na 1ª execução — registrar: ____)

### 6.2 Quanto gastei ontem

Preparação: ter ao menos 1 lançamento de ontem (editar data em 5.x ou criar com "ontem": `gastei 25 ontem no lanche no pix` → `sim`).

Frase: `quanto gastei ontem?` → esperado: janela 00:00–23:59 de ontem (regressão histórica: retornava "sem lançamentos").

### 6.3 Pergunta fora de contexto

Frase: `que dia é hoje?` → resposta com a data corrente correta, sem quebrar nenhum fluxo pendente (testar no meio de um fluxo suspenso: iniciar `gastei 15 no café no pix`, na pergunta de categoria mandar `que dia é hoje?`, depois retomar com a resposta da categoria e concluir ou cancelar).

### 6.4 Fatura do cartão

Frase: `quanto está minha fatura do cartão nubank?` → valor e vencimento corretos, itens batem com `transactions_card_invoice_items` do ref_month corrente.

---

## Bloco 7 — Recorrências: CRUD + dedup

### 7.1 Listar recorrências

Frase: `quais são minhas recorrências?` → lista aluguel (onboarding), salário e pensão (bloco 4) com valores/dias corretos. Sem alucinação.

### 7.2 Dedup — criar a mesma recorrência 2x (fix B-23)

Frases: `todo dia 5 pago 1500 de aluguel no pix` → (confirmação) `sim`

Esperado: **bloqueio com mensagem amigável** ("Você já tem uma recorrência igual ativa... Não criei outra para não lançar em dobro...") — NUNCA um 2º template de aluguel 1500 dia 5.

```sql
SELECT count(*) FROM mecontrola.transactions_recurring_templates
WHERE user_id='<USER_ID>' AND description ILIKE '%aluguel%' AND deleted_at IS NULL;
```

PASS: count=1. FAIL: 2 templates; ou criou e disse "Prontinho!".

### 7.3 Editar recorrência

Frases: `muda o aluguel para 1600` → (candidato/confirmação) `sim`

Check: amount_cents=160000, version+1, mesmo id.

### 7.4 Excluir recorrência

Frases: `cancela a recorrência da pensão` → `sim`

Check: `deleted_at` preenchido no template da pensão; demais intactas.

### 7.5 Materialização (janela real, se aplicável)

Se houver template com day_of_month = dia de amanhã, criar um proposital (`todo dia <amanhã> pago 10 de teste materialização no pix` → `sim`) e no dia seguinte verificar:

```sql
SELECT t.description, m.ref_month, m.materialized_transaction_id, m.materialized_at
FROM mecontrola.transactions_recurring_materializations m
JOIN mecontrola.transactions_recurring_templates t ON t.id=m.template_id
WHERE t.user_id='<USER_ID>';
```

PASS: 1 lançamento materializado de 1000 cents com occurred_at no dia. FAIL: duplicado, retroativo, ou valor errado. Depois excluir o template e o lançamento de teste.

---

## Bloco 8 — Orçamento

### 8.1 Consultar orçamento

Frase: `como está meu orçamento?` → números batem com `budgets` + gastos reais do mês (validar contra query do bloco 1 + soma de transações do mês).

### 8.2 Alterar total com reescalonamento

Frases: `quero mudar meu orçamento total para 5000` → (confirmação) `sim`

Check: total_cents=500000; alocações reescaladas somando 500000.

---

## Bloco 9 — Resiliência e idempotência

### 9.1 Duplo "sim" (reenvio da mesma mensagem)

Em qualquer confirmação, enviar `sim` duas vezes seguidas rapidamente. Esperado: 1 único lançamento; o 2º "sim" recebe resposta segura (não duplica, não quebra).

### 9.2 Re-entrega de evento (técnico)

```sql
SELECT id, event_type, status FROM mecontrola.outbox_events
WHERE event_type LIKE 'agents.whatsapp.inbound%' ORDER BY created_at DESC LIMIT 3;
UPDATE mecontrola.outbox_events SET status=1, attempts=0, locked_at=NULL, locked_by=NULL, published_at=NULL
WHERE id='<ID_DO_EVENTO_JA_PROCESSADO>';
```

Esperado: consumer ignora com outcome=deduplicated (métrica `agents_whatsapp_inbound_total` + log), zero mensagem/lançamento duplicado.

### 9.3 Mensagem fora de qualquer fluxo

Frase: `asdfgh` → resposta segura/orientação, sem criar nada, sem "Prontinho!".

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
