# Tarefa 5.0: Criar Scripts de Docker Secrets e Backup do .env para S3

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar scripts para gerenciar Docker secrets no Swarm a partir do `.env` e realizar backup do `.env` para o S3 a cada deploy.

<requirements>
- Cobrir RF-10: Docker secrets para segredos sensíveis.
- Cobrir RF-11: backup do `.env` no S3 criptografado a cada alteração.
</requirements>

## Subtarefas

- [ ] 5.1 Criar `deployment/scripts/create-secrets.sh` que lê `.env` e cria/atualiza Docker secrets.
- [ ] 5.2 Definir lista de segredos críticos (DB_PASSWORD, META_ACCESS_TOKEN, META_APP_SECRET, KIWIFY_WEBHOOK_SECRET, KIWIFY_CLIENT_SECRET, OPENROUTER_API_KEY, ONBOARDING_TOKEN_ENCRYPTION_KEY, IDENTITY_GATEWAY_SHARED_SECRET_CURRENT/NEXT).
- [ ] 5.3 Implementar rotação de secrets no Swarm (secrets imutáveis exigem novo nome).
- [ ] 5.4 Criar `deployment/scripts/backup-env-s3.sh` para upload do `.env` para S3 (SSE-S3).
- [ ] 5.5 Configurar IAM restricto permitindo apenas `PutObject` no prefixo `mecontrola-env-backups/`.
- [ ] 5.6 Integrar ambos os scripts em `deploy.sh` e `deploy-local.sh`.
- [ ] 5.7 Atualizar `compose.swarm.yml` para montar os secrets nos services.

## Detalhes de Implementação

Ver seção "4. Segredos e Configuração" e ADR-003 de `techspec.md`. A estratégia é híbrida: Docker secrets para valores críticos e `.env` no host para configurações não sensíveis.

Rotação de secrets no Swarm:

```bash
create_or_rotate_secret() {
  local name="$1"
  local value="$2"
  local new_name="${STACK}_${name}_$(date +%s)"
  printf '%s' "$value" | docker secret create "$new_name" -
  docker service update \
    --secret-rm "${STACK}_${name}" \
    --secret-add "source=${new_name},target=${name}" \
    "${STACK}_server-1"
  # ... demais services
}
```

> A rotação deve ser feita service por service para evitar downtime. Versões antigas de secrets podem ser removidas após deploy bem-sucedido.

## Critérios de Sucesso

- `bash deployment/scripts/create-secrets.sh .env` cria todos os secrets sem erros.
- `docker secret ls` mostra secrets no formato `mecontrola_<nome>`.
- `bash deployment/scripts/backup-env-s3.sh .env` faz upload para S3 com SSE-S3.
- Services conseguem ler os secrets como variáveis de ambiente.
- `.env` no host tem permissão 600.

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste sintático dos scripts com `shellcheck`.
- [ ] Teste em staging: criar secrets, fazer deploy e verificar que a aplicação lê os valores.
- [ ] Teste de backup: verificar arquivo no S3 após deploy.
- [ ] Teste de IAM: confirmar que a credencial não consegue listar/deletar objetos fora do prefixo.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.env` (não versionado)
- `deployment/scripts/create-secrets.sh` (novo)
- `deployment/scripts/backup-env-s3.sh` (novo)
- `deployment/scripts/deploy.sh`
- `deployment/scripts/deploy-local.sh`
- `deployment/compose/compose.swarm.yml`
