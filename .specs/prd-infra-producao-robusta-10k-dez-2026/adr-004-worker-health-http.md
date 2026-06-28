# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Health Check do Worker via Servidor HTTP Interno na Porta 8081
- **Data:** 2026-06-27
- **Status:** Proposta
- **Decisores:** Time fundador/engenharia do MeControla
- **Relacionados:** PRD `infra-producao-robusta-10k-dez-2026`, Tech Spec `techspec.md`

## Contexto

O PRD exige que o `worker` exponha endpoints de readiness/liveness que validem a capacidade real de processar jobs, não apenas a existência do processo. Atualmente, o `worker` é um processo Go sem servidor HTTP.

Precisamos escolher uma forma de expor health checks compatível com Docker Swarm e com a arquitetura existente.

## Decisão

Adicionar um **servidor HTTP mínimo interno ao worker** na porta `8081`, expondo:

- `GET /livez` — retorna 200 se o processo está vivo.
- `GET /readyz` — retorna 200 se o worker consegue acessar o banco de dados; 503 caso contrário.

O servidor será iniciado em uma goroutine dentro de `cmd/worker/worker.go` e encerrado graciosamente durante o shutdown.

## Alternativas Consideradas

### 1. Health check via comando no container (`pgrep` + psql)

- **Vantagens:** não requer mudança de código Go.
- **Desvantagens:** não reflete a saúde real do worker (apenas processo e banco); mais lento e propenso a falsos positivos; exige `psql` na imagem distroless.
- **Motivo de não ter sido escolhida:** não atende ao requisito de readiness significativo.

### 2. Escrever arquivo de status no filesystem

- **Vantagens:** simples, sem porta adicional.
- **Desvantagens:** dificulta health check remoto; requer volume compartilhado; menos padronizado.
- **Motivo de não ter sido escolhida:** não é compatível com health checks HTTP do Swarm.

### 3. Servidor HTTP interno (escolhida)

- **Vantagens:** padrão da indústria; compatível com Swarm e Caddy; permite verificar banco e scheduler; leve e fácil de testar.
- **Desvantagens:** adiciona uma goroutine e uma porta ao worker; requer cuidado com graceful shutdown.

## Consequências

### Benefícios Esperados

- Health check padronizado e significativo para o worker.
- Facilita integração com Swarm (`healthcheck` no Compose) e possível exposição futura via Caddy.
- Base para futuras métricas do worker (`/metrics`).

### Trade-offs e Custos

- Código adicional no `cmd/worker`.
- Porta `8081` adicional a ser documentada e protegida (não exposta publicamente).
- Graceful shutdown deve encerrar o health server corretamente.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Health server falha mas worker continua processando | Baixo | Liveness continua OK; readiness reflete banco |
| Porta 8081 conflita com outro serviço | Baixo | Usar porta não padrão e documentar |
| Goroutine de health vaza em shutdown | Baixo | Usar `http.Server.Shutdown` com context timeout |

## Plano de Implementação

1. Criar `cmd/worker/health.go` com `healthServer`.
2. Iniciar health server em `cmd/worker/worker.go` após inicialização do banco.
3. Encerrar health server no `shutdown` do worker.
4. Configurar `healthcheck` no `compose.swarm.yml` apontando para `http://localhost:8081/readyz`.
5. Adicionar testes unitários para os handlers.

## Monitoramento e Validação

- `docker inspect mecontrola_worker-1` deve mostrar `Health.Status: healthy`.
- Logs do worker devem mostrar inicialização do health server.
- Simular falha de banco e verificar se `/readyz` retorna 503.

## Impacto em Documentação e Operação

- Documentar porta `8081` como porta interna de health do worker.
- Atualizar diagramas de arquitetura.
- Incluir health do worker nos runbooks de troubleshooting.

## Revisão Futura

Revisitar quando:
- O worker passar a expor métricas Prometheus (`/metrics`).
- Houver necessidade de expor health do worker externamente.
