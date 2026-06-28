# Tarefa 1.0: Criar compose.swarm.yml e Configurar Docker Swarm Single-Node

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o arquivo `deployment/compose/compose.swarm.yml` que define a stack de produção no Docker Swarm single-node, com 2 réplicas nomeadas de `server` e 2 de `worker`, networks overlay encrypted, volumes persistentes, resource limits, restart policies e startup ordenado.

<requirements>
- Cobrir RF-01: 2 réplicas de server/worker no início, caminho para 3.
- Cobrir RF-02: Docker Swarm single-node como orquestrador.
- Cobrir RF-06: ordem de startup determinística.
- Cobrir RF-08: rede backend isolada (overlay encrypted) e frontend apenas para Caddy.
- Cobrir RF-09: PostgreSQL não exposto publicamente.
- Cobrir RF-20: restart policy, resource limits/reservations e logging rotativo.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `deployment/compose/compose.swarm.yml` com services: `postgres`, `pgbouncer`, `migrate`, `server-1`, `server-2`, `worker-1`, `worker-2`, `caddy`, `otel-lgtm`.
- [ ] 1.2 Configurar networks `backend` (overlay encrypted) e `frontend` (overlay).
- [ ] 1.3 Definir volumes nomeados locais para postgres, pgbackrest, caddy e otel-lgtm.
- [ ] 1.4 Configurar `deploy.resources.limits/reservations`, `deploy.restart_policy` e `deploy.update_config` para cada service.
- [ ] 1.5 Configurar `depends_on` com `condition: service_healthy` ou `service_started` conforme necessário.
- [ ] 1.6 Garantir que `postgres` não exponha a porta 5432 externamente.
- [ ] 1.7 Inicializar Docker Swarm em staging e validar `docker stack deploy`.
- [ ] 1.8 Documentar procedimento de migração de Compose para Swarm.

## Detalhes de Implementação

Ver seção "1. Docker Swarm" de `techspec.md`. Cada service nomeado (`server-1`, `server-2`, etc.) deve ter `deploy.replicas: 1`. O `update_config` deve usar `parallelism: 1`, `delay: 20s`, `failure_action: pause`, `order: stop-first`.

Resources iniciais sugeridos para KVM2:

| Service | CPU limit | RAM limit |
|---|---|---|
| server-1/server-2 | 0.75 | 768M |
| worker-1/worker-2 | 0.50 | 384M |
| postgres | 1.00 | 2G |
| pgbouncer | 0.25 | 128M |
| caddy | 0.25 | 128M |
| otel-lgtm | 0.50 | 512M |

> Esses values devem ser validados em staging; ajustar se houver OOM.

## Critérios de Sucesso

- `docker stack deploy -c deployment/compose/compose.swarm.yml mecontrola` executa sem erros.
- `docker service ls` mostra todos os services no estado `1/1` ou `replicated 1/1`.
- `docker network ls` mostra networks overlay `mecontrola_backend` e `mecontrola_frontend`.
- Porta 5432 não está exposta em `0.0.0.0` (verificar com `ss -tlnp`).
- Logs de startup mostram ordem respeitada: postgres → pgbouncer → migrate → server/worker → caddy.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes manuais em staging: subir stack e verificar services saudáveis.
- [ ] Teste de rede: confirmar que `server-1` consegue pingar `postgres` e `pgbouncer`.
- [ ] Teste de segurança: confirmar que `postgres` não é acessível externamente.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/compose/compose.yml`
- `deployment/compose/compose.prod.yml`
- `deployment/compose/compose.swarm.yml` (novo)
- `deployment/runbooks/deploy.md`
