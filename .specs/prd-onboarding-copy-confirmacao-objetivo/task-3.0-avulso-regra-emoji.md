# Tarefa 3.0: Avulso card_create_confirm — regra de 💳 (mantém confirmação inicial + selo de sucesso)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Aplicar a regra de emoji 💳 ao fluxo avulso `internal/agents/application/workflows/card_create_confirm_workflow.go`: manter 💳 apenas na pergunta de confirmação inicial e no selo de sucesso; remover 💳 de todas as demais mensagens. Preservar o caráter single-shot e a lógica de confirmação/idempotência/TTL/reprompt.

<requirements>
- RF-07 (parte avulso): 💳 apenas na 1ª mensagem do fluxo (pergunta de confirmação) e no selo de sucesso.
- RF-15: remover 💳 de reprompt, cancelamento, erros de domínio, erros de infraestrutura e idempotência ("já estava cadastrado"); manter 💳 na pergunta de confirmação (linha 94) e no selo de sucesso (linha 155, "✅ 💳 *<apelido>* cadastrado com sucesso."); fluxo permanece single-shot.
</requirements>

## Subtarefas

- [ ] 3.1 Remover 💳 (trocando por "cartão" onde apropriado) nas mensagens: cancelamento (60), reprompt (65), cancelamento ambíguo (87), erros de infra (132, 145), idempotência (153), erros de domínio (179, 181, 187, 189). Linhas 183/185 já não têm 💳.
- [ ] 3.2 Manter inalteradas: pergunta de confirmação (94) e selo de sucesso (155).
- [ ] 3.3 Adicionar/atualizar asserts unitários: reprompt/cancelamento/erros/idempotência NÃO contêm 💳; confirmação inicial e selo de sucesso CONTÊM 💳; frase "cadastrado com sucesso" preservada (gate de falso-sucesso permanece válido).

## Detalhes de Implementação

Ver techspec.md, seção "Strings Concretas" (bloco "Avulso") e "Abordagem de Testes". As strings exatas por linha estão na techspec; não rewordar "cadastrado com sucesso" (preserva o gate `card_create_harness_test.go`).

## Critérios de Sucesso

- No avulso, 💳 aparece só na confirmação inicial e no selo de sucesso; 0 em reprompt/cancelamento/erros/idempotência.
- Fluxo permanece single-shot; lógica de confirmação/idempotência/TTL/reprompt inalterada.
- `go build`/`vet`/`test -race`/lint verdes; gates de falso-sucesso do avulso permanecem verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — a mudança vive no workflow avulso de cartão do consumidor de agente sobre o substrato Mastra; copy apenas, sem alterar a lógica de confirmação.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/card_create_confirm_workflow.go` (produção)
- `internal/agents/application/workflows/card_create_confirm_workflow_test.go` (asserts RF-15)
