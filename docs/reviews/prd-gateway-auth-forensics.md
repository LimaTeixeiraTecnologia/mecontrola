# Prompt de Review - prd-gateway-auth-forensics

Use a skill `review` para revisar o diff real vinculado a `.specs/prd-gateway-auth-forensics/`.

Regras mandatórias:
- Nao implemente nada.
- Nao aceite evidencia indireta.
- Nao conclua por semelhanca.
- Nao presuma atendimento parcial como suficiente.
- Nao aceite `APPROVED_WITH_REMARKS` como estado terminal.
- Nao aceite risco residual com impacto de producao.

Contexto obrigatorio:
- `AGENTS.md`
- `.agents/skills/review/SKILL.md`
- `.agents/skills/go-implementation/SKILL.md`
- `.specs/prd-gateway-auth-forensics/prd.md`
- `.specs/prd-gateway-auth-forensics/techspec.md`
- `.specs/prd-gateway-auth-forensics/tasks.md`
- task file ativa quando houver
- ADRs e execution reports desta spec quando o diff tocar a decisao correspondente
- working tree atual
- diff real

Objetivo:
- provar ou refutar, com evidencia objetiva, atendimento integral dos RF-01 a RF-23, M-01 a M-09 e criterio de aceitacao global
- verificar aderencia real a `go-implementation`
- verificar uso correto e obrigatorio de DMMF onde a spec exigir

Gates inegociaveis:
- `RequireGatewayAuth` antes de `InjectPrincipalFromHeader`
- HMAC com `current/next`, `hmac.Equal`, janela de 60s, 401 sem vazamento de detalhe
- smart constructors para `GatewaySignature` e `GatewayTimestamp`
- discriminated union exaustiva para `GatewayAuthResult`
- workflow puro `VerifyGatewayRequest` sem IO, sem `context`, sem mock
- adapter middleware fino, sem regra de negocio inline
- `auth_events` com `request_id` e `client_ip` corretos
- `lint:auth-bypass`, observabilidade, runbook, benchmark e rollout atomico
- zero comentario em `.go` de producao
- zero dependencia nova em `go.mod`

Gate DMMF:
- ausencia indevida de smart constructor, discriminated union ou workflow puro exigido pela spec = finding bloqueante
- degradacao para `bool + error`, enum fragil com campo nullable, branching de dominio em middleware ou diluicao de invariantes = finding bloqueante
- nao exigir DMMF em adapter fino, wiring ou infra sem regra de dominio

Severidade minima:
- RF nao comprovado = `high`
- quebra de seguranca, bypass, falsificacao de fronteira de confianca, falha de rollout, falha de observabilidade minima ou ausencia de teste relevante = `critical` ou `high`
- violacao material de `go-implementation` ou DMMF exigido = `high`

Loop obrigatorio:
1. Rode `review`.
2. Se `BLOCKED`, pare e detalhe a evidencia faltante.
3. Se existir qualquer finding, inclusive `medium` ou `low`, gere bugs canonicos para `bugfix`.
4. Rode `bugfix` no escopo emitido, com testes de regressao obrigatorios.
5. Rode nova `review` apenas no delta da remediacao.
6. Repita ate `APPROVED`.

Saida obrigatoria:
1. `verdict`
2. `spec_alvo`
3. `files_reviewed`
4. `refs_loaded`
5. `task_criteria_check`
6. `findings`
7. `bugs_for_bugfix`
8. `residual_risks`
9. `validations_run`
10. `next_action`
