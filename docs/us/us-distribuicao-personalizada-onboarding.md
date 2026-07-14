# US-001: Distribuição personalizada do orçamento no onboarding com validação de soma, zeros e valores por extenso

## Declaração
Como usuário em onboarding no WhatsApp que não aceita a sugestão automática de distribuição, quero personalizar quanto vai para cada uma das 5 categorias com orientação clara quando a soma não fecha, para ativar um orçamento que reflete exatamente as minhas prioridades sem travar no fluxo.

## Contexto
- Problema: no passo de distribuição do onboarding (`reviewAwaitDistribution`), responder apenas "não" não abre um modo de personalização — a resposta é forçada no enum fechado `{confirm, percent, reais}` e, sem números, colapsa para `confirm`, aplicando silenciosamente a distribuição padrão (`internal/agents/application/workflows/onboarding_workflow.go:250-271`). Além disso, quando a soma erra, as mensagens de erro só dizem que precisa somar 100% ou o orçamento, sem apontar o quanto passou/faltou nem orientar a redistribuição (`onboarding_workflow.go:283-303`), e o prompt de extração da distribuição não tem exemplos de conversão por extenso (`onboarding_workflow.go:596-601`), diferente dos prompts de meta e orçamento mensal que já suportam "mil"/"400 mil"/"1,5 milhão" (`onboarding_workflow.go:605-613,621-629`).
- Resultado esperado: no passo de distribuição, uma recusa/intenção de personalizar abre um modo que pergunta o valor de cada categoria, reforça que o orçamento inteiro deve ser distribuído e explica a regra do ZERO; somas acima ou abaixo do total geram mensagem com o delta exato na unidade que o usuário usou e pedido de redistribuição; categorias zeradas são aceitas como intencionais com um aviso único antes do resumo; e valores por extenso, monetários e percentuais são interpretados por categoria — tudo isso incrementando o fluxo atual, sem regressão.
- Fonte: solicitação do usuário (mensagem com os 5 cenários) e confronto com a base de código em `internal/agents/application/workflows/onboarding_workflow.go` e `internal/agents/application/workflows/onboarding_workflow_test.go`.

## Regras de Negócio
- RN-01 Personalizar por recusa: no sub-passo `reviewAwaitDistribution`, quando o usuário responder com recusa ou intenção de personalizar (ex.: "não", "nao", "quero personalizar", "prefiro escolher") sem informar valores, o agente entra em modo personalizar — pergunta quanto alocar em cada uma das 5 categorias, reforça que o orçamento inteiro precisa ser distribuído e explica que categorias sem sentido devem receber ZERO. O prompt do modo personalizar mostra o valor do orçamento mensal como âncora e lista as 5 categorias com seus rótulos (`categoryLabels` em `onboarding_workflow.go:51-57`), ex.: "Seu orçamento mensal é R$ 5.000,00. Me diga quanto vai para cada categoria (pode ser em R$ ou %); coloque 0 nas que não fazem sentido: 💰 Custo Fixo, 🎓 Conhecimento, 🎉 Prazeres, 🎯 Metas, 🏦 Liberdade Financeira." Nesse caso NÃO aplica a distribuição padrão (corrige o colapso atual para `confirm` em `onboarding_workflow.go:250-271`).
- RN-02 Ultrapassar o total: quando a soma dos valores exceder o total (100% no modo percentual ou o orçamento mensal no modo reais), o agente informa exatamente o quanto passou, reafirma o alvo (100% ou o valor do orçamento mensal), ecoa os valores que o usuário enviou e pede para redistribuir; não ativa nem avança, permanecendo no passo de distribuição. Ex.: "Você somou 110% — passou 10%. O total precisa fechar em 100%. Redistribua, por favor."
- RN-03 Não atingir o total: quando a soma ficar abaixo do total, o agente informa exatamente quanto falta, reafirma o alvo (100% ou o valor do orçamento mensal), ecoa os valores enviados e orienta a redistribuição; não ativa nem avança. Ex.: "Você somou R$ 2.500,00 — faltam R$ 500,00 para fechar seu orçamento de R$ 3.000,00. Pode redistribuir?"
- RN-04 Unidade do delta: o delta (quanto passou ou faltou) é expresso na mesma unidade que o usuário usou — em porcentagem quando ele enviou percentuais e em reais (formatado via `money.FromCents(...).BRL()`, `internal/platform/money/money.go:60`) quando ele enviou valores em reais.
- RN-05 Reprompt com contexto do usuário: as mensagens de RN-01, RN-02 e RN-03 ecoam os valores que o próprio usuário tentou (quando houver) e NÃO reexibem a sugestão padrão dentro do modo personalizar, para não confundir; na recusa pura (RN-01) a mensagem lista as 5 categorias pedindo um valor para cada uma e a regra do ZERO.
- RN-06 Categoria zerada aceita com aviso no resumo: valor 0 em uma categoria é aceito como intencional e persiste com 0 basis points; o aviso é entregue como uma linha anexada ao próprio resumo de confirmação (sem turno extra), nomeando de forma explícita quais categorias ficarão zeradas (ex.: "⚠️ Você deixou 🎓 Conhecimento e 🎉 Prazeres zeradas — confirme se está certo."); no resumo essas categorias aparecem como R$ 0,00 (0%) (`renderAllocationLines` em `onboarding_workflow.go:412-420`, que já lista sempre as 5 categorias). O aviso só aparece quando existir ao menos uma categoria zerada.
- RN-12 Tolerância de arredondamento: quando a soma ficar a uma diferença mínima do total por arredondamento (até 0,5% no modo percentual ou até R$ 0,05 no modo reais), a distribuição é aceita e o resto é absorvido na maior categoria, reaproveitando a lógica de maior-resto já existente (`centsToBasisPoints` em `onboarding_workflow.go:318-344`), garantindo que a soma dos basis points feche em 10000. Diferenças acima da tolerância continuam caindo em RN-02/RN-03 (passou/faltou).
- RN-13 Unidades misturadas: quando o usuário misturar unidades na mesma resposta (ex.: "custo fixo 40%, prazeres R$ 500"), o agente trata como ambíguo, ecoa o que entendeu e pede que o usuário use uma única unidade (apenas % ou apenas R$) para todas as categorias, sem ativar nem avançar.
- RN-14 Copy anuncia a opção de personalizar: o prompt de distribuição (`methodologyPrompt` em `onboarding_workflow.go:649-655`, inclusive quando reaberto a partir do "não" no resumo) passa a anunciar explicitamente as três opções, ex.: "Aceita esta sugestão? Responda 'sim' para confirmar, envie novos valores (R$ ou %), ou responda 'não' para personalizar categoria por categoria." O texto "Aceita esta sugestão" é mantido na nova copy para preservar o teste de reabertura (`onboarding_workflow_test.go:1386`, RN-09), e a novidade é apenas o anúncio explícito do caminho "não → personalizar".
- RN-07 Aceitar por extenso, monetário e percentual: no passo de distribuição, valores por extenso ("mil reais", "quinhentos"), monetários ("R$ 1.000,00", "1000") e percentuais ("40%", "40") devem ser interpretados corretamente por categoria, alinhando o prompt de extração da distribuição (`onboarding_workflow.go:596-601`) ao padrão de conversão já usado nos prompts de meta e orçamento mensal (`onboarding_workflow.go:605-613,621-629`).
- RN-08 Distribuição continua exigindo somar o total exato: a validação canônica permanece — percentuais devem somar 100% e valores em reais devem somar o orçamento mensal; a soma dos basis points resultante deve fechar em 10000 (`DecideDistribution` em `onboarding_workflow.go:219-240`). A mudança é apenas de orientação/mensageria e do modo personalizar, não do invariante de fechamento.
- RN-09 Sem regressão do fluxo atual: "sim"/aceite sem valores aplica a distribuição padrão e avança para o resumo (`onboarding_workflow.go:267-271`; teste `onboarding_workflow_test.go:1247`); valores em reais que somam o orçamento e percentuais que somam 100% continuam aceitos e convertidos (`onboarding_workflow.go:272-312`; teste `onboarding_workflow_test.go:1291`); "não" no passo de confirmação do resumo (`reviewAwaitConfirm`) continua reabrindo a distribuição exibindo a sugestão padrão (`onboarding_workflow.go:1046-1055`; teste `onboarding_workflow_test.go:1386`); soma inválida continua re-suspendendo no mesmo sub-estado sem ativação parcial (teste `onboarding_workflow_test.go:1331`).
- RN-10 Invariantes de plataforma (obrigatórias): estados de espera permanecem tipos fechados (DMMF state-as-type; R-AGENT-WF-001.3) — `reviewAwaitKind` (`onboarding_workflow.go:132-154`) e `allocationInputKind` (`onboarding_workflow.go:156-179`) continuam enums fechados e qualquer novo estado (modo personalizar) deve ser constante tipada, nunca string livre; regra de decisão pura vive em funções `Decide*` puras, sem IO nem `context.Context`, e o passo só orquestra; zero comentários em Go de produção (R-ADAPTER-001.1); LLM apenas nas call-sites sancionadas via `agent.Agent.Execute` (`onboarding_workflow.go:985-991`), nunca no kernel de workflow.
- RN-11 Persistência antes de perguntar: toda pergunta de personalização/redistribuição re-suspende o workflow durável salvando o estado no Snapshot antes de responder ao usuário (R-AGENT-WF-001.7), preservando o padrão `suspendStep` já existente (`onboarding_workflow.go:725-731`).

## Critérios de Aceite
```gherkin
Cenário: Recusa abre modo personalizar (fluxo feliz)
  Dado que estou no passo de distribuição do onboarding e recebi a sugestão automática que anuncia responder "não" para personalizar
  Quando respondo apenas "não"
  Então o agente mostra o valor do meu orçamento mensal como âncora
  E pergunta quanto quero alocar em cada uma das 5 categorias, com seus rótulos
  E reforça que preciso distribuir o meu orçamento inteiro
  E explica que categorias sem sentido devem receber ZERO
  E o workflow permanece suspenso aguardando os valores, sem aplicar a distribuição padrão

Cenário: Valores em reais que somam o orçamento são aceitos (fluxo feliz)
  Dado orçamento mensal de R$ 13.500,00 no passo de distribuição
  Quando envio valores em reais cuja soma é exatamente R$ 13.500,00
  Então o agente aceita, converte os valores para basis points
  E avança para o resumo de confirmação

Cenário: Soma ultrapassa o total em percentual (fluxo alternativo)
  Dado que estou no passo de distribuição
  Quando envio percentuais que somam 110%
  Então o agente informa que passei 10% do orçamento
  E reafirma que o total precisa fechar em 100%
  E ecoa os valores que enviei
  E pede para eu redistribuir
  E não ativa nem avança, permanecendo no passo de distribuição

Cenário: Soma não atinge o total em reais (fluxo alternativo)
  Dado orçamento mensal de R$ 3.000,00 no passo de distribuição
  Quando envio valores em reais que somam R$ 2.500,00
  Então o agente informa que faltam R$ 500,00
  E reafirma que o total precisa fechar no orçamento de R$ 3.000,00
  E ecoa os valores que enviei
  E orienta a redistribuição
  E não ativa nem avança

Cenário: Categoria zerada é aceita com aviso único (fluxo alternativo)
  Dado que estou distribuindo o orçamento
  Quando envio valores válidos que somam o total com uma ou mais categorias em 0
  Então o agente aceita os zeros como intencionais
  E avisa uma única vez quais categorias ficarão zeradas
  E segue para o resumo, no qual as categorias zeradas aparecem como R$ 0,00 (0%)

Cenário: Valores por extenso são interpretados por categoria (fluxo alternativo)
  Dado orçamento mensal de R$ 5.000,00 no passo de distribuição
  Quando escrevo os valores por extenso, como "custo fixo dois mil, conhecimento quinhentos, prazeres quinhentos, metas mil, liberdade mil"
  Então o agente interpreta corretamente o valor de cada categoria
  E, como somam o orçamento, avança para o resumo de confirmação

Cenário: Diferença mínima por arredondamento é absorvida (fluxo alternativo)
  Dado que estou no passo de distribuição
  Quando envio percentuais 33,3 + 33,3 + 33,4 + 0 + 0 que arredondam para 99%
  Então o agente aceita dentro da tolerância de arredondamento
  E absorve o resto na maior categoria de forma que a soma feche em 100%
  E avança para o resumo de confirmação

Cenário: Unidades misturadas geram re-pergunta (fluxo alternativo)
  Dado que estou no passo de distribuição
  Quando respondo misturando unidades, como "custo fixo 40%, prazeres R$ 500"
  Então o agente ecoa o que entendeu
  E pede que eu use uma única unidade, apenas % ou apenas R$, para todas as categorias
  E não ativa nem avança

Cenário: Recusa repetida dentro do modo personalizar (erro/limite)
  Dado que estou no modo personalizar e ainda não enviei uma distribuição utilizável
  Quando respondo novamente sem valores utilizáveis, como "não sei"
  Então o agente repete a orientação de personalização, perguntando os valores por categoria e a regra do ZERO
  E permanece suspenso no passo de distribuição, sem ativar

Cenário: Aceite da sugestão padrão permanece funcionando (não-regressão)
  Dado a sugestão automática exibida no passo de distribuição
  Quando respondo "sim" sem informar valores
  Então o agente aplica a distribuição padrão
  E avança para o resumo de confirmação

Cenário: "não" no resumo reabre a distribuição com a sugestão padrão (não-regressão)
  Dado que estou no passo de confirmação do resumo
  Quando respondo "não"
  Então o agente reabre a distribuição exibindo a sugestão padrão com o texto "Aceita esta sugestão"
  E o prompt reaberto anuncia a opção de responder "não" para personalizar
  E não ocorre ativação parcial do orçamento
```

## Dados e Permissões
- Dados obrigatórios: `MonthlyBudgetCents` já definido e maior que zero no passo anterior (`DecideMonthlyBudgetCents` em `onboarding_workflow.go:212-217`); os 5 slugs canônicos de categoria (`canonicalSlugs` em `onboarding_workflow.go:43-49`); o valor por categoria informado pelo usuário (em %, R$ ou por extenso); `OnboardingState.Allocations` como mapa slug -> basis points (`onboarding_workflow.go:191`).
- Perfis/permissões: fluxo executado sob o principal do próprio usuário do WhatsApp identificado por `UserID`/`PeerID` no estado do onboarding (`onboarding_workflow.go:181-195`, `applyDraftBudget` usa `uuid.Parse(state.UserID)` em `onboarding_workflow.go:913-916`); não há perfil administrativo nem acesso a dados de outro usuário.

## Dependências
- `interfaces.BudgetPlanner.SuggestAllocation` para gerar o preview/resumo das alocações (`internal/agents/application/interfaces/budget_planner.go:17`; tipos `AllocationBP`/`AllocationCents` em `internal/agents/application/interfaces/types.go:251-260`).
- `agent.Agent.Execute` com `llm.Schema` (Structured Output estrito) para extrair a intenção e os valores por categoria (`onboarding_workflow.go:985-991`); o schema `allocationInputSchema` (`onboarding_workflow.go:499-511`) precisa acomodar o novo sinal de personalizar como estado fechado.
- Kernel de workflow durável para suspend/resume por merge-patch (`workflow.StepStatusSuspended`, `suspendStep` em `onboarding_workflow.go:725-731`); nenhuma alteração no kernel é necessária.
- `money.FromCents(...).BRL()` para formatar o delta em reais (`internal/platform/money/money.go:60`).
- `applyDraftBudget` e `BuildActivationStep` permanecem inalterados e continuam sendo o ponto de criação/ativação do orçamento (`onboarding_workflow.go:913-952,1058-1072`).

## Fora de Escopo
- Alterar o passo de confirmação do resumo (`reviewAwaitConfirm`) além de manter o comportamento atual de reabrir a distribuição.
- Mudar a distribuição padrão (`defaultDistributionBP` em `onboarding_workflow.go:59-65`) ou a estrutura das 5 categorias.
- Edição de orçamento já ativo fora do onboarding (fluxo conversacional de editar orçamento é outra história de backlog).
- Alterar os passos de meta, orçamento mensal, cartões, recorrência, ativação e conclusão.
- Adicionar novas categorias ou permitir slugs fora dos 5 canônicos.
- Persistir apelidos/dicionário pessoal de categorias (item separado do backlog).

## Evidências
- Entrada: mensagem do usuário com os 5 cenários (recusa "não" = personalizar; ultrapassar o total; não atingir o total; zero como categoria zerada; aceitar valor por extenso/monetário/percentual), com foco em `internal/agents/application/workflows/onboarding_workflow.go`, exigência de zero regressão e uso das skills go-implementation, domain-modeling-production, mastra e design-patterns-mandatory. Decisões confirmadas por perguntas de múltipla escolha (três rodadas): reprompt ecoa os valores do usuário + delta (sem reexibir a sugestão padrão); gatilho de personalizar aceita qualquer recusa/intenção; delta na unidade usada pelo usuário; categoria zerada aceita com aviso único anexado ao resumo (sem turno extra); tolerância de arredondamento com absorção do resto na maior categoria; unidades misturadas geram re-pergunta pedindo unidade única; o prompt de distribuição anuncia explicitamente a opção "não → personalizar"; o modo personalizar mostra o valor do orçamento mensal como âncora com as 5 categorias; a mensagem de passou/faltou reafirma o alvo (100% ou o orçamento) além do delta.
- Base de código: passo de distribuição e sub-estados em `onboarding_workflow.go:954-1056`; enums fechados `reviewAwaitKind` (`:132-154`) e `allocationInputKind` (`:156-179`); classificação e override de tipo em `DecideAllocationKind` (`:250-263`); regras de soma e conversão em `DecideAllocationsBP` (`:265-316`) e `DecideDistribution` (`:219-240`); mensagens de erro exatas de percentual (`:283-285`) e reais (`:301-303`); prompt de extração da distribuição sem exemplos por extenso (`:596-601`) versus prompts de meta/orçamento mensal com conversão por extenso (`:605-613,621-629`); render das linhas do resumo sempre com as 5 categorias (`:412-420`); prompts `methodologyPrompt`/`methodologyReprompt`/`summaryPrompt` (`:649-670`); testes de baseline de regressão em `onboarding_workflow_test.go:1247,1291,1331,1386`.
- Inferências: para atender RN-01 sem violar RN-10, o modo personalizar deve ser modelado como estado fechado (nova constante tipada em `reviewAwaitKind` ou em `allocationInputKind`, mais um valor "personalize" no enum do schema), decidido na techspec; a distinção "quanto passou/faltou" pode ser calculada por uma função `Decide*` pura que recebe os valores e o total, mantendo o passo como orquestrador; o reaper de onboarding (`onboarding_workflow.go:1210-1212`) cobre eventual abandono no modo personalizar sem necessidade de cap de tentativas.
- Não evidenciado: não existe hoje sinal/estado de "personalizar" ou "recusar" no passo de distribuição — a busca nos enums `reviewAwaitKind` (`onboarding_workflow.go:132-154`) e `allocationInputKind` (`onboarding_workflow.go:156-179`) mostra apenas `{distribution, confirm}` e `{confirm, percent, reais}`, sem caminho de recusa; e o `allocationInputSystemPrompt` (`onboarding_workflow.go:596-601`) não contém nenhum exemplo de conversão por extenso.

## Notas de Validação
- Cobertura de cenários: 2 de fluxo feliz (recusa abre personalizar; reais somando o orçamento), 6 alternativos (ultrapassar; faltar; categoria zerada com aviso; valor por extenso; tolerância de arredondamento; unidades misturadas) e 3 de não-regressão/limite (recusa repetida no modo personalizar; aceite "sim" da sugestão padrão; reabertura da distribuição a partir do "não" no resumo).
- Validação técnica exigida pelo repositório: testes de unidade whitebox com testify/suite e mockery para o passo de distribuição (padrão de `onboarding_workflow_test.go`), preservando os testes de baseline citados em RN-09; e validação real com LLM (RUN_REAL_LLM=1 com credenciais OPENROUTER) para os cenários de classificação de intenção e conversão de valores, já que mocks não exercitam a extração real — conforme a política de validação real-LLM do projeto.
- Gates obrigatórios antes de concluir a implementação: build, vet, test race e lint no módulo alterado; zero comentários em Go de produção; sem regra de negócio/branching de domínio no kernel; estados de espera como tipos fechados.
- Pergunta pendente que altere escopo, aceite ou prioridade: não há; as decisões abertas foram resolvidas nas perguntas de múltipla escolha registradas em Evidências/Entrada.
