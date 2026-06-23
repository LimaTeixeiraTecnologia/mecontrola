# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Fronteira de bounded contexts — agent é ponte; cada domínio no seu módulo
- **Data:** 2026-06-23
- **Status:** Aceita
- **Decisores:** Dono do produto, time de plataforma
- **Relacionados:** PRD `.specs/prd-onboarding-v2/prd.md`, techspec.md, ADR-001, ADR-004; R-AGENT-WF-001, R-ADAPTER-001; memória [[feedback_agent_calls_modules_own_persistence]]

## Contexto

O `internal/agent` é a ponte do sistema: WhatsApp → `cmd/server` → `internal/agent` → OpenRouter
(LLM) → `internal/<modulos>`. A premissa **inegociável** é que **cada bounded context é dono do seu
domínio e da sua persistência**, acessado pela porta de entrada (binding → usecase) ou via
integração por eventos do outbox. Nenhum módulo toca a tabela de outro. Mapa de propriedade neste
fluxo:

- `internal/budgets`: alocação/distribuição de orçamento (`AllocationDistributor`, `BasisPoints`,
  `RootSlug`) e budget real (`CreateBudget`/`ActivateBudget`).
- `internal/card`: cartões (criação/persistência).
- `internal/transactions`: lançamentos (`CreateTransaction`, card purchase, recorrências).
- `internal/onboarding`: **apenas o fluxo de onboarding e o estado de `onboarding_sessions`** —
  fase, objetivo, intenção de split, `recent_turns`, `welcome_sent_at`, `completed_at` — emitindo
  eventos de domínio para os módulos donos materializarem.
- `internal/agent`: ponte — parse via LLM, scripts/copy e primitivos Mastra próprios (Thread, Run,
  WorkingMemory, Pending Step). Sem regra/persistência de outro módulo.

**Integração já existente (não recriar):** o onboarding emite `onboarding.income_registered`,
`onboarding.card_registered`, `onboarding.splits_calculated`, `onboarding.completed`. Consumidores
dos módulos donos materializam o estado: `internal/budgets` consome `splits_calculated` →
`CreateBudget`+`ActivateBudget`; `internal/card` consome `card_registered` → cria o cartão
(onboarding injeta `SynchronousCardCreator`); a 1ª transação vai por
`agent → ExpenseRecorder → internal/transactions`. Logo, o onboarding guardar a intenção
(`custom_split`, card draft) no seu próprio `payload` **não é violação** — é estado do fluxo + fonte
do evento.

**Única violação real a corrigir:** o cálculo da distribuição (`buildAutoSplits`, com a matemática
basis points × renda → cents) está em `internal/agent`. Essa matemática é domínio de
`internal/budgets` (`AllocationDistributor`) e deve ser delegada a ele.

## Decisão

1. **Mover a matemática de distribuição para `internal/budgets`**: expor um usecase de sugestão
   (ex.: `SuggestAllocation`, encapsulando `AllocationDistributor`) consumido via binding. Remover
   `buildAutoSplits` de `internal/agent`.
2. **`internal/onboarding` mantém apenas a política de recomendação** ligada ao objetivo: o VO
   `ObjectiveProfile` e o template **perfil → basis points** (objetivo/perfil é conceito do
   onboarding). O cálculo cents é delegado a `internal/budgets`; a materialização do budget continua
   via evento `splits_calculated`.
3. **Mover para `internal/onboarding` o estado de fluxo do onboarding** hoje indevidamente no agente
   (histórico de turnos, `welcome_sent_at`, `completed_at`), com usecases próprios
   (`AppendOnboardingTurn`, `LoadOnboardingTurns`, `MarkWelcomeSent`, `CompleteOnboardingSession`).
4. **`internal/agent` acessa tudo via binding→usecase** (interface no consumidor R6.3, adapter fino
   sem SQL/regra). O hint `objective_profile` é preenchido pelo LLM no parse (legítimo do agente) e
   forwardado ao onboarding. WorkingMemory/Thread/Run/Pending Step permanecem agent-owned.
5. **Cartões e transações já delegam** (`SynchronousCardCreator`→`internal/card`;
   `ExpenseRecorder`→`internal/transactions`); o V2 apenas ajusta o cartão para `closing_day`.

## Alternativas Consideradas

- **Manter cálculo/estado no agente** (status quo): menos refator imediato, mas viola a premissa
  inegociável, espalha regra de domínio e acopla agente ao onboarding. Rejeitada.
- **Compartilhar repositório/transação entre agente e onboarding**: rejeitada — fere o isolamento de
  módulos (cada módulo é dono da sua persistência; nunca compartilhar TX — memória citada).

## Consequências

### Benefícios Esperados

- Regra de domínio única e testável no módulo dono; agente fino e substituível.
- Reuso dos usecases por outros entrypoints (não só o agente).
- Conformidade com R-AGENT-WF-001 / R-ADAPTER-001 e com a premissa de fronteira.

### Trade-offs e Custos

- Refator de código existente (`buildAutoSplits`, histórico) e novos usecases/bindings.
- Uma indireção a mais (binding) em caminhos antes resolvidos no agente.

### Riscos e Mitigações

- **Risco:** regressão ao mover lógica. **Mitigação:** testes prévios do comportamento atual; mudança
  incremental por usecase; gate de revisão (sem SQL/domínio de onboarding em `internal/agent`).
- **Rollback:** manter a função antiga no agente atrás do binding até o usecase estabilizar.

## Plano de Implementação

1. `internal/budgets`: expor `SuggestAllocation` (encapsula `AllocationDistributor`) via usecase +
   binding.
2. `internal/onboarding`: `ObjectiveProfile` + template perfil→basis points; `SuggestBudgetSplit`
   delega o cálculo cents ao binding de budgets; usecases de histórico/marcos/conclusão.
3. `internal/agent`: substituir `buildAutoSplits` e o acesso a histórico por chamadas de binding via
   adapters finos; remover `buildAutoSplits`.
4. Gate de revisão automatizado de fronteira.

## Monitoramento e Validação

- Gate (CI/review) — todos devem retornar vazio em `internal/agent`:
  - SQL: `QueryContext|ExecContext|db.Query|tx.Exec`.
  - Cálculo de split: `buildAutoSplits` e matemática `basis points × cents` em
    `internal/agent/.../onboarding*`.
  - Import de domínio/persistência de outro módulo fora de `internal/agent/infrastructure/binding/`.
- `go build ./...` + testes de `internal/onboarding`, `internal/budgets`, `internal/agent`.

## Impacto em Documentação e Operação

- AGENTS.md / runbook: reforçar o papel de ponte do `internal/agent`.

## Revisão Futura

- Reavaliar se outros fluxos do agente (daily) ainda retêm cálculo de domínio a migrar.
