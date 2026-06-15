# Runbook: VPS Bootstrap — Firewall ufw + SSH Hardening

**Escopo:** VPS Hostinger (Ubuntu) — aplicar firewall com regras explícitas e desabilitar autenticação SSH por senha.

**Resultado esperado (M-04):** `nmap` externo retorna apenas portas 22, 80, 443. SSH com senha rejeitado.

---

## Pré-requisitos

- Acesso root ao VPS (via chave SSH já provisionada).
- `ufw` instalado: `apt-get install -y ufw`.
- Chave SSH testada e funcionando **antes** de desabilitar autenticação por senha.
- Manter **ao menos duas sessões SSH abertas** durante o procedimento (rollback manual).
- Scripts disponíveis em `deployment/scripts/` (clonar o repositório ou copiar manualmente).

---

## Ordem de Execução

### Etapa 1 — SSH Hardening (executar primeiro)

```bash
sudo bash deployment/scripts/vps-ssh-hardening.sh
```

O script:
- Aplica `PasswordAuthentication no` em `/etc/ssh/sshd_config` se ausente.
- Valida a sintaxe com `sshd -t`.
- **Não** reinicia o `sshd` automaticamente.

Após confirmar acesso via chave em **outra sessão**, reinicie manualmente:

```bash
sudo systemctl restart sshd
```

Verificação esperada (deve falhar):

```bash
ssh -o PasswordAuthentication=yes -o PreferredAuthentications=password <ip-do-vps>
# Esperado: Permission denied (publickey)
```

### Etapa 2 — Firewall ufw (executar com sessão SSH aberta)

```bash
sudo bash deployment/scripts/vps-firewall.sh
```

O script aplica as regras abaixo de forma idempotente (re-executar não duplica):

| Política / Regra           | Comando equivalente              |
|----------------------------|----------------------------------|
| Default deny incoming      | `ufw default deny incoming`      |
| Default allow outgoing     | `ufw default allow outgoing`     |
| Permitir SSH               | `ufw allow 22/tcp`               |
| Permitir HTTP              | `ufw allow 80/tcp`               |
| Permitir HTTPS             | `ufw allow 443/tcp`              |

Revisar o output do script (regras listadas com `ufw status numbered`).

### Etapa 3 — Habilitar o firewall

**ATENÇÃO:** mantenha a sessão SSH aberta. Se as regras travarem o acesso, use o console do Hostinger para reverter.

```bash
sudo bash deployment/scripts/vps-firewall.sh --force-enable
```

Ou manualmente:

```bash
sudo ufw --force enable
sudo ufw status verbose
```

---

## Validação com nmap

Execute de uma **máquina externa** (não do próprio VPS):

```bash
nmap -p- --open -T4 <ip-do-vps>
```

Resultado esperado:

```
PORT    STATE SERVICE
22/tcp  open  ssh
80/tcp  open  http
443/tcp open  https
```

Todas as demais portas devem aparecer como `filtered` ou `closed`.

### Resultado do nmap em staging (referência)

```
# Registrar aqui após execução em staging
# Data: ____-__-__
# IP:   <ip-staging>
# Output nmap:
#
# PORT    STATE SERVICE
# 22/tcp  open  ssh
# 80/tcp  open  http
# 443/tcp open  https
```

---

## Rollback

Se o `ufw enable` bloquear acesso SSH:

1. Acessar o VPS via **console web do Hostinger** (não depende de SSH).
2. Executar: `sudo ufw disable`
3. Revisar as regras com `sudo ufw status numbered` e remover regras incorretas.
4. Corrigir e re-aplicar o script.

Se o SSH hardening bloquear acesso:

1. Acessar via console web do Hostinger.

## Rollback do hardening SSH

Rollback canônico (estado pré-execução do script):

    sudo cp /etc/ssh/sshd_config.pre-hardening /etc/ssh/sshd_config
    sudo sshd -t && sudo systemctl restart sshd

Rollback para a execução imediatamente anterior:

    ls -t /etc/ssh/sshd_config.bak.* | head -1 | xargs -I{} sudo cp {} /etc/ssh/sshd_config
    sudo sshd -t && sudo systemctl restart sshd

## Prova de idempotência

Antes do go-live, executar duas vezes e validar:

    sudo bash deployment/scripts/vps-ssh-hardening.sh
    MD5_1=$(md5sum /etc/ssh/sshd_config | awk '{print $1}')
    sudo bash deployment/scripts/vps-ssh-hardening.sh
    MD5_2=$(md5sum /etc/ssh/sshd_config | awk '{print $1}')
    [[ "$MD5_1" == "$MD5_2" ]] && echo "IDEMPOTENTE" || echo "FALHA"

Registrar a saída neste runbook após cada execução em staging.

---

## Referências

- RF-15, RF-16, RF-17 em `.specs/prd-pre-golive-hardening/prd.md`
- Scripts: `deployment/scripts/vps-firewall.sh`, `deployment/scripts/vps-ssh-hardening.sh`
- Receita Task: `task vps:firewall VPS_HOST=<ip>`
