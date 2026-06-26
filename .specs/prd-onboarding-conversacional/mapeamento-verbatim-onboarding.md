# Mapeamento Verbatim — Onboarding Oficial × Código Real

<!-- doc-version: 1 -->

> **Propósito:** auditoria de fidelidade etapa-a-etapa do onboarding, confrontando o diálogo/regras oficiais (`docs/oficial/2026_06_24_mecontrola_oficial.md`, Cap. 07/08/10) contra os artefatos reais de código (strings, prompts LLM, handlers, use cases, eventos). Insumo obrigatório e rastreável para a Especificação Técnica do PRD `.specs/prd-onboarding-conversacional/prd.md`.
> **Postura:** sem flexibilização. Fidelidade = a etapa existe, na ordem oficial, com prompt/logic codificando o conteúdo e as regras oficiais, no tom oficial (Cap. 03–06). Como decidido no discovery (D-01), as mensagens são **geradas por LLM no tom oficial** — portanto o texto oficial é o **contrato de conteúdo** que o prompt de cada etapa deve codificar, não uma string literal a ser igualada byte a byte.
> **Evidência:** todas as transcrições abaixo são literais, com `arquivo:linha` reais extraídos do working tree em 2026-06-25 via 3 subagentes Explore.

## Legenda de veredito

| Símbolo | Significado |
|---|---|
| ✅ FIEL | Etapa existe, na ordem certa, codificando conteúdo e regras oficiais. |
| 🟡 PARCIAL | Existe, mas com conteúdo incompleto, fora de ordem ou regra divergente. |
| 🔴 DIVERGENTE | Comportamento contradiz o oficial. |
| ⬛ AUSENTE | Não existe etapa/artefato correspondente. |

---

## 0. Sumário executivo — divergências macro

O código atual **não implementa as 8 etapas oficiais**. Implementa um fluxo de **4 etapas** (`🔵 Etapa 1/4 … 4/4`) + um passo extra de primeira transação. As fases reais são apenas 5:

```go
// internal/agent/application/usecases/run_onboarding_turn.go:12-18
OnbPhaseObjective     = "objective"
OnbPhaseBudget        = "budget"
OnbPhaseCards         = "cards"
OnbPhaseFinancialPlan = "financial_plan"
OnbPhaseFirstTx       = "first_tx"
```

| # | Divergência macro | Evidência | Ação para fidelidade |
|---|---|---|---|
| M-1 | Numeração "Etapa X/4" em vez de 8 etapas oficiais | `onboarding_scripts.go:231-236` (`onbHeader*`), `scriptWelcome` ("Etapa 1/4 — Objetivo") | Remodelar para as 8 etapas do Cap. 08; remover a numeração "/4". |
| M-2 | Apresentação das 5 categorias está embutida nas **boas-vindas**, não como etapa própria após os cartões | `onboarding_scripts.go:21-31` (`scriptWelcome`) | Mover para uma **ETAPA 5** própria (após cartões); tirar do welcome (RF-11). |
| M-3 | Coleta de valores por categoria é **em bloco** (todas as 5 numa mensagem) e/ou **auto-sugerida**; o oficial exige o usuário informar valor **categoria por categoria** | `onboarding_scripts.go:58-65` (`buildAutoSplitPreview`), `:218` (prompt "Envie as 5 categorias"), `suggest_budget_split.go` | Coletar **uma categoria por vez** (Regra 1, Cap. 06); auto-sugestão sai do caminho oficial (RF-13). |
| M-4 | Conclusão **exige primeira transação** (passo `first_tx` + `ErrOnboardingFirstTransactionRequired`) | `run_onboarding_turn.go:OnbPhaseFirstTx`, `complete_onboarding_session.go:22,73`, `onboarding_session.go:189-194` | Concluir após confirmação do Resumo; remover `FirstTxRecorded` de `IsReadyToComplete` (RF-19). |
| M-5 | Não há **Resumo Final** (ETAPA 7) como passo próprio com correção; o "Plano Financeiro" (passo 4/4) mistura valores+resumo e só permite re-distribuir | `onboarding_scripts.go:67-89` (`buildFinancialPlanMessage`) | Criar **ETAPA 7** com resumo (valor+percentual) + gate HITL + correção guiada por LLM (RF-16/17/18). |
| M-6 | Cartão coleta **dia de fechamento (`ClosingDay`)**; o oficial pede **dia de vencimento da fatura** | `save_onboarding_card_input.go:15-19`, ack `onboarding_tool_dispatcher.go:125` ("fecha dia %d") vs oficial "Vencimento: dia 13" | **✅ DECIDIDO (DP-1, §4):** coletar **somente dia de vencimento**; remover fechamento do onboarding. |
| M-7 | Boas-vindas **não tem** o handshake oficial "Vamos começar? → Sim"; o welcome já pergunta o objetivo | `onboarding_scripts.go:50-55` (welcome prompt pede objetivo), `run_onboarding_turn.go:214-229` | **✅ DECIDIDO (DP-2, §4):** incluir o handshake "Vamos começar?" como turno próprio (RF-04). |
| M-8 | `phase` é **string livre** (viola DMMF state-as-type) | `onboarding_session.go:76` (`Phase string`), constantes string em `run_onboarding_turn.go:12-18` | Tipo fechado `OnboardingPhase` (RF-22). |
| M-9 | Slug de categoria diverge entre camadas: **hyphen** no BD vs **underscore** com prefixo `expense.` no onboarding/eventos | BD `custo-fixo` (`migrations/000001_initial_baseline.up.sql:516-520`) vs `expense.custo_fixo` (`onboarding_tool_dispatcher.go:259-265`) | Documentar mapa canônico (ver §5); garantir normalização. |
| M-10 | Mapa emoji/label de categoria **duplicado em 3 lugares** | `onboarding_scripts.go:255-287`, `formatting.go:203-218`, `onboarding_tool_dispatcher.go:242-290` | Consolidar fonte única (risco de drift). |

---

## 1. Mapeamento etapa-a-etapa

> Cada etapa: **(a)** texto/regra oficial verbatim → **(b)** artefato real verbatim (`arquivo:linha`) → **(c)** veredito → **(d)** ação para fidelidade (referência RF do PRD).

### ETAPA 1 — Boas-vindas

**(a) Oficial (Cap. 08:399-413):**
```
👋 Oi! Eu sou o MeControla, seu parceiro pra organizar o dinheiro sem complicação.

Em poucos minutos a gente deixa tudo no controle e você começa a acompanhar seus objetivos de forma simples.

Vamos começar? 🚀
```
*Usuário:* `Sim`

**(b) Código real:**
- Prompt LLM de welcome — `internal/agent/application/usecases/onboarding_scripts.go:50-55`:
  ```
  onboardingWelcomeSystemPrompt = "Você é o MeControla, parceiro financeiro no WhatsApp. Esta é a PRIMEIRA mensagem para um usuário recém-ativado. Escreva UMA mensagem curta, calorosa e acolhedora em PT-BR que: (1) se apresente como parceiro financeiro; (2) diga em uma frase que organiza gastos e receitas direto no WhatsApp; (3) termine perguntando qual é o objetivo financeiro principal da pessoa (...)"
  ```
- Fallback determinístico — `onboarding_scripts.go:21-31` (`scriptWelcome`): apresenta as 5 categorias **e** "Etapa 1/4 — Objetivo".
- Disparo via consumer — `onboarding_bound_consumer.go:83-139` com `OnboardingWelcomeSignal = "__onboarding_welcome__"` (`onboarding_agent.go:13`).

**(c) Veredito: 🟡 PARCIAL / 🔴 em pontos**
- Welcome existe e é LLM no tom oficial ✅.
- 🔴 O welcome **já pergunta o objetivo** (funde ETAPA 1+2) e **não** há o handshake "Vamos começar? → Sim".
- 🔴 O fallback **apresenta as 5 categorias** no welcome (deveria ser ETAPA 5).

**(d) Ação:** boas-vindas isolada (apresentação + "Vamos começar? 🚀"), sem pedir objetivo e sem apresentar categorias; **aguardar confirmação do usuário ("Sim")** antes de seguir para a ETAPA 2 (handshake, DP-2 decidido). → RF-04.

---

### ETAPA 2 — Definição do Objetivo

**(a) Oficial (Cap. 08:417-445):**
```
🎯 Antes da gente falar de números, me conta uma coisa:

Qual objetivo você quer alcançar organizando melhor seu dinheiro?

Exemplos:
• Quitar dívidas
• Fazer uma viagem
• Comprar uma casa
• Criar uma reserva
• Sair do aperto financeiro
```
*Ack oficial:*
```
🎯 Perfeito!

Vamos montar tudo pensando nesse objetivo.
```

**(b) Código real:**
- Prompt da fase — `onboarding_scripts.go:208` (dentro de `onboardingDataPhasePrompt`, case `OnbPhaseObjective`):
  ```
  "Etapa: objetivo principal. SEMPRE chame save_onboarding_objective com o texto do objetivo EXATAMENTE como a pessoa escreveu (...). Qualquer objetivo informado já é válido e suficiente — NUNCA peça para a pessoa detalhar (...)"
  ```
- Ack — `onboarding_tool_dispatcher.go:89`:
  ```
  "🎯 Anotado: seu foco é *%s*. Vou usar isso pra te manter motivado!"
  ```
- Erro — `:84`: `"Não consegui entender seu objetivo. Pode me contar de novo, com poucas palavras? 😊"`
- Persistência — `save_onboarding_objective.go` (VO `FinancialObjective`, máx 280 chars).

**(c) Veredito: 🟡 PARCIAL**
- Persistência e ack existem, ordem correta ✅; tom LLM aceitável (D-01).
- 🟡 A **pergunta** do objetivo está embutida no welcome (M-7) e **não traz a lista de exemplos** oficiais.

**(d) Ação:** ETAPA 2 com pergunta própria + exemplos oficiais; ack codificando "vamos montar tudo pensando nesse objetivo". → RF-05.

---

### ETAPA 3 — Definição do Orçamento

**(a) Oficial (Cap. 08:449-470):**
```
💰 Agora me diga:

Qual o valor disponível do seu orçamento mensal?
```
*Ack oficial:*
```
✅ Orçamento registrado

💰 R$ 4.000
```

**(b) Código real:**
- Prompt — `onboarding_scripts.go:210` (case `OnbPhaseBudget`): `"Etapa: orçamento mensal. Converta o valor para centavos (...) e chame save_onboarding_income."`
- Ack — `onboarding_tool_dispatcher.go:107`: `"✅ Orçamento de *R$ %s* registrado!"`
- Erro — `:102`: `"Esse valor de orçamento não parece certo. Pode me dizer de novo o quanto você tem por mês? 💰"`
- Persistência — `save_onboarding_income.go` (VO `MonthlyIncome`, mín R$ 500 / máx R$ 100M — `monthly_income.go:14-15`).

**(c) Veredito: ✅ FIEL**
- Conteúdo, ordem e ack alinhados; tom LLM aceitável. Sem ação estrutural.
- ⚠️ Observação: piso de R$ 500 (`monthlyIncomeMinCents=50000`) não está no oficial — manter como regra técnica, mas a mensagem de erro deve seguir o tom (Cap. 11 "Receita Sem Valor", RF-07).

**(d) Ação:** nenhuma estrutural; garantir formato "Orçamento registrado + 💰 R$ valor". → RF-06/RF-07.

---

### ETAPA 4 — Cadastro de Cartões

**(a) Oficial (Cap. 08:474-510 + Cap. 10:878-888):**
```
💳 Você usa cartão de crédito?

Se sim, me diga:
• Apelido do cartão
• Dia de vencimento da fatura

Se não usar, é só me avisar 😊
```
*Ack oficial:*
```
✅ Cartão salvo

💳 Nubank
📅 Vencimento: dia 13

Deseja adicionar outro cartão?
```
**Regra (Cap. 10):** solicitar **apenas** apelido + dia de vencimento. Nunca limite, banco, bandeira ou dados sensíveis.

**(b) Código real:**
- Prompt — `onboarding_scripts.go:212` (case `OnbPhaseCards`): `"Etapa: cartões de crédito. O usuário pode enviar vários cartões (...). Para CADA cartão que tiver apelido E dia de fechamento, chame save_onboarding_card (...)"`
- Ack — `onboarding_tool_dispatcher.go:125`:
  ```
  "💳 Cartão *%s* salvo (fecha dia %d 📅). Quer adicionar outro? Se não usa cartão, é só dizer."
  ```
- Input coletado — `save_onboarding_card_input.go:15-19`: `UserID, Nickname, ClosingDay` (1–31). **Não coleta** limite/banco/bandeira/DueDay.
- "Não uso" — lista de negações `onboarding_scripts.go:119-123` (`onboardingNegations`).

**(c) Veredito: 🟡 PARCIAL / 🔴 no campo**
- Privacidade ✅ (só apelido + dia; nada sensível — fiel ao Cap. 10).
- Loop de N cartões ✅; caminho "não uso" ✅.
- 🔴 **Campo divergente:** oficial pede **dia de vencimento da fatura**; código coleta e exibe **dia de fechamento** (`ClosingDay` / "fecha dia"). **✅ DECIDIDO (DP-1):** coletar **somente o dia de vencimento**.

**(d) Ação:** coletar **apenas apelido + dia de vencimento** (due day), removendo o fechamento do onboarding; ack "✅ Cartão salvo / 💳 Nick / 📅 Vencimento: dia N / Deseja adicionar outro cartão?". Reconciliação com `card.CreateCard` (que exige `ClosingDay`) é da techspec — **QT-CARD** (§4). → RF-08/RF-09/RF-10.

---

### ETAPA 5 — Apresentação das Categorias

**(a) Oficial (Cap. 08:514-543):**
```
📊 Antes de montar seu planejamento, deixa eu te mostrar como organizamos o dinheiro por aqui.

Tudo vive em apenas 5 categorias:

💰 Custo Fixo
🎓 Conhecimento
🎉 Prazeres
🎯 Metas
🏦 Liberdade Financeira

Faz sentido? 😊
```
*Ack oficial:*
```
Perfeito!

Agora vamos montar seu planejamento.
```

**(b) Código real:**
- ⬛ **Não há etapa/prompt próprio.** A apresentação das 5 categorias só existe dentro do `scriptWelcome` (fallback de boas-vindas) — `onboarding_scripts.go:23-28` — e **não** após os cartões.
- Não há fase entre `OnbPhaseCards` e `OnbPhaseFinancialPlan` para apresentar categorias.

**(c) Veredito: ⬛ AUSENTE (como etapa) / 🔴 fora de ordem (no welcome)**

**(d) Ação:** criar **ETAPA 5** própria após os cartões, apresentando as 5 categorias oficiais e confirmando entendimento ("Faz sentido?"); remover a apresentação do welcome (M-2). → RF-11/RF-12.

---

### ETAPA 6 — Definição dos Valores das Categorias

**(a) Oficial (Cap. 08:547-564 + Cap. 10:842-864):**
```
MeControla: 💰 Quanto deseja definir para Custo Fixo?
Usuário: 2000
MeControla: ✅ Custo Fixo definido — R$ 2.000
            🎓 Quanto deseja definir para Conhecimento?
```
*(repetir até concluir as 5 categorias)*
**Regra:** o usuário **sempre** informa valores monetários; o sistema calcula percentuais.

**(b) Código real:**
- Prompt (em bloco) — `onboarding_scripts.go:218` (case `OnbPhaseFinancialPlan`):
  ```
  "Etapa: distribuição do orçamento EM REAIS por categoria. Converta cada valor para centavos e mapeie os nomes para root_slug: Custo Fixo->expense.custo_fixo (...). Envie as 5 categorias (0 para as não citadas) e chame save_onboarding_budget_splits."
  ```
- **Auto-sugestão** (não-oficial) — `onboarding_scripts.go:58-65` (`buildAutoSplitPreview` "📊 Aqui está uma sugestão de distribuição...") + `suggest_budget_split.go` (templates por perfil em `objective_profile.go:120-158`).
- Ack de sucesso (com %) — `onboarding_tool_dispatcher.go:230-240` (`splitsSuccessReply`): `"%s %s: R$ %s (%d%%)"`.
- Mismatch — `:220-228` (`splitsMismatchReply` "⚠️ Quase! Você distribuiu R$ X, mas seu orçamento é R$ Y...").
- Persistência — `save_onboarding_budget_splits.go`; invariante **exatamente 5** + soma == renda (`budget_allocation.go:30-32,52-54`).

**(c) Veredito: 🔴 DIVERGENTE**
- 🔴 Coleta **em bloco** (5 numa mensagem), não **uma por vez** (viola Regra 1 + fluxo oficial).
- 🔴 **Auto-sugestão de split** não existe no oficial (o usuário sempre informa valores).
- ✅ Cálculo de percentual a partir de R$ existe (basis points; `money.go:41-43`); ✅ invariante 5 categorias + soma.

**(d) Ação:** perguntar **categoria por categoria** (5 perguntas, ack por categoria como "✅ Custo Fixo definido — R$ X" seguido da próxima); **remover a auto-sugestão do caminho oficial** (DP-3 decidido — o usuário sempre informa os valores). → RF-13/RF-14.

---

### ETAPA 7 — Resumo Final

**(a) Oficial (Cap. 08:568-599):**
```
✅ Planejamento criado!

🎯 Objetivo:
Quitar dívidas

💰 Orçamento:
R$ 4.000

📊 Distribuição

💰 Custo Fixo
R$ 2.000 (50%)

🎓 Conhecimento
R$ 300 (7,5%)
... (todas as 5, valor + percentual) ...

Está tudo certo? 😊
```

**(b) Código real:**
- Quase-resumo no passo de valores — `onboarding_scripts.go:67-89` (`buildFinancialPlanMessage`): mostra Objetivo + Orçamento + Cartões + "Distribuição Final" e termina com:
  ```
  "\nResponde *sim* pra confirmar, ou me diz como quer distribuir."
  ```
  ⚠️ Esse builder lista **apenas R$** (`formatBRLCents`), **sem percentual** — o percentual só aparece no `splitsSuccessReply`.
- ⬛ **Não há** passo de resumo próprio após a coleta de valores, nem gate de confirmação durável tipado, nem correção de objetivo/orçamento/cartões (só re-distribuir).

**(c) Veredito: 🟡 PARCIAL / 🔴**
- 🟡 Existe um resumo-like, mas é o passo de valores, não uma ETAPA 7 distinta.
- 🔴 Exibe só R$ no plano (oficial exige **valor + percentual**).
- 🔴 Sem correção guiada por LLM (objetivo/orçamento/cartões/valores); sem gate HITL durável tipado.

**(d) Ação:** criar **ETAPA 7** própria: resumo consolidado com **valor + percentual** por categoria + "Está tudo certo?"; gate HITL durável e tipado; **correção guiada por LLM** (D-05). → RF-16/RF-17/RF-18.

---

### ETAPA 8 — Conclusão

**(a) Oficial (Cap. 08:603-618 + Cap. 07:343-367):**
```
🚀 Seu planejamento está pronto!

Agora é só me enviar suas movimentações normalmente.

Exemplos:
• Mercado 120 pix
• Uber 35 Nubank
• Recebi salário 4000
• Como estou esse mês?
• Quanto ainda posso gastar?
```
**Jornada (Cap. 07):** *Conclusão do Onboarding* → **depois** *Operação Diária*. A primeira movimentação é Operação Diária, **não** parte do onboarding.

**(b) Código real:**
- Passo extra `OnbPhaseFirstTx` (não-oficial) — `run_onboarding_turn.go` + schema `onboarding_structured_schema.go` (`onboarding_first_tx`).
- Conclusão exige 1ª transação — `complete_onboarding_session.go:22` (`ErrOnboardingFirstTransactionRequired`), `:73`; e `onboarding_session.go:189-194`:
  ```go
  func (s OnboardingSession) IsReadyToComplete() bool {
      return s.payload.FirstTxRecorded &&
          strings.TrimSpace(s.payload.Objective) != "" &&
          s.payload.IncomeCents > 0 &&
          len(s.payload.CustomSplit) == 5
  }
  ```
- Mensagem de conclusão — `onboarding_tool_dispatcher.go` const `onboardingCompletedReply`:
  ```
  "🎉 *Onboarding concluído!* Agora é só me chamar: registrar gastos, ver fatura do cartão, acompanhar metas e pedir o resumo do mês. Conta comigo! ✅"
  ```
- Evento — `onboarding.completed` (`onboarding_session_events.go:49`).

**(c) Veredito: 🔴 DIVERGENTE**
- 🔴 Conclusão **gated na 1ª transação** (passo `first_tx` + `ErrOnboardingFirstTransactionRequired` + `FirstTxRecorded` em `IsReadyToComplete`) — contraria Cap. 07/08.
- 🟡 Mensagem de conclusão existe, mas **sem a lista de exemplos** de uso diário do oficial.

**(d) Ação:** concluir **após a confirmação do Resumo** (ETAPA 7), removendo `OnbPhaseFirstTx`, `ErrOnboardingFirstTransactionRequired` e `FirstTxRecorded` de `IsReadyToComplete`; mensagem de conclusão com **exemplos** de uso diário. → RF-19/RF-20/RF-21.

---

## 2. Regras transversais (Cap. 10) — veredito

| Regra oficial | Evidência de código | Veredito | Ação |
|---|---|---|---|
| Cartão: só apelido + dia; nunca limite/banco/bandeira | `save_onboarding_card_input.go:15-19` (só Nickname+ClosingDay) | ✅ FIEL (privacidade) | Manter |
| Cartão: "dia de **vencimento** da fatura" | código usa `ClosingDay` ("fechamento") | 🔴 DIVERGENTE | ✅ DP-1: coletar só vencimento (QT-CARD) |
| 5 categorias oficiais, nomes exatos | BD `migrations/...:516-520` ("Custo Fixo"…"Liberdade Financeira") | ✅ FIEL (nomes) | Manter |
| Nenhuma categoria adicional | invariante "exatamente 5" `budget_allocation.go:30-32` | ✅ FIEL | Manter |
| Entrada por valores; sistema calcula % | basis points `money.go:41-43`, `splitsSuccessReply` mostra % | ✅ FIEL (cálculo) | Exibir % no resumo (ETAPA 7) |
| Exibição sempre "valor + percentual" | `buildFinancialPlanMessage` mostra só R$ | 🟡 PARCIAL | Resumo com valor+% (RF-16) |
| Emojis oficiais 💰🎓🎉🎯🏦 | presentes em 3 mapas (`onboarding_scripts.go`, `formatting.go`, `onboarding_tool_dispatcher.go`) | ✅ FIEL / ⚠️ duplicado | Consolidar (M-10) |

---

## 3. Invariantes técnicas e estados

| Item | Estado atual | Alvo de fidelidade |
|---|---|---|
| `phase` | `string` livre (`onboarding_session.go:76`) | Tipo fechado `OnboardingPhase` (RF-22) |
| Estado de sessão | `OnboardingStateInProgress`/`Active` (`onboarding_state.go:3-6`) | Manter; `InProgress = CompletedAt == nil` |
| Fases reais | 5 (`objective…first_tx`) | 8 etapas (welcome, objetivo, orçamento, cartões, categorias, valores, resumo, conclusão) |
| `IsReadyToComplete` | exige `FirstTxRecorded` | remover `FirstTxRecorded` (RF-19) |
| Headers | "🔵 Etapa X/4" | refletir 8 etapas / sem numeração "/4" |
| Eventos emitidos | `onboarding.card_registered`, `onboarding.splits_calculated`, `onboarding.completed`, `onboarding.subscription_bound` | ✅ manter (idempotência por `event_id`, RF-28) |
| recent_turns | máx 3 pares (`onboarding_session.go:196` `maxRecentTurnPairs=3`) | ✅ manter (RT-15) |
| Abandono | inexistente | adicionar `onboarding.step_abandoned` + funil (RF-30) |

---

## 4. Decisões resolvidas (fechadas pelo usuário — "seguir o oficial")

> Resolvidas em 2026-06-25. Diretriz: **seguir fielmente a Cap. 08 do documento oficial, ETAPA 1 → ETAPA 8, sem flexibilização.**

- **DP-1 — Cartão: SOMENTE dia de vencimento. ✅ DECIDIDO.** O onboarding coleta **apenas apelido + dia de vencimento da fatura** (due day), conforme oficial (Cap. 08 ETAPA 4 + Cap. 10). **Não** coletar dia de fechamento, limite, banco ou bandeira. O ack exibe "📅 Vencimento: dia N". A reconciliação com o agregado `card` (que hoje exige `ClosingDay`) é responsabilidade da techspec — ver **QT-CARD** abaixo.
- **DP-2 — Handshake de boas-vindas. ✅ DECIDIDO (incluir).** A ETAPA 1 é só apresentação + "Vamos começar? 🚀" e aguarda a confirmação do usuário ("Sim") antes de seguir para o objetivo (ETAPA 2), fiel ao Cap. 08 ETAPA 1.
- **DP-3 — Sem auto-sugestão de split. ✅ DECIDIDO (fora do caminho oficial).** Na ETAPA 6 o usuário **sempre** informa os valores monetários, **categoria por categoria**; `suggest_budget_split`/`buildAutoSplitPreview` saem do caminho oficial do MVP (Cap. 08 ETAPA 6 + Cap. 10 — Regra de Distribuição).

**QT-CARD (para a techspec):** o onboarding passa a coletar apenas `DueDay` (vencimento). O `card.CreateCard` hoje exige `ClosingDay` (1–31) e tem `DueDay` opcional (`create_card.go:9-16`). A techspec deve definir como o cartão é criado com apenas o vencimento (ex.: `DueDay` obrigatório e `ClosingDay` derivado/assumido, ou ajuste do contrato do módulo `card`), sem pedir o fechamento ao usuário.

---

## 5. Mapa canônico consolidado de categorias (referência para a techspec)

| Oficial (Cap. 10) | Emoji | BD slug (`categories`) | Onboarding root_slug (eventos/prompt) | `CategoryKind` (uint8) | `CategoryKind.String()` |
|---|---|---|---|---|---|
| Custo Fixo | 💰 | `custo-fixo` | `expense.custo_fixo` | `CategoryKindFixedCost` (1) | `fixed_cost` |
| Conhecimento | 🎓 | `conhecimento` | `expense.conhecimento` | `CategoryKindKnowledge` (2) | `knowledge` |
| Prazeres | 🎉 | `prazeres` | `expense.prazeres` | `CategoryKindPleasures` (3) | `pleasures` |
| Metas | 🎯 | `metas` | `expense.metas` | `CategoryKindGoals` (4) | `goals` |
| Liberdade Financeira | 🏦 | `liberdade-financeira` | `expense.liberdade_financeira` | `CategoryKindFinancialFreedom` (5) | `financial_freedom` |

> ⚠️ **M-9:** o BD usa hífen (`custo-fixo`) e o onboarding usa `expense.custo_fixo` (ponto + underscore). Qualquer junção entre camadas deve normalizar — a techspec precisa definir a fonte única de verdade do slug.

---

## 6. Checklist de fidelidade (gate de aceite da implementação)

- [ ] 8 etapas distintas, na ordem do Cap. 07, sem "Etapa X/4".
- [ ] ETAPA 1 só boas-vindas + handshake "Vamos começar?" (aguarda "Sim"); sem pedir objetivo nem apresentar categorias.
- [ ] ETAPA 2 pergunta o objetivo com exemplos oficiais; ack no tom oficial.
- [ ] ETAPA 3 registra orçamento; ack "Orçamento registrado + R$".
- [ ] ETAPA 4 coleta **só apelido + dia de vencimento** (sem fechamento/limite/banco/bandeira); loop de N cartões; caminho "não uso".
- [ ] ETAPA 5 apresenta as 5 categorias após cartões e confirma entendimento.
- [ ] ETAPA 6 coleta valor **categoria por categoria** (usuário sempre informa; sem auto-sugestão); ack por categoria.
- [ ] ETAPA 7 resumo com **valor + percentual** + "Está tudo certo?" + correção guiada por LLM + gate HITL durável.
- [ ] ETAPA 8 conclui após o resumo (sem 1ª transação); mensagem com exemplos; emite `onboarding.completed`.
- [ ] `phase` como tipo fechado; `IsReadyToComplete` sem `FirstTxRecorded`.
- [ ] Mensagens geradas por LLM no tom oficial (Cap. 03–06), codificando o conteúdo oficial de cada etapa.
- [ ] `onboarding.step_abandoned` + funil por etapa, cardinalidade controlada.
- [ ] Propagação por evento idempotente (cartões→`card`, splits→`budgets`, conclusão→`agent`/`identity`).

---

## 7. Apêndice — arquivos-fonte auditados

- `internal/agent/application/usecases/run_onboarding_turn.go` — orquestração de fases.
- `internal/agent/application/usecases/onboarding_scripts.go` — prompts, scripts, headers, builders, emoji/label.
- `internal/agent/application/usecases/onboarding_structured_schema.go` — schemas JSON por fase.
- `internal/agent/application/usecases/onboarding_tool_catalog.go` — nomes de tools + slugs.
- `internal/agent/infrastructure/onboarding/onboarding_tool_dispatcher.go` — acks, replies, mapas de categoria.
- `internal/agent/application/services/onboarding_agent.go` — entrypoint + sinal de welcome.
- `internal/agent/infrastructure/messaging/database/consumers/onboarding_bound_consumer.go` — disparo do welcome.
- `internal/onboarding/application/usecases/*` — persistência por etapa, conclusão, fase, turns.
- `internal/onboarding/domain/entities/onboarding_session.go` — payload, `IsReadyToComplete`, recent_turns.
- `internal/onboarding/domain/valueobjects/*` — VOs (objetivo, renda, cartão, alocação, category kind).
- `internal/onboarding/domain/entities/onboarding_session_events.go` — eventos emitidos.
- `internal/card/application/dtos/input/create_card.go` — campos do cartão (closing/due day).
- `migrations/000001_initial_baseline.up.sql:516-520` — seed das 5 categorias.
