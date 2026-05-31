# Runbook: Triage de CVE — Vulnerability Disclosure

**Referências:** ADR-012 (supply chain scan), ADR-013 (SECURITY.md + cosign)

## Quando Usar

- CVE recebida via `SECURITY.md` (responsible disclosure).
- `govulncheck` ou `trivy` reportam CVE HIGH/CRITICAL no CI.
- Notificação do GitHub Dependabot Advanced Security.

## Fluxo de Triage

```
Recebimento  →  Triagem  →  Avaliação de Impacto  →  Remediação  →  Divulgação
```

### 1. Receber e confirmar

CVEs chegam por:
- E-mail via `SECURITY.md` (endereço configurado no repositório).
- Pull request do Dependabot com label `security`.
- Alerta do GitHub Advanced Security.

Confirmar que a vulnerabilidade é real e afeta o código em uso.

### 2. Criar issue privada

No GitHub:
- `Security` → `Advisories` → `New draft security advisory`.
- Preencher: CVE ID, pacote afetado, versão vulnerável, CVSS score.
- Severidade: LOW / MEDIUM / HIGH / CRITICAL.

### 3. Avaliar impacto no MeControla

```sh
govulncheck ./...
trivy fs --severity HIGH,CRITICAL .
```

Verificar se a função vulnerável é realmente chamada (`govulncheck` analisa call graph).

### 4. Remediação

**Se há fix disponível:**
```sh
go get <pacote>@<versao-corrigida>
go mod tidy
```
Abrir PR com label `security` + referência ao CVE.

**Se não há fix:**
- Adicionar entrada documentada em `.trivyignore`:
  ```
  # CVE-XXXX-YYYY: <justificativa> — revisar em <data>
  CVE-XXXX-YYYY
  ```
- Criar issue de acompanhamento com prazo de reavaliação.

### 5. Verificar scan pós-remediação

```sh
govulncheck ./...
trivy fs --severity HIGH,CRITICAL .
```

Ambos devem passar sem reportar a CVE.

### 6. Divulgação (após fix em produção)

- Publicar o advisory no GitHub (Security → Advisories → Publish).
- Notificar o reporter com agradecimento e link para o advisory.
- Atualizar `SECURITY.md` se o processo precisar de ajuste.

## SLA de Resposta

| Severidade | Resposta inicial | Remediação |
|---|---|---|
| CRITICAL | 24h | 72h |
| HIGH | 48h | 7 dias |
| MEDIUM | 5 dias úteis | 30 dias |
| LOW | 10 dias úteis | 90 dias |

## Referências

- [SECURITY.md](../../SECURITY.md)
- [ADR-012: Supply chain scan](../../.specs/prd-mecontrola-foundation/adr-012-supply-chain-scan-deps.md)
- [ADR-013: Signing + disclosure](../../.specs/prd-mecontrola-foundation/adr-013-signing-attestation-disclosure.md)
