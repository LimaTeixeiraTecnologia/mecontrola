# Prompt Enriquecido: Refatoração do Módulo internal/agent com Go, Mastra e DMMF

Este prompt foi gerado e enriquecido com base no objetivo do usuário de refatorar o módulo `internal/agent` seguindo os princípios de Go-Implementation (`go-implementation`), Domain Modeling Made Functional (DMMF) e o framework Mastra como inspiração inegociável de arquitetura de agentes, workflows e ferramentas.

---

## prompt_metadata
- **Título**: Refatoração do Módulo internal/agent com Go, Mastra e DMMF
- **Idioma**: Português (PT-BR)
- **Tecnologias**: Go, DMMF, Mastra (Arquitetura de Agents, Workflows, Tools, Threads, Runs)
- **Regras Mandatórias**: `.agents/skills/go-implementation`, `AGENTS.md`, `.claude/rules/agent-workflows-tools.md`, `.claude/rules/governance.md`
- **Status**: Production-ready, com critérios estritos de aceitação e evidência.

---

# PROMPT DE INSTRUÇÃO PARA O AGENTE DE EXECUÇÃO

## Objetivo Principal
Você deve refatorar por completo o módulo `internal/agent` do projeto `mecontrola` para torná-lo aderente ao padrão canônico **Workflow + Tool** de roteamento, usando o framework **Mastra** (https://github.com/mastra-ai/mastra) como inspiração arquitetural (trazendo seus conceitos de Agents, Workflows, Tools, Threads, Runs e WorkingMemory para Go de forma idiomática e robusta). A implementação deve respeitar as regras estritas da skill de Go-Implementation (`.agents/skills/go-implementation`), os princípios de Domain Modeling Made Functional (DMMF) e as diretrizes transversais de governança (`AGENTS.md` e `.claude/rules/agent-workflows-tools.md`).

---

## 1. Contexto e Requisitos Inegociáveis (Hard Constraints)

### 1.1. Arquitetura Canônica de Roteamento (R-AGENT-WF-001.1)
O fluxo de execução do agente deve ser estritamente linear e desacoplado:
```
IntentRouter -> WorkflowRegistry.Resolve(kind) -> Workflow.Execute -> Tool.Execute -> binding -> usecase -> domain -> repo
```
- **Proibido**: Adicionar novos `case` de domínio ao `switch` de `internal/agent/application/services/daily_ledger_agent.go`. Todo novo comportamento deve ser um `Workflow` registrado no `WorkflowRegistry`.
- **Proibido**: Lógica de roteamento por intent kind fora de um `Workflow`.
- **Proibido**: Chamar bindings ou use cases diretamente do entrypoint sem passar pela cadeia de execução `Workflow -> Tool`.
- O arquivo `daily_ledger_agent.go` deve permanecer fino: apenas orquestra o registry, executa guardas de escrita e formata saídas compartilhadas.

### 1.2. Padrão Tool como Adapter Fino (R-AGENT-WF-001.2)
Cada `Tool` deve ter **uma única responsabilidade** e atuar como um adapter fino sobre `binding -> usecase`.
- **Proibido** em qualquer `Tool` (`internal/agent/application/tools/`) ou `Workflow` (`internal/agent/application/workflow/`):
  1. Lógica ou regras de negócio (ex: normalizar allocations, decidir status).
  2. Consultas SQL diretas (`QueryContext`, `ExecContext`, etc.).
  3. Desvios (branching) baseados no estado de domínio.
- **Permitido**: Mapear `intent.Intent` para o DTO/command correspondente do usecase, invocar o binding, mapear o retorno para `ToolResult` e fazer wrapping de erro.
- A lógica de pré-escrita (autorização, replay de segurança, políticas e auditoria) deve viver centralizada no step de guarda reutilizável (`write_guard.go`), compartilhado por todos os workflows de escrita, sem duplicação em cada tool.

### 1.3. Domain Modeling Made Functional (DMMF) & State-as-Type (R-AGENT-WF-001.3)
Toda definição de estado ou resultado deve usar tipos fechados enumerados, nunca strings livres:
- `RunStatus` aceita exclusivamente: `running | succeeded | failed`.
- `ToolOutcome` aceita exclusivamente o conjunto fechado de resultados do agente (ex: `routed`, `clarify`, `usecaseError`, `missingResolver`).
- `AwaitingKind` aceita exclusivamente: `category_confirm | category_choice`.
- `TransactionKind` aceita exclusivamente: `expense | income | card_purchase`.
- **Proibido**: Representar esses estados como strings puras nas assinaturas ou lógica de ramificação. Use smart constructors para criar e validar as transições dessas structs de estado.

### 1.4. Ciclo de Execução: Thread-First & Run Auditável (R-AGENT-WF-001.5 / R-AGENT-WF-001.6)
Toda chamada a `AgentRuntime.Execute` deve seguir o ciclo de vida inspirado no Mastra:
1. **Thread Gateway**: Resolver ou criar uma `Thread` baseada em `(user_id, channel)` usando `ThreadGateway.GetOrCreate(userID, channel)`.
   - Identidade canônica: `resourceId = user_id`, `threadId = channel`.
2. **Run Auditável**: Abrir e registrar a execução como um `Run` contendo: `thread_id`, `run_id`, `workflow`, `tool`, `status` (`RunStatus`), `duration_ms` e `error`.
   - Se for uma escrita, deve referenciar o `decision_id` correspondente para a trilha de auditoria.
   - Métricas: Apenas enums fechados (`agent_id`, `channel`, `workflow`, `status`, `tool`, `outcome`) são permitidos como labels de métricas. **Proibido** usar identificadores de usuário ou de categoria como labels de métricas.

### 1.5. Limitações e Controles de Inteligência Artificial
- **LLM apenas no step de Parse (R-AGENT-WF-001.4)**: A IA (LLM) só pode atuar na decodificação e intenção inicial (`ParseInbound`). Workflows e tools executam lógica determinística em cima do `intent.Intent` já estruturado. Proibido chamar LLM dentro de workflows ou tools.
- **WorkingMemory no System Prompt (R-AGENT-WF-001.8)**: O `ContextBuilder` deve ler a `WorkingMemory` (em markdown estruturado) do usuário pelo `user_id` e acoplá-la ao system prompt do LLM no parser se ela existir.
- **Drafts em Clarificações de Categoria (R-AGENT-WF-001.7)**: Quando um erro de categoria ambígua ou confirmação de categoria for detectado (`CategoryAmbiguousError` ou `CategoryNeedsConfirmationError`), o sistema deve salvar um `pendingexpense.Draft` (com `AwaitingKind` fechado) no banco de dados e retornar `OutcomeClarify`. A retomada (`resume`) deve interceptar o fluxo antes do parse do LLM.

---

## 2. Padrões de Implementação Go (R0-R7)

Ao escrever e estruturar o código Go do módulo, você deve aplicar com rigor a skill `go-implementation`:
- **R0 (init proibido)**: Nenhuma função `init()` é permitida em qualquer arquivo de produção.
- **R1 (Struct Methods)**: Todas as funções de domínio, aplicação ou infraestrutura devem ser métodos de structs. Funções puras são permitidas apenas para main, factories/construtores de instâncias e helpers.
- **Zero comentários em código Go**: Proibido adicionar comentários explicativos em Go. Apenas comentários de geração de código (ex: mock) e diretivas `//go:` ou `//nolint:` (com justificativa na mesma linha) são permitidos.
- **Contratos e Context**: Toda fronteira de I/O deve receber `context.Context` como primeiro parâmetro. Injeção de Dependência (DI) deve ser feita exclusivamente via construtores manuais explícitos em `module.go`.
- **Proibido `var _ Interface = (*Type)(nil)`**: Não use este padrão de asserção estática de interface.
- **Sem `clock.Clock`**: Não use mocks ou abstrações compartilhadas de clock no domínio ou usecases. Use `time.Now().UTC()` ou passe instantes em commands.
- **Tratamento de erros**: Trate erros uma única vez. Use `errors.New`, `%w` com `fmt.Errorf` para contexto ou `errors.Join` para agregação.

---

## 3. Roteiro Passo a Passo de Execução

Siga rigorosamente estas fases ao implementar a refatoração:

### Fase 1: Exploração e Mapeamento
1. Localize a implementação atual em `internal/agent/`.
2. Analise `internal/agent/module.go` para compreender a fiação de dependências (wiring) atual.
3. Mapeie o switch-case existente em `daily_ledger_agent.go` e liste todas as intenções tratadas que devem migrar para o padrão de `Workflow` e `Tool`.
4. Leia as dependências de banco de dados, repositórios de auditoria e workers que o agente utiliza.

### Fase 2: Modelagem DMMF (Domain & Application Types)
1. Crie ou ajuste os tipos enumerados e construtores inteligentes (smart constructors) para garantir que `RunStatus`, `ToolOutcome`, `AwaitingKind` e `TransactionKind` sejam tipos fechados e seguros.
2. Modele as structs `Workflow` e `Tool` de forma que encapsulem sua execução com segurança de tipos.
3. Modele os esquemas de dados e entidades de persistência de `Thread` e `Run` sob `internal/agent/domain/entities` ou `valueobjects`.

### Fase 3: Desenvolvimento dos Workflows e Tools
1. Implemente o `WorkflowRegistry` capaz de registrar e resolver workflows pelo intent kind.
2. Implemente os workflows de escrita e de leitura do agente.
3. Crie a proteção de gravação centralizada (`write_guard.go`) para validar autorizações e replays antes que qualquer tool de escrita seja executada.
4. Escreva cada `Tool` individual como adapter fino, realizando o mapeamento do intent e chamando o binding apropriado.
5. Remova as lógicas de switch-case de domínio de `daily_ledger_agent.go`, deixando apenas o fluxo canônico de resolução de workflows.

### Fase 4: Thread, Run e Working Memory
1. Implemente o `ThreadGateway` para resolver `(userID, channel)` no banco de dados.
2. Garanta que `AgentRuntime.Execute` abra um `Run` no banco ao iniciar e atualize o status para `succeeded` ou `failed` e salve o erro no final da execução de forma atômica ou via transação adequada.
3. Acople a leitura da `WorkingMemory` no `ContextBuilder` para alimentar o parser de LLM.
4. Adicione a lógica de salvar o `pendingexpense.Draft` quando retornar clarificação de categoria.

### Fase 5: Fiação (Wiring) e Testes
1. Atualize a DI manual em `internal/agent/module.go` com os novos repositórios, gateways, workflows, tools e registries criados.
2. Escreva testes unitários para os workflows e as tools de forma desacoplada.
3. Escreva testes de integração cobrindo a resolução de intents via registry até a persistência do Run e do Thread correspondentes.

---

## 4. Portões de Validação (Verification Gates)

Você deve validar a refatoração localmente antes de considerar a tarefa finalizada. Execute os seguintes testes no terminal do workspace:

1. **Validação de Compilação e Erros Básicos**:
   ```bash
   go build ./internal/agent/...
   go vet ./internal/agent/...
   ```

2. **Execução de Testes Unitários e Race Conditions**:
   ```bash
   go test -race -count=1 ./internal/agent/...
   ```

3. **Verificação de Regras Estritas de Roteamento (Gate 1)**:
   Garanta que nenhum case de domínio sobrou no switch-case do agente:
   ```bash
   f=$(find internal/agent -name "daily_ledger_agent.go" ! -name "*_test.go")
   if [ ! -z "$f" ]; then
     cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
     if [ "${cases:-0}" -gt 1 ]; then
       echo "FAIL: O switch de domínio cresceu ou permaneceu em daily_ledger_agent.go (cases=$cases). Use WorkflowRegistry!"
       exit 1
     fi
   fi
   ```

4. **Verificação de Comentários Proibidos (Gate 2)**:
   Garanta que nenhuma tool ou workflow contém comentários explicativos:
   ```bash
   grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
     "^[[:space:]]*//" \
     internal/agent/application/tools/ \
     internal/agent/application/workflow/ 2>/dev/null \
     | grep -Ev "(//go:|//nolint:|// Code generated)" \
     && { echo "FAIL: Comentários proibidos encontrados em tools/workflow!"; exit 1; } || true
   ```

5. **Verificação de Acesso SQL Direto (Gate 3)**:
   Garanta que nenhuma tool ou workflow interage diretamente com o banco por meio de SQL:
   ```bash
   grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
     "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
     internal/agent/application/tools/ \
     internal/agent/application/workflow/ 2>/dev/null \
     && { echo "FAIL: Lógica de SQL direto encontrada em tool ou workflow!"; exit 1; } || true
   ```

---

## 5. Critérios de Aceitação (Definition of Done)

Para que a entrega seja considerada concluída, as seguintes evidências e estados devem ser atendidos:
1. **Compilação e Sucesso nos Testes**: Todos os testes do pacote `internal/agent` passam sem race condition.
2. **Padrão Workflow + Tool Totalmente Operacional**: Toda execução do agente passa pela cadeia `IntentRouter -> WorkflowRegistry -> Workflow -> Tool`.
3. **Mastra Engine Reproduzida**: Existência dos conceitos estruturados e implementados de `ThreadGateway`, `Run` auditável (persistido), e `WorkingMemory` acoplada ao prompt do parser.
4. **Drafts de categoria operantes**: O draft é salvo em banco em caso de clarificação e restabelecido no fluxo de resume.
5. **Adesão às regras de Go e DMMF**: Código limpo, sem `init()`, sem panics, sem comentários em Go, sem imports proibidos, e usando tipos enumerados fechados para outcomes, status e kinds.
6. **Relatório de Modificações**: Um arquivo de runbook ou log detalhando quais arquivos foram criados, modificados ou removidos, e comprovando a execução dos 5 portões de validação acima.
