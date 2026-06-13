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

# Verificar se PasswordAuthentication ja esta desabilitado corretamente
if grep -qE '^PasswordAuthentication[[:space:]]+no' "$SSHD_CONFIG"; then
  echo "==> [ja configurado] PasswordAuthentication no — nenhuma alteracao necessaria."
else
  echo "==> Criando backup em $BACKUP_FILE..."
  cp "$SSHD_CONFIG" "$BACKUP_FILE"

  # Substituir ou adicionar a diretiva
  if grep -qE '^#?[[:space:]]*PasswordAuthentication' "$SSHD_CONFIG"; then
    # Diretiva presente (comentada ou com valor diferente) — substituir
    sed -i.tmp 's/^#\?[[:space:]]*PasswordAuthentication.*/PasswordAuthentication no/' "$SSHD_CONFIG"
    rm -f "${SSHD_CONFIG}.tmp"
    echo "==> [atualizado] PasswordAuthentication no em $SSHD_CONFIG"
  else
    # Diretiva ausente — adicionar ao final
    echo "" >> "$SSHD_CONFIG"
    echo "PasswordAuthentication no" >> "$SSHD_CONFIG"
    echo "==> [adicionado] PasswordAuthentication no ao final de $SSHD_CONFIG"
  fi
fi

echo "==> Validando sintaxe com sshd -t..."
if sshd -t; then
  echo "==> [OK] Sintaxe do sshd_config valida."
else
  echo "ERRO: sshd -t falhou. Revertendo backup..." >&2
  cp "$BACKUP_FILE" "$SSHD_CONFIG"
  echo "Backup restaurado de $BACKUP_FILE" >&2
  exit 1
fi

echo ""
echo "==> Hardening aplicado com sucesso."
echo "ATENCAO: o servico sshd NAO foi reiniciado automaticamente."
echo "    1. Confirme que sua chave SSH funciona em outra sessao."
echo "    2. Entao execute: systemctl restart sshd"
echo "    3. Verifique: ssh -o PasswordAuthentication=yes <host> (deve falhar)"
