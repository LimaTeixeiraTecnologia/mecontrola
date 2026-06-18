# 2026-06-18 Onboarding Agent V1 Gap Analysis

## Objetivo

Este documento detalha tudo o que precisa ser implementado para que o fluxo integrado entre `internal/agent` e `internal/onboarding` atenda de forma real, sem falso positivo, o onboarding definido em `MeControla_Onboarding_SystemPrompt_V1.md`.

Escopo deste documento:
- comparar requisito versus comportamento real
- explicitar o que ja existe
- listar gaps funcionais, de modelagem, de persistencia, de orquestracao e de testes
- definir o que precisa existir para considerar o fluxo pronto para `main`
- evitar conclusoes falsas baseadas apenas em persona ou prompt amplo

## Veredito Atual

Status atual: **nao pronto** para afirmar aderencia ao onboarding V1.

O sistema atual possui:
- um agente conversacional financeiro funcional
- um onboarding persistido e integrado ao agente
- testes passando no escopo atual de `internal/agent` e `internal/onboarding`

Mas o sistema **nao implementa** o roteiro V1 fim a fim. Hoje ele cobre um onboarding reduzido, centrado em:
- captura de renda
- decisao sobre cadastrar cartao
- cadastro de cartao
- aplicacao de sugestao padrao de distribuicao
- conclusao do onboarding

Isso e insuficiente para cumprir o V1, que exige:
- boas-vindas estruturadas
- explicacao didatica da metodologia
- explicacao individual das 5 categorias com confirmacao
- definicao de objetivo financeiro
- orcamento por valores individuais por categoria
- validacao da distribuicao contra renda
- resumo final com ajuste/confirmar
- transicao para primeiro lancamento
- celebracao apos primeiro lancamento
- encerramento com sensacao de onboarding concluido

## Evidencias do Estado Atual

### 1. Inicio real do fluxo

O inicio oficial do onboarding via agente acontece pela use case `StartBudgetConfiguration`, que:
- cria ou reseta sessao em `AwaitingIncome`
- responde com pergunta direta sobre renda mensal

Evidencias:
- `internal/onboarding/application/usecases/start_budget_configuration.go`
- resposta inicial: "Beleza! Qual a sua renda mensal? Pode me dizer o valor."

Impacto:
- nao ha etapa de boas-vindas
- nao ha etapa de duvida inicial
- nao ha explicacao da metodologia antes da coleta de dados
- nao ha explicacao do motivo da etapa atual

### 2. Modelo persistido da sessao

A sessao de onboarding hoje persiste apenas:
- `IncomeCents`
- `Cards`
- `PendingCard`
- `HasPending`
- `Split`

Evidencia:
- `internal/onboarding/domain/entities/onboarding_session.go`

Impacto:
nao existe estrutura persistida para:
- etapa atual do roteiro V1 alem do fluxo reduzido
- objetivo principal do usuario
- confirmacao individual das categorias
- quantidade informada de cartoes
- cartoes sem vencimento quando o roteiro pedir apenas fechamento
- orcamento por valor absoluto por categoria
- percentuais calculados por categoria
- saldo restante nao distribuido
- estado de ajuste de orcamento
- resumo final confirmado
- estado de transicao para primeiro lancamento
- estado de primeiro lancamento concluido
- mensagem de celebracao ja emitida

### 3. Maquina de estados atual

A maquina atual cobre:
- `AwaitingToken`
- `AwaitingIncome`
- `AwaitingCardDecision`
- `AwaitingCardName`
- `AwaitingCardLimit`
- `AwaitingCardClosingDay`
- `AwaitingCardDueDay`
- `AwaitingMoreCards`
- `AwaitingSplitConfirm`
- `Active`

Evidencia:
- `internal/onboarding/domain/services/onboarding_workflow.go`

Impacto:
faltam estados para:
- boas-vindas
- metodologia
- categoria 1 confirmada
- categoria 2 confirmada
- categoria 3 confirmada
- categoria 4 confirmada
- categoria 5 confirmada
- coleta de objetivo principal
- coleta de quantidade de cartoes
- loop por apelido de cartao no formato V1
- distribuicao por categoria em valores
- validacao de distribuicao
- tela de resumo
- ajuste de categorias
- transicao para primeiro lancamento
- aguardando primeiro lancamento
- primeiro lancamento confirmado
- celebracao
- encerramento

### 4. Prompt/persona em producao

A persona atual do agente e ampla e generica. Ela:
- descreve o MeControla como parceiro financeiro conversacional
- orienta a jornada de forma flexivel
- evita soar como robo preso a roteiro

Evidencia:
- `internal/agent/application/prompting/persona.system.tmpl`

Impacto:
isso ajuda o agente geral, mas nao garante onboarding V1. O risco e parecer correto no texto sem executar o fluxo real exigido.

### 5. Integracao real agent -> onboarding

A integracao existe e esta conectada no servidor:
- `StartBudgetConfiguration` e usado pelo agent para iniciar configuracao
- `ProcessConversation` do onboarding e usado como continuacao do fluxo

Evidencias:
- `cmd/server/agent_wiring.go`
- `internal/agent/application/services/intent_router.go`

Impacto:
a infraestrutura de integracao existe. O principal problema nao e wiring; e cobertura funcional incompleta do onboarding V1.

### 6. Cobertura de testes atual

Os testes atuais provam o fluxo existente, incluindo:
- criacao/resume/reset da sessao
- avanco de estados de onboarding
- publicacao de eventos em passos existentes
- suite de `internal/agent` e `internal/onboarding` passando

Validacao ja observada:
- `go test ./internal/agent/... ./internal/onboarding/...`

Impacto:
isso da confianca no produto atual, mas nao comprova aderencia ao onboarding V1.

## Gaps Funcionais do V1

### Gap 1. Boas-vindas estruturadas

Requisito V1:
- mensagem de boas-vindas
- tom acolhedor
- explicacao de que o onboarding vai organizar a vida financeira
- opcoes "Vamos la" e "Tenho uma duvida"

Estado atual:
- inexistente

Implementacao necessaria:
- novo estado inicial de boas-vindas
- resposta padronizada de entrada
- tratamento de confirmacao de inicio
- tratamento de duvida antes de avancar
- persistencia do usuario ainda nao iniciado versus iniciou onboarding

### Gap 2. Explicacao da metodologia MeControla

Requisito V1:
- explicar que o dinheiro e organizado em 5 categorias principais
- apresentar as categorias individualmente
- apos cada uma perguntar "Faz sentido para voce?"

Estado atual:
- inexistente como workflow
- existe apenas conhecimento geral da persona e da modelagem de orcamento

Implementacao necessaria:
- estados para cada explicacao de categoria
- controle de progressao categoria por categoria
- persistencia da confirmacao de entendimento por etapa
- possibilidade de usuario responder duvida ou discordancia sem quebrar fluxo
- respostas didaticas e nao genericas

### Gap 3. Definicao de objetivo principal

Requisito V1:
- perguntar o objetivo financeiro
- registrar resposta
- reapresentar no resumo final

Estado atual:
- inexistente no onboarding
- existe apenas intent de consulta de meta no agente, que e outra funcionalidade

Implementacao necessaria:
- novo campo persistido para objetivo principal
- novo estado de coleta
- regras para aceitar texto livre
- possibilidade de resposta aberta
- exibicao posterior no resumo

### Gap 4. Sequencia de coleta desalinhada

Requisito V1:
- boas-vindas
- metodologia
- objetivos
- renda
- cartoes
- distribuicao
- resumo
- transicao
- primeiro lancamento
- celebracao
- encerramento

Estado atual:
- renda
- cartao opcional
- split padrao
- conclusao

Implementacao necessaria:
- reordenar a jornada do onboarding conversacional
- separar “orcamento” de “onboarding reduzido”
- preservar reentrada e retomada sem perder consistencia

### Gap 5. Fluxo de cartoes incompativel com V1

Requisito V1:
- perguntar quantos cartoes o usuario usa
- se `0`, seguir
- para cada cartao pedir apelido
- pedir fechamento
- confirmar cadastro

Estado atual:
- pergunta se quer cadastrar cartao
- pede nome
- pede limite
- pede fechamento
- pede vencimento
- pergunta se quer outro

Impacto:
o comportamento atual e diferente do prompt V1 e do copy esperado.

Implementacao necessaria:
- decidir se o V1 continuara exigindo somente fechamento ou se o produto mantera vencimento e limite como obrigatorios
- se o produto precisar manter limite e vencimento, o prompt V1 precisa ser atualizado para refletir comportamento real
- se o objetivo e aderir ao V1 literal, a modelagem do fluxo deve aceitar cadastro minimo com apelido + fechamento
- prever compatibilidade com o modulo de cartoes real, que hoje conhece limite e vencimento

Decisao recomendada:
- alinhar produto e prompt antes da implementacao
- preferencialmente atualizar o onboarding V1 para incluir limite e vencimento, pois isso e mais coerente com o dominio real do sistema

### Gap 6. Distribuicao de orcamento por valores absolutos

Requisito V1:
- perguntar valor individual por categoria
- somar valores
- calcular percentuais
- validar contra renda
- lidar com saldo restante
- impedir ultrapassar renda
- permitir ajuste

Estado atual:
- apenas sugestao fixa de split padrao
- sem input por categoria
- sem validacao de soma contra renda
- sem ciclo de ajuste
- sem resumo detalhado antes da confirmacao

Implementacao necessaria:
- novo modelo persistido de alocacoes por categoria
- calculo de percentuais derivado de renda
- validacao de soma
- estados para cada categoria:
  - custo fixo
  - conhecimento
  - prazeres
  - metas
  - liberdade financeira
- estado de revisao da soma
- estado de erro por excedente
- estado de saldo restante
- estado de ajuste
- estado de confirmacao final

### Gap 7. Resumo final com confirmar/ajustar

Requisito V1:
- mostrar renda
- mostrar distribuicao por categoria com valor e percentual
- listar cartoes
- mostrar objetivo principal
- perguntar se faz sentido
- permitir ajustar categorias e gerar novo resumo

Estado atual:
- inexistente

Implementacao necessaria:
- formatter de resumo de onboarding
- estado de confirmacao
- loop de ajuste retornando as categorias
- preservacao dos dados ja coletados
- evitar duplicacao de evento final enquanto o usuario ainda estiver ajustando

### Gap 8. Transicao para primeiro lancamento

Requisito V1:
- apos confirmacao do planejamento, conduzir usuario ao primeiro lancamento
- pedir um exemplo de gasto ou renda real

Estado atual:
- inexistente no onboarding
- o agente suporta intents de lancamento, mas nao como etapa guiada do onboarding

Implementacao necessaria:
- novo estado “aguardando primeiro lancamento”
- integracao explicita onboarding -> agent logging
- roteamento que reconheca que o usuario esta em onboarding e que a proxima mensagem precisa ser tratada como tentativa de primeiro lancamento
- resposta de orientacao quando o texto ainda nao for um lancamento valido

### Gap 9. Celebracao e encerramento

Requisito V1:
- apos primeiro lancamento bem-sucedido, celebrar
- informar que o onboarding foi concluido
- reforcar valor do produto

Estado atual:
- inexistente

Implementacao necessaria:
- deteccao confiavel de primeiro lancamento persistido com sucesso
- resposta de celebracao
- marcacao persistida de onboarding concluido apos o primeiro lancamento, nao antes
- eventual emissao de evento especifico de conclusao real do onboarding V1

## Gaps de Modelagem

### Sessao de onboarding

Adicionar a payload da sessao, no minimo:
- etapa/subetapa do onboarding V1 quando a enum atual nao for suficiente
- objetivo principal
- flags de confirmacao da metodologia
- quantidade informada de cartoes
- indice atual do cartao em cadastro
- cartoes em coleta no formato do onboarding
- alocacoes por categoria em valor absoluto
- percentuais calculados
- total distribuido
- saldo restante
- status de resumo confirmado
- status de transicao para primeiro lancamento
- status de primeiro lancamento concluido
- status de celebracao emitida

### Estados

Expandir `OnboardingState` para refletir o fluxo real. O implementador nao deve improvisar estados. Recomenda-se criar estados explicitos para:
- `AwaitingWelcomeDecision`
- `AwaitingMethodologyIntroAck`
- `AwaitingCategoryFixedCostAck`
- `AwaitingCategoryKnowledgeAck`
- `AwaitingCategoryPleasuresAck`
- `AwaitingCategoryGoalsAck`
- `AwaitingCategoryFinancialFreedomAck`
- `AwaitingPrimaryGoal`
- `AwaitingIncome`
- `AwaitingCardCount`
- `AwaitingCardNickname`
- `AwaitingCardClosingDay`
- `AwaitingNextCard`
- `AwaitingBudgetFixedCost`
- `AwaitingBudgetKnowledge`
- `AwaitingBudgetPleasures`
- `AwaitingBudgetGoals`
- `AwaitingBudgetFinancialFreedom`
- `AwaitingBudgetAdjustmentDecision`
- `AwaitingBudgetAdjustmentTarget`
- `AwaitingBudgetSummaryConfirm`
- `AwaitingFirstLaunch`
- `AwaitingFirstLaunchRetry`
- `Completed`

Observacao:
se o produto decidir manter limite e vencimento obrigatorios, incluir tambem:
- `AwaitingCardLimit`
- `AwaitingCardDueDay`

## Gaps de Orquestracao entre Agent e Onboarding

### Continuacao prioritaria

Hoje o `IntentRouter` prioriza `onboarding.Continue()` antes do parser normal. Isso e bom e deve ser mantido.

Necessidade:
- preservar essa prioridade
- permitir que o onboarding decida quando delegar ao parser de lancamentos
- evitar que o parser generico “roube” mensagens que ainda pertencem ao onboarding

### Primeiro lancamento

Necessidade:
- quando a sessao estiver em `AwaitingFirstLaunch`, a mensagem deve ser tratada por um fluxo coordenado
- o onboarding deve:
  - tentar interpretar a mensagem como lancamento
  - delegar ao caso de uso correto do agent
  - so marcar onboarding completo se a persistencia realmente acontecer
  - se falhar, responder de forma honesta e pedir nova tentativa

### Sem falso positivo

Regras obrigatorias:
- nunca celebrar antes da confirmacao de persistencia
- nunca marcar onboarding concluido antes do primeiro lancamento salvo
- nunca responder como se metodologia/objetivo/orcamento estivessem completos se a sessao nao possuir esses dados
- nunca inferir percentuais sem calculo sobre os valores coletados
- nunca inferir objetivo principal sem captura explicita

## Gaps de Persistencia e Eventos

### Persistencia

Necessario revisar:
- schema persistido da sessao de onboarding
- serializacao da payload
- compatibilidade com sessoes antigas
- estrategia de reset de sessao ativa versus migracao

### Eventos de dominio

Eventos minimos adicionais recomendados:
- `onboarding.methodology_presented`
- `onboarding.primary_goal_registered`
- `onboarding.card_count_registered`
- `onboarding.budget_allocation_registered`
- `onboarding.budget_summary_confirmed`
- `onboarding.first_launch_started`
- `onboarding.first_launch_completed`
- `onboarding.completed_v1`

Se o sistema nao precisar desses eventos agora, ao menos manter:
- consistencia entre sessao e marcos internos
- um evento final que represente de fato o onboarding V1 completo

## Gaps de Prompt e UX Conversacional

### Prompt de onboarding dedicado

O onboarding V1 nao deve depender so da persona ampla do agent.

Necessario:
- prompt dedicado ou respostas deterministicas do workflow para o onboarding
- copy consistente com o V1
- motivo da etapa atual explicitado
- tom acolhedor, didatico e motivador
- emojis limitados ao conjunto aprovado

### Copy padronizada

Padronizar:
- boas-vindas
- explicacao das 5 categorias
- perguntas por etapa
- mensagens de erro de input
- resumo final
- convite ao primeiro lancamento
- celebracao
- encerramento

## Gaps de Testes

### Testes unitarios do workflow

Adicionar cobertura para:
- entrada feliz completa do onboarding V1
- respostas invalidas em cada etapa
- retomada de sessao em qualquer subetapa
- fluxo com zero cartoes
- fluxo com multiplos cartoes
- distribuicao menor que a renda
- distribuicao igual a renda
- distribuicao acima da renda
- fluxo de ajuste de categorias
- primeiro lancamento invalido
- primeiro lancamento valido
- conclusao apenas apos persistencia do primeiro lancamento

### Testes de use case

Adicionar cobertura para:
- criacao de nova sessao V1
- resume em subetapa
- reset controlado
- publicacao de eventos adicionais
- coordenacao onboarding -> agent no primeiro lancamento

### Testes de integracao

Adicionar testes de integracao para:
- WhatsApp iniciando onboarding V1
- usuario percorrendo todas as etapas
- persistencia de sessao entre mensagens
- resumo final correto
- primeiro lancamento persistido
- celebracao emitida apos persistencia
- ausencia de escrita indevida em caso de erro

### Testes de regressao do agent

Garantir que:
- intents normais continuam funcionando fora do onboarding
- onboarding em andamento intercepta corretamente mensagens
- usuario ja ativo fora do onboarding nao cai em fluxo incorreto
- `configure_budget` nao reduza o onboarding V1 para o fluxo antigo de renda + split

## Implementacao Recomendada por Ordem

1. Revisar e congelar a spec do onboarding V1 real.
2. Decidir se cartao no onboarding exige apenas fechamento ou tambem limite e vencimento.
3. Expandir `OnboardingState` e payload persistida.
4. Reescrever `OnboardingWorkflow` para o roteiro V1 completo.
5. Ajustar `StartBudgetConfiguration` para iniciar em boas-vindas, nao em renda.
6. Implementar coleta e persistencia de objetivo principal.
7. Implementar fluxo de cartoes aderente a spec final.
8. Implementar coleta de orcamento por categoria com calculos e validacoes.
9. Implementar resumo final com confirmar/ajustar.
10. Implementar transicao para primeiro lancamento.
11. Integrar persistencia real do primeiro lancamento com conclusao do onboarding.
12. Padronizar copy do onboarding.
13. Cobrir com testes unitarios, de integracao e E2E conversacional.
14. So depois considerar prontidao para `main`.

## Criterios de Pronto para Main

O onboarding V1 so pode ser considerado pronto para `main` quando todos os itens abaixo forem verdadeiros:

- o fluxo comeca com boas-vindas, nao com pergunta de renda
- a metodologia das 5 categorias e apresentada individualmente
- o objetivo principal e coletado e reapresentado no resumo
- a renda e coletada e validada
- o fluxo de cartoes segue a spec final aprovada
- o orcamento e coletado por categoria em valor absoluto
- soma e percentuais sao calculados pelo sistema
- excedente e saldo restante sao tratados corretamente
- o usuario consegue ajustar e reconfirmar
- o resumo final mostra renda, categorias, cartoes e objetivo
- o onboarding so avanca para conclusao apos primeiro lancamento persistido
- ha celebracao explicita apos sucesso do primeiro lancamento
- existem testes cobrindo o roteiro feliz e os principais desvios
- nao ha resposta que sugira sucesso quando o sistema nao persistiu os dados

## Assuncoes e Defaults

- Assuncao atual: o objetivo e aderir ao `MeControla_Onboarding_SystemPrompt_V1.md` como fonte do comportamento desejado.
- Assuncao atual: `internal/agent` continuara integrado ao `internal/onboarding`, nao havera um terceiro modulo novo.
- Default recomendado: manter a priorizacao de `onboarding.Continue()` antes do parser geral do agent.
- Default recomendado: concluir onboarding apenas apos o primeiro lancamento salvo com sucesso.
- Default recomendado: se houver conflito entre V1 e o dominio real de cartoes, atualizar a spec antes de implementar para evitar retrabalho.
