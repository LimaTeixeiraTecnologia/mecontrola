# Tarefa 7.0: Wiring, Resume Chain e Instruções do Agente

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fecha o cadastro conversacional de cartão ligando os artefatos das tarefas 5.0 e 6.0 no composition root (`module.go`), inserindo `tryContinueCardCreate` na resume chain do `WhatsAppInboundConsumer` na ordem determinística correta, e atualizando as instruções do `mecontrola-agent` para descrever a capacidade `create_card` e proibir afirmação de cadastro sem tool call. Ver techspec.md, seções "Visão Geral dos Componentes", "Exclusão Mútua e Ordem de Resume (RF-18)" e "Guardrail Anti-Alucinação (RF-13)".

<requirements>
- RF-13: guardrail anti-alucinação — proibir afirmar "cadastrei/não consegui cadastrar" sem uma chamada `create_card`; relayar `ConfirmationPrompt` verbatim.
- RF-18: exclusão mútua via resume-consumes-message; ordem `pending_entry → destructive_confirm → card-create → onboarding → ParseInbound`.
- RF-19: tool `create_card` registrada em `buildFinancialTools` e no write tool set (`agent.WithWriteToolSet`).
- Depende das tarefas 5.0 (estado/decisão/workflow/execução idempotente) e 6.0 (tool + continuer + reaper). Confirmar que todas as deps existem antes do wiring — nada de placeholders.
- Zero comentários em Go de produção (R-ADAPTER-001.1); sem `switch case intent.Kind` (R-AGENT-WF-001.1) — roteamento via registry/tool set.
</requirements>

## Subtarefas

- [ ] 7.1 Em `module.go`: construir `workflow.NewEngine[workflows.CardCreateState](workflowStore, deps.O11y)`; instanciar `workflows.BuildCardCreateConfirmWorkflow(idemAdapter, cardManager)`; construir `usecases.NewCardCreateConfirmContinuer(cardCreateEngine, cardCreateDef, ...)`; registrar o reaper (`workflow.NewStaleSuspendedReaper(workflowStore, workflows.CardCreateConfirmWorkflowID, 15*time.Minute, 100, deps.O11y)` fiado em `jobhandlers.NewConfirmReaperJob` e adicionado a `Jobs`). Confirmar cada símbolo importado das tarefas 5.0/6.0 antes de referenciá-lo.
- [ ] 7.2 Em `module.go` `buildFinancialTools`: adicionar parâmetros do engine/def de card-create e registrar `agenttools.BuildCreateCardTool(cardCreateEngine, cardCreateDef, cardManager)` na lista de tools; incluir `"create_card"` em `agent.WithWriteToolSet(...)`.
- [ ] 7.3 Em `module.go`: registrar o continuer no consumidor via nova `consumers.ConsumerOption` (`WithCardCreateResolver`) na lista `consumerOpts`.
- [ ] 7.4 Em `whatsapp_inbound_consumer.go`: adicionar campo/option e método `tryContinueCardCreate`; inseri-lo em `Handle` na ordem `tryContinuePendingEntry → tryContinueDestructive → tryContinueCardCreate → tryResolveOnboarding → handleAgentInbound` (BEFORE `handleAgentInbound`).
- [ ] 7.5 Em `mecontrola_agent.go`: atualizar instruções para descrever `create_card` com slot-filling um-slot-por-vez, e proibir afirmar cadastro/falha sem tool call + relayar `ConfirmationPrompt`/`ClarifyPrompt` verbatim (RF-13). Editar com cuidado extremo — é contrato de comportamento.

## Detalhes de Implementação

Referenciar techspec.md. Pontos-chave:

- **Wiring (techspec "Visão Geral dos Componentes", "Sequenciamento de Desenvolvimento" passos 5–7):** o `IdempotentWriter` já existe como `idemAdapter` em `module.go`; `cardManager` e `workflowStore` já estão instanciados. O reaper segue o padrão de `confirmReaperJob`/`pendingEntryReaperJob` (`module.go:239-242`), com TTL de 15 min (`cardCreateConfirmTTL`).
- **Resume chain (techspec "Exclusão Mútua e Ordem de Resume (RF-18)"):** cada resume consome a mensagem quando há run suspenso; portanto o `card-create` só inicia (via tool no último passo) quando nenhum outro gate está suspenso → exclusão mútua natural. `tryContinueCardCreate` espelha `tryContinueDestructive` (`whatsapp_inbound_consumer.go:194-212`): guarda `nil`, métrica de erro `outcome="card_create_error"`, `sendReply` no `handled`.
- **Instruções (techspec "Guardrail Anti-Alucinação (RF-13)"):** a mensagem de confirmação e a final de sucesso/falha são texto determinístico do workflow/continuer, não do LLM. Alinhar com a REGRA ABSOLUTA ANTI-SIMULAÇÃO e a REGRA ABSOLUTA DE PENDÊNCIA CONVERSACIONAL já existentes; `create_card` entra como tool de escrita no catálogo e nas regras de confirmação.

## Critérios de Sucesso

- `internal/agents` compila (`build`), passa `vet` e `lint` sem novos achados.
- `create_card` aparece na lista de `buildFinancialTools` e em `agent.WithWriteToolSet`.
- `tryContinueCardCreate` executa antes de `ParseInbound`/`handleAgentInbound` e não inicia novo confirm quando outro gate está suspenso (RF-18).
- Instruções proíbem afirmar cadastro/falha sem tool call e mandam relayar o prompt verbatim (RF-13).
- Zero comentários em Go de produção; sem `switch case intent.Kind`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — wiring do composition root, resume chain e instruções do agente no padrão do substrato.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Escopo mínimo:
- Build + vet + lint de `internal/agents`.
- Teste de consumer (unit): `tryContinueCardCreate` roda antes de `ParseInbound`/`handleAgentInbound` e não inicia novo confirm quando outro gate (pending_entry/destructive) está suspenso — preserva exclusão mútua RF-18.
- Verificação de que `create_card` está registrada no write tool set (RF-19).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/agents/application/agents/mecontrola_agent.go`
