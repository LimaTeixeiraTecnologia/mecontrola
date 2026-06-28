# Prompt Mandatório de Execução — Orquestração `execute-all-tasks`

> **Uso:** substitua os placeholders `<SLUG>`, `<N_TAREFAS>` e demais marcadores pelo PRD alvo e cole o bloco em "PROMPT PRONTO PARA USO" como mensagem inicial de uma sessão de execução. Este documento é o **contrato de execução**: inegociável, sem flexibilização, MVP robusto e production-ready/proof, com fidelidade ao PRD, DoD e critérios de aceite atendidos **com evidência**.
>
> **Data:** 2026-06-27 · **Template para:** `.specs/prd-<SLUG>/`

---

## Instruções de preenchimento

Antes de copiar o prompt, substitua:

| Placeholder | Valor esperado | Exemplo |
|-------------|----------------|---------|
| `<SLUG>` | Nome curto do PRD (diretório sem prefixo `prd-`) | `agent-capability-catalog` |
| `<N_TAREFAS>` | Quantidade total de tarefas em `tasks.md` | `6` |
| `<DAG>` | Resumo do DAG de dependências (copiar de `tasks.md`) | `1.0 → 2.0 → (3.0 ∥ 4.0) → 5.0` |
| `<PARALELOS>` | Tarefas declaradas como paralelizáveis | `3.0 ∥ 4.0` |
| `<MODULOS_GO>` | Pacotes Go afetados | `internal/agent/... internal/platform/workflow/...` |
| `<LINGUAGENS>` | Skills de linguagem obrigatórias | `go-implementation` |
| `<SKILLS_ADICIONAIS>` | Skills processuais declaradas no PRD | `mastra` |
| `<GATES_EXTRA>` | Gates específicos do domínio (opcional) | ver exemplos nos PRDs anteriores |

---

## PROMPT PRONTO PARA USO

```text
TAREFA: Implementar e ENTREGAR, de ponta a ponta, TODAS as <N_TAREFAS> tarefas de
`.specs/prd-<SLUG>/` (task-1.0 … task-<N_TAREFAS>.0), respeitando o DAG de
dependências, com MVP robusto e production-ready/proof. Inegociável e sem flexibilização.

MODO DE EXECUÇÃO — OBRIGATÓRIO:
- Use a skill `execute-all-tasks` para orquestrar o PRD inteiro.
- Spawnar UM SUBAGENTE FRESH por tarefa, isolando contexto, conforme a skill `execute-all-tasks`.
- Respeitar o grafo de dependências de `.specs/prd-<SLUG>/tasks.md`, halt-first e retomada idempotente.
- Paralelizar apenas tarefas declaradas como `Paralelizável: Sim`/`Com X.Y, Z.W` em `tasks.md` e apenas quando o tool suportar spawn nativo.
- NÃO executar `execute-task` inline no orquestrador.
- NÃO pular tarefa, NÃO marcar `done` sem evidência, NÃO mascarar falha.

FONTE DA VERDADE (ler na íntegra antes de tocar código):
- `.specs/prd-<SLUG>/prd.md` — requisitos funcionais e decisões travadas.
- `.specs/prd-<SLUG>/techspec.md` — arquitetura, interfaces, contratos, riscos.
- `.specs/prd-<SLUG>/adr-*.md` — decisões arquiteturais inegociáveis.
- `.specs/prd-<SLUG>/tasks.md` + todos os `task-X.Y.md` — DoD, critérios, subtarefas.
- `AGENTS.md` e `CLAUDE.md` — governança canônica do repositório.

SKILLS OBRIGATÓRIAS:
- `agent-governance` é auto-carregada em toda tarefa.
- `<LINGUAGENS>` é MANDATÓRIA para TODA edição de código: carregar SKILL.md e executar as Etapas 1–5
  (Regras Estritas + Checklist de Validação). Verificar a versão em `go.mod` / `package.json` / `pyproject.toml`
  antes de usar APIs/dependências novas.
- `<SKILLS_ADICIONAIS>` é OBRIGATÓRIA nas tarefas que tocarem seus domínios (conforme declarado em cada `task-X.Y.md`).
- Carregar APENAS as skills declaradas em `## Skills Necessárias` do task file e na coluna `Skills` de `tasks.md`;
  divergência entre as duas fontes BLOQUEIA a tarefa (`failed: skills sync drift`).

REGRAS HARD — INEGOCIÁVEIS (qualquer violação BLOQUEIA o `done`):
- R-AGENT-WF-001 (se `internal/agent` for afetado): roteamento Workflow→Tool/Step→binding→usecase;
  PROIBIDO novo `case intent.Kind` de domínio no switch de `daily_ledger_agent.go`;
  Tool/Step finos (zero regra de negócio, SQL direto ou branching de domínio);
  `ToolOutcome`/`RunStatus`/`AwaitingKind` como TIPOS FECHADOS;
  Run auditável (`thread_id`, `run_id`, `workflow`, `tool`, `status`, `duration_ms`, `error`);
  resume ANTES do parse; LLM só no step de parse.
- R-WF-KERNEL-001 (se `internal/platform/workflow` for afetado): kernel permanece GENÉRICO —
  proibido import de `internal/agent`, `internal/transactions`, `internal/billing`, `internal/identity`
  ou tipos semânticos de domínio; proibido regra/branching/LLM/SQL no kernel; estados fechados.
- R-ADAPTER-001.1: ZERO comentários em `.go` de produção (exceções: `//go:`, `//nolint:`, `// Code generated`).
- R-ADAPTER-001.2: adapters/consumers/jobs/handlers finos `adapter→usecase`; sem SQL direto.
- R6: `context.Context` em toda fronteira de IO; DI via construtores explícitos.
- R5.10: erros via `errors.New`, `fmt.Errorf("ctx: %w", err)`; tratar erro uma única vez.
- R5.12: sem `panic` em produção.
- R7.6: usar `errors.Join` para agregar erros.
- DMMF (Wlaschin): state-as-type, smart constructors, `Decide*` PURO (sem IO/context/time.Now),
  pipeline parse→validate→decide→persist→publish. PROIBIDO `Result`/`Either` custom, currying, DSL,
  mônada. PROIBIDO abstrair tempo (usar `time.Now().UTC()` inline quando permitido) e
  `var _ Interface = (*T)(nil)`.
- Idempotência por `event_id` em todo consumer de outbox.

DEFINITION OF DONE (por tarefa — TODOS obrigatórios):
1. Todos os `<requirements>`, subtarefas e critérios de sucesso do `task-X.Y.md` atendidos.
2. Todos os RF cobertos pela tarefa implementados e rastreáveis.
3. Código escrito seguindo as regras hard acima; sem comentários, sem `panic`, sem SQL em adapter.
4. Testes da tarefa criados E executados, PASSANDO. Sem teste = sem `done`.
5. Build, lint e teste do pacote afetado verdes:
   - `go build ./...` / `go vet ./...` / `go test -race -count=1 <MODULOS_GO>` (Go)
   - ou equivalente da stack detectada.
6. Gates de verificação (§4) retornam VAZIO/OK.
7. Relatório de execução salvo em `.specs/prd-<SLUG>/<id>_execution_report.md` com:
   - arquivos criados/modificados (lista exata);
   - comandos rodados + saída literal;
   - RF cobertos → evidência;
   - riscos residuais e suposições.
8. Status em `tasks.md` atualizado para `done` somente após 1–7.

CRITÉRIOS DE ACEITE GLOBAIS (com evidência ao final):
- Todos os critérios de aceite do `prd.md` atendidos e comprovados.
- Nenhuma suíte existente quebrada.
- `ai-spec check-spec-drift .specs/prd-<SLUG>` sem drift.
- `_orchestration_report.md` consolidado com estado de cada tarefa, evidências e riscos.

GATES DE VERIFICAÇÃO OBRIGATÓRIOS (devem retornar vazio/OK antes de cada `done`):

# zero comentários em Go de produção
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" <MODULOS_GO> | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentários em Go de produção" || echo "OK: sem comentários"

# kernel genérico: sem import de domínio (se internal/platform/workflow afetado)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/agent\|internal/transactions\|internal/billing\|internal/identity" \
  internal/platform/workflow/ && echo "FAIL: import domínio no kernel" || echo "OK: kernel genérico"

# sem SQL direto nem LLM no kernel (se internal/platform/workflow afetado)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|openai\|anthropic\|ParseInbound" \
  internal/platform/workflow/ | grep -v "infrastructure/postgres" \
  && echo "FAIL: SQL/LLM no kernel" || echo "OK: kernel sem SQL/LLM"

# switch de domínio não cresce em daily_ledger_agent.go (se internal/agent afetado)
f=$(find internal/agent -name daily_ledger_agent.go ! -name "*_test.go" 2>/dev/null); \
  [ -n "$f" ] && c=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f"); \
  [ "${c:-0}" -gt 1 ] && echo "FAIL: switch de domínio cresceu" || echo "OK: switch contido"

<GATES_EXTRA>

PROIBIÇÕES (qualquer ocorrência invalida a entrega):
- Flexibilizar qualquer regra, etapa, critério ou decisão por conveniência, ferramenta ou prazo.
- Marcar `done` sem testes executados e evidência anexada (falso positivo é inaceitável).
- Introduzir comentários em `.go`, SQL em adapter, regra de domínio em Tool/Step/Handler/Consumer/Job ou no kernel.
- Inventar package/handler/evento/tool/rota/consumer/memória inexistente sem verificar o código real.
- Git destrutivo ou push/PR sem pedido explícito do usuário.
- Executar `execute-task` inline no orquestrador — toda tarefa em subagente fresh.

PROTOCOLO POR TAREFA:
1. Ler `prd.md` + `techspec.md` + ADRs pertinentes + o `task-X.Y.md`.
2. Carregar `<LINGUAGENS>` (Etapas 1–5) e `<SKILLS_ADICIONAIS>` quando aplicável.
3. Modelar respeitando fronteiras; implementar adaptando exemplos ao código real (nunca copiar cegamente).
4. Criar e rodar testes; rodar build/lint/teste e os gates de verificação.
5. Escrever `<id>_execution_report.md` com evidências; atualizar `tasks.md` para `done`.
6. Retornar ao orquestrador o YAML estrito:
   ```yaml
   status: done
   report_path: .specs/prd-<SLUG>/<id>_execution_report.md
   summary: <uma linha>
   ```

ENTREGA FINAL:
- Relatório consolidado `.specs/prd-<SLUG>/_orchestration_report.md` com o estado de cada tarefa,
  evidências dos critérios de aceite globais e riscos residuais.
- `ai-spec check-spec-drift .specs/prd-<SLUG>` sem drift.
- Build/lint/teste/integração/e2e verdes conforme exigido pelo PRD.
- NÃO commitar/push/PR a menos que o usuário peça explicitamente.

Em caso de ambiguidade material não coberta por PRD/techspec/ADR/AGENTS.md: PARAR e perguntar em
múltipla escolha (recomendação na 1ª opção). Não inventar comportamento.
```

---

## Notas de uso (fora do prompt)

- **Orquestração:** o orquestrador deve invocar `execute-all-tasks` com o `<SLUG>` e NUNCA executar `execute-task` inline. Cada subagente carrega apenas o necessário (`agent-governance` + linguagem + skills declaradas no task file).
- **DAG e paralelismo:** respeitar literalmente a coluna `Dependências` e `Paralelizável` de `tasks.md`. Halt-first ao primeiro `failed`/`blocked`/`needs_input` não resolvido.
- **Evidência = contrato:** cada tarefa só fecha com `<id>_execution_report.md` contendo arquivos, comandos+saída, RF cobertos, riscos e suposições — política de evidência de `AGENTS.md` e `.claude/rules/governance.md`.
- **Rastreabilidade:** `tasks.md` carrega `spec-hash-prd`/`spec-hash-techspec`; rodar `ai-spec check-spec-drift .specs/prd-<SLUG>/tasks.md` ao final.
- **Subagentes especializados:** usar o agent nativo do tool quando disponível (Claude Code: `.claude/agents/task-executor.md`; Copilot CLI: `.github/agents/task-executor.agent.md`). Para Codex/Gemini, usar subprocesso isolado (`codex exec` / `gemini --acp`) ou degradar para inline sequencial e registrar no `_orchestration_report.md`.
- **Validação do YAML retornado:** status ∈ `{done, blocked, failed, needs_input}`; `report_path` relativo à raiz do repositório; arquivo não vazio; `tasks.md` consistente com `done`.

**Fim do template. Execução inegociável.**
