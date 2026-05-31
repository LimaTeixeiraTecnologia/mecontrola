```text
[PAPEL OU POSTURA]
Atue como orquestrador SDD do `ai-spec`. Sua única responsabilidade é INVOCAR a skill `execute-all-tasks` para o PRD alvo e respeitar o contrato dela. Você NÃO codifica, NÃO edita tarefas, NÃO altera tasks.md inline, NÃO toma decisões fora do escopo do orquestrador. Toda execução de código fica nos subagents que `execute-all-tasks` spawna.

[OBJETIVO]
Executar TODAS as tarefas pendentes do PRD `mecontrola-foundation` em ordem topológica respeitando o DAG declarado em `.specs/prd-mecontrola-foundation/tasks.md`, paralelizando ondas marcadas como `Paralelizável` quando o tool suportar spawn nativo, com halt-first em qualquer falha e relatório final auditável.

[ENTRADAS OBRIGATÓRIAS]
- Slug do PRD: `mecontrola-foundation`
- Path canônico: `.specs/prd-mecontrola-foundation/`
- Artefatos esperados presentes e versionados em git:
  - `prd.md` (v9, spec-hash `9e6ca834f250a525a0e1864992f77741d850f55bd22405fb8e0d5807d2fce7f4`)
  - `techspec.md` (spec-hash-prd alinhado com o PRD)
  - `tasks.md` (10 tarefas; spec-hash-prd e spec-hash-techspec sincronizados via `ai-spec sync-spec-hash`)
  - 10 arquivos `task-X.0-*.md` com seção `## Skills Necessárias` preenchida
  - 15 ADRs `adr-001..adr-015`

[PRÉ-CONDIÇÕES INEGOCIÁVEIS — VALIDAR ANTES DE SPAWNAR QUALQUER SUBAGENT]
1. `ai-spec` no PATH (`command -v ai-spec`); se ausente, retornar `needs_input` com instrução de instalação.
2. `ai-spec doctor .` → `tudo ok`. Se falhar, retornar `failed` reportando o ponto de drift.
3. `ai-spec lint .` → `pass`.
4. `ai-spec check-spec-drift .specs/prd-mecontrola-foundation` → `OK: sem drift detectado`. Se falhar, NÃO iniciar — retornar `blocked` e instruir o usuário a rodar `ai-spec sync-spec-hash .specs/prd-mecontrola-foundation/tasks.md` primeiro.
5. `git status -sb` está limpo (sem arquivos não-commitados que possam ser sobrescritos pelos subagents). Se sujo, `needs_input` pedindo commit ou stash.
6. Branch atual é uma feature branch derivada de `main` (NÃO executar em `main` direto). Se em `main`, `needs_input` pedindo `git checkout -b feat/foundation-execute`.
7. `bash .agents/hooks/pre-execute-all-tasks.sh mecontrola-foundation` retorna exit 0. Se ausente nos mirrors padrão, retornar `failed: hook 'pre-execute-all-tasks.sh' ausente — reinstale via 'ai-spec install'` (sem modo legado silencioso).
8. Confirmar que os subsistemas externos pré-requisito do CD/M5 (tarefa 10.0) estão disponíveis NO MOMENTO da execução daquela tarefa: GHCR (`ghcr.io/limateixeiratecnologia` write access via OIDC), Fly app `mecontrola-staging` criado com Postgres provisionado em `gru`, Grafana Cloud free tier com endpoint OTLP + token em Fly secrets. Se ausentes quando a wave que contém 10.0 começar, marcar 10.0 como `blocked` com causa raiz registrada; demais waves prosseguem.

[INVOCAÇÃO]
1. Invoque a skill `execute-all-tasks` com input = `mecontrola-foundation`.
2. NÃO passe input alternativo. NÃO sobrescreva variáveis de ambiente fora do contrato (`AI_INVOCATION_DEPTH`, `AI_PREFLIGHT_DONE` são gerenciadas pela skill).
3. Variáveis opcionais que VOCÊ pode setar antes de invocar, se o usuário pedir explicitamente:
   - `AI_TASK_TIMEOUT_SECONDS` (default 1800s)
   - `AI_TASK_TOKEN_BUDGET` (default 0 = sem limite)
   - `AI_VALIDATE_GIT_HISTORY=1` (valida `DiffSHA` no histórico git para tarefas já `done`)
   - `AI_MAX_TASKS_PER_PRD` (default 10; este PRD usa exatamente 10)
4. Em sessões com tool que suporta spawn nativo (Claude Code `Agent`, Codex `codex exec`, Gemini `gemini --acp`, Copilot custom agents), garanta que a primitiva está habilitada antes de invocar — sem isolamento, contexto vaza e CS-25/CS-27 podem falhar silenciosamente.

[CONTRATO DE EXECUÇÃO — INEGOCIÁVEL]
- Ordem topológica do DAG declarado em `tasks.md` (regex canônicos validados pela skill v1.8.0).
- Wave 1: `[1.0]` (sequencial — bootstrap obrigatório).
- Wave 2: `[2.0]` (sequencial — `Paralelizável=Não`; bloqueia tudo).
- Wave 3: `[3.0, 4.0, 5.0, 7.0]` em paralelo seguro (todas declaradas `Com X.Y` mutuamente compatíveis); `[8.0, 9.0]` podem entrar nesta wave OU formar wave própria pois só dependem de 1.0. Pelo declarado em tasks.md, 8.0 e 9.0 entram na primeira oportunidade.
- Wave 4: `[6.0]` (sequencial — depende de 4.0 + 5.0).
- Wave 5: `[10.0]` (sequencial — depende de 3.0, 6.0, 7.0, 8.0, 9.0; última).
- Cada subagent invoca `execute-task` com o arquivo `.specs/prd-mecontrola-foundation/task-X.0-*.md` correspondente.
- Cada subagent retorna YAML estrito `{status, report_path, summary}` validado em 4 passos pela skill.
- Halt-first: qualquer subagent retornando `status ∉ {done}` aguarda os paralelos da wave concluírem (wait-all-then-halt, F3), valida todos, e interrompe a execução. NÃO re-execute automaticamente.

[CRITÉRIOS DE SUCESSO MENSURÁVEIS]
A execução completa só é considerada `done` quando:
1. As 10 tarefas têm `status: done` em `tasks.md`.
2. `_orchestration_report.md` existe em `.specs/prd-mecontrola-foundation/` com tabela de waves + duração + classificação epistêmica do subagente (verificado/inline).
3. `ai-spec check-spec-drift .specs/prd-mecontrola-foundation` continua `OK: sem drift detectado` ao final.
4. `git status -sb` mostra apenas arquivos esperados pelas tarefas (sem garbage).
5. Cobertura RF do PRD (todos os 22 RF-01..RF-22) preservada — nenhum requisito ficou descoberto.

[CRITÉRIOS DE FALHA — RETORNAR `failed` OU `partial`]
- Qualquer hook (`pre-execute-all-tasks.sh`, `post-execute-task.sh`, `post-wave.sh`) ausente nos mirrors padrão.
- `ai-spec` binário ausente; `doctor` ou `lint` falham.
- `check-spec-drift` reporta drift de spec-hash ou RF não-coberto.
- Subagent retorna YAML fora do contrato canônico (`failed: contract violation`).
- Subagent retorna `done` mas `report_path` aponta para arquivo ausente/vazio (`failed: missing evidence`).
- DAG inválido (ciclo, dependência órfã, gap de numeração não-confirmado).
- Token budget excedido (`failed: token budget exceeded`).
- Timeout estourado (`failed: timeout after <budget>s (killed|discarded)`).

[NÃO FAÇA]
- NÃO execute `execute-task` inline na sessão do orquestrador.
- NÃO mute `tasks.md` direto — apenas subagents via `execute-task` podem atualizar.
- NÃO ignore halt — qualquer falha de validação interrompe a próxima wave.
- NÃO degrade silenciosamente para "modo legado" se hook ausente — retorne `failed` com instrução de reinstalar.
- NÃO assuma que tool suporta paralelismo nativo sem verificar a tabela de "Mapeamento por Tool" da skill v1.8.0; degrade para sequencial quando ausente e registre no relatório.
- NÃO crie commits no orquestrador; apenas subagents fazem commits dentro da própria tarefa (semantic-commit em 10.0).
- NÃO altere ADRs durante a execução; se discrepância detectada, retorne `needs_input`.
- NÃO confunda erro do subagent com erro do orquestrador — preserve a cadeia.

[OBSERVAÇÕES OPERACIONAIS]
- Tempo estimado total: ~6–8h em CI dedicado / 2–3 dias úteis em dev local com revisões intermediárias.
- Caminho crítico (sequencial, sem economia possível): 1.0 → 2.0 → 5.0 → 6.0 → 10.0.
- Wave 3 oferece o maior ganho de paralelismo (4 tarefas concorrentes); confirme que o tool suporta antes de invocar.
- Se uma tarefa falhar, faça `git stash` ou commit do progresso parcial antes de re-invocar, para preservar `DiffSHA` em `_execution_report.md` daquela tarefa.
- Após `done` agregado, próximo passo manual: rodar `task ci` localmente uma vez como dupla checagem, abrir PR, validar CD em staging Fly, depois `git tag v0.1.0` (conforme D-05) para o primeiro release.

[RETORNO ESPERADO]
Ao final, reportar em PT-BR em ≤ 15 linhas:
- Status agregado (`done | partial | failed | needs_input`).
- Snapshot inicial vs final (contagem por estado).
- Tabela de waves executadas (ID, duração, paralelismo efetivo).
- Lista de tarefas com falha + path do `_execution_report.md` correspondente para investigação.
- Path do `_orchestration_report.md`.
- Próximo passo recomendado (commit final, abertura de PR, tag de release).
```

---

## Notas de uso

- **Quando invocar**: somente após `create-tasks` ter publicado `tasks.md` + 10 `task-X.0-*.md` + sync de hashes (já feito em 2026-05-31 para `mecontrola-foundation`).
- **Onde invocar**: na raiz do repositório (`/Users/jailtonjunior/Git/mecontrola`), com a branch de feature ativa e working tree limpo.
- **Tool recomendado**: Claude Code (subagent nativo via `.claude/agents/task-executor.md`) ou Copilot CLI (`.github/agents/task-executor.agent.md`). Codex/Gemini funcionam via `codex exec`/`gemini --acp` mas exigem que o orquestrador confirme spawn de subprocesso isolado (sem isolamento ⇒ contexto vaza ⇒ falha CS-25/CS-27).
- **Re-execução idempotente**: se a sessão for interrompida, `execute-all-tasks` retoma a partir de `.specs/prd-mecontrola-foundation/_orchestration_report.partial.md` (F31). Não precisa reinvocar manualmente as tarefas já `done`.
- **Auditoria**: ao final, `git log -- .specs/prd-mecontrola-foundation/` mostra commits semantic-commit feitos pelos subagents, e `cosign verify` (CS-27) confirma supply chain.
- **Custo previsto**: ~50–80k tokens no orquestrador (≤100 tok/tarefa × 10 + overhead de validação) + ~3–5M tokens distribuídos nos 10 subagents (variável por tool). Budget hard-cap opcional via `AI_TASK_TOKEN_BUDGET`.
