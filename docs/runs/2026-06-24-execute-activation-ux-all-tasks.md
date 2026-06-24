# Prompt Mandatório — Executar TODAS as Tarefas de Activation UX (`execute-all-tasks`)

- **Data**: 2026-06-24
- **Skill alvo**: `.github/skills/execute-all-tasks/` (orquestra `execute-task` por tarefa)
- **Bundle de origem**: `.specs/prd-activation-ux/` (prd.md, techspec.md, adr-001..002, tasks.md, task-1.0..3.0)
- **Escopo**: 3 tarefas (2 backend Go + 1 frontend Astro/JS); MVP robusto, production-ready/proof, inegociável
- **Repos envolvidos**: `mecontrola` (tasks 1.0 e 2.0) e `mecontrola-landingpage` (task 3.0)
- **Restrição crítica**: zero regressão no fluxo de ativação existente — fluxo funciona 100% hoje

---

## Como usar

Cole o bloco abaixo (entre as cercas) como prompt para o agente nesta mesma sessão/repo. Ele invoca a
skill `execute-all-tasks` com o slug `activation-ux` e impõe os critérios de aceite com evidência.

```text
Você é o orquestrador de execução. Invoque a skill `execute-all-tasks` para o PRD `activation-ux`
(`.specs/prd-activation-ux/`) e execute TODAS as 3 tarefas até `done`, de forma INEGOCIÁVEL e
production-ready/proof. Não faça nada fora do que as tarefas, o PRD, a techspec e os ADRs definem.

ENTRADA
- slug: activation-ux
- bundle: .specs/prd-activation-ux/ (prd.md, techspec.md, adr-001..002, tasks.md, task-1.0..3.0)

REGRAS INVIOLÁVEIS (além das da própria skill)

1. SUBAGENTS FRESH + DAG OBRIGATÓRIO
   Cada tarefa roda em subagent fresh via `execute-task`; o orquestrador NUNCA executa inline.
   Respeite o DAG e o `Paralelizável` de tasks.md:
   - Wave 1 (paralelas): 1.0 e 2.0 (backends independentes).
   - Wave 2 (sequencial): 3.0 somente após 1.0 `done`.
   Halt-first: pare na primeira tarefa ≠ done antes de avançar a próxima wave.

2. ZERO REGRESSÃO — INEGOCIÁVEL
   O fluxo de ativação existente (email → /ativar?token= → WhatsApp ATIVAR) funciona 100% hoje.
   Arquivos intocáveis de cada task (listados em "Intocáveis (zero mudança)" nos task files) NÃO
   podem ser modificados. Após qualquer mudança, gate obrigatório:
     git diff --name-only HEAD | grep -E "consume_magic_token|whatsapp_message_processor|dispatcher|activation_command|magic_token_repository"
   Resultado não-vazio = falha imediata com `failed: regressão em arquivo intocável`.

3. DoD + CRITÉRIOS DE ACEITE COM EVIDÊNCIA
   Uma tarefa só é `done` quando TODOS os itens de "## Testes da Tarefa" e "## Critérios de Sucesso"
   do task-*.md forem satisfeitos e EXECUTADOS, com a saída real registrada no
   `.specs/prd-activation-ux/<id>_execution_report.md`. Sem evidência física e legível = NÃO é done
   (`failed: missing evidence`). Proibido falso positivo: relatar verde sem rodar é violação inegociável.

4. REGRAS Go (tasks 1.0 e 2.0) — HARD, sem exceção
   a. Zero comentários em `.go` de produção (R-ADAPTER-001.1). Gate:
        grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
          "^[[:space:]]*//" \
          internal/onboarding/application/ \
          internal/onboarding/infrastructure/ \
          configs/ \
          | grep -Ev "(//go:|//nolint:|// Code generated)" \
          && echo "FAIL: comentários proibidos" && exit 1 || true
   b. Testes no padrão testify/suite whitebox com `fake.NewProvider()` (R-TESTING-001).
      Proibido `package <X>_test` em `usecases/`; proibido `noop.NewProvider()`.
   c. DTOs de input com `Validate() error` usando `errors.Join` (R-DTO-VALIDATE-001).
   d. Adaptadores finos — zero SQL direto em handlers/consumers/producers (R-ADAPTER-001.2).
   e. `errors.Join` para agregação de erros; `fmt.Errorf("ctx: %w", err)` para wrapping (R7.6).
   f. Zero `init()` (R0); zero `panic` em produção (R5.12); `context.Context` em toda fronteira IO (R6).

5. GATES OBRIGATÓRIOS POR TASK Go (tasks 1.0 e 2.0)
   Cada subagent DEVE rodar na sequência abaixo e capturar saída no report:
     go build ./...
     go test ./internal/onboarding/...
   Além dos gates específicos listados em "## Critérios de Sucesso" de cada task file.
   Qualquer falha em `go build` ou `go test` = `failed`; não avançar.

6. TASK 1.0 — CONTRATO DA API (R-ADAPTER-001, ADR-001)
   Após a task 1.0, o endpoint `GET /api/v1/onboarding/tokens/{token}/state` DEVE retornar:
   - `ready_to_activate: true`  → JSON inclui `support_url` não-vazio.
   - `not_found / expired / pending` → JSON inclui `reason` e `support_url`; NÃO inclui `wa_me_url`.
   - `consumed` → JSON inclui `reason="consumed"`, `wa_me_url`, `bot_number_display`, `support_url`.
   Verificar via testes unitários do handler (token_state_handler_test.go) — evidência obrigatória.
   `sanitizeE164` movida para `e164.go`; task 2.0 consome esse helper — garantir que o arquivo existe.

7. TASK 2.0 — CONTRATO DO EMAIL (ADR-002)
   Após a task 2.0:
   - `ActivationTemplateInput` tem `WaMeURL` e `SupportURL`; NÃO tem `ActivateURL`.
   - Botão CTA do HTML aponta para `WaMeURL` (wa.me com ATIVAR TOKEN pré-preenchido).
   - Nenhum texto visível do email contém a string `token=` (URL crua proibida).
   - `SupportURL` não contém `?text=` (link limpo, sem texto pré-preenchido).
   - `configs/config.go` não contém `ActivateURL` nem `EMAIL_ACTIVATE_URL`.
   - `go build ./...` passa limpo após a remoção do campo.

8. TASK 3.0 — FRONTEND (repositório `mecontrola-landingpage`)
   Task 3.0 toca SOMENTE o repositório `mecontrola-landingpage`. Subagent DEVE:
   a. Tornar `src/pages/ativar.astro` a rota canonical (HTML completo); `activate.astro` vira
      redirect 301 para `/ativar` + query params preservados.
   b. `activate.js` deve: timeout 5s via AbortController; mapa ERROR_MESSAGES por reason
      (expired/pending/not_found); estado `consumed` → `#activate-consumed` (não `#activate-error`);
      countdown 3s com `setInterval` antes de redirect para `wa_me_url`; botão imediato visível;
      `#activate-support-btn` preenchido com `support_url` nos estados de erro.
   c. Novos elementos HTML obrigatórios em `ativar.astro`: `#activate-consumed`, `#activate-countdown`,
      `#activate-support-btn`, `#activate-error-detail`.
   d. Zero asset de logo criado — usar exclusivamente asset existente no repo.
   e. Evidência de testes E2E: `pnpm playwright test` 100% pass com os cenários de "## Testes da Tarefa"
      (expired, pending, not_found, consumed, timeout, countdown, botão imediato, support button,
      redirect 301). Saída do playwright registrada no report.

9. INTEGRIDADE CROSS-REPO (risco de integração)
   Tasks 1.0 e 2.0 compartilham `sanitizeE164` via `e164.go` (criado em 1.0, consumido em 2.0).
   Subagent de 2.0 DEVE verificar a existência de `e164.go` antes de compilar — se ausente,
   retornar `blocked: e164.go não encontrado; aguardando task 1.0 done`.
   A wave paralela 1.0↔2.0 é segura apenas se 2.0 consumir o helper sem criar sua própria cópia.

PROCEDIMENTO
- Rode o pré-voo da skill (hook `pre-execute-all-tasks.sh`, `ai-spec skills --verify`,
  `ai-spec check-spec-drift .specs/prd-activation-ux/tasks.md`). Se algum gate falhar, retorne
  `blocked`/`needs_input` com o stderr — não degrade silenciosamente.
- Execute as waves respeitando dependências e `Paralelizável`:
    Wave 1: spawn 1.0 e 2.0 em paralelo. Aguardar ambos antes de decidir.
    Wave 2: spawn 3.0 somente após 1.0 `done`. Se 1.0 ≠ done → halt com `partial`.
- Cada subagent retorna YAML `{status, report_path, summary}`; valide os 4 passos obrigatórios
  (formato canônico, status ∈ {done,blocked,failed,needs_input}, evidência física em
  `.specs/prd-activation-ux/<id>_execution_report.md` não-vazio, consistência tasks.md).
- Ao final, gere `.specs/prd-activation-ux/_orchestration_report.md` com snapshot inicial vs final,
  tabela de tarefas executadas/puladas, waves, gates rodados e próximos passos de deploy.

CRITÉRIO DE ENCERRAMENTO
- `done` somente se as 3 tarefas estiverem `done` com evidência física e os gates de regressão
  (Regra 2 + gates Go + playwright) retornarem OK.
- Caso contrário, `partial`/`blocked`/`failed` com o motivo exato e a primeira tarefa que travou.
- NÃO marque `done` com qualquer DoD/critério de aceite não comprovado — falso positivo é
  violação inegociável desta execução.
```

---

## Ordem de execução esperada (DAG de tasks.md)

| Wave | Tarefas | Repositório | Foco |
|------|---------|------------|------|
| 1 (paralelas) | 1.0, 2.0 | `mecontrola` | Backend: API reason+support_url; email WaMeURL |
| 2 (sequencial) | 3.0 | `mecontrola-landingpage` | Frontend: bridge canonical, reason-aware, countdown |

> Task 3.0 só inicia após 1.0 `done` — consome `reason` e `support_url` da API no smoke test final.
> Task 2.0 é independente funcionalmente de 1.0, mas ambas compartilham `sanitizeE164` via `e164.go`;
> subagent de 2.0 deve verificar presença do arquivo antes de compilar.

## Pré-requisitos operacionais

- Binário `ai-spec` no PATH; hooks de governança instalados (`.agents/hooks/` via `ai-spec install`).
- `ai-spec check-spec-drift .specs/prd-activation-ux` deve retornar "sem drift" antes de iniciar
  (já validado em 2026-06-24).
- Go toolchain disponível para tasks 1.0 e 2.0.
- Node.js + pnpm + Playwright instalados no repo `mecontrola-landingpage` para task 3.0.
- Para o smoke test final de 3.0: backend de 1.0 disponível (ou mock da API com os payloads de
  referência da techspec seção 9).

## Evidência mínima por tarefa (no `<id>_execution_report.md`)

**Tasks 1.0 e 2.0 (Go):**
- Saída de `go build ./...` (verde, sem erros).
- Saída de `go test ./internal/onboarding/...` (verde, zero falhas).
- Saída dos greps de gate (zero comentários, zero SQL em adapters, zero arquivos intocáveis modificados).
- Checklist de `## Testes da Tarefa` marcado item a item com referência à evidência.
- Para task 1.0: amostras do JSON retornado pelo handler para cada estado do token.
- Para task 2.0: confirmação de que `ActivateURL` foi removido de `configs/config.go`.

**Task 3.0 (Frontend):**
- Saída completa de `pnpm playwright test` (todos os cenários passing, incluindo os novos 9).
- Confirmação de redirect 301 para `/activate?token=x` → `/ativar?token=x`.
- Confirmação de que nenhum asset de logo foi criado (output de `git status`).
- Checklist de `## Testes da Tarefa` marcado item a item.

## ADRs vinculadas

| ADR | Decisão | Impacto |
|-----|---------|---------|
| `adr-001-expor-reason-token-state.md` | Expor `reason` + `support_url` no endpoint de estado | Task 1.0 |
| `adr-002-email-cta-wame.md` | CTA do email aponta para wa.me (não para página web) | Task 2.0 |
