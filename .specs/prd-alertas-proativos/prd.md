# Documento de Requisitos do Produto (PRD) — Alertas Proativos

<!-- spec-version: 1 -->

> Fonte de user stories: `docs/us/2026-07-07-us-alertas-proativos.md` (US-01 a US-12).
> Data: 2026-07-08.
> Slug: `alertas-proativos`.

## Visão Geral

O MeControla acompanha o dinheiro do usuário via WhatsApp, mas hoje é majoritariamente
**reativo**: só fala quando o usuário fala primeiro. Isso permite abandono silencioso — o usuário
para de registrar, o orçamento perde aderência e o valor percebido cai antes de qualquer sinal.

Esta funcionalidade introduz um conjunto coeso de **alertas proativos** enviados pelo MeControla ao
usuário no WhatsApp, cobrindo o ciclo mensal completo: começo do mês (definir orçamento),
acompanhamento de limiares por categoria (80%, 90%, 100%), meta atingida, fechamento do mês,
reativação de quem parou de registrar e reforço motivacional de constância. Um orquestrador único
aplica prioridade e frequência para que o usuário receba a informação mais crítica sem ser
sobrecarregado.

O objetivo é manter o usuário ativo, reduzir o churn por abandono e reforçar a sensação de que
"o MeControla está acompanhando meu dinheiro comigo".

Esta entrega também **consolida** o subsistema de alertas de limiar: o fluxo legado reativo por
evento (`ThresholdEvaluator`/`AlertWorkflow`/`AlertRepository`/`EvaluateAlert` e o modo dual
`legacy`/`job`/`both`) será removido, unificando tudo no fluxo por job orquestrado.

## Objetivos

- **Métrica primária:** aumentar o **engajamento de registro** — número médio de lançamentos por
  usuário por mês, comparando o período antes e depois do lançamento dos alertas proativos.
  - **Alvo provisório:** **+15%** em lançamentos/usuário/mês em **90 dias** após o lançamento,
    medido sobre a coorte que optou por receber alertas (opt-in). O alvo é provisório e será
    recalibrado após o primeiro baseline real em produção.
- **Métricas secundárias:**
  - Taxa de reativação após alertas de retomada (US-08) e risco de abandono (US-09): % de usuários
    que voltam a interagir ou registrar em até 72h do alerta.
  - Redução da proporção de usuários assinantes ativos com 7+ dias de inatividade.
  - Taxa de entrega efetiva de template (enviado vs aceito pela Meta vs falha).
  - Taxa de opt-in nos alertas proativos (adoção) e taxa de opt-out após opt-in (percepção de spam).

## Histórias de Usuário

As histórias canônicas estão em `docs/us/2026-07-07-us-alertas-proativos.md`. Resumo mapeado a este
PRD (persona única: **usuário assinante do MeControla no WhatsApp**):

- US-01 — Início de mês: sem orçamento no mês vigente, receber convite para definir o orçamento.
- US-02 — 80% da categoria: aviso ao atingir 80% do planejado, com opção de detalhamento por
  subcategoria ao responder "sim".
- US-03 — 90% da categoria: aviso de atenção ao atingir 90% do planejado.
- US-04 — 100% da categoria: aviso de limite atingido/ultrapassado, com excedente.
- US-05 — Meta atingida: comemorar quando a categoria "Metas" atinge 100% do planejado.
- US-06 — Fechamento do mês: resumo planejado vs realizado por categoria no último dia do mês.
- US-07 — Motivação (constância): mensagem semanal de reforço para usuários ativos.
- US-08 — Retomada de uso: reativar quem ficou 3 dias sem registrar ou interagir.
- US-09 — Risco de abandono: reativar quem ficou 7+ dias sem interagir.
- US-10 — Orçamento não revisado: reforço no 3º dia do mês para quem segue sem orçamento.
- US-11 — Prioridade de envio: quando vários alertas são elegíveis no mesmo dia, respeitar ordem e
  enviar no máximo um por dia.
- US-12 — Regras gerais: categorias oficiais, tom de voz, não repetir o mesmo evento.

## Funcionalidades Core

1. **Alertas de ciclo de orçamento (início e reforço)** — US-01 e US-10: detectar ausência de
   orçamento no mês vigente e convidar o usuário a defini-lo, com um envio no início do mês e um
   reforço no 3º dia caso continue sem orçamento.

2. **Alertas de limiar por categoria (80/90/100)** — US-02, US-03, US-04: avaliar o consumo de cada
   categoria oficial contra o planejado e avisar ao cruzar cada limiar, uma vez por categoria por
   limiar por mês. O alerta de 80% oferece detalhamento por subcategoria mediante resposta "sim".

3. **Alerta de meta atingida** — US-05: comemorar quando o acumulado da categoria "Metas" alcança
   100% do planejado.

4. **Alerta de fechamento do mês** — US-06: no último dia do mês, enviar resumo comparativo
   (planejado vs realizado vs status) por categoria oficial, com ponte para o próximo ciclo.

5. **Alertas de engajamento e reativação** — US-07, US-08, US-09: reforço motivacional semanal para
   ativos, retomada após 3 dias e risco de abandono após 7 dias, apoiados em rastreamento de
   atividade do usuário.

6. **Orquestrador de prioridade e frequência** — US-11 e US-12: passo diário único que reúne todos
   os alertas elegíveis do usuário, aplica a ordem de prioridade e envia no máximo um alerta por dia,
   respeitando as regras de frequência individuais, opt-out e deduplicação por evento.

7. **Entrega compatível com a Meta (WhatsApp Business)** — todos os alertas são mensagens
   business-initiated entregues via **templates aprovados** (parametrizados), respeitando a regra da
   janela de 24h. Respostas do usuário (ex.: "sim" da US-02) são tratadas in-session pelo agente.

8. **Consolidação do subsistema de alertas** — remover o fluxo legado e o modo dual, unificando no
   fluxo por job orquestrado.

## Requisitos Funcionais

### Consolidação do legado

- RF-01: O sistema DEVE remover o fluxo legado de alertas de limiar do módulo `internal/budgets`
  (`ThresholdEvaluator`, `AlertWorkflow`, `AlertStateResolver`/`MaxDeliveredAlerts`,
  `AlertRepository`, use case `EvaluateAlert`, enum `valueobjects.Threshold`) e o modo dual
  `legacy`/`job`/`both`, unificando toda a avaliação de limiar no fluxo por job.
- RF-02: A remoção do legado NÃO PODE quebrar consumidores existentes; o sistema DEVE confirmar que
  nenhum consumidor depende do evento `budgets.expense.committed.v1` exclusivamente para gerar
  alertas, e DEVE ser acompanhada de testes de regressão.
- RF-03: Após a consolidação, a avaliação de limiares (80/90/100 e meta) DEVE ocorrer no passo
  orquestrado diário, não mais de forma reativa no commit da despesa.

### Limiares por categoria e meta

- RF-04: O sistema DEVE suportar múltiplos limiares por categoria — 80%, 90% e 100% do valor
  planejado — evoluindo a configuração de limiar de um único ratio por categoria para um conjunto
  ordenado de ratios.
- RF-05: Cada alerta de limiar de categoria DEVE ser enviado no máximo **uma vez por categoria, por
  limiar, por mês** (deduplicação por evento).
- RF-06: O alerta de 80% (US-02), 90% (US-03) e 100% (US-04) DEVE exibir os valores exigidos pela
  respectiva US: categoria, planejado, gasto e saldo restante; no caso de 100%, gasto atual e
  **excedente** (gasto − planejado).
- RF-07: O sistema DEVE tratar a categoria "Metas" atingindo 100% do planejado como o alerta de
  **meta atingida** (US-05), enviado no máximo uma vez por mês quando a condição for satisfeita,
  exibindo nome ("Metas"), valor da meta (planejado) e valor acumulado (aportado).
- RF-08: O limiar de meta DEVE ter default de **100%** (1.00), corrigindo o default atual de 0.50.

### Alertas de ciclo de orçamento

- RF-09: O sistema DEVE enviar o alerta de início de mês (US-01) quando, no início do mês vigente,
  **não existir orçamento cadastrado** para esse mês; não DEVE enviar quando já houver orçamento.
- RF-10: O sistema DEVE enviar o alerta de orçamento não revisado (US-10) como **reforço único no 3º
  dia do mês** quando o usuário continuar sem orçamento cadastrado/revisado para o mês vigente.
- RF-11: Os alertas RF-09 e RF-10 DEVEM ser enviados no máximo uma vez por mês cada.

### Fechamento do mês

- RF-12: O sistema DEVE enviar o alerta de fechamento do mês (US-06) **no último dia do mês**, uma
  vez por mês, com o comparativo **planejado vs realizado vs status** para cada categoria oficial
  (Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira), reutilizando os dados de resumo
  mensal existentes.
- RF-13: O campo "status" por categoria DEVE usar três estados fechados derivados da relação
  realizado/planejado: **"Dentro do planejado"** (realizado ≤ 95% do planejado), **"No limite"**
  (realizado entre 95% e 105% do planejado, inclusive) e **"Ultrapassado"** (realizado > 105% do
  planejado). A faixa de "No limite" (95–105%) é a definição default.

### Rastreamento de atividade e reativação

- RF-14: O sistema DEVE persistir dois carimbos de atividade por usuário: `last_interaction_at`
  (qualquer mensagem inbound recebida no WhatsApp) e `last_registration_at` (qualquer lançamento de
  gasto registrado).
- RF-15: O sistema DEVE atualizar `last_interaction_at` a cada inbound processado e
  `last_registration_at` a cada registro de lançamento efetivado.
- RF-16: O alerta de retomada de uso (US-08) DEVE ser disparado quando o usuário assinante ativo
  ficar **3 dias** sem registrar gasto **e** sem interagir (usando o mais recente entre
  `last_registration_at` e `last_interaction_at`), no máximo **1 vez por semana**.
- RF-17: O alerta de risco de abandono (US-09) DEVE ser disparado quando o usuário ficar **7 dias ou
  mais** sem interagir (`last_interaction_at`), no máximo **1 vez por mês**.
- RF-18: O alerta de motivação (US-07) DEVE ser enviado **1 vez por semana** apenas para usuários
  **ativos**, definidos como quem registrou ≥ 1 gasto nos últimos 7 dias (`last_registration_at`).
- RF-19: A elegibilidade a qualquer alerta proativo DEVE exigir, cumulativamente: (a) consentimento
  ativo do usuário (opt-in — ver RF-34) e (b) assinatura **ativa para cobrança** (Active, Past Due,
  Canceled Pending). Usuários sem opt-in ou sem assinatura ativa NÃO são elegíveis.

### Orquestração, prioridade e frequência

- RF-20: O sistema DEVE executar um **passo diário único** (hora configurável, default 09:00
  America/Sao_Paulo) que reúne todos os alertas elegíveis por usuário, aplica prioridade e envia.
- RF-21: Quando dois ou mais alertas forem elegíveis para o mesmo usuário no mesmo dia, o sistema
  DEVE enviar **no máximo um alerta por usuário por dia**, escolhendo o de maior prioridade segundo a
  ordem: (1) 100% categoria, (2) 90% categoria, (3) meta atingida, (4) orçamento não revisado,
  (5) início de mês, (6) fechamento do mês, (7) 80% categoria, (8) retomada de uso, (9) risco de
  abandono, (10) motivação.
- RF-22: Alertas suprimidos por prioridade em um dia PERMANECEM elegíveis em dias seguintes, sujeitos
  às suas regras de frequência individuais; a supressão diária não consome a cota de frequência do
  alerta suprimido.
- RF-23: O sistema NÃO DEVE enviar alertas repetidos sobre o mesmo evento (deduplicação por
  identidade do evento: usuário, tipo de alerta e período de referência).
- RF-24: O sistema DEVE permitir que o usuário **desligue e religue** os alertas proativos a
  qualquer momento por comando no WhatsApp; quando desligado, o orquestrador NÃO DEVE enviar nenhum
  alerta proativo até religamento. O estado de consentimento (opt-in/opt-out) é único e global por
  usuário (ver RF-34 a RF-37).

### Entrega WhatsApp e interação

- RF-25: Todos os alertas proativos DEVEM ser entregues como **mensagens business-initiated via
  template aprovado** da Meta (parametrizado com os valores da respectiva US), respeitando a regra
  da janela de 24h; o sistema NÃO DEVE depender de texto livre para o disparo proativo.
- RF-26: O detalhamento por subcategoria da US-02 DEVE ser tratado **in-session**: quando o usuário
  responde "sim" (dentro da janela de 24h aberta pela resposta), o MeControlaAgent DEVE retornar o
  detalhamento por subcategoria (subcategoria + valor gasto no mês) da categoria alertada, via uma
  nova tool fina do agente (adapter que delega a um use case de consulta, sem regra de negócio na
  tool). O detalhamento PODE ser ordenado do maior para o menor gasto.
- RF-27: As CTAs dos alertas (ex.: "Qual seu orçamento?", "Quer ver seu orçamento completo?",
  "Quer criar a próxima meta agora?", "Quer que eu monte a base do próximo mês com esses dados?")
  DEVEM ser resolvidas pelas capacidades já existentes do agente/onboarding quando o usuário
  responde; este PRD NÃO introduz seeding automático de orçamento nem criação guiada de meta.
- RF-28: Respostas in-session DEVEM sair formatadas para WhatsApp (negrito com `*`, sem `**`),
  reutilizando a normalização de formatação existente.

### Copy, tom de voz e categorias

- RF-29: Os textos dos alertas DEVEM aplicar a **copy oficial** de cada US (parametrizada nos
  templates), substituindo os textos genéricos atuais.
- RF-30: Os alertas de categoria DEVEM considerar **apenas** as categorias oficiais: Custo Fixo,
  Conhecimento, Prazeres, Metas, Liberdade Financeira.
- RF-31: Todos os alertas DEVEM seguir o tom de voz oficial: simples, humano, leve, objetivo, sem
  julgamento, sempre com uma chamada para próxima ação.

### Observabilidade e frequência técnica

- RF-32: Cada avaliação e envio de alerta DEVE ser observável (métricas e logs) com cardinalidade
  controlada — labels permitidos são enums fechados (ex.: tipo de alerta, categoria oficial, status
  de envio, outcome); NÃO DEVE usar `user_id` ou `category_id` como label de métrica.
- RF-33: As janelas de frequência ("1x por semana", "1x por mês", "1x por dia") DEVEM ser avaliadas
  em base de **calendário civil** no fuso America/Sao_Paulo — semana de **segunda a domingo** e mês
  civil. Não são janelas deslizantes.

### Consentimento e opt-in (Meta business-initiated)

- RF-34: Os alertas proativos DEVEM ser **opt-in explícito, desligados por default**. Nenhum alerta
  proativo pode ser enviado a um usuário que não tenha registrado consentimento ativo.
- RF-35: Para **novos usuários**, o sistema DEVE capturar o consentimento como parte do fluxo de
  onboarding existente.
- RF-36: Para a **base de usuários já existente** (que não passou pelo onboarding com consentimento),
  o sistema DEVE enviar **um único template de consentimento** perguntando se a pessoa aceita receber
  os alertas; o opt-in só é registrado mediante resposta afirmativa. O template de consentimento não
  DEVE ser reenviado repetidamente para quem não respondeu ou recusou.
- RF-37: O usuário DEVE poder **ligar e desligar** os alertas a qualquer momento por comando no
  WhatsApp; o estado de consentimento DEVE ser persistido e consultado pelo orquestrador antes de
  qualquer envio (RF-19, RF-24).

## Experiência do Usuário

- **Canal único:** WhatsApp. Todos os alertas chegam como mensagem do MeControla no WhatsApp do
  usuário.
- **Ritmo controlado:** no máximo um alerta proativo por dia; frequências semanais/mensais evitam
  repetição. O usuário nunca recebe dois alertas concorrentes no mesmo dia.
- **Diálogo natural:** alertas terminam com uma pergunta/CTA; ao responder, o usuário conversa
  normalmente com o agente (que já resolve orçamento, categorias, consultas). A US-02 abre um
  detalhamento por subcategoria ao responder "sim".
- **Controle do usuário:** alertas proativos são **opt-in, desligados por default**. O usuário
  consente no onboarding (novos) ou por um único template de consentimento (base atual), e pode
  **ligar/desligar** a qualquer momento por comando no WhatsApp.
- **Tom:** mensagens acolhedoras, sem julgamento, reforçando parceria ("estou acompanhando seu
  dinheiro com você").

## Restrições Técnicas de Alto Nível

- **Regra da Meta (janela de 24h):** fora da janela de 24h desde a última mensagem do usuário, só é
  permitido enviar templates aprovados; texto livre é bloqueado. Alertas proativos são, por
  natureza, fora da janela. **Dependência externa:** cada tipo de alerta exige um template aprovado
  pela Meta antes de entrar em produção.
- **Classificação de template pela Meta:** alertas de reforço/reativação/motivação (US-07/08/09)
  podem ser classificados pela Meta como categoria **Marketing** (sujeita a opt-out e às
  preferências de marketing do usuário), enquanto alertas de conta/orçamento tendem a **Utility**.
  Isso pode afetar a entregabilidade e é um risco a validar na aprovação dos templates.
- **Ausência de rastreamento de atividade:** o produto não possui hoje `last_seen`/`last_activity`;
  US-07/08/09 exigem introduzir persistência de atividade (dois carimbos) e atualizá-la nas
  fronteiras de inbound e de registro.
- **Substrato existente:** reutilizar `internal/platform/notification` (`ChannelGateway`),
  `internal/platform/worker/job` (padrão `job.NewAdapter`/`WithTimeout`), o resumo mensal existente
  (`GetMonthlySummary`) e o substrato agentivo (`internal/platform/{agent,tool,memory,workflow}`)
  para a tool de detalhamento da US-02. Não recriar primitivos de plataforma.
- **Governança de código:** adaptadores finos e zero comentários (R-ADAPTER-001); tools sem regra de
  negócio/SQL/branching de domínio; regra de negócio em `domain/services` como decisões puras
  (DMMF); estados de alerta como tipos fechados; sem `init()`; `context.Context` em toda fronteira
  de IO.
- **Fuso horário:** todos os gatilhos por data avaliados em America/Sao_Paulo; competência = mês
  civil (sem dia de fechamento configurável).
- **Idempotência/frequência:** deduplicação por `(usuário, tipo, período de referência)` persistida,
  reaproveitando o padrão de `budget_alerts_sent` do fluxo novo, estendido aos novos tipos de alerta.

## Fora de Escopo

- Metas individuais nomeadas com alvo e acumulado próprios (novo agregado/tabela). A US-05 usa a
  categoria "Metas" agregada.
- Preferências **granulares por tipo** de alerta (o consentimento é único e global — liga/desliga
  todos; granularidade por tipo fica como evolução futura).
- Seeding automático do orçamento do próximo mês a partir do fechamento e criação guiada de meta —
  as CTAs são resolvidas pelas capacidades já existentes do agente.
- Dia de fechamento de competência configurável por usuário.
- Canais além do WhatsApp (e-mail, push, SMS).
- Envio intraday/near-real-time de alertas de limiar (o disparo é diário no passo orquestrado).
- Aprovação e gestão do ciclo de vida dos templates na Meta (processo operacional externo; o PRD
  apenas o declara como dependência).

## Decisões Consolidadas

Todos os pontos de divergência foram resolvidos em cinco rodadas de esclarecimento. Registro das
decisões que orientam a especificação técnica (nenhuma é ressalva — são escolhas fechadas):

- D1 — **Entrega:** exclusivamente por WhatsApp, via templates aprovados parametrizados por tipo de
  alerta (regra da janela de 24h da Meta). Nenhum outro canal nesta versão. (RF-25)
- D2 — **Consolidação:** o fluxo legado de threshold e o modo dual são removidos neste PRD. (RF-01)
- D3 — **Meta atingida (US-05):** categoria "Metas" a 100% do planejado; sem metas nomeadas. (RF-07)
- D4 — **Prioridade:** no máximo 1 alerta/dia/usuário, o de maior prioridade. (RF-21)
- D5 — **Atividade:** dois carimbos (`last_interaction_at`, `last_registration_at`). (RF-14)
- D6 — **Tempo:** mês civil em America/Sao_Paulo; semana civil segunda a domingo; janelas não
  deslizantes. (RF-33)
- D7 — **US-01/US-10:** US-01 no 1º dia do mês, US-10 (reforço) no 3º dia, ambos condicionados à
  ausência de orçamento no mês vigente. (RF-09, RF-10)
- D8 — **Detalhamento US-02:** in-session via nova tool fina do agente. (RF-26)
- D9 — **CTAs:** resolvidas por capacidades existentes do agente/onboarding; sem seeding automático
  nem criação guiada de meta. (RF-27)
- D10 — **Cadência:** passo orquestrador diário único, hora configurável, default 09:00
  America/Sao_Paulo. (RF-20)
- D11 — **Usuário ativo (US-07):** registrou ≥ 1 gasto nos últimos 7 dias. (RF-18)
- D12 — **Status de fechamento (US-06):** três estados fechados com faixa "No limite" 95–105%.
  (RF-13)
- D13 — **Métrica primária:** engajamento de registro, alvo provisório +15% em 90 dias, revisável
  pós-baseline.
- D14 — **Consentimento:** opt-in explícito, default desligado; captura no onboarding (novos) + um
  único template de consentimento (base existente) + comando liga/desliga. (RF-34 a RF-37)
- D15 — **Deduplicação:** reaproveita o padrão da tabela `budget_alerts_sent`, estendido aos novos
  tipos de alerta e períodos de referência (detalhamento de schema na especificação técnica).

**Questões em aberto:** nenhuma. O PRD está fechado para handoff à especificação técnica.
