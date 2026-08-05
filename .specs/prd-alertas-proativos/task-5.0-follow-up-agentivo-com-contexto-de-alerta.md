# Tarefa 5.0: Follow-up agentivo com contexto de alerta

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Permitir que respostas como "sim" sejam resolvidas a partir de contexto recente de alerta, usando o runtime agentivo existente e sem recriar primitivos Mastra.

<requirements>
- Cobrir RF-14 e REQ-15.
- Contexto expirado deve pedir esclarecimento.
- Thread/Run/WorkingMemory/MessageStore existentes devem ser consumidos, não recriados.
- Follow-up deve acionar ferramenta existente coerente com o alerta.
</requirements>

## Subtarefas

- [ ] 5.1 Mapear inbound WhatsApp e `AgentRuntime`.
- [ ] 5.2 Definir contexto mínimo de alerta recente e expiração.
- [ ] 5.3 Integrar contexto ao prompt/memória sem alto acoplamento.
- [ ] 5.4 Cobrir "sim" para categoria e orçamento.
- [ ] 5.5 Cobrir contexto expirado sem inferência falsa.

## Detalhes de Implementação

Referenciar `techspec.md` seções `Arquitetura do Sistema`, `Pontos de Integração` e `Conformidade com Padrões`.

## Critérios de Sucesso

- O agente não inventa intenção sem contexto.
- Follow-up usa ferramentas existentes.
- Kernel de workflow permanece genérico.
- Não há chamada LLM fora das call-sites sancionadas.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alteração consome runtime agentivo, memória e ferramentas do MeControla.
- `domain-modeling-production` — modela contexto de alerta e expiração como estado válido.
- `design-patterns-mandatory` — valida ausência de roteamento por switch/pattern indevido.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/agents/...`
- [ ] `go test -race -count=1 ./internal/platform/agent/... ./internal/platform/memory/...`
- [ ] Gates Mastra de `rules-checklist.md`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/mecontrola_agent.go`
- `internal/agents/application/tools`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/platform/agent`
- `internal/platform/memory`
