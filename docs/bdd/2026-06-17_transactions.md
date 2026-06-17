# BDD: Módulo Transactions
**Data:** 2026-06-17
**Status:** MVP Robust / Production-Ready
**Referência:** Domain Modeling Made Functional (DMMF)

## Objetivo
Core do sistema: gestão de fluxos de caixa, compras no cartão, transações recorrentes e consolidação de resumos mensais.

## Fluxo 1: Criação de Transação Comum
**Funcionalidade:** Registro de despesa ou receita avulsa.

**Cenário:** Registro de nova despesa em dinheiro
- **Dado** que o usuário informa valor R$ 50,00, categoria 'Transporte' e data hoje
- **Quando** a transação é criada
- **Então** o saldo da conta deve ser atualizado
- **E** o resumo mensal (MonthlySummary) deve refletir o novo gasto imediatamente.

## Fluxo 2: Compra no Cartão de Crédito
**Funcionalidade:** Gestão de parcelas e limites de crédito.

**Cenário:** Compra parcelada registrada no cartão
- **Dado** um cartão com limite disponível de R$ 1000,00
- **Quando** o usuário registra uma compra de R$ 300,00 em 3 parcelas
- **Então** o limite disponível deve cair para R$ 700,00
- **E** 3 itens de fatura (CardInvoiceItem) devem ser agendados para os meses subsequentes.

## Fluxo 3: Transações Recorrentes (Templates)
**Funcionalidade:** Automação de contas fixas.

**Cenário:** Materialização de conta de luz recorrente
- **Dado** um template de recorrência para o dia 10 de cada mês
- **Quando** o sistema executa a materialização do dia
- **Então** uma nova transação real deve ser criada com base no template
- **E** o usuário deve ser notificado sobre o registro automático.

## Regras de Domínio (DMMF)
- **Workflow Pipeline:** A criação de transação segue: `ValidateInput -> UpdateBalance -> ProjectSummary -> EmitEvent`.
- **Invariantes:** O valor de uma transação não pode ser zero.
- **Discriminated Union:** Uma transação pode ser do tipo `Income` (Receita) ou `Expense` (Despesa).

## Validação de Produção
- [ ] Garantir atomicidade entre a criação da transação e a atualização do resumo mensal.
- [ ] Validar o cálculo de faturas futuras em compras parceladas.
- [ ] Verificar se a materialização de templates é idempotente para o mesmo dia/período.
