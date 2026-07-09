# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Cadeia de guardas conversacionais no padrão Chain of Responsibility
- **Data:** 2026-07-09
- **Status:** Aceita
- **Decisores:** Plataforma / dono do agente MeControla
- **Relacionados:** `prd.md` (RF-01..RF-06, RF-09, RF-10, RF-12, RF-48), `techspec.md`,
  seletor `design-patterns-mandatory` (`status=ok`), US-001

## Contexto

Cerca de 20 regras críticas de segurança conversacional vivem hoje num prompt monolítico
(`internal/agents/application/agents/mecontrola_agent.go:17-250`) e dependem da probabilidade do LLM
para não falhar. Existe um único guard determinístico — `MultiItemGuard` — implementado como decorator
que embute `agent.Agent` e sobrescreve apenas `Execute` (`multi_item_guard.go:55-72`), curto-circuitando
com `agent.Result{Content: MultiItemOrientationMessage, ToolOutcome: ToolOutcomeClarify}` antes do LLM.

O problema é encadear múltiplos validadores/roteadores pequenos, ordenados, observáveis e testáveis,
executados **antes e depois** da chamada LLM, cada um podendo tratar (curto-circuitar), passar adiante
ou delegar. Restrições: preservar o contrato público `BuildMeControlaAgent`; não reescrever o substrato
`internal/platform`; Go 1.26.5; zero comentários; state-as-type; economia/robustez.

O seletor determinístico obrigatório de `design-patterns-mandatory` foi executado
(`scripts/select_pattern.py`, `status: ok`) com sinais `sequential_conditional_handlers` +
`prefer_composition` e restrições `preserve_public_contract`, `minimize_indirection`,
`team_needs_low_cognitive_load`, `high_change_frequency`. Resultado: primário **Chain of Responsibility**
(score 4, sem high_bar, sem blockers), complementar `null`, alternativa simples "sequência explícita de
funções". State não pontuou (nenhum `state_transition_driven_behavior` — coerente com a rejeição da US,
pois os workflows de estado já vivem no substrato).

## Decisão

Introduzir uma **cadeia de guardas conversacionais (Chain of Responsibility)** no consumidor
`internal/agents`, como um decorator `guardChainAgent` que implementa `agent.Agent` e percorre listas
ordenadas de `PreGuard` (antes do LLM) e `PostGuard` (depois do LLM). Cada handler expõe `Name()` e
retorna `GuardDecision{Handled, Result}`; o primeiro `PreGuard` que trata curto-circuita sem chamar o
LLM (economia RF-48); os `PostGuard` podem sobrescrever o `Result` para um fallback seguro em caso de
violação inequívoca.

O `MultiItemGuard` atual é **absorvido como o primeiro `PreGuard`** da cadeia, preservando comportamento
determinístico, `ToolOutcomeClarify` e a mensagem verbatim (RF-02). `WithMultiItemGuard` é substituído
por `WithGuardChain(...)` em `BuildMeControlaAgent`, mantendo a assinatura pública intacta. `Stream` é
delegado por embedding (paridade com o guard atual).

Handlers iniciais: `multi_item` (pré), `verbatim_relay`, `empty_answer`, `internal_terms`,
`success_without_tool`, `card_provenance` (pós). Novos comportamentos entram como novos handlers, sem
crescer branching no prompt nem `switch case intent.Kind` (R-AGENT-WF-001.1).

## Alternativas Consideradas

- **Sequência explícita de funções (alternativa simples que quase venceu):** chamar validadores como
  funções encadeadas diretamente no `Execute`. Vantagem: menos tipos. Desvantagem: não organiza ordem
  nem observabilidade por handler, dificulta teste isolado e adição de novos guards; regras de alta
  frequência de mudança ficam acopladas. Rejeitada por perder a governança explícita do fluxo.
- **State (máquina de estado) como primário:** rejeitada pela US e pelo seletor — os workflows de
  estado (pending/confirm/onboarding) já existem no substrato; reintroduzir State duplicaria
  responsabilidade e custo estrutural.
- **Manter regras no prompt + reforço de instrução:** rejeitada — é exatamente a causa-raiz
  (dependência probabilística; scorers baixos em produção).
- **Múltiplos decorators empilhados (um por regra):** rejeitada — sem ordem explícita nem
  observabilidade unificada; N wraps aninhados aumentam custo cognitivo.

## Consequências

### Benefícios Esperados

- Regras críticas viram código determinístico, testável e observável por handler (RF-03).
- Curto-circuito pré-LLM economiza custo/latência (RF-48).
- Contrato público preservado; novos comportamentos entram como novos handlers (extensível).
- Observabilidade por handler (`agent_guard_decisions_total{guard, decision}`) com cardinalidade fechada.

### Trade-offs e Custos

- Um novo tipo/decorator e um diretório `guards/` (indireção adicional aceitável frente ao ganho de
  organização/testabilidade).
- Handlers pós-LLM que sobrescrevem `Result` exigem disciplina para agir só sobre violação inequívoca.

### Riscos e Mitigações

- **Risco:** override pós-LLM regride fluidez de respostas válidas. **Mitigação:** handlers agem apenas
  em violação determinística inequívoca; golden de regressão garante que respostas válidas não são
  tocadas. **Rollback:** o decorator pode ser removido restaurando o wrap `WithMultiItemGuard` (o guard
  atual permanece equivalente no handler absorvido).

## Plano de Implementação

1. Criar `guard_chain.go` (`GuardDecision`, `PreGuard`, `PostGuard`, `guardChainAgent`, `WithGuardChain`,
   métrica por handler).
2. Migrar `MultiItemGuard` para `guards/multi_item.go` como `PreGuard`, com teste de equivalência.
3. Implementar os `PostGuard` iniciais.
4. Trocar o wrap em `BuildMeControlaAgent` para `WithGuardChain(...)`.
5. Cobrir cada handler e o runner com testify/suite table-driven.

Concluído quando: cadeia integrada, MultiItemGuard equivalente sob teste, wrap público inalterado,
gates de governança verdes.

## Monitoramento e Validação

- `agent_guard_decisions_total{agent_id, guard, decision}` mostra qual handler tratou cada run.
- Golden set (ADR-005) valida ausência de regressão de fluidez e cobertura das regras.
- Revisar a decisão se a cadeia acumular lógica de domínio (sinal de que um handler deveria ser tool/
  usecase/workflow).

## Impacto em Documentação e Operação

- Runbook do agente: descrever a cadeia e os handlers.
- Dashboards: painel de decisões de guarda por handler.

## Revisão Futura

Revisitar quando um novo comportamento crítico não couber como handler, ou se a ordem dos handlers
gerar acoplamento — nesse caso avaliar submáquina dedicada (novo ADR).
