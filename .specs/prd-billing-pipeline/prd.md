# Documento de Requisitos do Produto (PRD) — E2: Billing Pipeline

<!-- spec-version: 1 -->

> **Origem:** Épico **E2 — `billing-pipeline`** (`docs/epics/epic-02-billing-pipeline.md`).
> **Discovery base:** `docs/discoveries/discovery-billing-hotmart-kiwify.md`.
> **Run de seleção:** `docs/runs/2026-06-05-next-prd-billing-pipeline.md` (decisões de negócio consolidadas em 4 rodadas com o time de produto em 2026-06-05).
> **Posição no roadmap:** segunda fatia do MVP. Bloqueado por E1 (`identity-foundation`) apenas para execução; PRD/techspec podem ser escritos em paralelo. Bloqueia E4 (`reconciliation-hardening`).
> **Próxima skill:** `create-technical-specification`.

---

## Visão Geral

A landing `mecontrola.app.br` promete planos pagos recorrentes (Mensal, Trimestral e Anual). Sem este pipeline, o produto não consegue saber se um usuário pagou, não consegue manter sua assinatura ao longo do tempo e não consegue bloquear acesso de quem não está em dia. Toda mensagem do usuário acaba consumindo recursos do bot sem que o produto tenha qualquer noção de direito de uso.

A funcionalidade entrega o **ciclo de vida da assinatura** ponta a ponta, sob o ponto de vista do negócio: a partir do momento em que uma compra é aprovada na plataforma de pagamento (Kiwify), o sistema passa a manter o estado canônico da assinatura daquele usuário, reage automaticamente a renovações, atrasos, cancelamentos e reembolsos, e responde com decisão única e auditável sobre "este usuário tem direito ao serviço agora?". Esse direito é consumido pelo restante do produto (uso do bot, gate de funcionalidades pagas, futura ativação de onboarding em E3).

Em produto, o valor é direto: **destravar receita recorrente e tornar o gate de uso confiável**. A fonte autoritativa do estado de cobrança é Kiwify; o sistema reflete e respeita o que Kiwify diz, em janela curta o suficiente para que o usuário pagante não sinta atrito e o usuário inadimplente seja bloqueado dentro do que o negócio considera tolerável.

### Por que agora

1. **Pré-requisito de receita.** Sem E2, o produto não tem como cobrar nem como reconhecer pagamento.
2. **Bloqueia E3.** O onboarding por magic token (E3) depende do estado `PAID` produzido pelo processamento do webhook de E2; sem ele, a ativação no WhatsApp não tem entrada.
3. **Identity já entrega o gate.** A função pura `IsEntitled(sub, now)` está implementada em E1 aguardando o agregado `Subscription` que este PRD especifica.
4. **Premissa crítica validada.** A hipótese H7 — propagação do token de funil `?s={token}` no webhook Kiwify — foi confirmada em compra real R$1 Pix em sandbox em 2026-06-05; o desenho de negócio fica firmado nessa base.

---

## Objetivos

1. **Reconhecer cada compra paga** originada do funil próprio dentro de janela operacional curta, transformando-a em uma assinatura ativa associada ao usuário correto.
2. **Manter o estado da assinatura coerente ao longo do tempo** reagindo a renovações, atrasos, cancelamentos, reembolsos e estornos sem intervenção manual no MVP.
3. **Entregar uma decisão única e determinística de direito de acesso** ("tem direito? por quê?") consumível pelo restante do produto, sem permitir que decisões de gate vivam em mais de um lugar.
4. **Garantir que o que está no sistema bate com Kiwify**, em janela horária, com a regra clara de que **Kiwify é a fonte de verdade** em caso de divergência.
5. **Não cobrar duas vezes nem aplicar o mesmo evento duas vezes**: mesmo evento entregue várias vezes pela plataforma de pagamento produz exatamente um efeito no estado da assinatura.

### Métricas de sucesso (mensuráveis)

- **M-01:** Taxa de aprovação de checkout — % das compras iniciadas no funil próprio que chegam ao estado `ACTIVE`. Meta inicial: ≥ 95% (calibração após 4 semanas em produção).
- **M-02:** Taxa de renovação por plano (Mensal, Trimestral, Anual) — % das assinaturas que renovam no fim do período vigente sem cair para `PAST_DUE` ou `EXPIRED`.
- **M-03:** Churn mensal — % de assinaturas ativas no início do mês que saíram (`CANCELED_PENDING` no período, `EXPIRED` ou `REFUNDED`) no fim do mês.
- **M-04:** Latência p95 de processamento de webhook — tempo entre o recebimento do evento pela plataforma de pagamento e a refletida no direito de acesso do usuário. Meta inicial: ≤ 30 segundos p95.

---

## Histórias de Usuário

Os atores deste PRD são o **assinante**, o **sistema billing** (consumidor interno: bot do WhatsApp e demais módulos que precisam saber se o usuário tem direito) e o **suporte**.

- **Como** assinante, **quero** que minha compra aprovada na Kiwify libere meu acesso ao bot do WhatsApp dentro de poucos minutos, **para que** eu comece a usar o produto sem precisar abrir chamado.
- **Como** assinante, **quero** que minha renovação seja reconhecida automaticamente, **para que** eu não perca acesso ao virar o período do plano.
- **Como** assinante em atraso, **quero** continuar com acesso por uma janela curta após o vencimento, **para que** eu tenha tempo de regularizar sem ser bloqueado imediatamente.
- **Como** assinante que cancelou, **quero** continuar com acesso até o fim do período já pago, **para que** eu não perca o que paguei pelo cancelamento.
- **Como** assinante reembolsado, **espero** perder o acesso imediatamente, e o produto deve assumir essa regra como inegociável para proteger receita.
- **Como** sistema billing (consumidor interno), **quero** uma decisão única sobre "este usuário tem direito agora?" com a razão associada, **para que** todo lugar do produto que precisa fazer gate use a mesma fonte sem replicar regra de negócio.
- **Como** time de operações, **quero** que o sistema esteja em sincronia com Kiwify em janela horária, **para que** divergências de estado não viralizem.
- **Como** suporte, **quero** que cada evento de cobrança recebido tenha efeito previsível, **para que** eu possa responder ao usuário sem precisar reprocessar manualmente eventos duplicados.

---

## Funcionalidades Core

### F-01. Ciclo de vida da assinatura

- **O que faz:** mantém, para cada usuário, o estado da sua assinatura ao longo do tempo (vigente, em atraso, cancelada, expirada, reembolsada), com o período atual de cobertura.
- **Por que é importante:** sem estado de assinatura, o produto não tem como decidir nada sobre acesso pago.
- **Como funciona em alto nível:** o sistema reage aos sinais que recebe da plataforma de pagamento e move a assinatura entre cinco estados de negócio. Cada plano (Mensal, Trimestral, Anual) tem seu próprio período de vigência, que é estendido a cada renovação.

### F-02. Decisão única de direito de acesso

- **O que faz:** responde, em qualquer ponto do produto que precisar, à pergunta "este usuário tem direito ao serviço agora?", retornando também a razão (vigente, em graça, expirado, cancelado dentro do período, reembolsado, sem assinatura).
- **Por que é importante:** evita que regras de gate vivam espalhadas pelo produto. Centraliza a verdade.
- **Como funciona em alto nível:** a decisão deriva determinísticamente do estado atual da assinatura e do momento da consulta. É a mesma decisão que será consumida pelo bot do WhatsApp e por qualquer funcionalidade paga.

### F-03. Captura confiável de eventos de cobrança

- **O que faz:** recebe os eventos de cobrança vindos da plataforma de pagamento e aplica seu efeito sobre a assinatura, mesmo que o mesmo evento chegue múltiplas vezes ou fora de ordem.
- **Por que é importante:** plataformas de pagamento entregam eventos com garantia "ao menos uma vez", podem repetir e podem chegar fora de ordem. Sem tratamento adequado, isso vira cobrança duplicada, estado inconsistente ou perda de eventos.
- **Como funciona em alto nível:** cada evento é identificado pelo seu identificador único e tem um efeito específico (criar, renovar, marcar atraso, marcar cancelamento, marcar reembolso). Repetições do mesmo evento produzem exatamente o mesmo resultado.

### F-04. Sincronia horária com a plataforma de pagamento

- **O que faz:** em janela horária, compara o que o sistema acredita ser o estado da assinatura com o que a plataforma de pagamento (Kiwify) diz, e ajusta o estado local em caso de divergência.
- **Por que é importante:** webhooks podem se perder; sem reconciliação, divergências viram dívida operacional. Kiwify é a fonte de verdade.
- **Como funciona em alto nível:** em intervalo regular o sistema consulta o estado real das assinaturas em Kiwify e corrige o estado local se houver discrepância. Em qualquer divergência, vence Kiwify.

### F-05. Notificação ao assinante em transições relevantes

- **O que faz:** quando a assinatura muda de estado em transições que afetam o usuário (entrou em atraso, perdeu acesso, foi reembolsada), o sistema tenta avisar o usuário via WhatsApp.
- **Por que é importante:** o usuário precisa entender por que perdeu (ou está prestes a perder) o acesso.
- **Como funciona em alto nível:** notificação é **best-effort no MVP**. Falha de envio é tolerada e não bloqueia nem reverte a mudança de estado. O esforço de garantia de entrega fica para hardening pós-MVP.

---

## Requisitos Funcionais

Cada requisito está ancorado em uma decisão consolidada do épico, da discovery ou do run de seleção em `docs/runs/2026-06-05-next-prd-billing-pipeline.md`.

### Captação e elegibilidade da compra

- **RF-01:** O sistema deve aceitar como entrada apenas eventos de cobrança originados da plataforma de pagamento **Kiwify**. Outras plataformas (Hotmart, Stripe, Pagar.me, Asaas etc.) estão fora do escopo deste PRD e exigem PRD próprio.
- **RF-02:** O sistema deve oferecer três planos pagos no MVP: **Mensal**, **Trimestral** e **Anual**, cada um com seu período de vigência próprio (respectivamente trinta, noventa e trezentos e sessenta e cinco dias). Não há trial gratuito no MVP.
- **RF-03:** O sistema deve **rejeitar** qualquer evento de compra que não contenha o token do funil de onboarding (`?s={token}`). Compras sem token são consideradas fora do funil próprio e **não** geram assinatura. O token é a chave que liga a compra ao usuário do WhatsApp via E3.

### Estados da assinatura

- **RF-04:** Cada assinatura deve estar, a qualquer momento, em exatamente um dos seguintes estados de negócio: `ACTIVE`, `PAST_DUE`, `CANCELED_PENDING`, `EXPIRED`, `REFUNDED`. O estado `TRIALING` é reservado para uso futuro e permanece sem caller no MVP.
- **RF-05:** `ACTIVE` significa assinatura vigente e dentro do período pago. O assinante tem direito de acesso.
- **RF-06:** `PAST_DUE` significa que a Kiwify sinalizou atraso no pagamento. O assinante deve continuar com direito de acesso por uma **janela de graça de três (3) dias corridos** contados a partir da entrada em `PAST_DUE`. Após esse prazo, perde o direito de acesso (continuando em `PAST_DUE` até que outro evento mude o estado).
- **RF-07:** `CANCELED_PENDING` significa cancelamento solicitado pelo usuário diretamente na plataforma de pagamento. O assinante mantém direito de acesso até o fim do período já pago (`period_end`); após esse momento, perde o acesso.
- **RF-08:** `EXPIRED` significa que a assinatura chegou ao fim do período pago sem renovação. Sem direito de acesso.
- **RF-09:** `REFUNDED` cobre tanto reembolso quanto estorno (chargeback) — os dois eventos produzem o mesmo efeito de negócio: revogação imediata do direito de acesso, independente de período remanescente.

### Reação aos eventos de cobrança

- **RF-10:** O sistema deve reagir aos seguintes eventos vindos da Kiwify e somente a eles no MVP:
  - **compra aprovada** → cria a assinatura e a marca como `ACTIVE`, com período de vigência conforme o plano.
  - **renovação** → estende o período de vigência da assinatura existente conforme o plano.
  - **atraso de pagamento** → marca a assinatura como `PAST_DUE` e inicia a janela de graça de três dias.
  - **cancelamento** → marca a assinatura como `CANCELED_PENDING`.
  - **reembolso / estorno** → marca a assinatura como `REFUNDED`.
- **RF-11:** O sistema deve garantir **idempotência por identificador único de evento**: o mesmo evento entregue pela plataforma de pagamento N vezes (por retry, replay ou falha de rede) produz exatamente um único efeito sobre a assinatura. Esta é uma garantia de negócio explícita; auditoria e relatórios contábeis dependem dela.
- **RF-12:** O sistema deve aplicar corretamente eventos entregues **fora de ordem cronológica**, sem regredir o estado já alcançado. Por exemplo: se uma renovação chega antes da confirmação da compra original, o sistema não pode terminar em estado inconsistente.

### Decisão de direito de acesso

- **RF-13:** O sistema deve oferecer uma decisão única, determinística e consultável de "este usuário tem direito de acesso agora?", retornando junto a razão da decisão (vigente, em graça, expirado, cancelado dentro do período, reembolsado, sem assinatura).
- **RF-14:** A regra desta decisão é a única fonte de verdade para o gate de uso pago no produto. Toda funcionalidade paga consulta essa decisão; não há regra de gate duplicada em outro lugar do produto.
- **RF-15:** A decisão deve ser **rápida o bastante para servir qualquer interação com o bot** sem perceptível atraso para o usuário, em condições normais de carga.

### Vinculação assinatura ↔ usuário

- **RF-16:** Cada assinatura criada deve ser **vinculada a um e apenas um usuário** identificado a partir do token do funil de onboarding presente na compra. A vinculação ao usuário do WhatsApp efetivo é fechada por E3 (onboarding); E2 entrega a assinatura na forma "presa ao token" para que E3 a consuma ao ativar.
- **RF-17:** Um mesmo usuário deve ter, a qualquer momento, **no máximo uma assinatura ativa simultaneamente**. Tentar criar uma segunda assinatura ativa para o mesmo usuário é erro de negócio e deve ser tratado como caso de exceção operacional. Plano família, plano equipe ou múltiplas assinaturas paralelas estão fora do escopo deste PRD e exigem PRD próprio.

### Sincronia com a plataforma de pagamento

- **RF-18:** O sistema deve, em intervalo **horário**, comparar o estado local das assinaturas com o estado real reportado pela API da Kiwify, e corrigir o estado local quando houver divergência. **Kiwify vence sempre** — não há intervenção manual nesse fluxo no MVP.
- **RF-19:** O sweep diário full (recomputação dos últimos 90 dias) e o dashboard de MRR/churn não fazem parte deste PRD e ficam para o épico de hardening pós-MVP (E4).

### Comunicação ao assinante

- **RF-20:** O sistema deve, em **regime best-effort**, notificar o assinante via WhatsApp nas transições de estado que afetam o seu acesso (`ACTIVE → PAST_DUE`, `PAST_DUE → EXPIRED`, transição para `REFUNDED`). Falha de envio é tolerada e **não** impede, atrasa ou reverte a mudança de estado da assinatura. **Nota de implementação (ADR-005 + bugfix 2026-06-09):** a transição `PAST_DUE → EXPIRED` é materializada por um job dedicado de expiração de graça (`ExpireGraceJob`, cron `@every 30m` por padrão) que percorre assinaturas com `status='PAST_DUE' AND grace_end < now()`, aplica `StatusExpired` em transação e publica o evento outbox `billing.subscription.expired_after_grace`. Esse evento aciona o `NotificationHandler` de expiração. O gate de acesso continua usando `IsEntitled(sub, now)` (decisão runtime, ADR-005), portanto o usuário perde acesso ao terminar a graça mesmo antes do job rodar.

### Relação com o gate de uso

- **RF-21:** O PRD declara apenas a regra binária "tem direito / não tem direito". A whitelist de comandos administrativos que podem ser executados pelo usuário **sem direito de acesso vigente** (ex.: `ATIVAR`, `/ajuda`, `/cancelar`, `/contato`) é responsabilidade do PRD de E3 (onboarding) e não entra neste documento.

---

## Restrições Técnicas de Alto Nível

Estas são restrições de produto, não decisões de desenho:

- **Kiwify como única fonte de verdade.** Estado divergente é sempre corrigido para o que Kiwify reporta. Não há tela ou processo de "ignorar Kiwify" no MVP.
- **Origem controlada das compras.** Apenas compras iniciadas pelo funil próprio (com token de onboarding presente) são aceitas. Compras feitas fora do funil controlado, mesmo que pagas legitimamente, **não** geram assinatura no MVP. Essa restrição é uma escolha consciente para garantir vinculação confiável ao usuário do WhatsApp.
- **Janela operacional de reconciliação:** o atraso máximo aceitável entre o que Kiwify reporta e o que o sistema reflete é de **uma (1) hora**.
- **Notificação ao assinante é best-effort.** O produto aceita silenciar transições em casos de falha de canal de comunicação no MVP. Garantia de entrega é hardening pós-MVP.
- **Idempotência como requisito de auditoria.** O sistema **não pode** aplicar duas vezes o mesmo evento de cobrança. Esta é uma garantia para o usuário (sem cobrança duplicada) e para o suporte (sem reprocessamento manual).
- **Compatibilidade com Identity (E1).** A definição de estados de assinatura deste PRD permanece compatível com a função pura de decisão de direito já entregue em E1 (cinco estados ativos no MVP + estado reservado para trial futuro).
- **LGPD básico.** O fato de o sistema persistir vínculo entre dado pessoal (telefone, e-mail) e cobrança herda as obrigações de privacidade já estabelecidas em E1 (soft delete). Operações específicas de anonimização programada e LGPD ficam para E4.

---

## Fluxos e cenários de negócio

### Fluxo 1 — Primeira compra aprovada

1. O assinante completa a compra na plataforma de pagamento dentro do funil próprio (token de onboarding presente).
2. A plataforma envia o evento de compra aprovada ao sistema.
3. O sistema cria a assinatura, define o período de vigência conforme o plano e marca o estado como `ACTIVE`.
4. A assinatura fica vinculada ao token do funil, aguardando o ato de ativação que será feito em E3 (vincular ao usuário do WhatsApp).
5. A partir do momento em que E3 vincula a assinatura ao usuário do WhatsApp, qualquer consulta do produto pelo direito de acesso desse usuário retornará "tem direito".

### Fluxo 2 — Renovação

1. A plataforma de pagamento sinaliza a renovação no fim do período vigente.
2. O sistema estende o período de vigência conforme o plano.
3. O estado da assinatura permanece `ACTIVE`. Direito de acesso preservado.

### Fluxo 3 — Atraso e graça

1. A plataforma sinaliza atraso no pagamento.
2. O sistema marca a assinatura como `PAST_DUE` e abre a janela de graça de três dias corridos.
3. Durante esses três dias, o assinante mantém direito de acesso; o sistema tenta notificá-lo via WhatsApp (best-effort).
4. Se nenhum novo evento ocorrer até o fim da janela de graça, o assinante perde o direito de acesso. O estado permanece `PAST_DUE` (não vira automaticamente outro estado por passagem do tempo) — qualquer evolução futura virá de novo evento da plataforma.

### Fluxo 4 — Cancelamento solicitado pelo usuário

1. O usuário solicita o cancelamento **diretamente na plataforma de pagamento** (o produto não oferece comando de cancelamento via WhatsApp no MVP).
2. A plataforma sinaliza o cancelamento ao sistema.
3. O sistema marca a assinatura como `CANCELED_PENDING`.
4. O assinante mantém direito de acesso até o fim do período já pago.
5. Quando o período acaba e nenhuma renovação ocorre, o assinante perde o direito de acesso.

### Fluxo 5 — Reembolso ou chargeback

1. A plataforma sinaliza reembolso ou estorno.
2. O sistema marca a assinatura como `REFUNDED`.
3. O assinante perde o direito de acesso **imediatamente**, independente de período remanescente.
4. O sistema tenta notificá-lo via WhatsApp (best-effort).

### Fluxo 6 — Reconciliação horária

1. Em intervalo horário, o sistema consulta a plataforma de pagamento sobre o estado real das assinaturas.
2. Para cada divergência detectada, o sistema corrige o estado local de modo a refletir o que a plataforma reporta.
3. Não há intervenção manual nem revisão humana nesse fluxo no MVP.

### Cenários de borda explicitamente cobertos

- **Mesmo evento entregue múltiplas vezes:** o sistema produz exatamente um efeito (RF-11).
- **Eventos fora de ordem:** o estado final é correto sem regressão (RF-12).
- **Compra sem token de funil:** rejeitada; nenhuma assinatura é criada (RF-03).
- **Tentativa de segunda assinatura ativa para o mesmo usuário:** tratada como erro de negócio (RF-17).
- **Reembolso de assinatura cancelada que ainda estava no período pago:** prevalece `REFUNDED`; perde acesso imediatamente.
- **Falha de notificação ao assinante:** tolerada, sem impacto no estado da assinatura (RF-20).

---

## Dependências externas (do ponto de vista de negócio)

- **Plataforma de pagamento Kiwify.** Conta operacional, planos cadastrados (Mensal, Trimestral, Anual), webhook configurado para entregar os cinco tipos de evento listados em RF-10, e API disponível para a consulta horária de reconciliação (RF-18). Limite de taxa da API é conhecido e respeitado pelos processos de reconciliação.
- **Canal de comunicação com o assinante (WhatsApp Business).** Necessário apenas para a notificação best-effort de transições (RF-20). Indisponibilidade do canal não bloqueia o ciclo de vida da assinatura.
- **Módulo de identidade (E1).** Já entrega o agregado `User` e a função pura de decisão de direito de acesso consumida por este PRD. Para execução das tarefas derivadas deste PRD, E1 precisa estar implementado em produção; a redação do PRD e da especificação técnica podem ocorrer em paralelo.
- **Funil de onboarding (E3) — relação dirigida.** Este PRD entrega a assinatura "presa ao token de funil". A ativação ao usuário do WhatsApp efetivo é responsabilidade de E3. Sem E3 implementado, este pipeline funciona até o estado `ACTIVE` mas a vinculação ao usuário do WhatsApp não fecha.

---

## Fora de Escopo

Para gestão clara de escopo, este PRD **não** trata:

- Suporte a qualquer outra plataforma de pagamento (Hotmart, Stripe, Pagar.me, Asaas, Lemon Squeezy, Paddle etc.).
- Período de trial gratuito.
- Cancelamento iniciado pelo assinante via WhatsApp (`/cancelar` ou equivalente). O assinante cancela na plataforma de pagamento; o sistema reage ao evento.
- Override administrativo de direito de acesso (suporte conceder acesso por exceção). Fica para hardening pós-MVP (E4).
- Whitelist de comandos administrativos sem direito de acesso (ex.: `ATIVAR`, `/ajuda`, `/cancelar`, `/contato`). Pertence a E3.
- Anonimização programada de dados pessoais para LGPD (rotinas periódicas). Fica para E4.
- Plano família, plano equipe ou plano com múltiplas linhas paralelas.
- Múltiplas assinaturas ativas simultâneas por usuário.
- Reconciliação diária full retroativa dos últimos 90 dias. Fica para E4.
- Dashboard de MRR e churn em tempo real. Fica para E4.
- Garantia de entrega das notificações ao assinante (replay, fila com retry forte). MVP é best-effort.
- Comando administrativo `is_admin` no agregado de usuário (proibido por decisão do PRD de E1 em 2026-06-05).
- Painel administrativo web para suporte realizar reembolso, override, replay de evento.
- Rate limit por usuário no consumo do bot.
- Operações de auditoria contábil avançadas (relatórios fiscais, conciliação tributária).

---

## Suposições e Questões em Aberto

### Premissas firmes

- **P-01 (H7):** A Kiwify propaga o token de funil (`?s={token}`) nos eventos de webhook de compra. Validada por compra real R$1 Pix em sandbox em 2026-06-05.
- **P-02:** A regra "um usuário = uma assinatura ativa simultânea" é inegociável no MVP, herdada do bundle de discovery do produto.
- **P-03:** A função pura de decisão de direito de acesso entregue em E1 cobre os cinco estados ativos definidos neste PRD (com `TRIALING` reservado e inerte).
- **P-04:** O assinante cancela na plataforma de pagamento; o produto reage ao evento. Não há comando ativo de cancelamento no WhatsApp neste MVP.
- **P-05:** A taxa de eventos vinda da Kiwify (incluindo retries) e a frequência da reconciliação horária operam dentro do limite de taxa da API Kiwify aplicável aos planos do MeControla.

### Riscos de negócio

- **R-01 (médio):** Dependência operacional de E1 estar em produção para execução das tarefas derivadas deste PRD. Mitigação: PRD e especificação técnica podem ser escritos em paralelo a E1; a execução espera E1 estar implementado.
- **R-02 (médio):** A notificação ao assinante em transições é best-effort. Em janelas de instabilidade do canal de WhatsApp, alguns usuários podem perder ou descobrir tarde mudanças de estado. Mitigação aceita para o MVP; hardening (replay, retry forte) cabe a E4.
- **R-03 (baixo):** A rejeição de webhook sem token de funil cria atrito caso a Kiwify falhe intermitentemente em propagar o token. Mitigação: P-01 já foi validada com compra real; se falhar em produção, vira incidente operacional e mitigação cabe a E4.
- **R-04 (baixo):** Reconciliação horária com a Kiwify pode esbarrar no limite de taxa da API se a base de assinaturas crescer muito. Mitigação: revisar capacidade quando a base atingir o limiar definido pelo épico de hardening (≥ 5k assinaturas ativas).
- **R-05 (baixo):** A rigidez "Kiwify vence sempre" pode propagar um estado errado da Kiwify para o sistema em caso de bug do lado da plataforma. Mitigação: a probabilidade é considerada baixa pelo produto, e qualquer caso real exige investigação manual fora do MVP.

### Questões em aberto

- **Q-01:** Comunicação ao assinante na transição `ACTIVE → PAST_DUE` deve incluir lembrete de regularização com prazo (os três dias de graça)? O conteúdo exato da mensagem fica para a especificação técnica/operacional, mas o conteúdo de negócio (incluir prazo, incluir link de regularização) precisa ser confirmado pelo time de produto antes da implementação.
- **Q-02:** O período de graça de três dias é parametrizável por plano (Mensal/Trimestral/Anual) ou é uniforme? PRD declara uniforme; abrir como questão caso negócio queira diferenciar.
- **Q-03:** Em caso de reembolso parcial sinalizado pela Kiwify, o produto trata como `REFUNDED` (revoga imediatamente) ou como evento ignorado? PRD assume reembolso parcial = `REFUNDED`. Confirmar com financeiro antes da implementação.

---

## Próximos passos pós-aprovação

1. Atualizar `docs/epics/epic-02-billing-pipeline.md` (`status: prd_done`, `next_skill: create-technical-specification`, `artifacts.prd: .specs/prd-billing-pipeline/prd.md`).
2. Rodar `create-technical-specification` para materializar a especificação técnica derivada deste PRD.
3. Em seguida, `create-tasks` decompondo a especificação em tarefas implementáveis.
4. A execução das tarefas espera E1 estar com `status: implemented` confirmado.
