# ADR-008 — HTTP middleware stack: defaults do `devkit-go` + CORS estrito + OTel HTTP

## Metadados

- **Título:** Adoção dos defaults do `devkit-go/pkg/http_server` com CORS allowlist e instrumentação OTel
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §RNF-004, §RF-01](./prd.md), [techspec §Pontos de Integração, §Estratégia de Erros](./techspec.md), [security-app.md §HTTP](../../.agents/skills/agent-governance/references/security-app.md), [R-SEC-001](../../.agents/skills/agent-governance/references/security.md)

## Contexto

`devkit-go/pkg/http_server` entrega middleware stack production-ready: RequestID, Recovery, Timeout, BodyLimit, SecurityHeaders, Problem Details. A foundation precisa decidir o subset a habilitar + política de CORS + integração com OTel HTTP instrument.

security-app.md §HTTP exige: CORS allowlist (sem `*`), rate-limit em endpoints públicos, headers de segurança equivalentes a helmet.

## Decisão

**Habilitar todos os middlewares default do `devkit-go/pkg/http_server`** no `internal/infrastructure/http/server.go`:
- **RequestID** (propaga via header `X-Request-ID` + context).
- **Recovery** (captura panic, log estruturado, retorna 500 com ProblemDetails).
- **Timeout** (hard cap por handler; default 25 s alinhado ao timeout do OpenAI no discovery).
- **BodyLimit** (default 1 MiB; revisado por endpoint).
- **SecurityHeaders** (HSTS, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Content-Security-Policy mínimo).
- **CORS estrito** com allowlist via env `CORS_ALLOWED_ORIGINS` (lista separada por vírgula); default vazio em produção ⇒ rejeita qualquer origin. **Sem `*` jamais.**
- **OTel HTTP instrument** envolvendo o handler raiz.
- **Problem Details translator** chamando `internal/infrastructure/errors.ToProblemDetails`.

**Rate-limit não entra na foundation** — é Epic 05 do discovery, vai no PRD próprio.

## Alternativas Consideradas

1. **Subset mínimo (só RequestID + Recovery + OTel)**.
   - Vantagens: stack enxuto; menos configuração.
   - Desvantagens: perde CORS/timeout/body-limit/security-headers que security-app.md exige; não atende production-ready.
2. **Defaults + rate-limit Postgres (Epic 05)**.
   - Vantagens: adianta segurança real.
   - Desvantagens: viola fora-de-escopo do PRD; antecipa decisão de Epic 05 (token bucket schema, política de bypass admin); aumenta complexidade prematura.
3. **Stack custom sem devkit-go** (`chi.Router` direto).
   - Vantagens: controle total.
   - Desvantagens: reinventa wheel; devkit-go já tem testes e observabilidade integrados; viola decisão fundacional do PRD (`devkit-go` foundation obrigatória).

## Consequências

### Benefícios Esperados

- Production-ready desde o dia 1 (security headers + CORS estrito + recovery).
- Observabilidade automática em todos os endpoints (OTel HTTP).
- Erros consistentes via ProblemDetails (ADR-004).
- Conformidade total com security-app.md §HTTP e R-SEC-001.

### Trade-offs e Custos

- Configuração de CORS via env requer onboarding cuidadoso (default vazio = bloqueio total).
- Body-limit 1 MiB pode ser apertado para algum endpoint futuro (revisão por endpoint).
- Timeout 25 s herdado do discovery (limite OpenAI); pode ser longo para handlers leves (revisão por endpoint).

### Riscos e Mitigações

- **Risco:** CORS bloqueando legítimo em dev por esquecimento de env.
  - **Mitigação:** README com seção "primeira execução local"; `task setup` cria `.env.local` com CORS de dev pré-configurado.
- **Risco:** SecurityHeaders quebrando integração com terceiro (e.g. iframe).
  - **Mitigação:** Frame-Options default `DENY`; revisar por endpoint se admin UI precisar.
- **Risco:** Timeout 25 s atrapalhando teste de carga (k6) que deliberadamente lentifica.
  - **Mitigação:** k6 roda contra ambiente staging com env override.

## Plano de Implementação

1. `internal/infrastructure/http/server.go`: factory `NewServer(cfg, deps) (*http.Server, error)` compondo o devkit-go com middlewares listados.
2. `internal/infrastructure/http/middleware.go`: helpers para CORS allowlist parsing + validação.
3. `internal/infrastructure/http/health.go`: handlers `/health`, `/live`, `/ready` registrados antes do prefixo `/api`.
4. Integration test em `http_integration_test.go`: valida CORS allowlist (request com origin permitido vs não permitido); valida security headers presentes; valida `/ready` 200 ↔ 503 conforme DB.
5. CI carrega CORS de dev via env do workflow; prod via Fly secret.

## Monitoramento e Validação

- Métricas automáticas do devkit-go (`http.server.request.duration`, etc.).
- Métrica custom `http_cors_rejected_total{origin}` (counter).
- Métrica custom `http_security_header_missing_total` (sentinel; deveria ficar zero).
- Alerta: `http_cors_rejected_total` >100/min sustenta ataque potencial.

## Impacto em Documentação e Operação

- Runbook "Adicionar nova origin de CORS": editar Fly secret + redeploy.
- Runbook "Investigar 5xx": começar por log de recovery middleware + trace ID.
- README: seção "Configuração de CORS local".

## Revisão Futura

- Revisitar body-limit por endpoint quando upload de avatar/CSV entrar (PRDs futuros).
- Revisitar timeout quando webhook Meta entrar (Epic 06; provavelmente 5 s para ack).
- Revisitar quando rate-limit (Epic 05) for adicionado — alguns headers podem se sobrepor.
