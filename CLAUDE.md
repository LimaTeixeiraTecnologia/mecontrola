# Claude Code

Use `AGENTS.md` como fonte canonica das regras deste repositorio.

## Instrucoes

1. Ler `AGENTS.md` no inicio da sessao.
2. `.claude/skills/` sao symlinks para `.agents/skills/` — a fonte de verdade e sempre `.agents/skills/`.
3. `.claude/agents/` sao wrappers leves que delegam para a habilidade canonica.
4. Em tarefas de execucao, carregar apenas `AGENTS.md`, `agent-governance` e a skill operacional da linguagem ou atividade afetada.
5. Skills de planejamento (`analyze-project`, `create-prd`, `create-technical-specification`, `create-tasks`) entram apenas quando a tarefa pedir esse fluxo explicitamente.
6. Carregar referencias adicionais apenas quando a tarefa exigir.
7. Preservar estilo, arquitetura e fronteiras existentes antes de propor mudancas.
8. Validar mudancas com comandos proporcionais ao risco.

## Contrato Go

Em qualquer tarefa que envolva codigo Go, carregar obrigatoriamente:

1. `AGENTS.md`
2. `.agents/skills/agent-governance/SKILL.md`
3. `.agents/skills/go-implementation/SKILL.md`

Esta regra e `[HARD]` para criacao, edicao, refatoracao, revisao, correcao ou validacao que
toque `.go`, `go.mod`, `go.sum`, testes Go, mocks, build, lint, CI ou configuracao Go.
Verificar fatos locais antes de assumir versao, dependencia, ferramenta, framework ou padrao.
Carregar referencias da skill Go apenas sob demanda e adaptar exemplos ao contexto real.
