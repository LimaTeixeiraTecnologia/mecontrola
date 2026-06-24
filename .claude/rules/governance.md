# Governança de Regras

- Rule ID: R-GOV-001
- Severidade: hard
- Escopo: `.agents/skills/`, `.claude/rules/` e `.claude/skills/`.

## Objetivo
Definir precedência, resolução de conflitos e critérios de evidência para uso com agentes de IA.

## Fonte de Verdade
- Processos detalhados: `.agents/skills/`
- Regras transversais: `.claude/rules/`
- Referências de governança: `.agents/skills/agent-governance/references/`
- Referências Go: `.agents/skills/go-implementation/references/`

## Precedência
1. Esta governança transversal
2. `.agents/skills/agent-governance/references/security.md`
3. Referências de arquitetura e implementação carregadas pela skill ativa
4. `.agents/skills/agent-governance/references/` (`ddd`, `error-handling`, `tests`)
5. Uber Go Style Guide PT-BR como base transversal (quando aplicável)

Se duas regras do mesmo nível conflitarem:
- prevalece `hard` sobre `guideline`
- se a severidade empatar, prevalece a regra mais restritiva para correção, segurança e determinismo
- convenção explícita local prevalece sobre o guia da Uber quando documentada nas referências
- `go-implementation` prevalece sobre `object-calisthenics-go` quando houver conflito — object calisthenics é ferramenta de revisão e heurística de design, não substitui as diretrizes de implementação. Exemplo prático: `architecture.md` define "preferir tipos concretos por padrão"; OC regra #3 sugere "encapsular primitivos de domínio". Neste caso, encapsular apenas quando o valor carregar invariante de domínio (ex: `OrderID`, `Email`), não para primitivos sem regra de validação
- `domain-modeling.md` (DMMF adaptado) prevalece sobre estilo idiomático genérico (Uber) para regras de **tipo e estado** — smart constructor, discriminated union, state-as-type, workflow pipeline. Estilo genérico continua autoritativo para layout, naming, imports e wrapping de erro. Anti-padrões rejeitados em `domain-modeling.md` (Result/Either customizado, currying, DSL de pipeline) são `hard`: não introduzir mesmo sob influência de F#/Scala/Rust.

## Política de Evidência
- Toda alteração deve ser justificável pelo PRD, por regra explícita ou por necessidade técnica demonstrável.
- Relatórios devem incluir arquivos alterados, validações executadas, riscos residuais e suposições assumidas.
- Não aprovar solução com lacuna crítica conhecida.

## Segurança Operacional
- Não executar ações de git destrutivas ou publicações remotas sem pedido explícito.
- Se faltar input obrigatório e não houver inferência segura, a execução deve pausar ou falhar de forma explícita.

## Regras de Modulo

- `transactions-workflows.md` — codifica o gate hard ADR-006 para o modulo `internal/transactions`: `Decide*` puro obrigatorio, validacao so em smart constructors, producers so mapeiam domain event, cardinalidade controlada em metricas. Ver `.claude/rules/transactions-workflows.md`.
- `agent-workflows-tools.md` — codifica o gate hard `R-AGENT-WF-001` para o modulo `internal/agent`: roteamento canonico `Workflow -> Tool -> binding -> usecase`, proibido novo `case` de dominio no switch de `daily_ledger_agent.go`, Tool fina sem regra/SQL/branching, `ToolOutcome`/`RunStatus` como tipos fechados (DMMF state-as-type), LLM so no step de parse, Run auditavel. Addendum `.6-A`/`.8-A` distingue mecanismo kernel de semantica agent. Ver `.claude/rules/agent-workflows-tools.md`.
- `workflow-kernel.md` — codifica o gate hard `R-WF-KERNEL-001` para `internal/platform/workflow`: kernel generico sem import de dominio, sem regra/SQL/branching de dominio fora do adapter postgres, estados como tipos fechados (`RunStatus`/`StepStatus`/`SuspendReason`), cardinalidade controlada (sem `user_id`/`correlation_key`/`category_id`), LLM proibido. Gate bloqueante (ADR-004): redigido antes de qualquer codigo do kernel. Ver `.claude/rules/workflow-kernel.md`.

## Proibido
- Aprovação sem evidência.
- Loops infinitos de remediação.
