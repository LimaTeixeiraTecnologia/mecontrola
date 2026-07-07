# Tarefa 6.0: Correção de `knownPaymentMethods` + gate `map × ParsePaymentMethod`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Corrigir um bug latente: o mapa `knownPaymentMethods` traduz formas de pagamento para valores que **não são códigos válidos** do VO `PaymentMethod` (ex.: `"boleto" → "bank_slip"`, `"ted" → "bank_transfer"`), fazendo `ParsePaymentMethod` falhar no slot de clarificação de pagamento para métodos in-scope da PRD. Alinhar os valores aos códigos exatos do VO e adicionar um gate de teste que impede regressão.

<requirements>
- RF-01: registro exige forma de pagamento resolvida sem erro para os métodos suportados.
- RF-02: mapeamento de formas de pagamento para códigos válidos (`pix`, `cash`, `debit_card`, `debit_in_account`, `credit_card`, `boleto`, `vale_refeicao`, `vale_alimentacao`).
- techspec › Correção de `knownPaymentMethods`: alinhar valores in-scope; gate `map → ParsePaymentMethod`.
</requirements>

## Subtarefas

- [ ] 6.1 Corrigir os valores in-scope de `knownPaymentMethods` para os códigos exatos aceitos por `ParsePaymentMethod` do VO (`boleto`, `ted`, `debit_in_account`, etc.), conferindo com `internal/transactions/domain/valueobjects/payment_method.go`.
- [ ] 6.2 Adicionar teste de tabela assertando que **todo valor** de `knownPaymentMethods` é aceito por `ParsePaymentMethod` (gate anti-regressão).
- [ ] 6.3 Documentar no diff (mensagem/PR, não comentário em código) que entradas fora do escopo da PRD (`ted`/`doc`/`transferencia` colapsadas em `bank_transfer`) permanecem como risco conhecido a tratar em iniciativa própria — não silenciar.

## Detalhes de Implementação

Ver `techspec.md` › **Correção de `knownPaymentMethods`** e **Riscos Conhecidos**. Não duplicar.

## Critérios de Sucesso

- Todo valor de `knownPaymentMethods` in-scope parseia para um `PaymentMethod` válido do VO.
- O gate de teste `map × ParsePaymentMethod` está verde e falharia se um valor divergir.
- `go build`/`go vet` limpos; zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — corrige a decisão de mapeamento de pagamento do workflow `pending-entry` do consumidor agentivo.

## Testes da Tarefa

- [ ] Testes unitários (gate `map × ParsePaymentMethod`; slot de clarificação resolve método in-scope)
- [ ] Testes de integração (n/a — comportamento validado em 8.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/pending_entry_decisions.go` — `knownPaymentMethods`.
- `internal/transactions/domain/valueobjects/payment_method.go` — códigos válidos (fonte do gate).
