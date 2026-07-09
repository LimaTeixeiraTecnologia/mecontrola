# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 2 -->

Orçamento Retroativo Conversacional e Mês por Extenso

- Slug: `orcamento-retroativo-conversacional-e-mes-por-extenso`
- Origem: `docs/us/2026-07-08-us-orcamento-retroativo-conversacional-e-mes-por-extenso.md`
- Evidência de produção: usuário `f56e1142-0960-4dd9-aa09-955aa519fee1` (+5511986896322), thread `74d83407-b758-465c-9d16-975eab3a75d1`, incidente 2026-07-08.
- Módulo alvo: `internal/agents` sobre o substrato `internal/platform` (kernel `internal/platform/workflow` permanece genérico).

## Visão Geral

O agente financeiro do MeControla no WhatsApp oferece ao usuário a criação de orçamento por conversa ("Posso te ajudar a criar um?"), conduz o diálogo, mas **não existe tool/fluxo que persista o orçamento**: na confirmação final o modelo recorre ao tool `adjust_allocation` sobre uma competência inexistente, o use case falha (`BudgetNotFound`), o run fecha como `failed/usecaseError` e o usuário recebe o fallback genérico "Não consegui criar o orçamento. Tente novamente em breve." Em paralelo, o agente resolve "mês passado" incorretamente (respondeu "setembro de 2023" para uma pergunta cuja resposta correta é junho de 2026) e exibe a competência em formato ISO (`2026-06`) em vez de por extenso ("junho de 2026").

Esta fatia de valor entrega uma **capacidade conversacional real de criação e ativação de orçamento** — inclusive **retroativa, sem limite inferior de antiguidade** — com **distribuição por categoria coletada por diálogo**, **resolução determinística de mês** (relativo, nomeado, passado e futuro), **exibição de todo mês por extenso** e **retrospectiva planejado vs realizado** do mês. Para quem: usuário final pessoa física que controla finanças pessoais no WhatsApp. Por que é valioso: elimina uma promessa quebrada do agente (oferece algo que não executa), remove mensagens de erro genéricas neste caminho e permite organizar e entender qualquer mês.

## Objetivos

- Permitir criar e ativar orçamento por conversa para qualquer competência (passada sem limite, corrente ou futura), com distribuição completa por categoria, sem que a confirmação resulte em erro genérico.
- Resolver meses relativos e nomeados de forma determinística e correta, eliminando a classe de bug "mês errado" (ex.: "mês passado" → "setembro de 2023").
- Exibir toda referência de competência ao usuário por extenso ("junho de 2026"), mantendo `YYYY-MM` apenas como contrato interno.
- Entregar retrospectiva planejado vs realizado por categoria e total, com percentual de execução.
- **Critério de sucesso primário mensurável (composto):**
  - Taxa de conclusão do fluxo de criação de orçamento por conversa ≥ meta acordada (baseline atual = 0% executável).
  - **Zero** ocorrência do fallback genérico ("Não consegui criar o orçamento…" / "não entendi") neste caminho quando o use case retorna sucesso.
  - Gate de avaliação **real-LLM ≥ 0.90** nos cenários-chave (criação com distribuição, retroativo, resolução de mês, retrospectiva), executado com `RUN_REAL_LLM=1`.
- Métrica de instrumentação: passar a existir a série `agent_tool_invocations_total{tool="create_budget"}`; reduzir a zero os runs `failed/usecaseError` originados por criação de orçamento via `adjust_allocation` sobre competência inexistente.

## Histórias de Usuário

- Como usuário do WhatsApp, quero criar um orçamento conversando (inclusive de um mês passado), informando a distribuição por categoria, para que ele seja de fato persistido e ativado ao confirmar — sem mensagem de erro genérica.
- Como usuário do WhatsApp, quero que o assistente entenda "mês passado", "mês que vem" e meses nomeados corretamente, para que ele não fale de um mês errado.
- Como usuário do WhatsApp, quero que o assistente sempre cite os meses por extenso ("junho de 2026"), para ler de forma natural.
- Como usuário do WhatsApp, quero perguntar "como foi meu mês de junho de 2026?" e receber o comparativo entre o que planejei e o que realizei, para entender o mês.
- Casos de borda cobertos: distribuição que não fecha 100%; competência muito antiga; competência já existente (inclusive draft de mês futuro); mês nomeado sem ano; retrospectiva sem orçamento; retrospectiva sem lançamentos; confirmação negada; falha real do use case.

## Funcionalidades Core

1. **Criação conversacional de orçamento com HITL.** Fluxo multi-turno durável (suspend/resume) que coleta total e distribuição por categoria, apresenta resumo, exige confirmação humana explícita e persiste via tool fina `create_budget` (adapter sobre os use cases de criação e ativação já existentes). Espelha o padrão de `BuildOnboardingWorkflow`/`BuildDestructiveConfirmWorkflow`.
2. **Resolução determinística de mês.** Função pura (`Decide*`, recebe `now time.Time`, sem relógio interno) que resolve "mês atual", "mês passado", "mês que vem"/"próximo mês" e meses nomeados com ano para `YYYY-MM` em `America/Sao_Paulo`; entrada ambígua (incluindo mês nomeado sem ano) pede esclarecimento. Aplicada a **todos** os tools que recebem competência.
3. **Formatação de mês por extenso.** Mapeamento competência → texto ("junho de 2026") aplicado a toda saída ao usuário, sem alterar o dado persistido (`YYYY-MM` permanece contrato interno).
4. **Retrospectiva planejado vs realizado.** Comparativo por categoria e total (planejado das allocations ativas, realizado dos lançamentos do mês, percentual de execução), reutilizando as leituras existentes (`query_plan` + `query_month`) sem nova fonte de verdade.
5. **Confiabilidade e auditabilidade.** Capacidade só é oferecida quando há tool/fluxo que a execute; falha de execução usa mensagem específica de indisponibilidade, distinta do fallback de "não entendi"; todo run permanece auditável com status/outcome fechados.

## Requisitos Funcionais

### Criação conversacional de orçamento

- RF-01: O sistema DEVE expor um tool fino `create_budget` (adapter sobre os use cases de criação e ativação de orçamento existentes), resolvido por registry, sem `switch case intent.Kind`, sem regra de negócio, SQL ou branching de domínio.
- RF-02: A criação DEVE ocorrer em um workflow durável com suspend/resume e HITL, coletando total e distribuição por categoria antes de persistir, espelhando os workflows de onboarding e de confirmação destrutiva existentes.
- RF-03: A distribuição DEVE ser coletada por diálogo completo por categoria antes de criar; o sistema NÃO reaproveita perfil de distribuição de outros meses automaticamente (D1).
- RF-04: A ativação DEVE exigir total maior que zero e distribuição somando exatamente 100% (10000 basis points); enquanto a soma for diferente de 100%, o agente pede o ajuste e o orçamento NÃO é ativado.
- RF-05: O sistema DEVE aceitar competência retroativa de qualquer mês passado, sem limite inferior de antiguidade (D2); competência futura permanece permitida; a única validação de tempo é o formato `YYYY-MM`.
- RF-06: Antes de exibir a pergunta de confirmação, o estado de espera do diálogo (tipo fechado: aguardando total / aguardando distribuição / aguardando confirmação) DEVE ser persistido no `Snapshot` do kernel; o resume aplica JSON merge-patch sobre o `Snapshot.State` antes de qualquer parse.
- RF-07: A confirmação humana explícita DEVE ser obrigatória antes de persistir; ao efetivar, cancelar ou expirar, o estado de espera é limpo e o run é encerrado (`succeeded`/`failed`), nunca permanecendo `suspended`.
- RF-08: Ao confirmar ("sim"), o orçamento DEVE ser criado e ativado com allocations somando 100%, e o usuário DEVE receber confirmação de sucesso citando o mês por extenso.
- RF-09: Ao negar ("não"/"cancela"), NENHUM orçamento é criado, o estado de espera é limpo e o run encerra sem efeito.
- RF-10: A escrita DEVE ser idempotente, sem duplicar orçamento em replay de mensagem, ancorada na identidade do inbound (`agent.InboundIdentityFromContext`). O mecanismo concreto (chave do run durável + detecção de replay do `messageID` no gate de confirmação + unicidade `(user_id, competence)`) é definido na especificação técnica.

### Unicidade e competência já existente

- RF-11: A unicidade `(user_id, competence)` DEVE ser respeitada: se já existir orçamento para a competência, o fluxo informa e NÃO duplica.
- RF-12: Se a competência alvo já existir como orçamento **draft** de mês futuro (state=1, auto_draft), o fluxo DEVE tratá-la como já existente — informa que já há orçamento e NÃO duplica nem ativa via este fluxo (D5). Ativação/edição de orçamento existente permanece Fora de Escopo.

### Resolução determinística de mês

- RF-13: A resolução de mês DEVE ser determinística e pura (recebe `now time.Time`, sem relógio interno), em `America/Sao_Paulo`: "mês atual" = competência corrente; "mês passado" = corrente menos um; "mês que vem"/"próximo mês" = corrente mais um (D9).
- RF-14: Meses nomeados COM ano DEVEM resolver para o `YYYY-MM` exato (ex.: "junho de 2026" → `2026-06`, "janeiro de 2025" → `2025-01`).
- RF-15: Mês nomeado SEM ano (ex.: apenas "junho") DEVE pedir esclarecimento do ano em vez de assumir (D6).
- RF-16: Expressão sem mês nem referência relativa reconhecível DEVE pedir que o usuário informe o mês, em vez de assumir um período.
- RF-17: O resolvedor determinístico DEVE ser aplicado a **todos** os tools que recebem competência (incluindo `query_month` e `query_plan`, além dos novos de criação e retrospectiva), preservando o fallback para mês corrente onde já existe (D8).

### Mês por extenso

- RF-18: Toda saída ao usuário que cite competência DEVE usar mês por extenso (ex.: "junho de 2026"); `YYYY-MM` permanece apenas como contrato interno e o formato de armazenamento não muda.
- RF-19: O idioma da formatação é português do Brasil; internacionalização está Fora de Escopo.

### Retrospectiva planejado vs realizado

- RF-20: A retrospectiva DEVE usar as allocations ativas como planejado e os lançamentos reais do mês como realizado, apresentando por categoria e total: valor planejado, valor realizado e percentual de execução, com o mês por extenso.
- RF-21: A retrospectiva DEVE reutilizar as leituras existentes (`query_plan` para planejado e `query_month` para realizado), sem introduzir nova fonte de verdade.
- RF-22: Retrospectiva de mês COM orçamento ativo mas SEM lançamentos DEVE apresentar o comparativo com realizado = 0 e execução 0% por categoria e total (D10) — sem caso especial de "sem movimentação".
- RF-23: Retrospectiva de mês SEM orçamento mas COM lançamentos reais DEVE oferecer criar o orçamento E apresentar os gastos realizados do mês (realizado sem planejado) (D7).
- RF-24: Retrospectiva de mês SEM orçamento e SEM lançamentos DEVE oferecer criar o orçamento.

### Confiabilidade, mensagens e auditoria

- RF-25: Uma capacidade só DEVE ser oferecida ao usuário quando existe tool/fluxo que a execute.
- RF-26: Falha de execução do use case de criação DEVE devolver mensagem específica de indisponibilidade temporária, **distinta** do fallback de "não entendi"; o run DEVE ser registrado como falho e auditável.
- RF-27: Todo run DEVE ser auditável com status e outcome fechados (`RunStatus`/`ToolOutcome`), com `thread_id`, `run_id`, `workflow`, `tool`, `status`, `duration_ms` e erro quando houver.
- RF-28: Estados de fronteira (estado de espera do diálogo, outcome do tool, status do run) DEVEM ser tipos fechados (state-as-type), nunca string livre.
- RF-29: Métricas DEVEM ter cardinalidade controlada: proibido `user_id` ou `competence` como label; labels permitidos são enums fechados (`agent_id`, `channel`, `workflow`, `tool`, `status`, `outcome`).
- RF-30: **Lacuna de observabilidade a corrigir:** a mensagem de erro do use case DEVE ficar persistida de forma auditável (o incidente teve `platform_runs.error` vazio, spans sem `status=error` e logs apenas de outbox), garantindo que a causa de um run falho seja recuperável.

## Experiência do Usuário

- Fluxo principal (criação): usuário pede orçamento para um mês → agente resolve a competência (por extenso) → coleta total → coleta distribuição por categoria até fechar 100% → apresenta resumo (mês por extenso + distribuição) → pede confirmação → ao "sim", cria/ativa e confirma sucesso.
- Fluxo retrospectiva: usuário pergunta "como foi meu mês de junho de 2026?" → agente resolve competência → se há orçamento, mostra comparativo planejado vs realizado (execução %); se não há orçamento mas há lançamentos, mostra realizados e oferece criar; se não há nada, oferece criar.
- Regras de confirmação (reuso do contrato de confirmação destrutiva existente): confirmação/cancelamento explícito encerra; resposta ambígua re-pergunta uma vez e, persistindo a ambiguidade, cancela sem efeito; TTL expirado cancela sem efeito e o texto segue para o parse normal.
- Toda referência de mês exibida por extenso; mensagens de erro específicas separam "não entendi" de "falhei ao executar".

## Restrições Técnicas de Alto Nível

- Canal único WhatsApp (Meta) inbound; usuário autenticado como principal do inbound; leitura e escrita apenas dos próprios dados.
- Substrato obrigatório: `internal/platform/agent` (Thread→Run, HITL, merge-patch no resume, run auditável) e `internal/platform/workflow` (kernel genérico, sem domínio); LLM apenas nas call-sites sancionadas; OpenRouter único provider.
- Reuso obrigatório dos use cases de criação/ativação de orçamento já existentes (consumidos hoje só pelo onboarding) e das constraints `budgets_user_comp_uk` e `budgets_competence_chk`.
- Invariantes de domínio preservadas em `internal/budgets`: total > 0; soma de allocations = 10000 bps para ativar; unicidade `(user_id, competence)`; formato `YYYY-MM`; estados fechados draft(1)/active(2).
- Governança de código: zero comentários em `.go` de produção; tool fina sem regra/SQL/branching; `Decide*` puro; DTO de tool com `Validate()`/`errors.Join`; testes testify/suite whitebox com mocks via `.mockery.yml`; validação real-LLM obrigatória.

## Fora de Escopo

- Edição e exclusão de orçamento por conversa (inclusive ativação conversacional de drafts já existentes).
- Reaproveitamento automático de distribuição entre meses.
- Materialização retroativa de lançamentos/recorrências e recomputo histórico de alertas.
- Interpretação de intervalos de datas ou trimestres; internacionalização além de português do Brasil.
- Mudança do formato de armazenamento da competência (`YYYY-MM` permanece).
- Redesenho geral das mensagens de erro do agente fora do contexto de orçamento.

## Suposições e Questões em Aberto

Questões materiais foram resolvidas em duas rodadas de esclarecimento (decisões D1–D10 acima refletidas nos RFs). Suposições resolvidas por derivação da base de código existente, sem questão em aberto:

- SUP-1: O contrato de confirmação (TTL, re-prompt único, semântica estrita, replay) reutiliza o já implementado no fluxo de confirmação destrutiva do substrato — não é redesenhado nesta fatia.
- SUP-2: A distribuição por categoria usa o conjunto de categorias raiz já usado no onboarding (custo_fixo, liberdade_financeira, metas, prazeres, conhecimento) e é expressa em basis points somando 10000.
- SUP-3: O "realizado" da retrospectiva agrega os lançamentos do mês por categoria raiz, reutilizando o retorno de `query_month`; o "planejado" reutiliza o retorno de `query_plan`.
- SUP-4: O parsing do valor total (ex.: "R$ 13.874,40" → centavos) reutiliza o comportamento existente do agente.
- SUP-5: A implementação ocorre em etapa posterior (especificação técnica + tarefas); este PRD não altera código de produção.

Nenhuma questão em aberto remanescente.
