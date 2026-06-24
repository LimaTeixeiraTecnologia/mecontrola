# Prompt Mandatório — Executar TODAS as Tarefas do PRD `refatoracao-agent-canonico`

- **Data:** 2026-06-24
- **Skill obrigatória:** `.github/skills/execute-all-tasks/` (category `governance`; spawna subagent fresh por tarefa, respeita DAG, halt-first, retomada idempotente)
- **Spec:** `.specs/prd-refatoracao-agent-canonico/` (prd.md spec-version 2, 45 RFs; techspec.md + ADR-001..008; tasks.md com 9 tarefas linear 1→9)
- **Skills por tarefa:** `mastra` (declarada) + `go-implementation` e `object-calisthenics-go` (auto-load por diff). DMMF (state-as-type, smart constructors, `Decide*` puro, discriminated unions) obrigatório.
- **Objetivo inegociável:** MVP robusto, production-ready/proof, 0 gaps, 0 lacunas, 0 falso positivo; cada tarefa só vira `done` com **DoD + critérios de aceite atendidos fielmente E evidências anexadas**.

---

## Como usar (pronto para uso)

1. Garanta o estado base: branch a partir de `main`, working tree limpo, `ai-spec` no PATH, Docker para integration tests (testcontainers).
2. Cole o **PROMPT MANDATÓRIO** abaixo na sessão e invoque a skill `execute-all-tasks` para o slug `refatoracao-agent-canonico`.
3. Execução respeita a sequência linear `1.0 → 9.0` (sem paralelismo — arquivos compartilhados de alto contágio). Halt-first: qualquer tarefa que não atinja o DoD com evidência **para** a orquestração.

---

## PROMPT MANDATÓRIO (copiar e colar)

> Execute **TODAS** as tarefas do PRD `refatoracao-agent-canonico` usando a skill **`execute-all-tasks`** (`.github/skills/execute-all-tasks/`), spawnando um subagent fresh por tarefa, respeitando o DAG linear `1.0→2.0→3.0→4.0→5.0→6.0→7.0→8.0→9.0` de `.specs/prd-refatoracao-agent-canonico/tasks.md`. **Halt-first**: ao primeiro DoD não atingido ou evidência ausente, pare e reporte `failed`/`needs_input` — não prossiga para a próxima tarefa.
>
> **Regras inegociáveis (gate de aprovação de cada tarefa):**
> 1. **Skills:** carregar `go-implementation` (Etapas 1–5 + checklist R0–R7) e `mastra` antes de qualquer `.go`; aplicar DMMF (state-as-type, smart constructors, `Decide*` puro, discriminated unions) e padrões de projeto pertinentes. `object-calisthenics-go` quando o diff justificar.
> 2. **Governança (HARD, bloqueante):** `R-AGENT-WF-001`, `R-WF-KERNEL-001`, `R-ADAPTER-001` (zero comentários em Go de produção), `R-TESTING-001` (testify/suite whitebox + `fake.NewProvider()` + mocks mockery), `R-DTO-VALIDATE-001`, `R-TXN-WORKFLOWS-001`. Sem `init()` (R0), sem `panic` em produção (R5.12), `context.Context` em toda fronteira de IO (R6), `errors.Join`/`fmt.Errorf %w` (R7).
> 3. **Fronteira de dados:** `internal/agent` acessa só tabelas próprias; consumo de outro BC só por porta de entrada (usecase/handler/producer/consumer/job). O gate `scripts/ci/agent-data-boundary.sh` deve ficar **verde** ao fim de cada tarefa.
> 4. **Cardinalidade de métrica:** labels apenas de enums fechados; proibido `user_id`/`category_id`/`correlation_key`/`message_id`.
> 5. **Decisões travadas (ADR-001..008):** canal único WhatsApp Meta (Telegram eliminado 100% incl. schema); Structured Output `Strict=true` nas classes estruturadas (parse + onboarding migrado de tool-calling→json_schema; haiku/gpt-5-nano inelegíveis; guard real-LLM); roteamento de modelo por classe; kernel caminho único (flag `TransactionsWriteEnabled` removida; deps ausentes = falha de boot); editar/apagar por referência com desambiguação reusando `destructive_confirm` (busca ILIKE no `transactions`, nunca no agent); HITL contrato ADR-003 as-is; plano multi-tool 1..N durável que **suspende o plano inteiro** no HITL e retoma do cursor, idempotência por passo (migration 000021); migration 000020 assume zero usuários Telegram com **verificação pré-deploy fail-fast**; alerting Telegram do Grafana **mantido** (fora de escopo).
> 6. **DoD por tarefa (todos obrigatórios):** subtarefas concluídas; critérios de sucesso do `task-*.md` atendidos; testes **unitários e de integração** criados e **verdes** (`go build ./...`, `go vet ./...`, `go test ./... -count=1 -race`, `golangci-lint run`, `mockery --config mockery.yml --dry-run`); gates R-* + gate de fronteira de dados verdes; `deadcode`/`staticcheck` sem resíduo nas tarefas de eliminação (4.0/5.0); migrations com up/down idempotentes e teste de integração (3.0/8.0); guard real-LLM verde quando aplicável (6.0).
> 7. **Evidências obrigatórias (anexar no relatório de cada tarefa):** comando + saída resumida de build/vet/test/lint; resultado de cada gate (verde/vermelho); para eliminação, `grep` comprovando remoção (ex.: `telegram` ausente, `EnableKernel` ausente, eventos órfãos sem par); para features, nomes dos testes e e2e correspondentes (ex.: `Apaga o Uber`, `Apaga o mercado`, `O Uber foi 42 e não 35`, `paguei 50 no mercado e quanto gastei?`); mapeamento RF→evidência. **Sem evidência verificável, a tarefa NÃO é `done`.**
> 8. **0 falso positivo:** não declarar `done` sem rodar os comandos; não marcar cobertura por narrativa; remoção de eventos só após confirmação por **constante de event-type** (não por nome de arquivo); preservar eventos com par.
>
> Ao final, produzir `_orchestration_report.md` em `.specs/prd-refatoracao-agent-canonico/` com o rollup: status por tarefa, evidências, RFs cobertos (todos os 45) e riscos residuais. Atualizar `tasks.md` (status `done`/`failed`) de forma idempotente.

---

## Definition of Done — global (rollup)

A iniciativa só é `done` quando **todas** as 9 tarefas estão `done` e:

- [ ] Os **45 RFs** do PRD têm evidência de cobertura (RF→teste/gate/migração).
- [ ] Canal único: `grep -ri telegram internal/ cmd/ configs/` retorna só alerting Grafana (categoria B) ou nada.
- [ ] Fronteira de dados: gate `scripts/ci/agent-data-boundary.sh` verde; zero SQL direto/import de repo de outro BC em `internal/agent`.
- [ ] Kernel caminho único: `grep -r "EnableKernel\|continuePendingExpenseConfirmationLegacy\|parity_test\|TransactionsWriteEnabled" internal/agent` vazio.
- [ ] Eventos órfãos removidos (confirmados por constante); eventos com par preservados.
- [ ] Structured Output `Strict=true` no parse + onboarding em json_schema; guard real-LLM verde por classe.
- [ ] Editar/apagar por referência + desambiguação + plano multi-tool com e2e verdes.
- [ ] Migrations 000020 e 000021 com up/down idempotentes e teste de integração; verificação pré-deploy documentada.
- [ ] Suíte completa verde: `go build ./...`, `go vet ./...`, `go test ./... -count=1 -race`, `golangci-lint run`.
- [ ] `_orchestration_report.md` gerado com evidências e riscos residuais.

## Critérios de aceite por tarefa (resumo — fonte de verdade nos `task-*.md`)

| Tarefa | Aceite verificável (evidência) |
|--------|-------------------------------|
| 1.0 | Gate de fronteira verde no estado atual; vermelho em PR de teste negativo; gates R-* wired no CI. |
| 2.0 | Build/test verdes; `grep telegram` só alerting; WhatsApp ingress/egress + E164 intactos. |
| 3.0 | Migration 000020 up/down idempotente em testcontainer; verificação pré-deploy documentada. |
| 4.0 | `grep EnableKernel\|*Legacy\|parity` vazio; resume kernel-only verde; `deadcode`/`staticcheck` limpos. |
| 5.0 | Zero producer-sem-consumer e consumer-sem-producer (por constante); eventos com par preservados. |
| 6.0 | Parse `Strict=true` (schema required-completo) sem alta de invalid_json; onboarding json_schema com paridade; guard real-LLM verde. |
| 7.0 | e2e `Apaga o Uber`/`Apaga o mercado`/`O Uber foi 42 e não 35` verdes; busca ILIKE no `transactions`; gate de fronteira verde; fix do `NewAmount`. |
| 8.0 | e2e plano composto + plano com HITL (suspend→resume do cursor); replay por `step_index` não duplica; migration 000021 testada; single-intent sem regressão. |
| 9.0 | Recorrência/% categoria via porta; consultas/parcelada/continuidade-sem-orçamento; casos especiais; undo redirect; respostas batem tom/emoji do Documento Oficial. |

## Referências
- PRD: `.specs/prd-refatoracao-agent-canonico/prd.md`
- Techspec: `.specs/prd-refatoracao-agent-canonico/techspec.md`
- ADRs: `.specs/prd-refatoracao-agent-canonico/adr-001..008-*.md`
- Tarefas: `.specs/prd-refatoracao-agent-canonico/tasks.md` + `task-1.0..9.0-*.md`
- Documento Oficial do produto: `docs/oficial/2026_06_24_mecontrola_oficial.md`
- Skill: `.github/skills/execute-all-tasks/SKILL.md`
