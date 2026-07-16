# US-001: Recorrência do Orçamento por Linguagem Natural no Onboarding

## Declaração
Como novo assinante do MeControla que está concluindo o onboarding financeiro pelo WhatsApp, quero responder em linguagem natural se e por quantos meses (entre 1 e 12) meu orçamento deve se repetir, para deixar meu planejamento replicado exatamente pelo período que eu decidir, com confirmação clara da decisão aplicada.

## Contexto
- Problema: o step de recorrência do onboarding (`internal/agents/application/workflows/onboarding_workflow.go:1517-1554`) só interpreta sim/não via `recurrenceSchema={confirmed:bool}` (`onboarding_workflow.go:687-694`, `821`) e sempre chama `budgets.CreateRecurrence(ctx, userUUID, competence, 12)` com o número de meses cravado em `12` (`onboarding_workflow.go:1548`). Não existe caminho para o usuário pedir uma quantidade específica de meses, nem tratamento de quantidade inválida, nem confirmação explícita da decisão no momento em que ela é tomada — ao concluir (tanto no "sim" quanto no "não") o step segue direto para o step de cartões sem devolver mensagem (`onboarding_workflow.go:1539-1552`), e a recorrência só reaparece no resumo final com o texto fixo "repete pelos próximos 12 meses" (`recurrenceSummaryLine`, `onboarding_workflow.go:966-971`).
- Resultado esperado: o agente interpreta a resposta do usuário por linguagem natural e resolve uma de três decisões — sem recorrência, recorrência padrão de 12 meses, ou recorrência por uma quantidade específica de 1 a 12 meses (numérica ou por extenso) — aplica exatamente o período resolvido via `CreateRecurrence(...months)`, confirma imediatamente a decisão ao usuário e reflete o período correto no resumo final, seguindo rigorosamente o Tom de Voz oficial. Quantidades fora de 1–12 e respostas ininteligíveis não aplicam recorrência: o agente repergunta.
- Fonte: documento de produto `US_Recorrencia_do_Orcamento_MeControla.md` (fornecido pelo usuário) confrontado com a implementação atual do workflow de onboarding e do módulo `internal/budgets`.

## Regras de Negócio
- RN-01 (Cenário 1 — negativa): quando a resposta expressa intenção negativa (ex.: "não", "num quero", "só esse mês", "deixa só esse mês", "ñ", "n"), o agente não cria recorrência, mantém o orçamento apenas na competência atual e confirma ao usuário que o orçamento não será repetido.
- RN-02 (Cenário 2 — positiva sem quantidade): quando a resposta expressa intenção positiva sem informar quantidade (ex.: "sim", "quero", "pode repetir", "pode ser", "repete"), o agente aplica recorrência de 12 meses e confirma o período de 12 meses.
- RN-03 (Cenário 3 — quantidade específica): quando a resposta informa uma quantidade inteira entre 1 e 12, numérica ("6 meses", "só 3", "coloca por 6") ou por extenso ("seis meses", "três meses"), inclusive embutida em frase de linguagem natural ("sim, mas coloca só pra 3 meses", "deixa assim por 6 meses"), o agente converte a quantidade para número, aplica a recorrência por exatamente essa quantidade de meses e confirma o período aplicado.
- RN-04 (Prioridade de interpretação): a ordem é (1) quantidade específica válida entre 1 e 12 identificada → aplica a quantidade informada; (2) intenção positiva sem quantidade → aplica 12 meses; (3) intenção negativa → não aplica. Uma quantidade específica válida sempre prevalece sobre o padrão de 12 meses.
- RN-05 (Valores inválidos): quando a resposta contém uma quantidade fora do intervalo 1–12 (ex.: "0 meses", "13 meses", "15 meses", "24 meses"), o agente não aplica recorrência automaticamente e repergunta informando, no Tom de Voz oficial, que é possível repetir por um período entre 1 e 12 meses, solicitando uma quantidade válida.
- RN-06 (Resposta ininteligível/ambígua): quando a resposta não é claramente positiva, negativa nem uma quantidade utilizável (ex.: "talvez", "sei lá", emoji isolado, texto sem intenção reconhecível), o agente repergunta a questão de recorrência no Tom de Voz oficial, não assume negativa nem positiva e não aplica recorrência. Substitui o comportamento atual, em que o parser sim/não trataria essa entrada como "não" silenciosamente.
- RN-07 (Reperguntar sem limite): para valores inválidos (RN-05) e respostas ambíguas (RN-06), o agente repergunta indefinidamente até obter intenção válida (negativa, positiva ou quantidade 1–12), sem introduzir contador máximo de tentativas, consistente com os demais steps do onboarding (objetivo e orçamento mensal repergunta sem limite — `goalReprompt`/`monthlyBudgetReprompt`, `onboarding_workflow.go:703,731`). O abandono do onboarding permanece coberto pelo reaper de suspensos obsoletos existente (`OnboardingStaleAfter = 7 * 24 * time.Hour`, `onboarding_workflow.go:39`, `BuildOnboardingReaper`, `onboarding_workflow.go:1660-1662`).
- RN-08 (Confirmação imediata + resumo): o agente sempre confirma a decisão aplicada em dois pontos — (a) mensagem de confirmação explícita no momento da decisão (sem recorrência, 12 meses, ou N meses), prefixada no prompt do step seguinte de cartões, reutilizando o padrão já existente de confirmação encadeada `GoalConfirmation` (`onboarding_workflow.go:1113-1115`); e (b) a linha de recorrência do resumo final passa a refletir o período real aplicado (N meses ou sem recorrência), corrigindo o texto fixo "12 meses" de `recurrenceSummaryLine`.
- RN-09 (Aplicação downstream 1–12): o agente aplica a decisão via `BudgetPlanner.CreateRecurrence(ctx, userID, competence, months)` (`onboarding_workflow.go:1548`, interface `internal/agents/application/interfaces/budget_planner.go:13`, adapter `internal/agents/infrastructure/binding/budget_planner_adapter.go:120-134`) passando o número de meses resolvido. O intervalo 1–12 já é validado em profundidade no domínio de budgets (`internal/budgets/domain/commands/create_recurrence.go:12-40` com `minRecurrenceMonths=1`/`maxRecurrenceMonths=12`) e o usecase materializa exatamente N competências (`internal/budgets/application/usecases/create_recurrence.go:65-70`, laço `for range cmd.Months`); esta história não altera esse contrato de domínio.
- RN-10 (Estado do onboarding): o estado do onboarding passa a guardar a quantidade de meses da recorrência, além do indicador booleano atual (`OnboardingState.Recurrence bool`, `onboarding_workflow.go:321`), para que confirmação e resumo reflitam o N exato; ausência de recorrência é representada explicitamente (sem recorrência / zero meses).
- RN-11 (Tom de Voz oficial): todas as mensagens deste step (pergunta, reperguntas de inválido/ambíguo e confirmações) seguem o Tom de Voz oficial do MeControla, verificável pelos scorers oficiais — asterisco simples em vez de negrito duplo e presença de emoji oficial (`internal/agents/application/scorers/behavioral_scorers.go:318-345`) e aderência avaliada pelo scorer de tom (`internal/agents/application/scorers/mecontrola_scorers.go:50`, `toneAdherenceScorerInstructions`).
- RN-12 (Zero regressão de fluxo): o step de recorrência permanece na mesma posição da sequência (após ativação e antes de cartões — `onboarding_workflow.go:1649-1652`), continua sendo um step suspend/resume durável e mantém a semântica atual para as respostas positiva-padrão e negativa já suportadas.

## Critérios de Aceite
```gherkin
Cenário: Usuário recusa a recorrência
  Dado que o usuário concluiu a ativação do orçamento no onboarding
  E o agente perguntou se deve repetir o orçamento nos próximos 12 meses
  Quando o usuário responde "só esse mês"
  Então nenhuma recorrência é criada para as competências futuras
  E o agente confirma, no Tom de Voz oficial, que o orçamento não será repetido nos próximos meses
  E o resumo final indica que a recorrência está desligada

Cenário: Usuário aceita sem informar quantidade e recebe 12 meses
  Dado que o agente perguntou sobre repetir o orçamento
  Quando o usuário responde "pode repetir"
  Então o agente aplica a recorrência para os próximos 12 meses
  E o agente confirma, no Tom de Voz oficial, que o orçamento será repetido por 12 meses

Cenário: Usuário informa quantidade específica numérica dentro de 1 a 12
  Dado que o agente perguntou sobre repetir o orçamento
  Quando o usuário responde "sim, mas coloca só pra 3 meses"
  Então o agente aplica a recorrência por exatamente 3 meses, e não por 12
  E o agente confirma, no Tom de Voz oficial, que o orçamento será repetido por 3 meses
  E o resumo final reflete a recorrência de 3 meses

Cenário: Usuário informa quantidade por extenso em linguagem natural
  Dado que o agente perguntou sobre repetir o orçamento
  Quando o usuário responde "quero manter esse orçamento por oito meses"
  Então o agente converte "oito" para 8 e aplica a recorrência por 8 meses
  E o agente confirma, no Tom de Voz oficial, que o orçamento será repetido por 8 meses

Cenário: Quantidade fora do intervalo permitido não gera recorrência automática
  Dado que o agente perguntou sobre repetir o orçamento
  Quando o usuário responde "pode colocar nos próximos 24 meses"
  Então nenhuma recorrência é aplicada
  E o agente repergunta, no Tom de Voz oficial, informando que é possível repetir por um período entre 1 e 12 meses e pedindo uma quantidade válida

Cenário: Resposta ininteligível é reperguntada sem assumir negativa
  Dado que o agente perguntou sobre repetir o orçamento
  Quando o usuário responde "talvez"
  Então nenhuma recorrência é aplicada
  E o agente não assume intenção negativa nem positiva
  E o agente repergunta a questão de recorrência no Tom de Voz oficial

Cenário: Reperguntas se repetem até intenção válida
  Dado que o usuário enviou uma quantidade inválida "13 meses" e foi reperguntado
  Quando o usuário responde em seguida "então deixa por seis meses"
  Então o agente aplica a recorrência por 6 meses
  E o agente confirma, no Tom de Voz oficial, que o orçamento será repetido por 6 meses
```

## Dados e Permissões
- Dados obrigatórios: identidade do usuário do onboarding (`OnboardingState.UserID`, resolvida para `uuid.UUID`, `onboarding_workflow.go:1542-1545`); competência atual derivada de `America/Sao_Paulo` com fallback UTC (`competenceLocation` + `time.Now().In(loc).Format("2006-01")`, `onboarding_workflow.go:1210-1215,1546-1547`); orçamento já ativado no step anterior (`BuildActivationStep`, `onboarding_workflow.go:1501-1515`); texto livre da resposta do usuário (`OnboardingState.ResumeText`); número de meses resolvido (1–12) quando houver recorrência.
- Perfis/permissões: o próprio usuário assinante conduzindo seu onboarding pelo WhatsApp; a execução é escopada ao `resourceId = userID` do Thread/Run do onboarding (`ResolveOnboardingOrAgent`, `internal/agents/application/usecases/resolve_onboarding_or_agent.go:58-159`). Não há ação administrativa nem de terceiros neste fluxo.

## Dependências
- `BudgetPlanner.CreateRecurrence(ctx, userID, competence, months int)` — disponível na interface (`internal/agents/application/interfaces/budget_planner.go:13`) e no adapter de binding (`internal/agents/infrastructure/binding/budget_planner_adapter.go:120-134`); já aceita `months`.
- Domínio de recorrência do módulo budgets com validação 1–12 e materialização por N meses — disponível (`internal/budgets/domain/commands/create_recurrence.go:12-40`; `internal/budgets/application/usecases/create_recurrence.go:47-77`).
- Parsing por Structured Output do agente (LLM) com conversão de números por extenso — padrão já usado no onboarding para objetivo, valor da meta e distribuição, com exemplos de conversão por extenso nos system prompts (`onboarding_workflow.go:774-831`); a extensão do schema/prompt de recorrência segue esse mesmo mecanismo.
- Reaper de onboarding suspenso obsoleto — disponível (`onboarding_workflow.go:39-41,1660-1662`).
- Scorers oficiais de Tom de Voz para verificação — disponíveis (`internal/agents/application/scorers/behavioral_scorers.go:318-345`; `internal/agents/application/scorers/mecontrola_scorers.go:50`).

## Fora de Escopo
- Alterar, listar ou excluir a recorrência após a conclusão do onboarding (fluxos conversacionais do dia a dia, ex.: `budget_manage_workflow.go` e `goal_edit_workflow.go`, permanecem inalterados por esta história).
- Recorrência por período superior a 12 meses ou por período parcial/por categoria — mantém-se o limite de domínio 1–12 e a replicação do orçamento inteiro.
- Alterações no contrato de domínio do módulo `internal/budgets` (comando, usecase, validação de meses) — reaproveitados como estão.
- Canais fora do onboarding pelo WhatsApp e qualquer redesenho da ordem dos steps do onboarding.

## Evidências
- Entrada: `US_Recorrencia_do_Orcamento_MeControla.md` fornecido pelo usuário — define os três cenários, os exemplos de resposta, a regra de prioridade, o tratamento de valores inválidos, a exigência de confirmação sempre e a obrigatoriedade do Tom de Voz oficial.
- Base de código: step de recorrência atual apenas sim/não com 12 fixo (`internal/agents/application/workflows/onboarding_workflow.go:1517-1554`, schema `687-694`, prompt `821`); ausência de mensagem ao concluir (`1539-1552`); resumo com texto fixo (`recurrenceSummaryLine`, `966-971`, uso em `987`); estado booleano (`OnboardingState.Recurrence`, `321`); padrão de confirmação encadeada `GoalConfirmation` (`1113-1115`); interface/adapter `CreateRecurrence` com `months` (`budget_planner.go:13`, `budget_planner_adapter.go:120-134`); domínio 1–12 e materialização por N meses (`internal/budgets/domain/commands/create_recurrence.go:12-40`, `internal/budgets/application/usecases/create_recurrence.go:65-70`); scorers de Tom de Voz (`behavioral_scorers.go:318-345`, `mecontrola_scorers.go:50`); reaper de suspensos (`onboarding_workflow.go:39-41,1660-1662`).
- Inferências: a confirmação imediata deve reutilizar o padrão de prefixação `GoalConfirmation` no prompt do step de cartões (decisão do usuário confirmada em rodada de múltipla escolha); a interpretação de intenção/quantidade deve ocorrer via Structured Output do agente, coerente com os demais steps do onboarding que já usam LLM com schema e conversão por extenso.
- Não evidenciado: não há documento em prosa único intitulado "Tom de Voz oficial" no repositório; a fonte de verdade verificável do Tom de Voz é o conjunto de scorers citado (asterisco simples, emojis oficiais, aderência avaliada por `toneAdherenceScorerInstructions`), que os critérios de aceite usam como referência de conformidade.

## Notas de Validação
- Todas as ambiguidades materiais foram resolvidas em rodada de múltipla escolha com o usuário: confirmação imediata mais reflexo no resumo; resposta ambígua reperguntada sem assumir negativa; reperguntas sem limite até intenção válida; quantidade de meses incluída no estado e no resumo dentro do escopo desta história.
- Cobertura de cenários: fluxo feliz positivo (12 meses e N meses), fluxo alternativo (negativa, por extenso, quantidade específica prevalecendo sobre 12) e fluxos de erro/bloqueio (quantidade fora de 1–12 e resposta ininteligível reperguntadas). O caso negativo cobre também confirmação de "não repetir".
- Zero regressão exigida: preservados posição do step, durabilidade suspend/resume, reaper e o contrato de domínio de recorrência 1–12; a mudança é aditiva no step de onboarding (schema, decisão pura de resolução de meses, estado e mensagens de confirmação).
- Zero falso positivo: toda afirmação sobre suporte existente (meses 1–12 downstream, adapter, reaper, scorers, padrão de confirmação) está ancorada em caminho e linha verificáveis; a ausência de documento único de Tom de Voz foi declarada explicitamente com a fonte alternativa verificável.
