# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Runtime deriva workflow/tool do catálogo; drift de classificação corrigido explicitamente
- **Data:** 2026-06-25
- **Status:** Aceita
- **Decisores:** Time de plataforma (agent)
- **Relacionados:** PRD (RF-07..11, RF-13, RF-17), techspec, ADR-001, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.5), `.claude/rules/transactions-workflows.md` (R-TXN-004).

## Contexto

O `AgentRuntime.Execute` deriva os labels `workflow`/`tool` (gravados no `Run`, no span e nas métricas Prometheus) das funções manuais `workflowFor`/`toolFor` (`agent_runtime.go:208-245`), um `switch` paralelo à resolução de execução do registry. A auditoria empírica do código revelou **drift real e ativo em produção**:

| Kind | Label legado (`workflowFor`) | Workflow real (owner em `buildRegistry`) |
|---|---|---|
| `KindQueryIncomeSummary` | `conversational` (cai no `default`); `toolFor`=`""` | `transactions` |
| `KindBudgetRecurrence` | `conversational` (default) | `budget` |
| `KindDeleteTransactionByRef` | `conversational` (default) | destrutivo (transactions) |
| `KindEditTransactionByRef` | `conversational` (default) | destrutivo (transactions) |

Ou seja: execuções reais de `transactions`/`budget` são contabilizadas como `conversational`, e `QueryIncomeSummary` não emite `agent_tool_invocations_total` (tool vazio). Métricas e audit trail mentem; o review passa porque nenhum teste cobre esses kinds na classificação.

## Decisão

O `AgentRuntime` passa a **derivar** `workflow`/`tool` via `catalog.Classify(kind)` (ADR-001), eliminando `workflowFor`/`toolFor` como fonte de verdade. Para kind não-catalogado, `Classify` retorna o fallback conhecido (`workflowConversational`, `""`), preservando o comportamento de borda.

O drift dos 4 kinds acima é **corrigido, não preservado**: o catálogo atribui o `WorkflowID` real (owner do registry). A correção é tornada **visível e auditável** por:
- **Teste de equivalência por kind (RF-17):** tabela capturando os labels legados; assert idêntico para todos os kinds **exceto** a lista explícita de 4 drifts, cujos novos valores corretos são afirmados no teste.
- **Comunicação de impacto no PR:** documentar a mudança esperada nas séries de métrica (queda em `workflow="conversational"`, alta em `transactions`/`budget`, surgimento de `tool="query_income_summary"`).

Cardinalidade de métrica permanece controlada: labels restritos a `agent_id`/`channel`/`workflow`/`status`/`tool`/`outcome` (R-AGENT-WF-001.5 / R-TXN-004).

## Alternativas Consideradas

- **Preservar o label errado para não mexer em dashboards (RF-09 literal para todos os kinds):** manteria a métrica mentindo; contraria O2/MS-02 (eliminar drift) e a própria razão do PRD. Rejeitada.
- **Corrigir silenciosamente, sem teste de equivalência:** arriscaria regressões não detectadas em kinds concordantes e leitura de produção como anomalia. Rejeitada — a correção precisa ser explícita e testada.
- **Manter `workflowFor` como delegador fino sobre o catálogo (RF-08 alternativa):** aceitável como passo intermediário, mas mantém superfície redundante; preferida a remoção. Permitido pelo RF-08 caso a remoção exponha acoplamento inesperado.

## Consequências

### Benefícios Esperados
- Métricas e audit trail passam a refletir a execução real.
- Kinds novos herdam classificação correta automaticamente (zero drift estrutural — MS-02).
- `QueryIncomeSummary` passa a ser observável como tool.

### Trade-offs e Custos
- Mudança observável em 4 séries de métrica no deploy; exige comunicação à operação.
- Necessidade de manter o teste de equivalência sincronizado quando novos kinds entram (intencional — é o guard-rail).

### Riscos e Mitigações
- **Risco:** dashboards/alertas existentes assumirem a classificação antiga. **Mitigação:** documentar impacto no PR; é correção de defeito. **Rollback:** reintroduzir `workflowFor`/`toolFor` (mudança isolada no `AgentRuntime`).
- **Risco:** kind destrutivo by-ref mudar de série quebra um alerta de volume conversational. **Mitigação:** revisar alertas que filtram por `workflow="conversational"` antes do merge.

## Plano de Implementação
1. Injetar `*capability.Catalog` no `AgentRuntime` (construtor).
2. Substituir `agent_runtime.go:80-81` por `workflow, tool := rt.catalog.Classify(result.Kind)`.
3. Remover `workflowFor`/`toolFor` (ou reduzir a delegadores).
4. Escrever teste de equivalência por kind com lista explícita de exceções.
5. Atualizar descrição do PR com o impacto de métricas.

## Monitoramento e Validação
- Teste de equivalência por kind verde (MS-03 com exceções documentadas).
- Pós-deploy: confirmar surgimento das séries corrigidas e ausência de regressão nos kinds concordantes.
- Cardinalidade inalterada (sem novos labels).

## Impacto em Documentação e Operação
- Revisar dashboards/alertas que dependam de `workflow="conversational"`.
- Nota no PR sobre as 4 séries corrigidas.

## Revisão Futura
- Revisitar se surgir necessidade de mais labels (ex.: `mode`) — avaliar impacto de cardinalidade antes.
