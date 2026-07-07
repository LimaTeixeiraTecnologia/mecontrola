# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Parser de dias da semana como função pura determinística estendendo `parseInputDate`
- **Data:** 2026-07-07
- **Status:** Aceita
- **Decisores:** time de plataforma / agentes
- **Relacionados:** `prd.md` (RF-06, RF-07, RF-08; Decisão D-02), `techspec.md`, `.agents/skills/go-implementation/references/domain-modeling.md` (Princípio 6: pure core / IO shell)

## Contexto

`internal/agents/application/workflows/pending_entry_decisions.go` já contém `parseInputDate(text string, now time.Time) string` — **função pura** (recebe `now`, não chama `time.Now()`), que resolve `hoje`/`ontem`/`anteontem`, `DD/MM` e `YYYY-MM-DD`, retornando `""` (sentinel) quando não reconhece. A PRD exige (RF-07) interpretar **dias da semana** ("segunda".."domingo", com variante "X passada/passado") e (RF-08) **rejeitar** expressões de baixa precisão ("semana passada", "mês passado") pedindo data específica.

Duas abordagens competem para resolver datas em linguagem natural: delegar ao LLM a extração da data absoluta, ou estender o parser determinístico local. O contexto exige **zero alucinação de data** (M-03) e determinismo testável.

## Decisão

Estender o parser **determinístico e puro**, adicionando uma etapa de dias da semana a `parseInputDate`, antes do fallback de formato explícito:

- Nova função pura `parseWeekday(text string, now time.Time) (date string, ok bool)`:
  - Reconhece nomes de dia da semana em pt-BR normalizados (`normalizeText` já remove acento/caixa): `segunda`/`segunda-feira`, `terca`, `quarta`, `quinta`, `sexta`, `sabado`, `domingo`.
  - Sem sufixo → ocorrência **mais recente** daquele dia da semana, **incluindo hoje** se `now` já for aquele dia.
  - Sufixo `passada`/`passado` → a ocorrência corrente menos 7 dias.
  - Fuso: usa `now.Location()` (o chamador injeta `America/Sao_Paulo`), preservando o padrão atual.
  - Retorna `("", false)` quando o texto não é um dia da semana reconhecido.
- `parseInputDate` chama `parseWeekday` **somente** quando o token for de dia da semana; expressões como "semana passada"/"mês passado" **não** casam (não contêm nome de dia da semana) e caem no retorno `""` — que, no fluxo atual, leva o passo a **suspender e pedir data específica**, cumprindo RF-08 sem branch especial.
- O LLM continua responsável apenas por **repassar o texto de data cru** (ex.: "terça") no campo `occurredAt`; a resolução para `YYYY-MM-DD` é do parser puro. O agente nunca converte a data por conta própria (anti-alucinação).

## Alternativas Consideradas

- **Resolução de data pelo LLM (occurredAt já em `YYYY-MM-DD`).** Vantagem: menos código. Desvantagens: não determinístico, sujeito a alucinação de data (viola M-03), difícil de testar isoladamente, dependente do "hoje" que o LLM assume. **Rejeitada.**
- **Biblioteca de NLP de datas (ex.: parser tipo `dateparser`).** Vantagem: cobre mais expressões. Desvantagens: dependência externa nova, comportamento opaco, difícil de auditar e fixar em `America/Sao_Paulo`; excede a necessidade (a PRD delimita o vocabulário). **Rejeitada.**
- **Rejeição explícita de "semana/mês passado" com branch dedicado + mensagem própria.** Vantagem: mensagem mais específica. Desvantagem: duplica o caminho de "não reconheci a data" que já suspende e pergunta. **Rejeitada** em favor do sentinel `""` reutilizado; a mensagem de clarificação de data já cobre o caso.

## Consequências

### Benefícios Esperados

- Determinismo total: mesma entrada + mesmo `now` → mesma data; testável sem IO nem LLM.
- Cumpre RF-07/RF-08 reutilizando o sentinel e o fluxo de suspensão existentes.
- Anti-alucinação de data preservada (M-03).

### Trade-offs e Custos

- Vocabulário limitado ao definido (dias da semana + relativos já existentes + formatos explícitos); outras expressões caem em "pedir data específica" — comportamento intencional.

### Riscos e Mitigações

- **Risco:** ambiguidade "segunda" = próxima ou passada. **Mitigação:** contrato fixado (sem sufixo = ocorrência mais recente incluindo hoje; "passada" = −7). Documentado e coberto por testes de tabela com `now` fixo em vários dias da semana.
- **Risco:** "sábado"/"sabado" e hífen ("segunda-feira") não normalizados. **Mitigação:** testar variações acentuadas e com "-feira" via `normalizeText`.
- **Rollback:** remover `parseWeekday` e sua chamada reverte ao parser atual sem efeito colateral.

## Plano de Implementação

1. Implementar `parseWeekday(text, now)` pura em `pending_entry_decisions.go`.
2. Encaixar a chamada em `parseInputDate` antes do fallback de formato explícito.
3. Testes de tabela cobrindo cada dia da semana, "X passada/passado", variações de acento/hífen, e não-casamento de "semana/mês passado" (retorna `""`).

## Monitoramento e Validação

- Sem métrica nova; validação por testes unitários puros + cenários real-LLM (R2 usa "ontem"; adicionar cenário com dia da semana).
- Critério de sucesso: 100% dos casos de tabela verdes; nenhum caso de "semana/mês passado" produzindo data.

## Impacto em Documentação e Operação

- Instruções do agente (`mecontrola_agent.go`) devem explicitar que o agente **repassa** o texto de data (incluindo dia da semana) sem convertê-lo.

## Revisão Futura

- Revisitar se surgir demanda por expressões adicionais ("depois de amanhã", "início do mês") — cada uma entra como caso puro adicional, nunca via LLM.
