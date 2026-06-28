# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Services Separados (`server-1`, `server-2`) em vez de `deploy.replicas=2`
- **Data:** 2026-06-27
- **Status:** Proposta
- **Decisores:** Time fundador/engenharia do MeControla
- **Relacionados:** PRD `infra-producao-robusta-10k-dez-2026`, Tech Spec `techspec.md`, ADR-001

## Contexto

Dentro do Docker Swarm, há duas formas de ter múltiplas instâncias de um serviço:

1. Um único service com `deploy.replicas=N`.
2. N services distintos, cada um com `deploy.replicas=1`.

A escolha afeta diretamente a capacidade do Caddy de realizar health checks ativos por réplica e de controlar o rolling update de forma granular.

## Decisão

Adotar **services separados e nomeados**: `server-1`, `server-2`, `worker-1`, `worker-2`.

Cada service terá:

```yaml
deploy:
  replicas: 1
```

O Caddy fará referência explícita aos upstreams:

```caddy
reverse_proxy server-1:8080 server-2:8080 { ... }
```

## Alternativas Consideradas

### 1. Service único com `deploy.replicas=2`

- **Vantagens:** menos linhas de YAML, rolling update nativo em um único comando, service discovery DNS simples.
- **Desvantagens:** o Caddy não consegue fazer health check ativo de cada task individual sem plugin de service discovery; falha de uma task só é detectada via DNS round-robin ou health check no virtual IP.
- **Motivo de não ter sido escolhida:** o PRD exige health checks ativos por réplica, e o Caddy não possui service discovery nativo de tasks do Swarm.

### 2. Services separados e nomeados

- **Vantagens:** health checks ativos por upstream no Caddy; controle granular de resources e update por service; fácil identificação de qual réplica falhou.
- **Desvantagens:** mais verboso no Compose; ao escalar para 3 réplicas, é necessário adicionar um novo service e atualizar o Caddyfile.
- **Motivo de ter sido escolhida:** atende ao requisito de health checks ativos e permite evolução controlada.

## Consequências

### Benefícios Esperados

- Failover real entre `server-1` e `server-2` baseado em health checks.
- Identificação rápida de qual réplica está degradada.
- Possibilidade de isolar uma réplica para investigação sem afetar a outra.

### Trade-offs e Custos

- YAML mais extenso.
- Scaling horizontal exige edição de dois arquivos (`compose.swarm.yml` e `Caddyfile`).
- Não é a forma mais "Swarm-native" de escalar.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Esquecer de atualizar Caddyfile ao adicionar nova réplica | Médio | Checklist de deploy; validação automatizada |
| Configuração duplicada entre server-1 e server-2 | Baixo | Usar YAML anchors ou gerador de config |

## Plano de Implementação

1. Definir services `server-1` e `server-2` no `compose.swarm.yml`.
2. Definir services `worker-1` e `worker-2`.
3. Atualizar `Caddyfile` para listar os dois upstreams.
4. Documentar procedimento para adicionar a terceira réplica futuramente.

## Monitoramento e Validação

- Verificar no Caddy logs se health checks de ambos os upstreams respondem 200.
- `docker service ps mecontrola_server-1` e `mecontrola_server-2` devem mostrar estado `Running`.
- Métricas de latência por upstream, se disponíveis.

## Impacto em Documentação e Operação

- Atualizar `Caddyfile` e runbooks de deploy.
- Documentar que scaling horizontal requer alteração manual no Caddyfile.

## Revisão Futura

Revisitar quando:
- Houver 3+ réplicas e a manutenção manual do Caddyfile se tornar impraticável.
- For adotado um proxy com service discovery nativa (ex.: Traefik).
