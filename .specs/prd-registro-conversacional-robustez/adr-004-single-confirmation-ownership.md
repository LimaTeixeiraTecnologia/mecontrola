# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Confirmação única — gate HITL do workflow é o dono; LLM nunca confirma
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Time de plataforma MeControla
- **Relacionados:** PRD (RF-14..RF-18, prioridade máxima), techspec.md, `.claude/rules/agent-workflows-tools.md` (Addendum R-AGENT-WF-001.7-A)

## Contexto

Bug de confirmação dupla comprovado (2026-07-08 15:17, livro no pix). Existem **dois emissores** de
confirmação:

1. **LLM (instruções)** em `mecontrola_agent.go:58-66` e `:158-165`: o texto atual diz "Toda escrita
   financeira exige confirmação humana explícita antes de persistir... Aguarde resposta explícita
   sim/não". Isso induz o LLM a **emitir a própria pergunta** de confirmação (resumo pobre, só a
   raiz: "Confirma? R$ 50,00 em Conhecimento...").
2. **Gate HITL do workflow** em `buildConfirmSummary` (`pending_entry_workflow.go:712-731`), o
   emissor legítimo, com path completo + data ("Confirma? ...Conhecimento > Livros e E-books para
   hoje (08/07/2026) no pix?"), decidido por `DecideConfirmation` (`pending_entry_decisions.go:238`)
   e persistido no snapshot (`AwaitingSlotConfirmation`, tipo fechado `AwaitingSlot`).

Sequência do bug: usuário lança → LLM pergunta a própria confirmação → usuário "sim" → LLM chama
`register_expense` → workflow suspende em `AwaitingSlotConfirmation` e retorna `clarify` com
`buildConfirmSummary` → LLM repassa → **segunda** confirmação. O gate do workflow já é durável e
idempotente (snapshot + merge-patch no resume; replay-guard por `ProcessedMessageID`).

## Decisão

O **gate HITL do workflow é o único dono** da confirmação. O LLM nunca emite pergunta de confirmação
própria — apenas repassa, literalmente, o campo `message` retornado pela tool quando
`outcome=clarify`.

Reescrever as instruções do agente (`mecontrola_agent.go`) para:

1. Remover qualquer texto que instrua o LLM a "aguardar sim/não" ou "exigir confirmação antes de
   persistir" como se fosse responsabilidade do LLM. A confirmação é do sistema.
2. Manter/reforçar a REGRA ABSOLUTA DE PENDÊNCIA (`:58-66`): ao receber `outcome=clarify` com
   `message` não-vazio, a resposta ao usuário DEVE ser **exatamente** `message`, sem reescrever,
   resumir, acrescentar ou inventar confirmação.
3. Instruir explicitamente: ao registrar um lançamento, **sempre chamar a tool de escrita
   imediatamente** (com os dados disponíveis); NUNCA formular pergunta de confirmação por conta
   própria. O sistema devolverá o resumo de confirmação via `message` quando necessário.
4. Após o usuário confirmar, NÃO chamar a tool novamente — o sistema (resume do workflow) executa.

O gate permanece: um único `buildConfirmSummary` por lançamento, `AwaitingSlotConfirmation`
persistido no snapshot **antes** de perguntar, resume por merge-patch, `DecideConfirmation` aceitando
"sim/confirmar/ok/pode", cancelando em "não", com replay-guard por `ProcessedMessageID` e reprompt
único. Nenhuma mudança no kernel; a correção é de instruções + garantia de que o único texto de
confirmação vem de `buildConfirmSummary`.

## Alternativas Consideradas

1. **Suprimir o resumo do workflow e deixar o LLM confirmar** — Descartada: viola o Addendum
   R-AGENT-WF-001.7-A (confirmação HITL é do gate durável, não do LLM); o LLM não é durável nem
   idempotente e produz resumo impreciso (só raiz, sem data).
2. **Detectar e deduplicar a segunda confirmação em runtime** — Descartada: trata sintoma, não a
   causa; frágil e dependente de heurística de texto.
3. **Manter as instruções e só melhorar o resumo do LLM** — Descartada: mantém dois donos da
   confirmação; o PRD (prioridade máxima) exige confirmação única e LLM sem confirmação própria.

## Consequências

### Benefícios Esperados

- Confirmação exatamente uma vez por lançamento (RF-15, RF-16); o caso do livro no pix não se repete.
- LLM nunca emite confirmação própria (RF-14).
- Preserva durabilidade/idempotência do gate (RF-17) e cancelamento por "não" (RF-18).

### Trade-offs e Custos

- Depende de aderência do LLM às instruções; mitigado por testes real-LLM e pela regra literal de
  repassar `message`.

### Riscos e Mitigações

- **Risco:** LLM ainda parafrasear a confirmação. **Mitigação:** instrução literal "responda
  EXATAMENTE o conteúdo de message"; teste real-LLM "LLM não emite confirmação própria" e "compra de
  livro no pix não pede confirmação duas vezes" como gate.
- **Risco:** LLM chamar a tool duas vezes. **Mitigação:** instrução "após confirmar, não chame a
  ferramenta novamente"; replay-guard do gate por `ProcessedMessageID` neutraliza duplicação.

## Plano de Implementação

1. Reescrever as seções de confirmação nas instruções (`mecontrola_agent.go`), removendo a
   responsabilidade de confirmação do LLM.
2. Garantir que `buildConfirmSummary` é o único texto de confirmação; verificar os 4 pontos de
   transição para `AwaitingSlotConfirmation` (`:156, :255, :335, :686`).
3. Testes real-LLM de confirmação única e ausência de confirmação do LLM.

## Monitoramento e Validação

- Critérios de aceite: "Compra de livro no pix não pede confirmação duas vezes", "LLM não emite
  confirmação própria", "Cancelamento explícito descarta o lançamento sem gravar".
- Métrica `agents_pending_entry_slot_total{slot="confirmation",outcome}` — observar ausência de
  reprompts espúrios.

## Impacto em Documentação e Operação

- Documentar o contrato "confirmação é do gate, não do LLM" nas instruções do agente e no runbook.

## Revisão Futura

- Reavaliar quando a techspec do `prd-platform-mastra` reemitir os gates HITL (hoje SUPERSEDED como
  caminho literal) apontando para os arquivos reais do consumidor.
