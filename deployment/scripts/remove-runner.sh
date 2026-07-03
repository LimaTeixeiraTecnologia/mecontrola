#!/usr/bin/env bash
set -euo pipefail

RUNNER_DIR="${RUNNER_DIR:-/home/github-runner/actions-runner}"
RUNNER_USER="${RUNNER_USER:-github-runner}"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }

if [[ $EUID -ne 0 ]]; then
  log "ERRO: execute como root ou com sudo"
  exit 1
fi

log "Parando e desabilitando servico do runner"
SERVICE_UNIT=$(systemctl list-units --full --all --no-legend \
  | awk '{print $1}' \
  | grep -E '^actions\.runner\.' \
  | head -n1 || true)
if [[ -n "$SERVICE_UNIT" ]]; then
  log "Unidade encontrada: $SERVICE_UNIT"
  systemctl stop "$SERVICE_UNIT" 2>/dev/null || true
  systemctl disable "$SERVICE_UNIT" 2>/dev/null || true
else
  log "Nenhuma unidade systemd de runner encontrada; pulando stop/disable"
fi

log "Desregistrando runner no GitHub"
if [[ -f "${RUNNER_DIR}/config.sh" ]]; then
  if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    log "AVISO: GITHUB_TOKEN nao definido — desregistro remoto pulado (remover manualmente em Settings > Actions > Runners)"
  else
    TOKEN_RESPONSE=$(curl -fsSL -X POST \
      -H "Authorization: token ${GITHUB_TOKEN}" \
      -H "Accept: application/vnd.github+json" \
      "${GITHUB_API_URL:-https://api.github.com}/repos/${GITHUB_REPOSITORY}/actions/runners/remove-token")
    REMOVE_TOKEN=$(echo "$TOKEN_RESPONSE" | grep -o '"token":"[^"]*"' | cut -d'"' -f4 || true)
    if [[ -n "$REMOVE_TOKEN" ]]; then
      cd "$RUNNER_DIR"
      sudo -u "$RUNNER_USER" ./config.sh remove --token "$REMOVE_TOKEN" || true
      log "Runner desregistrado no GitHub"
    else
      log "AVISO: nao foi possivel obter remove-token; remover manualmente no GitHub"
    fi
  fi
else
  log "Diretorio $RUNNER_DIR nao encontrado; desregistro pulado"
fi

log "Removendo diretorio do runner: /home/${RUNNER_USER}"
rm -rf "/home/${RUNNER_USER}" || true

log "Removendo usuario ${RUNNER_USER}"
if id "$RUNNER_USER" &>/dev/null; then
  userdel -r "$RUNNER_USER" 2>/dev/null || userdel "$RUNNER_USER" 2>/dev/null || true
  log "Usuario $RUNNER_USER removido"
else
  log "Usuario $RUNNER_USER ja inexistente"
fi

log "Limpando build cache do Docker"
docker builder prune -af || true

log "Limpando imagens Docker nao utilizadas"
docker image prune -af || true

log "Espaco em disco apos limpeza:"
df -h /

log "Remocao do runner concluida"
