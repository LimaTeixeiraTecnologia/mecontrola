# Claude Code

Use `AGENTS.md` como fonte canonica das regras deste repositorio.

Claude deve respeitar TODAS as regras, skills, references, validacoes, restricoes de arquitetura, economia de contexto e politicas de comentarios definidas em `AGENTS.md` de forma igualitaria ao Codex. Isso e obrigatorio e inegociavel.

Este `CLAUDE.md` segue as recomendacoes oficiais do Claude Code: manter o arquivo enxuto, colocar regras genericas em `AGENTS.md`, e usar `.claude/skills/` e `.claude/agents/` para conhecimento especifico carregado sob demanda.

## Instrucoes

1. Ler `AGENTS.md` no inicio da sessao.
2. `.claude/skills/` referenciam `.agents/skills/` (por symlink ou copia sincronizada) — a fonte de verdade e sempre `.agents/skills/`.
3. `.claude/agents/` sao wrappers leves que delegam para a habilidade canonica.
4. Em tarefas de execucao, carregar apenas `AGENTS.md`, `agent-governance` e a skill operacional da linguagem ou atividade afetada.
5. Skills de planejamento (`analyze-project`, `create-prd`, `create-technical-specification`, `create-tasks`) entram apenas quando a tarefa pedir esse fluxo explicitamente.
6. Carregar referencias adicionais apenas quando a tarefa exigir.
7. Preservar estilo, arquitetura e fronteiras existentes antes de propor mudancas.
8. Validar mudancas com comandos proporcionais ao risco.
9. Nao flexibilizar nenhuma regra por diferenca de ferramenta, hook, agente pre-carregado ou conveniencia operacional.
10. Para `internal/identity` e `internal/billing`, seguir o "Padrao Obrigatorio de Modulo" em `AGENTS.md`; nao inventar wiring, routers, jobs, consumers ou adapters ausentes.

## Praticas Oficiais do Claude Code

### Prompts internos

- Usar XML tags (`<context>`, `<task>`, `<rules>`, `<format>`, `<example>`) para instrucoes multi-parte.
- Preferir prompts curtos e encadeados a um mega-prompt.
- Incluir regra de uncertainty: "se incerto, diga explicitamente".
- Fornecer um check pass/fail (teste, build, linter) antes de encerrar uma tarefa.
- Preferir Structured Outputs (schema estrito) a prefill para saida conformada — prefill foi descontinuado nos modelos Claude 4.6+.
- Tratar refusal e stop_reason (`refusal`, `stop_reason`/`finish_reason=length`) antes de consumir o payload do modelo.
- Usar enums fechados no schema para tornar estados ilegais irrepresentaveis (sinergia direta com DMMF state-as-type).

### Subagentes e economia de contexto

- Delegar investigacao que leia >=10 arquivos ou exceda ~20 tool calls a um subagente (`Explore`, `Plan` ou custom).
- Usar subagentes para revisao adversarial do diff antes de declarar pronto.
- Manter a sessao principal focada na implementacao; apenas conclusoes voltam dela.

### CLAUDE.md, Skills e Hooks

- `CLAUDE.md` e o contrato raiz; regras genericas e transversais ficam em `AGENTS.md`.
- Skills em `.claude/skills/` (fonte `.agents/skills/`) carregam conhecimento sob demanda.
- Hooks garantem acoes deterministicas (ex.: formatacao apos edit, lint antes de commit).
- Para novas capacidades, preferir criar uma skill a aumentar o tamanho deste arquivo.

## Go — Regra Mandatória e Inegociável

Toda implementação, alteração ou revisão de código Go DEVE obrigatoriamente:

1. Carregar o **Trio Obrigatorio de Desenvolvimento Go** definido em `AGENTS.md` (secao `## Skills Obrigatorias`), no modelo consultar-sempre / materializar-por-gatilho: `go-implementation` (SEMPRE, antes de qualquer edição), `design-patterns-mandatory` (gate de desenho `aplicar` vs. `nao aplicar padrao`; seletor/bundle so sob gatilho de pattern) e `domain-modeling-production` (quando a mudança toca domínio ou discovery de fluxo).
2. Verificar a versão declarada em `go.mod` antes de introduzir APIs da linguagem ou dependências.
3. Executar as **Etapas 1 a 5** do SKILL.md na integra.
4. Aplicar a matriz de validacao de `AGENTS.md` (secao `## Validacao` e item 6 de `## Modo de Trabalho`) conforme o risco da mudanca:
   - `domain/` ou API publica/contrato: build, vet, test race, lint e gates de governanca em todo o projeto.
   - `application/` ou `infrastructure/` sem API publica: build, vet, test race, lint no modulo alterado.
   - adapters (handlers/consumers/jobs/producers): build/vet do pacote + lint + gates R-ADAPTER-001.
   - scripts/docs/configs: `gofmt -l` e validacao sintatica.

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

**R-ADAPTER-001 (hard) — Zero comentários e adaptadores finos:**
- Zero comentários em todos os arquivos `.go` de produção — inegociável. Exceções: `// Code generated`, diretivas `//go:`, `//nolint:` com justificativa na mesma linha.
- Nos quatro caminhos de adapter (`handlers/`, `consumers/`, `jobs/handlers/`, `producers/`): fluxo `adapter → usecase` obrigatório; SQL direto, branching de domínio e regra de negócio são proibidos.
- Carregar referências go-implementation conforme Matriz R-ADAPTER-001.3 em `.claude/rules/go-adapters.md`.
- Ver `.claude/rules/go-adapters.md` para contrato completo e gates de verificação.

**R-AGENT-WF-001 (hard) — substrato `internal/platform/agent` + consumidor `internal/agents` (emenda 2026-06-29; `internal/agent` descontinuado):**
- Fluxo inbound: `InboundRequest → AgentRuntime.Execute → ThreadGateway.GetOrCreate → RunStore.Insert → AgentRegistry.Resolve → Agent.Execute (loop tool-calling) → MessageStore.Append → closeRun`. Durável multi-step via `workflow.Engine[S].Start/Resume`.
- Comportamento novo = novo agente/tool/workflow/scorer no consumidor, montando primitivos do substrato. Sem `switch case intent.Kind`; resolução por registry. Skill operacional: `mastra`.
- Tool é adapter fino (`tool.NewTool[I,O]`): zero regra de negócio, SQL ou branching; o `exec` delega a client/usecase.
- Estados fechados: `agent.RunStatus`/`agent.ToolOutcome`, `workflow.RunStatus`/`StepStatus`/`SuspendReason`, `scorer.ScorerKind`, `memory.MessageRole` — nunca string livre.
- Thread → Run: toda execução resolve `Thread(resourceId, threadId)` opaco e abre/fecha um `Run` auditável.
- Pending step: estado de espera fechado salvo no `Snapshot` do kernel; resume por merge-patch antes do parse.
- WorkingMemory injetada no system prompt quando disponível. LLM só nas call-sites sancionadas (loop do agent, step `Stream`, scorer LLM-judged); OpenRouter único provider.
- Thread/Run/WorkingMemory/PendingStep são primitivos de `internal/platform/{agent,memory}`; consumidos pelos módulos, nunca reimplementados em domínio.
- Ver `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.1–001.8 + addendum .6-A/.8-A) para contrato completo.

**R-WF-KERNEL-001 (hard) — Kernel genérico de workflow em `internal/platform/workflow`:**
- Proibido import de pacote de dominio (`internal/transactions`, `internal/billing`, `internal/identity`) ou de camada superior que consome o kernel (`internal/platform/agent`, `internal/platform/memory`, `internal/platform/llm`, `internal/platform/scorer`, `internal/platform/tool`), bem como qualquer tipo semantico (`intent`, `pendingexpense`, `category`, `agent`).
- Proibido regra de negocio, branching de dominio e LLM no kernel.
- SQL apenas no adapter Postgres (`infrastructure/postgres/`).
- Estados (`RunStatus`/`StepStatus`/`SuspendReason`) sao tipos fechados — nunca string livre.
- Metricas com cardinalidade controlada: labels permitidos `workflow`, `step`, `status`, `outcome`; proibido `user_id`/`correlation_key`/`category_id` como label.
- Gate bloqueante: regra redigida antes de qualquer codigo do kernel (ADR-004).
- Ver `.claude/rules/workflow-kernel.md` para contrato completo e gates de verificacao.

## DMMF e Mastra

Ver secoes "DMMF — Domain Modeling Made Functional" e "Padrao de Agent — substrato `internal/platform/agent`" em `AGENTS.md`. Resumo critico para Claude:

- **State-as-type `[HARD]`**: `agent.RunStatus`/`agent.ToolOutcome`/`agent.AwaitingKind`, `workflow.RunStatus`/`StepStatus`/`SuspendReason`, `scorer.ScorerKind`, `memory.MessageRole` sao tipos fechados com constantes enumeradas; nunca `string` livre em assinatura publica.
- **Pending step** (Mastra-inspired): estado de espera modelado como tipo fechado, salvo no `Snapshot` do kernel e retomado por merge-patch antes do parse; sem side-store de dominio.
- **Decide* puro `[HARD]`**: sem IO, sem `context.Context`, deterministico; regra de negocio vive exclusivamente aqui.
- **Anti-padroes proibidos `[HARD]`**: `Result[T,E]` customizado, currying, DSL de pipeline, monades.
- **Primitivos de plataforma `[HARD]`**: Thread = `(resourceId, threadId)` opaco resolvido a cada execucao; Run aberto e fechado com RunStatus fechado; working memory no system prompt; resume antes do parse. Vivem em `internal/platform/{agent,memory}`; consumidos pelos modulos (ex.: `internal/agents`), nunca reimplementados em dominio.

## Outbox

Ver secao "Outbox" em `AGENTS.md` para o contrato completo do `outbox.Publisher` e a regra obrigatoria de idempotencia por `event_id`.

## Referencias Oficiais

- [Claude Platform Docs — Intro](https://platform.claude.com/docs/en/overview)
- [Claude — Prompting best practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-4-best-practices)
- [Claude — Tool use overview](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview)
- [Claude — Structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)
- [Claude — MCP connector](https://platform.claude.com/docs/en/agents-and-tools/mcp-connector)
- [Claude Code — Best Practices](https://code.claude.com/docs/en/best-practices)
- [Claude Code — Common Workflows](https://code.claude.com/docs/en/common-workflows)
- [OpenAI — Function calling](https://developers.openai.com/api/docs/guides/function-calling)
- [OpenAI — Structured outputs](https://developers.openai.com/api/docs/guides/structured-outputs)
- [OpenAI — Agents SDK](https://developers.openai.com/api/docs/guides/agents)
- [OpenAI — Cost optimization](https://developers.openai.com/api/docs/guides/cost-optimization)
- [OpenAI — Deployment checklist](https://developers.openai.com/api/docs/guides/deployment-checklist)
