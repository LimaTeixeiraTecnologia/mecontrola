# História de Usuário — Orçamento Retroativo Conversacional e Mês por Extenso

- Iniciativa: dar ao agente financeiro WhatsApp a capacidade real de criar orçamentos por conversa (inclusive retroativos), entender e citar meses por extenso, e mostrar a retrospectiva planejado vs realizado.
- Origem: investigação de produção do usuário `f56e1142-0960-4dd9-aa09-955aa519fee1` (+5511986896322) em 2026-07-08.
- Destino de backlog: refinamento técnico (create-technical-specification) do módulo `internal/agents` sobre o substrato `internal/platform`.
- Persona primária: usuário final do MeControla no WhatsApp (pessoa física que controla finanças pessoais).
- Skills aplicadas de forma mandatória nesta modelagem: `domain-modeling-production`, `mastra`, `go-implementation`, `design-patterns-mandatory`.

---

## A. Objetivo de Negócio

O usuário ativo tentou, por conversa natural no WhatsApp, entender o mês anterior e criar um orçamento retroativo para junho/2026. A conversa falhou duas vezes com mensagem genérica, o orçamento nunca foi persistido e o agente referiu o mês como `2026-06` em vez de "junho de 2026". Esta história entrega uma capacidade conversacional real de criação de orçamento (com distribuição por categoria e retroativa sem limite), resolução determinística de mês relativo/nomeado, exibição de mês por extenso e retrospectiva planejado vs realizado — tudo como uma única fatia de valor coerente.

## B. Evidência de Produção (Proof)

Investigação executada por SSH em `root@187.77.45.48`, Postgres `mecontrola_db` (container `mecontrola_postgres`), thread `74d83407-b758-465c-9d16-975eab3a75d1` (resource_id = `f56e1142-0960-4dd9-aa09-955aa519fee1`, thread_id = `+5511986896322`). Logs Loki e traces Tempo estavam sem retenção no momento da coleta (serviços `mecontrola_server-*` reiniciaram cerca de 30 minutos antes; stdout dos containers com zero linhas), portanto a evidência durável utilizada é o banco de dados.

Diálogo capturado (`platform_messages`, horários em UTC; -03:00 no relógio do usuário):

| Hora UTC | Papel | Conteúdo (verbatim) |
| --- | --- | --- |
| 18:06:33 | user | "Como eu fui mes passado?" |
| 18:06:33 | assistant | "Não há lançamentos em setembro de 2023. [...]" |
| 18:06:49 | user | "Como foi meu mes de junho de 2026?" |
| 18:06:49 | assistant | "Você ainda não tem um orçamento para \*2026-06\*. Posso te ajudar a criar um?" |
| 18:07:23 | user | "sim" |
| 18:07:23 | assistant | "Para criar seu orçamento, preciso saber o valor total [...] para o mês de junho de 2026. Qual valor você tem em mente? 💰" |
| 18:09:49 | user | "R$ 13.874,40" |
| 18:09:49 | assistant | "Vamos criar seu orçamento para junho de 2026! Confirma? R$ 13.874,40 para o mês de junho de 2026? ✅" |
| 18:09:56 | user | "Sim" |
| 18:09:56 | assistant | "Não consegui criar o orçamento. Tente novamente em breve." |

Execuções (`platform_runs`, mesmo thread): dois runs `failed / usecaseError` às 18:06:30 (3461ms) e 18:09:55 (1887ms); os demais `succeeded / routed`. A coluna `error` dos runs falhos veio vazia.

Estado de orçamentos (`budgets`): não existe competência `2026-06` para o usuário. Existem apenas `2026-07` (state=2 ACTIVE, criado no onboarding às 18:48) e 12 drafts `2026-08`→`2027-07` (state=1, auto_draft, às 18:49). Allocations ativas de 2026-07: custo_fixo 4500, liberdade_financeira 2000, metas 2000, prazeres 1000, conhecimento 500 (soma 10000). Conclusão factual: o orçamento retroativo de junho/2026 nunca foi criado. A data corrente é 2026-07-08, logo "mês passado" = junho/2026 (`2026-06`); o agente respondeu "setembro de 2023", mês incorreto.

Métricas (Prometheus na stack `otel-lgtm` do host, job `mecontrola-worker`): `agent_runs_total{status="failed"}` = 4 e `{status="succeeded"}` = 10 (dois dos runs falhos são os desta thread). `agent_tool_invocations_total` por tool registra `query_month` = 2, `query_plan` = 1, `register_expense` = 2, `register_income` = 2, `classify_category` = 1 e `adjust_allocation` = 1; não existe a série para `create_budget` (o tool inexiste). Traces (Tempo): a trace `3f4c0b8c3b020502a576590ec389ab24` (root `agents.consumer.whatsapp_inbound.handle`, início 18:09:55 UTC — o run falho da confirmação) contém, após `llm.complete`, os spans `budgets.usecase.edit_category_percentage` e `agents.binding.budget_planner.edit_category_percentage`, que são exatamente o caminho do tool `adjust_allocation`. Ou seja, na confirmação "Sim", o modelo chamou `adjust_allocation` (ajuste de categoria de orçamento existente) sobre a competência 2026-06 inexistente; o edit falhou e o run fechou como `usecaseError`. Isto está confirmado por trace, não apenas inferido. O use case `EditCategoryPercentage` (`internal/budgets/application/usecases/edit_category_percentage.go`) possui ramo explícito de orçamento não encontrado, coberto por teste (`edit_category_percentage_test.go:121` `TestExecute_BudgetNotFound`), que é exatamente a falha esperada para 2026-06. Rastreabilidade de versão: `HEAD` do repositório é `a8fa8ee` e a imagem em produção reporta `service_version=a8fa8eef`; a base de código citada é a mesma que rodou no incidente (sem drift). Observabilidade textual: os logs Loki do worker no período registram apenas `outbox: dispatcher processed batch` (nível info); os spans não carregam `status=error` e `platform_runs.error` veio vazio, portanto a string exata da mensagem de erro não ficou persistida em nenhuma camada — lacuna de observabilidade real, registrada como achado.

## C. Root Causes Confirmados na Base de Código

- RC-1 — Não existe capacidade conversacional de criar orçamento. Os tools financeiros registrados em `internal/agents/module.go` (retorno `[]tool.ToolHandle`) somam 23 e incluem `suggest_allocation` (read-only) e `adjust_allocation` (ajuste de orçamento existente), mas não há `create_budget` nem `activate_budget`. Busca por `CreateBudget`/`ActivateBudget` em `internal/agents/application/tools/` retorna vazio. `CreateBudgetUC` (porta `internal/agents/application/interfaces/budget_planner.go`, adapter `internal/agents/infrastructure/binding/budget_planner_adapter.go`) só é consumido pelo `internal/agents/application/workflows/onboarding_workflow.go` (chamadas `budgets.CreateBudget` ~768/779; ativação ~830). A instrução do agente (arquivo `mecontrola_agent.go`, em `internal/agents`, camada application, pacote agents, linha 213) manda oferecer a criação; o LLM improvisa o diálogo (as frases "Vamos criar seu orçamento...", "preciso saber o valor total..." e "Não consegui criar o orçamento..." não existem no código-fonte). Na confirmação não há tool para persistir: o modelo recorre a `adjust_allocation` (tool `internal/agents/application/tools/adjust_allocation.go`, que chama `planner.EditCategoryPercentage(ctx, userID, competence, rootSlug, percentage)` sobre um orçamento existente), mas 2026-06 não existe, então o exec retorna erro; o run fecha como falha em `internal/platform/agent/runtime.go` (~184-194; `internal/platform/agent/agent.go:142` marca `ToolOutcomeUsecaseError`) e o consumer entrega o fallback genérico `fallbackReply` em `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (~263). Métricas e trace confirmam: ausência da série `create_budget`, exatamente uma invocação de `adjust_allocation` e o span `budgets.usecase.edit_category_percentage` presente no run falho de 18:09:55.
- RC-2 — Resolução de mês relativo/nomeado delegada ao LLM, sem determinismo. Não há utilitário que resolva "mês passado", "junho de 2026" ou "janeiro de 2025" para `YYYY-MM`; a instrução `mecontrola_agent.go:187` só pede que o modelo converta. Efeito real: "mês passado" virou "setembro de 2023". Os tools `internal/agents/application/tools/query_month.go` e `query_plan.go` só aplicam fallback para o mês corrente quando o parâmetro chega vazio; não corrigem mês incorreto injetado pelo modelo.
- RC-3 — Competência exibida em ISO (`2026-06`), não por extenso. `internal/budgets/domain/valueobjects/competence.go` expõe `String()` retornando `YYYY-MM`. A instrução tem MAPA slug→nome só para categorias (`mecontrola_agent.go:189-194`), nada para meses; a mensagem de orçamento não encontrado (linha 213) injeta `{competência}` cru. O usuário pede explicitamente meses por extenso.
- Contexto de domínio verificado: `NewCompetence` valida apenas formato `YYYY-MM` (sem invariante de mês passado); a constraint `budgets_competence_chk` confirma o formato; `Budget.Activate` (`internal/budgets/domain/entities/budget.go`) exige `total_cents > 0` e soma de allocations = 10000 bps; estados fechados `BudgetStateDraft` (1) e `BudgetStateActive` (2); unicidade `(user_id, competence)`.

## D. Decisões de Produto (confirmadas pelo usuário)

- D1 — Distribuição: diálogo completo por categoria (como no onboarding) antes de criar; o sistema não reaproveita perfil automaticamente.
- D2 — Alcance retroativo: qualquer mês passado, sem limite inferior.
- D3 — Propósito: habilitar retrospectiva planejado vs realizado ("como foi meu mês de junho"), integrando allocations ativas com os lançamentos reais do mês.

## E. Modelagem das Skills Mandatórias

- domain-modeling-production: linguagem ubíqua (Competência `YYYY-MM`, Mês por Extenso, Orçamento, Distribuição, Orçamento Retroativo, Retrospectiva). Comando novo `CriarOrçamentoConversacional(userID, competência, totalCents, allocations[])` materializado sobre `CreateBudgetCommand` + `Budget.Activate`. Invariantes preservadas (total > 0; soma allocations = 10000 bps; unicidade; formato de competência). Política de tempo nova: competência retroativa permitida sem limite (D2). Estado de espera do diálogo modelado como tipo fechado (aguardando total / distribuição / confirmação), nunca string livre. Fronteiras: cálculo/validação e formatação de competência no módulo `internal/budgets`; orquestração conversacional em `internal/agents` sobre `internal/platform`; kernel `internal/platform/workflow` permanece genérico. Anti-padrões proibidos mantidos (sem Result/Either customizado, currying, DSL de pipeline).
- mastra: Workflow durável com suspend/resume para a criação multi-turno com HITL (espelhando `BuildOnboardingWorkflow`/`BuildDestructiveConfirmWorkflow`); persistência via Tool fina `create_budget` (adapter sobre `CreateBudgetUC`/`ActivateBudget`) resolvida por registry (sem `switch case intent.Kind`). Fluxo canônico Thread→Run respeitado. Estado de espera fechado salvo no `Snapshot` antes da pergunta; resume por JSON merge-patch antes do parse; limpeza determinística (nenhum run permanece suspenso). Escrita idempotente via `agent.InboundIdentityFromContext` e `agent.WithWriteToolSet`. LLM só nas call-sites sancionadas; OpenRouter único provider.
- go-implementation: zero comentários em `.go` de produção; `Decide*` puro e determinístico para resolução de mês (recebe `now time.Time`, sem relógio interno); DTO da tool com `Validate() error` via `errors.Join` nomeando campos; estados como tipos fechados; testes testify/suite whitebox com `fake.NewProvider()`, dependencies struct com IIFE por mock e SUT em `s.Run`; mocks via `.mockery.yml` + `task mocks`; métricas com cardinalidade controlada (sem `user_id`/`competence` como label); validação real-LLM obrigatória (RUN_REAL_LLM=1).
- design-patterns-mandatory: aplicar Registry/Command já existentes do substrato para expor `create_budget` e o Workflow (evita novo switch de intenção); aplicar State-as-type para o estado de espera do diálogo; não introduzir padrão estrutural novo (a Tool fina já é o adapter; evitar over-engineering); não usar Strategy para formatação de mês (função pura de mapeamento competência→extenso é suficiente e mais econômica).

---

## Declaração
Como usuário do MeControla no WhatsApp, quero criar orçamentos conversando — inclusive retroativos de qualquer mês, informando a distribuição por categoria — e que o assistente entenda e cite os meses por extenso e me mostre o comparativo entre planejado e realizado, para organizar e entender qualquer mês sem receber mensagens de erro genéricas.

## Contexto
- Problema: o agente oferece criar orçamento ("Posso te ajudar a criar um?"), conduz o diálogo, mas não há tool/fluxo para persistir; a confirmação final resulta em run `failed/usecaseError` e fallback genérico. Além disso, "mês passado" foi resolvido para "setembro de 2023" e a competência foi exibida como `2026-06` em vez de "junho de 2026".
- Resultado esperado: ao confirmar, o orçamento (retroativo ou não) é criado e ativado; meses relativos/nomeados são resolvidos corretamente; toda referência de mês ao usuário é por extenso; e há retrospectiva planejado vs realizado do mês.
- Fonte: investigação de produção (seções B e C) e decisões D1–D3.

## Regras de Negócio
- Criação exige total maior que zero e distribuição por categoria somando exatamente 100% (10000 bps) para ativar; a distribuição é coletada por diálogo completo por categoria antes de criar (D1); o sistema não reaproveita perfil automaticamente.
- Confirmação humana explícita é obrigatória antes de persistir; o estado de espera (tipo fechado) é salvo antes de exibir a pergunta e retomado antes do parse; ao efetivar, cancelar ou expirar, o estado é limpo e o run é encerrado.
- Competência retroativa é permitida sem limite inferior (D2); competência futura permanece permitida como hoje; a única validação de tempo é o formato `YYYY-MM`.
- Unicidade `(user_id, competence)`: se já houver orçamento para a competência, o fluxo informa e não duplica.
- Resolução de mês é determinística e pura: "mês atual" = competência corrente em America/Sao_Paulo; "mês passado" = corrente menos um; meses nomeados com ano resolvem para `YYYY-MM` exato; entrada ambígua pede esclarecimento em vez de assumir.
- Toda saída ao usuário que cite competência usa mês por extenso (ex.: "junho de 2026"); `YYYY-MM` permanece apenas como contrato interno.
- Retrospectiva usa as allocations ativas como planejado e os lançamentos reais do mês como realizado, apresentando por categoria e total valor planejado, realizado e percentual de execução; sem orçamento para a competência, o sistema oferece criar.
- Uma capacidade só é oferecida ao usuário quando existe tool/fluxo que a execute; falha de execução usa mensagem específica de indisponibilidade, distinta do fallback de "não entendi"; cada run falho permanece auditável com status/outcome fechados.
- A tool `create_budget` é adapter fino: sem regra de negócio, SQL ou branching de domínio; delega ao use case de criação/ativação.

## Critérios de Aceite
```gherkin
Cenário: Criação bem-sucedida com distribuição completa
  Dado que sou um usuário ativo sem orçamento para a competência alvo
  Quando informo o total e a distribuição por categoria e confirmo com "sim"
  Então o orçamento é criado e ativado com allocations somando 100%
  E recebo uma confirmação de sucesso citando o mês por extenso

Cenário: Distribuição incompleta bloqueia a ativação
  Dado que informei um total válido
  Quando a soma das categorias informadas é diferente de 100%
  Então o agente pede o ajuste da distribuição
  E o orçamento não é ativado enquanto a soma não fechar 100%

Cenário: Orçamento retroativo aceito
  Dado que a competência corrente é 2026-07
  Quando peço para criar orçamento para junho de 2026 e concluo o diálogo de distribuição
  Então o orçamento para 2026-06 é criado e ativado

Cenário: Mês passado muito antigo continua permitido
  Dado que informo uma competência passada distante, por exemplo janeiro de 2025
  Quando concluo o diálogo de criação
  Então o orçamento para 2025-01 é criado sem bloqueio por antiguidade

Cenário: Mês passado resolve corretamente
  Dado que a data corrente é 8 de julho de 2026 em America/Sao_Paulo
  Quando peço algo sobre "mês passado"
  Então o período resolvido é 2026-06 e é citado como "junho de 2026"

Cenário: Expressão sem mês reconhecível pede esclarecimento
  Dado que minha mensagem não contém mês nem referência relativa reconhecível
  Quando o sistema tenta resolver o período
  Então o agente pede que eu informe o mês em vez de assumir um período

Cenário: Retrospectiva planejado vs realizado
  Dado que criei orçamento para junho de 2026 e há lançamentos nesse mês
  Quando pergunto "como foi meu mês de junho de 2026?"
  Então recebo o comparativo planejado vs realizado por categoria e total, com o mês por extenso

Cenário: Competência já existente não duplica
  Dado que já existe orçamento para a competência informada
  Quando tento criar novamente para o mesmo mês
  Então o sistema informa que já existe e não cria duplicado

Cenário: Confirmação negada não persiste
  Dado que o agente apresentou o resumo do orçamento para confirmação
  Quando respondo "não" ou "cancela"
  Então nenhum orçamento é criado e o estado de espera é limpo, encerrando o run sem efeito

Cenário: Falha ao persistir devolve mensagem específica e auditável
  Dado que confirmei a criação
  Quando o use case de criação retorna erro
  Então recebo uma mensagem específica de indisponibilidade temporária, distinta do fallback de "não entendi"
  E o run é registrado como falho e auditável
```

## Dados e Permissões
- Dados obrigatórios: identificador do usuário (resourceId), texto do usuário, data de referência (America/Sao_Paulo), competência resolvida em `YYYY-MM`, total em centavos, lista de allocations (raiz + basis points), allocations ativas e lançamentos do mês (para retrospectiva).
- Perfis/permissões: usuário autenticado como principal do WhatsApp inbound; leitura e escrita apenas dos próprios dados; escrita idempotente via identidade do inbound.

## Dependências
- Use cases de criação e ativação de orçamento existentes (`internal/agents/application/interfaces/budget_planner.go`, `internal/agents/infrastructure/binding/budget_planner_adapter.go`, `internal/agents/application/workflows/onboarding_workflow.go`).
- Substrato de workflow durável com suspend/resume, merge-patch no resume e Run auditável (`internal/platform/workflow`, `internal/platform/agent`).
- Tools de leitura existentes `internal/agents/application/tools/query_plan.go` (planejado) e `query_month.go` (realizado) para a retrospectiva.
- Constraints `budgets_user_comp_uk` e `budgets_competence_chk` (migração inicial do schema).

## Fora de Escopo
- Edição e exclusão de orçamento por conversa.
- Reaproveitamento automático de distribuição entre meses.
- Materialização retroativa de lançamentos/recorrências e recomputo histórico de alertas.
- Interpretação de intervalos de datas ou trimestres, e internacionalização além de português do Brasil.
- Mudança do formato de armazenamento da competência (`YYYY-MM` permanece).
- Redesenho geral das mensagens de erro do agente fora do contexto de orçamento.

## Evidências
- Entrada: diálogo de produção com falha na confirmação final; resolução incorreta de "mês passado"; pedido explícito do usuário de criar orçamento por conversa e de citar meses por extenso ("junho de 2026, janeiro de 2025").
- Base de código: ausência de `create_budget` em `internal/agents/module.go` (retorno `[]tool.ToolHandle`) e em `internal/agents/application/tools/`; `CreateBudgetUC` consumido só por `internal/agents/application/workflows/onboarding_workflow.go` (~768/779/830); fallback `fallbackReply` em `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (~263); decisão de outcome em `internal/platform/agent/runtime.go` (~184-194) e `internal/platform/agent/agent.go:142`; instrução do agente `mecontrola_agent.go` (em `internal/agents`, camada application, pacote agents; linhas 187, 189-194, 213); `Competence.String()` ISO em `internal/budgets/domain/valueobjects/competence.go`; `Budget.Activate` (soma 10000 bps) em `internal/budgets/domain/entities/budget.go`; `query_month.go`/`query_plan.go` com fallback apenas para mês corrente.
- Métricas/traces/observabilidade: `agent_runs_total{status="failed"}`=4 e `succeeded`=10; `agent_tool_invocations_total` sem série `create_budget` e com exatamente 1 `adjust_allocation`; a trace `3f4c0b8c3b020502a576590ec389ab24` (18:09:55 UTC) mostra `budgets.usecase.edit_category_percentage` (caminho do `adjust_allocation`) no run falho — caminho confirmado por trace; logs Loki do worker no período só com `outbox: dispatcher processed batch` (info) e `platform_runs.error` vazio — a string exata do erro não ficou persistida em log/trace/db (lacuna de observabilidade).
- Inferências: o passo de persistência deve ser um Workflow com HITL espelhando `BuildDestructiveConfirmWorkflow`; a resolução de mês deve ser uma função `Decide*` pura; o comparativo planejado vs realizado combina os retornos de `query_plan` e `query_month`, sem nova fonte de verdade; adicionar mapeamento competência→extenso resolve a inconsistência sem alterar o dado persistido.
- Não evidenciado: não há, na base de código, caminho de criação de orçamento fora do onboarding, utilitário de resolução de mês relativo/nomeado, formatação de mês por extenso no domínio de orçamento, validação que rejeite competência passada, nem distinção no fallback entre "não entendi" e "falhei ao executar" — buscas executadas em `internal/agents` e `internal/budgets` sem correspondência.

## Notas de Validação
- Cobre fluxo feliz (criação com distribuição, retroativo, retrospectiva), fluxos alternativos (distribuição incompleta, mês nomeado antigo, mês passado resolvido) e fluxos de erro/bloqueio (esclarecimento, competência existente, confirmação negada, falha de persistência).
- Confronto com a base de código executado; toda afirmação técnica cita caminho/linha ou descreve a busca sem correspondência; evidências separam entrada, base de código e inferência; ausência de achado não é tratada como prova sem busca descrita.
- Riscos e mitigações: regressão de roteamento do agente (validação real-LLM com RUN_REAL_LLM=1 cobrindo criação, retroativo e retrospectiva); divergência de fuso na resolução de mês (testes de tabela em viradas de ano em America/Sao_Paulo); estado de espera órfão no HITL (limpeza determinística e housekeeping do kernel, nenhum run permanece suspenso).
- Este documento é o input de refinamento; a implementação ocorre em etapa posterior via especificação técnica e tarefas, sem alteração de código de produção nesta etapa.
