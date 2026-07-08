# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Estado do valor da meta como `int64` sentinela + flag booleana `GoalValueAsked` (DMMF state-as-type)
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Jailton (owner), técnica via subagente domain-modeling-production
- **Relacionados:** [prd.md](prd.md) (RF-02, RF-03.3, RF-07, RF-08, RF-10), [techspec.md](techspec.md), [adr-001-extracao-combinada-sentinela.md](adr-001-extracao-combinada-sentinela.md), `.claude/rules/governance.md`, R-WF-KERNEL-001.7

## Contexto

O valor da meta é opcional (RF-02): ausência/zero/negativo/recusa são estados **válidos** (não erro), diferentemente da renda (`DecideIncomeCents` rejeita `<=0`). O valor precisa sobreviver no `OnboardingState` (serializado no `Snapshot.State` do kernel) do `step-goal` até o `step-conclusion` (RF-10), e o sistema precisa lembrar que a repergunta única de valor já foi feita (RF-03.3). A governança DMMF (`governance.md`) exige state-as-type, smart constructor com validação única, e proíbe `Result`/`Either`/`Option`/currying/DSL. O resume do kernel aplica JSON merge-patch parcial (RFC 7386, `codec.go:48-71`): chaves ausentes no patch são preservadas.

## Decisão

1. **Valor persistido como `int64` sentinela**: campo `GoalValueCents int64` (`json:"goalValueCents"`), onde `0` = não informado e qualquer positivo = informado (centavos). Espelha `IncomeCents int64`.
2. **Constructor puro `DecideGoalValueCents(hasAmount bool, amountBRL float64) (int64, bool)`** (novo, distinto de `DecideIncomeCents`, RF-08): retorna `(0, false)` para ausência (sem `error`, pois ausência é válida); `(round(amountBRL*100), true)` para informado. Sem IO, sem `context`, determinístico. A entrada `hasAmount` vem do schema (ADR-001); a **camada de estado** guarda apenas o sentinela `int64` (o `bool` de retorno é comma-ok idiomático, não um `Option`).
3. **Flag "asked once" como booleano fechado**: campo `GoalValueAsked bool` (`json:"goalValueAsked"`). Transição monotônica `false→true` no momento em que o step emite qualquer repergunta de valor (combinada ou específica). Bloqueia segunda pergunta (RF-03.3).
4. **Proibido `omitempty`** nesses dois campos.

Justificativa do sentinela `0`: o domínio do valor da meta é **estritamente positivo** (RF-07). `0` nunca é um valor legal de meta, logo não há colisão semântica entre "meta de zero" e "não informado" — o estado ilegal (meta=0) já é inexprimível no domínio, tornando o sentinela seguro sem violar "make illegal states unrepresentable". Um `bool` para `GoalValueAsked` é suficiente: os estados "perguntou-e-respondeu" e "perguntou-e-recusou" são deriváveis de `(GoalValueAsked, GoalValueCents)`; um enum de 3+ estados adicionaria estado redundante/ilegal. Todas as 4 combinações do produto `(asked, cents)` são legais — inclusive `(false, >0)`, que é o caminho feliz de extração inline (RF-01).

## Alternativas Consideradas

- **`(cents int64, informed bool)` como par de estado persistido**: Desvantagem: cria estado ilegal representável (`(500,false)`, `(0,true)`) exigindo invariante extra. Rejeitada — aumentaria a superfície de estado ilegal. (O `bool` existe apenas como retorno comma-ok do constructor, não como campo de estado.)
- **Tipo-valor fechado `GoalValue` com `IsInformed()`**: Desvantagem: custo cognitivo/manutenção desproporcional para um `int64` positivo sem invariante de transição. `governance.md` manda encapsular primitivo só quando carrega invariante de domínio; aqui o invariante ("positivo ou ausente") é capturado pelo sentinela. Rejeitada por economia.
- **Enum `iota` para `GoalValueAsked`**: rejeitada — não há terceiro estado material; `design-patterns-mandatory` retornou `reject` para State pattern. Bool + branching na closure é a escolha econômica.
- **`Option[int64]`/`Maybe`**: rejeitada — anti-padrão DMMF proibido em `governance.md`.

## Consequências

### Benefícios Esperados

- Estado mínimo e ortogonal; toda combinação `(asked, cents)` é legal e bem-definida.
- Serialização trivial no `Snapshot.State` (tipos JSON primitivos), idêntica a `IncomeCents`/`CardsDone`.
- Validação única no constructor puro; nenhuma regra de valor vaza para o step/adapter.

### Trade-offs e Custos

- Sentinela `0` exige disciplina de checar `> 0` na persistência condicional (RF-11/RF-12) — checagem trivial e localizada.

### Riscos e Mitigações

- **R1 — patch de estado inteiro no resume zeraria os campos**: se um caller emitir resume com o `OnboardingState` inteiro re-serializado, zero-values sobrescreveriam valor/flag. Mitigação: manter contrato de patch parcial (`{"resumeText":...}`), exigido por R-WF-KERNEL-001.7; **teste de regressão** que faz merge-patch parcial sobre snapshot com `goalValueCents>0`/`goalValueAsked=true` e verifica preservação.
- **R2 — `omitempty` apagaria `0`/`false` do encode**: proibido `omitempty` nesses campos; tags exatas `json:"goalValueCents"`/`json:"goalValueAsked"`, espelhando `IncomeCents`/`CardsDone`.

## Plano de Implementação

1. Adicionar os dois campos a `OnboardingState` (sem `omitempty`).
2. Adicionar `DecideGoalValueCents` junto a `DecideGoal`/`DecideIncomeCents`.
3. Teste unitário puro do constructor (tabela input→output) + teste de preservação em merge-patch.
4. Consumir nos steps (ver techspec).

## Monitoramento e Validação

- Critério de sucesso: testes unitários do constructor e de preservação em resume verdes; harness real-LLM (ADR-003) confirma valores esperados em centavos.
- Sinal de revisão: se surgir um estado de espera adicional (ex.: confirmação HITL do valor), reavaliar se `bool` ainda basta ou se vira tipo fechado.

## Impacto em Documentação e Operação

- Nova chave `objetivo_financeiro_valor_centavos` no `metadata` de `platform_resources` — documentar como contrato para consumidores futuros (alertas/orçamento) quando forem construídos. Sem migration.

## Revisão Futura

- Revisitar quando um consumidor downstream (relatórios/alertas/orçamento) passar a **ler** o valor: nesse momento avaliar promover o `int64` sentinela a VO de domínio se surgir invariante adicional.
