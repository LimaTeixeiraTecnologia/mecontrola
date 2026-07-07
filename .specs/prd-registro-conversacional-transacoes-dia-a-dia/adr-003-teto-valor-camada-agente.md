# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Teto de valor (sanity-check) na camada do agente, sem alterar o VO `Money`
- **Data:** 2026-07-07
- **Status:** Aceita
- **Decisores:** time de plataforma / agentes
- **Relacionados:** `prd.md` (RF-04, RF-05; Decisão D-01), `techspec.md`, `.claude/rules/transactions-workflows.md` (R-TXN-002)

## Contexto

A User Story afirmava um limite de "R$ 99.999.999,99", mas o VO `valueobjects.Money` (`internal/transactions/domain/valueobjects/money.go`) valida **apenas** `cents > 0`, sem teto — o limite superior real é o de `int64`. O agente precisa rejeitar valores irreais com mensagem amigável (RF-05), mas o VO `Money` é compartilhado por `transactions`, `budgets` e `card`; alterar seu invariante teria blast radius em todo write financeiro do sistema.

R-TXN-002 exige que validação de invariante de domínio viva em smart constructors dos VOs/commands. Um **teto de sanity** contra digitação/alucinação de valor não é invariante de domínio de `Money` (não há regra de negócio dizendo que R$ 10.000.001 é inválido como quantia) — é uma **guarda de experiência conversacional** na fronteira do agente.

## Decisão

Implementar o teto como **guarda pura na camada da aplicação agentiva**, sem tocar o VO `Money`:

- Constante `maxEntryAmountCents int64` na camada de tools/usecases do agente (default `1_000_000_000` = R$ 10.000.000,00).
- Função pura `validateEntryAmount(cents int64) error` (ou sentinel dedicado) que rejeita `cents <= 0` e `cents > maxEntryAmountCents`.
- Chamada no **exec das tools de registro** (`register_expense`, `register_income`), antes de acionar `RegisterAttempt`, retornando uma resposta amigável de correção (outcome de clarificação), **não** um erro de schema/tool. Assim o usuário recebe "esse valor parece muito alto, pode confirmar?" em vez de uma falha técnica.
- O VO `Money` permanece intacto: continua sendo a autoridade final de `cents > 0` no domínio; a guarda do agente é defesa adicional na borda, alinhada a "pure core / IO shell".

## Alternativas Consideradas

- **Adicionar teto ao smart constructor de `Money`.** Vantagem: um único ponto. Desvantagem: muda invariante de domínio compartilhado (transactions/budgets/card), podendo rejeitar quantias legítimas em outros contextos (ex.: importação, ajustes); alto blast radius. **Rejeitada** (D-01).
- **Somente `maximum` no JSON schema da tool.** Vantagem: declarativo. Desvantagem: violação de schema vira erro de tool (experiência ruim) e o valor vem do usuário — melhor capturar e responder com clareza. **Rejeitada como mecanismo único** (pode complementar, mas a guarda pura é a fonte de verdade da mensagem).
- **Sem teto (só `> 0`).** **Rejeitada:** deixa passar valores claramente irreais, contra RF-05.

## Consequências

### Benefícios Esperados

- Rejeição amigável de valores irreais (RF-05) sem risco de regressão no domínio financeiro.
- Guarda pura, determinística e testável isoladamente.

### Trade-offs e Custos

- Teto vive na borda do agente; se outra superfície de escrita for criada fora do agente, ela não herda o teto (aceitável — é uma guarda conversacional, não invariante de domínio).
- Valor default configurável; ajuste não requer mudança de RF.

### Riscos e Mitigações

- **Risco:** teto baixo demais bloquear compra legítima cara. **Mitigação:** default alto (R$ 10.000.000,00); a guarda pede confirmação em vez de bloquear em definitivo (mensagem de correção, não veto absoluto).
- **Rollback:** remover a chamada da guarda reverte ao comportamento atual (`Money > 0`).

## Plano de Implementação

1. Definir `maxEntryAmountCents` e `validateEntryAmount(cents)` puros na camada do agente.
2. Chamar no exec de `register_expense` e `register_income` antes de `RegisterAttempt`.
3. Testes de tabela: `0`, negativo, no teto, acima do teto, valor normal.

## Monitoramento e Validação

- Sem métrica nova obrigatória; opcionalmente um counter enum-fechado de rejeições por motivo (`amount_non_positive`, `amount_above_ceiling`) — sem `user_id` como label.
- Critério de sucesso: cenários de valor inválido rejeitados com mensagem amigável; nenhum valor acima do teto persistido.

## Impacto em Documentação e Operação

- Instruções do agente podem mencionar que valores muito altos pedem confirmação; runbook registra a constante e seu default.

## Revisão Futura

- Revisitar o default caso surja caso de uso legítimo acima de R$ 10.000.000,00 recorrente; tornar configurável por ambiente se necessário.
