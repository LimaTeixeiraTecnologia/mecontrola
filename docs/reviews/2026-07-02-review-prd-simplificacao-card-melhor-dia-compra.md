# Prompt Enriquecido — Revisão Criteriosa do PRD `simplificacao-card-melhor-dia-compra`

> Artefato gerado pela skill `prompt-enricher` em 2026-07-02.
> **Este arquivo é um PROMPT para execução posterior por um agente.** Ele NÃO executa nada por si só.
> A regra "não implemente nada" vale apenas para a etapa de criação deste prompt; o agente que **consumir**
> este prompt DEVE executar o ciclo completo `review → bugfix → review` até `APPROVED` e concluir com o
> veredito de merge para `main` após validação em produção.

---

## 1. Objetivo (contrato inegociável)

Executar a skill `@.claude/skills/review/` de forma **criteriosa, estrita e sem qualquer flexibilização**,
validando o código atual do repositório contra a especificação canônica em
`.specs/prd-simplificacao-card-melhor-dia-compra/` (PRD + techspec + ADRs + tasks + execution reports).

O ciclo só encerra quando **TODOS** os critérios abaixo forem comprovadamente satisfeitos, com evidência
física (file:line, saída de teste, query, log, trace, métrica) — nunca por inferência ou suposição:

- [ ] **100% dos critérios de aceite** de cada task (`## Critérios de Sucesso` / `## Critérios de Aceite`) — **implementados e comprovados**.
- [ ] **DoD 100% atendido** em todas as 9 tarefas (1.0–9.0) — **implementado e comprovado**.
- [ ] **0 gaps.**
- [ ] **0 lacunas.**
- [ ] **0 falsos positivos** (nenhum achado descartado sem prova; nenhum "atendido" sem evidência).
- [ ] **0 ressalvas** — veredito `APPROVED_WITH_REMARKS` **não é aceitável** como estado final; só `APPROVED` encerra.
- [ ] **Todas as Regras de Negócio implementadas** (ver §5).

Se **qualquer** problema for encontrado (achado `critical`/`high`/`medium`/`low`, gap, lacuna, ressalva,
regra de negócio não atendida), acionar `@.claude/skills/bugfix/` e **repetir o ciclo `review → bugfix → review`**
até obter `APPROVED` limpo, sem falso positivo e em conformidade total com a especificação.

> **Proibição explícita nesta fase de enriquecimento:** não implementar, não editar código, não rodar bugfix
> agora. Apenas produzir/salvar este prompt. A execução é do agente que o consumir.

---

## 2. Contexto do repositório (carga base obrigatória)

- **Projeto:** `mecontrola` — monólito modular Go, DDD seletivo (DMMF), arquitetura por bounded contexts.
- **Diretório raiz:** `/Users/jailtonjunior/Git/mecontrola` · **Branch base de merge:** `main`.
- **Governança canônica:** ler `AGENTS.md` no início; respeitar TODAS as regras `.claude/rules/*`
  (R-ADAPTER-001, R-DTO-VALIDATE-001, R-TXN-WORKFLOWS-001, R-AGENT-WF-001, R-WF-KERNEL-001, R-TESTING-001, R-GOV-001).
- **Skills obrigatórias para código Go:** `.agents/skills/go-implementation/SKILL.md` (Etapas 1–5, R0–R7 `[HARD]`).
  Toda revisão de arquivo `.go` DEVE confrontar: **zero comentários** em produção, sem `panic`, sem `init()`,
  sem `_ = variável` para silenciar não-uso, `context.Context` na fronteira de IO, `errors.Join`/wrapping `%w`,
  interface no consumidor.
- **Módulos no escopo desta entrega:** `internal/card` (núcleo), `internal/budgets` (remoção cirúrgica),
  `internal/agents` (onboarding), com **não-regressão obrigatória** de `internal/transactions` (RF-14).

### Especificação a validar (fonte de verdade — ler integralmente)

| Documento | Caminho |
|---|---|
| PRD (spec-version 2, RF-01..RF-20) | `.specs/prd-simplificacao-card-melhor-dia-compra/prd.md` |
| Techspec | `.specs/prd-simplificacao-card-melhor-dia-compra/techspec.md` |
| ADR-001 (bank texto livre, sem FK) | `.../adr-001-bank-free-text-no-fk.md` |
| ADR-002 (closing_day derivado+cache) | `.../adr-002-closing-day-derived-cached.md` |
| ADR-003 (purchase-day serviço puro) | `.../adr-003-purchase-day-pure-service.md` |
| ADR-004 (remoção budgets/card-limit) | `.../adr-004-budgets-cardlimit-removal.md` |
| ADR-005 (consolidação nickname) | `.../adr-005-consolidate-nickname.md` |
| Tasks (9 tarefas) + execution reports | `.../tasks.md`, `.../<n>.0_execution_report.md`, `.../task-<n>.0-*.md` |

---

## 3. Critérios de aceite a confrontar (Requisitos Funcionais — obrigatório 1:1)

Confrontar **cada RF** contra o código real (file:line) e marcar `atendido` (com evidência no diff/código),
`não atendido` (achado bloqueante `high`+) ou `não verificável` (registrar como lacuna a fechar — **não é aprovação**).

- **RF-01** CreateCard exige exatamente 3 dados de negócio: identificação (nome/apelido), banco, `due_day` (1..31).
- **RF-02** Identificação consolidada em **um único campo** (apelido), preservando unicidade por usuário (ADR-005).
- **RF-03** Banco emissor como **texto livre** normalizado; banco não catalogado **não** é erro de validação.
- **RF-04** `closing_day` deixa de ser entrada; **derivado** de banco+`due_day` e **persistido como cache** (ADR-002).
- **RF-05** Remoção **total** de `limit_cents`/`UpdateCardLimit` de `internal/card` (coluna, entidade, VO `CardLimit`, DTOs, usecase, handler, rota `PATCH /cards/{id}/limit`).
- **RF-06** Remover a regra antiga `dueDay = closingDay + 7` do CreateCard (relação invertida).
- **RF-07** UpdateCard permite alterar identificação/banco/`due_day`; recalcula e re-persiste `closing_day`; mantém versão otimista.
- **RF-08** Domain service **puro** (sem IO, sem `context.Context`, determinístico): `fechamento = due_day - dias_antes`, `melhor_dia = fechamento + 1`.
- **RF-09** `dias_antes` vem de **tabela persistida** banco→dias; banco ausente → **fallback 7 dias**.
- **RF-10** Tabela administrável sem deploy; seed inicial: **Nubank 7, Itaú 8, Santander 8, Bradesco 7, Banco do Brasil 7, Caixa 7, Inter 7, C6 Bank 7**.
- **RF-11** Tratar virada de mês / dias inválidos reutilizando `clamp`/`advanceMonth` de `billing_cycle.go`; resultado sempre 1..31.
- **RF-12** "Melhor dia de compra" é **consulta pura** — não exige cartão persistido.
- **RF-13** Exposto em (a) `GET /cards/best-purchase-day` (query `bank`, `due_day`) **e** (b) campos derivados `closing_day`/`best_purchase_day` na resposta de Create/Update/leitura. Exemplo testável: **Nubank / venc. 20 → fechamento 13 → melhor dia 14**.
- **RF-14** `internal/transactions` **sem alteração de contrato de leitura** — **qualquer diff em `internal/transactions` é regressão**. Suíte verde obrigatória.
- **RF-15** Remoção **cirúrgica** do alerta de threshold por limite de cartão em `internal/budgets` (remove `CardThresholdReader`, `ListActiveCardsForThresholdScan`, `ActiveCardForScan`, ramo `buildCardSnapshots`/`activeCards`, constante `ThresholdAlertCardLimit`, testes); **preservar intactos** `ThresholdAlertCategory` e `ThresholdAlertGoal`.
- **RF-16** Onboarding `internal/agents`: (a) coleta `banco`; (b) corrige drift enviando `DueDay: in.DueDay` (não `ClosingDay`) e passa `Bank`; (c) tipos ajustados (`LimitCents` removido, `ClosingDay` reflete derivado, `Bank` adicionado); (d) banco desconhecido segue com fallback 7 **sem atrito**.
- **RF-17** Migration: `cards` ganha `bank` (texto, NOT NULL) e **perde** `limit_cents`; `closing_day` permanece (populado por derivação). Sem backfill (sem cartões em produção).
- **RF-18** OpenAPI atualizado: remove `limit_cents`/`closing_day` como **entrada**, remove `UpdateCardLimitRequest` e rota de limite, adiciona `bank` como entrada, adiciona `closing_day`/`best_purchase_day` como **derivados de resposta**.
- **RF-19** Notificação/evento `card.invoice_due.v1` **deixa de transportar `limit_cents`**; texto e payload sem menção a limite; alerta mantém identificação + vencimento + dias restantes. (Inclui rename `card_name`→`card_nickname`.)
- **RF-20** Normalização determinística de `bank` (trim + lowercase/sem acentos), **única e compartilhada** entre cadastro, endpoint de consulta e onboarding ("Nubank"="nubank"="NuBank").

### Métricas de sucesso do PRD (também são critérios)

- Cadastro (API e onboarding WhatsApp) exige **exatamente 3 dados de negócio** — verificável em OpenAPI + schema de extração do onboarding.
- 100% dos 8 bancos produzem fechamento/melhor-dia corretos; desconhecido → 7 dias — verificável por teste unitário do domain service puro.
- **Zero** referência a `limit_cents`/`closing_day` como **entrada** no OpenAPI.
- `internal/transactions` compila e passa nos testes **sem alteração de contrato**.

---

## 4. Definition of Done (DoD) — confrontar por tarefa (1.0–9.0)

Para **cada** uma das 9 tarefas, abrir o respectivo `task-<n>.0-*.md` e `<n>.0_execution_report.md` e confirmar:

- [ ] Todos os itens de `## Critérios de Sucesso` / `## Critérios de Aceite` comprovados com **evidência física** (não apenas marcado `[x]`).
- [ ] `## Definition of Done (DoD)` 100% marcado **e reproduzível** (não confiar no relatório; re-verificar).
- [ ] Cobertura de requisitos da matriz de `tasks.md` batendo com o código (1.0→RF-10/17; 2.0→RF-02/03/06/08/11/20; 3.0→RF-09/10; 4.0→RF-01/04/05/07/12/13; 5.0→RF-01/05/13/18; 6.0→RF-05/19; 7.0→RF-14/18; 8.0→RF-15; 9.0→RF-16).
- [ ] Riscos de integração de `tasks.md` mitigados: co-entrega migration↔budgets; ordering da rota chi `best-purchase-day` **antes** de `/{id}`; rename `card_name`→`card_nickname` coeso publish↔consume; cache de `closing_day` (ADR-002).

> **Falso positivo em DoD é bloqueante.** Um DoD marcado `[x]` sem evidência reproduzível conta como gap.

---

## 5. Regras de negócio a validar (implementadas — sem exceção)

1. **Derivação determinística:** `closing_day = due_day − dias_antes(banco)`; `best_purchase_day = closing_day + 1`.
2. **Fallback obrigatório:** banco fora da tabela ⇒ `dias_antes = 7` (nunca erro, nunca atrito no onboarding).
3. **Seed exato dos 8 bancos** (RF-10) — valores idênticos, sem divergência.
4. **Exemplo âncora:** Nubank, `due_day=20` ⇒ `closing_day=13`, `best_purchase_day=14` (teste verde obrigatório).
5. **Virada de mês / clamp:** resultado sempre dia válido 1..31, reutilizando `clamp`/`advanceMonth`.
6. **Recompute em UpdateCard:** alterar banco ou `due_day` ⇒ recalcula e re-persiste `closing_day` (versão otimista preservada).
7. **Consulta pura:** best-purchase-day não exige cartão persistido.
8. **Remoção total sem campo morto:** nenhum campo/rota/DTO/coluna/VO fora do novo escopo pode restar (nem comentado, nem depreciado).
9. **Preservação cross-module:** `internal/transactions` intacto (RF-14); `internal/budgets` só perde o ramo card-limit (RF-15), categoria e metas preservados.
10. **Normalização de banco única e compartilhada** (RF-20).

---

## 6. Procedimento de execução (ciclo fechado)

1. **Setup do review:** carregar `AGENTS.md` + governança; detectar linguagem do diff (Go) e carregar gatilhos.
   Determinar escopo do diff contra `origin/main` (branch atual `main` com working tree modificada — revisar o
   conjunto efetivo de alterações da entrega card/melhor-dia). Se o orçamento de diff estourar, **fatiar por módulo**
   (`internal/card`, `internal/budgets`, `internal/agents`, migrations, openapi) — **nunca amostrar e aprovar**.
2. **Review estrito (skill `review`):** produzir achados com severidade canônica antes do veredito; confronto
   incondicional de critérios de aceite (RF-14 da skill). Cada RF/DoD/regra de negócio de §3–§5 é item de checklist.
3. **Validações locais mínimas (evidência):**
   - `go build ./...` e `go vet ./...` verdes.
   - `go test ./internal/card/... ./internal/budgets/... ./internal/agents/... ./internal/transactions/...` verdes
     (transactions **sem diff** de contrato).
   - Gates HARD do repo: zero comentários em `.go` de produção; SQL só em repositório de infra; adapters finos;
     `Validate()` em input DTOs; ordering da rota chi.
   - Teste-âncora Nubank (13/14) e cobertura dos 8 bancos + fallback 7 presentes e verdes.
   - OpenAPI sem `limit_cents`/`closing_day` como entrada; com `bank` entrada e `closing_day`/`best_purchase_day` derivados.
4. **Se houver qualquer achado:** emitir lista no formato canônico `{id, severity, file, line, reproduction, expected, actual}`
   e acionar `@.claude/skills/bugfix/`. Após a correção, **re-review apenas do delta** (`AI_REVIEW_PRIOR_SHA`).
   Repetir até `APPROVED` limpo. Nunca encerrar em `APPROVED_WITH_REMARKS`.
5. **Veredito determinístico:** só `APPROVED` (0 achados, 0 ressalvas) encerra o ciclo local.

---

## 7. Validação em produção (obrigatória para o veredito de "pronto para main")

Antes de declarar pronto e **fazer merge para `main`**, validar o comportamento real em produção via
`ssh root@187.77.45.48` (Docker Swarm; serviços: `postgres`, `pgbouncer`, `server-1`/`server-2`,
`worker-1`/`worker-2`, `otel-lgtm` [Grafana+Prometheus+Loki+Tempo], `caddy`, `migrate`). Fechar **todas** as
lacunas/ressalvas/gaps com evidência coletada de:

- **Banco de dados (Postgres via pgbouncer):** confirmar migration aplicada — `mecontrola.banks` com os 8 seeds
  corretos; `mecontrola.cards` **com** coluna `bank` (NOT NULL) e **sem** `limit_cents`; `closing_day` presente e
  coerente com a derivação para eventuais cartões de teste. Nenhuma referência a `c.limit_cents` em queries vivas.
- **Logs (Loki):** ausência de erros/panics nos serviços `server-*` e `worker-*` relacionados a card/onboarding/
  invoice-due após a entrega; fluxo de cadastro e de `card.invoice_due.v1` sem campo de limite.
- **Tracing (Tempo):** spans de CreateCard/UpdateCard/BestPurchaseDay e da cadeia invoice-due íntegros, sem erro,
  refletindo o novo contrato.
- **Métricas (Prometheus/Grafana):** sem regressão de erro/latência nos endpoints de card; cardinalidade controlada
  (sem `user_id`/`category_id` como label); alertas de budgets de categoria/metas ainda emitindo (card-limit ausente).

> Reforço da memória de projeto: quando a mudança tocar o agente/onboarding, a **validação com LLM real é obrigatória**
> (`RUN_REAL_LLM=1` com credenciais `OPENROUTER_*` do `.env`); mocks não bastam como evidência.
>
> **Segurança operacional:** consultas em produção são **read-only** para diagnóstico. Nenhuma ação destrutiva
> (drop/delete/restart) sem pedido explícito. O `merge para main` só ocorre após APPROVED local **e** produção limpa.

---

## 8. Orquestração por subagentes (disparar quando agregar qualidade)

Paralelizar a revisão por eixo, consolidando os achados no fluxo canônico (sem trabalho sequencial no main loop):

- **`reviewer` / `feature-dev:code-reviewer`** — um por módulo: `internal/card`, `internal/budgets`, `internal/agents`,
  contrato OpenAPI + migrations, não-regressão `internal/transactions`.
- **`Explore`** — varredura de "campo morto": qualquer resíduo de `limit_cents`/`CardLimit`/`UpdateCardLimit`/
  `closing_day` como entrada em todo o repo.
- **`bugfixer`** — correção por causa raiz com teste de regressão obrigatório para cada achado `critical`/`major`.
- **Subagente de produção** — coleta paralela de evidências em DB / Loki / Tempo / Prometheus via SSH.

Enviar múltiplos subagentes independentes numa única leva para execução concorrente; relatar apenas as conclusões
(não despejos de arquivo).

---

## 9. Formato de saída esperado (do agente executor)

1. **Relatório de review** estruturado por rodada: `verdict`, `files_reviewed`, `refs_loaded`, `findings`
   (`{severity, file, line, impact, fix_hint}`), `residual_risks`, `validations_run`.
2. **Tabela de conformidade** RF-01..RF-20 + métricas + DoD (1.0–9.0) + regras de negócio §5, cada linha com
   status e evidência (file:line / comando / query / trace).
3. **Relatório de bugfix** (se houve ciclo): bugs, causa raiz, teste de regressão, resultado.
4. **Evidências de produção**: queries DB, trechos de log/trace/métrica coletados.
5. **Veredito final**: `PRONTO PARA MAIN` **somente** com `APPROVED` local + produção limpa + 0 gaps/lacunas/
   ressalvas/falsos positivos + todas as regras de negócio atendidas. Caso contrário, listar o que falta e reabrir o ciclo.

---

## Anexo — Prompt original × enriquecido

**Original (usuário):** revisar o PRD `simplificacao-card-melhor-dia-compra` com a skill review de forma criteriosa e
sem flexibilização; 100% dos critérios de aceite e DoD; 0 gaps/lacunas/falsos positivos/ressalvas; todas as regras de
negócio; ciclo review→bugfix→review até APPROVED; subagentes especializados; validar em produção via SSH (logs, tracing,
métricas, DB) e fechar tudo antes de merge para main.

**Adições no enriquecimento e por quê:**
- **Enumeração 1:1 dos RF-01..RF-20 + métricas + regras de negócio §5** — evita "atendido" genérico; força evidência por item.
- **DoD por tarefa 1.0–9.0 com re-verificação** — impede falso positivo herdado dos execution reports.
- **Gates HARD do repo explicitados** (zero comentários, adapters finos, pureza do domain service, ordering de rota) —
  ancoram a revisão nas regras reais do `mecontrola`, não em heurística genérica.
- **RF-14 como sentinela de regressão** (qualquer diff em `internal/transactions` reprova) — protege o contrato crítico.
- **Roteiro de produção por sinal** (DB/Loki/Tempo/Prometheus + serviços Swarm reais) + segurança read-only — torna o
  "fechar lacunas em produção" verificável e seguro.
- **Estratégia de subagentes por eixo** — paraleliza sem perder consolidação, conforme preferência de orquestração do projeto.
- **Veredito binário (`APPROVED` apenas)** — remove ambiguidade de `APPROVED_WITH_REMARKS` como estado final.
