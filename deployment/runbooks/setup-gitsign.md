# Runbook: Configurar gitsign para Novo Desenvolvedor

**Referências:** ADR-013 (gitsign + Sigstore), ADR-014 (tool pinning)

## Visão Geral

O MeControla usa `gitsign` para assinar commits via Sigstore (keyless OIDC).
Commits assinados são obrigatórios em `main` (branch protection).

## Pré-requisitos

- Conta no GitHub com acesso ao repositório.
- Go 1.26+ instalado.
- `task` instalado (`brew install go-task`).

## Instalação Automática

O `task setup` faz a instalação completa:

```sh
git clone https://github.com/LimaTeixeiraTecnologia/mecontrola.git
cd mecontrola
task setup
```

## Instalação Manual

### 1. Instalar gitsign

```sh
# macOS
brew install gitsign

# Linux / Go install
GITSIGN_VERSION=$(grep 'GITSIGN_VERSION:' taskfiles/vars.yml | awk '{print $2}' | tr -d "'\"")
go install github.com/sigstore/gitsign@${GITSIGN_VERSION}
```

### 2. Configurar git para usar gitsign

```sh
git config gpg.format x509
git config gpg.x509.program gitsign
git config gitsign.connectorID https://github.com/login/oauth
```

Para configuração global (todos os repositórios):

```sh
git config --global gpg.format x509
git config --global gpg.x509.program gitsign
git config --global gitsign.connectorID https://github.com/login/oauth
```

### 3. Testar assinatura

```sh
git commit --allow-empty -m "test: verificar gitsign"
```

Um browser abrirá para autenticação OIDC com o GitHub.

### 4. Verificar a assinatura

```sh
git log --show-signature -1
```

Saída esperada:
```
commit <sha>
Good "git" signature for <email> with issuer "https://github.com/login/oauth"
...
```

## Configuração em CI (GitHub Actions)

No CI, os commits são assinados automaticamente pelo workflow com OIDC.
Não é necessária configuração manual.

## Solução de Problemas

| Problema | Solução |
|---|---|
| `gitsign: command not found` | Verificar `$PATH`; reinstalar com `brew install gitsign` |
| Browser não abre | `gitsign --version`; verificar `gpg.x509.program` no git config |
| `bad signature` no `git log` | Reconfigurar: `git config gpg.format x509` |
| Commit rejeitado na branch protection | Verificar se `gitsign` está configurado + commit re-assinado |

## Referências

- gitsign: https://github.com/sigstore/gitsign
- Sigstore: https://www.sigstore.dev/
- [ADR-013: cosign + gitsign](../../.specs/prd-mecontrola-foundation/adr-013-signing-attestation-disclosure.md)
