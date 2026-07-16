# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Extensão aditiva do enum PaymentMethod e desbloqueio de DOC
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Solicitante do produto, engenharia de plataforma
- **Relacionados:** PRD `prd.md` (RF-03); techspec `techspec.md`; DMMF state-as-type

## Contexto

O documento exige reconhecer formas de pagamento hoje ausentes do enum `PaymentMethod` (`internal/transactions/domain/valueobjects/payment_method.go`): carteiras digitais (Apple Pay, Google Pay, PicPay, Mercado Pago), cheque, transferência e DOC. O enum atual tem 10 valores (pix, ted, debit_in_account, debit_card, cash, boleto, credit_card, doc, vale_refeicao, vale_alimentacao). `doc` existe mas é bloqueado na criação (`ParsePaymentMethodForCreate` -> `ErrPaymentMethodDocReadOnly`); `transferencia` hoje é mapeada para `ted` no reconhecimento do agente. A US decidiu que cada nova forma persiste como valor próprio.

## Decisão

Estender o enum de forma **aditiva** com `PaymentMethodTransferencia`, `PaymentMethodApplePay`, `PaymentMethodGooglePay`, `PaymentMethodPicPay`, `PaymentMethodMercadoPago`, `PaymentMethodCheque`, atualizando `ParsePaymentMethod`, `String()` e o limite superior de `PaymentMethodFromInt`. **Desbloquear `doc` na criação** (remover `ErrPaymentMethodDocReadOnly` do caminho de create). Todos os novos valores são não-cartão: seguem automaticamente o ramo sem fatura em `DecideCreate`/`DecideUpdate` (o gate é `IsCreditCard()`), sem trabalho extra. "Cartão \<Banco\>" continua resolvendo para um cartão do usuário por apelido (não é forma de pagamento própria). Sinônimos de reconhecimento (Espécie -> cash, VR -> vale_refeicao, VA -> vale_alimentacao) ficam no mapa de reconhecimento do agente.

## Alternativas Consideradas

- **Mapear as formas novas para o enum atual** (ex.: PicPay/Apple Pay -> pix): zero mudança no domínio; rejeitada por perder granularidade e classificar errado, ferindo a fidelidade do documento.
- **Manter `doc` bloqueado**: rejeitada porque RF-03 exige reconhecer e registrar DOC como forma própria.

## Consequências

### Benefícios Esperados

- Fidelidade às formas do documento com dados corretos por método.
- Sem impacto em fatura/parcelas (novos valores são não-cartão por padrão).

### Trade-offs e Custos

- Vários pontos de atualização sincronizados (const, parse, String, bound, enum do LLM, labels, mapa de reconhecimento).

### Riscos e Mitigações

- Risco: esquecer um ponto de sincronização (ex.: `PaymentMethodFromInt` bound, causando erro de scan no DB). Mitigação: checklist na task + testes de round-trip parse/String/FromInt para cada novo valor.
- Risco: implicações de desbloquear `doc`. Mitigação: `doc` é não-cartão; comportamento idêntico às demais formas não-cartão; teste de criação com `doc`.

## Plano de Implementação

1. Adicionar os `iota` novos e atualizar `ParsePaymentMethod`/`String()`/`PaymentMethodFromInt`.
2. Remover o bloqueio de `doc` em `ParsePaymentMethodForCreate`.
3. Atualizar o enum do LLM em `register_expense.go` (e `edit_entry` quando expuser forma de pagamento), `formatPaymentLabel` e `knownPaymentMethods`.
4. Testes de round-trip e de criação por cada forma nova.

## Monitoramento e Validação

- Testes unitários parse/String/FromInt; golden de reconhecimento de formas de pagamento.
- Sucesso: cada forma do documento é reconhecida, persistida como valor próprio e não gera fatura indevida.

## Impacto em Documentação e Operação

- Atualizar a documentação do enum `PaymentMethod` e o catálogo de formas reconhecidas.

## Revisão Futura

- Revisitar se alguma carteira digital passar a exigir tratamento de fatura específico.
