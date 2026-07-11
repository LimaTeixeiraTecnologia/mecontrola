# Controle de Acesso por Assinatura no Agente do WhatsApp — User Story única, pronta para desenvolvimento

> Fonte: pedido do usuário (analisar `internal/billing` em busca de gap real e materializar UMA US robusta) confrontado com a base de código real do repositório `mecontrola`.
> Data de geração: 2026-07-11
> Nome do arquivo: `2026-07-11-us-controle-de-acesso-por-assinatura-whatsapp.md`

## Resumo e decisão de escopo

O módulo `internal/billing` está estruturalmente completo: recebe os webhooks da Kiwify (ingress com HMAC, rate limit e raw-body), modela o ciclo de vida da assinatura em domínio DMMF puro (6 status fechados, 6 triggers, detecção de regressão temporal), publica 7 eventos de domínio no outbox e reconcilia contra a API da Kiwify por job agendado. O identity, por sua vez, projeta esses eventos numa tabela de entitlement (`identity_entitlements`) e — o ponto central desta história — **já possui a decisão de acesso pronta e pura**: `IsEntitled(sub, now) (bool, Reason)` com tipos fechados (`internal/identity/domain/entitlement.go:35`), embrulhada pelo serviço `EntitlementDecider.Decide` (`internal/identity/domain/services/entitlement_decider.go:16`), e uma porta de leitura `EntitlementReader.FindByUserID` (`internal/identity/application/interfaces/entitlement_repository.go:22-24`).

O gap real, confirmado por rastreamento de chamadas no repositório inteiro, é que **nada disso é invocado em produção**: nenhum use case, handler HTTP, consumer, job ou o consumer de inbound do WhatsApp chama `EntitlementReader.FindByUserID`, `EntitlementDecider.Decide` ou `IsEntitled`. A capacidade paga (o agente financeiro no WhatsApp) atende qualquer usuário sem verificar o estado da assinatura. Um usuário `EXPIRED`, `REFUNDED`, `CANCELED_PENDING` com período vencido ou `PAST_DUE` além da graça continua com acesso total ao produto pago.

Atendendo ao pedido de "uma única US robusta e pronta para desenvolvimento", esta história foca em **aplicar (enforce) o entitlement de assinatura no ponto de entrada do agente financeiro no WhatsApp**, reutilizando a decisão pura já existente. A construção do billing, a projeção do identity e a decisão `IsEntitled` entram como referências e dependências existentes, não como trabalho a construir.

## Confronto com o Codebase

Comportamento **atual** (evidenciado):

- Billing publica 7 eventos no outbox (`internal/billing/infrastructure/messaging/database/producers/events.go:6-12`): `billing.subscription.activated`, `.activated_without_token`, `.renewed`, `.past_due`, `.canceled`, `.refunded`, `.expired_after_grace`.
- Identity consome 5 desses eventos + `onboarding.subscription_bound` e projeta o entitlement corrente (`internal/identity/module.go:158-163`; `internal/identity/application/usecases/project_subscription_event.go:16-22,67-88`), lendo `status`, `period_end`, `grace_end` do projetor de assinatura (`internal/identity/infrastructure/repositories/postgres/subscription_projection_reader.go:35`) e gravando em `identity_entitlements` (`migrations/000001_initial_schema.up.sql:128-144`, com CHECK de status fechado).
- A decisão de acesso é pura e cobre os seis status, período e graça: `IsEntitled` (`internal/identity/domain/entitlement.go:35-68`) sobre os tipos fechados `SubscriptionStatus` (`entitlement.go:7-14`) e `Reason` (`entitlement.go:24-33`), embrulhada por `EntitlementDecider.Decide` (`entitlement_decider.go:16-19`).
- A porta de leitura `EntitlementReader.FindByUserID` está exposta na superfície pública do módulo identity (`internal/identity/module.go:51,146,211-216`).
- O agente do WhatsApp é servido pelo consumer de inbound `WhatsAppInboundConsumer.Handle`, que valida `user_id`/`peer`/`text`/`message_id`, tenta a cadeia de resume de fluxos financeiros e resolve onboarding antes de rotear ao agente (`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:154-306`).
- O módulo `internal/agents` já depende do identity (contrato cross-module existente), importando o pacote `auth` de `internal/identity/application` em bindings (`internal/agents/module.go:26`); o bootstrap já injeta capacidades do identity no agents via adapters (precedente `BuildCardChannelResolver(identityModule)` em `internal/bootstrap/resolver.go:30-35`).

Gaps e defeitos **confirmados** que esta US fecha:

1. Nenhum caminho de produção invoca `EntitlementReader.FindByUserID`, `EntitlementDecider.Decide` ou `IsEntitled` — a decisão de acesso existe mas nunca é aplicada. O consumer de inbound do WhatsApp não tem nenhum gate de assinatura (`whatsapp_inbound_consumer.go:154-306` não referencia entitlement).
2. Consistência de projeção: identity não consome `billing.subscription.expired_after_grace` (`internal/identity/module.go:158-163` não registra esse tipo; `project_subscription_event.go:85-87` cai em `default: return nil`). Após a graça expirar, o rótulo de status do entitlement permanece `PAST_DUE` indefinidamente. A decisão de acesso ainda nega corretamente (porque `IsEntitled` reavalia `grace_end` contra o instante atual), mas o status armazenado deixa de refletir a verdade apurada pelo billing.

## Análise DMMF e de padrões (skills obrigatórias)

- `domain-modeling-production`: a decisão de acesso já é modelada como estado-como-tipo (`SubscriptionStatus` e `Reason` fechados) e função pura determinística `IsEntitled` (sem IO, sem `context.Context`), separando decisão de efeito. Esta US NÃO reimplementa a decisão; apenas a liga ao efeito (servir ou negar o agente). Estados ilegais permanecem irrepresentáveis por serem enums fechados.
- `design-patterns-mandatory`: o seletor resulta em **não aplicar padrão novo**. A ligação exige apenas (a) um adapter fino que converta `EntitlementRecord` (status `string`, `period_end`, `grace_end`) na interface de domínio `domain.Subscription` (`Status()/PeriodEnd()/GracePeriodEnd()`, `entitlement.go:16-20`) e (b) reuso do decider existente — ambos já cobertos por Adapter e Factory Function inline no SKILL do go-implementation. Nenhum GoF adicional é necessário.
- `go-implementation`: o gate é um adaptador fino no consumer (`adapter → usecase/decider`), sem regra de negócio nova, sem SQL direto e sem branching de domínio embutido; regra vive na decisão pura. Métrica com cardinalidade controlada (labels fechados). Zero comentários em `.go`.

---

## Declaração
Como responsável pelo produto MeControla (dono do negócio), quero que o agente financeiro no WhatsApp só atenda usuários com assinatura vigente, negando com uma mensagem determinística de renovação quem estiver sem assinatura ativa, para proteger a receita e garantir que o acesso ao produto pago reflita o estado real da assinatura já apurado pelo billing.

## Contexto
- Problema: o billing apura o ciclo de vida da assinatura e o identity projeta o entitlement e já contém a decisão de acesso pronta (`IsEntitled`), mas essa decisão nunca é aplicada; o agente financeiro atende qualquer usuário, inclusive `EXPIRED`, `REFUNDED`, cancelado com período vencido ou inadimplente além da graça — vazamento de receita e acesso indevido ao produto pago.
- Resultado esperado: um gate de acesso no ponto de entrada do agente no WhatsApp que consulta o entitlement do usuário, decide via a função pura existente e, quando não vigente, nega o atendimento com mensagem determinística orientando a renovação, sem processar nenhum lançamento; usuários vigentes seguem sem alteração de comportamento.
- Fonte: pedido do usuário e confronto com a base de código (`internal/identity/domain/entitlement.go`, `internal/identity/domain/services/entitlement_decider.go`, `internal/identity/application/interfaces/entitlement_repository.go`, `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`, `internal/billing/infrastructure/messaging/database/producers/events.go`).

## Regras de Negócio
- Ponto de aplicação: o gate roda no consumer de inbound do WhatsApp (`whatsapp_inbound_consumer.go`), no caminho que serve o agente financeiro (inclusive as cadeias de resume de fluxos financeiros: pending entry, destructive confirm, card create, budget creation), antes de qualquer atendimento do agente.
- Escopo do bloqueio (decisão confirmada): quando a assinatura não está vigente, **toda** interação com o agente financeiro é negada — leitura e escrita —, não apenas escritas.
- Reuso da decisão pura: a elegibilidade é decidida exclusivamente por `IsEntitled`/`EntitlementDecider.Decide` (`entitlement.go:35`, `entitlement_decider.go:16`) sobre o registro obtido via `EntitlementReader.FindByUserID` (`entitlement_repository.go:22-24`); a decisão não é reimplementada no agents nem no consumer.
- Instante de avaliação: usar `time.Now().UTC()` inline como `now` na chamada de decisão, sem abstração de tempo.
- Vigência por status (herda a semântica já modelada em `IsEntitled`, `entitlement.go:39-67`):
  - `ACTIVE` com `period_end` no futuro: vigente (atende).
  - `TRIALING` com `period_end` no futuro: vigente (atende).
  - `PAST_DUE` com `grace_end` no futuro: vigente durante a graça (atende).
  - `CANCELED_PENDING` com `period_end` no futuro: vigente até o fim do período pago (atende).
  - `ACTIVE`/`TRIALING`/`CANCELED_PENDING` com período vencido: não vigente (nega).
  - `PAST_DUE` com `grace_end` no passado: não vigente (nega).
  - `EXPIRED`: não vigente (nega).
  - `REFUNDED`: não vigente (nega).
- Janela de graça (decisão confirmada): durante `PAST_DUE` dentro da graça (3 dias, `internal/billing/domain/valueobjects/grace_window.go`), o acesso é liberado normalmente, **sem aviso** anexado; mantém a decisão já modelada (`ReasonPastDueGrace`).
- Isenção de onboarding (decisão confirmada): o gate não se aplica ao fluxo de onboarding nem à ativação. Quando a mensagem é atendida pelo caminho de onboarding (`tryResolveOnboarding`, `whatsapp_inbound_consumer.go:286-305`), o gate de assinatura não é avaliado. Isso evita travar quem está entrando, cuja ativação sem token faz o binding posterior via `onboarding.subscription_bound`, evento que o identity consome e projeta o entitlement.
- Ausência e falha na consulta (decisão confirmada — política híbrida):
  - Ausência definitiva de registro de entitlement para um usuário fora do onboarding: fail-closed (negar acesso e orientar renovação). Como o onboarding só existe após ativação de assinatura, ausência de entitlement após o onboarding é anomalia e é tratada como não vigente.
  - Erro transitório de infraestrutura na consulta (ex.: timeout de banco): fail-open degradado (atender), registrando log e incrementando métrica de falha degradada para alerta; não punir usuário pagante por erro momentâneo. Alinha-se ao tratamento de timeout de inbound já existente (`recordInboundTimeout`, `whatsapp_inbound_consumer.go:306`).
- No-false-success: usuário bloqueado nunca recebe confirmação de lançamento; nenhuma escrita financeira é executada para requisição negada. A resposta de bloqueio é determinística e não menciona termos de infraestrutura ("workflow", "entitlement", "sistema interno").
- Consistência de projeção (defesa em profundidade): o identity passa a consumir `billing.subscription.expired_after_grace` e reprojeta o entitlement, de modo que o status armazenado transite para `EXPIRED` após a graça, refletindo a verdade apurada pelo billing. A decisão de acesso já nega o caso via `grace_end`; esta regra garante que o rótulo persistido seja verdadeiro.
- Cardinalidade controlada: a métrica do gate usa apenas labels de baixa cardinalidade e fechados (ex.: `channel`, `decision`, `reason`); proibido `user_id` como label.

## Critérios de Aceite
```gherkin
Cenário: Usuário com assinatura ativa é atendido normalmente
  Dado que o usuário tem entitlement com status ativo e período ainda vigente
  Quando ele envia uma mensagem ao agente financeiro no WhatsApp
  Então o gate permite o atendimento e o agente processa a mensagem como hoje

Cenário: Usuário inadimplente dentro da graça é atendido sem aviso
  Dado que o usuário está em atraso mas ainda dentro da janela de graça
  Quando ele envia uma mensagem ao agente financeiro
  Então o gate permite o atendimento
  E nenhuma mensagem de aviso de atraso é anexada à resposta

Cenário: Assinatura cancelada com período ainda vigente é atendida
  Dado que o usuário cancelou mas o período pago ainda não terminou
  Quando ele envia uma mensagem ao agente financeiro
  Então o gate permite o atendimento até o fim do período pago

Cenário: Assinatura expirada é bloqueada com orientação de renovação
  Dado que o usuário tem entitlement com status expirado
  Quando ele envia uma mensagem ao agente financeiro
  Então o gate nega o atendimento
  E a resposta é uma mensagem determinística orientando a renovar a assinatura
  E nenhum lançamento é registrado

Cenário: Assinatura reembolsada é bloqueada
  Dado que o usuário tem entitlement com status reembolsado
  Quando ele envia uma mensagem ao agente financeiro
  Então o gate nega o atendimento e orienta a renovação

Cenário: Inadimplência além da graça é bloqueada mesmo sem status expirado projetado
  Dado que o usuário está em atraso e a janela de graça já passou
  Quando ele envia uma mensagem ao agente financeiro
  Então o gate nega o atendimento com base no fim da graça
  E a decisão não depende de o rótulo de status já ter sido atualizado para expirado

Cenário: Cancelado com período vencido é bloqueado
  Dado que o usuário cancelou e o período pago já terminou
  Quando ele envia uma mensagem ao agente financeiro
  Então o gate nega o atendimento e orienta a renovação

Cenário: Ausência de entitlement após onboarding bloqueia (fail-closed)
  Dado que o usuário não está em onboarding e não possui registro de entitlement
  Quando ele envia uma mensagem ao agente financeiro
  Então o gate nega o atendimento e orienta a renovação

Cenário: Erro transitório na consulta de entitlement permite atender (fail-open degradado)
  Dado que a consulta ao entitlement falha por erro transitório de infraestrutura
  Quando o usuário envia uma mensagem ao agente financeiro
  Então o gate permite o atendimento de forma degradada
  E registra log e incrementa a métrica de falha degradada para alerta

Cenário: Usuário em onboarding é isento do gate
  Dado que a mensagem do usuário é atendida pelo fluxo de onboarding
  Quando a mensagem chega ao consumer de inbound
  Então o gate de assinatura não é avaliado
  E o onboarding segue o seu fluxo normal

Cenário: Usuário bloqueado não recebe falso sucesso em reenvio
  Dado que um usuário sem assinatura vigente já foi bloqueado ao tentar registrar um lançamento
  Quando ele reenvia a mesma mensagem
  Então nenhum lançamento é registrado em nenhuma tentativa
  E a resposta permanece a mensagem determinística de bloqueio, sem confirmar registro

Cenário: Consistência do entitlement após expiração da graça
  Dado que o billing publicou o evento de expiração após a graça para uma assinatura
  Quando o identity consome esse evento
  Então o status do entitlement projetado passa a expirado
```

## Dados e Permissões
- Dados obrigatórios: `user_id` do usuário (derivado do payload de inbound `p.UserID`, `whatsapp_inbound_consumer.go:172`), usado como chave do entitlement (PK `identity_entitlements.user_id`); do registro lido: `status`, `period_end`, `grace_end`; `now` avaliado como `time.Now().UTC()` inline no momento da decisão.
- Perfis/permissões: usuário final autenticado no canal WhatsApp; a consulta de entitlement é restrita ao próprio usuário (chave por `user_id`); a decisão é read-only e não altera o entitlement.

## Dependências
- Decisão pura e serviço existentes: `IsEntitled` (`internal/identity/domain/entitlement.go:35`) e `EntitlementDecider.Decide` (`internal/identity/domain/services/entitlement_decider.go:16`) — reusados, não reimplementados.
- Porta de leitura existente: `EntitlementReader.FindByUserID` (`internal/identity/application/interfaces/entitlement_repository.go:22-24`), já exposta na superfície do módulo identity (`internal/identity/module.go:51,146,211-216`).
- Ponto de aplicação: `WhatsAppInboundConsumer` (`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`) e a resolução de onboarding para a isenção (`tryResolveOnboarding`, mesmo arquivo, linhas 286-305).
- Wiring cross-module: bootstrap injetando a leitura de entitlement do identity no agents via adapter, seguindo o precedente de `BuildCardChannelResolver(identityModule)` (`internal/bootstrap/resolver.go:30-35`); o agents já depende do identity (`internal/agents/module.go:26`).
- Adapter fino a criar: converter `EntitlementRecord` (status `string`, `period_end`, `grace_end`) na interface `domain.Subscription` (`Status()/PeriodEnd()/GracePeriodEnd()`, `entitlement.go:16-20`) para alimentar o decider.
- Projeção de consistência: registrar handler de `billing.subscription.expired_after_grace` no projetor de assinatura do identity (`internal/identity/module.go:158-163`; `project_subscription_event.go:16-22,67-88`), evento já publicado pelo billing (`producers/events.go:12`).
- Origem dos dados: billing publicando os eventos de ciclo de vida no outbox (`internal/billing/infrastructure/messaging/database/producers/events.go:6-12`) e o projetor de assinatura do identity (`subscription_projection_reader.go:35`).

## Fora de Escopo
- Notificações de billing ao usuário sobre atraso/reembolso/expiração: hoje o billing tem consumers de notificação, porém com sender noop (`internal/billing/module.go`); é um gap separado de comunicação, não de controle de acesso.
- Onboarding consumir `renewed`/`canceled`/`refunded` para métricas de churn.
- Endpoint REST ou API pública para consultar/expor entitlement.
- Cobrança, precificação ou qualquer cálculo financeiro do plano (o domínio de billing não modela preço).
- Status `TRIALING` como fluxo ativo do billing: o billing nunca define `TRIALING` hoje; a decisão `IsEntitled` já o trata, mas não há trabalho de entrada desse estado nesta história.
- Triggers de webhook inertes no billing (`billet_created`, `pix_created`, `order_rejected`, `abandoned_cart`) e a função de client `GetSale` não utilizada.

## Evidências
- Entrada: pedido do usuário para analisar `internal/billing`, identificar gap/lacuna real production-ready sem inventar resposta e materializar UMA única US robusta em `docs/us` com prefixo da data de hoje; decisões de escopo confirmadas em rodada de múltipla escolha (bloquear tudo; política híbrida de falha/ausência; isentar onboarding; liberar na graça sem aviso).
- Base de código: `internal/identity/domain/entitlement.go:7-14,24-33,35-68` (decisão pura e tipos fechados); `internal/identity/domain/services/entitlement_decider.go:16-19` (serviço decider); `internal/identity/application/interfaces/entitlement_repository.go:16-24` (porta de leitura); `internal/identity/module.go:51,146,158-163,211-216` (exposição da porta e registro dos 5 eventos, sem `expired_after_grace`); `internal/identity/application/usecases/project_subscription_event.go:16-22,67-88` (eventos consumidos e `default: return nil`); `internal/identity/infrastructure/repositories/postgres/subscription_projection_reader.go:35` (leitura de status/período/graça); `migrations/000001_initial_schema.up.sql:128-144` (tabela `identity_entitlements` com CHECK de status); `internal/billing/infrastructure/messaging/database/producers/events.go:6-12` (7 eventos publicados, incluindo `expired_after_grace`); `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:154-306` (fluxo de inbound sem gate de assinatura); `internal/agents/module.go:26` e `internal/bootstrap/resolver.go:30-35` (dependência agents→identity e precedente de wiring cross-module).
- Inferências: o ponto de aplicação no consumer de inbound, o adapter `EntitlementRecord`→`domain.Subscription`, a política híbrida de falha/ausência, a isenção via resolução de onboarding e o consumo de `expired_after_grace` para consistência são propostas de estado-alvo, ancoradas no código existente e separadas do comportamento atual.
- Não evidenciado: não existe hoje nenhuma chamada de produção a `EntitlementReader.FindByUserID`, `EntitlementDecider.Decide` ou `IsEntitled` (rastreamento de chamadas em `internal` e `cmd`, excluindo testes e mocks, retornou apenas definições e wiring); não existe gate de assinatura no consumer de inbound; não existe handler de `billing.subscription.expired_after_grace` no identity; não existe caso golden/real-LLM para o caminho de bloqueio — são artefatos a criar.

## Notas de Validação
- Cobertura: a história cobre fluxo feliz (ativo → atende), variações válidas (graça sem aviso; cancelado com período vigente; fail-open degradado; isenção de onboarding) e bloqueios/erros (expirado, reembolsado, cancelado vencido, inadimplente além da graça, ausência fail-closed, no-false-success em reenvio) em critérios de aceite verificáveis, além da consistência de projeção pós-graça.
- Robustez para desenvolvimento: reutilizar a decisão pura existente (não reimplementar), estado como tipo fechado (DMMF state-as-type), gate como adaptador fino no consumer, adapter `EntitlementRecord`→`domain.Subscription`, `time.Now().UTC()` inline, zero comentários em `.go`, métrica com labels fechados. O seletor de padrões indica não aplicar padrão novo (reuso de Adapter + decider).
- Prontidão de teste: testes unitários da decisão já existem no identity; adicionar testes do adapter e do gate (matriz de status × período/graça), teste do fail-open degradado vs fail-closed, teste de isenção de onboarding e teste do novo handler de `expired_after_grace`; incluir caso golden/real-LLM do caminho de bloqueio para validar a mensagem determinística de renovação, com gate por categoria conforme a prática do repositório.

## Notas Técnicas para Desenvolvimento
- Criar no `internal/agents` um adapter/porta consumidor `EntitlementGate` que recebe a leitura de entitlement do identity (via bootstrap) e o `EntitlementDecider`, expondo uma decisão booleana + `Reason` para o consumer; o consumer permanece fino (adapter → decisão), sem regra de negócio embutida.
- Converter `EntitlementRecord.Status` (string vinda de `identity_entitlements`, com CHECK fechado) para `domain.SubscriptionStatus` e montar um `domain.Subscription` de leitura (Status/PeriodEnd/GracePeriodEnd) para alimentar `IsEntitled`; status desconhecido cai no ramo `default` de `IsEntitled` (nega) — manter esse comportamento.
- Posicionar o gate no `WhatsAppInboundConsumer.Handle` após a validação de payload e a isenção de onboarding, antes das cadeias de resume financeiras e do agente; quando negar, responder a mensagem determinística de renovação e encerrar o processamento sem efeitos.
- Fail-open degradado deve reaproveitar o padrão de observabilidade já usado no timeout de inbound (`recordInboundTimeout`), com log e métrica dedicada de falha degradada de gate.
- Registrar `billing.subscription.expired_after_grace` no projetor de assinatura do identity (`module.go:158-163`) e estender o `switch` de `project_subscription_event.go` para reprojetar o entitlement nesse evento; a reprojeção reusa `FindCurrentBySubscriptionID`, então o status corrente já apurado pelo billing (`EXPIRED`) é persistido.
- Métrica do gate com labels fechados (`channel`, `decision`, `reason`), sem `user_id`; herda a política de cardinalidade controlada do projeto.
