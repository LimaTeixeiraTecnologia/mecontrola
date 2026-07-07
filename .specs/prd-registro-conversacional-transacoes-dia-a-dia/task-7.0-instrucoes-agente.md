# Tarefa 7.0: Endurecimento das instruções do agente `mecontrola`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Atualizar a `const mecontrolaAgentInstructions` para refletir o comportamento final: cinco campos obrigatórios sem invenção, repasse do texto de data (incluindo dias da semana) sem conversão pelo LLM, fronteira multi-item (pedir um por vez), e reforço do mapeamento de pagamento e formatação WhatsApp já presentes. Depende de 4.0/5.0/6.0 para não descrever comportamento não implementado.

<requirements>
- RF-01, RF-21: cinco campos obrigatórios; nunca inventar valor/data/categoria/subcategoria/cartão/forma de pagamento; não inferir de memória.
- RF-02: reforçar mapeamento de formas de pagamento (manter enum existente).
- RF-03: subcategoria folha obrigatória (despesa e receita).
- RF-09: data assumida "hoje" aparece explícita no resumo; sem passo extra de confirmação de data.
- RF-11, RF-12, RF-13: protocolo de categorização (classify_category; thresholds 0,80/0,55; nunca chutar).
- RF-14, RF-15: resolução de cartão sem falso positivo; parcelas 1..24, default 1.
- RF-16: múltiplos lançamentos numa mensagem → pedir um por vez, sem registrar nada.
- RF-17, RF-18: resumo de confirmação por tipo; aceitar/cancelar.
- RF-22, RF-23: idioma pt-BR, emojis contextuais, `*asterisco simples*`, valores `R$ x,yz`, datas `DD/MM/YYYY`.
</requirements>

## Subtarefas

- [ ] 7.1 Adicionar bloco dos cinco campos obrigatórios + regra de não-invenção (RF-01, RF-21).
- [ ] 7.2 Instruir o repasse do texto de data cru em `occurredAt` (incluindo "terça", "segunda passada"), sem o LLM converter a data (RF-06, RF-07, RF-09).
- [ ] 7.3 Adicionar a fronteira multi-item: ao detectar mais de um lançamento, pedir um por vez, sem registrar nada (RF-16).
- [ ] 7.4 Reforçar mapeamento de pagamento (enum), thresholds de categoria, resolução de cartão, parcelas e formato de confirmação/formatação (RF-02, RF-03, RF-11..RF-18, RF-22, RF-23).
- [ ] 7.5 Verificar coerência com as instruções já existentes (não duplicar/contradizer as regras de asterisco e confirmação atuais).

## Detalhes de Implementação

Ver `techspec.md` › **Instruções do agente** e `prd.md` › Requisitos Funcionais e Exemplos de diálogo (R1–R8, incluindo o fallback multi-item verbatim). Não duplicar; adaptar ao tom já existente.

## Critérios de Sucesso

- Instruções cobrem os cinco campos, repasse de data, fronteira multi-item, pagamento, categorização, cartão, confirmação e formatação, sem contradizer regras existentes.
- Nenhuma regra instrui o LLM a converter data ou inventar campo.
- `go build` limpo; a `const` continua sem `**duplo asterisco**`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — altera as instruções (system prompt) do agente `mecontrola` do stack Mastra Go.

## Testes da Tarefa

- [ ] Testes unitários (asserções sobre a `const`: presença de blocos-chave, ausência de `**`)
- [ ] Testes de integração (comportamento verificado via real-LLM em 8.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/mecontrola_agent.go` — `const mecontrolaAgentInstructions`.
