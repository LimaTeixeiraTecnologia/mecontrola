# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

Feature: Distribuição personalizada do orçamento no onboarding
Slug: distribuicao-personalizada-onboarding
Fonte primária: `docs/us/us-distribuicao-personalizada-onboarding.md` (US-001)
Área de código-alvo: `internal/agents/application/workflows/onboarding_workflow.go` (passo de distribuição / `reviewAwaitDistribution`)

## Visão Geral

No onboarding conversacional do MeControla (WhatsApp), depois de informar o orçamento mensal, o usuário recebe uma sugestão automática de como distribuir o dinheiro entre 5 categorias (Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira). Hoje esse passo aceita bem "sim" (aplica a sugestão padrão) e valores que fecham exatamente 100% ou o orçamento, mas falha em três frentes que travam ou frustram quem quer personalizar: responder apenas "não" não abre um caminho de personalização (a resposta colapsa para a sugestão padrão), as mensagens de soma errada não dizem o quanto passou/faltou nem orientam a redistribuir, e valores por extenso ("mil reais") não são interpretados de forma confiável por categoria.

Esta funcionalidade torna o passo de distribuição realmente conversável: uma recusa vira um convite explícito para o usuário dizer quanto quer em cada categoria, com orientação clara quando a soma não fecha, aceitação de valores por extenso/monetário/percentual, e tratamento intencional de categorias zeradas. É valiosa porque o passo de distribuição é o ponto onde o usuário conecta o próprio dinheiro às prioridades dele — se ele trava aqui, não ativa o orçamento e o onboarding não se completa.

O problema, o resultado esperado e todas as regras de comportamento estão detalhados e confrontados com a base de código na US-001 (14 regras de negócio, 11 cenários de aceite Gherkin, evidências com arquivo:linha). Este PRD consolida objetivos, escopo, restrições e requisitos funcionais numerados para rastreabilidade downstream.

## Objetivos

- Eliminar o beco sem saída atual: responder "não" no passo de distribuição passa a abrir personalização em vez de aplicar silenciosamente a sugestão padrão.
- Tornar auto-corrigível a distribuição com soma errada: o usuário sempre sabe exatamente quanto passou ou faltou, na unidade que ele usou, e como redistribuir.
- Aceitar a forma natural de escrever valores no WhatsApp: por extenso ("mil reais"), monetário ("R$ 1.000,00") e percentual ("40%").
- Preservar 100% do fluxo atual que já funciona (sem regressão), ancorado nos testes de baseline existentes.

Critérios de sucesso mensuráveis:

- CS-01: Taxa de onboardings que fecham o passo de distribuição e ativam o orçamento (conversão do passo `budget_review` → `activation`) aumenta em relação ao baseline atual.
- CS-02: Redução do número de runs de onboarding expirados pelo reaper (`OnboardingStaleAfter`, `onboarding_workflow.go:39`) enquanto suspensos no passo de distribuição.
- CS-03: Taxa de distribuição personalizada aceita até a segunda tentativa do usuário (proporção de personalizações que fecham sem mais de um reprompt de soma).
- CS-04: Zero regressão comprovada — a suíte de testes de baseline do passo de distribuição (`onboarding_workflow_test.go`) permanece verde, e os cenários golden real-LLM dos 5 comportamentos passam.

## Histórias de Usuário

Persona primária: usuário final do MeControla em onboarding pelo WhatsApp, distribuindo o próprio orçamento mensal entre as 5 categorias.

- US-001 (canônica, `docs/us/us-distribuicao-personalizada-onboarding.md`): "Como usuário em onboarding no WhatsApp que não aceita a sugestão automática de distribuição, quero personalizar quanto vai para cada uma das 5 categorias com orientação clara quando a soma não fecha, para ativar um orçamento que reflete exatamente as minhas prioridades sem travar no fluxo."

Fluxos e casos de borda cobertos pela US (base para os RFs abaixo): recusa que abre personalização; soma acima do total; soma abaixo do total; categoria zerada intencional; valores por extenso/monetário/percentual; diferença mínima por arredondamento; unidades misturadas; e os caminhos de não-regressão (aceite "sim" da sugestão padrão, valores válidos em reais/percentual, "não" no resumo reabrindo a distribuição, soma inválida re-suspendendo sem ativação parcial).

Persona secundária impactada por consistência: usuário que cria orçamento retroativo pelo fluxo `budget_creation_workflow` — herda as melhorias de mensageria, tolerância e valores por extenso por compartilhar o mesmo núcleo de decisão (ver RF-15), sem receber o sub-modo de personalização.

## Funcionalidades Core

- Modo personalizar acionado por recusa: quando o usuário recusa ou sinaliza intenção de personalizar, o agente pergunta explicitamente quanto vai para cada categoria, reforça que o orçamento inteiro deve ser distribuído, explica a regra do ZERO e ancora no valor do orçamento mensal. Importante porque transforma um "não" em progresso em vez de aplicar a sugestão padrão à revelia do usuário. Em alto nível: uma recusa no passo de distribuição leva a um sub-estado de espera dedicado que coleta os valores por categoria.
- Orientação de redistribuição com delta explícito: toda soma que não fecha o total gera uma mensagem que diz o quanto passou ou faltou, reafirma o alvo (100% ou o orçamento), ecoa os valores do próprio usuário e pede a redistribuição, sem ativar nada. Importante porque remove o principal ponto de travamento. Em alto nível: o cálculo do delta é determinístico e a resposta re-suspende o passo mantendo o sub-estado.
- Aceitação de valores em linguagem natural: valores por extenso, monetários e percentuais são interpretados por categoria, alinhando o passo de distribuição ao que já funciona nos passos de meta e orçamento mensal. Importante porque é como as pessoas escrevem no WhatsApp. Em alto nível: enriquecimento do prompt de extração estruturada usado pelo classificador.
- Categorias zeradas intencionais: valor 0 é aceito como decisão do usuário, com um aviso único no resumo nomeando o que ficará zerado. Importante para dar controle sem esconder consequências. Em alto nível: o zero persiste como alocação de valor zero e o aviso é anexado à mensagem de resumo.
- Robustez de soma: diferença mínima por arredondamento é absorvida na maior categoria (sem travar o usuário por centavos), e unidades misturadas na mesma resposta geram um pedido de padronização. Importante para não punir arredondamentos legítimos nem adivinhar unidade errada.

## Requisitos Funcionais

- RF-01: No passo de distribuição do onboarding, quando o usuário responder com recusa ou intenção de personalizar sem informar valores (ex.: "não", "nao", "quero personalizar", "prefiro escolher"), o sistema entra em modo personalizar — pergunta quanto alocar em cada uma das 5 categorias, reforça que o orçamento inteiro precisa ser distribuído e explica que categorias sem sentido devem receber ZERO — e NÃO aplica a distribuição padrão. (US RN-01, RN-05)
- RF-02: O prompt do passo de distribuição anuncia explicitamente as três opções, incluindo responder "não" para personalizar, mantendo o texto "Aceita esta sugestão" para não regredir a reabertura a partir do resumo. (US RN-14)
- RF-03: O prompt do modo personalizar exibe o valor do orçamento mensal como âncora e lista as 5 categorias com seus rótulos. (US RN-01)
- RF-04: Quando a soma dos valores exceder o total (100% no modo percentual ou o orçamento mensal no modo reais), o sistema informa exatamente o quanto passou, reafirma o alvo, ecoa os valores enviados pelo usuário e pede a redistribuição, sem ativar nem avançar. (US RN-02)
- RF-05: Quando a soma ficar abaixo do total, o sistema informa exatamente quanto falta, reafirma o alvo, ecoa os valores enviados e orienta a redistribuição, sem ativar nem avançar. (US RN-03)
- RF-06: O delta (quanto passou ou faltou) é expresso na mesma unidade que o usuário usou — porcentagem quando ele enviou percentuais, reais quando ele enviou valores monetários. (US RN-04)
- RF-07: Valor 0 em uma categoria é aceito como intencional e persiste com alocação zero; o sistema anexa ao resumo de confirmação um aviso único nomeando as categorias que ficarão zeradas; no resumo essas categorias aparecem como R$ 0,00 (0%). (US RN-06)
- RF-08: No passo de distribuição, valores por extenso ("mil reais", "quinhentos"), monetários ("R$ 1.000,00", "1000") e percentuais ("40%", "40") são interpretados corretamente por categoria. (US RN-07)
- RF-09: Quando a soma ficar a uma diferença mínima do total apenas por arredondamento (dentro de uma tolerância pequena definida na especificação técnica), a distribuição é aceita e o resto é absorvido na maior categoria, garantindo o fechamento exato do invariante; diferenças acima da tolerância caem em RF-04/RF-05. (US RN-12)
- RF-10: Quando o usuário misturar unidades na mesma resposta (percentual e reais juntos), o sistema trata como ambíguo, ecoa o que entendeu e pede que ele use uma única unidade para todas as categorias, sem ativar nem avançar. (US RN-13)
- RF-11: O invariante de fechamento é preservado — percentuais devem somar 100% e valores em reais devem somar o orçamento mensal, com a distribuição final fechando integralmente o orçamento. A mudança é de orientação/mensageria e do modo personalizar, não do invariante. (US RN-08)
- RF-12: Nenhum caminho atual regride — aceitar "sim" aplica a sugestão padrão e avança ao resumo; valores válidos em reais e percentuais continuam aceitos; "não" no passo de confirmação do resumo continua reabrindo a distribuição com a sugestão padrão; soma inválida continua re-suspendendo no mesmo sub-estado sem ativação parcial. (US RN-09)
- RF-13: Toda pergunta de personalização ou redistribuição persiste o estado de espera de forma durável antes de responder ao usuário; o estado é retomado no resume antes de qualquer interpretação da resposta. (US RN-11)
- RF-14: Os estados de espera do passo (incluindo o novo modo personalizar) são representados como tipos fechados enumerados, nunca como texto livre. (US RN-10)
- RF-15: As melhorias que residem no núcleo de decisão compartilhado (orientação de passou/faltou, tolerância de arredondamento e aceitação de valores por extenso/monetário/percentual) valem também para o fluxo `budget_creation_workflow` (criação de orçamento retroativo), que já reutiliza esse núcleo; o sub-modo "não → personalizar" e a copy de anúncio permanecem exclusivos do onboarding nesta entrega. Nenhuma função de decisão é duplicada para isolar os fluxos.
- RF-16: O sistema emite um sinal de observabilidade do resultado do passo de distribuição — um contador com rótulo de resultado fechado (por exemplo: personalizar acionado, sugestão padrão aceita, valores aceitos, acima do total, abaixo do total, unidades misturadas, arredondamento absorvido) — respeitando cardinalidade controlada, sem `user_id` nem `category_id` como rótulo.
- RF-17: A entrega é liberada diretamente, sem feature flag ou toggle de ambiente; a segurança do rollout vem da garantia de não-regressão por testes de baseline.

## Experiência do Usuário

- Persona e necessidade: usuário final no WhatsApp que quer que o orçamento reflita as prioridades dele; precisa de um passo de distribuição que aceite recusa, corrija somas erradas com clareza e entenda como ele escreve valores.
- Fluxo principal: o usuário vê a sugestão padrão com as três opções anunciadas (aceitar, enviar valores, ou personalizar); ao responder "não", recebe o convite de personalização ancorado no orçamento mensal e na lista das 5 categorias com a regra do ZERO; envia valores em R$, % ou por extenso; se a soma não fecha, recebe o delta exato e a orientação de redistribuir; ao fechar, vê o resumo (com aviso de categorias zeradas quando houver) e confirma para ativar.
- Interações e tom: linguagem em português do Brasil, conversacional, com emojis já usados nas 5 categorias; mensagens curtas e acionáveis que sempre dizem o próximo passo.
- Casos de borda tratados na experiência: recusa repetida no modo personalizar (repete a orientação, permanece suspenso), unidades misturadas (pede padronização), diferença de arredondamento (absorvida sem atrito), categorias zeradas (aceitas com aviso único).
- Acessibilidade: interface é texto no WhatsApp; a clareza é garantida por mensagens explícitas de delta, alvo e próximo passo, sem depender de formatação visual complexa.

## Restrições Técnicas de Alto Nível

- Skills obrigatórias de desenvolvimento (uso mandatório declarado pelo solicitante e pela governança do repositório): `go-implementation` (implementação, validação e gates Go), `domain-modeling-production` (modelagem de domínio, estados como tipos, decisões puras), `design-patterns-mandatory` (gate de seleção de padrão com justificativa) e `mastra` (substrato de agent/workflow/tool/memory da plataforma). A especificação técnica deve materializar essas skills, não apenas citá-las.
- Substrato de plataforma (R-AGENT-WF-001 e R-WF-KERNEL-001): o comportamento vive no consumidor `internal/agents` sobre os primitivos de `internal/platform/{agent,memory,workflow}`; nada de regra de negócio, SQL ou LLM no kernel de workflow; execução como Run auditável com estados fechados; estado de espera persistido no Snapshot antes de pedir input e retomado por merge-patch antes do parse.
- Modelagem de estado (DMMF state-as-type): os sub-estados do passo de distribuição, incluindo o novo modo personalizar, são tipos fechados enumerados; a regra de decisão (delta, classificação de unidade, absorção de arredondamento, validação de fechamento) vive em funções puras, sem IO nem contexto, testáveis sem mock.
- Núcleo compartilhado: o classificador de entrada e o decididor de alocação são compartilhados entre `onboarding_workflow` e `budget_creation_workflow`; qualquer mudança nesse núcleo é deliberadamente comum aos dois fluxos (RF-15) e não pode regredir nenhum dos dois — ambas as suítes de teste devem permanecer verdes.
- LLM e provider: interpretação de intenção e de valores ocorre apenas nas call-sites sancionadas do agent via estrutura de saída estrita (Structured Output), com OpenRouter como único provider; sem LLM no kernel nem dentro de decisões puras.
- Observabilidade e cardinalidade: o contador de resultado de distribuição (RF-16) usa apenas rótulos de baixa cardinalidade (resultado, e no máximo canal/workflow), proibido `user_id`/`category_id` como rótulo, herdando R-TXN-004 e R-AGENT-WF-001.5.
- Estilo Go obrigatório: zero comentários em Go de produção (R-ADAPTER-001.1), adaptadores finos, sem `init()`, sem `panic` em produção, agregação de erros com `errors.Join`/wrapping com `%w`.
- Idioma: toda a copy é em português do Brasil, coerente com os prompts e rótulos já existentes.
- Compatibilidade de dados: o resultado continua sendo uma alocação em basis points que fecha o orçamento (soma 10000); nenhuma mudança de contrato de persistência de orçamento é introduzida por este PRD.

## Fora de Escopo

- Levar o sub-modo "não → personalizar" e a copy de anúncio para o fluxo `budget_creation_workflow` (apenas as melhorias de núcleo compartilhado chegam lá nesta entrega — RF-15).
- Edição ou reconfiguração de orçamento já ativo fora do onboarding (fluxo conversacional de editar orçamento é iniciativa separada).
- Alterar a distribuição padrão sugerida ou a estrutura/semântica das 5 categorias.
- Adicionar novas categorias ou permitir categorias fora dos 5 slugs canônicos.
- Persistir apelidos ou dicionário pessoal de categorias.
- Alterar os demais passos do onboarding (meta, orçamento mensal, cartões, recorrência, ativação, conclusão) além do necessário para o passo de distribuição.
- Introduzir feature flag ou toggle de ambiente para esta mudança (RF-17).
- Definir o valor numérico exato da tolerância de arredondamento e o modelo concreto do sub-estado (identificadores, enum, funções) — decisões da especificação técnica.

## Suposições e Questões em Aberto

Suposições confirmadas com o solicitante (nenhuma questão material em aberto):

- Comportamento do passo de distribuição integralmente definido pela US-001, com dez decisões confirmadas em três rodadas de perguntas (conteúdo do reprompt ecoando valores do usuário; gatilho de personalizar por qualquer recusa/intenção; delta na unidade do usuário; categoria zerada aceita com aviso único no resumo; tolerância de arredondamento com absorção na maior categoria; unidades misturadas com pedido de unidade única; anúncio da opção de personalizar na copy; âncora do orçamento no modo personalizar; reafirmação do alvo nas mensagens de passou/faltou).
- Escopo do núcleo compartilhado, critérios de sucesso, sinal de observabilidade novo e rollout direto confirmados em rodada de perguntas específica deste PRD (opções recomendadas aceitas).

Itens que pertencem à especificação técnica (não são lacunas de produto): valor exato da tolerância de arredondamento; modelagem concreta do sub-estado do modo personalizar (tipo fechado e funções de decisão); nome e conjunto final de valores do rótulo do contador de resultado; atualização dos testes de baseline de `budget_creation_workflow` impactados pelo núcleo compartilhado.
