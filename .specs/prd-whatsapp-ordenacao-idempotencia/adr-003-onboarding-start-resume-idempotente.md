# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Start de onboarding idempotente-resume e persistência de turnos
- **Data:** 2026-07-01
- **Status:** Aceita
- **Decisores:** time de plataforma (autor), owner do produto (decisões D-02 do PRD)
- **Relacionados:** PRD (RF-09/10/11/12), techspec `techspec.md`, ADR-001,
  regras R-WF-KERNEL-001, R-AGENT-WF-001 (.7 pending step/resume), `domain-modeling.md`

## Contexto

O onboarding reinicia mesmo quando o usuário já respondeu. Causa comprovada (pesquisa de código):

- `internal/agents/application/usecases/resolve_onboarding_or_agent.go`: sequência `store.Load`
  (sem `FOR UPDATE`) → checa marcador `"## Objetivo Financeiro"` na working memory → `engine.Start`,
  **sem atomicidade** — janela TOCTOU.
- `internal/platform/workflow/engine.go` `Start`: quando duas execuções concorrentes passam o `Load`
  e ambas chamam `Insert`, a 2ª viola o índice único parcial
  `workflow_runs_active_key_uidx (workflow, correlation_key) WHERE status IN ('running','suspended')`
  e retorna **erro genérico** (`insert snapshot: %w`), **não** `ErrRunAlreadyExists` (esse só sai do
  check prévio). No consumer vira `outcome="onboarding_error"` — ~68% na janela do incidente.
- O onboarding **não persiste turnos** em `platform_messages` (só o agent runtime persiste, em
  `runtime.go:138-153`); por isso o empty-reply do onboarding foi indiagnosticável.

Já correto e a preservar: o marcador de conclusão bloqueia restart de usuário concluído
(`resolve_onboarding_or_agent.go`), e cada passo suspende com estado no `Snapshot` **antes** de pedir
input (R-AGENT-WF-001.7 — pending step). `OnboardingPhase` é tipo fechado (state-as-type).

Nota de schema (verificado contra produção `mastra-20260629` em 2026-07-02): os nomes reais são
`workflow_runs` (status **texto** `'running'|'suspended'`) — o índice `workflow_runs_active_key_uidx`
existe exatamente como citado — e `platform_messages` (FK `thread_pk` → `platform_threads.id`; o
`threadId` opaco é `platform_threads.thread_id`, TEXT). Turnos de onboarding usam a **mesma thread** do
agente (D-15). Não há drift entre a migration local `000001` e o schema deployado.

## Decisão

1. **Start idempotente-resume:** no `engine.Start` (kernel genérico), capturar a violação do índice
   único parcial no `Insert` e, em vez de erro, **recarregar o run ativo e retomar** (equivalente
   semântico de `ErrRunAlreadyExists` → resume). Mantém o kernel sem domínio: a decisão é sobre o
   mecanismo (unique_violation → Load → resume), não sobre regra de negócio.
2. **Atomicidade por usuário:** a resolução onboarding-vs-agente ocorre sob a serialização por usuário
   do claim particionado (ADR-001), fechando a janela TOCTOU sem lock de sessão. A checagem
   `Load`→marcador→`Start` deixa de ter corrida porque só há 1 evento do usuário em voo.
3. **Persistir turnos de onboarding:** os passos do onboarding passam a gravar inbound e outbound em
   `platform_messages` via o mesmo contrato `memory.MessageStore.Append` já usado pelo agent runtime
   — histórico único da conversa e diagnóstico de empty-reply.
4. **Preservar** o bloqueio de restart por marcador e o contrato de pending step/resume (merge-patch
   antes do parse), sem regressão.

## Alternativas Consideradas

1. **`SELECT ... FOR UPDATE` no `Load` do Start:** serializa o check no banco, mas segura linha/tx e
   não cobre o caso de run inexistente (o conflito é no `Insert`, não no `Load`); insuficiente
   sozinho. O idempotente-resume no `Insert` é mais direto e barato.
2. **Tratar unique_violation só no consumer** (retry): mascara o problema, mantém `onboarding_error`
   e não retoma o run correto; rejeitada.
3. **Mover onboarding para dentro do agent runtime** (reuso automático da persistência de turnos):
   refatoração ampla, fora de escopo do PRD; rejeitada agora (só adota o contrato `Append`).

## Consequências

### Benefícios Esperados

- Fim do loop de onboarding e da taxa de ~68% `onboarding_error` (meta < 2%).
- Conflito concorrente vira retomada correta do run existente (sem perder progresso).
- Turnos de onboarding rastreáveis; empty-reply diagnosticável.

### Trade-offs e Custos

- Lógica extra no `Start` do kernel (captura de unique_violation) — deve permanecer genérica
  (sem domínio) para não violar R-WF-KERNEL-001.
- Escrita adicional em `platform_messages` (volume baixo de turnos humanos).

### Riscos e Mitigações

- **Risco:** capturar unique_violation de forma acoplada ao Postgres. **Mitigação:** detectar via
  código de erro SQLSTATE `23505` no adapter/store e traduzir para `ErrRunAlreadyExists` (sentinela já
  existente), mantendo o engine agnóstico. **Rollback:** reverter a captura restaura o comportamento
  atual (erro → onboarding_error).
- **Risco:** duplo-persistir turno em resume. **Mitigação:** persistir no ponto único do fluxo, testado.

## Plano de Implementação

1. Adapter/store `workflow`: mapear SQLSTATE `23505` do `Insert` para `ErrRunAlreadyExists`.
2. `engine.Start`: ao receber `ErrRunAlreadyExists` do `Insert`, `Load` + retornar resume
   (reaproveitar `resolveConflictOrFail`/caminho de resume existente).
3. `resolve_onboarding_or_agent`: tratar o retorno de resume como handled; garantir atomicidade sob
   claim particionado (ADR-001).
4. Onboarding: chamar `messages.Append` para inbound e outbound de cada passo.
5. Testes de integração de concorrência (duas Start simultâneas → 1 run, 2ª retoma, 0 error) e
   verificação de turnos em `platform_messages`.

Concluído quando: CA-04 verde; `onboarding_error` < 2% em produção; turnos presentes em `platform_messages`.

## Monitoramento e Validação

- Métricas: `onboarding_error` (< 2%), outcome novo `resumed_on_conflict`, contagem de turnos
  persistidos. Alertar se `onboarding_error` voltar a subir.

## Impacto em Documentação e Operação

- Runbook de onboarding; regra `.claude/rules/agent-workflows-tools.md` (reemissão do gate HITL se
  reintroduzido); dashboards de onboarding.

## Revisão Futura

- Revisar se o onboarding migrar para dentro do agent runtime (elimina o caminho separado) ou se o
  contrato de conclusão (marcador na working memory) mudar.
