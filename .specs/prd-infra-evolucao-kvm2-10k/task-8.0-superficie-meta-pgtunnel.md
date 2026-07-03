# Tarefa 8.0: Endurecimento de superfície — rate-limit Meta e bind do pg-tunnel

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Dois riscos de superfície: (1) o rate-limit por IP do Caddy/webhook pode estrangular rajadas legítimas dos webhooks da Meta (que chegam de poucos IPs); (2) o `pg-tunnel` escuta em `0.0.0.0:15432`, dependendo apenas do ufw. Endurecer ambos.

<requirements>
- RF-21: tratar rate-limit × webhooks Meta (allowlist/limite específico para IPs da Meta) sem afrouxar a proteção contra abuso.
- RF-22: restringir o pg-tunnel (bind loopback e/ou remoção quando ocioso) além do ufw.
</requirements>

## Subtarefas

- [ ] 8.1 Ajustar `Caddyfile`/`WHATSAPP_WEBHOOK_RATE_LIMIT_*` para allowlist ou limite específico dos blocos de IP de origem da Meta, validado contra a doc oficial da Meta.
- [ ] 8.2 Alterar o bind do `pg-tunnel` de `0.0.0.0:15432` para loopback (ou torná-lo opt-in/removível quando ocioso) em `compose.swarm.yml`.
- [ ] 8.3 Documentar a política de acesso ao banco e o rate-limit da Meta no runbook de segurança.

## Detalhes de Implementação

Ver `techspec.md` REQ-08. Manter o ufw como segunda camada. Mudança em código Go (se o rate-limit for aplicado no app) segue `go-implementation` e as regras de adapters.

## Critérios de Sucesso

- Rajadas legítimas da Meta não recebem 429; abuso de outros IPs continua limitado.
- `pg-tunnel` não mais exposto em `0.0.0.0`; acesso DBA documentado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (lógica de rate-limit/allowlist, se em código Go: testify/suite)
- [ ] Testes de integração (rajada simulada dos IPs da Meta passa; pg-tunnel não acessível externamente)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/caddy/Caddyfile`, `deployment/caddy/Caddyfile.ratelimit`
- `deployment/config/prod.env` (WHATSAPP_WEBHOOK_RATE_LIMIT_*)
- `deployment/compose/compose.swarm.yml` (pg-tunnel)
- `deployment/scripts/vps-firewall.sh`, `deployment/runbooks/` (segurança)
