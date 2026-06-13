# Tarefa 7.0: B5 — Firewall VPS ufw + SSH hardening

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa firewall `ufw` no VPS Hostinger com regras explícitas e idempotentes (apenas 22/80/443 abertos), combinado com hardening de SSH (chave only, sem senha). Sem isso, Postgres/Redis/admin podem ser alcançáveis externamente apesar do Caddy.

<requirements>
- RF-15: runbook documentando todas as regras (`default deny incoming`, `allow 22/tcp`, `allow 80/tcp`, `allow 443/tcp`, `enable`)
- RF-16: script `deployment/scripts/vps-firewall.sh` idempotente (re-executar não duplica)
- RF-17: validação manual com `nmap` externo — apenas 22/80/443 abertos
- SSH config: `PasswordAuthentication no` em `/etc/ssh/sshd_config`
- Sessão SSH aberta para rollback antes de `ufw enable` (documentado no runbook)
</requirements>

## Subtarefas

- [ ] 7.1 Criar `deployment/scripts/vps-firewall.sh` shell idempotente:
  - Verifica se `ufw` instalado; se não, abort com mensagem.
  - Aplica `ufw default deny incoming`, `ufw default allow outgoing`, `ufw allow 22/tcp`, `ufw allow 80/tcp`, `ufw allow 443/tcp`.
  - Verifica regras existentes antes de adicionar (idempotência).
  - **Não** chama `ufw enable` automaticamente — requer flag `--force-enable` explícita. Runbook documenta passo manual.
- [ ] 7.2 Criar `deployment/scripts/vps-ssh-hardening.sh` que aplica `PasswordAuthentication no` em `/etc/ssh/sshd_config` se ausente, valida com `sshd -t`, sem `systemctl restart` automático (requerido manualmente após confirmação).
- [ ] 7.3 Criar `docs/runbooks/vps-bootstrap.md` cobrindo:
  - Pré-requisitos (acesso root, chave SSH provisionada).
  - Ordem de execução dos 2 scripts.
  - Validação com `nmap`.
  - Rollback procedure.
- [ ] 7.4 Aplicar em staging primeiro; documentar resultado de `nmap` no runbook.
- [ ] 7.5 Adicionar receita `task vps:firewall` em `taskfiles/local.yml` ou `taskfiles/security.yml` que invoca o script via `ssh`.

## Detalhes de Implementação

Ver plano-fonte §5 B5. Skill `taskfile-production` recomendada para garantir que receitas no Taskfile sigam padrão idempotente do projeto.

## Critérios de Sucesso

- `nmap -p- <staging-ip>` mostra apenas 22, 80, 443 abertos.
- `ssh -o PasswordAuthentication=yes <staging>` falha (apenas chave aceita).
- Re-executar `vps-firewall.sh` não duplica regras.
- Runbook revisado, sem TODOs.

## Skills Necessárias

<!-- MANDATÓRIO -->

- `taskfile-production` — adicionar receita `vps:firewall` em `taskfiles/` seguindo padrão idempotente do projeto (RF-15, RF-16)

## Testes da Tarefa

- [ ] `nmap` externo em staging
- [ ] Re-execução idempotente do script
- [ ] SSH com senha rejeita

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/scripts/vps-firewall.sh` (novo)
- `deployment/scripts/vps-ssh-hardening.sh` (novo)
- `docs/runbooks/vps-bootstrap.md` (novo)
- `taskfiles/local.yml` ou `taskfiles/security.yml` (modificado)
