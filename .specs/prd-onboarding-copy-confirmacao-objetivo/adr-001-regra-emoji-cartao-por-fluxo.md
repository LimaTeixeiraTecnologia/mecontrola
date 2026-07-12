# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Regra de emoji 💳 por-fluxo (1ª mensagem de cartão + selo de sucesso), escopo onboarding + avulso
- **Data:** 2026-07-12
- **Status:** Aceita
- **Decisores:** Solicitante do produto (decisões por múltipla escolha 2026-07-12), time de plataforma
- **Relacionados:** PRD `.specs/prd-onboarding-copy-confirmacao-objetivo/prd.md` (RF-07, RF-08, RF-11, RF-12, RF-15); techspec da mesma pasta; supersede copy de emoji de `prd-onboarding-cartao-resumo-conclusao` (RF-01/02/03) e refina `prd-cadastro-conversacional-cartao`

## Contexto

Na jornada real e no código, o emoji 💳 é repetido em toda ocorrência da palavra "cartão": 18 aparições no onboarding e 12 no fluxo avulso `card_create_confirm`. O `prd-onboarding-cartao-resumo-conclusao` chegou a exigir 💳 acompanhando a palavra "cartão" em toda mensagem (RF-01/02/03). O solicitante determinou, como regra mandatória e inegociável, que o 💳 apareça apenas no começo, não a cada palavra "cartão". Restrições: não alterar o motor de workflow, a extração via LLM, o system prompt do agente nem a idempotência; preservar os fragmentos de exemplo de cartão já entregues.

## Decisão

Em cada um dos dois fluxos determinísticos de cartão (onboarding `onboarding_workflow.go` e avulso `card_create_confirm_workflow.go`), o emoji 💳 aparece **apenas**: (a) na primeira mensagem de cartão do fluxo, na primeira menção da palavra; e (b) no selo de sucesso do cadastro. É proibido em qualquer outra mensagem/ocorrência (reprompts, convite ao próximo cartão, cancelamento, erros, idempotência, seção de cartões do resumo). No onboarding, a primeira mensagem é o convite de cartão (`cardsPrompt`) e o selo é `💳 Cartão registrado com sucesso ✅`; no avulso, a primeira mensagem é a pergunta de confirmação e o selo é `✅ 💳 *<apelido>* cadastrado com sucesso.`.

Escopo estritamente restrito a esses dois fluxos. O system prompt do agente (`mecontrola_agent.go`, 45× 💳), as tools de cartão, `pending_entry`, `destructive_confirm` e os golden cases **não** são alterados. A regra é resolvida na origem (constantes/`fmt.Sprintf`), sem depender do normalizador de saída.

## Alternativas Consideradas

- **💳 em toda mensagem (status quo entregue)**: consistente com RF-01/02/03 anteriores, mas poluído e explicitamente rejeitado pelo solicitante.
- **1ª menção de cada mensagem (per-message)**: menos poluído, mas ainda repete 💳 muitas vezes no fluxo; rejeitado pelo solicitante em favor de "1ª menção de todo o fluxo".
- **Regra global em todo o produto (inclui system prompt do LLM + golden)**: máxima consistência, mas altíssimo risco (reescrever o "cérebro" do agente e revalidar todos os gates golden); rejeitado por blast radius desproporcional ao pedido.
- **Rastrear a 1ª mensagem via flag de estado**: permitiria 💳 no primeiro prompt de cartão mesmo em sessão retomada; rejeitado por adicionar estado ao `OnboardingState` sem ganho de produto — a regra estática (convite inicial + selo) é suficiente e determinística.

## Consequências

### Benefícios Esperados

- Linguagem de cartão limpa e consistente nos dois fluxos, conforme decisão mandatória.
- Blast radius contido: só copy em 2 arquivos de produção; nenhum impacto no LLM, tools ou golden.
- Sem novo estado, métrica ou dependência.

### Trade-offs e Custos

- Supersede decisões de copy de dois PRDs já entregues (documentado na seção "Relação com PRDs Existentes" do PRD).
- Assimetria intencional: no caminho multi-cartão do onboarding, cada selo de sucesso reintroduz 💳 (autorizado por RF-07(b)); o objetivo "no máximo 2×" vale para o caminho comum 0–1 cartão.
- Atualização de vários asserts de copy (mapeados por file:line na techspec).

### Riscos e Mitigações

- **Risco:** asserts de integração de journey (`replies[6]/[7]` com 💳) podem quebrar. **Mitigação:** reverificar índice a índice; o convite inicial e o selo mantêm 💳. **Rollback:** reverter as strings (mudança isolada, sem migração).
- **Risco:** gate de falso-sucesso do avulso. **Mitigação:** o selo mantém a frase "cadastrado com sucesso" e o 💳; gates permanecem válidos.

## Plano de Implementação

1. Reescrever as constantes/funções de cartão do onboarding (bullets + 💳 só no convite inicial; selo de sucesso).
2. Remover 💳 da seção de cartões do resumo (`Cartões:`, `Nenhum cartão cadastrado.`).
3. Remover 💳 das mensagens do avulso exceto confirmação inicial e selo de sucesso.
4. Atualizar asserts unit/integration; rodar `go test -race` + lint no `internal/agents`.
5. Rodar gate golden real-LLM (`CategoryOnboarding` ≥ 0,90).

## Monitoramento e Validação

- Critério de sucesso: contagem de 💳 por fluxo = 1 (convite/confirmação inicial) + selos de sucesso; 0 em reprompts/cancelamento/erros/idempotência/resumo.
- Sinais: testes determinísticos verdes; gate golden agregado verde; inspeção visual da jornada.
- Critério de revisão: se um novo fluxo de cartão determinístico for criado, aplicar a mesma regra.

## Impacto em Documentação e Operação

- PRD e techspec desta pasta atualizados.
- Sem runbook, alerta ou dashboard afetado (mudança de copy).

## Revisão Futura

- Revisar se o solicitante decidir estender a regra ao system prompt do agente e às tools (fora do escopo atual), o que exigirá novo PRD e revalidação dos gates golden.
