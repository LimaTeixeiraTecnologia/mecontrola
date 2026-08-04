# Registro de Decisao Arquitetural (ADR)

## Metadados

- **Titulo:** Outcome terminal por WAMID para audio
- **Data:** 2026-08-04
- **Status:** Aceita
- **Decisores:** Engenharia Me Controla
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O dispatcher grava dedup por WAMID antes de rotear em `internal/platform/whatsapp/dispatcher/dispatcher.go:132`.
O consumer tambem possui dedup e compensacao em erro em
`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:159` e
`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:190`.

Para audio, uma falha de download ou STT nao pode permitir reprocessamento automatico futuro do mesmo
WAMID, porque isso pode transformar um retry em mutacao financeira tardia.

## Decisao

Toda mensagem de audio deve produzir outcome terminal unico por WAMID na auditoria de audio. Falhas de
download, validacao, STT, incerteza tecnica e custo acima do budget sao terminais para aquele WAMID.
Reenvio pelo usuario so sera processado quando chegar com novo WAMID.

## Alternativas Consideradas

- Compensar dedup em qualquer erro: rejeitado por risco de mutacao tardia em replay.
- Reprocessar automaticamente STT falho: rejeitado por custo, duplicidade e falta de controle do usuario.
- Ignorar auditoria e confiar apenas no dispatcher: rejeitado porque nao preserva reason/outcome tecnico.

## Consequencias

### Beneficios Esperados

- Idempotencia forte para audio.
- Menor risco de duplicidade financeira.
- Diagnostico operacional por WAMID sem reter audio bruto.

### Trade-offs e Custos

- Alguns erros transientes exigem que o usuario envie novo audio.
- Precisa de tabela de auditoria e testes de integracao.

### Riscos e Mitigacoes

- Risco: falha transiente causar friccao.
- Mitigacao: resposta textual clara pedindo reenvio e metricas para operar taxa de falha.

## Plano de Implementacao

1. Criar tabela com `wamid` como PK.
2. Inserir linha antes do download.
3. Atualizar outcome terminal em transacao curta.
4. Bloquear `dedup.Delete` para falhas de audio ja terminalizadas.

## Monitoramento e Validacao

- Gate de integracao: mesmo WAMID nao chama STT nem `HandleInbound` duas vezes.
- Gate golden: falso-sucesso mutacional por audio igual a `0`.

## Impacto em Documentacao e Operacao

Runbook deve orientar usuario a reenviar audio em falha terminal e orientar engenharia a investigar por
outcome/reason, nao por audio bruto.

## Revisao Futura

Revisar se a taxa de falhas transientes terminais ultrapassar threshold pos-deploy definido.
