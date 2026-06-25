# PROMPT MANDATÓRIO — Execução Completa da Refatoração Canônica do `internal/agent`

> **Uso:** cole o bloco "PROMPT" abaixo como instrução inicial de uma sessão de execução (Claude Code
> / Codex). Ele é **autossuficiente, inegociável e production-ready/proof**. Não resumir, não pular
> seções, não flexibilizar.
> **Fonte da verdade:** `.specs/prd-refatoracao-agent-canonico/` (prd.md, techspec.md, tasks.md,
> task-1.0..9.0, adr-001..008) + runbook `docs/plans/2026_06_25_runbook_jornada_completa_agent_canonico.md`.

---

## Instrução de Uso (procedimento único e mandatório — sem flexibilidade)

Há **um** caminho de execução. Não há modos alternativos, atalhos nem variações.

**Passo 0 — Pré-requisitos (obrigatórios; falha aqui = não inicie):**
- `go install golang.org/x/tools/cmd/deadcode@latest` (hoje só `staticcheck` existe; Tasks 4/5 dependem).
- Exportar `RUN_REAL_LLM=1` + chave OpenRouter no ambiente (Task 6, guard real-LLM por classe).
- Ambiente com acesso ao banco para a verificação fail-fast da migration 000020 (Task 3).
- Repo limpo (`git status` sem pendências) na branch correta.

**Passo 1 — Abrir sessão nova** (contexto limpo) na raiz do repositório.

**Passo 2 — Orquestrar com o skill, ancorado neste prompt:**
```
/execute-all-tasks .specs/prd-refatoracao-agent-canonico
```
e, **na mesma mensagem inicial**, colar **todo o bloco `## PROMPT`** deste arquivo (de "Você vai executar
TODAS as 9 tarefas" até "(fim do PROMPT)"). O bloco é mandatório: reforça DoD, evidências, decisões
fechadas e guardas anti-regressão. **Não** rodar o skill sem o bloco; **não** colar o bloco parcial.

**Passo 3 — Execução governada pelo PROMPT:** o agente executa `1.0→9.0` **estritamente em ordem**,
um subagent fresh por tarefa, aplicando o Protocolo por tarefa (§4) e coletando evidência (§5). Cada
tarefa só vira `done` com DoD 100% + gates `R-*` verdes + Testes da Tarefa rodados.

**Passo 4 — Critério de término (todos obrigatórios):** `tasks.md` com as 9 tarefas `done` **e** o
relatório `docs/runs/2026-06-25-execucao-refatoracao-agent-canonico.md` com evidência por tarefa e a
checklist final de aceite (0 gap / 0 lacuna / 0 falso positivo) **toda verde**. Enquanto qualquer item
estiver pendente, a iniciativa **não** está concluída.

**Proibições (hard):** rodar fora de ordem; paralelizar tarefas; pular os Testes da Tarefa; marcar `done`
sem evidência; flexibilizar qualquer regra do §0/§2/§3 por ferramenta, conveniência ou deadline.
**Qualquer gate vermelho não-corrigível no escopo → parar e escalar.**

---

## PROMPT (copie a partir daqui)

Você vai **executar TODAS as 9 tarefas** de `.specs/prd-refatoracao-agent-canonico/` até o fim, com
foco em **MVP robusto, production-ready/proof de forma inegociável**, atendendo **fielmente** ao
Definition of Done (DoD) e aos critérios de aceite de **cada** tarefa, **com evidências**. Meta dura,
sem exceção: **0 gap, 0 lacuna, 0 falso positivo**.

### 0) Regras inegociáveis (precedência sobre conveniência, ferramenta ou deadline)

1. **Carregue obrigatoriamente** no início e antes de tocar Go: `AGENTS.md`, a skill `mastra` e a skill
   `go-implementation` (Etapas 1–5 + checklist R0–R7). Para tarefas de migration/CI sem Go, carregue só
   o necessário.
2. **Leia `prd.md` e `techspec.md` desta pasta antes de cada tarefa** — toda tarefa traz
   `<critical>` exigindo isso; pular **invalida** a tarefa.
3. **Respeite a Errata de Verificação (2026-06-25)** no fim do `techspec.md` e em `prd.md` (RF-39/40/41).
   Ela corrige falsos positivos do spec original. **Não reintroduza** os erros listados em §"Guardas
   anti-regressão" abaixo.
4. **Gates hard obrigatórios** em toda alteração Go: `R-ADAPTER-001` (adapters finos + **zero
   comentários**), `R-AGENT-WF-001` (Workflow→Tool→binding→usecase; sem novo `case intent.Kind` no
   switch; tipos fechados; LLM só no parse), `R-WF-KERNEL-001` (kernel genérico, sem domínio, merge-patch
   no resume), `R-TESTING-001` (testify/suite whitebox + `fake.NewProvider()`), `R-DTO-VALIDATE-001`,
   `R-TXN-WORKFLOWS-001` (quando tocar transactions). Checklist R0–R7 por tarefa.
5. **DMMF state-as-type:** estados/outcomes/intenções/slots como tipos fechados; proibidos `Result[T,E]`
   customizado, currying, DSL de pipeline, monads, `init()`, `panic` em produção, abstração de tempo
   (use `time.Now().UTC()` inline), `var _ Interface = (*T)(nil)`.
6. **Fronteira de dados (hard):** `internal/agent` acessa **apenas** tabelas próprias; consumo de outro
   BC só por porta de entrada (usecase/handler/producer/consumer/job). **Proibido** SQL direto a outro
   BC e import de repo/infra de outro contexto.
7. **Cardinalidade de métrica:** labels só de enums fechados; **proibido** `user_id`/`category_id`/
   `correlation_key`/`message_id`.
8. **Sem ação destrutiva de git ou publicação remota** sem pedido explícito. Não commitar/abrir PR a
   menos que solicitado; se for commitar, branch a partir de `main`.
9. **Pare e escale** (não invente, não flexibilize) se: faltar input obrigatório sem inferência segura;
   um gate ficar vermelho sem correção possível dentro do escopo; ou uma pré-condição não for satisfeita.

### 1) Ordem de execução (DAG linear — sem paralelismo, decisão consciente por robustez)

Execute estritamente nesta ordem; cada tarefa depende da anterior estar `done` com evidência:

`1.0 → 2.0 → 3.0 → 4.0 → 5.0 → 6.0 → 7.0 → 8.0 → 9.0`

- **1.0** Gate de fronteira de dados + gates de governança (verde no estado atual; vermelho no PR de teste negativo).
- **2.0** Eliminação Telegram (código + config + env + VO de canal).
- **3.0** Migration `000020_drop_telegram_channel` (schema) + **verificação pré-deploy fail-fast** (`count(*) telegram` = 0).
- **4.0** Kernel caminho único: remover `kernelEnabled`/`EnableKernel`/`parity_test`/`TransactionsWriteEnabled` (kernel sempre-on; dep ausente = falha de boot). **Pré-1..4 verdes antes de remover.**
- **5.0** Limpeza de eventos órfãos cross-module (lista corrigida — ver §Guardas).
- **6.0** Structured Output `Strict=true` + roteamento por classe (`ClassRouter`/`LLMClass`) + onboarding tool-calling→json_schema + **migração do `ConfigureBudgetConversation` para parse estruturado** + guard `RUN_REAL_LLM`.
- **7.0** Editar/apagar por referência + desambiguação (search no transactions + steps `resolve_candidates`/`select_target` + tipos `AwaitingSelect`/`TargetCandidate`/`OperationDeleteByRef`/`OperationEditByRef` + kinds by-ref).
- **8.0** Plano multi-tool 1..N + idempotência por passo (migration `000021_agent_decisions_step_index`).
- **9.0** Operação diária via portas (recorrência de orçamento `budgets.CreateRecurrence`, `EditCategoryPercentage`, consultas, casos especiais, tom/UX).

### 2) Decisões fechadas (2026-06-25) — aplique sem reabrir

- `external.expense.v1` → **remover o pipeline inteiro** (consumer + `IngestExternalExpense` + command + strategy + registro `budgets/module.go:153` + testes). Entra na Task 5.
- RF-40 → **instalar `deadcode`** (`go install golang.org/x/tools/cmd/deadcode@latest`) e rodar `deadcode ./...` na Task 4; se nada apontar no fluxo de orçamento, RF-40 fica satisfeito com evidência. **Não** remover branch sem evidência.
- `ConfigureBudgetConversation` (RF-10) → **migrar para parse estruturado** (Structured Output `Strict=true` + execução determinística); **não** sancionar nova exceção de LLM. Entra na Task 6/9.
- Modelo de onboarding sob Strict=true → **guard real-LLM decide** (`RUN_REAL_LLM`): mantém `claude-haiku-4.5` se passar com json_schema estrito, troca por modelo elegível se quebrar. Entra na Task 6.

### 3) Guardas anti-regressão (NÃO reintroduzir falsos positivos do spec original)

- **MANTER** `transactions.card_purchase.deleted` — TEM consumer `recomputeConsumer` (`transactions/module.go:321`). Removê-lo quebra o recompute do resumo mensal. **Nunca** apagar.
- **MANTER** `onboarding.splits_calculated`, `onboarding.card_registered`, `onboarding.completed` (têm consumer).
- **NÃO** procurar `continuePendingExpenseConfirmationLegacy`/`PendingExpenseConfirmationGateway` — não existem (já removidos).
- **NÃO** "corrigir" o `NewAmount` do `NewLastTransactionEditorExecutor` — já correto (`hitl_adapters.go:89` + `daily_ledger_agent.go:622`). O risco real é optimistic-lock: mapear `ErrTransactionVersionConflict` para mensagem amigável.
- Cartões **não publicam eventos** (`CreateCard`/`UpdateCard`/`SoftDeleteCard` gravam `cards` via UoW direto). Não invente `card.*` events.
- Confirmação de remoção de órfão **por constante de event-type** (não por nome de arquivo). Órfãos a remover: `agent.intent.{rejected,executed}`, `budgets.budget_activated` (remover só o `Publish` em `activate_budget`/`edit_category_percentage:123`, mantendo a escrita), `transactions.recurring_template.{created,updated,deleted}`, `onboarding.income_registered` (`save_onboarding_income:79`), `external.expense.v1` (pipeline).

### 4) Protocolo por tarefa (repita para 1.0..9.0)

Para **cada** tarefa, execute o ciclo completo e só avance quando `done` com evidência:

1. **Carregar contexto:** ler `prd.md`, `techspec.md`, o `task-N.md` correspondente e os ADRs citados nele. Declarar as `## Skills Necessárias` da tarefa + skills de linguagem inferidas do diff.
2. **Modelar antes de codar:** respeitar fronteiras arquiteturais; nada de wiring/router/job/consumer inventado.
3. **Implementar:** adaptar exemplos das skills ao contexto real (nunca copiar literal). Em refatorações amplas, paralelizar por categoria via subagents (usecase/repo/handler/job/outbox) — trabalho sequencial em main loop é proibido para escopo amplo.
4. **Criar e EXECUTAR os Testes da Tarefa** (seção `## Testes da Tarefa`) — unit (testify/suite whitebox) + integração (`testcontainers-go` `//go:build integration`) + E2E Godog quando a tarefa exigir. `<critical>` da tarefa: **nunca** marcar `done` sem criar e rodar os testes.
5. **Validar proporcional ao risco:** `task build`, `task test`, `task lint`/`golangci-lint`, `task test:integration` quando tocar IO/migration, `deadcode ./...` (Tasks 4/5), guard `RUN_REAL_LLM` (Task 6). Rodar **todos** os gates `R-*` + zero-comentário + cardinalidade (ver §6 do runbook para os greps).
6. **DoD da tarefa = atender 100% de:** `<requirements>` (RFs) + `## Critérios de Sucesso` + `## Testes da Tarefa` verdes + gates `R-*` verdes + checklist R0–R7 + evidência coletada. Critério de aceite = comportamento observável dos fluxos do Documento Oficial sem regressão.
7. **Coletar evidência** (ver §5) e **atualizar `tasks.md`** (status da tarefa `pending`→`done`).
8. **Halt-first:** se algo ficar vermelho e não for corrigível no escopo, **parar**, registrar e escalar — não seguir para a próxima tarefa.

### 5) Evidência obrigatória (sem evidência = não-feito)

Para cada tarefa, registre no relatório de execução (ver §6):
- Arquivos criados/alterados/removidos (lista).
- Comandos de validação executados **e sua saída relevante** (build/test/lint/integration/deadcode/guard).
- Saída dos gates `R-*` relevantes (devem retornar vazio/OK).
- Mapeamento RF → como foi atendido (1 linha por RF da tarefa).
- Riscos residuais e suposições assumidas (explícitos).
- Para Task 3: saída da verificação fail-fast (`count(*) telegram`). Para Task 8: prova de idempotência por `step_index`. Para Task 6: resultado do guard `RUN_REAL_LLM` por classe.

Regra: **não aprovar solução com lacuna crítica conhecida**; **não** produzir falso positivo (afirmar feito o que não foi).

### 6) Entregável final

- Atualizar `tasks.md` com todas as 9 tarefas `done`.
- Escrever **um relatório de execução** em `docs/runs/2026-06-25-execucao-refatoracao-agent-canonico.md`
  com: sumário, status por tarefa, evidências (§5) e as **três checklists do §7** (7.1 por tarefa, 7.2
  aceite final, 7.3 robustez) com **cada caixa marcada e o comando/saída que a comprova**. 100% PASS é
  condição de conclusão; qualquer `FAIL`/não-verificado bloqueia a entrega.
- Não declarar a iniciativa concluída enquanto **qualquer** critério acima estiver pendente.

### 7) DoD + Critérios de Aceite — 100% binário (gate de conclusão, robustez inegociável)

DoD e critérios de aceite são **binários**: cada item é `PASS` (com evidência) ou `FAIL`. **Não existe
parcial.** A tarefa só é `done`, e a iniciativa só é concluída, com **100% PASS**. Qualquer `FAIL` ou
"não verificado" = **não-feito** → parar e escalar.

**7.1 Gate de aceite POR TAREFA (todos PASS para marcar `done`):**
- [ ] Todos os RFs do `<requirements>` da tarefa atendidos (1 linha de evidência por RF).
- [ ] Todos os itens de `## Critérios de Sucesso` da tarefa comprovados.
- [ ] `## Testes da Tarefa` **criados e executados**, verdes (unit testify/suite whitebox + integração + E2E quando exigido).
- [ ] Gates `R-*` aplicáveis retornando vazio/OK + checklist R0–R7 completo.
- [ ] `task build` + `task test` (+ `task test:integration`/`lint`/`deadcode`/guard `RUN_REAL_LLM` quando aplicável) verdes, com saída anexada.
- [ ] Zero comentário em Go de produção; cardinalidade de métrica controlada.
- [ ] `tasks.md` atualizado para `done`; evidência (§5) registrada no relatório.

**7.2 Matriz de aceite FINAL da iniciativa (métricas do PRD — 100% PASS, com prova):**
- [ ] Referências a Telegram em código/config/env/schema = **0** (grep verde em `internal/`, `configs/`, `cmd/`, `.env.example`, `migrations/` exceto 000020).
- [ ] Acessos do `internal/agent` a tabela de outro BC (SQL direto ou import de repo/infra) = **0** (gate de CI `agent-data-boundary.sh` verde).
- [ ] % de ações de domínio originadas de Structured Output validado contra schema = **100%** (inclui `ConfigureBudgetConversation` migrado; nenhum LLM em execução fora do parse + exceções sancionadas `KindUnknown`/onboarding).
- [ ] Operações destrutivas/sensíveis efetivadas sem confirmação humana explícita = **0** (HITL durável nas 4 operações + by-ref).
- [ ] Caminhos *legacy* coexistindo com o kernel = **0** (`kernelEnabled`/`EnableKernel`/`parity_test`/`TransactionsWriteEnabled` removidos; kernel sempre-on).
- [ ] Eventos producer-sem-consumer e consumer-sem-producer = **0** por **constante de event-type** (órfãos removidos; `card_purchase.deleted`/`splits_calculated`/`card_registered`/`onboarding.completed` mantidos).
- [ ] Cobertura dos fluxos do Documento Oficial por cenário de aceite = **100%** (onboarding 8 etapas; operação diária; edição/exclusão por referência + desambiguação; recorrência; % categoria; casos especiais).
- [ ] Diferença de comportamento observável nos fluxos válidos antes vs. depois = **0** (não regressão de UX — strings/fluxos batem com o runbook).
- [ ] Cardinalidade de métrica: **sem** `user_id`/`category_id`/`correlation_key`/`message_id` como label (grep verde).
- [ ] Suíte completa verde (unit + integração + E2E) e todos os gates `R-*` verdes no CI.

**7.3 Robustez (production-ready/proof — todos PASS):**
- [ ] Durabilidade/idempotência: HITL e pending steps via suspend/resume do kernel; idempotência por `event_id`/`wamid` e por `step_index` (plano multi-tool); replay não duplica mutação.
- [ ] Sem `init()`, sem `panic` em produção, `context.Context` em toda fronteira de IO, `errors.Join`/`%w`.
- [ ] Goroutines canceláveis, shutdown cooperativo, sem leak.
- [ ] Optimistic-lock (`version`) tratado em edit/delete (incl. by-ref): `ErrTransactionVersionConflict` → mensagem amigável.
- [ ] Migrations 000020/000021 com `up`/`down` reversíveis; 000020 com verificação fail-fast pré-deploy.
- [ ] Tipos fechados (state-as-type) em todos os estados/outcomes/operações novos.

O relatório final (§6) DEVE conter as três checklists (7.1 por tarefa, 7.2 e 7.3 finais) com **cada
caixa marcada e o comando/saída que a comprova**. Caixa sem evidência = `FAIL`.

## (fim do PROMPT)

---

## Notas de operação (fora do prompt)

- Skill recomendada para orquestrar: `execute-all-tasks` (spawna subagent fresh por tarefa, respeita DAG,
  halt-first, retomada idempotente). Alternativa por tarefa: `execute-task`.
- Pré-requisito de ambiente: instalar `deadcode` (só `staticcheck` está presente hoje).
- Conferir o runbook `docs/plans/2026_06_25_runbook_jornada_completa_agent_canonico.md` para o mapa
  Task→código, strings de UX verbatim, DDL de tabelas e gates de verificação (§12).
