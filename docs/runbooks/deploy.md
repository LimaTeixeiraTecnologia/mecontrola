# Runbook: Deploy MeControla via Fly.io

**Referências:** ADR-011 (Docker + Fly), ADR-012 (supply chain scan), ADR-013 (cosign signing)

## Visão Geral

O deploy do MeControla usa um pipeline em 5 etapas:

```
task build:docker:build  →  task security:image-scan  →  task security:sbom  →  task security:sign-image  →  flyctl deploy
```

Produção roda **2 processes** no Fly.io:
- `app` — servidor HTTP (`mecontrola server`); recebe tráfego na porta 8080
- `worker` — processo de background (`mecontrola worker`); sem ingress HTTP

---

## Pré-requisitos

| Ferramenta | Instalação |
|---|---|
| `docker` | https://docs.docker.com/get-docker/ |
| `flyctl` | `brew install flyctl` ou https://fly.io/docs/hands-on/install-flyctl/ |
| `trivy` | `brew install trivy` ou https://aquasecurity.github.io/trivy/latest/getting-started/installation/ |
| `cosign` | `brew install cosign` ou https://docs.sigstore.dev/cosign/installation/ |
| `task` | `brew install go-task` |

Configurar credenciais:
```sh
flyctl auth login
docker login ghcr.io -u <github-user> -p <github-pat>
```

---

## Fluxo Completo de Deploy

### 1. Build da imagem Docker

```sh
# Build local (tag dev)
task build:docker:build

# Build com tag de release
task build:docker:build IMAGE_TAG=$(git describe --tags --always)
```

O script `taskfiles/scripts/build-image.sh` valida:
- `User: nonroot` — UID 65532 (ADR-011)
- Tamanho ≤ 30 MB

### 2. Scan de vulnerabilidades da imagem

```sh
task security:image-scan IMAGE_SHA=<sha-ou-tag>
```

- Exit 1 se CVE HIGH ou CRITICAL encontrada
- Suprimir CVE conhecida: adicionar entrada documentada em `.trivyignore` (ver cabeçalho do arquivo)

### 3. Gerar SBOM

```sh
task security:sbom IMAGE_SHA=<sha-ou-tag>
# Gera: sbom.spdx.json
```

### 4. Assinar imagem (CI only — requer OIDC)

```sh
# Somente no GitHub Actions com id-token: write
task security:sign-image IMAGE_SHA=<sha> SBOM_FILE=sbom.spdx.json PROVENANCE_FILE=provenance.json
```

Executa:
1. `cosign sign` — assina a imagem e registra no Rekor transparency log
2. `cosign attest --type spdxjson` — atesta o SBOM
3. `cosign attest --type slsaprovenance` — atesta a provenance SLSA L3

### 5. Deploy no Fly.io

```sh
# Deploy com rolling update (não derruba app e worker simultaneamente)
flyctl deploy --strategy rolling --app mecontrola
```

Aguardar que ambos os processes subam:
```sh
flyctl status -a mecontrola
# Esperado: "started" para app e worker
```

---

## Verificar Assinatura da Imagem

```sh
task security:verify-image IMAGE_SHA=<sha-ou-tag>
```

Ou diretamente:
```sh
cosign verify \
  --certificate-identity-regexp '^https://github\.com/LimaTeixeiraTecnologia/mecontrola/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/limateixeiratecnologia/mecontrola:<sha-ou-tag>
```

---

## Smoke Test Pós-Deploy

```sh
# Verificar ambos os processes ativos
flyctl status -a mecontrola | grep -E "(app|worker)"

# Testar health endpoint do app
curl -s https://mecontrola.fly.dev/ready | jq .

# Verificar logs de startup
flyctl logs -a mecontrola -p app   # HTTP server started
flyctl logs -a mecontrola -p worker  # worker idle, no jobs registered
```

---

## Scale de Processes

```sh
# Escalar individualmente
fly scale count app=2 worker=1

# Ver contagem atual
fly status -a mecontrola
```

---

## Debug sem Shell

A imagem distroless **não tem shell**. Opções de debug:

1. **Logs** (recomendado):
   ```sh
   flyctl logs -a mecontrola -p app
   flyctl logs -a mecontrola -p worker
   ```

2. **OTel traces/metrics** — via Grafana Cloud (configurado em `O11yConfig`)

3. **Help do binário** (disponível mesmo sem shell):
   ```sh
   fly ssh console -a mecontrola -C '/mecontrola --help'
   ```

4. **Imagem temporária com shell** (emergência apenas):
   ```sh
   # Build temporário com alpine em vez de distroless
   docker build --build-arg BASE=alpine --tag mecontrola:debug .
   flyctl deploy --image mecontrola:debug --strategy immediate
   # Reverter imediatamente após diagnóstico
   ```

---

## Rollback

```sh
# Listar releases
flyctl releases -a mecontrola

# Rollback para release anterior
flyctl deploy --image <image-da-release-anterior>
```

---

## Alertas e Monitoramento

| Condição | Ação |
|---|---|
| Billing Fly ≥ R$ 50/mês | Revisar scale + usage |
| `process=worker, state≠started` por >5min | Verificar logs + restart |
| CVE HIGH/CRITICAL em scan | Criar entrada em `.trivyignore` + abrir issue urgente |
| `cosign verify` falha | Não fazer deploy; investigar pipeline CI |

---

## Referências

- [ADR-011: Docker distroless + Fly 2 processes](./../../../.specs/prd-mecontrola-foundation/adr-011-docker-fly-deploy.md)
- [ADR-012: Supply chain scan + Dependabot](./../../../.specs/prd-mecontrola-foundation/adr-012-supply-chain-scan-deps.md)
- [ADR-013: cosign + gitsign + SECURITY.md](./../../../.specs/prd-mecontrola-foundation/adr-013-signing-attestation-disclosure.md)
- [SECURITY.md](./../../SECURITY.md)
