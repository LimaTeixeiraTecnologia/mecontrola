# Prompt de Review - prd-pre-golive-hardening

Use a skill `review` para revisar o diff real vinculado a `.specs/prd-pre-golive-hardening/`.

Regras mandatórias:
- Nao implemente nada.
- Nao considere checklist marcado como prova.
- So aceite evidencias verificaveis no diff, scripts, runbooks, testes e working tree.
- Nao aceite `APPROVED_WITH_REMARKS` como estado terminal.
- Nao aceite item operacional apenas documentado e nao provado.

Contexto obrigatorio:
- `AGENTS.md`
- `.agents/skills/review/SKILL.md`
- `.agents/skills/go-implementation/SKILL.md` quando o diff tocar Go
- `.specs/prd-pre-golive-hardening/prd.md`
- `.specs/prd-pre-golive-hardening/techspec.md`
- `.specs/prd-pre-golive-hardening/tasks.md`
- task file ativa quando houver
- working tree atual
- diff real

Objetivo:
- confrontar B2, B3, B4, B5, B6, B7, A2/A4 e A10 contra o diff
- provar ou refutar atendimento integral dos RF-01 a RF-34, M-01 a M-10 e criterio de aceitacao global
- verificar aderencia real a `go-implementation`
- verificar uso de DMMF apenas onde fizer sentido real

Gates inegociaveis:
- B2 com janela de 5 min, 200 silencioso, reasons corretos e testes cobrindo dentro/fora da janela
- B3 com headers obrigatorios, bloqueio de `/admin`, `/debug/pprof`, `/metrics` e strip de headers externos sensiveis
- B4 com restore idempotente, smoke queries reais, runbook e cron
- B5 com `ufw` idempotente, regras corretas e runbook seguro
- B6 com `Config.Validate()` bloqueando CORS vazio ou `*` em `production`
- B7 com rate limit do webhook WhatsApp, envs e testes de integracao
- A2/A4 com fallback CORS seguro e ausencia de vazamento relevante do header `Server`
- A10 com rate limit por `user_id`, extractor correto, fallback coerente e metrica sem cardinalidade ruim
- zero comentario em `.go` de producao
- zero dependencia nova em `go.mod`

Gate DMMF:
- nao exigir DMMF em configuracao, Caddyfile, shell script, runbook, wiring ou adapter fino sem regra de dominio
- se o diff introduzir modelagem de dominio nova em Go, verificar invariantes e modelagem correta apenas na superficie afetada
- uso ornamental de DMMF que aumente custo sem ganho concreto = finding

Severidade minima:
- RF nao comprovado = `high`
- falha de hardening, borda exposta, restore nao provado, firewall inseguro, CORS inseguro, limiter ineficaz ou teste ausente em superficie critica = `critical` ou `high`

Loop obrigatorio:
1. Rode `review`.
2. Se `BLOCKED`, pare e detalhe a lacuna de evidencia.
3. Todo finding, inclusive `medium` ou `low`, vira bug canonico para `bugfix`.
4. Rode `bugfix` com causa raiz e testes de regressao obrigatorios.
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
