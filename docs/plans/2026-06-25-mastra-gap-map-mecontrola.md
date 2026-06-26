# Plano recomendado — aproximação do mecontrola ao modelo de experiência inspirado no Mastra

Data: 2026-06-25

## Objetivo

Aproximar o `mecontrola` do que o Mastra entrega de melhor como experiência de produto para agentes, sem copiar o framework nem abandonar as restrições arquiteturais do projeto.

O foco não é reproduzir APIs TypeScript do Mastra. O foco é evoluir o `mecontrola` em:
- introspecção
- operação
- segurança de evolução
- ergonomia para extensão
- memória
- avaliação semântica
- composição futura de capabilities/agentes

## Diagnóstico executivo

O `mecontrola` já está forte no núcleo de runtime:
- `Thread` e `Run` auditáveis
- workflow durável com suspend/resume
- tools como adapters finos
- gates de confirmação humana
- working memory básica
- canal WhatsApp com deduplicação, rate limit e establishment de principal

Os gaps principais não estão no core de execução. Estão na camada de experiência operacional e de evolução:
- ausência de console/studio
- ausência de evals semânticos first-class
- registry sem catálogo introspectável rico
- skill `mastra` em drift em relação ao código real
- mapeamentos manuais no runtime com risco de auditoria/métricas incorretas
- memória ainda simples
- falta de workspaces/multi-agent explícitos
- falta de primitive uniforme para ferramentas/capabilities externas

## Matriz prática de gaps

| Gap | Evidência no código | Risco atual | Ação recomendada | Esforço | Ganho no mecontrola |
|---|---|---|---|---|---|
| Console operacional do agent | Há `Run`, `Thread`, workflow kernel e confirmação, mas sem superfície unificada de inspeção | Debug lento, baixa visibilidade de suspensões, difícil operar incidentes | Criar UI/console para listar threads, runs, status, tool, workflow, pending states, replay, fallback e resume manual | alto | muito alto |
| Evals semânticos first-class | Há muitos testes de código, mas não uma camada explícita de eval do agent | Regressão silenciosa de roteamento, clarificação e resposta | Criar datasets versionados e runner de eval para parse, routing, clarify, destructive confirm, by-ref, fallback e memory | médio | muito alto |
| Catálogo introspectável de capabilities | O registry executa, mas não parece expor metadados ricos | DX fraca, docs implícitas, difícil alimentar UI/policy | Formalizar `ToolSpec` e `WorkflowSpec` ricos: descrição, side effects, confirmation required, resumable, channel support, owner, metrics key | médio | alto |
| Skill `mastra` em drift | A skill ainda descreve o seam principal como se fosse apenas `buildRegistry()` | Implementação nova pode sair errada mesmo seguindo a skill | Atualizar a skill para refletir o modelo real: registry + kernel + confirm + plan + resumes | baixo | alto |
| Mapeamento de auditoria e métricas incompleto | O runtime usa tabela manual de workflow/tool por kind | Métricas e auditoria incorretas, leitura errada de produção | Fazer o runtime derivar workflow/tool do registry ou de metadata canônica, não de tabela manual | baixo | alto |
| Memory ainda simples | `WorkingMemory` é markdown persistido e injetado no prompt | Baixa precisão contextual, crescimento desordenado e política implícita | Separar memória de perfil, memória por thread, fatos derivados e observações; definir TTL/refresh/curation | médio | alto |
| Multi-agent/workspaces | Forte centralização no `DailyLedgerAgent` | Escala funcional tende a concentrar complexidade em um único agente | Introduzir workspaces ou agentes explícitos: onboarding, ledger, budget, cards, support/recovery | alto | médio/alto |
| Runtime de ferramentas externas estilo MCP | Não identifiquei primitive equivalente no código auditado | Integrações novas exigem wiring específico demais | Definir contrato de external capability/tool provider com policy, timeout, audit e retry padronizados | médio | médio |
| Observability semântica de IA | Há métricas, mas faltam visões de produto do agent | Difícil otimizar custo, qualidade e UX | Adicionar dashboards por parse confidence, clarify rate, confirm rate, resume success, fallback reason, provider/model, cost | médio | alto |
| Canal como primitive | WhatsApp está robusto, mas ainda como adapter | Crescer para novos canais exige duplicação de semântica | Formalizar `ChannelCapability` com constraints de UX, resume behavior, formatting e rate policy | médio | médio |

## Sequência recomendada

### Fase 1 — corrigir a base de verdade

Objetivo:
eliminar drift entre governança, runtime e observabilidade antes de expandir capabilities.

Entregas:
- corrigir o mapeamento manual de `workflow/tool` no runtime
- fazer o runtime derivar `workflow/tool` de metadata canônica
- atualizar a skill `mastra` para refletir o desenho real
- revisar a documentação canônica do padrão do agent para incluir:
  - registry
  - kernel write path
  - confirmation engine
  - plan executor
  - cadeia de resumes
- fechar checklist de extensão com passos objetivos para:
  - novo kind
  - nova tool
  - novo workflow
  - novo pending state
  - novo gate de confirmação
  - novo plan step

#### Antes

O runtime mantém classificação manual por `intent.Kind`:

```go
func workflowFor(kind intent.Kind) string {
	switch kind {
	case intent.KindRecordExpense,
		intent.KindRecordIncome,
		intent.KindRecordCardPurchase:
		return workflowTransactions
	case intent.KindMonthlySummary,
		intent.KindHowAmIDoing:
		return workflowBudget
	default:
		return workflowConversational
	}
}
```

Problemas:
- kinds novos podem ficar fora do mapeamento
- auditoria e métricas podem mentir
- registry e runtime viram duas fontes de verdade
- reviews passam mesmo com classificação errada se o teste não cobrir aquele kind

#### Depois

O runtime passa a derivar a classificação de metadata canônica do próprio catálogo:

```go
type CapabilityMeta struct {
	Kind                 intent.Kind
	WorkflowID           string
	ToolName             string
	IsWrite              bool
	RequiresConfirmation bool
	SupportsResume       bool
}

type CapabilityCatalog interface {
	Lookup(kind intent.Kind) (CapabilityMeta, bool)
}
```

```go
func (rt *AgentRuntime) classify(kind intent.Kind) (string, string) {
	meta, ok := rt.catalog.Lookup(kind)
	if !ok {
		return workflowConversational, ""
	}
	return meta.WorkflowID, meta.ToolName
}
```

Ganhos:
- elimina drift entre execução e observabilidade
- kinds novos passam a herdar auditoria correta automaticamente
- reduz custo de manutenção
- prepara terreno para console e evals

#### Antes

A skill `mastra` diz que `buildRegistry()` é o único seam.

Problema:
- o código real já exige entender também:
  - kernel de escrita
  - confirmation engine
  - plan executor
  - order de resumes

#### Depois

A skill passa a declarar explicitamente os seams reais de evolução:

```text
1. Registry seam: read tools e write tools simples
2. Kernel seam: writes duráveis com suspend/resume
3. Confirm seam: destructive/sensitive operations
4. Plan seam: multi-step execution
5. Resume chain seam: pending category -> pending plan -> pending approval
```

Ganhos:
- reduz falso positivo de governança
- evita implementação no lugar errado
- melhora onboarding técnico do time

Critérios de aceite:
- nenhum `intent.Kind` suportado fica fora do mapeamento de auditoria/métricas
- a skill `mastra` deixa de afirmar que `buildRegistry()` é o único seam
- a documentação de extensão do agent fica consistente com o código atual
- testes do agent e workflow continuam verdes

### Fase 2 — capability catalog introspectável

Objetivo:
transformar o registry de execução em uma fonte de verdade útil também para UI, docs, policy e testes.

Entregas:
- formalizar metadata rica para tools e workflows
- cada capability deve expor no mínimo:
  - id
  - descrição
  - `intent.Kind`
  - workflow owner
  - leitura/escrita
  - requer confirmação humana
  - pode suspender
  - suporta resume
  - canal suportado
  - labels de métricas
- criar um ponto de consulta programática para listar capabilities
- usar esse catálogo como base para auditoria, console e testes de cobertura

#### Antes

O registry é muito bom para executar, mas fraco para introspecção:

```go
transactionsWorkflow, err := agentwf.NewIntentWorkflow("transactions",
	agentwf.KindTool{Kind: intent.KindRecordExpense, Tool: tools.NewRecordExpense(...)},
	agentwf.KindTool{Kind: intent.KindRecordIncome, Tool: tools.NewRecordIncome(...)},
)
```

Problemas:
- descrição da capability não está centralizada
- UI futura não sabe listar capacidades úteis
- policy e métricas dependem de conhecimento espalhado
- testes de cobertura por capability ficam manuais

#### Depois

Introduzir metadata rica junto ao binding:

```go
type CapabilitySpec struct {
	ID                   string
	Description          string
	Kind                 intent.Kind
	WorkflowID           string
	ToolName             string
	Mode                 CapabilityMode
	RequiresConfirmation bool
	SupportsSuspend      bool
	SupportsResume       bool
	Channels             []string
	MetricsKey           string
}
```

```go
type RegisteredCapability struct {
	Spec CapabilitySpec
	Tool tools.Tool
}
```

```go
catalog.Register(RegisteredCapability{
	Spec: CapabilitySpec{
		ID:                   "transaction.record_expense",
		Description:          "Registra despesa",
		Kind:                 intent.KindRecordExpense,
		WorkflowID:           "transactions",
		ToolName:             "record_expense",
		Mode:                 CapabilityWrite,
		RequiresConfirmation: false,
		SupportsSuspend:      true,
		SupportsResume:       true,
		Channels:             []string{"whatsapp"},
		MetricsKey:           "record_expense",
	},
	Tool: tools.NewRecordExpense(...),
})
```

Ganhos:
- uma fonte única de verdade
- base direta para console/studio
- base direta para métricas e evals
- melhor DX para adicionar capability nova

Critérios de aceite:
- é possível listar programaticamente todas as capabilities do agent
- não existe mais duplicação relevante entre registry, runtime mapping e docs
- cada capability tem classificação operacional mínima definida

### Fase 3 — console operacional do agent

Objetivo:
dar ao time uma experiência próxima de “studio” para operar o agent em produção e em homologação.

Entregas:
- tela ou console para listar:
  - threads
  - runs
  - workflow
  - tool
  - outcome
  - status
  - duration
  - fallback
  - replay
  - suspensões
- visualização de estado pendente:
  - clarificação de categoria
  - confirmação destrutiva
  - seleção by-ref
  - plano suspenso
- ação controlada de inspeção/resume manual para ambientes autorizados
- filtros por usuário, canal, período, workflow, outcome e status
- links ou correlação com traces/logs existentes

#### Antes

Diagnóstico depende de:
- log
- trace
- query manual em banco
- conhecimento tácito do fluxo de resume

Cenário típico ruim:
1. usuário responde “sim”
2. operação não acontece
3. time precisa procurar:
   - se havia pending approval
   - se houve timeout
   - se caiu em replay
   - se o state expirou
   - se o resume foi para a capability errada

#### Depois

Console exibe uma timeline operacional:

```text
Thread: user=123 channel=whatsapp
Run A: parse -> workflow=transactions -> outcome=clarify
Run B: resume confirm -> workflow=transactions_confirm -> suspended
Run C: resume confirm -> workflow=transactions_confirm -> succeeded
```

Visão detalhada por run:
- input recebido
- intent resolvida
- workflow/tool
- state suspenso
- prompt emitido
- resposta do usuário no resume
- outcome final
- tempo total

Ganhos:
- incidentes ficam tratáveis sem SQL manual
- reduz MTTR
- acelera validação de produção e homologação
- melhora confiança para expandir capabilities

Critérios de aceite:
- um incidente de roteamento ou resume pode ser inspecionado sem consultar diretamente banco e logs brutos
- o time consegue identificar rapidamente:
  - onde suspendeu
  - qual capability executou
  - qual resposta foi devolvida
  - se houve replay, fallback ou timeout

### Fase 4 — evals semânticos first-class

Objetivo:
reduzir regressão de comportamento de IA e de routing ao evoluir prompts, tools e workflows.

Entregas:
- dataset versionado com casos reais e sintéticos
- suíte mínima cobrindo:
  - parse de intents
  - confidence gating
  - fallback
  - clarify
  - destructive confirm
  - by-ref select
  - budget conversation
  - working memory
  - multi-step plan
- runner com relatório por capability
- baseline comparável por branch
- thresholds mínimos para regressão aceitável

#### Antes

Validação costuma depender de:
- testes unitários
- alguns e2e
- testes manuais por mensagem no WhatsApp
- memória do time sobre o comportamento esperado

Problema:
- regressão de linguagem natural passa fácil
- alterações de prompt/policy são difíceis de comparar
- branch A e branch B não têm score semântico comparável

#### Depois

Estruturar dataset e runner:

```yaml
- id: clarify-category-001
  input: "gastei 50 no extra ontem"
  expected:
    outcome: clarify
    kind: record_expense
    awaiting: category_choice
```

```yaml
- id: confirm-delete-001
  input_sequence:
    - "apaga meu último lançamento"
    - "sim"
  expected:
    final_outcome: routed
    confirmation_used: true
```

Relatório esperado:

```text
parse accuracy: 97%
clarify correctness: 95%
confirm flows: 100%
fallback rate: 2%
resume success: 98%
```

Ganhos:
- segurança para evoluir prompt e parser
- regressões semânticas ficam visíveis no PR
- dá maturidade real de produto de IA

Critérios de aceite:
- PRs do agent conseguem mostrar impacto semântico antes de merge
- regressões de intent/routing deixam de depender só de testes manuais
- o time consegue comparar versões de prompt, parser e policy com evidência

### Fase 5 — memory 2.0

Objetivo:
evoluir de memória única em markdown para camadas explícitas de contexto.

Entregas:
- separar:
  - memória de perfil persistente do usuário
  - memória operacional por thread/canal
  - observações derivadas
  - fatos estáveis
- definir política de atualização, expiração e curadoria
- restringir o que vai para prompt principal
- manter trilha auditável de atualizações relevantes
- avaliar se parte dessa memória deve alimentar capabilities específicas, não só prompt global

#### Antes

Memória atual:

```go
type WorkingMemory struct {
	UserID    uuid.UUID
	Content   string
	UpdatedAt time.Time
}
```

Problemas:
- mistura fatos estáveis com contexto momentâneo
- crescimento sem estrutura
- difícil auditar o que deveria ou não estar no prompt
- pouco reuso por capability específica

#### Depois

Separar os tipos de memória:

```go
type UserProfileMemory struct {
	UserID    uuid.UUID
	Content   string
	UpdatedAt time.Time
}

type ThreadMemory struct {
	UserID    uuid.UUID
	Channel   string
	Content   string
	UpdatedAt time.Time
}

type DerivedFact struct {
	UserID    uuid.UUID
	Key       string
	Value     string
	Source    string
	UpdatedAt time.Time
}
```

Injeção no prompt deixa de ser “manda tudo” e vira seleção por finalidade:
- perfil estável no parser
- contexto thread-local no resume
- fatos derivados só quando a capability precisa

Ganhos:
- prompt mais limpo
- menor ruído
- melhor precisão
- mais controle sobre custo e contexto

Critérios de aceite:
- o system prompt recebe contexto mais preciso e menos ruidoso
- memória não cresce indefinidamente sem critério
- atualizações de memória têm regra explícita e observável

### Fase 6 — workspaces e composição futura

Objetivo:
desacoplar crescimento funcional do `DailyLedgerAgent` sem perder a semântica atual.

Entregas:
- definir fronteira para workspaces ou agentes especializados
- candidatos iniciais:
  - onboarding
  - ledger
  - budget
  - cards
  - recovery/support
- estabelecer contrato de handoff entre capabilities/workspaces
- manter `Thread/Run` e observabilidade coesos

#### Antes

Um único agente concentra decisões de:
- parse
- fallback
- writes
- confirms
- plans
- budget session
- cartões
- ledger

Problema:
- crescimento funcional aumenta carga cognitiva
- risco de acoplamento semântico
- cada nova capability disputa espaço no mesmo orquestrador mental

#### Depois

Composição explícita por workspace:

```text
AgentRuntime
  -> WorkspaceResolver
    -> LedgerWorkspace
    -> BudgetWorkspace
    -> CardsWorkspace
```

Ou por agent especializado mantendo `Thread/Run` comum:

```text
AgentRuntime
  -> CapabilityCatalog.Resolve(kind)
  -> OwnerAgent.Execute(kind)
```

Ganhos:
- evolução mais modular
- menos colisão semântica
- mais clareza de ownership

Critérios de aceite:
- novas capacidades não exigem concentrar todo o fluxo em um único orquestrador mental
- composição entre domínios continua auditável e previsível

### Fase 7 — runtime uniforme para capabilities externas

Objetivo:
ganhar extensibilidade semelhante ao ecossistema de tools/MCP sem acoplamento ad hoc.

Entregas:
- definir contrato para capability externa com:
  - timeout
  - retry
  - auth
  - audit
  - policy
  - métricas
- classificar capabilities por criticidade e side effect
- integrar ao capability catalog
- deixar pronto para futuras integrações externas sem reabrir regras de governança

#### Antes

Cada integração nova tende a nascer como wiring próprio:
- contrato local
- timeout local
- policy local
- métrica local
- tratamento local de erro

Problema:
- inconsistência
- maior chance de desvio arquitetural
- difícil operar e auditar

#### Depois

Contrato uniforme:

```go
type ExternalCapability interface {
	ID() string
	Descriptor() ExternalCapabilitySpec
	Execute(ctx context.Context, in CapabilityInput) (CapabilityOutput, error)
}
```

```go
type ExternalCapabilitySpec struct {
	ID          string
	Timeout     time.Duration
	RetryPolicy string
	AuthMode    string
	SideEffect  bool
	Audited     bool
}
```

Ganhos:
- integração externa entra no mesmo ciclo de governança
- observabilidade uniforme
- menor risco de wiring ad hoc

Critérios de aceite:
- nova integração externa não precisa inventar wiring próprio fora do padrão
- capabilities externas entram no mesmo ciclo de auditoria, policy e observabilidade

## Ordem de implementação recomendada

1. corrigir runtime mapping e atualizar skill `mastra`
2. criar capability catalog canônico
3. construir console operacional
4. implantar evals semânticos
5. evoluir memory
6. abrir workspaces/multi-agent
7. formalizar runtime para capabilities externas

## Justificativa da ordem

- os dois primeiros passos corrigem a fonte de verdade
- o console e os evals criam segurança operacional e segurança de evolução
- a memória evoluída passa a valer mais depois que observabilidade e avaliação existem
- multi-agent e capabilities externas só devem crescer sobre uma base estável e introspectável

## Ganhos consolidados esperados

### Ganhos técnicos
- menos drift entre código, skill, docs e métricas
- menos classificação manual sujeita a erro
- mais segurança para adicionar kinds/capabilities
- base melhor para observabilidade e auditoria

### Ganhos de produto
- incidentes mais fáceis de diagnosticar
- confiança maior em produção
- capacidade de evoluir comportamento conversacional sem “medo cego”
- melhor experiência para usuários em fluxos de confirmação e resume

### Ganhos de time
- onboarding técnico mais rápido
- reviews mais objetivos
- menos dependência de conhecimento tácito
- governança mais executável e menos declaratória

## Resultado esperado

Se esse plano for executado nessa ordem, o `mecontrola` ficará mais próximo do valor real do Mastra sem perder sua identidade:

- runtime forte e auditável
- experiência operacional muito melhor
- menor drift entre código, skill e documentação
- evolução mais segura de prompts, workflows e tools
- base concreta para workspaces e futuras integrações externas

## Não objetivos

Este plano não propõe:
- reescrever o agent em TypeScript
- copiar APIs do Mastra
- enfraquecer as regras de bounded context ou DDD do repositório
- mover semântica de domínio para o kernel genérico
- substituir WhatsApp por abstração genérica prematuramente
