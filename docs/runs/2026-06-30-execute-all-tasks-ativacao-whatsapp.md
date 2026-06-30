# Run — Execução completa: Jornada de Ativação via WhatsApp (2026-06-30)

Prompt **pronto para uso** para orquestrar a implementação completa do PRD
`.specs/prd-ativacao-whatsapp` via a skill `execute-all-tasks`, sem desvios, sem flexibilização,
0 gap, 0 lacuna, cumprindo TODOS os critérios de aceite, regras de negócio e DoD.

- PRD: `.specs/prd-ativacao-whatsapp/prd.md` (spec-version 2, RF-01..RF-37)
- Techspec: `.specs/prd-ativacao-whatsapp/techspec.md` (spec-hash-prd `1f811a02…5076d`)
- ADRs: `adr-001` (event-driven + boas-vindas desacopladas), `adr-002` (janela 24h via `paidAt` nos 2 caminhos), `adr-003` (Oi/telefone, e-mail→/ativar, sem Telegram, cutover transicional)
- Tasks: `tasks.md` + `task-1.0…task-10.0` (drift OK, cobertura RF-01..RF-37, hashes sincronizados)

---

## PROMPT (copiar e colar)

```
Use a skill execute-all-tasks para implementar o PRD inteiro do slug: ativacao-whatsapp
(.specs/prd-ativacao-whatsapp). Execute do início ao fim, sem desvios e sem flexibilizar
nenhuma regra. Em caso de qualquer violação, PARE (halt-first) e reporte — não improvise.

CONTRATO DE ORQUESTRAÇÃO (inviolável)
- Rode o pré-voo da skill: hook pre-execute-all-tasks.sh, unset AI_PREFLIGHT_DONE,
  ai-spec skills --verify, confirmar prd.md/techspec.md/tasks.md, ai-spec check-spec-drift
  .specs/prd-ativacao-whatsapp/tasks.md. Qualquer drift/RF não coberto => blocked, não prossiga.
- Cada tarefa roda em SUBAGENT FRESH via execute-task; o orquestrador NUNCA executa inline.
  Contrato de retorno YAML estrito {status, report_path, summary}; violação => failed: contract
  violation, halt, relatório.
- Respeite o DAG e o paralelismo declarados em tasks.md exatamente:
  Ordem topológica: fundação [1.0, 2.0, 3.0] (paralelizáveis entre si) ->
  4.0 (dep 1.0,2.0; paralela com 8.0) -> 5.0 -> 6.0 -> 7.0 (núcleo sequencial) ;
  8.0 (dep 1.0,2.0,3.0; paralela com 4.0) ; 9.0 (dep 2.0,3.0; paralela com 8.0) ;
  10.0 (dep 7.0,8.0,9.0). Não reordene, não pule, não funda tarefas.
- Não mute tasks.md no orquestrador (só os subagents via execute-task). Halt-first: ao primeiro
  retorno != done após validação, pare a wave, gere _orchestration_report.md e encerre.

DEFINITION OF DONE POR TAREFA (obrigatório para marcar done)
- Implementação aderente à techspec e às ADRs referenciadas no task file (sem duplicar conteúdo:
  o subagent LÊ prd.md + techspec.md desta pasta).
- Skill go-implementation aplicada em toda mudança Go: Etapas 1-5 do SKILL.md, Regras R0-R7,
  DMMF (smart constructor, guard puro, state-as-type, pipeline, pure-core/IO-shell),
  time.Now().UTC() inline (sem Clock), errors.Join/%w, iota+1, sem init(), sem panic em produção.
- Conformidade hard: R-ADAPTER-001 (adapters finos, ZERO comentários em .go de produção, sem SQL/
  branching de domínio em handler/consumer/job/producer), R-DTO-VALIDATE-001 (Validate() no input
  DTO logo após o span), R-TESTING-001 (testify/suite whitebox, fake.NewProvider(), IIFE por mock),
  R-TXN-004 (métricas sem user_id/telefone/email como label).
- Testes CRIADOS e EXECUTADOS antes de done (unitários sempre; integração testcontainers onde a
  tarefa tocar Postgres; e2e/Playwright na 10.0). Validação proporcional ao escopo (local-minimal/
  boundary/global) reportada.
- Evidência física: <id>_execution_report.md não-vazio, report_path relativo à raiz, DiffSHA válido.
- Sem regressão nos testes existentes do módulo afetado.

REGRAS DE NEGÓCIO INVIOLÁVEIS (rejeitar qualquer implementação que viole)
1. O webhook Kiwify NUNCA ativa a conta: marca a Activation Session como PAID (registra paidAt) e
   dispara o e-mail. Ativação só ocorre na 1a mensagem do WhatsApp. (RF-03, RF-04)
2. Mensagem do usuário é "Oi" puro, sem código visível. O backend constrói wa_me_url com "Oi"
   quando há telefone esperado; token cru SOMENTE no caso de borda sem telefone (RF-30/31). Sem
   ATIVAR na UX. (RF-18, RF-20, RF-29)
3. Correlação por telefone normalizado E.164 (fonte única internal/platform/phone), seleção da
   sessão PAID mais recente por paidAt. (RF-07, RF-21, RF-22, RF-23)
4. Janela de ativação = 24h a partir de paidAt, aplicada NOS DOIS caminhos (telefone e token);
   expiresAt do checkout e o job de expiração permanecem intactos. (RF-10, ADR-002)
5. Ativação idempotente e segura sob concorrência: UpdateMarkConsumed checa RowsAffected==0 ->
   AlreadyActive; dedup por WAMID; reentrega não duplica efeito nem boas-vindas. (RF-25, RF-26, RF-27)
6. Boas-vindas DESACOPLADAS: WelcomeConsumer idempotente em onboarding.subscription_bound envia as
   2 mensagens de texto livre (welcome_activated + onboarding_intro / "Vamos começar?") dentro da
   janela de 24h; sem botão interativo (client Meta só faz text/template). (RF-27, RF-28, RF-32, RF-33)
7. No-match (número sem sessão PAID): responder orientação com THROTTLE durável por telefone
   (1 resposta por janela) + métrica/audit; nunca ativar conta errada, nunca silenciar. (RF-24)
8. Evento onboarding.activation.attempted.v1 é sem usuário => deve entrar em noUserEventAllowlist.
9. E-mail aponta para ${ONBOARDING_ACTIVATION_PAGE_URL}/ativar?token=...; supressão de e-mail quando
   a assinatura já está vinculada (recompra). Token nunca exposto ao usuário. (RF-06, RF-11..14, RF-29)
10. Sem Telegram em lugar nenhum: backend nunca retorna telegram_deep_link; landing remove o botão
    e o teste correspondente. (RF-17, RF-19)
11. Timestamps da jornada (RF-35): email_sent_at/activation_started_at no servidor (set-once-if-null);
    page_opened_at/whatsapp_opened_at via beacon dedicado POST /tokens/{token}/opened — NUNCA escrita
    no GET /state.
12. Jornada termina na boas-vindas (RF-34). NÃO implemente onboarding, cartão, salário, objetivos,
    lançamentos, nem um agente financeiro. Limitação conhecida e ACEITA: após a ativação a próxima
    mensagem roteia ao weather-agent existente — não tente trocar/registrar agente.

ESCOPO E FRONTEIRAS
- Backend Go em /Users/jailtonjunior/Git/mecontrola. A tarefa 10.0 também altera a landing em
  /Users/jailtonjunior/Git/mecontrola-landingpage (repo separado, Astro/TS) — trate como tarefa
  isolada; não misture commits dos dois repos.
- Cutover transicional: o consumer aceita token no texto (com/sem prefixo ATIVAR) enquanto tokens
  antigos não expiram; o dispatcher já remove o ramo ATIVAR. Não faça hard cutover.
- Deploy implícito: backend primeiro (inclui o beacon), landing depois — a página consome
  wa_me_url as-is, então não quebra.

PROIBIÇÕES
- Não altere PRD/techspec/ADRs/tasks. Não reabra decisões já fechadas.
- Não adicione comentários em .go de produção. Não use Clock/now injetado. Não use switch
  intent.Kind para roteamento. Não coloque LLM no kernel nem em tool.
- Não marque done sem testes executados e evidência. Não re-execute tarefa não-done.

SAÍDA
- Ao final, _orchestration_report.md com snapshot inicial vs final, waves, tarefas executadas/
  puladas e próximos passos. Status final: done (todas done) | partial | failed | needs_input.
```

---

## Critérios de aceite global (gate de conclusão do run)

O run só é `done` quando, além de todas as 10 tarefas `done`:

- `ai-spec check-spec-drift .specs/prd-ativacao-whatsapp` retorna sem drift e sem RF faltante.
- Todos os gates hard verificáveis passam (executar como conferência final, não como substituto dos testes):
  - Zero comentários: `grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" internal/ configs/ cmd/ | grep -Ev "(//go:|//nolint:|// Code generated)"` → vazio.
  - Sem SQL em adapter: gate de `R-ADAPTER-001.2` → vazio.
  - Kernel/agent: sem `case intent.Kind`, sem LLM no kernel.
  - Métricas sem `user_id`/telefone/`category_id` como label.
- A jornada e2e (tarefa 10.0) prova: paga → PAID (sem ativar) → `/state` retorna "Oi" → inbound
  não-vinculado → ativado → `subscription_bound` → 2 boas-vindas; e idempotência de reentrega.
- Playwright da landing verde, sem Telegram.

## Observações de operação

- A primitiva `Agent` do Claude Code roda in-process: no timeout o orquestrador apenas descarta o
  YAML tardio (soft-discard) — o subagent continua até completar. Ajuste `AI_TASK_TIMEOUT_SECONDS`
  se necessário.
- Tarefas paralelizáveis declaradas: fundação `1.0/2.0/3.0`; `4.0` com `8.0`; `8.0` com `9.0`. O
  paralelismo só é aplicado se o tool suportar spawn nativo; caso contrário, degrada para sequencial
  (registrar no report).
