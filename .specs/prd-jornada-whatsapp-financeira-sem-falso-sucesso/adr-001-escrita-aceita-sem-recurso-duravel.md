# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Escrita aceita sem recurso durável vira falha tipada, nunca cancelamento
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** time de plataforma / agente financeiro
- **Relacionados:**
  - PRD `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/prd.md` (RF-05, RF-07, RF-10, RF-11, RF-12)
  - `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/techspec.md`
  - US-001
  - ADR-002 (retry controlado via `FailedWriteResumeCount`)
  - ADR-004 (scorer `write_persistence_accuracy`)

## Contexto

Na jornada real, o usuário confirmou um lançamento com "Sim". A confirmação foi aceita e disparou a escrita idempotente, que retornou `resourceID == uuid.Nil` **sem** criar ledger nem transação — um efeito nulo, não um efeito válido.

O workflow pending-entry tratava esse retorno como se fosse um desfecho terminal bem-sucedido:

- `internal/agents/application/workflows/pending_entry_workflow.go:555-559` (`executeWithIdempotency`)
- `internal/agents/application/workflows/pending_entry_workflow.go:586-590` (`executeDirectWrite`)

Ambos os caminhos, no ramo `resourceID == uuid.Nil`, forçavam `state.Status = PendingStatusCancelled` e retornavam `workflow.StepStatusCompleted`.

O engine do kernel (`internal/platform/workflow/engine.go` L371/386/461) mapeia `StepStatusCompleted → RunStatusSucceeded`. Logo o `Run` terminava **succeeded** apesar de nenhum efeito durável ter ocorrido — um falso sucesso. Como o retry existente (`tryResumeFailedWrite`) exige `RunStatusFailed` para disparar, ele **nunca** era acionado nesse cenário: o sistema declarava vitória sobre uma escrita fantasma e não havia caminho de recuperação.

Restrições e premissas:

- O kernel `internal/platform/workflow` é genérico e não pode conhecer semântica de domínio (R-WF-KERNEL-001); o invariante "sucesso só com efeito durável" é responsabilidade do consumidor `internal/agents`.
- Os estados de fronteira já são tipos fechados (DMMF state-as-type): `RunStatus` e `ToolOutcome` em `internal/platform/agent/types.go`; `PendingStatus` em `pending_entry_state.go`.
- Existem dois caminhos de escrita (`executeWithIdempotency` e `executeDirectWrite`), ambos com o mesmo defeito.

## Decisão

1. **Falha tipada no ramo sem recurso.** No ramo `resourceID == uuid.Nil` com `outcome != agent.ToolOutcomeReplay`, o passo passa a retornar `workflow.StepStatusFailed` acompanhado de um erro de negócio tipado, sentinel local do consumidor:

   ```go
   var ErrWriteAcceptedWithoutResource = errors.New("workflows.pending_entry: escrita aceita sem recurso durável")
   ```

   O retorno embrulha o sentinel com `fmt.Errorf("...: %w", ErrWriteAcceptedWithoutResource)`, preservando a cadeia `errors.Is`.

2. **`PendingStatusActive` preservado.** `state.Status` permanece `PendingStatusActive` — nunca `PendingStatusCancelled`. `PendingStatusCancelled` fica reservado exclusivamente a cancelamento explícito do usuário, expiração (`PendingStatusExpired`) ou substituição (`PendingStatusReplaced`). Nenhuma dessas semânticas cabe a uma escrita que falhou em produzir efeito.

3. **Correção simétrica.** A mesma inversão é aplicada em `executeWithIdempotency` **e** `executeDirectWrite`, pois ambos carregam o defeito.

4. **Invariante puro centralizável (recomendado).** Extrair função pura que centraliza a decisão pós-escrita, testável sem mock:

   ```go
   func DecidePostWrite(outcome agent.ToolOutcome, resourceID uuid.UUID) (PendingStatus, workflow.StepStatus, error) {
       if outcome != agent.ToolOutcomeReplay && resourceID == uuid.Nil {
           return PendingStatusActive, workflow.StepStatusFailed, ErrWriteAcceptedWithoutResource
       }
       return PendingStatusCompleted, workflow.StepStatusCompleted, nil
   }
   ```

   Os dois caminhos de escrita consomem `DecidePostWrite`, eliminando a possibilidade de os ramos divergirem. O invariante "sucesso só com efeito durável" fica em um único ponto puro e determinístico.

5. **Merge-patch no resume preservado.** A retomada continua aplicando delta JSON merge-patch sobre o `Snapshot.State` (R-WF-KERNEL-001.7); nenhuma mudança na semântica de resume do kernel.

Como `StepStatusFailed` mantém `state.Status = PendingStatusActive`, o retry controlado existente (contador `FailedWriteResumeCount` em `pending_entry_state.go`) passa a disparar automaticamente ao detectar `RunStatusFailed` — mecânica detalhada no ADR-002.

Estados fechados envolvidos, todos já existentes:

- `RunStatus`: `running | succeeded | failed`.
- `PendingStatus`: `Active | Completed | Cancelled | Expired | Replaced`.
- `ToolOutcome`: `routed | clarify | usecaseError | missingResolver | replay | reconciled | truncated`.

## Alternativas Consideradas

### (a) Adicionar novo valor ao enum `ToolOutcome` da plataforma para "sem recurso"

- **Descrição:** criar, por exemplo, `ToolOutcomeNoResource` em `internal/platform/agent/types.go`.
- **Vantagens:** desfecho explícito no enum de plataforma.
- **Desvantagens:** amplia o enum genérico de plataforma para um caso local do consumidor; força atualização de todos os `switch`/`String()`/`Parse` de `ToolOutcome`; sem ganho concreto sobre um erro tipado local.
- **Motivo da rejeição:** o defeito e sua semântica são locais ao consumidor `internal/agents`. Segundo DMMF, erro de negócio tipado local (sentinel + `errors.Is`) modela o caso sem inchar o vocabulário de plataforma.

### (b) Manter `PendingStatusCancelled` no ramo sem recurso

- **Descrição:** conservar o comportamento atual (`Cancelled` + `StepStatusCompleted`).
- **Vantagens:** nenhuma mudança de código.
- **Desvantagens:** produz falso sucesso (`RunStatusSucceeded`), mistura a semântica de cancelamento com a de falha e mantém o retry morto.
- **Motivo da rejeição:** é exatamente a causa raiz do incidente.

## Consequências

### Benefícios Esperados

- Elimina o falso sucesso: escrita sem efeito durável agora termina em `RunStatusFailed`, refletindo a realidade.
- Habilita o retry controlado existente (`FailedWriteResumeCount`), que passa a disparar automaticamente (ADR-002).
- Torna o invariante "sucesso só com efeito durável" explícito, único e testável por teste puro (`DecidePostWrite`), sem mock.
- Restaura a semântica correta de `PendingStatusCancelled` (apenas cancelamento explícito, expiração ou substituição).

### Trade-offs e Custos

- O usuário recebe uma mensagem de falha ("Não consegui registrar. Tente novamente em breve.") em vez de um "ok" falso — comportamento correto, porém menos "suave" que o falso positivo anterior.
- Pequeno acréscimo de superfície (sentinel + função pura) no consumidor.

### Riscos e Mitigações

- **Risco:** divergência entre os dois caminhos de escrita (`executeWithIdempotency` e `executeDirectWrite`) se apenas um for corrigido.
  - **Impacto:** um caminho continuaria produzindo falso sucesso.
  - **Mitigação:** corrigir ambos os caminhos e, preferencialmente, centralizar a decisão em `DecidePostWrite`, garantindo por construção que os dois ramos compartilham o mesmo invariante. Teste puro cobre a tabela de casos (`replay` vs `nil` vs recurso válido).
  - **Rollback:** reverter o commit restaura o comportamento anterior (`Cancelled` + `Completed`); nenhuma migração de dados envolvida. Snapshots suspensos permanecem compatíveis (apenas `Status` e `StepStatus` mudam de valor).

## Plano de Implementação

1. Declarar `var ErrWriteAcceptedWithoutResource` no pacote de workflows do consumidor.
2. Implementar a função pura `DecidePostWrite(outcome, resourceID)` com o invariante e cobri-la com teste de tabela puro (replay, `uuid.Nil`, recurso válido).
3. Substituir os ramos `resourceID == uuid.Nil` em `executeWithIdempotency:555-559` e `executeDirectWrite:586-590` por chamadas a `DecidePostWrite`, retornando `StepStatusFailed` + erro embrulhado quando aplicável e mantendo `PendingStatusActive`.
4. Validar que o engine mapeia `StepStatusFailed → RunStatusFailed` e que `tryResumeFailedWrite` passa a disparar (integração com ADR-002).
5. Rodar build, vet, test race e lint no módulo `internal/agents`; executar gates de governança (R-WF-KERNEL-001, R-AGENT-WF-001, R-ADAPTER-001.1).

**Critério de conclusão:** ambos os caminhos de escrita, ao receber recurso vazio sem replay, produzem `RunStatusFailed` com `PendingStatusActive`; teste puro de `DecidePostWrite` verde; nenhum ramo produz `RunStatusSucceeded` sem efeito durável.

## Monitoramento e Validação

- O `Run` agora reflete `RunStatusFailed` no cenário de escrita fantasma — observável no audit trail e nas métricas de status do run (labels de cardinalidade controlada, sem `user_id`).
- O scorer `write_persistence_accuracy` (ADR-004) reprova qualquer desfecho que declare sucesso sem efeito durável correspondente, servindo de guarda de regressão.
- **Sucesso:** taxa de `RunStatusSucceeded` sem recurso associado tende a zero; retries por `FailedWriteResumeCount` aparecem quando a escrita não persiste.
- **Sinal de revisão:** aumento sustentado de `RunStatusFailed` por `ErrWriteAcceptedWithoutResource` indica defeito upstream na escrita idempotente (investigar antes de afrouxar o invariante).

## Impacto em Documentação e Operação

- Techspec do PRD: registrar o invariante `DecidePostWrite` e o sentinel `ErrWriteAcceptedWithoutResource`.
- Runbook do agente: descrever o novo caminho de falha (falha tipada → retry) e a mensagem ao usuário.
- Observabilidade: garantir que o scorer `write_persistence_accuracy` (ADR-004) esteja ativo no gate.

## Conformidade — Design Patterns

Nenhum novo padrão GoF é introduzido. A mudança é um refactor local: inversão de `StepStatus` (de `Completed` para `Failed`) mais um erro sentinel tipado, opcionalmente centralizados em uma função pura de decisão. Conforme a decisão da skill `design-patterns-mandatory`, o gate de desenho resulta em **não aplicar padrão** — não há problema recorrente de acoplamento ou variação que justifique um pattern estrutural, criacional ou comportamental. O ganho vem exclusivamente de tornar o estado ilegal (sucesso sem efeito durável) irrepresentável no fluxo.

## Revisão Futura

Revisitar quando:

- Um terceiro caminho de escrita for adicionado ao workflow pending-entry (garantir que também consuma `DecidePostWrite`).
- A escrita idempotente passar a poder retornar recurso vazio como desfecho legítimo (hoje inexistente) — exigiria remodelar o invariante.
- O contrato de `ToolOutcome` da plataforma mudar de forma a expressar nativamente "sem recurso", reabrindo a alternativa (a).
