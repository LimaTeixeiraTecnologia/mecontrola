# Prompt Enriquecido: Refatoração do Módulo internal/agent com Go, Mastra e DMMF

Este prompt de instrução destina-se a orientar um agente de IA no processo de refatoração do módulo `internal/agent` utilizando Go, o framework Mastra como inspiração de design de agentes/workflows, e os princípios do Domain Modeling Made Functional (DMMF) adaptados para Go.

---

## prompt_metadata
- **Título**: Refatoração do Módulo internal/agent com Go, Mastra e DMMF
- **Idioma**: Português (PT-BR)
- **Tecnologias**: Go (Golang), DMMF (Domain Modeling Made Functional), Mastra AI (Conceitos: Agent, Workflow, Tool, Thread, Run, WorkingMemory, Pending Step)
- **Regras Mandatórias**: `.agents/skills/go-implementation`, `AGENTS.md`, `.claude/rules/agent-workflows-tools.md`, `.claude/rules/governance.md`
- **Objetivo**: Refatorar o módulo `internal/agent` removendo lógica de acoplamento do orchestrador, migrando fluxos para o padrão Workflow + Tool, garantindo persistência robusta, concorrência segura, e cobertura de testes.
- **Status**: Production-ready, com critérios estritos de aceitação e gates de verificação.

---

# INSTRUÇÕES DE EXECUÇÃO: REFATORAÇÃO DO MÓDULO internal/agent

## 1. Objetivo Principal
Você deve refatorar o módulo `internal/agent` do projeto `mecontrola`. O objetivo é eliminar o switch-case gigante e centralizado de domínio do `DailyLedgerAgent` e migrar a execução das intenções para o padrão canônico **Workflow + Tool** inspirado pelo framework **Mastra** (https://github.com/mastra-ai/mastra), mantendo o código 100% idiomático em Go. Toda a modelagem de domínio deve seguir os princípios de **Domain Modeling Made Functional (DMMF)** e as regras estritas da skill de **Go-Implementation** (`.agents/skills/go-implementation`).

---

## 2. Contexto e Requisitos Inegociáveis (Hard Constraints)

### 2.1. Arquitetura Canônica de Roteamento (R-AGENT-WF-001.1)
O fluxo de execução do agente deve ser estritamente linear e baseado em roteamento de intenções:
```
IntentRouter -> AgentRuntime.Execute (Thread-first -> Run auditável) -> WorkflowRegistry.Resolve(kind) -> Workflow.Execute -> Tool.Execute -> binding -> usecase -> domain -> repo
```
- **Proibido**: Adicionar novos `case` de domínio ou lógica de negócio ao `switch` de `daily_ledger_agent.go`. O arquivo do agente deve permanecer fino, orquestrando apenas o registry, executando guardas transversais e formatando respostas de saída.
- **Proibido**: Lógica de roteamento por intent kind fora de um `Workflow`. Toda nova capacidade deve ser adicionada como um par `Workflow`/`Tool` no registry.

### 2.2. Padrão Tool como Adapter Fino (R-AGENT-WF-001.2)
Cada `Tool` deve ter responsabilidade única e atuar como um adapter fino sobre `binding -> usecase`.
- **Proibido** em qualquer `Tool` (`internal/agent/application/tools/`) ou `Workflow` (`internal/agent/application/workflow/`):
  1. Lógica ou regras de negócio (ex: tomar decisão de janela, normalizar valores, calcular limites).
  2. Consultas SQL diretas (`QueryContext`, `ExecContext`, `db.Exec`, etc.).
  3. Desvios (branching) baseados em estado de domínio.
- **Permitido**: Mapear `intent.Intent` para o DTO/command do usecase correspondente, invocar o usecase/binding, mapear a resposta para `ToolResult` e retornar.
- A lógica de pré-escrita (autorização, replay de idempotência/segurança, políticas e auditoria) deve viver centralizada no step de guarda reutilizável (`write_guard.go`), compartilhado por todos os workflows de escrita, sem duplicação nas ferramentas individuais.

### 2.3. Domain Modeling Made Functional (DMMF) & State-as-Type (R-AGENT-WF-001.3)
Toda definição de estado ou resultado deve usar tipos fechados enumerados, nunca strings livres:
- `RunStatus` aceita exclusivamente: `running | succeeded | failed`.
- `ToolOutcome` aceita exclusivamente o conjunto fechado de resultados (ex: `routed`, `clarify`, `usecaseError`, `missingResolver`).
- `AwaitingKind` aceita exclusivamente: `category_confirm | category_choice`.
- `TransactionKind` aceita exclusivamente: `expense | income | card_purchase`.
- **Proibido**: Representar esses estados como strings puras nas assinaturas ou lógica de ramificação. Use smart constructors para criar e validar as transições dessas structs de estado.
- **Regras DMMF de Domínio**:
  1. **Smart constructors**: Value Objects e commands devem expor apenas construtores que retornam `(T, error)`, mantendo campos privados para garantir que nenhum estado inválido seja instanciado (zero value inválido por construção).
  2. **Decide* puro**: Toda regra de negócio complexa de decisão deve viver em funções puras `Decide*` (sem I/O, sem `context.Context`, deterministicas, recebendo dependências e data/hora como parâmetros).
  3. **Workflow pipeline**: O fluxo deve ser linear (`parse → validate → decide → persist → publish`), passando tipos ricos e estruturados de uma etapa para outra.
  4. **Discriminated union via errors.As**: Tratar divergências de fluxo (ex: categoria ambígua, necessidade de confirmação) via `errors.As(err, &typed)` ou `errors.Is`. Nunca fazer `switch` em campos string.

### 2.4. Ciclo de Execução: Thread-First & Run Auditável (R-AGENT-WF-001.5 / R-AGENT-WF-001.6)
Toda chamada a `AgentRuntime.Execute` deve seguir o ciclo de vida:
1. **Thread-first**: Resolver ou criar uma `Thread` baseada em `(user_id, channel)` usando `ThreadGateway.GetOrCreate(userID, channel)`.
   - Identidade canônica: `resourceId = user_id`, `threadId = channel`.
2. **Run auditável**: Abrir e persistir a execução como um `Run` contendo: `thread_id`, `run_id`, `workflow`, `tool`, `status` (`RunStatus`), `duration_ms` e `error`.
   - Em operações de escrita, deve referenciar o `decision_id` correspondente para a trilha de auditoria e replay de segurança.
   - Métricas: Apenas enums fechados (`agent_id`, `channel`, `workflow`, `status`, `tool`, `outcome`) são permitidos como labels de métricas. Proibido usar identificadores de usuário ou de categoria como labels.

### 2.5. Limitações e Controles de Inteligência Artificial
- **LLM apenas no step de Parse (R-AGENT-WF-001.4)**: O LLM só pode atuar no parser inicial (`ParseInbound`). Workflows e tools executam lógica puramente determinística baseada no `intent.Intent` estruturado.
- **WorkingMemory no System Prompt (R-AGENT-WF-001.8)**: O `ContextBuilder` deve ler a `WorkingMemory` do usuário (em markdown estruturado) pelo `user_id` e acoplá-la ao system prompt se disponível.
- **Drafts em Clarificações (R-AGENT-WF-001.7)**: Quando um erro de categoria ambígua ou confirmação for retornado, o sistema deve salvar um `pendingexpense.Draft` (com `AwaitingKind` fechado) no banco de dados e retornar `OutcomeClarify`. A retomada (`resume`) deve interceptar o fluxo antes do parse do LLM.

---

## 3. Padrões de Implementação Go (R0-R7)

Aplique com rigor as regras estritas da skill `go-implementation`:
- **R0 (init proibido)**: Nenhuma função `init()` é permitida em qualquer arquivo de produção.
- **R1 (Struct Methods)**: Funções de domínio/aplicação/infraestrutura devem ser métodos de structs. Funções puras são reservadas para factories/construtores de instâncias e helpers de teste.
- **Zero comentários em código Go**: Proibido adicionar comentários explicativos em Go. Apenas comentários de geração de código (ex: mocks) ou diretivas `//go:` e `//nolint:` (com justificativa na mesma linha) são permitidos.
- **Contratos e Context**: Toda fronteira de I/O deve receber `context.Context` como primeiro parâmetro. Injeção de Dependência (DI) deve ser feita exclusivamente via construtores manuais explícitos em `module.go`.
- **Proibido `var _ Interface = (*Type)(nil)`**: Não use este padrão de asserção estática.
- **Sem `clock.Clock`**: Não use mocks ou abstrações compartilhadas de clock no domínio ou usecases. Use `time.Now().UTC()` ou passe instantes em commands.
- **Tratamento de erros**: Trate erros uma única vez. Use `errors.New`, `%w` com `fmt.Errorf` para contexto ou `errors.Join` para agregação.

---

## 4. Estrutura do Pacote internal/agent

A organização do código deve respeitar a estrutura de pastas obrigatória por módulo:
```text
internal/agent/
  application/
    dtos/input/     # Estruturas de entrada para use cases
    dtos/output/    # Estruturas de saída dos use cases
    usecases/       # Casos de uso orquestrando a lógica do agente (ex: parse, runtime, etc.)
    interfaces/     # Contratos consumidos pela aplicação (gateways de persistência, etc.)
    services/       # DailyLedgerAgent, AgentRuntime, etc.
    tools/          # Implementações concretas de tools.Tool (adapters finos)
    workflow/       # Workflows e composite.go
  domain/
    entities/       # Thread, Run, WorkingMemory
    valueobjects/   # Tipos enumerados e Value Objects (RunStatus, ToolOutcome, etc.)
    pendingexpense/ # Draft e estruturas de clarificação
    services/       # Serviços puros do domínio do agente
    interfaces/     # Contratos específicos do domínio
  infrastructure/
    providers/      # Provedores de LLM externos (ex: openrouter)
    repositories/   # Implementações de banco dos gateways de Thread, Run, WorkingMemory, etc.
  module.go         # Bootstrap, fiação manual de DI
```

---

## 5. Roteiro Passo a Passo de Execução

### Fase 1: Exploração e Validação de Baseline
1. Mapeie todas as dependências em `internal/agent/module.go` e analise como o `DailyLedgerAgent` é instanciado.
2. Identifique todos os métodos `route*` em `daily_ledger_agent.go` e documente os use cases/bindings que cada um invoca.
3. Certifique-se de que os gateways e repositórios necessários (`ThreadRepository`, `RunRepository`, `WorkingMemoryRepository`) estão corretamente declarados no banco de dados e expostos através da factory.

### Fase 2: Definição dos Tipos DMMF e Modelos do Domínio
1. Ajuste ou crie as constantes e tipos fechados sob `internal/agent/domain/valueobjects/` (`RunStatus`, `ToolOutcome`, etc.), assegurando que não existam strings livres nas assinaturas.
2. Crie ou ajuste os smart constructors para garantir que as structs de domínio tenham suas invariantes mantidas (ex: validações de tamanho, limites de confiança).
3. Modele os fluxos de clarificação usando `pendingexpense.Draft` com os enums fechados `AwaitingKind` e `TransactionKind`.

### Fase 3: Implementação dos Workflows e Tools
1. Crie o `WorkflowRegistry` capaz de mapear intents para instâncias de `Workflow`.
2. Migre cada comando de roteamento existente para o respectivo `Workflow` (`transactions`, `budget`, `cards`, `conversational`).
3. Refatore cada ação individual do agente para se tornar uma `Tool` (adapter fino sob `internal/agent/application/tools/`).
4. Desenvolva o mecanismo transversal de `write_guard.go` para ser executado como um step antes de qualquer escrita no domínio (validando autorização, decisão de auditoria e replay de idempotência).
5. Limpe o `daily_ledger_agent.go` de forma a remover todos os métodos `route*` obsoletos, fazendo com que ele apenas resolva o workflow correspondente e formate a saída.

### Fase 4: Thread, Run e Working Memory
1. Garanta que o `AgentRuntime` resolva a `Thread` a partir de `(userID, channel)` antes de rodar o workflow.
2. Certifique-se de que o `AgentRuntime` abra um `Run` no banco ao iniciar a execução, compute o tempo gasto e grave o resultado final (`succeeded` com output ou `failed` com mensagem de erro).
3. Adapte o `ContextBuilder` para injetar a `WorkingMemory` no system prompt se ela estiver presente para o usuário no banco de dados.
4. Garanta que se o workflow retornar um erro de categoria ambígua ou pendente, um `Draft` seja salvo e a resposta formatada para o usuário.

### Fase 5: Fiação (Wiring) e Testes
1. Ajuste o fiação manual de DI em `internal/agent/module.go` para registrar os novos workflows, tools, registry e runtime.
2. Crie ou atualize os testes unitários garantindo isolamento total por componente.
3. Adicione testes de integração focados no ciclo completo (Thread-first, Run auditável e execução determinística de workflows).

---

## 6. Portões de Validação (Verification Gates)

Você deve validar a refatoração localmente executando os comandos a seguir no terminal do workspace:

1. **Validação de Compilação e Erros Básicos**:
   ```bash
   go build ./internal/agent/...
   go vet ./internal/agent/...
   ```

2. **Execução de Testes Unitários com Race Detector**:
   ```bash
   go test -race -count=1 ./internal/agent/...
   ```

3. **Verificação de Regras Estritas de Roteamento (Gate 1)**:
   Garanta que nenhum switch-case com comportamento de domínio sobrou no orchestrador:
   ```bash
   f=$(find internal/agent -name "daily_ledger_agent.go" ! -name "*_test.go")
   if [ ! -z "$f" ]; then
     cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
     if [ "${cases:-0}" -gt 1 ]; then
       echo "FAIL: O switch de domínio permaneceu em daily_ledger_agent.go (cases=$cases). Use o WorkflowRegistry!"
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
   Garanta que nenhuma tool ou workflow interage diretamente com o banco de dados via SQL:
   ```bash
   grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
     "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
     internal/agent/application/tools/ \
     internal/agent/application/workflow/ 2>/dev/null \
     | grep -Ev "(//go:|//nolint:|// Code generated)" \
     && { echo "FAIL: Lógica de SQL direto encontrada em tool ou workflow!"; exit 1; } || true
   ```

---

## 7. Critérios de Aceitação (Definition of Done)

A entrega da refatoração será considerada concluída quando as seguintes evidências e condições forem atendidas:
1. **Compilação e Sucesso nos Testes**: O build do módulo e todos os testes unitários/integração do pacote `internal/agent` passam sem race condition.
2. **Padrão Workflow + Tool Operando**: Toda execução de intenções passa obrigatoriamente pela cadeia `IntentRouter -> WorkflowRegistry -> Workflow -> Tool`.
3. **Mastra Engine Reproduzida**: Existência das structs e tabelas/repositórios operantes de `Thread` (identidade canônica) e `Run` auditável (persistência correta do tempo de execução e status).
4. **Resolução de Clarificações e Drafts**: Mecanismo de resume operando com base no `pendingexpense.Draft` em banco para categoria ambígua ou pendente de confirmação.
5. **Adesão Estrita a Go e DMMF**: Código limpo, sem `init()`, sem panic, sem comentários no código Go, e utilizando tipos fechados enumerados para estados e outcomes.
6. **Relatório de Refatoração**: Um arquivo em markdown registrando os arquivos alterados, adicionados ou removidos e comprovando que todos os portões de validação acima passaram com sucesso.
