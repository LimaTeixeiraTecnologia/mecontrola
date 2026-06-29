# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Eliminação física de `internal/agent` + desligamento do onboarding conversacional do WhatsApp
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Solicitante do produto; time de plataforma
- **Relacionados:** PRD RF-23,24,25,26,27; techspec.md; review prd-platform-mastra (achado B1)

## Contexto

`internal/agent` (assistente financeiro) está vivo e wired em `cmd/server` (`server.go`, `whatsapp_wiring.go`) e `cmd/worker` (`worker.go`), consulta as 7 tabelas `agent_*`, e a migration `000003` já as dropa — uma regressão de runtime latente (B1). A decisão de produto é **eliminar 100% `internal/agent`** e **desligar o onboarding conversacional do WhatsApp** (rota de ativação `ATIVAR <token>`), deixando apenas `internal/agents` (weather) responder. O dispatcher hoje tem duas rotas (`onboardingRoute`, `agentRoute`); três e2e de `internal/onboarding` importam `internal/agent/application/services`.

## Decisão

Apagar fisicamente `internal/agent/**`. Simplificar o `dispatcher` para **rota única** → `internal/agents` (remover `onboardingRoute`/ativação do caminho WhatsApp). Remover o wiring de `internal/agent` e do onboarding conversacional em `cmd/server`/`cmd/worker` (NewAgentModule, EventHandlers, jobs, `WhatsAppAgentRoute`, `agentbinding` no card-creator do onboarding). O **módulo `internal/onboarding` não é necessariamente apagado**, mas é **desconectado do dispatcher WhatsApp**; os e2e que importam `internal/agent` são removidos ou reescritos para não depender do módulo eliminado. Migrar config `AGENT_*` para a config do novo módulo. Critério de pronto: `grep internal/agent` (≠ `internal/platform/agent`) vazio, `test -d internal/agent` falso, build/CI verdes.

## Alternativas Consideradas

- **Manter `internal/agent` em paralelo (feature flag)**: rejeitada — viola "eliminar 100%"; mantém a regressão B1 e código morto.
- **Apagar também `internal/onboarding`**: não decidido aqui — o usuário pediu desligar o onboarding *conversacional do WhatsApp*; apagar o módulo inteiro é maior e pode afetar binding de assinatura por outros canais. Mantemos o módulo, desconectado do WhatsApp; reavaliar em ADR futura se for confirmado descarte total.
- **Soft-delete (mover para `_legacy`)**: rejeitada — pedido explícito de exclusão física.

## Consequências

### Benefícios Esperados
- Elimina a regressão B1 (migration dropando tabelas de módulo vivo).
- Remove superfície morta; canal WhatsApp simplificado.

### Trade-offs e Custos
- Perda do comportamento financeiro e do onboarding conversacional no WhatsApp (decisão de produto).
- "ATIVAR <token>" deixa de ser atendido no WhatsApp.

### Riscos e Mitigações
- Irreversível: executar a exclusão **somente após** `internal/agents` wired e build/CI verdes; commit isolado para rollback via revert.
- Quebra de e2e/onboarding: ajustar/remover antes do build final; CI determinístico como gate.
- Config órfã: checklist de migração `AGENT_*`.

## Plano de Implementação

1. Implementar `internal/agents` e wiring WhatsApp (ADR-001/002/003) — antes de apagar.
2. Religar `cmd/server`/`cmd/worker` para `internal/agents`; rota única no dispatcher.
3. Apagar `internal/agent/**`; remover/realocar e2e dependentes; migrar config.
4. Rodar gates: `grep internal/agent` vazio, `test -d internal/agent` falso, `go build ./...`, `go test`, gates de governança, gofmt.

## Monitoramento e Validação

- CI verde; gate de ausência de `internal/agent`; smoke de inbound WhatsApp.

## Impacto em Documentação e Operação

- Runbook: remoção do agente financeiro e do onboarding conversacional; comunicação de que "ATIVAR" não responde mais; atualização de envs.

## Revisão Futura

- Reavaliar descarte total de `internal/onboarding` e eventual reintrodução do domínio financeiro sobre a plataforma.
