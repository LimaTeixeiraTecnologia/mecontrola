# Claude Code

Use `AGENTS.md` como fonte canonica das regras deste repositorio.

Claude deve respeitar TODAS as regras, skills, references, validacoes, restricoes de arquitetura, economia de contexto e politicas de comentarios definidas em `AGENTS.md` de forma igualitaria ao Codex. Isso e obrigatorio e inegociavel.

## Instrucoes

1. Ler `AGENTS.md` no inicio da sessao.
2. `.claude/skills/` sao symlinks para `.agents/skills/` — a fonte de verdade e sempre `.agents/skills/`.
3. `.claude/agents/` sao wrappers leves que delegam para a habilidade canonica.
4. Em tarefas de execucao, carregar apenas `AGENTS.md`, `agent-governance` e a skill operacional da linguagem ou atividade afetada.
5. Skills de planejamento (`analyze-project`, `create-prd`, `create-technical-specification`, `create-tasks`) entram apenas quando a tarefa pedir esse fluxo explicitamente.
6. Carregar referencias adicionais apenas quando a tarefa exigir.
7. Preservar estilo, arquitetura e fronteiras existentes antes de propor mudancas.
8. Validar mudancas com comandos proporcionais ao risco.
9. Nao flexibilizar nenhuma regra por diferenca de ferramenta, hook, agente pre-carregado ou conveniencia operacional.

## Go — Regra Mandatória e Inegociável

Toda implementação, alteração ou revisão de código Go DEVE obrigatoriamente:

1. Carregar `.agents/skills/go-implementation/SKILL.md` antes de qualquer edição.
2. Verificar a versão declarada em `go.mod` antes de introduzir APIs da linguagem ou dependências.
3. Executar as **Etapas 1 a 5** do SKILL.md na íntegra:
   - **Etapa 1** — carregar `references/architecture.md` e as Regras Estritas R0–R7 (todas `[HARD]`).
   - **Etapa 2** — selecionar apenas as referências pertinentes ao escopo da mudança (máximo 4 simultâneas); carregar exemplos concretos (`examples-domain-flow.md`, `examples-testing.md`, `examples-infrastructure.md`) sempre que a tarefa envolver fluxo end-to-end, estratégia de testes ou lifecycle.
   - **Etapa 3** — modelar a alteração respeitando fronteiras arquiteturais antes de escrever código.
   - **Etapa 4** — implementar adaptando os exemplos ao contexto real; nunca replicar literalmente.
   - **Etapa 5** — executar o Checklist de Validação R0–R7 de `references/build.md` e reportar resultado.

**Economia de contexto obrigatória:**
- Carregar somente as referências exigidas pelos gatilhos da tarefa — cada referência desnecessária consome tokens sem benefício.
- Se mais de 4 referências forem necessárias, priorizar as 3 mais críticas e registrar as demais como contexto não carregado.
- Nunca carregar `patterns-structural.md` para Factory Function, Functional Options, Adapter, Decorator ou Facade — esses patterns já estão inline no SKILL.md.

**Robustez obrigatória:**
- R0: sem `init()`.
- R5.12: sem `panic` em produção.
- R6: `context.Context` em toda fronteira de IO; interface no consumidor, nunca no produtor.
- R7.6: `errors.Join` para agregar erros; `fmt.Errorf("ctx: %w", err)` para wrapping.
- Goroutines sempre canceláveis, com shutdown cooperativo e sem leak.

## Outbox

Ver secao "Outbox" em `AGENTS.md` para o contrato completo do `outbox.Publisher` e a regra obrigatoria de idempotencia por `event_id`.
