# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Retry transitório limitado no passo de escrita com classificação de erro
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Time de plataforma MeControla
- **Relacionados:** PRD (RF-22..RF-25), techspec.md, ADR-002 (propagação de erro), `.claude/rules/workflow-kernel.md`

## Contexto

Hoje não há retry: qualquer falha de escrita cancela imediatamente o pending
(`pending_entry_workflow.go:453-456` e `:465-468` setam `PendingStatusCancelled` e respondem
"Não consegui registrar. Tente novamente em breve."). O kernel já oferece o combinador
`workflow.Retry(step, RetryPolicy{MaxAttempts, BaseBackoff, MaxBackoff})` com backoff exponencial +
jitter (`combinators.go:158-229`), porém **não é usado**; o `pending_entry_workflow` define
`MaxAttempts=1`.

A idempotência de escrita é garantida por duas camadas:
- `IdempotentWrite.Execute(ctx, userID, wamid, itemSeq, operation, resourceKind, WriteFn)`
  (`idempotent_write.go:42-138`) faz `FindByKey` (replay) antes de executar o `WriteFn` e insere no
  `agents_write_ledger` (UNIQUE `wamid,item_seq,operation`, `ON CONFLICT DO NOTHING`).
- O `WriteFn` retorna `(resourceID, reconciled bool, err)`; o sinal `reconciled` indica que o lado
  `transactions` **deduplica por chave** e devolve o mesmo recurso — reexecutar o `WriteFn` é
  seguro (retorna reconciled, não cria segundo lançamento).

Não existe classificação transitório-vs-permanente: todo erro não-`UniqueViolation` é tratado como
fatal.

## Decisão

Introduzir **retry transitório limitado** localizado no passo de escrita, com política e
classificação explícitas:

1. **Política e local**: até **2 tentativas** (1 original + 1 retry adicional) com backoff curto —
   `BaseBackoff` ~100ms, `MaxBackoff` tal que o total fique abaixo de ~2s no mesmo turno, com jitter
   e respeito a `ctx.Done()` (`resilience.md`). O retry é um **loop localizado restrito ao trecho de
   escrita** dentro de `executeWithIdempotency`/`executeDirectWrite` (chamando `idem.Execute`
   novamente na falha transitória), **não** o combinador `workflow.Retry` de nível `Step[S]` nem o
   `MaxAttempts` do `Engine`. Justificativa: (a) a escrita é uma ramificação interna do mega-step
   `makePendingEntryStep` (após `DecideConfirmation` aceitar) — envolvê-la em `workflow.Retry`
   reexecutaria a decisão de confirmação; (b) `Engine.MaxAttempts` retentaria o passo inteiro
   através de novo resume (mecanismo durável cross-turno), inadequado para um blip síncrono. O
   seletor de padrões (`design-patterns-mandatory`) retornou `reject` ("não aplicar padrão"),
   confirmando a solução direta. `Engine.MaxAttempts` do `pending_entry` permanece `1`.
2. **Classificação de transitório**: adicionar um predicado `IsTransient(err) bool` no consumidor
   (`internal/agents`, camada application) que reconhece timeout, `context.DeadlineExceeded`,
   connection reset e erros de infraestrutura equivalentes via `errors.Is`/`errors.As`. Somente erro
   transitório é retentado; erro permanente (validação de domínio, `ErrKindMismatch`) falha direto
   e propaga (ADR-002), sem consumir tentativas.
3. **Segurança de idempotência**: o retry chama novamente `IdempotentWrite.Execute` com a mesma
   chave `(wamid, itemSeq, operation)`. O `FindByKey`/`reconciled` garantem replay/reconcile sem
   segundo lançamento (RF-24). Se a transação já foi criada mas o `agents_write_ledger` falhou, o
   reprocessamento posterior encontra a transação via dedup do `transactions` (reconciled) e apenas
   reinsere o ledger com `ON CONFLICT DO NOTHING` — não reexecuta a escrita de domínio (RF-25).
4. **Pending retomável**: esgotadas as 2 tentativas sem sucesso, o passo propaga `StepStatusFailed`
   (ADR-002) e o pending entry permanece retomável (snapshot durável preserva `Candidates` e
   `CategoryVersion` — `pending_entry_state.go:167-199`); a próxima confirmação reexecuta a escrita
   **sem repetir a classificação de categoria** (RF-23).

## Alternativas Consideradas

1. **Retentar o passo inteiro (`makePendingEntryStep`) via `workflow.Retry`** — Descartada: reexecuta
   `DecideConfirmation` e a lógica de slot; embora o replay-guard proteja, o retry deve cercar apenas
   a escrita para minimizar efeitos colaterais e latência.
2. **Retry sem classificação (retentar qualquer erro)** — Descartada: retentaria erros permanentes
   (validação) sem chance de sucesso, adicionando latência inútil e mascarando bugs.
3. **Escrita atômica única (ledger + transação na mesma tx)** — Descartada: o `agents_write_ledger` é
   agent-owned e a escrita de domínio pertence ao módulo `transactions`; compartilhar transação
   cruza fronteira de módulo (proibido). A dedup por chave do `transactions` (reconciled) já entrega
   a garantia at-most-once sem tx compartilhada.
4. **Backoff exponencial de 3 tentativas** — Descartada na decisão de produto (AskUserQuestion):
   adiciona latência perceptível no WhatsApp; 2 tentativas equilibram recuperação e fluidez.
5. **Introduzir padrão GoF (Strategy/Decorator) para a política de retry** — Descartada pelo seletor
   determinístico da skill `design-patterns-mandatory` (`select_pattern.py` ⇒ `status: reject`, sinais
   `prefer_direct_solution`/`single_variant_only`/`low_change_frequency`): política única, sem
   variação em runtime; o combinador `Retry` do kernel + predicado `IsTransient` são a solução direta
   com menor custo cognitivo e menor superfície de falha.

## Consequências

### Benefícios Esperados

- Falha transitória (blip de rede/DB) é recuperada sem o usuário recomeçar (RF-22).
- Nenhuma duplicação: replay/reconcile garantem exatamente um lançamento (RF-24, RF-25).
- Pending retomável cobre a falha persistente sem perder a categoria já resolvida (RF-23).

### Trade-offs e Custos

- Até ~2s adicionais de latência no pior caso de falha transitória.
- Novo predicado `IsTransient` precisa de cobertura de teste para não classificar erro de domínio
  como transitório.

### Riscos e Mitigações

- **Risco:** classificar erro permanente como transitório e retentar em vão. **Mitigação:**
  whitelist estrita de erros transitórios; default é permanente (não retenta).
- **Risco:** retry amplificar carga sob incidente de DB. **Mitigação:** teto de 2 tentativas e
  backoff curto com jitter limitam a amplificação.

## Plano de Implementação

1. Implementar `IsTransient(err) bool` no consumidor com testes de tabela.
2. Aplicar retry (combinador do kernel ou loop restrito) ao trecho `IdempotentWrite.Execute` com
   `MaxAttempts=2`, backoff ~100ms/jitter, teto <~2s.
3. Garantir que erro permanente falha imediatamente (ADR-002) sem consumir tentativas.
4. Testes: falha transitória na 1ª tentativa ⇒ persiste 1x na 2ª; reprocessar mesma chave ⇒ replay;
   transação criada + ledger falho ⇒ não reexecuta escrita (reconciled).

## Monitoramento e Validação

- Critério de aceite: "Falha transitória é retentada e o lançamento persiste uma vez" e
  "Reprocessamento da mesma chave não duplica lançamento".
- `agents_write_total{outcome}`: observar `created` vs `reconciled` vs `replay`; sem `user_id` como
  label.

## Impacto em Documentação e Operação

- Runbook de agents: comportamento de retry e como interpretar `reconciled`/`replay`.

## Revisão Futura

- Revisitar teto de tentativas/backoff se a telemetria mostrar falhas transitórias recorrentes acima
  do previsto.
