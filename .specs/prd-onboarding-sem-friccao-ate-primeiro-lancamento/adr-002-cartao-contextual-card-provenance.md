# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** `💳` contextual no onboarding e no guard de proveniência
- **Data:** 2026-07-11
- **Status:** Aceita
- **Decisores:** Requester, Codex
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD exige que `💳` seja opcional no onboarding e bloqueante apenas para lançamento com `paymentMethod=credit_card`. A exploração do codebase mostrou que `pending-entry` já decide corretamente: cartão só é solicitado quando `paymentMethod == "credit_card"` e não há cartão. O risco principal está no guard `card_provenance`, que hoje trata tool consumidora sem resolução prévia de cartão como falta de cartão, independentemente do meio de pagamento.

## Decisão

O guard `card_provenance` deve inspecionar os argumentos das tool calls consumidoras antes de forçar pergunta de `💳`. Para `register_expense`, a pergunta de `💳` só pode ser forçada quando `paymentMethod == "credit_card"` e não houve `resolve_card` ou `list_cards` antes. Para meios pix, dinheiro, boleto, TED, débito, vale refeição, vale alimentação ou receita, o guard deve passar.

No onboarding, `💳` continua opcional. Respostas com um único nome de banco/apelido e vencimento válido devem preencher `Nickname` e `Bank` com o mesmo valor antes de `CreateCard`.

## Alternativas Consideradas

- Remover `card_provenance`: eliminaria falso bloqueio, mas perderia proteção real contra lançamentos de cartão de crédito sem proveniência.
- Exigir cartão para toda despesa: contradiz o PRD e bloqueia pix/dinheiro/boleto/débito/TED.
- Resolver apenas no prompt do agente: mantém risco porque guard pós-tool pode sobrescrever confirmação correta.

## Consequências

### Benefícios Esperados

- Pix e receitas chegam à confirmação sem pergunta indevida de `💳`.
- Lançamento em cartão de crédito continua protegido.
- Confirmações verbatim das tools deixam de ser sobrescritas quando não envolvem `credit_card`.

### Trade-offs e Custos

- O guard passa a depender do parse de argumentos da tool call.
- Testes precisam cobrir meios de pagamento explicitamente para evitar regressão silenciosa.

### Riscos e Mitigações

- Risco: argumento ausente ou inválido em `register_expense`.
- Mitigação: tratar ausência de `paymentMethod` como não suficiente para exigir `💳`; a tool/pending-entry deve pedir o slot correto.

- Risco: ordem de post-guards sobrescrever verbatim.
- Mitigação: manter `verbatim_relay` antes de `card_provenance` e testar que `card_provenance` passa para não-credit.

## Plano de Implementação

1. Adicionar parser local e estrito dos argumentos relevantes de `register_expense`.
2. Restringir `consumerWithoutPriorResolution` a consumidor que realmente use `credit_card`.
3. Atualizar `card_provenance_test.go` com matriz de payment methods.
4. Atualizar testes de tool/agent para pix, receita e cartão de crédito.

## Monitoramento e Validação

- `agent_guard_decisions_total{guard="card_provenance",decision="handled"}` não pode crescer em jornadas pix/receita.
- Golden de pix sem `💳` deve passar.
- Golden de `credit_card` sem resolução deve continuar pedindo cartão.

## Impacto em Documentação e Operação

- Atualizar runbook de incidentes de falso bloqueio de cartão.
- Registrar que `💳` é obrigatório apenas para `credit_card`.

## Revisão Futura

Revisar se novas tools financeiras passarem a consumir cartão de crédito ou se o schema de `register_expense` mudar.
