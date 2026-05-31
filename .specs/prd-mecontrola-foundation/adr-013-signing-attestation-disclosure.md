# ADR-013 — Signing & attestation: GHCR + cosign + SLSA + SECURITY.md + gitsign

## Metadados

- **Título:** Supply chain attestation completa: image signing via cosign (keyless OIDC), commit/tag signing via gitsign, SECURITY.md disclosure policy
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §RF-07, §D-27, §D-30, §CS-27, §CS-30](./prd.md), [techspec §Plano de Rollout M5](./techspec.md), [ADR-011 (Docker)](./adr-011-docker-fly-deploy.md), [ADR-012 (supply chain)](./adr-012-supply-chain-scan-deps.md), [R-SEC-001](../../.agents/skills/agent-governance/references/security.md)

## Contexto

Foundation tem cadeia parcial: Docker distroless (ADR-011) + scanners govulncheck/trivy + SBOM (ADR-012). Faltam três pilares de supply chain production-ready:

1. **Image signing**: imagem publicada precisa ser verificável pelo Fly (e por qualquer auditor) como originada do pipeline oficial.
2. **Commit/tag signing**: integridade do histórico git; sem signing, um atacante com push force-pushed pode alterar histórico sem deixar marca verificável.
3. **Disclosure policy**: canal claro para pesquisador externo reportar vulnerabilidade sem pública.

Sigstore (cosign + gitsign + Rekor) resolve os três sem chaves manuais — usa OIDC via GitHub para identidade.

## Decisão

### Image signing — `cosign` keyless via OIDC

- **Registry**: `ghcr.io/limateixeiratecnologia/mecontrola` (D-27).
- **Pipeline CD** (após `flyctl deploy`):
  1. `docker build` + push.
  2. `cosign sign --yes ghcr.io/limateixeiratecnologia/mecontrola@<digest>` — assinatura registrada em Rekor (transparency log público, mas só hash visível para repo private).
  3. `cosign attest --predicate sbom.spdx.json --type spdxjson ghcr.io/...@<digest>` — atesta SBOM gerado pelo trivy.
  4. `cosign attest --predicate provenance.json --type slsaprovenance ghcr.io/...@<digest>` — atesta provenance SLSA L3 (GitHub Actions é builder verificável).
- **Verification em runbook**:
  ```
  cosign verify \
    --certificate-identity-regexp '^https://github.com/LimaTeixeiraTecnologia/mecontrola/' \
    --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
    ghcr.io/limateixeiratecnologia/mecontrola:<sha>
  ```
- **Fly**: configurar policy de admission (pré-deploy hook ou runbook manual no MVP) para verificar assinatura antes de aceitar imagem nova.

### Commit/tag signing — `gitsign` (Sigstore)

- **Substitui GPG tradicional**: dev não precisa gerenciar chave local; identidade vem do OIDC (GitHub).
- **Setup local**: `task setup` instala `gitsign` + configura `git config --global gpg.x509.program gitsign` + `git config --global gpg.format x509` + `git config --global commit.gpgsign true`.
- **Tag signing**: `git tag -s v0.x.y` usa gitsign.
- **Branch protection**: GitHub `main` exige "Require signed commits" + "Require signed pushes" (quando GA).
- **Verificação**: `git log --show-signature` ou `gitsign verify <commit>`; CI valida em job dedicado que todo commit do PR tem assinatura válida.

### Disclosure — `SECURITY.md`

- Localização: raiz do repo.
- Conteúdo mínimo:
  - **Canal de disclosure**: email com PGP key (ou Sigstore-encrypted) — recomendado usar `security@<dominio>` quando dominio estiver registrado; no MVP, `@JailtonJunior94` direto via email privado.
  - **SLA**: resposta inicial em 7 dias; correção orientada por severidade (Critical: 7d; High: 30d; Medium: 90d).
  - **Escopo**: o que está in-scope (binário, deps, infra Fly) e out-of-scope (orchestrator, devkit-go — projetos próprios).
  - **Hall of fame**: opcional; declarar política de crédito público.
  - **Safe harbor**: declarar que pesquisador agindo de boa fé não será processado.
- **Reconhecimento**: link no README + label `security` em issues + template de issue privada se GitHub Security Advisories ativado.

## Alternativas Consideradas

### Image signing

1. **GPG tradicional**: chave precisa ser gerada e gerenciada; secret no CI; perda de chave invalida histórico. cosign keyless elimina chave gerenciada.
2. **Notary v2 / DCT**: padrão Docker mas adoção menor; Sigstore vence em ecossistema.
3. **Sem signing**: aceita supply chain confiando só no registry. Inaceitável para production-ready.

### Commit signing

1. **GPG tradicional**: chave local + upload no GitHub; funcional mas friction alto (key revocation, perda de chave, signing em CI difícil).
2. **SSH signing (git ≥ 2.34)**: nativo, sem deps; mas sem transparency log; menos auditável.
3. **Sem signing**: histórico modificável sem trail; rejeitado.

### Disclosure

1. **Sem SECURITY.md**: GitHub mostra placeholder pedindo um. UX ruim para pesquisador.
2. **GitHub Security Advisories apenas**: bom para repos public; em private requer pesquisador ter acesso para abrir advisory — não funciona para reporters externos.
3. **Bug bounty (HackerOne, Bugcrowd)**: prematuro para foundation; entra quando produto for público.

## Consequências

### Benefícios Esperados

- **Supply chain L3 SLSA**: GitHub Actions é builder verificável; atestados provenance + SBOM completam o triângulo.
- **Zero chaves gerenciadas**: cosign + gitsign keyless via OIDC ⇒ sem revogação manual, sem perda de chave.
- **Rekor transparency**: qualquer auditor pode verificar histórico de assinaturas (mesmo private — só hash exposto).
- **SECURITY.md**: declara compromisso público com disclosure responsável.
- **Branch protection signed commits**: previne `git push --force` malicioso (não consegue forjar assinatura).

### Trade-offs e Custos

- **Setup local de gitsign**: 1 passo extra em `task setup`; primeira vez exige OAuth GitHub no browser (~30s).
- **CI tempo**: +30s por job para `cosign sign` + `cosign attest`.
- **Friction em PR**: dev sem gitsign configurado tem commit rejeitado pelo branch protection — necessário onboarding bem documentado.
- **Dependência de Sigstore disponibilidade**: rekor.sigstore.dev e fulcio.sigstore.dev são SPOFs. Mitigação: Sigstore tem SLA público; fallback documentado.

### Riscos e Mitigações

- **Risco**: Sigstore Fulcio CA expira certificado curto (10min); commit em laptop offline não consegue assinar.
  - **Mitigação**: documentar requisito de rede; dev offline usa `--allow-empty-commit --signoff` como fallback temporário com revisão.
- **Risco**: `cosign verify` no Fly não está integrado nativamente; admission policy é manual.
  - **Mitigação**: runbook manual pré-deploy; investigar Fly admission webhook quando GA.
- **Risco**: SECURITY.md com email pessoal vaza endpoint.
  - **Mitigação**: usar alias forwarder (Fastmail/SimpleLogin) ou setup `security@` quando domínio existir.
- **Risco**: dev sem permissão de admin no repo não pode configurar branch protection.
  - **Mitigação**: `@JailtonJunior94` (CODEOWNER) configura uma vez; doc no runbook.
- **Risco**: gitsign requer Go 1.21+ instalado; pode conflitar com dev em ambiente antigo.
  - **Mitigação**: Go 1.26.3 é pré-requisito do projeto (D-01); sem conflito.

## Plano de Implementação

1. `taskfiles/security.yml`: tarefa `task security:sign-image` (cosign sign + attest) e `task security:verify-image`.
2. `taskfiles/setup.yml`: tarefa `task setup` instala gitsign + configura `git config` global do projeto.
3. `.github/workflows/cd.yml`: adicionar steps `cosign sign` + `cosign attest --type spdxjson` + `cosign attest --type slsaprovenance` pós-push.
4. `.github/workflows/ci.yml`: job `verify-signatures` que valida assinatura de todos os commits do PR via `gitsign verify`.
5. Criar `SECURITY.md` na raiz conforme spec.
6. Configurar branch protection na `main` (manual via GitHub UI): "Require signed commits" + "Require signed pushes" (quando GA).
7. README: seção "Verificar assinatura" com comando `cosign verify` + link para `SECURITY.md`.
8. Runbook "Configurar gitsign localmente" (parte do onboarding).
9. Runbook "Pesquisador reporta CVE": passos de triage + comunicação.

## Monitoramento e Validação

- Métrica CI: `signing_duration_seconds` (cosign sign + attest) — cap 60s.
- Alerta: falha de assinatura em CD bloqueia deploy.
- Auditoria mensal: sample de 5 commits + 1 imagem → verificar assinatura manual.
- SECURITY.md tem link no README e badge no topo.

## Impacto em Documentação e Operação

- README: badges (signed image, signed commits, SBOM available); seção "Security".
- Onboarding: passo "Configurar gitsign" + "Verificar assinatura de imagem".
- Runbook "Disclosure de CVE recebida": triage + comunicação + patch.
- Runbook "Rotacionar identidade": como dev novo configura gitsign na primeira vez.
- ADRs relacionados: 011 (Docker), 012 (scan/Dependabot), 013 (signing).

## Revisão Futura

- Revisitar admission policy do Fly quando recurso GA.
- Revisitar para Sigstore Hardware Token Module se compliance interna exigir chave de longa duração.
- Revisitar SECURITY.md quando produto for público (talvez bug bounty).
- Revisitar quando equipe crescer >3 pessoas (ajustar processo de disclosure).
