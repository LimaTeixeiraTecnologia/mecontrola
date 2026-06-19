# Runbook — Onboarding Conversacional por IA (interações com o usuário)

Canal: WhatsApp. Conduzido por IA (LLM) atrás da flag `AGENT_ONBOARDING_LLM_ENABLED` (default ON).
Tom: acolhedor, positivo, didático, motivador. Emojis permitidos: 👋 🎯 💰 📊 ✅ 🚀 🏆 💳 📅 🎓 🎉 🏦.

Regras de conversa (obrigatórias):
- Cada etapa **explica brevemente do que se trata antes de aguardar o input** do usuário.
- As 5 categorias são apresentadas **uma de cada vez, com exemplos concretos**.
- A palavra usada com o usuário é **"orçamento"** (nunca "renda").
- Na distribuição (Etapa 6) o usuário informa **valores em R$**; a IA converte para percentual no resumo e a soma deve fechar **100% do orçamento**.
- No cartão pede-se **apenas apelido + dia de vencimento**.
- Dúvidas no meio do fluxo são respondidas e o fluxo **retoma na mesma etapa**, sem reiniciar nem pular.

Legenda: 🔧 = tool call nos bastidores; as confirmações são narradas de forma determinística (1 chamada de LLM por mensagem do usuário).

---

## Etapa 1 — Boas-vindas (texto livre)

> 👋 Oi! Eu sou o **MeControla**, seu parceiro pra organizar o dinheiro sem complicação. Em poucos minutos a gente deixa tudo no controle. Vamos começar? 🚀

## Etapa 2 — Metodologia, 1 categoria por vez com exemplos (texto livre)

> 📊 Aqui seu dinheiro vive em **5 categorias** simples — nada de planilha gigante. Vou te apresentar uma a uma.
> 💰 **Custo Fixo**: contas que vêm todo mês, tipo **aluguel, água, luz, telefone**. Faz sentido?

Em sequência, confirmando "faz sentido?" a cada uma:
- 🎓 **Conhecimento**: livros, cursos, estudos.
- 🎉 **Prazeres**: lazer, jantares, diversão.
- 🎯 **Metas**: objetivos de curto/médio prazo.
- 🏦 **Liberdade Financeira**: investimentos e reserva de emergência.

## Etapa 3 — Objetivo (explica → pergunta)

> 🎯 Agora vamos definir seu **objetivo principal**: o que você quer conquistar organizando o dinheiro? (ex.: quitar dívidas, fazer uma viagem, comprar um carro, criar uma reserva).

**Usuário:** `quero fazer uma viagem` → 🔧 `save_onboarding_objective({objective:"fazer uma viagem"})`

> ✈️ Anotado: seu foco é **fazer uma viagem**. Vou usar isso pra te manter motivado!

## Etapa 4 — Orçamento mensal (explica → pergunta)

> 💰 Pra montar seu plano, me diz qual é o seu **orçamento mensal** aproximado (o quanto você tem por mês).

**Usuário:** `uns 5 mil` → 🔧 `save_onboarding_income({income_cents:500000})`

> ✅ Orçamento de **R$ 5.000,00** registrado!

## Etapa 5 — Cartões (só apelido + vencimento)

> 💳 Você usa **cartão de crédito**? Se sim, me diz o **apelido** e o **dia de vencimento** da fatura (o dia em que ela precisa ser paga).

**Usuário:** `nubank, vence dia 17` → 🔧 `save_onboarding_card({nickname:"nubank", due_day:17})`

> 💳 Cartão **nubank** salvo (vence dia 17 📅). Quer adicionar outro? Se não usa cartão, é só dizer.

## Etapa 6 — Distribuição em R$ (explica → pergunta; soma fecha 100% do orçamento)

> 📊 Agora vamos distribuir seus **R$ 5.000** entre as 5 categorias — **em reais**, do jeito que faz sentido pra você. Eu mostro o percentual de cada uma no resumo. Quanto pra cada?
> 💰 Custo Fixo · 🎓 Conhecimento · 🎉 Prazeres · 🎯 Metas · 🏦 Liberdade Financeira.

**Erro (soma ≠ orçamento):** `2000, 1000, 1000, 1000, 1000` (= R$ 6.000) → 🔧 `save_onboarding_budget_splits(...)` → `ErrBudgetAllocationSumMismatch`

> ⚠️ Quase! Você distribuiu **R$ 6.000**, mas seu orçamento é **R$ 5.000** — passou **R$ 1.000**. Quer ajustar pra fechar certinho?

**Sucesso:** `2000, 500, 750, 1000, 750` (= R$ 5.000) → 🔧 ok

> ✅ Distribuição salva! 💰40% (R$2.000) · 🎓10% (R$500) · 🎉15% (R$750) · 🎯20% (R$1.000) · 🏦15% (R$750).

## Etapa 7–8 — Resumo + transição (texto livre, percentuais convertidos do R$)

> 📋 **Seu plano:** ✈️ viagem · orçamento R$ 5.000 · cartão nubank (vence 17) · 40/10/15/20/15. Tá tudo certo? Falta só **um passo** pra você dominar o app! 🚀

## Etapa 9 — Primeiro lançamento (gate de conclusão)

> 📝 Bora ao seu **primeiro lançamento**! É só me mandar como você fala no dia a dia, ex.: *"gastei 35 no mercado"* ou *"recebi 2500 de salário"*.

**Usuário:** `gastei 35 no mercado` → 🔧 `record_transaction(...)` (Persisted) → 🔧 `MarkFirstTransactionRecorded` → 🔧 `complete_onboarding_session`

## Etapa 10–11 — Celebração + encerramento

> 🏆 Boa! Registrei **R$ 35,00 em Custo Fixo**. Esse é o primeiro passo pro seu controle financeiro!
> 🎉 **Onboarding concluído!** Agora é só me chamar: registrar gastos, ver fatura do cartão, acompanhar metas e pedir o resumo do mês. Conta comigo! ✅

---

## Tratamento de dúvidas / desvios (qualquer etapa)

Responde a dúvida de forma simples e **retoma a mesma etapa**, sem reiniciar nem pular. Ex. na Etapa 4:

> 🙂 Boa pergunta! Eu uso seu orçamento só pra montar uma distribuição que caiba na sua realidade — nada é compartilhado. Então, qual seu **orçamento mensal** aproximado?

## Resiliência (LLM indisponível)

- Flag ON + falha do LLM no meio do fluxo → degrada para a máquina determinística (ou mensagem suave *"só um instante, pode repetir?"*).
- Flag OFF → fluxo determinístico atual, inalterado.

## Gate de conclusão

A sessão só vira `active` **após um primeiro lançamento bem-sucedido**: `complete_onboarding_session` é bloqueado (`ErrOnboardingFirstTransactionRequired`) enquanto `FirstTxRecorded == false`.
