#!/usr/bin/env bash
# deployment/scripts/vps-hardening.sh
#
# Instala e configura fail2ban e unattended-upgrades no VPS Hostinger.
# Idempotente: pode ser re-executado sem efeitos colaterais.
#
# Uso:
#   sudo ./vps-hardening.sh
#
# Pre-requisitos:
#   - Ubuntu 22.04 / Debian 12
#   - root ou sudo
#   - Internet disponível (apt)

set -euo pipefail

if [[ "$EUID" -ne 0 ]]; then
  echo "ERRO: requer root (execute com sudo ou como root)." >&2
  exit 1
fi

echo "==> Atualizando índice de pacotes..."
apt-get update -qq

# ---------------------------------------------------------------------------
# fail2ban
# ---------------------------------------------------------------------------
echo "==> Instalando fail2ban..."
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq fail2ban

JAIL_LOCAL=/etc/fail2ban/jail.local
if [[ ! -f "$JAIL_LOCAL" ]]; then
  cat >"$JAIL_LOCAL" <<'EOF'
[DEFAULT]
bantime  = 1h
findtime = 10m
maxretry = 5
backend  = systemd
destemail = root@localhost
action = %(action_mwl)s

[sshd]
enabled  = true
port     = ssh
filter   = sshd
maxretry = 3
bantime  = 24h

[nginx-http-auth]
enabled = false

[caddy]
enabled  = true
port     = http,https
filter   = caddy
logpath  = /var/lib/docker/containers/*/*.log
maxretry = 10
bantime  = 1h
EOF
  echo "    [criado] $JAIL_LOCAL"
else
  echo "    [ja existe] $JAIL_LOCAL"
fi

CADDY_FILTER=/etc/fail2ban/filter.d/caddy.conf
if [[ ! -f "$CADDY_FILTER" ]]; then
  cat >"$CADDY_FILTER" <<'EOF'
[Definition]
failregex = ^.*"remote_ip":"<HOST>".*"status":4[0-9][0-9].*$
            ^.*"remote_ip":"<HOST>".*"status":5[0-9][0-9].*$
ignoreregex =
EOF
  echo "    [criado] $CADDY_FILTER"
else
  echo "    [ja existe] $CADDY_FILTER"
fi

systemctl enable fail2ban
systemctl restart fail2ban
echo "==> fail2ban ativo. Status:"
fail2ban-client status

# ---------------------------------------------------------------------------
# unattended-upgrades
# ---------------------------------------------------------------------------
echo ""
echo "==> Instalando unattended-upgrades..."
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq unattended-upgrades apt-listchanges

UA_CONF=/etc/apt/apt.conf.d/50unattended-upgrades
cat >"$UA_CONF" <<'EOF'
Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}";
    "${distro_id}:${distro_codename}-security";
    "${distro_id}ESMApps:${distro_codename}-apps-security";
    "${distro_id}ESM:${distro_codename}-infra-security";
};
Unattended-Upgrade::Package-Blacklist {};
Unattended-Upgrade::AutoFixInterruptedDpkg "true";
Unattended-Upgrade::MinimalSteps "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::Automatic-Reboot-Time "03:00";
Unattended-Upgrade::Mail "root";
Unattended-Upgrade::MailReport "on-change";
EOF
echo "    [configurado] $UA_CONF"

AUTO_CONF=/etc/apt/apt.conf.d/20auto-upgrades
cat >"$AUTO_CONF" <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
EOF
echo "    [configurado] $AUTO_CONF"

systemctl enable unattended-upgrades
systemctl restart unattended-upgrades
echo "==> unattended-upgrades ativo."

# ---------------------------------------------------------------------------
# SSH hardening
# ---------------------------------------------------------------------------
echo ""
echo "==> Endurecendo configuracao SSH..."
SSHD_CONFIG=/etc/ssh/sshd_config
SSHD_BACKUP="${SSHD_CONFIG}.bak.$(date +%Y%m%d)"

if [[ ! -f "$SSHD_BACKUP" ]]; then
  cp "$SSHD_CONFIG" "$SSHD_BACKUP"
  echo "    [backup] $SSHD_BACKUP"
fi

# Validacao previa: garantir que ha pelo menos uma chave SSH autorizada antes de
# desabilitar password auth. Se nao houver, falhar — evita lockout.
AUTHORIZED_KEYS_FOUND=0
for home in /root /home/*; do
  if [[ -f "${home}/.ssh/authorized_keys" ]] && [[ -s "${home}/.ssh/authorized_keys" ]]; then
    AUTHORIZED_KEYS_FOUND=1
    break
  fi
done

if [[ "$AUTHORIZED_KEYS_FOUND" -eq 0 ]]; then
  echo "ERRO: nenhuma authorized_keys encontrada em /root/.ssh ou /home/*/.ssh." >&2
  echo "      Recusando endurecer SSH para evitar lockout. Adicione sua chave publica" >&2
  echo "      via 'ssh-copy-id' antes de re-rodar este script." >&2
  exit 1
fi

apply_sshd_directive() {
  local key="$1"
  local value="$2"
  if grep -qE "^[#[:space:]]*${key}[[:space:]]" "$SSHD_CONFIG"; then
    sed -i -E "s|^[#[:space:]]*${key}[[:space:]]+.*$|${key} ${value}|" "$SSHD_CONFIG"
  else
    echo "${key} ${value}" >>"$SSHD_CONFIG"
  fi
}

apply_sshd_directive "PasswordAuthentication" "no"
apply_sshd_directive "PermitRootLogin" "prohibit-password"
apply_sshd_directive "ChallengeResponseAuthentication" "no"
apply_sshd_directive "KbdInteractiveAuthentication" "no"
apply_sshd_directive "UsePAM" "yes"
apply_sshd_directive "X11Forwarding" "no"
apply_sshd_directive "MaxAuthTries" "3"
apply_sshd_directive "ClientAliveInterval" "300"
apply_sshd_directive "ClientAliveCountMax" "2"
apply_sshd_directive "LoginGraceTime" "30"

if sshd -t 2>/dev/null; then
  systemctl restart ssh
  echo "    [aplicado] PasswordAuthentication=no, PermitRootLogin=prohibit-password, etc."
else
  echo "ERRO: sshd -t falhou. Restaurando backup." >&2
  cp "$SSHD_BACKUP" "$SSHD_CONFIG"
  exit 1
fi

# ---------------------------------------------------------------------------
# Swapfile 2GB (Hostinger VPS 8GB → swap 25%)
# ---------------------------------------------------------------------------
echo ""
echo "==> Configurando swapfile 2GB..."
SWAPFILE=/swapfile
if [[ ! -f "$SWAPFILE" ]]; then
  fallocate -l 2G "$SWAPFILE"
  chmod 600 "$SWAPFILE"
  mkswap "$SWAPFILE"
  swapon "$SWAPFILE"
  if ! grep -q "^${SWAPFILE} " /etc/fstab; then
    echo "${SWAPFILE} none swap sw 0 0" >>/etc/fstab
  fi
  echo "    [criado] $SWAPFILE (2G)"
else
  echo "    [ja existe] $SWAPFILE"
fi

# Tunar swappiness (60 default e muito agressivo para servidor com SSD)
SYSCTL_FILE=/etc/sysctl.d/99-mecontrola-swap.conf
cat >"$SYSCTL_FILE" <<'EOF'
vm.swappiness = 10
vm.vfs_cache_pressure = 50
EOF
sysctl -p "$SYSCTL_FILE" >/dev/null
echo "    [aplicado] vm.swappiness=10"

# ---------------------------------------------------------------------------
# UFW (firewall)
# ---------------------------------------------------------------------------
echo ""
echo "==> Configurando UFW..."
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq ufw

ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp comment 'SSH'
ufw allow 80/tcp comment 'HTTP (Caddy/ACME)'
ufw allow 443/tcp comment 'HTTPS'
ufw --force enable
echo "    [ativo] UFW: 22/80/443 allow; resto deny"

echo ""
echo "==> Hardening concluído."
echo "    fail2ban: $(fail2ban-client --version 2>&1 | head -1)"
echo "    unattended-upgrades: $(dpkg -s unattended-upgrades 2>/dev/null | grep '^Version' | awk '{print $2}')"
echo "    SSH: PasswordAuthentication=no, PermitRootLogin=prohibit-password"
echo "    Swap: $(swapon --show=NAME,SIZE --noheadings | tr '\n' ' ')"
echo "    UFW: $(ufw status | head -1)"
echo ""
echo "    Próximos passos:"
echo "    - Revise /etc/fail2ban/jail.local e ajuste destemail"
echo "    - Teste: fail2ban-client status sshd"
echo "    - Teste: unattended-upgrade --dry-run --debug"
echo "    - Teste SSH em segundo terminal antes de fechar a sessao atual"
