# Tarefa 8.0: Gate lint:auth-bypass + runbook + dashboard + microbenchmark

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fecha o ciclo do PRD com 4 entregas de production-readiness:
1. **Gate mecânico** (`task lint:auth-bypass`) que falha o CI se qualquer rota com `InjectPrincipalFromHeader` no chain perder o `RequireGatewayAuth` à frente — defesa M-09 inegociável.
2. **Runbook** consolidando vetor fixo HMAC, procedimento de rotação `current/next`, plano de rollout atômico (ADR-005), troubleshooting por `result`.
3. **Dashboard Grafana** estendendo o painel "Auth Module" com falhas do gateway por `result`, rotação observada, latência p99.
4. **Microbenchmark** `BenchmarkRequireGatewayAuth_Valid` validando NRF-01 (overhead p99 ≤ 2 ms; target < 50µs por request em CI).

<requirements>
- RF-21: gate de revisão M-09 falha PR que adicione `InjectPrincipalFromHeader` sem `RequireGatewayAuth` imediatamente antes
- RF-22: runbook `docs/runbooks/gateway-auth.md` cobrindo rotação + rollout + troubleshooting
- NRF-01: microbenchmark valida p99 ≤ 2 ms em CI; documentar resultado
- NRF-06: documentação completa, sem comentários soltos
- Painel Grafana atualizado (JSON em `docs/dashboards/auth-module.json` se já existir)
- Zero comentário em `.go`
</requirements>

## Subtarefas

- [ ] 8.1 Criar `deployment/scripts/lint-auth-bypass.sh` que grep-a `InjectPrincipalFromHeader(WithO11y)?` em `internal/*/infrastructure/http/server/` (excluindo `_test.go` e `mocks/`) e valida que `RequireGatewayAuth` aparece nas 3 linhas anteriores. Falha CI com diagnóstico claro.
- [ ] 8.2 Adicionar receita `lint:auth-bypass` em `taskfiles/lint.yml` chamando o script (pattern do `lint:user-isolation` já implementado).
- [ ] 8.3 Validar gate adversarialmente: copiar router cards, remover `RequireGatewayAuth`, rodar gate, confirmar FAIL; restaurar, confirmar PASS. Documentar simulação no commit message.
- [ ] 8.4 Criar `docs/runbooks/gateway-auth.md` com seções: "Visão geral do contrato", "Vetor de teste fixo" (input + hex calculado em tarefa 2.0), "Procedimento de rotação", "Plano de rollout cutover" (linkando ADR-005), "Troubleshooting por result", "Alertas operacionais".
- [ ] 8.5 Criar `docs/runbooks/gateway-auth-rotation.md` com checklist passo-a-passo (provisionar NEXT → reload app → migrar cliente → promover NEXT para CURRENT → reload).
- [ ] 8.6 Atualizar `docs/dashboards/auth-module.json` (ou criar se ausente) com painéis: "Gateway Auth Result" (counter), "Gateway Auth Latency p99" (histogram), "Rotation Observed" (gauge).
- [ ] 8.7 Criar `internal/identity/infrastructure/http/server/middleware/require_gateway_auth_bench_test.go` com `BenchmarkRequireGatewayAuth_Valid` rodando happy path com input pré-computado.
- [ ] 8.8 Rodar `go test -bench=. -benchmem` no middleware e registrar resultado (ns/op + allocs/op) em comentário do PR.

## Detalhes de Implementação

Ver techspec seção "Monitoramento e Observabilidade" + ADRs 005, 006. O dashboard existente `auth-module.json` (do `prd-auth-foundation`) já tem painéis básicos; estender, não duplicar.

Skills processuais usadas: `otel-grafana-dashboards` para gerar painéis JSON consistentes com convenções; `taskfile-production` para garantir que `lint:auth-bypass` seja idempotente e robusto.

## Critérios de Sucesso

- `task lint:auth-bypass` executa em local com router de cards atual (com gateway) → PASS.
- Simulação adversarial (remover gateway temporariamente) → gate FAIL com mensagem clara identificando arquivo:linha.
- `docs/runbooks/gateway-auth.md` revisado, sem TODOs.
- Dashboard JSON validado por `jq .` (sintaxe).
- Microbenchmark roda em CI; resultado documentado no PR; ns/op < 50.000 (50µs).
- `task lint` PASS. `task test` PASS.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `otel-grafana-dashboards` — dashboard Grafana para métricas Prometheus do gateway (NRF-05); skill especifica convenção de painel auth-module no repo
- `taskfile-production` — adicionar receita `lint:auth-bypass` em `taskfiles/lint.yml` seguindo padrão production-ready do projeto (RF-21)

## Testes da Tarefa

- [ ] Simulação adversarial do gate de lint (revert + restore)
- [ ] Validação manual do runbook por leitura (sem TODOs, sem placeholders)
- [ ] `jq .` no dashboard JSON
- [ ] Execução do microbenchmark + registro de resultado

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/scripts/lint-auth-bypass.sh` (novo)
- `taskfiles/lint.yml` (modificado — receita nova)
- `docs/runbooks/gateway-auth.md` (novo)
- `docs/runbooks/gateway-auth-rotation.md` (novo)
- `docs/dashboards/auth-module.json` (criado ou estendido)
- `internal/identity/infrastructure/http/server/middleware/require_gateway_auth_bench_test.go` (novo)
