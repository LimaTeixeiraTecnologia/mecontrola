# Registro de Decisão Arquitetural (ADR-001)

## Metadados

- **Título:** Resolução determinística de data por extensão da função pura `ParseInputDate` com contrato verbatim LLM→tool
- **Data:** 2026-07-30
- **Status:** Aceita
- **Decisores:** Solicitante do produto (jailton.junior94), engenharia de plataforma
- **Relacionados:** PRD `.specs/prd-lancamento-edicao-receitas/prd.md` (RF-06, RF-22); techspec `.specs/prd-lancamento-edicao-receitas/techspec.md`; US `docs/us/2026-07-30-lancamento-e-edicao-de-receitas.md` (RN-04, RN-19)

## Contexto

- `ParseInputDate` (write_shared.go:295) resolve deterministicamente `hoje`, `ontem`, `anteontem`, dia da semana e `DD/MM`, mas retorna `""` para `semana passada`, `mês passado` e `dia N`. Nesse caso, `resolveEntryDate` (register_entry.go:111) cai silenciosamente para o dia corrente, gravando data errada sem avisar o usuário.
- Para uma receita já recebida (fato passado), gravar a data do dia corrente ou uma data futura é um defeito de correção, não apenas de UX.
- A função é pura e recebe `now` como parâmetro (sem abstração de tempo), então é possível resolver aritmética de calendário de forma determinística e testável sem IO.
- O LLM não conhece a data atual de forma confiável; delegar a ele o cálculo de datas relativas é frágil e difícil de provar.

## Decisão

1. Estender a função pura `ParseInputDate(text, now)` com três ramos determinísticos, mantendo a assinatura atual e a ordem de avaliação (novos ramos antes do `return ""`):
   - `semana passada` → `now.AddDate(0, 0, -7)`.
   - `mês passado`/`mes passado` → mesmo dia-do-mês do mês anterior, ajustado ao último dia válido quando o dia não existir (ex.: 31/03 → 28 ou 29/02).
   - `dia N` (regex ancorada `^\s*dia\s+([0-9]{1,2})\s*$`, N em 1..31) → a ocorrência mais recente com dia-do-mês N que não seja futura, varrendo até 12 meses para trás; `dia 31` num mês de 30 dias resolve para o mês anterior com 31 dias.
2. Introduzir helpers puros no pacote `workflows`: `resolveSameDayPreviousMonth`, `resolveMostRecentDayOfMonth`, `daysInMonth` — determinísticos, sem IO, sem `context.Context`, recebendo `now`.
3. Fixar o **contrato de data LLM→tool**: as ferramentas `register_income` e `edit_entry` recebem em `occurredAt` a **expressão de data verbatim do usuário** (`semana passada`, `mês passado`, `dia 10`, `ontem`, ...); o LLM não computa data ISO. A resolução é responsabilidade exclusiva de `ParseInputDate`. As descrições das propriedades `occurredAt` nos schemas passam a instruir esse comportamento.

## Alternativas Consideradas

- **LLM computa e envia data ISO (AAAA-MM-DD).** Vantagem: dispensa estender `ParseInputDate`. Desvantagem: coloca aritmética de calendário no LLM, que não sabe `now` com confiabilidade, produzindo datas erradas e difíceis de provar. Rejeitada por robustez e testabilidade.
- **Novo tipo/função `DecideEntryDate` separada.** Vantagem: DMMF mais explícito. Desvantagem: duplica o ponto de entrada já existente (`ParseInputDate`) sem ganho de robustez; adiciona abstração sem necessidade (contraria a economia de design). Rejeitada.
- **Clamp de `dia N` ao último dia do mês corrente.** Vantagem: mais simples. Desvantagem: altera o dia informado e pode gerar data futura/incorreta. Rejeitada em favor da varredura para a ocorrência real mais recente não-futura.

## Consequências

### Benefícios Esperados

- Elimina o fallback silencioso para o dia corrente — correção do defeito latente de data.
- Lógica determinística, pura e testável por unit sem mock, cobrindo fronteiras de calendário.
- Reuso do único ponto de entrada de data já compartilhado por lançamento e edição.

### Trade-offs e Custos

- Mais ramos e helpers em `write_shared.go`, com custo de teste de fronteira (ano bissexto, viradas de mês/ano).
- Depende de o LLM enviar a expressão verbatim; mitigado por descrição de schema + golden.

### Riscos e Mitigações

- Risco: colisão com o sufixo `passada`/`passado` de `parseWeekday`. Impacto: `semana passada` resolver errado. Mitigação: `parseWeekday` retorna `false` para `semana`/`mes`; unit tests cobrem ambos. Rollback: reverter os ramos novos restaura o comportamento anterior (fallback), sem quebrar as formas já suportadas.

## Plano de Implementação

1. Implementar helpers puros + ramos em `ParseInputDate`.
2. Unit tests table-driven com `now` fixo, incluindo bordas.
3. Ajustar descrições `occurredAt` em `register_income` e `edit_entry`.
4. Provar fim-a-fim com golden real-LLM (datas novas resolvidas ≠ dia corrente).

## Monitoramento e Validação

- Critério de sucesso: 0 receita gravada com data futura para entrada já recebida; golden de datas novas ≥ 0,90, 0 falso-sucesso.
- Sinais: Run auditável com a data resolvida; unit tests verdes.
- Revisão: caso surja necessidade de novas expressões de data (ex.: `retrasado`, intervalos), reavaliar em addendum.

## Impacto em Documentação e Operação

- Atualizar exemplos de uso do agente quando houver documentação de comandos conversacionais.
- Sem mudança em runbooks de operação; sem nova configuração de observabilidade.

## Revisão Futura

- Revisar se surgirem novas formas de data relativas recorrentes em produção ou se o contrato verbatim LLM→tool apresentar desvio medido pelo golden.
