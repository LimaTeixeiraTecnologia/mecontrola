# Execução completa — PRD `prd-alertas-proativos`

- Data: 06-08-2026
- Branch: `feat/alertas-proativos-prd-conformance`
- Fonte única: `.specs/prd-alertas-proativos/`
- Skill orquestradora: `execute-all-tasks`

## Snapshot inicial

- `pre-execute-all-tasks.sh alertas-proativos` → `OK (PRD alertas-proativos, 8 tarefas validadas)`
- `tasks.md`: 8/8 tarefas já marcadas `done` de uma sessão anterior (2026-08-05), trabalho não commitado.
- Achado de risco antes de aceitar o estado `done`: existia um prompt de review crítico salvo em
  `docs/reviews/2026-08-05-review-prd-alertas-proativos.md` (nunca executado) apontando risco de
  divergência residual em RF-05. Status `done` em markdown não foi aceito como prova — a instrução do
  usuário exige validação material.

## Decisão de execução

Como as 8 tarefas já estavam `done`, a Etapa 3 (loop topológico) do `execute-all-tasks` não tinha
`pending` para spawnar. Para cumprir a exigência do usuário ("não considere concluído enquanto existir
qualquer divergência"), a orquestração foi complementada com o ciclo `review → bugfix → review` da
skill `review`, confrontando PRD, techspec, ADRs, 8 task files e código real — sem aceitar declaração,
checklist ou status de task como evidência.

## Rodada 1 de review — `REJECTED`

Subagente `reviewer` (diff completo: 47 arquivos, ~2150 inserções/351 deleções, contra
`origin/main`) confrontou individualmente RF-01..18, REQ-01..25, os 3 ADRs e os critérios de
sucesso/DoD das 8 task files. Achados:

| # | Severidade | Resumo |
|---|---|---|
| 1 | HIGH | RF-07 ("no máximo 1 alerta por usuário por rodada diária") só era garantido dentro de uma única execução do use case, não entre rodadas de cron (`BUDGETS_THRESHOLD_ALERTS_CRON=@hourly` em produção). `alreadySent` era chaveado por kind, então um usuário suprimido por prioridade numa rodada podia receber um segundo alerta de kind diferente na rodada seguinte do mesmo dia. |
| 2 | HIGH | RF-05 do `prd.md` contradizia "Decisões Fechadas" e a `techspec.md`: descrevia uma feature ("motivação semanal e retomada de uso após 3 dias") nunca modelada no domínio como kind emitível do Release 1. |
| 3 | LOW | `budget_not_reviewed_day_3` permanentemente não entregável em produção (Meta classificou o template como `MARKETING`; sem fonte de consentimento no repositório) — risco residual documentado, não bug de código. |
| 4 | LOW | `SuppressedByFrequency` declarado no enum de `SuppressionReason` mas nunca emitido por nenhum caminho de código, divergindo do runbook. |
| 5 | LOW | Import fora do agrupamento convencional em `activation_attempt_consumer_integration_test.go`. |

## Correções aplicadas (bugfix)

1. **RF-07 / achado 1 e 4**: `services.DecideDailyRoundAlerts` (domínio, puro) passou a receber
   `alreadyAlertedToday map[uuid.UUID]struct{}` e suprime com `SuppressedByFrequency` qualquer
   candidato cujo usuário já tenha recebido qualquer alerta no dia — antes da seleção por prioridade.
   `evaluate_threshold_alerts.go` constrói esse conjunto a partir de `sentRepo.ListSentForDay`
   (todos os kinds do dia, não filtrado por kind). Testes novos:
   - `TestDecideDailyRoundAlertsSuppressesUserAlreadyAlertedInEarlierRound` (domínio).
   - `TestExecute_SuppressesUserAlreadyAlertedByDifferentKindInEarlierRound` (usecase, prova cross-kind
     fim a fim sem `Publish`/`InsertSent` configurados nos mocks — panicaria se fossem chamados).
2. **RF-05 / achado 2**: `prd.md` RF-05 reescrito para descrever o comportamento real — `weekly_motivation`
   e `usage_reactivation_3d` ficam fora do Release 1, bloqueados de forma auditável
   (`cmd/tools/audit-alert-readiness`) mesmo com template `APPROVED`/opt-in presentes. Seção "Fora de
   Escopo" atualizada nomeando os dois kinds. `spec-hash-prd` recalculado em `tasks.md`
   (`ai-spec hash` → `b60e8b64...`); `ai-spec check-spec-drift` confirma `OK: sem drift detectado.`
3. **Achado 3**: mantido como risco residual aceito — `DenyAllMarketingConsent` nega por padrão
   (comportamento seguro, não é falso-sucesso), documentado em `docs/runbooks/alertas-proativos.md` e
   `meta-templates-status.md`. Não é gap do PRD nem funcionalidade a inventar.
4. **Achado 5**: import reagrupado e `gofmt -w` aplicado.

## Rodada 2 de review — `APPROVED`

Subagente `reviewer` (rodada de remediação) confirmou, com evidência material (leitura de código,
testes, `ai-spec check-spec-drift`, `go build`/`vet`/`test -race`), que os 5 achados foram resolvidos
sem regressão em nenhum outro caller ou fluxo. Único ponto anotado como risco residual (não bloqueante):
ausência de teste de usecase para o cenário cross-kind — fechado nesta execução com o teste
`TestExecute_SuppressesUserAlreadyAlertedByDifferentKindInEarlierRound` adicionado após o veredito.

## Validações finais (após todas as correções)

```
go build ./...                     → OK
go build -tags integration ./...   → OK
go vet ./...                       → OK
go test -count=1 ./...             → 0 falhas em todo o repositório
gofmt -l <arquivos alterados>      → vazio
ai-spec check-spec-drift .specs/prd-alertas-proativos → OK: sem drift detectado.
```

Gates de governança nos arquivos tocados: zero comentários (R-ADAPTER-001.1), sem SQL fora do
adapter Postgres, sem `user_id`/`budget_id` como label de métrica Prometheus (uso de `budget_id`
existente é só em log, não em métrica).

## Conformidade final

- 8/8 tarefas `done` em `tasks.md`, agora com evidência material confirmada (não apenas status).
- RF-01 a RF-18 e REQ-01 a REQ-25 confrontados individualmente contra código real — sem gaps.
- 3 ADRs respeitados (dry-run antes de envio real, pattern formal não aplicado, threshold 90 condicionado).
- 0 achados residuais de qualquer severidade após a segunda rodada de review + teste adicional.
- 1 risco de negócio documentado e aceito (não é gap de spec): `budget_not_reviewed_day_3` só passa a
  ser entregável quando existir fonte de consentimento `MARKETING` — fora do escopo deste PRD.

## Estado do repositório

Diff final: 48 arquivos, +2190/-353 linhas, **não commitado** (nenhum commit foi criado nesta
execução — aguardando decisão explícita do usuário para commit/push).

## Próximos passos (fora do escopo desta execução)

- Decidir e commitar a mudança.
- Avaliar se `budget_not_reviewed_day_3` deve ganhar fonte de consentimento `MARKETING` em uma
  iniciativa futura, ou permanecer não entregável por decisão de produto.
- Deploy e smoke test em produção (dry-run obrigatório antes de qualquer envio real, conforme ADR-001).
