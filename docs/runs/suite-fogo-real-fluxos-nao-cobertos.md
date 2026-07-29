# Suíte de Fogo Real — Fluxos Não Cobertos (WhatsApp)

Data de criação: 2026-07-29
Usuário de teste: `9bbbbbcd-2081-40a8-9780-2b5d818f1580` (+5511986896322)
Base de evidências: complementar `docs/runs/resultados-e2e-agente-2026-07-28.txt`

## Como usar

1. Envie as frases EXATAS no WhatsApp, na ordem, aguardando a resposta do bot entre elas.
2. Compare com a resposta esperada. Quando marcado "resposta variável", valide a SEMÂNTICA (intenção e dados), não o texto exato — a frase final é gerada pelo LLM.
3. Rode os checks SQL após cada cenário (via `ssh mecontrola-vps`):

```bash
ssh mecontrola-vps "docker exec -i mecontrola_postgres.1.uvxevski7nrklemzf2gnusg75 psql -U mecontrola -d mecontrola_db -c \"<QUERY>\""
```

4. Critério global de falso positivo (FAIL imediato em qualquer cenário): o bot dizer "Prontinho! ✅" / "atualizei" / "removi" SEM o lançamento existir no banco, ou registrar valor/categoria/cartão diferente do confirmado, ou enviar mensagem duplicada.
5. Registre o resultado na tabela final.

Tabelas usadas nos checks: `mecontrola.transactions`, `mecontrola.transactions_card_invoices`, `mecontrola.transactions_card_invoice_items`, `mecontrola.transactions_recurring_templates`, `mecontrola.budgets`, `mecontrola.budgets_allocations`, `mecontrola.cards`, `mecontrola.workflow_runs`, `mecontrola.outbox_events`. (Schemas verificados na VPS em 2026-07-29.)

---

## Bloco A — Recorrências

### A1. Criar recorrência

Frases:

1. `todo dia 5 pago 1500 de aluguel`
2. (se o bot pedir confirmação) `sim`

Esperado: o bot entende recorrência mensal (dia 5, R$ 1.500,00, aluguel), resolve categoria (Custo Fixo > Aluguel — se pedir categoria, responda o número/nome) e pede confirmação ANTES de salvar. Após `sim`: confirmação de recorrência criada. Resposta variável no texto final — validar semântica.

Checks:

```sql
SELECT id, description, amount_cents, day_of_month, frequency, deleted_at
FROM mecontrola.transactions_recurring_templates
WHERE user_id='9bbbbbcd-2081-40a8-9780-2b5d818f1580' ORDER BY created_at DESC LIMIT 3;
```

```sql
SELECT workflow, status, left(last_error,80) AS err, created_at FROM mecontrola.workflow_runs
ORDER BY created_at DESC LIMIT 3;
```

PASS: template recorrente criado (dia 5, 150000 cents), zero transação materializada retroativa indevida, confirmação humana exigida. FAIL: criou sem confirmar, valores/dia errados, ou registrou lançamento avulso em vez de recorrência.

### A2. Listar recorrências

Frase: `quais são minhas recorrências?`

Esperado: lista contendo a recorrência criada em A1 (aluguel, R$ 1.500,00, dia 5). Resposta variável.

PASS: a recorrência de A1 aparece com valores corretos. FAIL: lista vazia, valores errados, ou alucinação de recorrência inexistente.

### A3. Editar recorrência

Frases:

1. `muda o aluguel para 1600`
2. (confirmação) `sim`

Esperado: o bot localiza a recorrência do aluguel, pede confirmação mostrando valor atual (1500) e novo (1600), e só persiste após `sim`.

Check:

```sql
SELECT description, amount_cents, day_of_month, deleted_at, version
FROM mecontrola.transactions_recurring_templates
WHERE user_id='9bbbbbcd-2081-40a8-9780-2b5d818f1580' ORDER BY created_at DESC LIMIT 3;
```

PASS: amount_cents=160000 após confirmação. FAIL: alterou sem confirmação ou editou outro item.

### A4. Excluir recorrência

Frases:

1. `cancela a recorrência do aluguel`
2. (confirmação) `sim`

Esperado: pede confirmação de remoção; após `sim`, remove/desativa.

Check: mesma query de A3 — o registro deve ter `deleted_at` preenchido.

PASS: removido só após confirmação. FAIL: removeu sem confirmar; "Prontinho!" sem efeito no banco.

---

## Bloco B — Orçamento (budget)

### B1. Consultar orçamento atual

Frase: `como está meu orçamento?`

Esperado: resumo do plano orçamentário do mês (planejado x gasto), via query_plan. Resposta variável — validar que os números batem com o banco.

Check:

```sql
SELECT id, competence, total_cents, state FROM mecontrola.budgets
WHERE user_id='9bbbbbcd-2081-40a8-9780-2b5d818f1580' ORDER BY created_at DESC LIMIT 2;
```

(state: confirmar o valor de "ativo" na 1ª execução — registrar aqui: ____)

PASS: números citados correspondem ao banco. FAIL: inventou números (falso positivo de leitura).

### B2. Alterar total do orçamento

Frases:

1. `quero mudar meu orçamento total para 5000`
2. (fluxo de confirmação/reescalonamento) `sim`

Esperado: inicia edit_budget_total, avisa que as categorias serão reescaladas proporcionalmente, pede confirmação, e só persiste após `sim`.

Check:

```sql
SELECT competence, total_cents, state FROM mecontrola.budgets
WHERE user_id='9bbbbbcd-2081-40a8-9780-2b5d818f1580' ORDER BY created_at DESC LIMIT 1;
SELECT root_slug, basis_points, planned_cents FROM mecontrola.budgets_allocations
WHERE budget_id IN (SELECT id FROM mecontrola.budgets WHERE user_id='9bbbbbcd-2081-40a8-9780-2b5d818f1580' ORDER BY created_at DESC LIMIT 1);
```

PASS: total=500000 cents e alocações (planned_cents) reescaladas somando o novo total. FAIL: alterou sem confirmação; soma das alocações diverge do total.

### B3. Criar orçamento (só se não houver ativo)

Frases: `quero criar um orçamento de 4000 por mês` e seguir o fluxo (distribuição por categoria, confirmações com `sim`).

Esperado: create_budget conduz coleta de total + distribuição até confirmação explícita. PASS: budget ativo criado só após confirmação. FAIL: criou sem confirmar ou perdeu o fio do fluxo.

---

## Bloco C — Cartões

### C1. Criar cartão

Frases:

1. `cadastra o cartão inter com vencimento dia 10`
2. (confirmação) `sim`

Esperado: create_card pede confirmação explícita ANTES de criar; após `sim`, cartão cadastrado.

Check:

```sql
SELECT id, nickname, bank, due_day, closing_day, deleted_at FROM mecontrola.cards
WHERE user_id='9bbbbbcd-2081-40a8-9780-2b5d818f1580' ORDER BY created_at DESC LIMIT 4;
```

PASS: cartão "inter" com due_day=10 só após confirmação. FAIL: criou sem confirmar; vencimento errado.

### C2. Listar cartões

Frase: `quais cartões eu tenho?`

Esperado: lista com os cartões reais do usuário (nubank, xp, inter...). PASS: corresponde ao banco. FAIL: cartão alucinado ou faltando.

### C3. Atualizar cartão

Frases:

1. `muda o vencimento do inter para dia 15`
2. (confirmação) `sim`

Esperado: update_card mostra alteração (10 → 15) e pede confirmação. Check: mesma query de C1 — due_day=15.

PASS: alterado só após confirmação. FAIL: alterou sem confirmar.

### C4. Compra parcelada no crédito + fatura

Frases:

1. `quanto está minha fatura do cartão inter?` (anotar o valor — baseline; pode ser "sem fatura aberta")
2. `comprei 900 de tênis em 3x no cartão inter`
3. (categoria, se pedir) responder número/nome (ex.: Compras Pessoais)
4. (confirmação) `sim`
5. `quanto está minha fatura do cartão inter?`

Esperado: compra registrada como 1 lançamento de R$ 900,00 em 3 parcelas de R$ 300,00; a fatura do mês corrente sobe R$ 300,00 (não R$ 900,00); as outras 2 parcelas caem nas faturas dos 2 meses seguintes.

Checks:

```sql
SELECT id, description, amount_cents, installments_total, card_id
FROM mecontrola.transactions
WHERE user_id='9bbbbbcd-2081-40a8-9780-2b5d818f1580' ORDER BY created_at DESC LIMIT 2;
```

```sql
SELECT i.ref_month, i.items_total_cents, it.installment_index, it.amount_cents
FROM mecontrola.transactions_card_invoice_items it
JOIN mecontrola.transactions_card_invoices i ON i.id = it.invoice_id
WHERE it.transaction_id = '<ID_DA_TRANSACAO_ACIMA>'
ORDER BY it.installment_index;
```

PASS: 3 itens de fatura de 30000 cents em ref_months consecutivos; fatura do mês sobe só a parcela. FAIL: fatura subiu 90000 de uma vez; parcelas com valor errado; falso "Prontinho!".

---

## Bloco D — Edição avançada e exclusão de lançamento

### D1. Editar categoria de um lançamento

Frases:

1. `gastei 22 no estacionamento no pix`
2. (categoria/confirmação) seguir fluxo e confirmar com `sim`
3. `o lançamento do estacionamento não é essa categoria, coloca em transporte`
4. (escolher candidato se listar) `1`
5. (confirmação) `sim`

Esperado: edit_entry localiza o lançamento, mostra categoria atual e nova, confirma, persiste. Check:

```sql
SELECT id, description, amount_cents, category_id, version, occurred_at
FROM mecontrola.transactions
WHERE user_id='9bbbbbcd-2081-40a8-9780-2b5d818f1580' ORDER BY created_at DESC LIMIT 2;
```

PASS: category_id muda, version incrementa, MESMO id. FAIL: criou lançamento novo em vez de editar.

### D2. Editar data de um lançamento

Frases:

1. `o estacionamento foi ontem, não hoje`
2. (candidato/confirmação) `1` depois `sim`

Esperado: edit_entry altera a data para ontem. Check: query acima + campo de data da transação.

PASS: data alterada com confirmação. FAIL: registrou novo lançamento de ontem.

### D3. Excluir lançamento

Frases:

1. `apaga o lançamento do estacionamento`
2. (candidato/confirmação) `1` depois `sim`

Esperado: delete_entry pede confirmação; após `sim`, lançamento removido (soft delete). Check: query acima — registro some ou `deleted_at` preenchido.

PASS: removido só após confirmação. FAIL: removeu sem confirmar; "Prontinho!" sem efeito.

---

## Bloco E — Onboarding (usuário novo)

ATENÇÃO: o usuário de teste já concluiu onboarding. Este bloco exige um número de WhatsApp NUNCA cadastrado (ou reset controlado em ambiente de teste — NÃO resetar o usuário de produção).

Frases (fluxo esperado, na ordem das etapas do workflow):

1. `oi` → esperado: boas-vindas + pergunta do nome de tratamento ("Como posso te chamar?")
2. `Jailton` → pergunta do orçamento mensal ("quanto você pode gastar por mês" ou similar)
3. `4000` → pergunta sobre cartões de crédito (tem? apelido + vencimento; ou "não tenho")
4. `não tenho` (ou informar cartão) → proposta de distribuição do orçamento por categoria (sugestão automática)
5. `sim` (aceitar distribuição) → pergunta sobre despesas recorrentes
6. `aluguel 1200 dia 5` (ou `não tenho`) → resumo final + conclusão

Esperado: cada etapa suspende aguardando input e retoma corretamente; ao final, usuário ativo com budget, alocações e (se informadas) recorrências e cartão criados.

Checks (com o user_id do NOVO usuário):

```sql
SELECT id, whatsapp_number, status, created_at FROM mecontrola.users ORDER BY created_at DESC LIMIT 1;
SELECT status, created_at FROM mecontrola.workflow_runs ORDER BY created_at DESC LIMIT 5;
SELECT total_cents FROM mecontrola.budgets WHERE user_id='<NOVO_USER_ID>';
SELECT nickname, due_day FROM mecontrola.cards WHERE user_id='<NOVO_USER_ID>';
```

PASS: fluxo completo sem loop, sem pular etapa, dados persistidos coerentes com as respostas. FAIL: etapa repetida em loop, dados errados, ou onboarding concluído sem dados.

---

## Bloco F — Idempotência (pós-deploy da migration 000011)

### F1. Re-entrega de evento não duplica processamento

Cenário técnico (não é frase de WhatsApp): após o deploy, re-publicar um evento inbound JÁ processado e confirmar que o consumer pula com `outcome=deduplicated` e NENHUMA mensagem nova chega no WhatsApp.

Como forçar (exemplo — pegar um evento inbound já publicado do dia e reenfileirar):

```sql
SELECT id, event_type, status, attempts FROM mecontrola.outbox_events
WHERE event_type LIKE 'agents.whatsapp.inbound%' ORDER BY created_at DESC LIMIT 5;
```

```sql
UPDATE mecontrola.outbox_events SET status=1, attempts=0
WHERE id='<ID_DO_EVENTO_JA_PROCESSADO>';
```

(status: 1=pending, 2=published — confirmado em `internal/platform/outbox/status.go`)

Esperado: worker consome, consumer registra `agents_whatsapp_inbound_total{outcome="deduplicated"}` (Prometheus) + log "mensagem duplicada ignorada" (Loki), zero mensagem no WhatsApp, zero lançamento novo.

PASS: dedup comprovado. FAIL: mensagem/lançamento duplicado.

---

## Registro de execução

| Cenário | Data | Resultado | Evidência (runs/queries) |
|---------|------|-----------|--------------------------|
| A1 | | | |
| A2 | | | |
| A3 | | | |
| A4 | | | |
| B1 | | | |
| B2 | | | |
| B3 | | | |
| C1 | | | |
| C2 | | | |
| C3 | | | |
| C4 | | | |
| D1 | | | |
| D2 | | | |
| D3 | | | |
| E  | | | |
| F1 | | | |
