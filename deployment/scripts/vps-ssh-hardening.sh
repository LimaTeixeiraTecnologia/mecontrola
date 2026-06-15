#!/usr/bin/env bash
# deployment/scripts/vps-ssh-hardening.sh
#
# Aplica hardening de SSH: desabilita autenticacao por senha.
# Idempotente: so altera /etc/ssh/sshd_config se necessario.
#
# Uso:
#   ./vps-ssh-hardening.sh
#
# ATENCAO: o restart do sshd NAO e feito automaticamente.
# Apos confirmar a chave SSH funcionando, reinicie manualmente:
#   systemctl restart sshd
#
# Pre-requisitos:
#   - root ou sudo
#   - Chave SSH ja provisionada e testada antes de desabilitar senha

set -euo pipefail

SSHD_CONFIG="/etc/ssh/sshd_config"
BACKUP_FILE="${SSHD_CONFIG}.bak.$(date +%Y%m%d%H%M%S)"

echo "==> Verificando privilegios..."
if [[ "$EUID" -ne 0 ]]; then
  echo "ERRO: este script requer root (execute com sudo ou como root)." >&2
  exit 1
fi

echo "==> Verificando presenca de /etc/ssh/sshd_config..."
if [[ ! -f "$SSHD_CONFIG" ]]; then
  echo "ERRO: $SSHD_CONFIG nao encontrado." >&2
  exit 1
fi

PRE_HARDENING_SNAPSHOT="${SSHD_CONFIG}.pre-hardening"
if [[ ! -f "$PRE_HARDENING_SNAPSHOT" ]]; then
  echo "==> Criando snapshot pre-hardening em $PRE_HARDENING_SNAPSHOT (rollback canonico)..."
  cp "$SSHD_CONFIG" "$PRE_HARDENING_SNAPSHOT"
  echo "==> [OK] Snapshot canonico gravado em $PRE_HARDENING_SNAPSHOT"
else
  echo "==> [ja existe] Snapshot pre-hardening em $PRE_HARDENING_SNAPSHOT (preservado)"
fi

ensure_sshd_setting() {
  local key="$1"
  local value="$2"

  if grep -qE "^${key}[[:space:]]+${value}$" "$SSHD_CONFIG"; then
    echo "==> [ja configurado] ${key} ${value}"
    return
  fi

  echo "==> Criando backup em $BACKUP_FILE..."
  if [[ ! -f "$BACKUP_FILE" ]]; then
    cp "$SSHD_CONFIG" "$BACKUP_FILE"
  fi

  if grep -qE "^#?[[:space:]]*${key}[[:space:]]+" "$SSHD_CONFIG"; then
    sed -i.tmp "s/^#\\?[[:space:]]*${key}.*/${key} ${value}/" "$SSHD_CONFIG"
    rm -f "${SSHD_CONFIG}.tmp"
    echo "==> [atualizado] ${key} ${value} em $SSHD_CONFIG"
    return
  fi

  echo "" >> "$SSHD_CONFIG"
  echo "${key} ${value}" >> "$SSHD_CONFIG"
  echo "==> [adicionado] ${key} ${value} ao final de $SSHD_CONFIG"
}

ensure_sshd_setting "PasswordAuthentication" "no"
ensure_sshd_setting "KbdInteractiveAuthentication" "no"
ensure_sshd_setting "ChallengeResponseAuthentication" "no"

echo "==> Validando sintaxe e config efetiva com sshd -t/-T..."
if ! sshd -t; then
  echo "ERRO: sshd -t falhou. Revertendo para snapshot pre-hardening..." >&2
  cp "$PRE_HARDENING_SNAPSHOT" "$SSHD_CONFIG"
  exit 1
fi

SSHD_EFFECTIVE="$(sshd -T 2>/dev/null || true)"
validate_effective() {
  local key="$1"
  local expected="$2"
  if echo "$SSHD_EFFECTIVE" | grep -qi "^${key} ${expected}$"; then
    echo "==> [OK efetivo] ${key} ${expected}"
    return 0
  fi
  if ! echo "$SSHD_EFFECTIVE" | grep -qi "^${key} "; then
    echo "==> [WARN] ${key} ausente em sshd -T (provavelmente deprecada nesta versao); ignorando"
    return 0
  fi
  echo "ERRO: ${key} efetivo difere de ${expected}" >&2
  return 1
}

ok=1
validate_effective "passwordauthentication" "no" || ok=0
validate_effective "kbdinteractiveauthentication" "no" || ok=0
validate_effective "challengeresponseauthentication" "no" || ok=0

if [[ $ok -ne 1 ]]; then
  echo "ERRO: validacao efetiva do sshd falhou. Revertendo para snapshot pre-hardening..." >&2
  cp "$PRE_HARDENING_SNAPSHOT" "$SSHD_CONFIG"
  exit 1
fi
echo "==> [OK] Sintaxe e config efetiva validas."

echo ""
echo "==> Hardening aplicado com sucesso."
echo "==> Snapshot canonico para rollback: $PRE_HARDENING_SNAPSHOT"
if [[ -f "$BACKUP_FILE" ]]; then
  echo "==> Backup da execucao atual (auditoria): $BACKUP_FILE"
fi
echo "ATENCAO: o servico sshd NAO foi reiniciado automaticamente."
echo "    1. Confirme que sua chave SSH funciona em outra sessao."
echo "    2. Entao execute: systemctl restart sshd"
echo "    3. Verifique: ssh -o PasswordAuthentication=yes <host> (deve falhar)"
