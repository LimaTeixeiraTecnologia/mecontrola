# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

**Slug:** `onboarding-categorias-orcamento-cartoes`
**Origem:** `docs/us/2026-07-10-us-onboarding-categorias-orcamento-cartoes.md` (US-001)
**Skills mandatórias declaradas para implementação:** `go-implementation`, `mastra`, `domain-modeling-production`, `design-patterns-mandatory` (decisão de pattern: **não aplicar padrão** — solução direta sobre o workflow durável existente).

## Visão Geral

O onboarding conversacional do MeControla no WhatsApp é a primeira experiência da cliente após a
ativação da assinatura. Hoje ele combina boas-vindas com a pergunta de objetivo, pergunta a
**renda mensal líquida**, cadastra **um único cartão antes** de apresentar as categorias e separa a
apresentação da metodologia da coleta do orçamento. Esse desenho gera três problemas de produto:
(1) a primeira mensagem sobrecarrega a cliente misturando saudação e pergunta; (2) pedir "renda
líquida" cria fricção e não reflete o que o produto planeja (o orçamento mensal que a pessoa quer
organizar); (3) o cadastro de um único cartão, posicionado cedo, impede quem tem mais de um cartão
e antecede a construção do plano.

Esta funcionalidade redesenha a **sequência** e a **linguagem** do onboarding durável, preservando o
substrato agentivo (`internal/platform/workflow`, Thread→Run, suspensão e retomada por merge-patch) e
as portas de domínio já existentes (`BudgetPlanner`, `CardManager`). A nova jornada é:

`boas-vindas isolada → meta/objetivo (valor opcional) → apresentação das 5 categorias + orçamento
mensal → distribuição → resumo → ativação → recorrência → cartões um por vez (loop) → conclusão`.

O objetivo é entregar um onboarding claro, sem lacunas de fluxo, sem falso sucesso em erros técnicos,
sem pedir renda líquida e com cadastro de múltiplos cartões, com o menor custo e a menor superfície
de mudança sobre o workflow atual.

## Objetivos

- Substituir a primeira mensagem combinada por uma **boas-vindas isolada** que apresenta o MeControla
  e suspende aguardando a resposta antes de perguntar a meta.
- Coletar a **meta/objetivo financeiro** no segundo passo, aceitando objetivo com ou sem valor
  monetário, sem bloquear o avanço quando não houver valor.
- Apresentar as **5 categorias oficiais** do orçamento no terceiro passo com o texto aprovado e
  avançar de imediato para a coleta do orçamento mensal, sem exigir confirmação "Faz sentido?".
- Eliminar completamente a expressão **"renda mensal líquida"** de prompts, erros, resumo,
  WorkingMemory e do modelo de estado, substituindo pela semântica de **orçamento mensal**.
- Cadastrar **um ou mais cartões, um por vez**, após a ativação do orçamento, repetindo a pergunta
  até a cliente recusar adicionar outro.
- Preservar durabilidade, estados fechados/parseáveis, suspensão/retomada por merge-patch e a
  ausência de falso sucesso em qualquer falha técnica de step.

### Métricas de sucesso

- **M-01 (fluxo):** 100% dos onboardings iniciados enviam a boas-vindas isolada como primeira
  mensagem, sem mencionar meta, orçamento, renda ou cartão no mesmo turno.
- **M-02 (linguagem):** 0 ocorrências de "renda líquida" / "renda mensal líquida" em prompts, erros,
  resumo, WorkingMemory e no modelo de estado renderizado à cliente.
- **M-03 (múltiplos cartões):** clientes conseguem cadastrar 2+ cartões em um único onboarding, um
  por vez, sem reabrir orçamento já ativado.
- **M-04 (sem falso sucesso):** toda falha técnica de step (listar/criar cartão, sugerir alocação,
  criar/ativar budget, gravar WorkingMemory) resulta em erro tipado rastreável e nenhuma afirmação de
  conclusão indevida.
- **M-05 (gate de extração real-LLM):** ratio de acerto ≥ 0,90 por categoria de extração estruturada
  do workflow (meta, valor da meta, orçamento mensal, distribuição, confirmações, cartão) em harness
  com LLM real, sem baixar a régua.

## Histórias de Usuário

- **Como** cliente nova do MeControla no WhatsApp, **quero** receber primeiro apenas uma mensagem de
  boas-vindas apresentando o produto, **para** entender onde estou antes de responder qualquer
  pergunta.
- **Como** cliente nova, **quero** informar minha meta financeira com ou sem valor, **para** não ser
  bloqueada caso eu ainda não tenha um número em mente.
- **Como** cliente nova, **quero** entender as 5 categorias antes de informar valores, **para** saber
  como o meu dinheiro será organizado.
- **Como** cliente nova, **quero** informar o **orçamento mensal** que desejo planejar (não minha
  renda líquida), **para** distribuir esse valor entre as categorias.
- **Como** cliente com mais de um cartão, **quero** cadastrar meus cartões um por vez ao final do
  onboarding, **para** registrar todos sem repetir o processo inteiro.
- **Como** cliente, **quero** que erros técnicos não me digam que algo foi concluído quando não foi,
  **para** confiar no que o sistema afirma.

## Funcionalidades Core

1. **Boas-vindas isolada (Passo 1).** Mensagem única de apresentação do MeControla, sem qualquer
   pergunta de meta, orçamento, renda ou cartão. O workflow suspende aguardando a resposta para
   avançar ao passo de meta.

2. **Coleta de meta com valor opcional (Passo 2).** Pergunta o objetivo financeiro; aceita resposta
   com valor monetário explícito e resposta sem valor. Mantém o comportamento existente de perguntar
   o valor uma vez de forma opcional, sem bloquear quando a cliente não informa.

3. **Apresentação de categorias + orçamento mensal (Passo 3).** Em **uma única mensagem** (D-01),
   apresenta as 5 categorias com o texto aprovado e, na sequência, pergunta o orçamento mensal. O
   workflow suspende **uma única vez** aguardando o valor do orçamento — sem exigir confirmação da
   apresentação.

4. **Distribuição do orçamento (Passo 4).** Sugere a distribuição padrão sobre o orçamento mensal e
   aceita três modos de resposta (D-03): aceitar a sugestão, informar valores em **reais** (soma
   validada contra o orçamento mensal) ou informar **percentuais** (soma validada em 100%). Valores
   válidos são convertidos para basis points.

5. **Resumo e ativação (Passo 5–6).** Apresenta o resumo (objetivo, orçamento mensal e distribuição
   por categoria — **sem** cartão e **sem** "renda mensal líquida") e ativa o orçamento **somente
   após confirmação explícita** da cliente.

6. **Recorrência (Passo 7).** Logo após a ativação e **antes** dos cartões (D-02), pergunta se a
   cliente deseja repetir o orçamento automaticamente pelos próximos 12 meses.

7. **Cadastro de cartões em loop (Passo 8).** Pergunta se deseja adicionar um cartão; ao informar
   apelido, banco e dia de vencimento válidos, cria o cartão e pergunta se deseja adicionar outro;
   repete até a cliente recusar. Cartão incompleto pede apenas os dados faltantes/ inválidos, não
   cria cartão parcial e não desfaz o orçamento já ativado.

8. **Conclusão (Passo 9).** Marca a etapa de cartões concluída, grava a WorkingMemory com o objetivo
   financeiro e envia a mensagem final de próximos passos, sem reabrir distribuição, resumo ou
   ativação.

## Requisitos Funcionais

### Passo 1 — Boas-vindas isolada
- RF-01: O primeiro passo do onboarding DEVE enviar apenas a mensagem de boas-vindas e apresentação
  do MeControla, sem perguntar meta, objetivo, orçamento, renda ou cartão no mesmo turno. A cópia da
  boas-vindas é redigida na implementação reaproveitando o tom/emojis já estabelecidos, sem a pergunta
  de meta (D-10).
- RF-02: Após enviar a boas-vindas, o workflow DEVE ficar suspenso aguardando a resposta da cliente
  para avançar ao passo de meta.
- RF-03: A resposta da cliente à boas-vindas DEVE ser tratada **apenas como gatilho de avanço** para o
  passo de meta (D-07); o texto NÃO é interpretado como objetivo, e a meta é sempre perguntada no
  passo seguinte.
- RF-03a: O gatilho de início do onboarding permanece o existente (evento de ativação/`subscription_bound`
  ou primeiro inbound WhatsApp); esta feature não altera o gatilho, apenas o conteúdo do primeiro passo.

### Passo 2 — Meta com valor opcional
- RF-04: O segundo passo DEVE perguntar qual é a meta/objetivo financeiro da cliente.
- RF-05: A coleta de meta DEVE aceitar resposta com valor monetário explícito, persistindo o valor da
  meta em centavos no estado.
- RF-06: A coleta de meta DEVE aceitar resposta sem valor monetário, sem bloquear o avanço.
- RF-07: Quando a cliente não fornecer valor junto ao objetivo, o sistema DEVE perguntar o valor uma
  única vez de forma opcional (D-08, preserva a feature "valor opcional da meta"), e a recusa/ausência
  de valor DEVE seguir o fluxo sem bloqueio.
- RF-08: Objetivo vazio/não identificável DEVE gerar reprompt específico de objetivo e manter o
  workflow suspenso no passo de meta.

### Passo 3 — Categorias + orçamento mensal
- RF-09: Concluído o passo de meta, o sistema DEVE apresentar as 5 categorias do orçamento.
- RF-10: A apresentação DEVE usar exatamente as categorias: **Custo Fixo, Conhecimento, Prazeres,
  Metas e Liberdade Financeira**, mapeadas aos slugs canônicos de `budgets/domain/valueobjects.RootSlug`.
- RF-11: A mensagem de apresentação DEVE preservar o conteúdo de negócio aprovado: "Antes de montar
  seu planejamento, deixa eu te mostrar como organizamos o dinheiro por aqui. Tudo vive em apenas 5
  categorias: Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira."
- RF-12: A apresentação das categorias e a pergunta de orçamento mensal DEVEM ser entregues em **uma
  única mensagem combinada** (D-01), com um **único** ponto de suspensão aguardando o valor do
  orçamento — sem exigir confirmação "Faz sentido?" da apresentação.
- RF-13: O sistema NÃO DEVE perguntar renda mensal líquida em nenhum ponto do onboarding.
- RF-14: A pergunta de valor DEVE usar a expressão "orçamento mensal" / "valor mensal planejado" e o
  valor informado DEVE ser persistido no estado como **total planejado** do onboarding.
- RF-15: Valor de orçamento mensal sem número positivo identificável DEVE gerar reprompt específico
  com exemplo em reais, manter o workflow suspenso na etapa de orçamento e NÃO criar budget, cartão
  ou distribuição naquele turno.

### Passo 4 — Distribuição
- RF-16: A sugestão de distribuição DEVE usar o **orçamento mensal** como total planejado.
- RF-17: A distribuição DEVE aceitar três modos (D-03): (a) aceitar a sugestão padrão, (b) valores em
  **reais** com soma validada contra o orçamento mensal, (c) **percentuais** com soma validada em 100%.
- RF-18: Valores em reais cuja soma **fecha** o orçamento mensal DEVEM ser convertidos para basis
  points (soma 10000); soma que **não fecha** DEVE pedir correção, sem ativar orçamento parcial.
- RF-19: A distribuição resultante DEVE conter as 5 categorias canônicas, sem categoria ausente e sem
  valor negativo.

### Passo 5–6 — Resumo e ativação
- RF-20: O resumo DEVE exibir objetivo, orçamento mensal e distribuição por categoria.
- RF-21: O resumo NÃO DEVE exibir "renda mensal líquida" nem cartão (cartões são cadastrados após a
  ativação).
- RF-22: A ativação do orçamento DEVE ocorrer **somente após confirmação explícita** da cliente no
  resumo.
- RF-23: Confirmação negativa/ambígua no resumo DEVE **reabrir a etapa de distribuição** (D-09) para a
  cliente enviar novos valores e, em seguida, retornar ao resumo; nenhum orçamento é ativado enquanto
  não houver confirmação explícita, e nenhum orçamento parcial é ativado nesse caminho. O texto do
  resumo DEVE ser coerente com esse caminho de revisão ("não" leva à revisão da distribuição).

### Passo 7 — Recorrência
- RF-24: Logo após a ativação bem-sucedida e **antes** do cadastro de cartões (D-02), o sistema DEVE
  perguntar se a cliente deseja repetir o orçamento automaticamente pelos próximos 12 meses.
- RF-25: Resposta afirmativa DEVE criar a recorrência de 12 meses; resposta negativa DEVE seguir o
  fluxo sem criar recorrência. Resposta **ambígua** DEVE ser tratada como "não" e seguir o fluxo, sem
  reprompt (D-11). Nenhuma das respostas DEVE desfazer o orçamento já ativado.

### Passo 8 — Cartões em loop
- RF-26: O cadastro de cartões DEVE ocorrer após a distribuição, ativação e recorrência do orçamento.
- RF-27: O sistema DEVE perguntar se a cliente deseja adicionar um cartão de crédito.
- RF-28: Ao informar apelido, banco emissor e dia de vencimento válidos (dia entre 1 e 31), o sistema
  DEVE criar o cartão.
- RF-29: Após criar um cartão, o sistema DEVE perguntar se a cliente deseja adicionar outro, repetindo
  o ciclo **um cartão por vez** até a cliente responder que não deseja adicionar outro.
- RF-30: Cartão incompleto/inválido DEVE pedir somente os dados faltantes ou inválidos do cartão
  atual, NÃO DEVE criar cartão parcial e NÃO DEVE desfazer, recriar ou alterar o orçamento ativado.
- RF-31: O onboarding NÃO DEVE permitir o cadastro de vários cartões em uma única mensagem (cada
  cartão exige seu próprio turno).
- RF-31a: O loop de cartões NÃO tem limite máximo de cartões (D-05); encerra exclusivamente quando a
  cliente recusa adicionar outro.

### Passo 9 — Conclusão
- RF-32: Ao recusar adicionar cartão, o workflow DEVE marcar a etapa de cartões como concluída.
- RF-33: A conclusão DEVE gravar a WorkingMemory com o objetivo financeiro (e valor da meta quando
  houver) usando semântica de orçamento mensal, e enviar a mensagem final de próximos passos.
- RF-34: A conclusão NÃO DEVE reabrir distribuição, resumo ou ativação de orçamento.

### Semântica de estado e linguagem
- RF-35: O modelo de estado DEVE usar semântica de **orçamento mensal**, renomeando o campo de estado
  de renda para orçamento mensal e ajustando prompts, erros, resumo e WorkingMemory (D-04). Nenhum
  texto visível à cliente pode conter "renda líquida".
- RF-36: Os estados/fases do workflow DEVEM permanecer **fechados e parseáveis** (tipo enumerado com
  `String`/`Parse`), sem qualquer fase representada por string solta; qualquer fase nova segue esse
  contrato.
- RF-37: Mensagens e WorkingMemory NÃO DEVEM expor termos internos (workflow, run, snapshot,
  correlação, plataforma, infraestrutura).

### Robustez e durabilidade
- RF-38: A implementação DEVE preservar o workflow durável com suspensão e retomada por merge-patch;
  NÃO DEVE substituir o onboarding por branching solto no agente, handler ou consumer.
- RF-39: Qualquer falha ao listar cartões, criar cartão, sugerir alocação, criar budget, ativar
  budget, criar recorrência ou gravar WorkingMemory DEVE retornar erro tipado no step correspondente.
- RF-40: Em falha técnica de step, o sistema NÃO DEVE afirmar que cartão, orçamento ou onboarding
  foram concluídos, e a falha DEVE ser rastreável por workflow, step, status e erro sanitizado.
- RF-41: A extração estruturada do workflow DEVE continuar usando o provider LLM existente
  (OpenRouter), sem introduzir outro provider nem fallback chain.
- RF-42: A extração estruturada de cada categoria do workflow (meta, valor da meta, orçamento mensal,
  distribuição, confirmação de resumo, recorrência e cartão) DEVE atingir ratio de acerto ≥ 0,90 por
  categoria em harness com LLM real, sem baixar a régua (gate de aceite).
- RF-43: A renomeação do campo de estado (D-04) NÃO exige shim de compatibilidade: onboardings em
  andamento no deploy que tiverem passado da etapa de valor PODEM re-perguntar o orçamento mensal uma
  única vez (D-06); nenhuma outra etapa é afetada e nenhum orçamento ativado é impactado.

## Experiência do Usuário

**Jornada feliz (resumo):**
1. Boas-vindas isolada → cliente responde algo (ex.: "vamos começar").
2. Pergunta de meta → cliente informa objetivo (com ou sem valor).
3. Mensagem única: apresentação das 5 categorias + "Qual é o seu orçamento mensal?" → cliente informa
   o valor.
4. Sugestão de distribuição → cliente aceita, ou envia valores em R$ / percentuais.
5. Resumo (objetivo, orçamento mensal, distribuição) → cliente confirma → orçamento ativado.
6. Pergunta de recorrência 12 meses → cliente responde.
7. "Deseja adicionar um cartão?" → cliente cadastra cartão(ões) um por vez, até recusar.
8. Mensagem final de próximos passos.

**Casos de borda cobertos:** objetivo vazio (reprompt), valor de orçamento inválido (reprompt sem
criar nada), soma de distribuição que não fecha (pede correção sem ativar parcial), confirmação
negativa no resumo (reabre a distribuição para revisar e volta ao resumo), cartão incompleto (pede só
o que falta, não cria parcial, não mexe no orçamento), recusa de cartão (conclui), e falha técnica em
qualquer step (erro tipado sem falso sucesso).

**Tom e canal:** mantém o tom, emojis e regras de WhatsApp do agente financeiro
(`mecontrola_agent.go`); linguagem em português, sem jargão interno.

## Restrições Técnicas de Alto Nível

- Reutilizar os primitivos de `internal/platform/workflow` (durabilidade, `StepStatusSuspended`,
  `SuspendAwaitingInput`, retomada por merge-patch); não recriar o substrato agentivo nem o runtime
  Thread→Run.
- Reutilizar as portas `internal/agents/application/interfaces.BudgetPlanner`
  (`SuggestAllocation`, `CreateBudget`, `ActivateBudget`, `CreateRecurrence`, `GetMonthlySummary`,
  `DeleteDraftBudget`) e `CardManager` (`ListCards`, `CreateCard`); não introduzir tool financeira nova
  quando o workflow existente resolver com menor mudança.
- Taxonomia de categorias é fixa: os 5 slugs canônicos de `budgets/domain/valueobjects.RootSlug`; não
  criar/alterar slugs.
- O provider LLM é o OpenRouter existente; sem novo provider e sem fallback chain.
- Estados/fases fechados e parseáveis (DMMF state-as-type); regra de negócio permanece no workflow
  durável, não em handler/consumer/prompt solto.
- Zero comentários em código Go de produção e adaptadores finos (R-ADAPTER-001, R-AGENT-WF-001) devem
  ser respeitados na implementação.
- Privacidade: mensagens e WorkingMemory não expõem termos internos de plataforma.
- Wiring existente (`internal/agents/module.go` → `BuildOnboardingWorkflow`) deve ser preservado; a
  assinatura do builder pode evoluir para acomodar a nova ordem de steps, mantendo o consumidor real.

## Fora de Escopo

- Criar novas categorias de orçamento ou alterar slugs canônicos.
- Reescrever `internal/platform/workflow`, `internal/platform/agent`, `internal/platform/memory` ou o
  runtime Thread→Run.
- Criar nova tool financeira para o onboarding quando o workflow existente puder resolver.
- Permitir cadastro de vários cartões em uma única mensagem.
- Perguntar renda líquida como dado separado do orçamento mensal.
- Alterar billing, assinatura, Kiwify, entitlement ou integrações de pagamento.
- Impor limite máximo de cartões no onboarding (decidido: sem limite — D-05).
- Publicar ticket em ferramenta externa ou abrir pull request sem comando explícito.

## Decisões

- **D-01 — Categorias + orçamento em mensagem única.** A apresentação das 5 categorias e a pergunta de
  orçamento mensal são entregues em uma única mensagem com um único suspend. Motivo: menor superfície
  de mudança no substrato e avanço imediato sem confirmação, conforme a US. (RF-12)
- **D-02 — Recorrência logo após a ativação, antes dos cartões.** A pergunta de recorrência 12 meses
  ocorre imediatamente após ativar o orçamento e antes do loop de cartões. Motivo: a ativação passa a
  ocorrer na confirmação do resumo; a recorrência é uma decisão sobre o orçamento e fica junto dele,
  antes de mudar de assunto para cartões. (RF-24, RF-25)
- **D-03 — Distribuição mantém os 3 modos.** Aceitar sugestão padrão, valores em reais e percentuais.
  Motivo: menor mudança e maior flexibilidade; reais valida contra o orçamento mensal, percentual
  valida em 100%. (RF-17, RF-18)
- **D-04 — Renomear campo de estado e todos os textos para orçamento mensal.** O campo de estado
  (hoje `IncomeCents`) e todos os prompts/erros/resumo/WorkingMemory passam a usar orçamento mensal.
  Motivo: a US exige semântica de estado completa. (RF-35)
- **D-05 — Cartões sem limite máximo.** O loop de cartões não tem teto; encerra apenas quando a
  cliente recusa. Motivo: aderência direta à US ("repetir até não querer mais") e comportamento mais
  previsível. (RF-31a)
- **D-06 — Sem shim de compatibilidade para snapshots in-flight.** Aceita-se que onboardings em
  andamento no deploy que já passaram da etapa de valor re-perguntem o orçamento mensal uma vez.
  Motivo: volume desprezível de onboardings incompletos simultâneos; nenhum orçamento ativado é
  afetado; evita código de migração. (RF-43)
- **D-07 — Resposta à boas-vindas é apenas gatilho de avanço.** O texto de resposta ao Passo 1 não é
  interpretado como objetivo; a meta é sempre perguntada no Passo 2. Motivo: passos distintos como na
  US, previsibilidade e ausência de extração precoce/errônea. (RF-03)
- **D-08 — Preservar a pergunta opcional do valor da meta.** Mantém a feature "valor opcional da
  meta": se a cliente não informou valor com o objetivo, o valor é perguntado uma vez de forma
  opcional, sem bloquear. Motivo: menor mudança e enriquecimento do dado. (RF-07)
- **D-09 — "não" no resumo reabre a distribuição.** Confirmação negativa/ambígua no resumo volta à
  etapa de distribuição para novos valores e retorna ao resumo. Motivo: honra a promessa "para
  revisar" do texto do resumo e oferece caminho real de edição, sem ativar orçamento parcial. (RF-23)
- **D-10 — Cópia da boas-vindas redigida na implementação.** O texto do Passo 1 é escrito
  reaproveitando o tom/emojis já estabelecidos, sem a pergunta de meta. Motivo: cópia é detalhe de
  implementação; o tom já existe e não requer decisão de produto adicional. (RF-01)
- **D-11 — Recorrência negativa/ambígua segue sem recorrência.** Resposta negativa ou ambígua na
  pergunta de recorrência segue direto para os cartões, sem reprompt e sem criar recorrência. Motivo:
  simplicidade e não travar o fluxo; orçamento ativado nunca é desfeito. (RF-25)

**Nenhuma suposição ou questão em aberto.** Todos os pontos ambíguos foram resolvidos por decisão
explícita (D-01..D-11).

## Rastreabilidade (US → RF)

- US "boas-vindas isolada" → RF-01, RF-02, RF-03, RF-03a
- US "meta com valor opcional" → RF-04..RF-08
- US "categorias + avanço imediato" → RF-09..RF-14
- US "orçamento mensal substitui renda" → RF-13, RF-14, RF-35
- US "valor inválido gera reprompt" → RF-15
- US "distribuição em reais usa orçamento como total" → RF-16..RF-19
- US "resumo e ativação antes dos cartões" → RF-20..RF-23
- US "cartões um por vez após ativação" → RF-24..RF-31, RF-31a
- US "cartão incompleto mantém loop sem desfazer orçamento" → RF-30
- US "cliente recusa cartões e conclui" → RF-32..RF-34
- US "erro técnico sem falso sucesso" → RF-38..RF-40
- US regras de estado/linguagem/durabilidade → RF-35..RF-41
- Decisões de refinamento (D-05..D-11, gate/migração/revisão/cópia/recorrência) → RF-31a, RF-42,
  RF-43, RF-03, RF-07, RF-23, RF-01, RF-25
