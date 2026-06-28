# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Docker Swarm Single-Node como Orquestrador de Produção
- **Data:** 2026-06-27
- **Status:** Proposta
- **Decisores:** Time fundador/engenharia do MeControla
- **Relacionados:** PRD `infra-producao-robusta-10k-dez-2026`, Tech Spec `techspec.md`

## Contexto

A infraestrutura atual roda em Docker Compose single-host com uma única réplica de `server` e `worker`. O PRD exige suporte a 2 réplicas de cada serviço no início, evoluindo para 3, dentro das restrições de uma VPS Hostinger KVM2 (2 vCPU / 8 GB RAM / 100 GB NVMe) e orçamento zero.

Precisamos escolher uma orquestração que permita múltiplas réplicas, rolling updates e health checks, sem introduzir complexidade desproporcional.

## Decisão

Adotar **Docker Swarm em modo single-node** como orquestrador de produção.

Escopo:
- Inicializar `docker swarm` na VPS de produção.
- Criar `deployment/compose/compose.swarm.yml` contendo todos os services da stack.
- Usar `docker stack deploy` para publicar a aplicação.
- Configurar `update_config` e `restart_policy` em cada service.

## Alternativas Consideradas

### 1. Manter Docker Compose single-host com services duplicados manualmente

- **Vantagens:** simplicidade, nenhuma mudança de orquestrador, scripts atuais continuam funcionando.
- **Desvantagens:** não há rolling update nativo; sem service discovery; gerenciamento de múltiplos containers do mesmo tipo fica manual e propenso a erros.
- **Motivo de não ter sido escolhida:** não atende aos requisitos de deploy robusto e resiliência a falhas de processo.

### 2. Kubernetes (k3s ou k0s)

- **Vantagens:** orquestração completa, ecossistema maduro, auto-healing avançado.
- **Desvantagens:** complexidade operacional alta para um time pequeno; overhead de recursos em uma KVM2; curva de aprendizado; adiciona componentes extras (kubelet, etcd, etc.).
- **Motivo de não ter sido escolhida:** exagero para o contexto de VPS única e orçamento zero.

### 3. Docker Swarm single-node

- **Vantagens:** nativo no Docker Engine, suporta réplicas, rolling updates, service discovery, secrets e networks overlay; curva de aprendizado menor que Kubernetes; adequado para single-host.
- **Desvantagens:** single-node não oferece HA real; falha do host indisponibiliza tudo; menos ecossistema que Kubernetes.
- **Motivo de ter sido escolhida:** melhor custo-benefício para o cenário atual, atende aos requisitos de réplicas e rolling updates sem complexidade excessiva.

## Consequências

### Benefícios Esperados

- Suporte nativo a múltiplas réplicas de `server` e `worker`.
- Rolling updates com controle de parallelism e delay.
- Service discovery via DNS interno do Swarm.
- Docker secrets integrados.
- Networks overlay criptografadas.

### Trade-offs e Custos

- Complexidade adicional na gestão do Swarm (inicialização, join tokens, diagnóstico).
- Necessidade de adaptar scripts de deploy (`deploy.sh`, `deploy-local.sh`) e CI/CD.
- Sem HA de host: falha da VPS ainda requer restore completo.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Falha do nó Swarm corrompe todo o cluster | Alto | Backups S3 via pgBackRest; runbook de restore da VPS |
| Curva de aprendizado do time | Médio | Documentação e runbooks atualizados |
| Migração de Compose para Swarm com downtime | Médio | Janela de manutenção e testes em staging |

## Plano de Implementação

1. Criar `deployment/compose/compose.swarm.yml`.
2. Atualizar scripts de deploy para usar `docker stack deploy`.
3. Inicializar Swarm na VPS de staging e validar.
4. Executar migração em produção em janela de manutenção.
5. Validar health checks e métricas pós-migração.

## Monitoramento e Validação

- Acompanhar estado dos services com `docker service ps mecontrola_<service>`.
- Alertas de indisponibilidade via Telegram.
- Critério de sucesso: 2 réplicas de server e 2 de worker saudáveis por pelo menos 24h.
- Revisar a decisão quando houver upgrade para multi-node.

## Impacto em Documentação e Operação

- Atualizar `deployment/runbooks/deploy.md`.
- Criar runbook de troubleshooting do Swarm.
- Atualizar onboarding técnico do time.

## Revisão Futura

Revisitar esta ADR quando:
- Orçamento permitir segunda VPS ou cluster multi-node.
- A carga ultrapassar consistentemente 70% de CPU/RAM na KVM2.
- Kubernetes se tornar justificável em custo/benefício.
