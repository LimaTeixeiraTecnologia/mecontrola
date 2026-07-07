# Alertas Proativos — User Stories

> Fonte: `MeControla_Alertas_Proativos_Documentacao_Final.md`
> Objetivo da funcionalidade: manter o usuário ativo, reduzir abandono silencioso e reforçar valor percebido do MeControla ao longo do mês.
> Data de geração: 2026-07-07
> Nome do arquivo: `2026-07-07-us-alertas-proativos.md`

---

## Confronto com o Codebase

Esta seção mapeia cada user story contra o código existente do repositório, identificando gaps e orientando a implementação futura com base nas skills `go-implementation` e `mastra`.

### Princípio orientador

> **Refatorar, reescrever e eliminar o legado quando necessário.**
>
> O codebase possui dois fluxos de alertas de threshold em `internal/budgets`: um legado baseado em `AlertRepository`/`EvaluateAlert` e um novo fluxo (`ThresholdWorkflow` + `ThresholdAlertSentRepository`). Esta documentação orienta a **consolidação no novo fluxo**, removendo código morto, duplicação e modos de execução dual (`legacy`/`job`/`both`).

### Arquitetura relevante identificada

- **Módulo `internal/budgets`**:
  - Domínio (novo fluxo — manter e expandir): `threshold_workflow.go` (`ThresholdWorkflow.DecideAlerts`), `domain/entities/alert.go` (`AlertState`), `domain/valueobjects/threshold.go` e `threshold_ratio.go`.
  - Domínio (legado — candidato a remoção): `threshold_evaluator.go`, `alert_workflow.go`, `alert_state_resolver.go` (`MaxDeliveredAlerts`).
  - Aplicação: `evaluate_threshold_alerts.go` (`EvaluateThresholdAlerts`), `notify_threshold_alert.go` (`NotifyThresholdAlert`).
  - Infraestrutura: `threshold_alerts_job.go` (`ThresholdAlertsJob` — refatorar para `job.NewAdapter`), `threshold_alert_notifier.go`, `threshold_alert_publisher.go`, repositórios Postgres.
- **Módulo `internal/agents`**:
  - Agente LLM: `BuildMeControlaAgent` em `application/agents/mecontrola_agent.go` — runtime genérico de tool-calling com instruções específicas de formatação para WhatsApp (negrito com `*`, proibição de `**`, emojis contextualizados).
  - Integração WhatsApp: `whatsapp_inbound_consumer.go` e roteador em `module.go`, usando gateway própria do módulo (`internal/platform/whatsapp/dispatcher`).
  - Tools: `internal/agents/application/tools/*` (registro, consultas, cartões, categorias, orçamento).
- **Plataforma `internal/platform/notification`**: interface `ChannelGateway` (`SendText`, `SendActivationTemplate`); implementação genérica `MultiChannelGateway`. O módulo `budgets` a consome via DI em `NotifyThresholdAlert`.
- **Plataforma `internal/platform/worker/job`**: padrão para cron jobs (`job.NewAdapter`, `job.NewAdapterWithTimeout`). O job existente deve ser migrado para esse padrão.
- **Plataforma `internal/platform/agent`, `tool`, `memory`, `workflow`**: substrato agentivo que deve ser consumido, não recriado (regra de ouro da skill `mastra`).

### Status por user story

| US | Alerta | Status no codebase | Onde vive hoje | O que falta / gap |
|----|--------|-------------------|----------------|-------------------|
| US-01 | Início de mês | **Não implementado** | — | Job cron + use case em `internal/budgets` para detectar ausência de orçamento no mês vigente e notificar via `ChannelGateway`. |
| US-02 | 80% da categoria | **Parcialmente implementado** | `threshold_workflow.go` (domínio), `notify_threshold_alert.go` (aplicação), `threshold_alerts_job.go` (infra) | `ThresholdConfig.Category` default é 0.80 (80%). Texto oficial não está aplicado (`renderThresholdAlertText` gera mensagem genérica). Falta resposta "sim" com detalhamento por subcategoria — implementar como tool em `internal/agents/application/tools`, delegando para use case de consulta. |
| US-03 | 90% da categoria | **Não implementado** | — | O novo fluxo usa `ThresholdConfig` com `ThresholdRatio` (default 0.80). É necessário evoluir a configuração para suportar múltiplos limiares por categoria (80%, 90%, 100%) e propagar em `DecideAlerts`, config e notificação. O enum legado `valueobjects.Threshold` e o `ThresholdEvaluator` devem ser removidos, não expandidos. |
| US-04 | 100% da categoria | **Parcialmente implementado** | `threshold_workflow.go` (domínio), `notify_threshold_alert.go`, `threshold_alerts_job.go` | O novo fluxo só define um ratio de categoria (default 0,80); não há limiar de 100 %. É necessário adicionar múltiplos ratios (80%, 90%, 100%), evoluir `DecideAlerts` para distinguir os limiares e aplicar o texto oficial com excedente. |
| US-05 | Meta atingida | **Parcialmente implementado** | `threshold_workflow.go` (`ThresholdAlertGoal`) | `ThresholdConfig.Goal` default é 0.50 (50%), não 100%. Texto genérico; precisa ajustar o default para 1.00 (100%) e aplicar o texto oficial de conquista. |
| US-06 | Fechamento do mês | **Não implementado** | — | Job/use case para consolidar planejado vs realizado por categoria no último dia do mês e enviar resumo. Pode reusar `GetMonthlySummary` como fonte de dados. |
| US-07 | Motivação — Constância | **Não implementado** | — | Job semanal que identifica usuários ativos e envia mensagem motivacional, respeitando exclusão com alertas críticos 90%/100%. |
| US-08 | Retomada de uso | **Não implementado** | — | Tracking de última interação/registro + job para disparar após 3 dias de inatividade para assinantes ativos. |
| US-09 | Risco de abandono | **Não implementado** | — | Tracking de inatividade + job para disparar após 7 dias, no máximo 1 vez por mês. |
| US-10 | Orçamento não revisado | **Não implementado** | — | Reforço no 3º dia do mês para usuários sem orçamento cadastrado/revisado. |
| US-11 | Prioridade de envio | **Não implementado** | — | Não existe orquestrador global de prioridade entre alertas. Cada job/disparo é independente. O modo dual `legacy`/`job`/`both` em `internal/budgets/module.go` deve ser eliminado junto com o fluxo legado. |
| US-12 | Regras gerais | **Parcialmente implementado** | `valueobjects` (categorias oficiais) | Tom de voz oficial não está nos textos atuais. Controle de frequência é local por alerta: novo fluxo deduplica por `(user_id, budget_id, kind, ref_day)` — 1 envio/dia/kind por orçamento. Não há garantia global de não-repetição entre diferentes jobs/fontes de alerta. |

### Notas de implementação orientadas pelas skills

#### go-implementation
- **Eliminar o legado**: remover `threshold_evaluator.go`, `alert_workflow.go`, `alert_state_resolver.go` e o modo dual `legacy`/`both` de `internal/budgets/module.go`. Consolidar tudo no novo fluxo (`ThresholdWorkflow` + `ThresholdAlertSentRepository`).
- **Refatorar `ThresholdAlertsJob`**: migrar para `job.NewAdapter` ou `job.NewAdapterWithTimeout` em `internal/budgets/infrastructure/jobs/handlers`.
- Todo novo job deve usar `job.NewAdapter` ou `job.NewAdapterWithTimeout`.
- Todo novo use case deve ser método de struct com DI via construtor explícito (R1).
- Regras de negócio novas devem viver em `domain/services` como métodos stateless puros quando possível (DMMF `Decide*`).
- Estados de alerta devem ser tipos fechados (`state-as-type`), sem `string` livre em assinatura pública.
- Novos repositórios devem declarar interfaces no consumidor (`application/interfaces`) e implementação em `infrastructure/repositories/postgres`.
- `init()` é proibida (R0); `context.Context` obrigatório em toda fronteira de IO (R6).

#### mastra
- Alertas proativos de disparo simples devem chegar ao usuário via `ChannelGateway.SendText` (no módulo `budgets`) ou via gateway WhatsApp do módulo `agents`, dependendo de onde forem implementados. Não forçar interação pelo agente LLM para mensagens unilaterais.
- Quando o alerta precisar de resposta inteligente (ex.: US-02 "sim" para detalhamento), implementar como **tool** no consumidor `internal/agents/application/tools`, delegando para use case existente de consulta. O `exec` da tool deve ser adapter fino, sem regra de negócio (R-ADAPTER-001).
- Não reimplementar primitivos de `internal/platform/{agent,llm,memory,workflow,tool,scorer}`; consumir o substrato existente.
- Se houver fluxo multi-step retomável (ex.: wizard de fechamento do mês), usar `workflow.Engine[S]`; se for disparo simples, usar job + use case + notificação.
- Toda escrita financeira idempotente via agente deve usar `agent.InboundIdentityFromContext` e `agent.WithWriteToolSet`.
- `BuildMeControlaAgent` é o runtime LLM + tools; a integração WhatsApp inbound está em `whatsapp_inbound_consumer.go` e no roteador de `internal/agents/module.go`.

### Riscos e dependências
- **Risco 1**: sem tracking centralizado de inatividade, US-07, US-08 e US-09 exigirão novo repositório/evento de atividade do usuário.
- **Risco 2**: US-11 (prioridade global) exige um orquestrador único de alertas diários, senão jobs independentes podem causar sobreposição.
- **Risco 3**: os textos oficiais usam `**negrito**` (markdown), mas o agente `mecontrola` exige `*negrito*` para WhatsApp. A camada de notificação deve adaptar a formatação conforme o canal.
- **Risco 4**: a remoção do fluxo legado (`EvaluateAlert`, `AlertRepository`, modos `legacy`/`both`) é uma mudança de comportamento; deve ser acompanhada de testes de regressão e confirmação de que nenhum consumidor depende do evento `budgets.expense.committed.v1` para alertas.

### Plano de consolidação sugerido (ordem de execução)
1. **Remover modo dual e legado** em `internal/budgets`: eliminar `threshold_evaluator.go`, `alert_workflow.go`, `alert_state_resolver.go`, `AlertRepository`, `EvaluateAlert` e as flags `ThresholdAlertsModeLegacy`/`Both`.
2. **Evoluir `ThresholdConfig`**: transformar de um único ratio por categoria/meta em uma lista/struct de limiares (80%, 90%, 100% para categorias; 100% para metas).
3. **Refatorar `ThresholdAlertsJob`**: migrar para `job.NewAdapter`/`job.NewAdapterWithTimeout`.
4. **Implementar novos alertas** (US-01, US-06 a US-10) como novos jobs/use cases, reusando `ChannelGateway` e padrões de deduplicação.
5. **Implementar orquestrador de prioridade** (US-11) para garantir ordem de envio quando múltiplos alertas forem elegíveis no mesmo dia.
6. **Aplicar textos oficiais** e adaptar formatação para WhatsApp (`*` em vez de `**`).

---

## US-01 — Alerta de Início de Mês

**Como** usuário do MeControla, **quero** receber um alerta no início do mês quando ainda não existir orçamento cadastrado para o mês vigente, **para que** eu inicie o ciclo mensal e não fique sem orçamento ativo no mês atual.

### Critérios de aceite
- O alerta deve ser enviado **somente** quando **não existir orçamento cadastrado para o mês vigente**.
- O alerta não deve ser enviado se o usuário já possui orçamento cadastrado para o mês vigente.
- O alerta deve conter o texto oficial e uma chamada para ação solicitando o orçamento.

### Texto oficial do alerta

**📅 Novo mês, novo controle**

Pra eu conseguir te avisar quando uma categoria estiver perto do limite, acompanhar sua evolução e fechar seu mês com clareza, primeiro preciso do seu orçamento deste mês.

**Qual seu orçamento para este mês?**

### Regras de negócio relacionadas
- Frequência: envio condicionado à ausência de orçamento no mês vigente.
- Prioridade de envio: 5º na ordem de prioridade quando múltiplos alertas estiverem elegíveis no mesmo dia.

### Rastreabilidade
- Documento fonte, seção 1 — Alerta de Início de Mês.

---

## US-02 — Alerta de 80% da Categoria

**Como** usuário do MeControla, **quero** ser avisado quando atingir 80% do orçamento planejado em qualquer categoria, **para que** eu perceba o risco antes de estourar o limite e analise a categoria com mais detalhe.

### Critérios de aceite
- O alerta deve ser disparado quando o usuário atingir **80% do orçamento planejado** em qualquer categoria.
- O alerta deve ser enviado **apenas 1 vez por categoria, por mês**.
- O alerta deve exibir: categoria, valor planejado, valor gasto até agora e saldo restante.
- Se o usuário responder **sim** à pergunta do alerta, o sistema deve retornar o detalhamento dos gastos daquela categoria por subcategoria.
- O detalhamento por subcategoria deve conter: subcategoria e valor gasto na subcategoria no mês.
- O detalhamento pode opcionalmente ser ordenado do maior para o menor gasto.

### Texto oficial do alerta

**⚠️ Atenção em {categoria}**

Você já usou **80%** do valor planejado para essa categoria neste mês.

Planejado: **R$ {valor_planejado}**
Gasto até agora: **R$ {valor_gasto}**
Ainda disponível: **R$ {saldo_restante}**

Ainda dá tempo de ajustar a rota sem deixar o mês sair do controle.

**Quer ver onde você mais gastou dentro de {categoria}?**

### Exemplo de resposta esperada para resposta "sim"

**Aqui está o detalhamento de {categoria} até agora:**

- **{subcategoria_1}** — R$ {valor_1}
- **{subcategoria_2}** — R$ {valor_2}
- **{subcategoria_3}** — R$ {valor_3}

Se quiser, depois eu também posso te ajudar a entender onde vale ajustar primeiro.

### Regras de negócio relacionadas
- Frequência: 1 vez por categoria, por mês.
- Categorias oficiais consideradas: Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira.
- Prioridade de envio: 7º na ordem de prioridade quando múltiplos alertas estiverem elegíveis no mesmo dia.

### Rastreabilidade
- Documento fonte, seção 2 — Alerta de 80% da Categoria.

---

## US-03 — Alerta de 90% da Categoria

**Como** usuário do MeControla, **quero** ser avisado quando atingir 90% do orçamento planejado em qualquer categoria, **para que** eu gere senso de atenção e olhe a fotografia completa do orçamento antes de estourar.

### Critérios de aceite
- O alerta deve ser disparado quando o usuário atingir **90% do orçamento planejado** em qualquer categoria.
- O alerta deve ser enviado **apenas 1 vez por categoria, por mês**.
- O alerta deve exibir: categoria, valor planejado, valor gasto até agora e saldo restante.

### Texto oficial do alerta

**🚨 Sua categoria {categoria} está quase no limite**

Você já consumiu **90%** do que planejou para ela neste mês.

Planejado: **R$ {valor_planejado}**
Gasto até agora: **R$ {valor_gasto}**
Ainda disponível: **R$ {saldo_restante}**

Vale olhar com carinho os próximos gastos dessa categoria pra não estourar.

**Quer ver seu orçamento completo por categoria?**

### Regras de negócio relacionadas
- Frequência: 1 vez por categoria, por mês.
- Categorias oficiais consideradas: Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira.
- Prioridade de envio: 2º na ordem de prioridade quando múltiplos alertas estiverem elegíveis no mesmo dia.

### Rastreabilidade
- Documento fonte, seção 3 — Alerta de 90% da Categoria.

---

## US-04 — Alerta de 100% da Categoria

**Como** usuário do MeControla, **quero** ser avisado quando atingir ou ultrapassar 100% do orçamento planejado em qualquer categoria, **para que** eu evite sensação de culpa e reorganize o restante do mês olhando o orçamento como um todo.

### Critérios de aceite
- O alerta deve ser disparado quando o usuário atingir ou ultrapassar **100% do orçamento planejado** em qualquer categoria.
- O alerta deve ser enviado **apenas 1 vez por categoria, por mês**.
- O alerta deve exibir: categoria, valor planejado, valor gasto atual e valor excedente.

### Texto oficial do alerta

**❌ A categoria {categoria} atingiu o limite do mês**

Você já usou todo o valor planejado para essa categoria.

Planejado: **R$ {valor_planejado}**
Gasto atual: **R$ {valor_gasto}**
Excedente: **R$ {valor_excedente}**

Calma. Isso não significa que o mês está perdido.

**Quer ver seu orçamento completo por categoria?**

### Regras de negócio relacionadas
- Frequência: 1 vez por categoria, por mês.
- Categorias oficiais consideradas: Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira.
- Prioridade de envio: 1º na ordem de prioridade quando múltiplos alertas estiverem elegíveis no mesmo dia.

### Rastreabilidade
- Documento fonte, seção 4 — Alerta de 100% da Categoria.

---

## US-05 — Alerta de Meta Atingida

**Como** usuário do MeControla, **quero** receber um alerta quando o valor acumulado de uma meta for igual ou maior que o valor definido por mim, **para que** eu sinta conquista real, crie vínculo emocional com o progresso e reforce o valor do MeControla.

### Critérios de aceite
- O alerta deve ser disparado quando o valor acumulado de uma meta for **igual ou maior** que o valor definido pelo usuário.
- O alerta deve ser enviado **apenas 1 vez por meta atingida**.
- O alerta deve exibir: nome da meta, valor da meta e valor acumulado.

### Texto oficial do alerta

**🎉 Você conseguiu. Sua meta {nome_meta} foi atingida.**

Olha o que isso significa na prática:
esse dinheiro não ficou perdido no mês, não foi engolido pela correria e nem saiu sem direção. Ele foi guardado com propósito — e agora virou conquista de verdade.

Valor da meta: **R$ {valor_meta}**
Valor acumulado: **R$ {valor_acumulado}**

É exatamente pra isso que o controle serve:
te aproximar da vida que você quer viver, um valor de cada vez. 💚

**Quer criar a próxima meta agora?**

### Regras de negócio relacionadas
- Frequência: 1 vez por meta atingida.
- Prioridade de envio: 3º na ordem de prioridade quando múltiplos alertas estiverem elegíveis no mesmo dia.

### Rastreabilidade
- Documento fonte, seção 5 — Alerta de Meta Atingida.

---

## US-06 — Alerta de Fechamento do Mês

**Como** usuário do MeControla, **quero** receber um resumo de fechamento do mês comparando o planejado com o realizado por categoria, **para que** eu perceba valor, veja minha evolução e crie ponte para o próximo ciclo.

### Critérios de aceite
- O alerta deve ser disparado no **último dia do mês** ou no **fechamento da competência financeira do usuário**.
- O alerta deve ser enviado **1 vez por mês**.
- O alerta deve apresentar, para cada categoria oficial, os valores: planejado, realizado e status.
- As categorias presentes no fechamento devem ser: Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira.

### Texto oficial do alerta

**📊 Fechamento do seu mês no MeControla**

Fechei o comparativo entre o que você planejou e o que realmente aconteceu em cada categoria:

**Custo Fixo**
Planejado: **R$ {planejado_custo_fixo}**
Realizado: **R$ {realizado_custo_fixo}**
Status: **{status_custo_fixo}**

**Conhecimento**
Planejado: **R$ {planejado_conhecimento}**
Realizado: **R$ {realizado_conhecimento}**
Status: **{status_conhecimento}**

**Prazeres**
Planejado: **R$ {planejado_prazeres}**
Realizado: **R$ {realizado_prazeres}**
Status: **{status_prazeres}**

**Metas**
Planejado: **R$ {planejado_metas}**
Realizado: **R$ {realizado_metas}**
Status: **{status_metas}**

**Liberdade Financeira**
Planejado: **R$ {planejado_liberdade}**
Realizado: **R$ {realizado_liberdade}**
Status: **{status_liberdade}**

Esse resumo mostra onde o dinheiro seguiu o plano e onde vale ajustar no próximo mês.

Quer que eu monte a base do orçamento do próximo mês com esses dados?

### Regras de negócio relacionadas
- Frequência: 1 vez por mês.
- Categorias oficiais consideradas: Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira.
- Prioridade de envio: 6º na ordem de prioridade quando múltiplos alertas estiverem elegíveis no mesmo dia.

### Rastreabilidade
- Documento fonte, seção 6 — Alerta de Fechamento do Mês.

---

## US-07 — Alerta de Motivação — Constância

**Como** usuário ativo do MeControla, **quero** receber uma mensagem semanal de motivação reforçando constância, **para que** eu mantenha o hábito de registrar gastos e permaneça emocionalmente engajado.

### Critérios de aceite
- O alerta deve ser enviado **1 vez por semana** para usuários ativos.
- Usuário ativo é definido como alguém que registrou gastos recentemente.
- O alerta **não deve ser enviado no mesmo dia** de alerta crítico de 90% ou 100%.

### Texto oficial do alerta

**💬 Só passando pra reforçar uma coisa:**

organizar o dinheiro não é sobre perfeição.
É sobre constância.

Cada lançamento que você faz hoje deixa seu mês mais claro amanhã.

Menos caos. Mais conquistas. 💚

### Regras de negócio relacionadas
- Frequência: 1 vez por semana.
- Exclusão: não enviar no mesmo dia de alerta crítico de 90% ou 100%.
- Prioridade de envio: 10º (último) na ordem de prioridade quando múltiplos alertas estiverem elegíveis no mesmo dia.

### Rastreabilidade
- Documento fonte, seção 7 — Alerta de Motivação — Constância.

---

## US-08 — Alerta de Retomada de Uso

**Como** usuário do MeControla com assinatura ativa, **quero** receber um alerta quando ficar 3 dias sem registrar gastos ou interagir, **para que** eu retome o uso e evite que o mês saia do trilho por abandono silencioso.

### Critérios de aceite
- O alerta deve ser disparado quando o usuário ficar **3 dias sem registrar gastos ou interagir**, estando com assinatura ativa.
- O alerta deve ser enviado **no máximo 1 vez por semana**.

### Texto oficial do alerta

**👀 Faz alguns dias que você não registra nada por aqui**

Se o mês saiu um pouco do trilho, tudo bem.
A gente retoma de onde parou.

Quer que eu te mostre como estão suas categorias até agora?

### Regras de negócio relacionadas
- Gatilho: 3 dias sem registro ou interação, com assinatura ativa.
- Frequência: no máximo 1 vez por semana.
- Prioridade de envio: 8º na ordem de prioridade quando múltiplos alertas estiverem elegíveis no mesmo dia.

### Rastreabilidade
- Documento fonte, seção 8 — Alerta de Retomada de Uso.

---

## US-09 — Alerta de Risco de Abandono

**Como** usuário do MeControla, **quero** receber um alerta de reativação quando ficar 7 dias ou mais sem interagir, **para que** eu reorganize meu mês e reduza o risco de churn por abandono.

### Critérios de aceite
- O alerta deve ser disparado quando o usuário ficar **7 dias ou mais sem interação**.
- O alerta deve ser enviado **no máximo 1 vez por mês**.

### Texto oficial do alerta

**📌 Seu mês ainda pode voltar pro controle**

Você ficou alguns dias sem registrar seus gastos, mas ainda dá tempo de organizar o que aconteceu.

Se quiser, eu te ajudo a fazer uma retomada simples:
primeiro a gente olha suas categorias, depois ajusta o restante do mês.

Quer retomar agora?

### Regras de negócio relacionadas
- Gatilho: 7 dias ou mais sem interação.
- Frequência: no máximo 1 vez por mês.
- Prioridade de envio: 9º na ordem de prioridade quando múltiplos alertas estiverem elegíveis no mesmo dia.

### Rastreabilidade
- Documento fonte, seção 9 — Alerta de Risco de Abandono.

---

## US-10 — Alerta de Orçamento Não Revisado

**Como** usuário do MeControla, **quero** receber um reforço caso continue sem orçamento cadastrado/revisado até o 3º dia do mês, **para que** eu evite que o mês avance sem orçamento ativo.

### Critérios de aceite
- O alerta deve ser disparado se o usuário continuar sem orçamento cadastrado/revisado até o **3º dia do mês**.
- O alerta deve ser enviado **apenas 1 reforço por mês**.
- O alerta deve explicar as consequências de não ter orçamento e propor o cadastro.

### Texto oficial do alerta

**📅 Seu orçamento do mês ainda não foi definido**

Sem ele, eu não consigo te avisar quando uma categoria estiver perto do limite, acompanhar seus gastos com clareza e te mostrar se o mês está indo bem ou não.

**Vamos cadastrar agora?**

### Regras de negócio relacionadas
- Gatilho: ausência de orçamento cadastrado/revisado até o 3º dia do mês.
- Frequência: 1 reforço por mês.
- Prioridade de envio: 4º na ordem de prioridade quando múltiplos alertas estiverem elegíveis no mesmo dia.

### Rastreabilidade
- Documento fonte, seção 10 — Alerta de Orçamento Não Revisado.

---

## US-11 — Prioridade de Envio dos Alertas

**Como** usuário do MeControla, **quero** que, quando mais de um alerta estiver elegível no mesmo dia, o sistema respeite uma ordem de prioridade, **para que** eu receba primeiro a informação mais crítica e evite sobreposição confusa.

### Critérios de aceite
- A ordem de prioridade deve ser respeitada quando múltiplos alertas estiverem elegíveis no mesmo dia:
  1. Alerta de 100% da categoria
  2. Alerta de 90% da categoria
  3. Alerta de meta atingida
  4. Alerta de orçamento não revisado
  5. Alerta de início de mês
  6. Alerta de fechamento do mês
  7. Alerta de 80% da categoria
  8. Alerta de retomada de uso
  9. Alerta de risco de abandono
  10. Alerta de motivação
- Alertas de mesma prioridade devem seguir as regras de frequência individuais de cada US.

### Regras de negócio relacionadas
- Prioridade aplica-se somente quando dois ou mais alertas estão elegíveis no mesmo dia.
- Regras de frequência individuais continuam válidas independentemente da prioridade.

### Rastreabilidade
- Documento fonte, seção 11 — Prioridade de Envio dos Alertas.

---

## US-12 — Regras Gerais dos Alertas

**Como** usuário do MeControla, **quero** que todos os alertas respeitem as regras gerais de categorias, frequência, tom de voz e intenção final, **para que** eu tenha uma experiência coesa, sem repetições e alinhada ao propósito do produto.

### Critérios de aceite
- Os alertas de categoria devem considerar **apenas** as categorias oficiais:
  - Custo Fixo
  - Conhecimento
  - Prazeres
  - Metas
  - Liberdade Financeira
- O MeControla **não deve enviar alertas repetidos sobre o mesmo evento**.
- Todos os alertas devem seguir o tom de voz:
  - Simples
  - Humanos
  - Leves
  - Objetivos
  - Sem julgamento
  - Com chamada para uma próxima ação
- Cada alerta deve fazer o usuário sentir: **“O MeControla está acompanhando meu dinheiro comigo.”**

### Regras de negócio relacionadas
- Aplicável a todas as USs de alertas (US-01 a US-10).
- Deve ser verificado em revisões de UX/copy e em testes de aceite de todos os alertas.

### Rastreabilidade
- Documento fonte, seção 12 — Regras Gerais.
