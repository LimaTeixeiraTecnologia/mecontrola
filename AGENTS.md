<!-- governance-schema: 1.0.0 -->
# Regras para Agentes de IA

Este diretorio centraliza regras para uso com agentes de IA em tarefas reais de analise, alteracao e validacao de codigo.

## Objetivo

Use estas instrucoes para manter consistencia, seguranca e qualidade ao trabalhar com codigo, configuracao, validacao e evolucao de sistemas.

## Arquitetura: monolito modular

O projeto aparenta ser um monolito modular, com separacao relevante por modulos, dominios ou componentes internos. A governanca deve proteger essas fronteiras e evitar dependencias circulares.

Stack detectada: Go.
Frameworks detectados: Fiber, gRPC.

## Estrutura de Pastas

```
.
.ai_spec_harness.json
.editorconfig
.env.example
.github
.github/agents
.github/agents/bugfix.agent.md
.github/agents/prd-writer.agent.md
.github/agents/project-analyzer.agent.md
.github/agents/refactorer.agent.md
.github/agents/reviewer.agent.md
.github/agents/task-executor.agent.md
.github/agents/task-planner.agent.md
.github/agents/technical-specification-writer.agent.md
.github/copilot-instructions.md
.github/dependabot.yml
.github/hooks
.github/hooks/governance.json
.github/hooks/post-execute-task.sh
.github/hooks/post-wave.sh
.github/hooks/pre-execute-all-tasks.sh
.github/hooks/subagent-stop-wrapper.sh
.github/hooks/validate-governance.sh
.github/hooks/validate-preload.sh
.github/skills
.github/skills/agent-governance
.github/skills/agent-governance/SKILL.md
.github/skills/agent-governance/references
.github/skills/agent-governance/references/bug-schema.json
.github/skills/agent-governance/references/ddd.md
.github/skills/agent-governance/references/enforcement-matrix.md
.github/skills/agent-governance/references/error-handling.md
.github/skills/agent-governance/references/messaging.md
.github/skills/agent-governance/references/multiple-choice-protocol.md
.github/skills/agent-governance/references/observability.md
.github/skills/agent-governance/references/persistence.md
.github/skills/agent-governance/references/security-app.md
.github/skills/agent-governance/references/security.md
.github/skills/agent-governance/references/severity-mapping.md
.github/skills/agent-governance/references/shared-architecture.md
.github/skills/agent-governance/references/shared-lifecycle.md
.github/skills/agent-governance/references/shared-patterns.md
.github/skills/agent-governance/references/shared-testing.md
.github/skills/agent-governance/references/testing.md
.github/skills/agent-governance/scripts
.github/skills/agent-governance/scripts/detect-architecture.sh
.github/skills/agent-governance/scripts/detect-toolchain.sh
.github/skills/agent-governance/triggers
.github/skills/agent-governance/triggers/go.yaml
.github/skills/agent-governance/triggers/node.yaml
.github/skills/agent-governance/triggers/python.yaml
.github/skills/analyze-project
.github/skills/analyze-project/SKILL.md
.github/skills/analyze-project/assets
.github/skills/analyze-project/assets/agents-template.md
.github/skills/analyze-project/assets/ai-tool-template.md
.github/skills/analyze-project/scripts
.github/skills/analyze-project/scripts/generate-governance.sh
.github/skills/analyze-project/scripts/lib
.github/skills/analyze-project/scripts/lib/codex-config.sh
.github/skills/analyze-project/scripts/lib/find-manifests.sh
.github/skills/bugfix
.github/skills/bugfix/SKILL.md
.github/skills/bugfix/assets
.github/skills/bugfix/assets/bugfix-report-template.md
.github/skills/bugfix/references
.github/skills/bugfix/references/canonical-bug-format.md
.github/skills/bugfix/scripts
.github/skills/bugfix/scripts/validate-bug-input.py
.github/skills/confluence-changelog-publisher
.github/skills/confluence-changelog-publisher/SKILL.md
.github/skills/create-prd
.github/skills/create-prd/SKILL.md
.github/skills/create-prd/assets
.github/skills/create-prd/assets/prd-template.md
.github/skills/create-tasks
.github/skills/create-tasks/SKILL.md
.github/skills/create-tasks/assets
.github/skills/create-tasks/assets/task-template.md
.github/skills/create-tasks/assets/tasks-template.md
```

## Padrao Arquitetural

Predominio de packages internos coesos, com estrutura orientada por dominio ou componente.

### Fluxo de Dependencias

- Transporte e infrastructure devem depender de casos de uso ou servicos explicitos, nao do contrario.
- Dominio nao deve conhecer detalhes de HTTP, banco, filas, serializacao ou drivers.
- Infraestrutura pode implementar contratos consumidos pela aplicacao, preservando dependencia para dentro.

### Layout Obrigatorio por Modulo

Novos modulos, novas features e refatoracoes estruturais em bounded contexts DEVEM usar separacao fisica clara por responsabilidade:

```text
internal/<modulo>/
  application/
    dtos/
      input/
      output/
    usecases/
    interfaces/
  domain/
    entities/
    valueobjects/
    services/
    interfaces/
  infrastructure/
    messaging/
      kafka/
        producers/
        consumers/
      rabbit/
        producers/
        consumers/
      nats/
        producers/
        consumers/
    repositories/
      postgres/
      mssql/
    http/
      server/
      client/
```

Responsabilidades obrigatorias:

1. `application/`: orquestracao de use cases, DTOs de entrada em `dtos/input`, DTOs de saida em `dtos/output` e interfaces/contratos consumidos pela aplicacao. Nao pode conter IO concreto, drivers, SDKs externos, SQL, brokers ou handlers HTTP.
2. `domain/`: entidades, value objects, domain services stateless, invariantes e interfaces/contratos que pertencem a linguagem ubiqua do dominio. Nao pode conhecer application, infrastructure, serializacao, banco, HTTP, filas ou configuracao.
3. `infrastructure/`: implementacoes concretas de IO e integracoes por tecnologia. Implementa contratos definidos em `application/` ou `domain/`, sem vazar tipos concretos para dentro do dominio.
4. `infrastructure/messaging/<broker>/producers`: publicacao/envio de mensagens para Kafka, Rabbit ou NATS.
5. `infrastructure/messaging/<broker>/consumers`: consumo/processamento de mensagens vindas de Kafka, Rabbit ou NATS.
6. `infrastructure/repositories/postgres` e `infrastructure/repositories/mssql`: persistencia concreta por banco.
7. `infrastructure/http/server`: entrada servida pelo modulo, como handlers, controllers e rotas HTTP/gRPC.
8. `infrastructure/http/client`: consumo de APIs externas ou servicos remotos.
### Plataforma Tecnica Compartilhada

Capacidades tecnicas reutilizaveis por mais de um modulo DEVEM viver em `internal/platform/`, mantendo visibilidade privada do monolito e evitando `pkg/` sem necessidade de consumo externo.

**Regra de Unicidade:** É PROIBIDO criar implementações locais ou redundantes de capacidades transversais (ex: geração de IDs, clock, hashing) dentro de `internal/<modulo>/...`. Se uma capacidade for necessária em múltiplos contextos, ela deve ser promovida ou criada diretamente em `internal/platform/`.

```text
internal/platform/
  clock/
  database/
  errors/
  events/
  http/
  httpclient/
  id/
  observability/
  outbox/
  worker/
  secrets/
```

`internal/platform/` nao e modulo de negocio e nao pode importar `internal/<modulo>/...`. Modulos podem consumir `internal/platform/` apenas nas camadas em que a dependencia tecnica seja permitida pela fronteira arquitetural; `domain` permanece puro e nao pode importar `platform`, banco, HTTP, filas, serializacao, configuracao ou drivers.

### HTTP Outbound Mandatório

Toda chamada HTTP a APIs externas (Kiwify, LLMs, provedores de notificação, etc.)
DEVE usar `internal/platform/httpclient` como wrapper sobre `devkit-go/pkg/httpclient`.
O wrapper garante timeouts explícitos, tracing W3C, métricas automáticas e política
segura de retry (apenas métodos idempotentes por padrão).

**Padrão de uso (bootstrap do módulo):**

```go
client, err := platformhttpclient.NewClient(
    provider.Observability(),
    platformhttpclient.WithTimeout(cfg.HTTPTimeout),
    platformhttpclient.WithBaseURL(cfg.APIBaseURL),
    platformhttpclient.WithDefaultRetry(cfg.HTTPRetryMaxAttempts, cfg.HTTPRetryBackoff),
    platformhttpclient.WithTarget("kiwify"),
)
```

**Proibido fora de testes:**

1. Instanciar `&http.Client{}` diretamente em código de produção.
2. Chamar `devkit-go/pkg/httpclient.NewObservableClient` sem passar por este wrapper.
3. Realizar requisições HTTP outbound sem timeout explícito.

Camadas auxiliares (rate limit, fluxo OAuth, retry específico de 429) ficam acima
do wrapper, dentro do client da integração em `internal/<modulo>/infrastructure/http/client/`.

### Proibições e Padrões desencorajados

1. **Proibido o uso de pacotes de Clock globais ou compartilhados:** Não criar ou utilizar pacotes como `internal/platform/clock`. O tempo deve ser tratado como uma dependência local.
   - **Use Cases/Domain:** Injetar `now func() time.Time`.
   - **Infrastructure:** Declarar interface `Clock` local e privada se necessário (Interface Segregation).
   - **Motivo:** Reduzir acoplamento desnecessário e "ruído" em camadas de domínio.

Os modulos ativos devem usar `infrastructure/` como camada fisica de implementacoes concretas. Nao criar diretorios alternativos para a mesma responsabilidade.

Fluxo permitido: `infrastructure -> application -> domain`. `application` pode importar `domain`, mas nao pode importar `infrastructure`. `domain` nao pode importar nenhuma camada externa. Comunicacao cross-module deve ocorrer apenas por interface declarada pelo consumidor, domain event/outbox ou contrato explicito.

## Modo de trabalho

1. Entender o contexto antes de editar qualquer arquivo.
2. Preferir a menor mudanca segura que resolva a causa raiz.
3. Preservar arquitetura, convencoes e fronteiras ja existentes no contexto analisado.
4. Nao introduzir abstracoes, camadas ou dependencias sem demanda concreta.
5. Atualizar ou adicionar testes quando houver mudanca de comportamento.
6. Rodar validacoes proporcionais a mudanca.
7. Registrar bloqueios e suposicoes explicitamente quando o contexto estiver incompleto.

## Diretrizes de Estrutura

1. Priorize entendimento do codigo e do contexto atual antes de propor refatoracoes.
2. Respeite padroes existentes de nomenclatura, organizacao e tratamento de erro.
3. Defina estrutura simples, evolutiva e com defaults explicitos.
4. Evite reescritas amplas quando uma alteracao localizada resolver o problema.
5. Estabeleca contratos, testes e comandos de validacao cedo quando eles ainda nao existirem.
6. Considere risco de regressao como restricao principal.
7. Evite overengineering disfarcado de arquitetura futura.
8. Elimine comentarios por padrao. Comentarios so devem existir quando forem extremamente relevantes para explicar decisao, invariante, risco, contrato publico ou comportamento nao obvio.
9. Proiba comentarios mecanicos, redundantes ou que apenas repitam nomes de funcoes, structs, campos ou fluxo evidente.

## Regras por Arquitetura

1. Respeitar fronteiras entre modulos e bounded contexts.
2. Evitar dependencia circular entre packages internos.
3. Nao extrair shared helpers sem demanda comprovada de mais de um modulo.

## Regras por Linguagem

Para tarefas que alteram codigo, carregar a skill:

- `.agents/skills/agent-governance/SKILL.md`

Para tarefas que alteram codigo Go, carregar tambem:

- `.agents/skills/go-implementation/SKILL.md`

O uso de `.agents/skills/go-implementation/SKILL.md` e mandatorio para qualquer alteracao em codigo Go. Seguir seus exemplos como referencia de padrao, adaptando ao contexto real do modulo e sem copia cega. Aplicar economia de contexto: carregar somente referencias necessarias pelos gatilhos e pela complexidade da tarefa, preferindo TL;DR quando suficiente.

Para tarefas de revisao ou refatoracao incremental de design em Go guiadas por heuristicas de object calisthenics, carregar tambem:

- `.agents/skills/object-calisthenics-go/SKILL.md`

Para tarefas de correcao de bugs com remediacao e teste de regressao, carregar tambem:

- `.agents/skills/bugfix/SKILL.md`

### Composicao Multi-Linguagem

Em projetos com mais de uma linguagem (ex: monorepo Go + Node), carregar apenas a skill da linguagem afetada pela mudanca. Se a tarefa cruzar linguagens, carregar ambas e aplicar a validacao de cada stack nos arquivos correspondentes. Nao misturar convencoes de uma linguagem em arquivos de outra.

## Referencias

Cada skill lista suas proprias referencias em `references/` com gatilhos de carregamento no respectivo `SKILL.md`. Nao duplicar a listagem aqui — consultar o SKILL.md da skill ativa para saber quais referencias carregar e em que condicao.

## Notas por Ferramenta

- **Claude Code**: skills pre-carregadas via `.claude/skills/`, hooks via `.claude/hooks/`, agents delegados via `.claude/agents/`.
- **Gemini CLI**: commands em `.gemini/commands/*.toml` apontam para skills canonicas. Sem hooks ou agents nativos — o modelo deve seguir as instrucoes procedurais do SKILL.md carregado.
- **Codex**: le `AGENTS.md` como instrucao de sessao. Entradas em `.codex/config.toml` sao metadados para `upgrade.sh`, nao spec oficial do Codex CLI. O agente deve seguir as instrucoes de `AGENTS.md` para descobrir e carregar skills.
- **Copilot**: `.github/copilot-instructions.md` como instrucao principal. `.github/agents/` sao wrappers. Sem hooks nativos — compliance depende do modelo seguir as instrucoes.

### Obrigatoriedade por Ferramenta

Codex e Claude DEVEM respeitar TODAS as regras, skills, references, validacoes, restricoes de arquitetura, regras de economia de contexto e politicas de comentarios de forma igualitaria. Isso e obrigatorio e inegociavel.

Nenhuma ferramenta pode flexibilizar regras por ausencia de enforcement automatico, diferenca de runtime, hooks, agentes pre-carregados ou conveniencia operacional. Quando uma ferramenta nao tiver enforcement programatico, a compliance deve ser procedural e igualmente obrigatoria.

Se houver divergencia entre arquivos especificos de ferramenta e `AGENTS.md`, prevalece `AGENTS.md`. Arquivos como `CLAUDE.md`, `GEMINI.md` e `.github/copilot-instructions.md` so podem reforcar ou apontar para a fonte canonica, nunca enfraquecer regras.

### Matrix de Enforcement

| Capacidade | Claude Code | Gemini CLI | Codex | Copilot |
|---|---|---|---|---|
| Carga base automatica | hook PreToolUse | procedural | procedural | procedural |
| Protecao de governanca | hook PostToolUse | procedural | procedural | procedural |
| Skills pre-carregadas | sim (symlinks) | sim (commands) | nao | sim (agents) |
| Enforcement programatico | sim (hooks) | nao | nao | nao |
| Validacao de evidencias | script | procedural | procedural | procedural |

Ferramentas sem enforcement programatico dependem do modelo seguir instrucoes procedurais. A compliance nessas ferramentas continua obrigatoria.

## Economia de Contexto

Carregar o minimo necessario para a tarefa reduz custo de tokens em 35-50%:

| Complexidade | Criterio | O que carregar |
|---|---|---|
| `trivial` | Rename, typo, import, formatacao | Apenas AGENTS.md |
| `standard` | Bug fix, novo metodo, refactor local | AGENTS.md + TL;DR das references afetadas |
| `complex` | Nova feature, interface publica, migracao | AGENTS.md + referencias completas |

- Classificar a complexidade **antes** de carregar qualquer referencia.
- Quando a reference tiver bloco `<!-- TL;DR ... -->`, preferir o TL;DR ao documento completo em tarefas standard.
- Override explicito via `--complexity=<nivel>` prevalece sobre classificacao automatica.

## Validacao

Antes de concluir uma alteracao:

Seguir Etapa 4 de `.agents/skills/agent-governance/SKILL.md` como base canonica.

Comandos detectados no projeto (Go):
1. Rodar fmt: `gofmt -w .`.
2. Rodar test: `go test ./...`.
3. Rodar lint: `golangci-lint run`.

## Outbox vs events.Bus

<!-- RF-38 / ADR-016 — contrato do outbox.Publisher -->

Use `outbox.Publisher` (`internal/platform/outbox`) para side-effects **criticos** que precisam ser entregues mesmo apos crash, deploy ou reinicio do worker: notificacoes, projecoes persistentes, integracoes externas disparadas pos-commit. O Publisher garante at-least-once escrevendo atomicamente na transacao do agregado.

Use `events.Bus` (ADR-003) para sinais **volateis** in-process que podem ser perdidos sem impacto ao produto: telemetria em tempo real, propagacao de cache local, triggers de UI. O Bus descarta mensagens quando o canal do subscriber esta cheio — comportamento intencional.

**Regra obrigatoria de idempotencia:** Todo `outbox.Handler` DEVE ser idempotente por `event.ID`. O Dispatcher entrega at-least-once; o handler e responsavel por evitar duplicacao via upsert ou tabela de deduplicacao.

Criterio de escolha resumido (ver ADR-016 para detalhes):

| Precisa sobreviver a restart? | Tem side-effect externo? | Publisher escolhido |
|---|---|---|
| Sim | Sim | `outbox.Publisher` |
| Nao | Nao | `events.Bus` |

Ambos coexistem; um nao substitui o outro. Documentar no godoc do handler qual foi escolhido e por que (RF-38).

## Restricoes

1. Nao inventar contexto ausente.
2. Nao assumir versao de linguagem, framework ou runtime sem verificar.
3. Nao alterar comportamento publico sem deixar isso explicito.
4. Nao usar exemplos como copia cega; adaptar ao contexto real.


### Controle de profundidade de invocacao

- Skills que invocam outros skills (execute-task, refactor) devem verificar profundidade via `scripts/lib/check-invocation-depth.sh`.
- Limite padrao: 2 niveis. Configuravel via `AI_INVOCATION_MAX`.
- Variaveis de ambiente: `AI_INVOCATION_DEPTH` (corrente), `AI_INVOCATION_MAX` (limite).
