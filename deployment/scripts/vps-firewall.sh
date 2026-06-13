#!/usr/bin/env bash
# deployment/scripts/vps-firewall.sh
#
# Configura ufw de forma idempotente no VPS Hostinger.
# Regras: default deny incoming, allow outgoing, allow 22/80/443 tcp.
#
# Uso:
#   ./vps-firewall.sh            -- aplica regras, NAO habilita ufw
#   ./vps-firewall.sh --force-enable  -- aplica regras E habilita ufw
#
# Pre-requisitos:
#   - ufw instalado (apt-get install ufw)
#   - root ou sudo
#   - Manter sessao SSH aberta antes de --force-enable (rollback manual)

set -euo pipefail

FORCE_ENABLE=false
for arg in "$@"; do
  [[ "$arg" == "--force-enable" ]] && FORCE_ENABLE=true
done

# Verificar presenca de ufw
if ! command -v ufw >/dev/null 2>&1; then
  echo "ERRO: ufw nao encontrado. Instale com: apt-get install -y ufw" >&2
  exit 1
fi

echo "==> Verificando privilegios..."
if [[ "$EUID" -ne 0 ]]; then
  echo "ERRO: este script requer root (execute com sudo ou como root)." >&2
  exit 1
fi

# Aplicar politicas default de forma idempotente
echo "==> Configurando politica default: deny incoming, allow outgoing..."
ufw default deny incoming
ufw default allow outgoing

# Funcao auxiliar: adicionar regra apenas se ausente
add_rule_if_missing() {
  local rule="$1"
  local description="$2"

  if ufw status | grep -qF "$rule"; then
    echo "    [ja existe] $rule — $description"
  else
    ufw allow "$rule"
    echo "    [adicionado] $rule — $description"
  fi
}

echo "==> Aplicando regras de entrada..."
add_rule_if_missing "22/tcp"  "SSH"
add_rule_if_missing "80/tcp"  "HTTP"
add_rule_if_missing "443/tcp" "HTTPS"

echo ""
echo "==> Status atual do ufw:"
ufw status numbered

if [[ "$FORCE_ENABLE" == "true" ]]; then
  echo ""
  echo "==> --force-enable detectado. Habilitando ufw..."
  echo "ATENCAO: mantenha sessao SSH aberta para rollback em caso de bloqueio."
  ufw --force enable
  echo "==> ufw habilitado com sucesso."
  ufw status verbose
else
  echo ""
  echo "==> Regras aplicadas. ufw NAO foi habilitado automaticamente."
  echo "    Para habilitar manualmente (mantenha SSH aberto):"
  echo "      ufw --force enable"
  echo "    Ou re-execute com: $0 --force-enable"
fi
