# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Despacho por PaymentMethod via orquestração de decisão pura (não Strategy de classe)
- **Data:** 2026-07-04
- **Status:** Aceita
- **Decisores:** Autor do PRD/techspec, time de plataforma
- **Relacionados:** `.specs/prd-transactions-crud-unificado/prd.md` (RF-10, RF-11), `techspec.md`,
  `.claude/rules/transactions-workflows.md` (R-TXN-001), `.claude/skills/go-implementation/`,
  refactoring.guru/design-patterns (referência conceitual)

## Contexto

No CRUD unificado, `payment_method=credit_card` diverge de comportamento (consulta cartão, resolve
fatura, parcela), enquanto `pix/ted/debit_in_account/debit_card/cash/boleto/vale_refeicao/vale_alimentacao`
são lançamento simples idêntico. Precisamos de um mecanismo de despacho que:
- mantenha a regra de negócio exclusivamente em funções `Decide*` puras (R-TXN-001, DMMF);
- não introduza `switch case` de domínio no use case (proibido por R-TXN-001/R-AGENT-WF-001.1);
- não sobre-projete com hierarquia de classes para um único caso divergente.

## Decisão

O use case orquestra: ele **seleciona qual caminho de decisão pura invocar** com base no VO fechado
`PaymentMethod`, mas **não contém regra de negócio**. Concretamente:
- Quando `credit_card`: o use case faz o IO sancionado (`CardLookup.GetForUser`) e passa o
  `CardBillingSnapshot` para `TransactionWorkflow.DecideCreate/Update/Delete`, que internamente
  compõe `InstallmentSplitter` + `BillingCycleResolver` (ambos puros) e emite `[]CardInvoiceItem` +
  `InvoiceDeltas`.
- Caso contrário: passa `option.None` de snapshot; o `Decide*` produz apenas a `Transaction` simples.

A diferenciação é um parâmetro opcional (`option.Option[CardBillingSnapshot]`) do `Decide*`, não um
branch de regra no use case. A escolha do path é orquestração de fronteira (equivalente ao "espírito"
do Strategy, porém como função/decisão parametrizada), não cálculo de domínio.

Escopo: `internal/transactions/application/usecases/{create,update,delete}_transaction.go` e
`internal/transactions/domain/services/transaction_workflow.go`.

## Alternativas Consideradas

- **Strategy clássico (uma classe-estratégia por PaymentMethod).** Vantagem: extensível por
  polimorfismo. Desvantagem: 8+ classes para 1 comportamento divergente; hierarquia de objetos
  contraria a preferência DMMF/go-implementation por união discriminada + funções puras.
  Rejeitado por over-engineering.
- **Registry `map[PaymentMethod]func(...)` no use case.** Vantagem: sem `switch`. Desvantagem: os
  dois paths têm assinaturas de efeito diferentes (um faz IO de cartão, o outro não); forçar uma
  assinatura comum vaza complexidade. Rejeitado — o `option` no `Decide*` é mais simples e mantém a
  regra no domínio.
- **`if payment_method == credit_card {...} else {...}` com regra no use case.** Rejeitado: seria
  branching de domínio no use case, violando R-TXN-001.

## Consequências

### Benefícios Esperados
- Regra 100% em `Decide*` puro, testável sem mock; conformidade R-TXN-001.
- Reuso integral de `InstallmentSplitter`/`BillingCycleResolver`/`CardPurchaseWorkflow` sem reescrita.
- Sem hierarquia de classes; código coeso e funcional.

### Trade-offs e Custos
- `TransactionDecision` cresce (`Items`, `InvoiceDeltas` nil para não-cartão) — leve aumento de
  superfície do tipo, aceitável e explícito.
- Adicionar um novo comportamento divergente futuro exige enriquecer `Decide*`, não plugar classe.

### Riscos e Mitigações
- Risco: erosão da pureza se alguém colocar IO no `Decide*`. Mitigação: gate R-TXN-001 no CI +
  revisão; IO permanece no use case.
- Rollback: reverter para dois use cases separados é possível, mas desnecessário (unificação é meta).

## Plano de Implementação
1. Enriquecer `TransactionWorkflow.Decide*` com parâmetro `option.Option[CardBillingSnapshot]`.
2. Migrar composição de `CardPurchaseWorkflow` para dentro do `Decide*` (ou chamada sequencial pura).
3. Ajustar use cases para IO condicional + orquestração.
4. Testes unitários puros cobrindo ambos os paths.

## Monitoramento e Validação
- Gate R-TXN-001 (regra fora de `Decide*` bloqueia PR) verde.
- Cobertura unitária dos dois paths; métrica `operation` por use case.
- Critério de sucesso: 0 branch de regra de domínio nos use cases (grep do gate vazio).

## Impacto em Documentação e Operação
- Atualizar runbook `docs/runbooks/transactions.md` com o fluxo credit_card unificado.

## Revisão Futura
- Revisar se surgir um segundo `payment_method` com comportamento divergente (ex.: boleto parcelado),
  quando o custo de um registry tipado passaria a compensar.
