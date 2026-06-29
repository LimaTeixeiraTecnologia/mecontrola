# Tarefa 6.0: Cutover — eliminação física de `internal/agent` + desligar onboarding conversacional

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Apagar fisicamente `internal/agent/**`, religar `cmd/server`/`cmd/worker` para `internal/agents`, simplificar o dispatcher WhatsApp para rota única (desligando o onboarding conversacional/ativação), e ajustar/remover e2e e config dependentes. Operação irreversível — só após a tarefa 5.0 estar wired e build/CI verdes.

<requirements>
- RF-23: remover 100% `internal/agent`; nenhuma referência (`grep internal/agent` ≠ platform vazio; `test -d internal/agent` falso).
- RF-24: desligar onboarding conversacional do WhatsApp (rota de ativação); ajustar/remover e2e de `internal/onboarding` que importam `internal/agent`.
- RF-25: religar `cmd/server` (server.go, whatsapp_wiring.go) e `cmd/worker` (worker.go) para `internal/agents` (rota, consumer, jobs).
- RF-26: migrar config `AGENT_*` → config do módulo `agents` (model ids, OpenRouter, embed model/dims); sem variável órfã.
- RF-27: manter migration 000003; sem dependência de runtime das tabelas `agent_*`.
- ADR-004.
</requirements>

## Subtarefas

- [ ] 6.1 Religar `cmd/server`/`cmd/worker` para construir e expor `internal/agents` (rota WhatsApp, consumer, jobs); remover `NewAgentModule`, EventHandlers e jobs de `internal/agent`.
- [ ] 6.2 Simplificar o dispatcher para rota única → `internal/agents`; remover `onboardingRoute`/ativação do caminho WhatsApp e o `agentbinding` no card-creator do onboarding.
- [ ] 6.3 Apagar `internal/agent/**` (físico). Ajustar/remover `internal/onboarding/e2e/*` que importam `internal/agent/application/services`.
- [ ] 6.4 Migrar `configs/config.go` (`AGENT_*` → config do módulo `agents`); atualizar `.env`/exemplos.
- [ ] 6.5 Rodar gates: `grep internal/agent` (≠ platform) vazio, `test -d internal/agent` falso, `go build ./...`, `go vet`, `go test`, gofmt.

## Detalhes de Implementação

Ver techspec.md §"Arquivos Relevantes e Dependentes", ADR-004. Executar a remoção em commit isolado para rollback via revert.

## Critérios de Sucesso

- `grep -rn "internal/agent\"" internal/ cmd/ test/ | grep -v internal/platform/agent` → vazio; `internal/agent/` não existe.
- `go build ./...`, `go vet`, `go test` (determinístico) verdes; gofmt limpo.
- WhatsApp inbound atendido só por `internal/agents`; "ATIVAR <token>" não mais roteado (decisão de produto).

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (suites existentes seguem verdes após rewire; e2e ajustados compilam/passam)
- [ ] Testes de integração (build/CI verdes; gate de ausência de `internal/agent`)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- Remover: `internal/agent/**`, `internal/onboarding/e2e/*` dependentes. Alterar: `cmd/server/server.go`, `cmd/server/whatsapp_wiring.go`, `cmd/worker/worker.go`, `internal/platform/whatsapp/dispatcher/dispatcher.go`, `configs/config.go`.
