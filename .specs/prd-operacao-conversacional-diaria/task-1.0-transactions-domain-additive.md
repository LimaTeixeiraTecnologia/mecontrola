# Tarefa 1.0: Domínio transactions aditivo (enum PaymentMethod + evento enriquecido)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender de forma aditiva o domínio de `internal/transactions`: adicionar novas formas de pagamento ao enum `PaymentMethod`, desbloquear `doc` na criação e enriquecer o evento `TransactionUpdated` com a subcategoria (serializada no producer/outbox). Mudança puramente aditiva, sem impacto em fatura/parcelas (todos os novos valores são não-cartão).

<requirements>
- RF-03 (formas de pagamento) e RF-15 (contribuição: subcategoria no evento de edição).
- ADR-006 (extensão aditiva do enum + desbloqueio de DOC).
- ADR-004 (parte do evento: enriquecer `TransactionUpdated` com `SubcategoryID` e serializar no producer).
- R-TXN-WORKFLOWS-001: regra de domínio apenas em `Decide*`; validação apenas em smart constructors.
- R-ADAPTER-001.1: zero comentários em `.go` de produção.
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar os valores ao enum `PaymentMethod` em `internal/transactions/domain/valueobjects/payment_method.go`: `PaymentMethodTransferencia`, `PaymentMethodApplePay`, `PaymentMethodGooglePay`, `PaymentMethodPicPay`, `PaymentMethodMercadoPago`, `PaymentMethodCheque`; atualizar `ParsePaymentMethod`, `String()` e o limite superior de `PaymentMethodFromInt`.
- [ ] 1.2 Desbloquear `doc` na criação: remover `ErrPaymentMethodDocReadOnly` do caminho de `ParsePaymentMethodForCreate`.
- [ ] 1.3 Enriquecer `TransactionUpdated` com `SubcategoryID uuid.UUID` em `internal/transactions/domain/entities/events.go` e serializar no producer `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher.go` (payload versionado).
- [ ] 1.4 Testes unitários: round-trip parse/`String`/`FromInt` por valor novo; criação com `doc`; serialização da subcategoria em `TransactionUpdated`.

## Detalhes de Implementação

Ver techspec.md, seção `### Interfaces Chave` e `### Considerações Técnicas -> Decisões Chave`, e ADR-006 / ADR-004 desta pasta. Pontos-chave (não duplicar, referenciar):

- Novos valores são não-cartão: seguem automaticamente o ramo sem fatura em `DecideCreate`/`DecideUpdate`; o gate de fatura é `IsCreditCard()` (ADR-006).
- Todos os pontos de sincronização (const `iota`, `ParsePaymentMethod`, `String()`, bound de `PaymentMethodFromInt`) devem ser atualizados juntos para evitar erro de scan no DB.
- Sinônimos de reconhecimento (transferência, espécie, VR, VA) ficam no mapa de reconhecimento do agente — fora do escopo desta tarefa.
- `TransactionUpdated` continua carregando `AmountCents`/`RefMonth`; adição de `SubcategoryID` é aditiva e versionada (ADR-004).

## Critérios de Sucesso

- Cada forma nova faz round-trip completo `parse -> String -> FromInt` sem perda.
- `doc` é criável na criação (sem `ErrPaymentMethodDocReadOnly`).
- Novos valores são não-cartão: não geram fatura em `DecideCreate`/`DecideUpdate` (gate `IsCreditCard()`).
- `TransactionUpdated` serializa a subcategoria no payload do producer.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — estado-como-tipo fechado para o enum PaymentMethod e o evento de domínio enriquecido.
- `design-patterns-mandatory` — gate de desenho para a extensão aditiva do value object e do evento.

## Testes da Tarefa

- [ ] Testes unitários (round-trip parse/String/FromInt por valor novo, criação com `doc`, serialização do evento com subcategoria)
- [ ] Testes de integração (não obrigatória nesta tarefa)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/transactions/domain/valueobjects/payment_method.go`
- `internal/transactions/domain/entities/events.go`
- `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher.go`
