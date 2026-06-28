# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Uso de Docker Secrets com `.env` como Fonte de Verdade no Host
- **Data:** 2026-06-27
- **Status:** Proposta
- **Decisores:** Time fundador/engenharia do MeControla
- **Relacionados:** PRD `infra-producao-robusta-10k-dez-2026`, Tech Spec `techspec.md`

## Contexto

O PRD exige que segredos sensíveis não trafeguem em variáveis de ambiente plain nem estejam em imagens. O projeto atualmente usa um arquivo `.env` montado nos containers via `env_file`.

Precisamos definir uma estratégia de gestão de segredos compatível com Docker Swarm e orçamento zero.

## Decisão

Adotar **Docker Secrets** para os segredos mais críticos, mantendo o `.env` no host como fonte de verdade para configurações não sensíveis e bootstrap.

Estratégia híbrida:

1. O `.env` continua no host com `chmod 600`.
2. Um script `create-secrets.sh` lê os segredos críticos do `.env` e cria/atualiza Docker secrets no Swarm.
3. Os secrets são montados nos services como variáveis de ambiente.
4. Configurações não sensíveis continuam sendo passadas via `env_file`.
5. O `.env` é backupeado para o S3 a cada deploy (SSE-S3 + IAM restricto).

Segredos críticos iniciais:

- `DB_PASSWORD`
- `META_ACCESS_TOKEN`
- `META_APP_SECRET`
- `KIWIY_WEBHOOK_SECRET`
- `KIWIY_CLIENT_SECRET`
- `OPENROUTER_API_KEY`
- `ONBOARDING_TOKEN_ENCRYPTION_KEY`
- `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT`
- `IDENTITY_GATEWAY_SHARED_SECRET_NEXT`

## Alternativas Consideradas

### 1. Manter `.env` montado em todos os containers

- **Vantagens:** simplicidade, compatibilidade total com código existente.
- **Desvantagens:** viola RF-10 do PRD; todos os segredos expostos como env em todos os containers; risco maior em caso de vazamento.
- **Motivo de não ter sido escolhida:** não atende ao requisito de segurança.

### 2. Docker secrets para todos os valores sensíveis e não sensíveis

- **Vantagens:** máxima segurança; nenhum segredo em `.env`.
- **Desvantagens:** complexidade operacional alta; rotação de secrets exige recriar services; mudança grande no fluxo de deploy.
- **Motivo de não ter sido escolhida:** complexidade excessiva para esta fase.

### 3. Estratégia híbrida (escolhida)

- **Vantagens:** melhora segurança sem ruptura total do processo; permite evolução gradual.
- **Desvantagens:** ainda há segredos no `.env` (embora protegido no host); requer script de sincronização.

## Consequências

### Benefícios Esperados

- Segredos críticos não ficam expostos em variáveis de ambiente plain nos containers.
- Backup do `.env` no S3 protege contra perda do host.
- Caminho claro para aumentar o uso de secrets no futuro.

### Trade-offs e Custos

- Rotação de secrets exige cuidado no Swarm (secrets são imutáveis).
- Script de deploy fica mais complexo.
- Ainda há dependência do `.env` no host.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Vazamento do `.env` no host | Alto | chmod 600, acesso restrito, MFA, backup criptografado no S3 |
| Falha na criação de secrets durante deploy | Médio | Validar antes do `docker stack deploy` |
| Secrets desatualizados | Médio | Script compara hash e recria quando necessário |

## Plano de Implementação

1. Criar `deployment/scripts/create-secrets.sh`.
2. Criar `deployment/scripts/backup-env-s3.sh`.
3. Atualizar `compose.swarm.yml` para montar secrets nos services.
4. Atualizar `deploy.sh` e `deploy-local.sh` para chamar os novos scripts.
5. Configurar IAM restricto para upload do `.env` no S3.

## Monitoramento e Validação

- Verificar que secrets existem: `docker secret ls`.
- Verificar que services os consomem: `docker service inspect mecontrola_server-1 --format '{{.Spec.TaskTemplate.ContainerSpec.Secrets}}'`.
- Validar que a aplicação lê os valores corretamente.

## Impacto em Documentação e Operação

- Atualizar runbook de rotação de segredos.
- Documentar script de backup do `.env`.
- Incluir na checklist de deploy.

## Revisão Futura

Revisitar quando:
- Houver orçamento para Vault ou secret manager externo.
- A lista de secrets críticos crescer e justificar 100% Docker secrets.
