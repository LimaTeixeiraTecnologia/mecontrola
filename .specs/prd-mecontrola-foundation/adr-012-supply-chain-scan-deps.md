# ADR-012 — Supply chain: `govulncheck` + `trivy fs` + Dependabot grupado

## Metadados

- **Título:** Stack de segurança de supply chain com `govulncheck`, `trivy fs/image`, SBOM e Dependabot configurado
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §D-25, §D-26, §CS-25, §CS-26](./prd.md), [techspec §Plano de Rollout M5](./techspec.md), [R-SEC-001](../../.agents/skills/agent-governance/references/security.md), [ADR-011 (Docker)](./adr-011-docker-fly-deploy.md)

## Contexto

A foundation precisa fechar o triângulo supply-chain desde o primeiro commit:
1. **Detectar CVE em dependências Go** — vetor mais comum de comprometimento.
2. **Detectar CVE em imagem Docker** — base image, layers, secrets em commit.
3. **Atualizar dependências automaticamente** — dep stale = vetor de CVE; updates manuais são esquecidos.

Skill `taskfile-production` mandou a tarefa `task security:vulncheck`; ficou em aberto qual ferramenta usar e como integrar Dependabot.

## Decisão

### Scan de vulnerabilidades — dois scanners complementares

**1. `govulncheck` (Google oficial)**
- Cobre CVE em dependências Go com **call-graph analysis** ⇒ só reporta CVE em código realmente executável.
- Roda em `task security:vulncheck` + job dedicado no CI.
- Versão: latest stable (pinada via release pin em `.github/workflows/ci.yml`).

**2. `trivy fs` (Aqua Security)**
- Cobre Dockerfile, `go.sum`, secrets em commit, license issues.
- Cobre `trivy image` para varrer a imagem final pós-build.
- Roda em `task security:vulncheck` + job CD pós-build.
- Versão: latest stable (pinada via release pin).

**3. SBOM**
- Gerado por `trivy image --format spdx-json` no CD.
- Anexado ao GitHub release como artefato.
- Re-validado em cada deploy.

**4. `.trivyignore`**
- Vazio no primeiro commit.
- Cada supressão exige: data, CVE-ID, justificativa, data de revisão.

### Dependabot — config nativa do GitHub

**`.github/dependabot.yml`**:

| Ecosystem | Grupo | Schedule | Auto-merge |
| --- | --- | --- | --- |
| `gomod` | 1 grupo `go-deps` para minor/patch; major separado | Semanal (terça 06:00 UTC) | minor/patch após CI verde + CODEOWNER review |
| `github-actions` | 1 grupo `ci-actions` | Semanal (terça 06:00 UTC) | minor/patch após CI verde + CODEOWNER review |
| `docker` | 1 grupo `docker-base` | Semanal (terça 06:00 UTC) | só minor; major nunca auto |

- Limite de 10 PRs abertos simultâneos.
- PRs em PT-BR (conforme orchestrator).
- Auto-merge via GitHub Actions workflow dedicado (`auto-merge.yml`) que valida labels + CI.

## Alternativas Consideradas

### Scanner

1. **Apenas `govulncheck`**: cobre Go bem, mas Dockerfile + secrets em commit ficam descobertos. Lacuna inaceitável.
2. **Apenas `trivy`**: cobre supply chain ampla, mas reporta muito falso positivo em deps Go não-executáveis. Ruído reduz adoção.
3. **Snyk SaaS**: comprehensive, dashboard rico, mas pago (>$0/mês). Viola "sem custos além do orçado".
4. **Grype + Syft**: alternativa open-source robusta. Trivy é mais consolidado e tem SBOM nativo; ganho marginal.

### Update bot

1. **Renovate Bot**: mais flexível (semantic commits, agendamento fino, monorepo support). Requer GitHub App + token; overhead operacional maior; valor adicional baixo para foundation simples.
2. **Sem bot, updates manuais**: deps ficam stale; CVE em dep não atualizada por meses; viola production-ready.
3. **gomod-tidy bot custom**: complexidade injustificada; Dependabot oficial cobre o caso.

## Consequências

### Benefícios Esperados

- **Cobertura completa**: código Go (govulncheck) + container/secrets/license (trivy) = sem lacuna.
- **Baixo ruído**: govulncheck call-graph reduz falso positivo a quase zero; só CVE executável.
- **SBOM auditável**: trivy SBOM no release ⇒ resposta rápida a query "qual versão de X estava em prod no dia Y?".
- **Updates contínuos**: Dependabot terça 06:00 UTC + auto-merge ⇒ deps atualizadas em ≤ 1 semana.
- **Custo zero**: tudo open-source ou nativo do GitHub.

### Trade-offs e Custos

- 2 scanners = 2 atualizações para manter pinadas (mitigado por Dependabot cobrindo `github-actions`).
- Auto-merge requer disciplina (review CODEOWNER ainda obrigatório, mas pode aprovar em batch).
- SBOM aumenta tempo de CD em ~30s (aceitável).

### Riscos e Mitigações

- **Risco**: CVE crítica reportada num domingo bloqueia merge na segunda.
  - **Mitigação**: política de supressão emergencial em `.trivyignore` documentada; oncall pode adicionar entrada com `expires: <data>` e revisitar em até 7 dias.
- **Risco**: Dependabot major bump introduz breaking change não detectado pelo CI.
  - **Mitigação**: major nunca auto-merged; CODEOWNER (`@JailtonJunior94`) revisa manualmente; integration test cobre caminho crítico (testcontainers).
- **Risco**: trivy reporta CVE em base distroless que Google ainda não corrigiu.
  - **Mitigação**: supressão temporária + bump quando Google publicar; SBOM permite rastrear.
- **Risco**: govulncheck atrasado para CVE novo (database update lag).
  - **Mitigação**: `task security:vulncheck` baixa db sempre; cron no CI roda 1×/dia além dos PRs.
- **Risco**: PR de Dependabot floda o repo.
  - **Mitigação**: grupos limitam a 3 PRs/semana (gomod, github-actions, docker); cap de 10 PRs simultâneos.

## Plano de Implementação

1. `taskfiles/security.yml`:
   - `task security:vulncheck` chama `govulncheck ./...` + `trivy fs --severity HIGH,CRITICAL --exit-code 1 .`.
   - `task security:image-scan` chama `trivy image --severity HIGH,CRITICAL --exit-code 1 <image>` (usado no CD).
   - `task security:sbom` chama `trivy image --format spdx-json --output sbom.json <image>`.
2. `.github/workflows/ci.yml`: job `security` rodando `task security:vulncheck`; cron diário às 06:00 UTC além de PRs.
3. `.github/workflows/cd.yml`: pós-build, `task security:image-scan` + `task security:sbom`; SBOM anexada ao release.
4. `.github/dependabot.yml`: grupos conforme tabela acima.
5. `.github/workflows/auto-merge.yml`: workflow simples que dá merge em PRs do Dependabot com label `dependencies` + `auto-merge` + CI verde + 1 review.
6. `.trivyignore`: vazio, com header documentando processo de supressão.
7. README: seção "Segurança" com fluxo de scan + Dependabot.

## Monitoramento e Validação

- Métrica CI: `security_scan_duration_seconds` (govulncheck + trivy) — cap 60s.
- Job de cron diário às 06:00 UTC executa scan mesmo sem PR (pega CVE novo em dep não tocada).
- Dashboard manual: contagem de PRs Dependabot abertos > 5 dispara revisão.
- Alerta GitHub: CVE HIGH/CRITICAL detectada via Dependabot Security Update.

## Impacto em Documentação e Operação

- README: "Segurança" + "Como contribuir com dep update".
- Runbook "CVE crítica detectada em prod": passo a passo (suprimir temporário + bump emergencial + redeploy).
- Onboarding: incluir esta ADR + ADR-011 + ADR-009.

## Revisão Futura

- Revisitar para adicionar `secrets-scanner` (truffleHog/gitleaks) se incidente de secret em commit ocorrer.
- Revisitar Snyk se compliance interna exigir (não aplicável no MVP).
- Revisitar grouping do Dependabot quando volume cruzar 20 PRs/mês.
