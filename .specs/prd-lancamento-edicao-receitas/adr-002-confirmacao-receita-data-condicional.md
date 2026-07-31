# Registro de Decisão Arquitetural (ADR-002)

## Metadados

- **Título:** Linha `📅 Data` condicional no bloco de confirmação de receita
- **Data:** 2026-07-30
- **Status:** Aceita
- **Decisores:** Solicitante do produto (jailton.junior94), engenharia de plataforma
- **Relacionados:** PRD `.specs/prd-lancamento-edicao-receitas/prd.md` (RF-10); techspec `.specs/prd-lancamento-edicao-receitas/techspec.md`; US `docs/us/2026-07-30-lancamento-e-edicao-de-receitas.md` (RN-07, D1)

## Contexto

- O bloco de confirmação de receita `IncomeConfirmationBlock` (catalog.go:251) exibe hoje apenas `💰 Valor` e `📥 Origem`, e `confirmSummaryIncome` (transaction_write_workflow.go:1023) preenche só `AmountFormatted` e `Origin`.
- O objetivo do produto pede a exibição da data na confirmação, mas o catálogo oficial de mensagens é a fonte de verdade do Tom de Voz (inegociável), e a decisão D1 mantém o texto `Posso registrar?` e adiciona a data **apenas quando** informada/derivada.
- `ConfirmationView` já possui o campo `DateFormatted` (catalog.go:106) e `formatConfirmDate` (transaction_write_workflow.go:1066) já retorna `""` quando a data é vazia ou igual ao dia corrente.

## Decisão

1. `IncomeConfirmationBlock` passa a renderizar a linha `📅 Data: <data>` **somente** quando `ConfirmationView.DateFormatted` for não vazio; caso contrário, mantém o formato atual (Valor/Origem + `Posso registrar?`).
2. `confirmSummaryIncome` passa a preencher `DateFormatted: formatConfirmDate(state.OccurredAt, now)`, espelhando `confirmSummaryExpense` (transaction_write_workflow.go:1060). Como `formatConfirmDate` já suprime a data quando é vazia ou igual a hoje, a linha aparece apenas para datas específicas informadas/derivadas.
3. Não se adota o texto alternativo do briefing (`Posso registrar essa receita?`) nem a mensagem de sucesso alternativa; o catálogo oficial permanece a fonte de verdade (D2).

## Alternativas Consideradas

- **Sempre exibir a linha de data (inclusive hoje).** Vantagem: layout uniforme. Desvantagem: polui a confirmação no caso comum (registro do dia) e contraria D1. Rejeitada.
- **Alterar o texto da pergunta para o do briefing.** Vantagem: fidelidade literal ao exemplo. Desvantagem: quebra o Tom de Voz oficial e mensagens verbatim já em produção/testes. Rejeitada.
- **Renderizar a data numa segunda mensagem.** Vantagem: separa layout. Desvantagem: adiciona mensagem sem valor e diverge do padrão de despesa. Rejeitada.

## Consequências

### Benefícios Esperados

- Confirmação mais informativa quando a data importa, sem ruído no caso comum.
- Reuso de `DateFormatted`/`formatConfirmDate` já existentes; mudança mínima e localizada.
- Paridade de comportamento entre confirmação de despesa e de receita.

### Trade-offs e Custos

- Alteração de mensagem em produção: exige atualizar/adicionar testes verbatim/golden que assertam o bloco de confirmação de receita.

### Riscos e Mitigações

- Risco: teste existente que fixa o formato exato do bloco de receita quebrar. Impacto: falso vermelho de regressão. Mitigação: atualizar os asserts para o formato condicional; unit test dedicado para os dois ramos (com e sem data). Rollback: reverter para o formato de dois campos.

## Plano de Implementação

1. Ajustar `IncomeConfirmationBlock` (ramo condicional).
2. Ajustar `confirmSummaryIncome` para preencher `DateFormatted`.
3. Unit test do bloco com e sem `DateFormatted`.
4. Cobrir no golden um caso de receita com data específica (linha `📅 Data` presente) e um sem (ausente).

## Monitoramento e Validação

- Critério de sucesso: bloco de confirmação de receita exibe `📅 Data` só quando há data específica; 0 regressão em testes verbatim/golden de confirmação.
- Sinais: unit tests verdes; golden de confirmação de receita.

## Impacto em Documentação e Operação

- Sem impacto em runbooks; sem nova configuração de observabilidade.

## Revisão Futura

- Revisar se o produto decidir capturar/mostrar outros campos na confirmação de receita (ex.: categoria) ou reintroduzir forma de pagamento (hoje fora de escopo — RF-15).
