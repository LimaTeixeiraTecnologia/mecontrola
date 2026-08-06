# Correção dos 4 defeitos de produção — confirmação, orçamento e cartão via áudio

- **Data**: 2026-08-06
- **Usuário afetado (reprodução)**: `9f79db88-ab08-4b3a-b493-de11c3c0c30d` / `+5511930111763` / `stefanykelly.lima@hotmail.com`
- **Janela investigada**: 2026-08-06 10:37–11:00 BRT (13:37–14:00 UTC)
- **Fontes**: ssh `mecontrola-vps`, Postgres `mecontrola_db`, logs Docker Swarm, `platform_runs`, `workflow_runs`, `platform_messages`, `agents_whatsapp_audio_messages`, `agents_write_ledger`, `cards`, `banks`
- **Status**: **IMPLEMENTADO e validado** (build, vet, test -race, lint 0 issues, gate real-LLM ≥0,90 e gate de áudio 1.0000). **Não commitado, não deployado.**

## Decisão de padrão (design-patterns-mandatory)

Seletor determinístico executado antes de qualquer código:

| Decisão | Entrada | Saída do seletor | Conclusão aplicada |
|---|---|---|---|
| Parser de confirmação | `prefer_direct_solution`, `single_variant_only`, `low_change_frequency` | `reject` | **Não aplicar padrão** — função pura `DecideConfirmAnswer`, sem indireção |
| Coleta de slots do cartão | `state_transition_driven_behavior`, `fixed_workflow_with_variable_steps`, `snapshot_and_restore` | `Memento` (4) empatado com `State` (4) | **Memento já existe** no kernel (`workflow.Snapshot`) — consumir, não recriar (regra de ouro da skill mastra). Para o despacho, `State` foi rejeitado em favor da sua alternativa simples documentada, "enum com regras localizadas", que é exatamente o padrão já usado por `budget-manage` |

---

## Contexto

**A transcrição de áudio NÃO é o problema.** As 18 mensagens de áudio da sessão estão todas
`outcome=dispatched / reason=approved`, com texto correto (ex.: *"Pode registrar sim."*,
*"Eu quero o custo fixo R$ 2.000, conhecimento R$ 1.000…"*, *"O banco é XP mesmo."*).
`AGENT_AUDIO_ENABLED=true`, `AGENT_STT_MODEL=openai/whisper-large-v3`, custo ~93–600 µUSD/áudio.

O áudio apenas **expõe** defeitos determinísticos pré-existentes: o Whisper devolve frases naturais
com pontuação, enquanto os parsers do repositório só aceitam tokens isolados sem pontuação.

### Danos comprovados

| Evidência | Fato |
|---|---|
| `agents_write_ledger` | A despesa de R$ 10 da padaria (10:38) **nunca foi gravada** — cancelada silenciosamente |
| `cards` | Só existe o Nubank do onboarding; o cartão XP **nunca foi criado** |
| `workflow_runs` | `card-manage`: **zero linhas em toda a base**, desde sempre |
| `platform_runs` | Todos os turnos de cartão/orçamento com `workflow=''`, `outcome=routed` → nunca entraram em workflow |
| Agregado histórico | 7× "Não entendi", **5× "registro foi cancelado"** (2026-08-01, 08-03, 08-06) → afeta mais usuários |

---

## Defeito 1 — Parser de confirmação (severidade máxima: perda silenciosa de dados)

`internal/agents/application/workflows/write_shared.go:110-111`

```go
reConfirmYes = regexp.MustCompile(`(?i)^(sim|confirmar|confirma|ok|pode)$`)
reConfirmNo  = regexp.MustCompile(`(?i)^(não|nao|cancela|cancels|deixa\s+pra\s+lá|não\s+registra)$`)
```

Ancorado, aplicado sobre `strings.TrimSpace` puro (`transaction_write_decisions.go:194`) — sem
remoção de pontuação nem de acento. Consequência: **até "Sim." com ponto final falha.**
1º ambíguo → reprompt; 2º ambíguo → `CANCEL` (`transaction_write_decisions.go:204-206`).

Produção: `"Pode registrar sim."` → "Não entendi"; `"Pode registrar."` → "registro foi cancelado".

Compartilhado por 5 workflows: `transaction_write_decisions.go:196`, `card_manage_decisions.go:52`,
`budget_manage_decisions.go:104`, `goal_edit_decisions.go:38`, `destructive_confirm_decisions.go:30`
(este via os duplicados `isSim`/`isNao`, `write_shared.go:164-180`).

### Correção
Novo `Decide*` puro em `write_shared.go` (sem IO, sem `context`, determinístico):

```go
type ConfirmAnswer int
const (
    ConfirmAnswerYes ConfirmAnswer = iota + 1
    ConfirmAnswerNo
    ConfirmAnswerAmbiguous
)
func DecideConfirmAnswer(text string) ConfirmAnswer
```

- Reusar `normalizeText` (`write_shared.go:487`) — já faz lower + NFD + remove acento e markdown.
- Remover pontuação de borda; casar **frases** afirmativas/negativas por token, não string inteira.
- **Negação tem precedência**: `"não pode"` / `"pode não"` → `No`. Só emite `Yes` na ausência de
  qualquer token de negação. Isso é o que impede falso-aceite.
- `ConfirmAnswer` é tipo fechado com `String()`/`IsValid()` (DMMF state-as-type).
- Os 5 workflows passam a chamar `DecideConfirmAnswer`; `isSim`/`isNao` e os dois regex são removidos.

**Não alterar** TTL, `maxReprompts=1` nem a política de cancelamento do ADR-003 — a correção remove
o *gatilho* falso, preservando o contrato HITL (R-AGENT-WF-001.7-A). Destrutivo usa o mesmo parser.

---

## Defeito 2 — Orçamento: falso bloqueio + pergunta inventada (4 correções)

Duas causas encadeadas:

**2a.** O pre-guard `multi_item` (`guards/multi_item.go:70-83`) bloqueia qualquer mensagem com
**≥2 tokens monetários**, sem nenhuma checagem semântica. A frase da usuária tem 5 (`R$ 2.000`,
`R$ 1.000`, `R$ 1.000`, `R$ 250`, `R$ 750`). A escapatória `IsCorrectionOrEditIntent`
(`multi_item.go:25`) **não lista `ajust*`** — embora `budget_write_shortcut.go:131` já liste
"ajustar". Roda em `guard_chain.go:61-80`, antes da LLM → é o `duration_ms=6` do `platform_runs`.
O post-guard `multi_item_false_block` não salva porque reusa o **mesmo** predicado
(`multi_item_false_block.go:36`), logo nunca contradiz o pre-guard.

**2b.** A pergunta *"precisamos definir a distribuição do seu orçamento de R$ 5.000,00 entre as
categorias"* **não existe no repositório** (`grep` retorna zero) — foi inventada pela LLM, sem
`adjust_allocation` por trás. Sem run suspenso, a resposta caiu no guard chain. Beco sem saída:
qualquer resposta conterá ≥2 valores e será bloqueada de novo.

### Correção (as 4 camadas, conforme decidido)
1. **Isenção de contexto no `multi_item`**: adicionar `ajust(ar|e|a)` ao `multiItemCorrectionRe` e um
   predicado puro `DetectBudgetAllocationIntent` (nomes de categoria + verbos de orçamento) que isenta
   a mensagem. Aplicar **no predicado compartilhado**, para o post-guard `multi_item_false_block`
   herdar a mesma isenção.
2. **Reordenar guards + aceitar R$**: mover `budget_write_shortcut` acima de `multi_item`
   (`mecontrola_agent.go:322-366`) e ampliar `parseBudgetDistributionShortcut`
   (`budget_write_shortcut.go:104-115`) para aceitar valores em R$ — hoje exige `%` literal.
3. **Post-guard `budget_slot_without_tool`**: espelhar `category_without_tool.go` — se a resposta
   perguntar slot de orçamento sem tool de orçamento chamada, força retry.
4. **System prompt**: exemplares "refazer/reconfigurar orçamento" → `create_budget` /
   `adjust_allocation` (`mecontrola_agent.go:115-116,189`).

Reuso: `BudgetManageOpEditDistribution` e o slot `distribution` **já existem**
(`budget_manage_state.go:11-17,51-58`); `handleBudgetManageDistributionSlot` já aceita R$ **ou** %
(`budget_manage_workflow.go:201-211`). Nada de workflow novo — só fazer a frase chegar até ele.

---

## Defeito 3 — Cartão: slot gathering sem estado durável

`tools/create_card.go` devolve `needs_slot` (:89-94) e `needs_closing` (:108-112) **antes** de
`engine.Start` (:134). Nada é persistido → nenhum run suspenso → `SuspendedRunIndex` não acha nada →
cada resposta é turno novo de LLM, que precisaria re-enviar os 4 slots juntos. Nunca faz. Loop eterno.

**Viola R-AGENT-WF-001.7**: "proibido pedir clarificação sem salvar o estado de retomada".

`BankRecognized("XP")=false` está **correto** (tabela `banks` tem 8 linhas, XP ausente) — não é o bug.
O guard `create_card_shortcut` não disparou (exige "cadastr|criar|adicionar"; usuária disse "registrar").

### Correção — workflow durável de slots (espelha `budget-manage`)
- `card_manage_state.go`: enum fechado `CardManageAwaiting` (nickname/bank/due_day/closing_day/confirm)
  com `String()`/`IsValid()`/`Parse*`, + campos `Awaiting`, `BankChecked`, `BankRecognized`,
  `SlotReprompt`, `ClosingDayEcho`.
- `card_manage_decisions.go`: `DecideCardManageNextSlot` e 4 `Decide*` puros por slot.
  Desambiguação de **"Dia 10"**: se o dia informado == `DueDay` já conhecido, re-pergunta **uma vez**
  nomeando fechamento vs vencimento; na repetição aceita. Limitado por `ClosingDayEcho`, termina sempre.
- `card_manage_workflow.go`: dispatcher por `Awaiting` (padrão de `budget_manage_workflow.go:74-91`),
  `cardManageEnter`, `cardManageAdvanceCreate`, 4 handlers de slot + helpers
  `cardManageSuspend/Complete/Fail/ExpireStep`.
- `tools/create_card.go`: vira adapter fino — `engine.Start` imediato com slots parciais; remover
  `createCardMissingSlot` e a chamada `BankRecognized` (passa a ser IO do workflow); `createCardOutcomeFor`
  preserva as 3 constantes de outcome do contrato atual.
- `module.go:513`: ajustar a assinatura de `BuildCreateCardTool`.

### Fluxo resultante (zero LLM após o turno 1)

| Turno | Rota | Estado |
|---|---|---|
| "registrar um cartão... XP... vencimento dia 10" | agent → `create_card` → `engine.Start` | run **inserido**, `awaiting=nickname`, suspenso |
| "Roxinho" | `SuspendedRunIndex` → `ContinueCardManage` | `Nickname=Roxinho`, `awaiting=bank` |
| "O banco é XP mesmo." | resume | `Bank=XP`, `BankRecognized=false`, `awaiting=closing_day` |
| "Dia 10" | resume | `Disambiguate`, `ClosingDayEcho=1`, segue `awaiting=closing_day` |
| "10" | resume | `ClosingDay=10`, `awaiting=confirm` |
| "sim" | resume | `executeCardManageCreate` → linha em `cards` |

### Guardas de 0-regressão (obrigatórias)
- **Migração de runs legados**: `Awaiting == 0 && ResumeText != "" ⇒ Confirm`. Antes desta mudança a
  única suspensão possível era o confirm, então snapshots em voo continuam corretos. Sem migração de dados.
- **TTL**: `SuspendedAt` reinicia a cada *avanço* de slot (não em reprompt); slots usam
  `cardManageSlotTTL=30m`; o confirm mantém `cardManageConfirmTTL=15m` intacto. Sem isso, expirar no
  meio da coleta devolve `handled=false` e **o loop volta**.
- **Reaper**: manter `CardManageStaleAfter=35m` > 30m para o TTL sempre vencer primeiro.
- **Edit path byte-idêntico**: `CardManageOpEdit` entra direto em `Awaiting=Confirm`.
- **Onboarding intocado**: `onboarding_workflow.go:1436` chama `cards.CreateCard` direto, não passa
  pela tool nem pelo workflow.
- **`workflow_runs_active_key_uidx`**: manter o branch `errors.Is(err, wf.ErrRunAlreadyExists)` como
  rede de segurança; revisar o texto de `messages.PendingCardCreationExists()`, que hoje assume yes/no.
- **Fora de escopo (registrar, não corrigir agora)**: `executeCardManageCreate` grava
  `ProcessedMessageID = state.MessageID` (o wamid da tool-call) mas `DecideCardManageConfirmation`
  compara com o wamid da confirmação — `CardManageActionReplay` nunca dispara para cartão.

---

## Defeito 4 — Alucinação "não consigo processar áudio"

`grep -i audio internal/agents/application/agents/mecontrola_agent.go` → **zero ocorrências** nas 384
linhas do prompt. A usuária perguntou *"Eu posso configurar de novo o meu orçamento em áudio?"* e o
agente respondeu que não processa áudio — enquanto processava aquele próprio áudio.

**Correção**: uma linha no system prompt — o agente recebe áudio já transcrito e nunca deve negar
essa capacidade.

---

## Arquivos

**Modificar**
- `internal/agents/application/workflows/write_shared.go` — `DecideConfirmAnswer`, remover `isSim`/`isNao`
- `internal/agents/application/workflows/{transaction_write,card_manage,budget_manage,goal_edit,destructive_confirm}_decisions.go` — usar o novo parser
- `internal/agents/application/workflows/card_manage_{state,decisions,workflow}.go` — slots duráveis
- `internal/agents/application/tools/create_card.go` — adapter fino
- `internal/agents/application/agents/guards/{multi_item,multi_item_false_block,budget_write_shortcut}.go`
- `internal/agents/application/agents/mecontrola_agent.go` — ordem dos guards + prompt (orçamento e áudio)
- `internal/agents/module.go` — assinatura de `BuildCreateCardTool`

**Criar**
- `internal/agents/application/agents/guards/budget_slot_without_tool.go`

---

## Verificação

**Ponto cego que deixou isto passar**: o harness golden exercita a LLM, mas o parser de confirmação
vive no caminho de **resume determinístico**, que o golden nunca toca. Fechar isso é parte da correção.

1. **Unit (testify/suite, table-driven, `fake.NewProvider()` — R-TESTING-001)**
   - `DecideConfirmAnswer` com as frases reais de produção: `"Pode registrar sim."`, `"Pode registrar."`,
     `"Tá muito legal esse vlog. Pode registrar sim."`, `"Sim."`, `"sim"`, `"Sim, pode"` → `Yes`;
     `"não"`, `"não pode"`, `"pode não"`, `"cancela"` → `No`; e casos genuinamente ambíguos → `Ambiguous`.
   - Os 4 `Decide*` de cartão, incluindo os **dois turnos** do caso `DueDay=10 → "Dia 10"`.
   - `DetectMultipleMonetaryValues` com a frase real de alocação → não bloqueia; e regressão: frase
     genuinamente multi-item (`"gastei 30 no uber e 50 no mercado"`) → **continua bloqueando**.
2. **Workflow durável**: caminhada de 6 turnos com engine real + store em memória; asserir que existe run
   `suspended` já após o **primeiro** `Start` com só `dueDay`.
3. **Golden real-LLM — OBRIGATÓRIO, mocks não bastam para gate de agente.**

   Uso obrigatório do `/Users/jailtonjunior/Git/mecontrola/.env`, de onde saem
   **`OPENROUTER_BASE_URL`** e **`OPENROUTER_API_KEY`** — as duas variáveis que
   `harness_realllm_test.go:31,35` e `harness_audio_realllm_test.go:148,161` leem via `os.Getenv`.
   Ambas verificadas como presentes no `.env` (`OPENROUTER_BASE_URL=https://openrouter.ai`).

   ```bash
   set -a && . /Users/jailtonjunior/Git/mecontrola/.env && set +a
   RUN_REAL_LLM=1 go test ./internal/agents/application/golden/... -run RealLLM -count=1 -v
   ```

   Critério: suíte completa ≥ 0,90 e **zero falso-sucesso** — inclusive `cases_card.go`,
   `cases_budget.go`, `cases_pending_confirmation.go` e o pareado de áudio.

   O trilho de STT real é separado e exige variáveis adicionais
   (`RUN_REAL_STT=1`, `STT_REAL_AUDIO_FIXTURE`, `harness_audio_realllm_test.go:144,153`);
   `AGENT_STT_MODEL` **não está no `.env`** (só no ambiente de produção), então deve ser exportado
   à parte se esse trilho for executado.
4. **Gates de governança**: R-ADAPTER-001.1 (zero comentários em `.go`), sem SQL em tool/guard,
   estados como tipos fechados, `internal/platform/workflow` sem tipo de domínio.
5. **Validação por risco (AGENTS.md)**: `build`, `vet`, `test -race`, `lint` no módulo alterado.
6. **Prova end-to-end em produção após deploy**, reproduzindo a sessão real por WhatsApp em áudio:
   despesa → `"Pode registrar sim."` grava (checar `agents_write_ledger`); cartão XP → 6 turnos →
   linha em `cards`; distribuição de orçamento em 5 valores R$ → entra em `budget-manage`
   (checar `workflow_runs`); pergunta sobre áudio → sem negativa.

---

## Resultado da validação executada (2026-08-06)

| Gate | Escopo | Resultado |
|---|---|---|
| `gofmt -l` | `internal/ cmd/ configs/` | limpo |
| `go build` | `./...` | OK |
| `go vet` | `./internal/agents/...` (com e sem tag `integration`) | OK |
| `go test` | `./...` | tudo passa |
| `go test -race` | `./internal/agents/...` | sem data race |
| `golangci-lint` (v2 via `.tools/bin`) | `./internal/agents/...` | **0 issues** |
| Golden real-LLM | `TestGoldenRealLLMSuite` (`-tags integration`, `RUN_REAL_LLM=1`) | **PASS** — 568s |
| Golden áudio pareado | `TestGoldenAudioRealLLMSuite` | **PASS** — expense/income/query/edit/confirmation todos `ratio=1.0000` (9/9 cada) |
| R-ADAPTER-001.1 zero comentários | `internal/ configs/ cmd/` | vazio (OK) |
| SQL direto em tool/guard | `tools/`, `agents/` | vazio (OK) |
| Estado como string solta | `CardManageAwaiting` | vazio (OK) |

Nota sobre o `golangci-lint` do PATH: é v1.64.8 e recusa a config v2 do repositório (problema
pré-existente do ambiente, não da mudança). O lint válido foi executado com o binário correto via
`taskfiles/scripts/ensure-golangci-lint.sh` → `./.tools/bin/golangci-lint`.

### Testes adicionados

- `workflows/confirm_answer_test.go` — 39 casos para `DecideConfirmAnswer`, incluindo as frases
  verbatim de produção, todo o conjunto legado (garantia de 0 regressão) e os casos que **devem**
  permanecer ambíguos (perguntas, nova operação, escolha numérica, nome de categoria).
- `workflows/card_manage_slots_test.go` — caminhada durável de 6 turnos reproduzindo exatamente a
  sessão do cartão XP até a criação; descarte de `closingDay` para banco reconhecido; migração de run
  legado sem `awaiting`; expiração de slot; cancelamento após reprompts; `normalizeCardSlotAnswer`
  (inclui o caso acentuado "O banco é XP mesmo."); `DecideCardManageNextSlot`; enum fechado.
- `guards/budget_allocation_exempt_test.go` — isenção do `multi_item` para alocação de orçamento,
  **com casos de regressão provando que multi-lançamento genuíno continua bloqueado**; atalho de
  distribuição aceitando R$; novo post-guard `budget_slot_without_tool`.

### Correções encontradas durante a implementação (achadas pelos próprios testes)

1. **Corrupção de texto acentuado**: a primeira versão de `normalizeCardSlotAnswer` fatiava a string
   original usando offsets de bytes da string normalizada — "é" tem 2 bytes contra 1 de "e", o que
   produzia mojibake em "O banco é XP mesmo.". Reescrito para operar token a token.
2. **Cancelamento agressivo demais em slot**: usar `DecideConfirmAnswer == No` nos slots fazia
   "não sei bem" cancelar o cadastro. Trocado por `isExplicitSlotCancel`, que só cancela em
   cancelamento explícito (`isCancelMessage` ou até 2 tokens, todos negativos).
3. **Semântica preservada de `closingDay`**: o tool antigo descartava o `closingDay` vindo do LLM
   quando o banco era reconhecido. Ao mover o `BankRecognized` para o workflow essa regra foi
   reimplantada em `cardManageAdvanceCreate` e coberta por teste dedicado.

### Fora de escopo, registrado

`executeCardManageCreate` grava `ProcessedMessageID = state.MessageID` (wamid da tool-call) enquanto
`DecideCardManageConfirmation` compara com o wamid da confirmação — `CardManageActionReplay` nunca
dispara para cartão. `budget-manage` acerta isso em `budget_manage_workflow.go:419`. Não corrigido
nesta entrega para não misturar com a correção do loop.

---

## Anexo A — Transcrição de produção (`platform_messages`)

Trechos que provam cada defeito. Horários em BRT.

**Defeito 1 — confirmação (perda de dados):**

```
10:38:16 user      Então, eu gastei 10 reais na padaria. Eu paguei no dinheiro.
10:38:16 assistant 💰 R$ 10,00 | Custo Fixo > Padaria | dinheiro | Posso registrar?
10:38:30 user      Pode registrar sim.
10:38:30 assistant Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar.
10:38:46 user      Pode registrar.
10:38:46 assistant Tudo certo, o registro foi cancelado. 🙂        <-- R$ 10 PERDIDOS

10:45:23 assistant 💰 R$ 150,00 | 📥 Origem: freelance | Posso registrar?
10:45:38 user      Tá muito legal esse vlog. Pode registrar sim.
10:45:38 assistant Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar.
10:45:46 user      Sim
10:45:46 assistant Boa notícia! 🎉                                  <-- só o token isolado funciona
```

**Defeito 2 — orçamento:**

```
10:47:50 assistant ...precisamos definir a distribuição do seu orçamento de R$ 5.000,00
                   entre as categorias...                          <-- texto INEXISTENTE no repo
10:48:42 user      Ah, eu quero fazer ajuste, ó. Eu quero o custo fixo R$ 2.000,
                   conhecimento R$ 1.000, prazeres R$ 1.000, metas R$ 250 e
                   liberdade financeira R$ 750.
10:48:42 assistant Stef, percebi mais de um lançamento na mesma mensagem...  <-- multi_item, 6ms
10:50:05 user      Eu estou editando o meu orçamento, a distribuição... R$ 2.500 no custo
                   fixo, conhecimento R$ 200, prazeres R$ 1.300, metas R$ 500 e
                   liberdade financeira R$ 500.
10:50:05 assistant Stef, percebi mais de um lançamento na mesma mensagem...  <-- de novo, 13ms
```

**Defeito 4 — alucinação de áudio:**

```
10:50:34 user      Eu posso configurar de novo o meu orçamento em áudio?
10:50:34 assistant Stef, desculpe, mas não consigo processar áudio.   <-- a própria msg era áudio
```

**Defeito 3 — cartão (loop, nada criado):**

```
10:58:13 user      Eu queria registrar um cartão de crédito novo que eu tenho, que é o XP
                   e ele tem vencimento para o dia 10.
10:58:13 assistant Qual é o nome do banco associado ao cartão XP?
10:58:35 user      O banco é XP mesmo.
10:58:35 assistant Não reconheço esse banco na minha lista. Qual é o dia de fechamento...?
10:59:01 user      Dia 10
10:59:01 assistant Não reconheço esse banco na minha lista. Qual é o dia de fechamento...?  <-- loop
10:59:35 user      Quero registrar um novo cartão de crédito.
10:59:35 assistant Qual é o apelido que você gostaria de dar a ele?
10:59:56 user      Roxinho
10:59:56 assistant Não reconheço esse banco na minha lista. Qual é o dia de fechamento...?  <-- loop
```

## Anexo B — Estado de produção verificado

```
-- STT saudável: 18/18 áudios aprovados, transcrição correta
select outcome, reason, count(*) from agents_whatsapp_audio_messages
 where user_id='9f79db88-...' group by 1,2;
   dispatched | approved | 18

-- card-manage NUNCA rodou, em toda a base
select workflow, status, count(*) from workflow_runs group by 1,2;
   transaction-write   | succeeded | 57
   destructive-confirm | succeeded |  3
   destructive-confirm | failed    |  2
   budget-manage       | succeeded |  5   (último em 2026-08-03)
   onboarding-workflow | succeeded |  2
   -- card-manage: AUSENTE

-- turnos de cartão/orçamento nunca entraram em workflow
select workflow, outcome, duration_ms from platform_runs
 where resource_id='9f79db88-...' and started_at > now() - interval '5 hours';
   '' | routed | 6      <-- 10:48:42 multi_item, pré-LLM
   '' | routed | 13     <-- 10:50:05 multi_item, pré-LLM
   '' | routed | 1335   <-- 10:58:12 cartão, turno livre de LLM

-- cartão XP nunca criado
select nickname, bank, due_day from cards where user_id='9f79db88-...';
   Nubank | Nubank | 10     (única linha, criada no onboarding em 02/08)

-- banks tem 8 linhas; XP ausente => BankRecognized=false está correto
   banco-do-brasil, bradesco, c6-bank, caixa, inter, itau, nubank, santander

-- despesa da padaria nunca gravada (só 4 escritas hoje, nenhuma às 13:38 UTC)
select operation, created_at from agents_write_ledger where user_id='9f79db88-...';
   register_expense 13:40:24  register_expense 13:40:56
   register_income  13:41:39  register_income  13:45:46

-- blast radius histórico
 dia         | nao_entendi | cancelado | multiitem | banco
 2026-08-06  |      2      |     1     |     2     |   3
 2026-08-03  |      3      |     3     |     0     |   0
 2026-08-01  |      2      |     1     |     0     |   0
```

Configuração de produção confirmada (`docker service inspect`):
`AGENT_AUDIO_ENABLED=true`, `AGENT_STT_MODEL=openai/whisper-large-v3`, `AGENT_STT_TIMEOUT=20s`,
`AGENT_AUDIO_MIN_CONFIDENCE=0.80`, `AGENT_LLM_PRIMARY_MODEL=openai/gpt-4o-mini`.
