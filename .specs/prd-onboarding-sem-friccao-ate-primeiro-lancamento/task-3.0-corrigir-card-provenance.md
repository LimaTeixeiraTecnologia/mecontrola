# Tarefa 3.0: Corrigir `card_provenance` para pagamentos não-credit_card

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Corrigir o guard `card_provenance` para que ele só exija cartão quando a tool realmente tentar `register_expense` com `paymentMethod == credit_card` e não houver resolução prévia de cartão. Preservar a ordem dos guards (`verbatim_relay` antes de `card_provenance`) e não sobrescrever confirmações verbatim de pagamentos que não usam cartão.

<requirements>
- RF-07: toda mensagem, critério, teste e copy sobre cartão usa o emoji 💳.
- RF-08: não introduzir outro emoji para cartão.
- RF-16: pagamentos pix, dinheiro, boleto, ted, débito, debit_card, debit_in_account, cash, vale_refeicao ou vale_alimentacao não podem pedir 💳.
- RF-17: despesa pix com valor, descrição, categoria e data resolvida deve pedir confirmação antes de persistir.
- RF-18: após confirmação positiva de despesa pix, persistir transação ativa com origem e categoria rastreáveis.
- RF-19: ausência de cartão ativo não impede confirmação nem persistência de despesa pix.
</requirements>

## Subtarefas

- [ ] 3.1 Inspecionar sequência de tool calls e argumentos da tool consumidora em `card_provenance`.
- [ ] 3.2 Restringir ação do guard a `register_expense` com `paymentMethod == "credit_card"` sem `resolve_card`/`list_cards` anterior.
- [ ] 3.3 Garantir que `verbatim_relay` continue podendo corrigir a resposta antes de `card_provenance`.
- [ ] 3.4 Adicionar testes por payment method cobrindo pix, cash, boleto, ted, debit_card, debit_in_account, vale_refeicao, vale_alimentacao e credit_card.
- [ ] 3.5 Validar que `card_provenance` não sobrescreve confirmação verbatim não relacionada a credit_card.

## Detalhes de Implementação

Ver `techspec.md` — seção **Visão Geral dos Componentes** (guard) e **Testes Unitários / Guards e agente**. O guard inspeciona argumentos da tool de forma estrita; parser de argumentos deve rejeitar ambiguidade. A ordem dos guards em `guard_chain.go` é contrato testado.

## Critérios de Sucesso

- Teste unitário confirma que `card_provenance` não trata `register_expense` quando `paymentMethod` é pix, cash, boleto, ted, debit_card, debit_in_account, vale_refeicao ou vale_alimentacao.
- Teste unitário confirma que `card_provenance` trata `register_expense` com `paymentMethod=credit_card` e sem `resolve_card`/`list_cards` anterior.
- Teste unitário confirma que `verbatim_relay` pode corrigir resposta antes de `card_provenance`.
- `go test -race -count=1 ./internal/agents/application/agents/...` passa.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alteração em guard pós-tool do agente consumidor.

## Testes da Tarefa

- [ ] Testes unitários do guard por payment method.
- [ ] Testes de interação entre `verbatim_relay` e `card_provenance`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/guards/card_provenance.go`
- `internal/agents/application/agents/guards/card_provenance_test.go`
- `internal/agents/application/agents/guard_chain.go`
- `internal/agents/application/agents/mecontrola_agent.go`
