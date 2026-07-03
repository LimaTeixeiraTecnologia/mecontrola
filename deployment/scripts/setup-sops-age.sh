#!/usr/bin/env bash
set -euo pipefail

# setup-sops-age.sh — Gera par de chaves age e configura SOPS para MeControla.
#
# Uso:
#   bash deployment/scripts/setup-sops-age.sh
#
# Saída:
#   - Atualiza .sops.yaml com a chave pública age.
#   - Gera key.txt no diretório atual (CHAVE PRIVADA — guarde em cofre/GitHub secret).
#   - Cria deployment/config/prod.secrets.env a partir do template.
#
# Após rodar:
#   1. Copie o conteúdo de key.txt para o GitHub secret AGE_PRIVATE_KEY.
#   2. Preencha deployment/config/prod.secrets.env com os secrets reais.
#   3. Criptografe: sops --encrypt --in-place deployment/config/prod.secrets.env
#   4. Delete key.txt local após guardar a chave privada com segurança.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SOPS_FILE="${REPO_ROOT}/.sops.yaml"
TEMPLATE="${REPO_ROOT}/deployment/config/prod.secrets.env.example"
SECRETS_FILE="${REPO_ROOT}/deployment/config/prod.secrets.env"

log() { echo "[setup-sops-age] $*"; }
die() { echo "[setup-sops-age] ERRO: $*" >&2; exit 1; }

command -v age-keygen >/dev/null || die "age-keygen não encontrado. Instale: https://age-encryption.org"
command -v sops >/dev/null || die "sops não encontrado. Instale: https://github.com/getsops/sops"

[[ -f "$TEMPLATE" ]] || die "template não encontrado: $TEMPLATE"

log "Gerando par de chaves age em key.txt (CHAVE PRIVADA)"
age-keygen -o key.txt

PUBLIC_KEY=$(grep '^# public key: ' key.txt | sed 's/# public key: //')
[[ -n "$PUBLIC_KEY" ]] || die "não foi possível extrair a chave pública de key.txt"

log "Chave pública: $PUBLIC_KEY"

log "Atualizando .sops.yaml"
sed -i.bak "s/PLACEHOLDER_AGE_RECIPIENT_CHANGE_ME/${PUBLIC_KEY}/" "$SOPS_FILE"
rm -f "${SOPS_FILE}.bak"

if [[ ! -f "$SECRETS_FILE" ]]; then
  log "Criando $SECRETS_FILE a partir do template"
  cp "$TEMPLATE" "$SECRETS_FILE"
else
  log "$SECRETS_FILE já existe — mantendo"
fi

log ""
log "============================================================"
log "PRÓXIMOS PASSOS:"
log "  1. Guarde key.txt em local seguro (1Password, Bitwarden)."
log "  2. Copie o conteúdo de key.txt para o GitHub secret"
log "     AGE_PRIVATE_KEY nos environments staging/production."
log "  3. Edite $SECRETS_FILE com os secrets reais."
log "  4. Criptografe: sops --encrypt --in-place $SECRETS_FILE"
log "  5. Delete key.txt local após confirmar que a chave privada"
log "     está guardada com segurança."
log "============================================================"
