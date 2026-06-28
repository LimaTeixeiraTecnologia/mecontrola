# Tarefa 2.0: Atualizar Caddyfile com Load Balancing, Health Checks e Rate Limit

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Atualizar o `deployment/caddy/Caddyfile` para suportar 2 upstreams explícitos (`server-1:8080`, `server-2:8080`) com health checks ativos, política de load balancing e rate limiting por IP via plugin `caddy-ratelimit`.

<requirements>
- Cobrir RF-03: 2 upstreams explícitos com health checks ativos e `lb_policy` definida.
- Cobrir RF-04: garantir que os endpoints de readiness/liveness do server (`/healthz`, `/readyz`) sejam consumidos pelo Caddy como health checks.
- Cobrir RF-21: rate limiting na borda (100 req/s por IP, burst 200).
</requirements>

## Subtarefas

- [ ] 2.1 Atualizar `deployment/caddy/Caddyfile` com bloco de configuração global.
- [ ] 2.2 Definir snippet `mecontrola-headers` com headers de segurança.
- [ ] 2.3 Definir snippet `rate-limit` com `caddy-ratelimit` (100 req/s, burst 200).
- [ ] 2.4 Configurar `reverse_proxy` para `server-1:8080 server-2:8080` com health checks ativos.
- [ ] 2.5 Aplicar snippets nas portas 80 e 443.
- [ ] 2.6 Validar Caddyfile com `caddy adapt --config ...`.
- [ ] 2.7 Confirmar que endpoints `/healthz` e `/readyz` do server respondem 200.
- [ ] 2.8 Testar em staging: derrubar `server-1` e verificar se Caddy direciona para `server-2`.

## Detalhes de Implementação

Ver seção "2. Caddy — Proxy de Borda" de `techspec.md`. Health checks devem usar `/healthz`, intervalo 10s, timeout 5s, status 200. Load balancing `round_robin`. Configurar `fail_duration 30s` e `max_fails 3` para failover passivo.

Exemplo de health check:

```caddy
reverse_proxy server-1:8080 server-2:8080 {
    lb_policy round_robin
    health_uri /healthz
    health_interval 10s
    health_timeout 5s
    health_status 200
    fail_duration 30s
    max_fails 3
}
```

## Critérios de Sucesso

- `caddy adapt --config deployment/caddy/Caddyfile` retorna JSON válido (exit 0).
- Requisições são distribuídas entre `server-1` e `server-2`.
- Quando `server-1` é parado, Caddy passa a enviar tráfego apenas para `server-2` em até ~30s.
- Requisições acima de 100 req/s por IP retornam HTTP 429.
- Headers de segurança estão presentes nas respostas.

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste de load balancing: logs mostram requisições em ambos os upstreams.
- [ ] Teste de health check: parar `server-1` e verificar failover.
- [ ] Teste de rate limit: script com `curl` em loop verifica 429 após burst.
- [ ] Teste de headers: `curl -I` valida headers de segurança.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/caddy/Caddyfile`
- `deployment/caddy/Dockerfile.caddy`
- `deployment/compose/compose.swarm.yml`
