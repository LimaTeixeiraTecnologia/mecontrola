# Tarefa 6.0: PendingEntryContinuer, Consumer, Wiring e Métricas

<critical>Ler prd.md, techspec.md e scenarios.md desta pasta — tarefa invalidada se pulado</critical>

## Visão Geral

Implementar `PendingEntryContinuer` como use case que carrega/retoma a pendência antes do agente aberto, integrar ao `WhatsAppInboundConsumer` na ordem correta, montar `workflow.Engine[PendingEntryState]`, registrar o reaper de pendências e emitir métricas `agents_pending_entry_*` com labels de baixa cardinalidade em `module.go`.

<requirements>
- PendingEntryContinuer.Continue(ctx, userID, peer, message, messageID) (PendingEntryResult, error): verifica pendência ativa; retoma via Engine.Resume; retorna Handled=true quando responde; Handled=false quando replaced (nova frase segue para agente)
- Ordem de resolução no consumer (RF-03): 1. PendingEntryContinuer → 2. DestructiveConfirmContinuer → 3. ResolveOnboardingOrAgent → 4. HandleInbound
- Sem inversão de ordem: pendência financeira tem prioridade máxima sobre onboarding e agente aberto
- module.go: montar Engine[PendingEntryState], PendingEntryContinuer, reaper (staleAfter=35min), injetar no consumer
- Métricas com labels de baixa cardinalidade (R-TXN-004, R-AGENT-WF-001.5):
    agents_pending_entry_total{outcome} — outcomes: started|resumed|completed|cancelled|expired|replaced|error
    agents_pending_entry_slot_total{slot,outcome} — slots fechados (category|payment_method|card|date|confirmation|correction); o slot terminal universal antes de toda escrita é slot="confirmation" (RF-38)
    agents_pending_entry_write_total{outcome} — success|replay|error|blocked
    agents_pending_entry_duration_seconds{outcome}
- Proibido label: user_id, thread_id, resource_id, category_id, subcategory_id, message_id
- Zero comentários Go de produção
</requirements>

## Subtarefas

- [ ] 6.1 Criar `pending_entry_continuer.go` em `internal/agents/application/usecases/`: `Continue` verifica `Engine.Resume("resourceID:threadID:pending-entry", patch)` — se run não existir, retorna `Handled=false` imediatamente; se existir, processa e retorna resultado
- [ ] 6.2 Atualizar `whatsapp_inbound_consumer.go`: inserir chamada a `PendingEntryContinuer.Continue` antes de `DestructiveConfirmContinuer`; se `Handled=true`, encerrar processamento; se `Handled=false` (replaced ou sem pendência), prosseguir para próxima etapa
- [ ] 6.3 Atualizar `module.go`: instanciar `workflow.Engine[PendingEntryState]` com store Postgres; instanciar `PendingEntryContinuer`; registrar reaper `workflow.NewStaleSuspendedReaper("pending-entry", 35*time.Minute)`; injetar continuer no consumer
- [ ] 6.4 Implementar métricas com Prometheus: `agents_pending_entry_total`, `agents_pending_entry_slot_total`, `agents_pending_entry_write_total`, `agents_pending_entry_duration_seconds` — emitir no `PendingEntryContinuer` e no step de write
- [ ] 6.5 Testes unitários do consumer: ordem de resolução pending→destructive→onboarding→agent; Handled=true encerra; Handled=false passa adiante; replaced passa para agente

## Detalhes de Implementação

Ver `techspec.md` seções **"Ordem de Resolução no Consumer"**, **"Correlação de Pendência"** e **"Monitoramento e Observabilidade"**.

Resultado do continuer:

```go
type PendingEntryResult struct {
    Handled bool
    Message string
    Mode    PendingEntryMode  // replied|passThrough|completed|cancelled|expired|replaced
}
```

`Mode=replaced` → `Handled=false` → consumer deixa mensagem seguir para `HandleInbound` como nova operação.

Key de correlação: `<resourceID>:<threadID>:pending-entry` — construída a partir de `InboundExecutionFromContext` (de 5.0). Zero uso como label de métrica.

Reaper: `workflow.NewStaleSuspendedReaper("pending-entry", 35*time.Minute)` registrado no job scheduler existente em `module.go` — não criar job separado; usar padrão já estabelecido por outros reapers no módulo.

Métricas (cardinalidade controlada):
- `outcome`: enum fechado de `PendingEntryMode` → string mapeado; nunca usar valor livre
- `slot`: enum fechado de `AwaitingSlot.String()` → nunca usar texto livre do usuário como label
- Gate: `grep -rn '"user_id"\|"thread_id"\|"resource_id"\|"category_id"' internal/agents/` deve retornar vazio

Integração com `DestructiveConfirmContinuer`: a ordem garante que pendências de registro financeiro não sejam tratadas como confirmações destrutivas e vice-versa. Verificar que consumer atual tem `DestructiveConfirmContinuer` antes de `ResolveOnboardingOrAgent` e inserir `PendingEntryContinuer` ANTES desse.

## Critérios de Sucesso

- `go build ./internal/agents/...` passa (wiring completo)
- `go test -race -count=1 ./internal/agents/...` verde
- Ordem de resolução correta: pendência → destructive → onboarding → agent (6.5)
- `Handled=true`: consumer encerra sem chamar etapas posteriores
- `Handled=false` com `Mode=replaced`: consumer chama `HandleInbound` com a mesma mensagem (G7-01, CA-02)
- Reaper registrado: `grep "NewStaleSuspendedReaper" internal/agents/module.go` retorna match
- Gate de métrica: `grep -rn '"user_id"\|"thread_id"\|"category_id"' internal/agents/` retorna vazio
- Gate zero comentários passa em `internal/agents/`

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — PendingEntryContinuer é o use case de retomada do consumidor internal/agents; wiring em module.go monta o substrato agent da plataforma para o workflow pending-entry

## Testes da Tarefa

- [ ] `whatsapp_inbound_consumer_test.go`: ordem de resolução pending→destructive→onboarding→agent; Handled=true encerra; replaced passa adiante; sem pendência → prossegue normal
- [ ] `pending_entry_continuer_test.go`: run não existe → Handled=false imediato; run existe + slot respondido → Handled=true; run replaced → Handled=false com Mode=replaced
- [ ] Métricas: verificar que `agents_pending_entry_total` é emitido com outcome correto; gate de cardinalidade (sem labels proibidos)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/usecases/pending_entry_continuer.go` (novo)
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (atualizar)
- `internal/agents/module.go` (atualizar: Engine, continuer, reaper, injeção)
- `internal/agents/application/usecases/destructive_confirm_continuer.go` (referência de padrão existente)
- `internal/platform/workflow/engine.go` (consumido)
- `internal/platform/workflow/infrastructure/postgres/store.go` (consumido)
- `.specs/prd-conversa-agentiva-fluida/techspec.md` (seções "Ordem de Resolução", "Correlação", "Monitoramento")
- `.specs/prd-conversa-agentiva-fluida/scenarios.md` (G7-01 replaced, G7-02 texto compatível com pendência antiga)
