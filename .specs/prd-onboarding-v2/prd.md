# Documento de Requisitos do Produto (PRD) — MeControla Onboarding V2

<!-- spec-version: 1 -->

## Visão Geral

O **Onboarding V2** redesenha a primeira experiência do usuário do MeControla no WhatsApp para
maximizar conversão e ativação, eliminando atrito conversacional. Hoje, após o usuário ativar a
conta com `ATIVAR [token]`, o sistema responde "Sua conta foi ativada!" e **para**: o onboarding só
começa quando o usuário envia outra mensagem por conta própria. Esse passo extra é fricção que
derruba ativação.

O Onboarding V2 transforma esse fluxo em uma jornada conduzida por IA (LLM), iniciada
automaticamente logo após a ativação, sem nenhuma mensagem adicional do usuário. A IA guia o
usuário por quatro etapas (Objetivo, Orçamento, Cartões, Plano Financeiro) e, na sequência,
conduz o registro da primeira transação financeira — momento em que o onboarding é concluído de
forma **determinística** e o usuário é entregue ao agente conversacional principal.

Duas garantias estruturais sustentam o produto:

1. **LLM mandatório**: o caminho conversacional por IA é sempre ativo; a máquina de estados (FSM)
   existe apenas como fallback de degradação quando o LLM falha ou expira.
2. **Persistência isolada e conclusão sem falso positivo**: todo o estado do onboarding (histórico
   e estado funcional) vive exclusivamente em `mecontrola.onboarding_sessions`; a conclusão é um
   fato de domínio persistido transacionalmente, nunca uma inferência textual.

**Para quem é**: novos assinantes do MeControla que acabaram de pagar e ativar a conta via WhatsApp.

**Por que é valioso**: reduz o tempo até o primeiro valor (primeira transação registrada),
aumenta a taxa de ativação e elimina ambiguidade de estado entre o onboarding e o agente
principal, evitando handoff prematuro ou duplicado.

## Objetivos

- **Ativação sem fricção**: 0 mensagens extras exigidas do usuário entre `ATIVAR [token]` e a
  primeira pergunta do onboarding.
- **Redução de volume de mensagens**: reduzir em 30% a 50% a quantidade de mensagens do fluxo de
  onboarding em relação ao fluxo anterior.
- **Conclusão com 0 falso positivo**: nenhuma sessão é marcada como concluída sem que o domínio
  confirme, de forma persistida e transacional, todos os pré-requisitos (objetivo, orçamento,
  cartões, distribuição e primeira transação).
- **Redução de atrito conversacional**: eliminar 100% das perguntas de confirmação supérfluas
  ("Faz sentido?", "Posso continuar?") do roteiro de onboarding.
- **Isolamento de estado**: 0 leituras ou escritas de estado de onboarding em
  `mecontrola.agent_sessions` após a implementação.
- **Idempotência da saudação proativa**: 0 saudações duplicadas mesmo sob reprocessamento do
  evento de ativação pelo outbox.
- **Métricas-chave a acompanhar**:
  - Taxa de conclusão de onboarding (sessões `state=active` / sessões iniciadas).
  - Tempo médio da ativação até a primeira transação registrada.
  - Taxa de queda por etapa (Objetivo → Orçamento → Cartões → Plano → 1ª transação).
  - Taxa de fallback FSM (turnos atendidos pela FSM / total de turnos).
  - Taxa de saudação duplicada (deve ser 0).

## Histórias de Usuário

### Persona primária — Novo assinante (caminho feliz)

- Como **novo assinante**, quero que o onboarding **comece sozinho** logo após eu ativar a conta,
  para que eu não precise descobrir o que fazer em seguida.
- Como **novo assinante**, quero ver **em que etapa estou** (ex.: `Etapa 2/4 — Orçamento`) a cada
  interação, para que eu saiba quanto falta.
- Como **novo assinante**, quero informar **todos os meus cartões de uma vez** (ou dizer que não
  uso cartão), para que o cadastro seja rápido.
- Como **novo assinante**, quero que o sistema **sugira automaticamente** minha distribuição
  financeira após eu informar objetivo e renda, para que eu não tenha que calcular nada.
- Como **novo assinante**, quero **ajustar a distribuição em linguagem natural** ("coloca mais em
  metas"), para que eu não precise aprender comandos.
- Como **novo assinante**, quero registrar minha **primeira transação ainda no onboarding**, para
  que eu sinta o produto funcionando imediatamente.

### Persona primária — Caminhos de exceção

- Como **novo assinante**, se eu **parar no meio** do onboarding e voltar depois, quero retomar
  exatamente de onde parei, sem refazer etapas já concluídas.
- Como **novo assinante**, se eu **corrigir** um dado já informado (ex.: trocar a renda), quero que
  o sistema recalcule apenas as diferenças, sem reiniciar a distribuição do zero.
- Como **novo assinante**, se o **LLM falhar temporariamente**, quero continuar o onboarding via
  fluxo de fallback, sem perder o progresso já coletado.

### Persona secundária — Agente principal (handoff)

- Como **agente conversacional principal**, quero reconhecer o fim do onboarding **apenas por sinal
  determinístico persistido** (`state=active` + `completed_at`, ou evento `OnboardingCompleted`),
  para que eu nunca assuma um onboarding concluído com base em texto ou memória parcial.
- Como **agente conversacional principal**, quero **não receber nenhuma mensagem** enquanto o
  onboarding estiver em andamento, para evitar interferência entre fluxos no mesmo canal WhatsApp.

### Persona secundária — Operação/Plataforma

- Como **operador**, quero que a aplicação **não suba** se o modelo de LLM de onboarding não estiver
  configurado, para que a degradação silenciosa seja impossível.
- Como **operador**, quero detectar **drift de estado** (`state=active` sem `completed_at`) de forma
  explícita, para que inconsistências não sejam tratadas como sucesso.

## Funcionalidades Core

### 1. Auto-start do onboarding pós-ativação

- **O que faz**: imediatamente após `ATIVAR [token]` válido, envia confirmação de ativação +
  apresentação do bot e cria a sessão de onboarding; a primeira pergunta é enviada proativamente
  pela IA, sem mensagem adicional do usuário.
- **Por que é importante**: elimina o principal ponto de fricção e queda na ativação.
- **Como funciona (alto nível)**: a ativação publica um evento de domínio
  (`onboarding.subscription_bound`) carregando o identificador do usuário e o número de contato; um
  consumidor no módulo do agente dispara a saudação inicial via LLM. A confirmação de ativação e a
  primeira pergunta da IA nunca colidem (apenas a IA emite a primeira pergunta).

### 2. Onboarding conduzido por LLM (mandatório)

- **O que faz**: conduz o usuário por quatro etapas conversacionais e pelo registro da primeira
  transação, interpretando linguagem natural.
- **Por que é importante**: experiência fluida e tolerante a variações de linguagem aumenta a
  conclusão.
- **Como funciona (alto nível)**: o LLM é sempre o caminho primário (Tier 1); a FSM determinística
  é fallback de degradação (Tier 2). A aplicação exige um modelo de LLM de onboarding configurado
  para iniciar.

### 3. Roteiro de baixo atrito

- **O que faz**: aplica as 10 regras de conversão (sem confirmações supérfluas, categorias em bloco
  único, indicador de progresso, coleta de cartões em uma mensagem, distribuição automática,
  preservação de progresso, ajuste conversacional, resumo enxuto, continuidade até a primeira
  transação, prioridade de experiência).
- **Por que é importante**: cada confirmação removida e cada etapa condensada reduz queda.
- **Como funciona (alto nível)**: o roteiro e os prompts são desenhados para fluir organicamente;
  o indicador de etapa aparece em toda interação de onboarding.

### 4. Persistência isolada do onboarding

- **O que faz**: armazena histórico de turnos e estado funcional do onboarding exclusivamente em
  `mecontrola.onboarding_sessions` (coluna JSONB `payload`), sem tocar `mecontrola.agent_sessions`.
- **Por que é importante**: evita colisão de estado entre onboarding e agente principal no mesmo
  canal WhatsApp.
- **Como funciona (alto nível)**: o `payload` passa a conter `recent_turns`, `welcome_sent_at` e
  `completed_at`, além dos campos já existentes (objetivo, renda, cartões, distribuição, fase,
  primeira transação).

### 5. Conclusão determinística e handoff seguro

- **O que faz**: promove a sessão para `state=active` e grava `completed_at` no mesmo write
  transacional, publicando `OnboardingCompleted`, somente quando todos os pré-requisitos de domínio
  estão satisfeitos.
- **Por que é importante**: garante 0 falso positivo na conclusão e um handoff inequívoco para o
  agente principal.
- **Como funciona (alto nível)**: o agente principal só passa a tratar mensagens como fluxo normal
  após detectar sinal determinístico persistido; nunca por heurística textual. A WorkingMemory de
  perfil é sintetizada de forma assíncrona, por um consumidor do evento `OnboardingCompleted`, sem
  bloquear a conclusão (RF-34).

### 6. Idempotência e robustez

- **O que faz**: garante saudação única sob reprocessamento de evento e degradação controlada sob
  falha de LLM.
- **Por que é importante**: o outbox pode reprocessar eventos; o usuário não pode receber boas-vindas
  duplicadas nem perder progresso.
- **Como funciona (alto nível)**: a saudação proativa usa uma chave de idempotência estável por
  evento; quando a saudação já foi registrada (`welcome_sent_at`), o reprocessamento não reenvia.

## Requisitos Funcionais

### Auto-start e ativação

- RF-01: Ao consumir `ATIVAR [token]` válido, o sistema DEVE enviar, sem exigir nova mensagem do
  usuário: (1) a confirmação de ativação, (2) a apresentação do bot e (3) criar a sessão de
  onboarding.
- RF-02: O sistema NÃO DEVE enviar a primeira pergunta da FSM (`startResult.Reply`) após a ativação;
  a primeira pergunta DEVE ser emitida exclusivamente pela IA (saudação proativa).
- RF-03: A ativação DEVE publicar o evento `onboarding.subscription_bound` carregando o
  identificador do usuário e o número de contato (peer) necessários para a saudação proativa.
- RF-04: Um consumidor no módulo do agente DEVE reagir ao evento `onboarding.subscription_bound` e
  disparar a saudação inicial do onboarding via runtime do agente.
- RF-05: Se a sessão de onboarding ainda não existir quando a saudação proativa for disparada, o
  consumidor DEVE forçar reprocessamento (retentativa) em vez de consumir o evento silenciosamente,
  até que a sessão exista.

### LLM mandatório e fallback

- RF-06: O caminho LLM do onboarding DEVE estar sempre ativo; NÃO DEVE existir feature flag que o
  desabilite.
- RF-07: A aplicação NÃO DEVE iniciar se o modelo de LLM de onboarding não estiver configurado,
  falhando com erro explícito na inicialização.
- RF-08: A FSM determinística DEVE permanecer disponível apenas como fallback de degradação quando
  o LLM falhar ou expirar, sem corromper o progresso já coletado.

### Roteiro de baixo atrito (10 regras de negócio)

- RF-09: O roteiro de onboarding NÃO DEVE conter perguntas de confirmação supérfluas (ex.: "Faz
  sentido?", "Entendeu?", "Posso continuar?", "Tudo certo até aqui?", "Posso seguir?"). (REGRA 1)
- RF-10: As 5 categorias fixas (Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira)
  DEVEM ser apresentadas em uma única mensagem formatada, sem solicitar confirmação nem explicar uma
  a uma. (REGRA 2)
- RF-11: Toda interação de onboarding DEVE exibir o estágio atual no formato de progresso (ex.:
  `Etapa 1/4 — Objetivo`, `Etapa 2/4 — Orçamento`, `Etapa 3/4 — Cartões`,
  `Etapa 4/4 — Plano Financeiro`). (REGRA 3)
- RF-12: A coleta de cartões DEVE ser feita em uma única mensagem, no formato apelido + dia de
  fechamento (`Nubank 13` / `Inter 5` / `Itaú 10`), aceitando também a resposta "Não uso". O número
  informado representa o **dia de fechamento** da fatura (não o vencimento). (REGRA 4)
- RF-13: Após receber Objetivo e Orçamento Mensal, o sistema DEVE sugerir automaticamente a
  distribuição financeira, sem exigir cálculo do usuário. A distribuição DEVE variar conforme o
  objetivo, via perfis fixos determinísticos: o objetivo é classificado em um perfil durante o parse
  do LLM e o cálculo dos valores é determinístico (sem LLM no cálculo). Perfis e percentuais (cada
  linha soma 100): Quitar dívidas = CF 45 / Conh 5 / Praz 10 / Metas 25 / LF 15; Reserva de
  emergência = 40 / 5 / 10 / 15 / 30; Investir/patrimônio = 40 / 10 / 10 / 10 / 30; Meta específica =
  40 / 5 / 10 / 30 / 15; Organizar gastos (default) = 40 / 10 / 15 / 20 / 15. (REGRA 5)
- RF-13a: Quando o objetivo for ambíguo ou não se encaixar em nenhum perfil conhecido, o sistema
  DEVE usar o perfil default "Organizar gastos" (40 / 10 / 15 / 20 / 15) e seguir o fluxo sem
  fricção, permitindo ajuste posterior em linguagem natural (RF-15).
- RF-14: Durante correções, o sistema DEVE preservar o progresso já coletado e recalcular apenas as
  diferenças, NUNCA reiniciar a distribuição do zero. (REGRA 6)
- RF-15: O sistema DEVE interpretar e aplicar alterações de distribuição/limite expressas em
  linguagem natural de forma fluida. (REGRA 7)
- RF-16: O resumo final DEVE exibir apenas as informações essenciais: Objetivo, Orçamento, Cartões e
  Distribuição Final. (REGRA 8)
- RF-17: Após o resumo, o sistema NÃO DEVE encerrar; DEVE iniciar imediatamente o fluxo de registro
  da primeira transação financeira. (REGRA 9)
- RF-18: Toda decisão de fluxo do agente DEVE priorizar redução de atrito e aumento de ativação.
  (REGRA 10)

### Persistência isolada

- RF-19: Todo o estado funcional do onboarding (objetivo, renda, cartões, distribuição/custom split,
  primeira transação registrada, fase) DEVE ser persistido exclusivamente em
  `mecontrola.onboarding_sessions` (coluna `payload`).
- RF-20: O histórico de turnos do onboarding (`recent_turns`) DEVE ser persistido em
  `onboarding_sessions.payload`, ser bounded em uma janela de 3 pares (usuário+assistente) e
  exclusivo do onboarding.
- RF-21: O onboarding NÃO DEVE ler nem gravar `recent_turns`, `pending_action` ou qualquer estado
  transitório próprio em `mecontrola.agent_sessions`.
- RF-22: O `payload` de onboarding DEVE persistir `welcome_sent_at` e `completed_at` além dos campos
  funcionais.

### Conclusão determinística e handoff

- RF-23: O onboarding só DEVE ser marcado como concluído quando todos os pré-requisitos de domínio
  estiverem satisfeitos: objetivo definido, orçamento mensal válido, cartões coletados (ou "não
  uso"), distribuição (custom split) gerada e primeira transação financeira registrada.
- RF-24: A conclusão DEVE ocorrer em um único write transacional que marca `state=active`, grava
  `completed_at` e publica o evento `OnboardingCompleted`.
- RF-25: O evento `OnboardingCompleted` só DEVE ser publicado após a persistência bem-sucedida do
  estado concluído.
- RF-26: O agente principal DEVE reconhecer onboarding concluído apenas por sinal determinístico
  persistido (`state=active` com `completed_at`, ou evento `OnboardingCompleted`), NUNCA por
  heurística textual, fase, ausência de mensagens ou memória conversacional.
- RF-27: Enquanto o onboarding estiver em andamento (`state != active`), as mensagens do usuário no
  canal DEVEM ser tratadas exclusivamente pelo fluxo de onboarding; o agente principal NÃO DEVE
  recebê-las.
- RF-28: Após a conclusão, uma nova mensagem do usuário NÃO DEVE reabrir o onboarding e DEVE ser
  tratada pelo fluxo normal do agente principal.

### Idempotência, retomada e drift

- RF-29: A saudação proativa inicial DEVE ser idempotente: sob reprocessamento do evento de
  ativação, o usuário NÃO DEVE receber saudação duplicada quando `welcome_sent_at` já estiver
  persistido (ou quando a decisão correspondente já existir pela chave de idempotência por evento).
- RF-30: Se o usuário interromper o onboarding e retornar, o sistema DEVE retomar exatamente da fase
  persistida, preservando os dados já coletados.
- RF-31: Um registro com `state=active` e `completed_at` ausente DEVE ser tratado como drift
  explícito (registro/observabilidade), NUNCA como sucesso silencioso.
- RF-32: Falhas do LLM durante o onboarding NÃO DEVEM promover conclusão, apagar progresso válido
  nem contaminar o estado do agente principal.

### Isolamento de agentes

- RF-33: O onboarding LLM e o agente conversacional principal DEVEM operar como workflows/tools
  independentes no módulo do agente, sem compartilhar estado transitório, evitando colisão no mesmo
  canal WhatsApp.

### Handoff, retenção e foco conversacional

- RF-34: A síntese da WorkingMemory do usuário (perfil injetado no system prompt do agente
  principal) DEVE ocorrer de forma assíncrona, em um consumidor do evento `OnboardingCompleted`,
  idempotente e com retentativa via outbox. NÃO DEVE rodar inline bloqueando o turno nem condicionar
  a conclusão do onboarding. A ausência temporária de WorkingMemory NÃO impede o agente principal de
  operar (degradação graciosa).
- RF-35: Ao concluir o onboarding (`state=active`), o sistema DEVE limpar `recent_turns` do
  `payload`, preservando o estado funcional (objetivo, renda, cartões, distribuição,
  `first_tx_recorded`, `phase`) e `completed_at`. O estado funcional e `completed_at` são retidos
  para auditoria; o histórico conversacional bounded não é retido após a conclusão.
- RF-36: Se o usuário enviar mensagem fora do fluxo (off-topic, dúvida, desabafo) durante o
  onboarding, o sistema DEVE responder de forma breve e empática e retomar a pergunta da etapa atual
  (com o indicador de progresso), sem sair do onboarding nem abrir conversa livre.

## Experiência do Usuário

### Jornada principal (caminho feliz)

```
Usuário:  ATIVAR ABC123

Bot:      [msg 1] Sua conta foi ativada! Bem-vindo ao MeControla. 🎉
Bot:      [msg 2] Eu sou seu assistente financeiro. Vou te ajudar a organizar
                  seu dinheiro em poucos minutos, aqui mesmo pelo WhatsApp.
Bot:      [msg 3 — IA] 🔵 Etapa 1/4 — Objetivo
                  Pra começar, qual é o seu principal objetivo financeiro agora?
                  (ex.: quitar dívidas, juntar uma reserva, investir mais)

Usuário:  quero montar uma reserva de emergência

Bot:      [IA] 🔵 Etapa 2/4 — Orçamento
                  Show. Qual é a sua renda mensal aproximada?

Usuário:  uns 6 mil

Bot:      [IA] Com base no seu objetivo (reserva de emergência) e renda, sugeri esta distribuição:
                  • Custo Fixo — 40%
                  • Conhecimento — 5%
                  • Prazeres — 10%
                  • Metas — 15%
                  • Liberdade Financeira — 30%

                  🔵 Etapa 3/4 — Cartões
                  💳 Você usa cartão de crédito? Me manda todos de uma vez, assim:
                  Nubank 13 / Inter 5 / Itaú 10 (apelido + dia de fechamento).
                  Se não usa, responde "Não uso".

Usuário:  Nubank 13 / Inter 5

Bot:      [IA] 🔵 Etapa 4/4 — Plano Financeiro
                  Seu plano ficou assim:
                  • Objetivo: Reserva de emergência
                  • Orçamento: R$ 6.000/mês
                  • Cartões: Nubank (fecha dia 13), Inter (fecha dia 5)
                  • Distribuição: Custo Fixo 40% · Conhecimento 5% · Prazeres 10% ·
                    Metas 15% · Liberdade Financeira 30%

                  Pra fechar, vamos registrar seu primeiro lançamento.
                  Me conta um gasto ou recebimento recente (ex.: "mercado 150").

Usuário:  mercado 150

Bot:      [IA] Registrei: Mercado — R$ 150,00 em Custo Fixo. ✅
                  Pronto, tudo configurado! A partir de agora é só me mandar seus
                  gastos e recebimentos que eu organizo pra você.

[handoff determinístico → agente principal assume o canal]
```

### Jornada de exceção — retomada

```
[usuário parou na Etapa 3/4 — Cartões e voltou no dia seguinte]

Usuário:  oi

Bot:      [IA] 🔵 Etapa 3/4 — Cartões
                  Quase lá! Faltou só me contar seus cartões.
                  Manda assim: Nubank 13 / Inter 5 / Itaú 10. Se não usa, "Não uso".
```

> **Nota de fidelidade ao roteiro**: o bloco das 5 categorias (RF-10) deve seguir o formato
> canônico do documento de referência `MeControla_Onboarding_V2.md` — uma única mensagem com
> 💰 Custo Fixo, 🎓 Conhecimento, 🎉 Prazeres, 🎯 Metas e 🏦 Liberdade Financeira, cada uma com
> uma linha curta de descrição, sem confirmação. O texto literal das mensagens é contrato e será
> fixado na Especificação Técnica.

### Jornada de exceção — ajuste conversacional

```
Bot:      [IA] ...sugeri esta distribuição:
                  • Custo Fixo — 40% ...

Usuário:  coloca mais em metas, uns 25%

Bot:      [IA] Ajustei: Metas subiu pra 25% e Custo Fixo desceu pra 45%.
                  O restante segue igual.
```

- **Considerações de UI/UX**: canal exclusivamente conversacional (WhatsApp/texto); mensagens
  curtas, sem jargão; indicador de etapa sempre visível; formatação de listas com marcadores; sem
  perguntas de confirmação supérfluas.
- **Acessibilidade**: linguagem simples, respostas tolerantes a variações de escrita e a respostas
  fora de ordem; aceitação de "Não uso" como resposta de primeira classe para cartões.

## Restrições Técnicas de Alto Nível

- **Integração existente — WhatsApp**: toda interação ocorre no canal WhatsApp; onboarding e agente
  principal compartilham o mesmo `(user_id, channel)` e não podem colidir.
- **Mensageria por outbox**: a saudação proativa depende de evento publicado via outbox, sujeito a
  reprocessamento — idempotência é não-negociável.
- **Persistência — PostgreSQL**: fonte canônica do lifecycle do onboarding é
  `mecontrola.onboarding_sessions`; `mecontrola.agent_sessions` permanece de uso exclusivo do agente
  principal.
- **Conclusão transacional**: a promoção para concluído e a publicação do evento de conclusão devem
  ser atômicas (mesmo write transacional).
- **Backend — Go**: implementação nos módulos `internal/onboarding` e `internal/agent`, seguindo o
  padrão Workflow/Tool do agente (Mastra) restrito a `internal/agent`, sem regra de domínio em
  adapters.
- **Compatibilidade de LLM (via OpenRouter, modelos já presentes no projeto)**: o onboarding DEVE
  usar exclusivamente os modelos já integrados via OpenRouter no projeto — Gemini 2.5 Flash Lite,
  Mistral Small 3.2 24B, Claude Haiku 4.5 e GPT-5 Nano — sem introduzir novos provedores. O modelo
  primário do onboarding é `anthropic/claude-haiku-4.5` (default atual). A escolha de primário e da
  cadeia de fallback DEVE respeitar a estabilidade de tool-calling por modelo já observada no
  projeto (flash-lite e gpt-5-nano têm tool-calling/parse instável em parte dos fluxos), priorizando
  os modelos comprovadamente estáveis para o tool-calling do onboarding.
- **Privacidade de dados**: o número de contato (peer) trafega em eventos e logs deve ser mascarado;
  o histórico do onboarding é bounded e não deve reter dados além da janela necessária.

### Padrões Mastra (inspiração obrigatória, restrita a `internal/agent`)

Os planos técnicos e a skill `mastra` (`.agents/skills/mastra/`) exigem que o onboarding V2 seja
construído sobre os primitivos Mastra já presentes no módulo `internal/agent`, mapeados ao código Go
real. A inspiração conceitual vem do framework Mastra (https://github.com/mastra-ai/mastra), porém a
implementação é 100% Go e estes padrões são **proibidos fora de `internal/agent`** (regra hard
R-AGENT-WF-001):

- **Workflow → Tool → binding → usecase**: todo comportamento novo do onboarding entra como
  Workflow/Tool reutilizando bindings e usecases; nunca como novo `case intent.Kind` no switch do
  agente. Onboarding LLM e agente principal são workflows isolados (RF-33).
- **Thread → Run**: `Thread = (user_id, channel)` resolvido a cada execução; cada turno abre/fecha um
  `Run` auditável. A saudação proativa também é um `Run`.
- **Pending Step (suspend & resume)**: a retomada do onboarding (RF-30) espelha o suspend/resume de
  workflow do Mastra — o estado é salvo como snapshot persistido (`payload` em
  `onboarding_sessions`) e retomado a partir da fase persistida, sobrevivendo a reinícios.
- **WorkingMemory**: a síntese de working memory para o agente principal (handoff, RF-26/RF-34) usa
  o escopo por `resource` (`user_id`, cross-channel) do Mastra e só deve ser produzida **após** a
  conclusão inequívoca do onboarding, de forma assíncrona via consumidor de `OnboardingCompleted`
  (idempotente, com retentativa do outbox), nunca inline bloqueando o turno.
- **ToolOutcome/RunStatus/AwaitingKind/TransactionKind** permanecem tipos fechados (state-as-type),
  nunca string livre.

## Análise de Lacunas e Incompatibilidades

Esta seção documenta inconsistências entre o codebase atual e os planos técnicos, para orientar a
Especificação Técnica. Detalhes de solução pertencem à techspec; aqui registramos apenas o risco de
produto e a fronteira a respeitar.

### LG-01 — Incompatibilidade de modelo de dados no payload de onboarding

O struct de persistência atual `onboardingSessionPayloadJSON`
(`internal/onboarding/infrastructure/repositories/postgres/onboarding_session_repository.go`)
persiste apenas: `income_cents`, `cards`, `pending_card`, `has_pending`, `split`, `objective`,
`custom_split`, `first_tx_recorded`, `phase`. **Faltam** os campos exigidos pela persistência
isolada: `recent_turns`, `welcome_sent_at` e `completed_at`. Risco: sem esses campos, retomada e
idempotência da saudação dependem de `agent_sessions`, violando o isolamento.

### LG-02 — `recent_turns` hoje vive em `agent_sessions` (colisão de fronteira)

O `RunOnboardingTurn` (`internal/agent/application/usecases/run_onboarding_turn.go`) lê e grava
`recent_turns` via `sessionRepo` apontando para `agent_sessions` (`loadOnbHistory`/`saveOnbTurn`).
Isso contraria a decisão de isolar o histórico do onboarding em `onboarding_sessions.payload`.
Risco: onboarding e agente principal disputam a mesma sessão `(user_id, channel)`, podendo
sobrescrever histórico ou estado um do outro.

### LG-03 — Estrutura de `recent_turns` vs. turnos do `RunOnboardingTurn`

O histórico atual serializa `[]entities.ConversationMessage` (formato do agente). O formato-alvo do
onboarding é um `OnboardingTurn` mínimo (`role`, `text`, `occurred_at`). Risco: divergência de
schema entre o que o repositório do onboarding persistirá e o que o turno LLM consome — exige
mapeamento explícito e definição única do formato na techspec.

### LG-04 — Race condition entre criação da sessão e saudação proativa

A criação da sessão (`StartBudgetConfiguration`) ocorre **após** o evento `subscription_bound` já
estar no outbox. Se o poller disparar a saudação antes de a sessão existir, o turno encontra
`InProgress=false` e o evento pode ser consumido sem reenvio. Risco de produto: usuário não recebe a
primeira pergunta. Mitigação requerida: forçar retentativa (RF-05).

### LG-05 — Chave de idempotência da saudação proativa

A decisão de escrita do agente usa `(user_id, channel, message_id)` como chave. O trigger proativo
não tem um `message_id` natural. Sem uma chave estável por evento, reprocessamento gera saudação
duplicada. Risco de produto: boas-vindas duplicadas (RF-29). Mitigação: usar identificador estável
do evento como chave.

### LG-06 — Estado dual (FSM + LLM) pode divergir

O estado é rastreado por `onboarding_sessions.state` (FSM) e pelo histórico/fase usados pelo LLM. Se
o dispatcher de tools falhar após a chamada do LLM mas antes de persistir a transição, a FSM pode
ficar presa enquanto o LLM considera o passo concluído. Risco: travamento ou avanço inconsistente.
Mitigação: conclusão idempotente e write atômico (RF-24, RF-32).

### LG-07 — Tolerância a campos extras no consumidor existente

A adição de `peer_e164` ao payload de `subscription_bound` pressupõe unmarshaling permissivo no
consumidor existente. Se o consumidor usar `DisallowUnknownFields`, a adição quebra o consumidor em
produção. Risco: regressão silenciosa. Mitigação: verificar antes de evoluir o payload.

### LG-09 — Coleta de cartão usa vencimento, não fechamento

O código atual coleta `due_day` (dia de vencimento) — a tool `save_onboarding_card`
(`internal/agent/application/usecases/onboarding_tool_catalog.go`) exige `nickname` + `due_day` e o
dispatcher responde "vence dia %d"; o script `scriptCardQuestion` pergunta "dia de vencimento". Isso
contraria o documento de referência (REGRA 4), que define **dia de fechamento**. Risco de produto:
ciclo de fatura incorreto (o fechamento é o que determina em qual fatura a compra cai). Mitigação:
alterar a coleta, o schema da tool, os scripts e o mapeamento de persistência para `closing_day`
(RF-12). Limite e vencimento não são coletados no onboarding (cartão skeleton, completado depois).

### LG-10 — Distribuição automática não varia por objetivo

`buildAutoSplits(incomeCents)` (`internal/agent/application/usecases/onboarding_scripts.go`) usa um
template fixo por renda (atual: 40/10/15/20/15) e ignora o objetivo. A decisão de produto (RF-13)
exige variar por objetivo via perfis fixos determinísticos. Risco: a recomendação financeira não
reflete o objetivo declarado. Mitigação: introduzir classificação de objetivo em perfil (no parse) e
um mapeamento determinístico perfil→template, com fallback "Organizar gastos".

### LG-08 — Colisão de canal entre processador de onboarding e agente

O `whatsapp_message_processor.go` (onboarding) e a infraestrutura de agentes operam sobre o mesmo
canal WhatsApp e o mesmo `(user_id, channel)`. Risco: dupla resposta ou interferência se ambos
tratarem a mesma mensagem. Mitigação: prioridade de roteamento que garante exclusividade do
onboarding enquanto `InProgress=true` (RF-27, RF-33).

## Casos de Borda (Edge Cases)

Comportamento esperado nos cenários de exceção, para um MVP robusto e production-ready. Cada caso
referencia o requisito funcional que o governa.

- **EB-01 — Saudação dispara antes da sessão existir** (race do outbox, LG-04): o consumidor força
  retentativa (retorna erro) até a sessão de onboarding existir; quando existir, `emitWelcome()` é
  enviado normalmente. (RF-05)
- **EB-02 — Reprocessamento do evento de ativação**: sob retry do outbox, a saudação não é
  duplicada — `welcome_sent_at` persistido e/ou a decisão por chave estável de evento detectam o
  replay. (RF-29)
- **EB-03 — Falha do LLM no meio do fluxo**: o turno cai no fallback FSM sem promover conclusão, sem
  apagar progresso válido e sem contaminar o agente principal; em falha da saudação proativa, o
  consumidor loga warn e retorna erro para retry, sem quebrar a ativação já confirmada. (RF-08,
  RF-32)
- **EB-04 — Usuário abandona o onboarding**: a sessão permanece `state != active` indefinidamente
  (sem TTL no MVP); ao retornar, retoma exatamente da fase persistida, com dados preservados. (RF-30)
- **EB-05 — Mensagem off-topic durante o onboarding**: resposta breve e empática + retomada da etapa
  atual, sem abrir conversa livre nem sair do onboarding. (RF-36)
- **EB-06 — Correção de dado já informado**: o sistema recalcula apenas as diferenças (ex.: ajuste de
  distribuição), preservando o progresso já coletado, sem reiniciar. (RF-14, RF-15)
- **EB-07 — Usuário sem cartão**: "Não uso" satisfaz o pré-requisito de cartões para conclusão,
  equivalendo a uma lista vazia. (RF-12, RF-23)
- **EB-08 — Conclusão sem primeira transação**: a conclusão é recusada (`first transaction required`)
  e o fluxo permanece na etapa de primeira transação até registrá-la. (RF-23, RF-17)
- **EB-09 — Mensagem após conclusão**: nova mensagem não reabre o onboarding e é roteada ao agente
  principal, que reconhece a conclusão apenas por sinal determinístico (`state=active` +
  `completed_at` ou evento `OnboardingCompleted`). (RF-26, RF-28)
- **EB-10 — Drift de estado** (`state=active` sem `completed_at`): tratado como inconsistência
  explícita (registro/observabilidade), nunca como sucesso silencioso. (RF-31)
- **EB-11 — Falha na síntese de WorkingMemory**: como é assíncrona via consumidor de
  `OnboardingCompleted` com retry, a falha não bloqueia a conclusão nem o handoff; o agente principal
  degrada graciosamente sem WorkingMemory até a retentativa persistir. (RF-34)
- **EB-12 — Mensagens concorrentes no mesmo canal**: enquanto `InProgress=true`, todas as mensagens
  do usuário são tratadas exclusivamente pelo onboarding; o agente principal não as recebe, evitando
  dupla resposta. (RF-27, RF-33)
- **EB-13 — Objetivo ambíguo/desconhecido**: o objetivo que não classifica em nenhum perfil cai no
  template default "Organizar gastos" (40/10/15/20/15) e o fluxo segue sem fricção; o usuário pode
  ajustar a distribuição depois em linguagem natural. (RF-13a, RF-15)
- **EB-14 — Cartão sem dia de fechamento válido**: entrada que não permite extrair apelido + dia de
  fechamento válido (ex.: dia fora de 1–31) leva a re-pergunta curta da etapa de cartões, sem travar
  o fluxo nem avançar com dado inválido. (RF-12)
- **EB-15 — Renda fora dos limites do módulo**: `internal/onboarding` exige renda em
  R$500..R$1B (`50000..10000000000` cents). Renda abaixo/acima leva a re-pergunta amigável da etapa
  de orçamento, nunca erro fatal nem avanço com valor inválido. (RF-13, contrato do módulo)
- **EB-16 — Apelido de cartão acima do limite**: `internal/card` exige nickname 1..32 caracteres; o
  fluxo valida na fronteira e re-pergunta quando excede, evitando rejeição a jusante. (RF-12)

## Fora de Escopo

- Conversa multi-turno aberta com o LLM no agente principal (P2-1) — permanece não-goal do MVP.
- Migração do histórico conversacional compartilhado para um pacote comum
  (`internal/platform/conversation`) — ação pós-MVP.
- Coluna SQL de discriminador de tipo para `agent_sessions.pending_action`
  (`pending_action_kind`) — melhoria de observabilidade pós-MVP.
- Onboarding em canais diferentes de WhatsApp (web, app) — fora do MVP.
- Internacionalização do roteiro (idioma além de pt-br).
- Edição/reabertura manual do onboarding após conclusão.
- Definição detalhada de prompts, schemas de tool, migrações SQL e wiring — pertencem à
  Especificação Técnica.

## Decisões Resolvidas

Decisões confirmadas com o dono do produto (2026-06-23), encerrando as questões em aberto da versão
inicial:

- **DR-01 — Modelos de LLM**: o onboarding usa exclusivamente os modelos já integrados via OpenRouter
  no projeto — Gemini 2.5 Flash Lite, Mistral Small 3.2 24B, Claude Haiku 4.5 e GPT-5 Nano — sem
  introduzir novos provedores. Primário: `anthropic/claude-haiku-4.5`; a cadeia de fallback respeita
  a estabilidade de tool-calling observada por modelo. (Restrições Técnicas)
- **DR-02 — Síntese de WorkingMemory assíncrona**: produzida por consumidor do evento
  `OnboardingCompleted`, idempotente e com retry do outbox; nunca inline nem bloqueando a conclusão.
  (RF-34)
- **DR-03 — Retenção pós-conclusão**: ao concluir, limpar `recent_turns`; preservar estado funcional
  e `completed_at` indefinidamente (sem job de TTL no MVP). (RF-35)
- **DR-04 — Off-topic durante o onboarding**: resposta breve + redirecionamento à etapa atual; sem
  conversa livre. (RF-36)
- **DR-05 — Cartão por dia de fechamento**: o onboarding coleta apelido + dia de **fechamento**
  (alinhado ao doc de referência), exigindo ajuste do código atual que usa vencimento (`due_day`).
  Limite e vencimento ficam para configuração posterior. (RF-12, LG-09)
- **DR-06 — Distribuição varia por objetivo (perfis fixos)**: 5 perfis determinísticos
  (Quitar dívidas, Reserva de emergência, Investir/patrimônio, Meta específica, Organizar gastos);
  objetivo classificado no parse, cálculo determinístico, fallback = Organizar gastos. (RF-13, LG-10)
- **DR-07 — Objetivo ambíguo → default**: cair no template "Organizar gastos" (40/10/15/20/15) sem
  fricção; ajuste posterior em linguagem natural. (RF-13a, EB-13)
- **DR-08 — Janela de `recent_turns` = 3 pares**: confirmado o tamanho atual do código. (RF-20)

## Suposições

- **Suposição**: as 5 categorias fixas (Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade
  Financeira) são estáveis e não configuráveis pelo usuário no onboarding.
- **Decisão (DR-09)**: "cartões coletados" é satisfeito por "Não uso" (lista vazia) — a conclusão
  NÃO exige cartão; o único gate de domínio é a 1ª transação (`HasFirstTransaction`). Confirma o doc
  (REGRA 4) e o comportamento atual do código.
- **Decisão (DR-10)**: cap de retry + dead-letter para os consumidores de saudação e WorkingMemory,
  reusando o suporte do outbox (`attempts`/`max_attempts` + backoff) e emitindo alerta/métrica
  `outbox_dead_letter_total{event_type}` — evita retry infinito silencioso.
- **Decisão (DR-11)**: `internal/card` passa a exigir apenas `closing_day` (`due_day` opcional,
  derivado no card); o onboarding envia só o dia de fechamento. (GAP-V1)

## Documentos de Origem

Este PRD foi derivado e deve ser rastreado contra:

- `docs/prompts/prd_onboarding_v2_prompt.md` — prompt enriquecido de entrada.
- `MeControla_Onboarding_V2.md` (referência de produto) — fonte canônica das 10 regras de negócio e
  dos formatos literais de mensagem; resultado esperado de 30–50% menos mensagens.
- `docs/plans/2026-06-23-onboarding-auto-start-llm-mandatory.md` — auto-start, LLM mandatório,
  saudação proativa via consumer.
- `docs/plans/2026-06-23-onboarding-persistencia-isolada-conclusao-deterministica-part-2.md` —
  persistência isolada e conclusão determinística.
- `docs/plans/2026-06-23-isolamento-agentes-gaps-persistencia-part-3.md` — mapa de persistência,
  gaps de isolamento e idempotência (origem da seção Análise de Lacunas).
- `.agents/skills/mastra/` e https://github.com/mastra-ai/mastra — inspiração dos primitivos
  Workflow, Tool, Thread, Run, WorkingMemory e Pending Step (suspend & resume).
