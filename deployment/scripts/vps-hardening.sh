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

echo ""
echo "==> Hardening concluído."
echo "    fail2ban: $(fail2ban-client --version 2>&1 | head -1)"
echo "    unattended-upgrades: $(dpkg -s unattended-upgrades 2>/dev/null | grep '^Version' | awk '{print $2}')"
echo ""
echo "    Próximos passos:"
echo "    - Revise /etc/fail2ban/jail.local e ajuste destemail"
echo "    - Teste: fail2ban-client status sshd"
echo "    - Teste: unattended-upgrade --dry-run --debug"
