# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Kernel de workflow genérico em `internal/platform`, coexistindo com o Run do agent
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Plataforma, dono do `internal/agent`
- **Relacionados:** PRD `.specs/prd-workflow-kernel/prd.md` (RF-01, RF-18); techspec `.specs/prd-workflow-kernel/techspec.md`; ADR-004 (governança)

## Contexto

- O `internal/agent` chama "Workflow" um dispatch plano `kind→tool` (`composite.go`), sem composição
  de passos, suspend/resume de primeira classe nem persistência de estado.
- O agent já possui auditoria de execução: `entities.Run` + tabela `agent_runs` + `RunGateway`,
  resolvida pelo `AgentRuntime` (Thread→Run), com métricas `agent_runs_total` etc.
- O PRD exige um kernel **genérico e reutilizável** em `internal/platform`, sem semântica de domínio.
- `R-AGENT-WF-001.6/.8` declaram Thread/Run/WorkingMemory/PendingStep **exclusivos** do `internal/agent`.
- Há sobreposição conceitual entre o "Run de auditoria de intent" do agent e o "Run de estado durável
  de steps" do kernel; era necessário decidir se unificar ou coexistir.

## Decisão

- Criar o kernel em `internal/platform/workflow`, **sem dependência de domínio** (`intent`, `agent`,
  `transactions`). O kernel opera sobre `Step[S]` genérico e um `correlationKey string` opaco.
- **Coexistir, não unificar**: `agent_runs`/`RunGateway`/`AgentRuntime` permanecem **inalterados** como
  auditoria por execução do agent. O kernel introduz `workflow_runs`/`workflow_steps` apenas para o
  estado durável de steps de runs de escrita/suspensíveis. O `IntentWorkflow` do agent referencia o
  run do kernel por id, sem fundir os dois modelos.
- Thread/Run/WorkingMemory/PendingStep **semânticos** continuam exclusivos do agent; o kernel fornece
  somente o mecanismo genérico (execução, snapshot, retomada).

## Alternativas Consideradas

- **Unificar (agent_runs vira projeção do kernel)**: reescrever `RunGateway` sobre o kernel.
  - Vantagens: um só conceito de Run.
  - Desvantagens: refactor amplo e arriscado do caminho auditável já em produção; contraria "MVP enxuto".
  - Motivo da rejeição: risco desproporcional ao ganho no MVP.
- **Kernel sem tabela própria (reusar `agent_runs`)**:
  - Vantagens: menos tabelas.
  - Desvantagens: acopla o kernel ao schema do agent, fere o requisito de reuso (RF-01).
  - Motivo da rejeição: elimina a generalidade que justifica o kernel.

## Consequências

### Benefícios Esperados

- Reuso real: qualquer módulo pode consumir o kernel sem herdar semântica do agent.
- Risco mínimo no MVP: o caminho auditável atual do agent não é tocado.
- Fronteira de governança clara entre kernel genérico e workflow de intent (ADR-004).

### Trade-offs e Custos

- Dois conceitos de "run" coexistem (auditoria do agent vs estado durável do kernel); exige
  documentação para evitar confusão.
- Leve duplicação de atributos (status, duration) entre as duas tabelas.

### Riscos e Mitigações

- **Risco:** confusão conceitual entre os dois runs. **Mitigação:** nomenclatura (`Workflow` no
  kernel, `IntentWorkflow` no agent — RF-19), runbook e esta ADR.
- **Rollback:** o kernel é aditivo e atrás de feature flag (ADR-005); desligar a flag retorna ao
  comportamento atual sem tocar `agent_runs`.

## Plano de Implementação

1. Gate de governança (ADR-004).
2. Implementar kernel puro + engine + store (techspec, itens 2–4).
3. Integrar no agent via `IntentWorkflow` sob flag (itens 5–7).
4. Critério de conclusão: kernel sem import de pacote de domínio (verificável por grep) e `agent_runs`
   inalterado.

## Monitoramento e Validação

- Métricas `workflow_*` (kernel) separadas de `agent_*` (agent).
- Sucesso: kernel compila/testa sem deps de domínio; nenhuma regressão em `agent_runs`.
- Revisão se um segundo consumidor exigir unificar os modelos de run.

## Impacto em Documentação e Operação

- `docs/runbooks/` do agent: nota sobre os dois conceitos de run.
- Skill `mastra`: atualizar mapa Mastra→Go com o kernel genérico.

## Revisão Futura

- Reavaliar unificação quando ≥2 módulos consumirem o kernel, ou se a duplicação de atributos gerar
  custo de manutenção relevante.
