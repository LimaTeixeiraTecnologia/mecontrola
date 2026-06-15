# Prompt de Review - prd-outbox-aggregate-user-id

Use a skill `review` para revisar o diff real vinculado a `.specs/prd-outbox-aggregate-user-id/`.

Regras mandatórias:
- Nao implemente nada.
- Nao trate grep, checklist ou alegacao textual como prova suficiente.
- Nao aceite `APPROVED_WITH_REMARKS` como estado terminal.
- Nao aceite cobertura vaga dos callers nem allowlist generica.

Contexto obrigatorio:
- `AGENTS.md`
- `.agents/skills/review/SKILL.md`
- `.agents/skills/go-implementation/SKILL.md`
- `.specs/prd-outbox-aggregate-user-id/prd.md`
- `.specs/prd-outbox-aggregate-user-id/techspec.md`
- `.specs/prd-outbox-aggregate-user-id/tasks.md`
- task file ativa quando houver
- ADRs e execution reports desta spec quando o diff tocar a decisao correspondente
- working tree atual
- diff real

Objetivo:
- provar ou refutar atendimento integral dos RF-01 a RF-20, M-01 a M-08 e criterio de aceitacao global
- verificar se o rollout atomico migration + codigo + callers + gate foi realmente suportado
- verificar aderencia real a `go-implementation`
- verificar uso contextual de DMMF sem cargo cult

Gates inegociaveis:
- migration `000017` com coluna `aggregate_user_id UUID NULL`, index parcial e down consistente
- `outbox.Event`, `EventInput`, `Row`, `NewEvent`, `Pack` e storage refletindo o novo campo sem regressao
- validacao opcional v1 correta, warning estruturado e metrica `outbox_events_inserted_total{has_user_id}`
- 12 callers atualizados de forma exaustiva, ou allowlist explicita, minima e justificavel para eventos de sistema
- `lint-outbox-user-id.sh` e `task lint:outbox-user-id` cobrindo os construtores relevantes
- ausencia de regressao em dispatcher, reaper, housekeeping, consumers e envelope JSON
- zero comentario em `.go` de producao
- zero dependencia nova em `go.mod`

Gate DMMF:
- nao exigir DMMF ornamental em migration, storage, taskfile, script de lint ou adapter de infra simples
- se houver novos tipos de dominio ou invariantes, verificar modelagem explicita apenas na superficie introduzida
- modelagem frouxa que torne tenancy ou invariantes mais ambiguos = finding

Severidade minima:
- caller faltante, gate incompleto, migration inconsistente, regressao de envelope ou risco de deploy quebrado = `high`
- lacuna que permita eventos seguirem sem `aggregate_user_id` por erro nao monitorado ou quebre consumers/storage = `critical` ou `high`

Loop obrigatorio:
1. Rode `review`.
2. Se `BLOCKED`, pare e detalhe o contexto faltante.
3. Todo finding, inclusive `medium` ou `low`, vira bug canonico para `bugfix`.
4. Rode `bugfix` com testes de regressao por bug.
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
