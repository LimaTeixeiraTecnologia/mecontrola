# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Seleção de mensagem de rejeição por reason dentro do use case, consumer intacto
- **Data:** 2026-08-07
- **Status:** Aceita
- **Decisores:** Usuário (dono do produto), agente de engenharia
- **Relacionados:** `.specs/prd-limite-audio-20-segundos-whatsapp/prd.md` (RF-03, RF-04), `techspec.md`, ADR-003

## Contexto

Hoje todas as rejeições pré-STT de áudio respondem com a mesma mensagem genérica `WA_MSG_AUDIO_REJECTED_RETRY`, escolhida por `replyFor(outcome)` em `internal/agents/application/usecases/process_audio_inbound.go:486-493`, e o consumer apenas envia `result.ReplyText` (`whatsapp_inbound_consumer.go:274-276`). O PRD exige mensagem específica informando o limite de 20 segundos quando o motivo for `duration_exceeded` (RF-03), mantendo as demais mensagens inalteradas (RF-04) e sem branching de domínio no consumer (regra de adapter fino do AGENTS.md).

## Decisão

Estender a seleção de mensagem no use case: `replyFor` passa a receber `(outcome AudioOutcome, reason AudioReason)` e, quando `reason == AudioReasonDurationExceeded`, retorna a nova configuração `WA_MSG_AUDIO_DURATION_EXCEEDED` com fallback para a constante `defaultAudioDurationExceededReply` (texto informal com emoji, decisão do usuário). `resultFromRecord` (`:471-484`) repassa `record.Reason`, que já é persistido na auditoria. A nova configuração é registrada em `AgentConfig`, `envKeys()`, validações base e de produção (não vazia) e defaults, espelhando o padrão das duas mensagens existentes. O consumer não recebe nenhuma alteração.

## Alternativas Consideradas

1. Branching por reason no consumer. Vantagem: zero mudança no use case. Desvantagem: viola a regra de adapter fino (R-ADAPTER-001), colocando decisão de domínio na porta de entrada. Rejeitada.
2. Mapa estático reason -> mensagem sem configuração. Vantagem: menos uma env var. Desvantagem: impede ajuste de copy por ambiente e diverge do padrão já estabelecido pelas duas mensagens configuráveis existentes. Rejeitada.
3. Mensagem única genérica reescrita para mencionar duração em todos os motivos. Desvantagem: informaria o motivo errado para falhas de formato, tamanho, custo ou mídia, gerando confusão e falso positivo de diagnóstico. Rejeitada.

## Consequências

### Benefícios Esperados

- Usuário sabe exatamente o motivo da rejeição e como corrigir, reduzindo reenvios longos repetidos.
- Consumer e contrato `AudioInboundResult` permanecem idênticos; nenhum teste de consumer precisa mudar.
- Padrão de configuração de mensagens segue o existente, sem convenção nova.

### Trade-offs e Custos

- Assinatura interna de `replyFor` muda; o único call-site é `resultFromRecord:482`, mantendo o raio de impacto mínimo.
- Mais uma chave de configuração para operar, com o risco clássico de esquecer o registro em `envKeys()`; mitigado por teste de override por env.

### Riscos e Mitigações

- Risco: divergência entre o número na mensagem e o limite efetivo alterado via env. Mitigação: mensagem também configurável por ambiente e nota de acoplamento no runbook; os dois valores são revisados juntos em qualquer ajuste operacional.
- Risco: server e worker divergirem por wiring esquecido em um entrypoint. Mitigação: repasse aplicado em `cmd/server/server.go:247-258` e `cmd/worker/worker.go:444-455` no mesmo passo, com build dos dois binários como gate.
- Plano de rollback: como a mudança é aditiva (campo novo com default), basta reverter o commit; em runtime, definir a env var com outro texto sem deploy.

## Plano de Implementação

1. Adicionar constante default e campo em `ProcessAudioInboundConfig`.
2. Estender `replyFor` e `resultFromRecord`; adicionar asserções de `ReplyText` nos testes do use case, incluindo não regressão dos demais motivos.
3. Registrar a config em `configs/config.go` (struct, `envKeys()`, validações, default) com testes espelhados.
4. Repassar o campo em `module.go`, `cmd/server` e `cmd/worker`.
5. Critério de conclusão: testes novos e existentes passando, consumer sem diff.

## Monitoramento e Validação

- Sucesso: rejeições `duration_exceeded` respondem com a mensagem dedicada, verificável em teste golden com `ReplyText` assertado e em produção pela taxa de reenvio dentro do limite observada manualmente via auditoria.
- Critério de revisão: reclamações de usuários ou taxa de reenvio longo repetido indicam copy inadequada e justificam ajuste da mensagem via env.

## Impacto em Documentação e Operação

- `deployment/runbooks/audio-whatsapp-stt.md`: nova linha na tabela de configs (após `:85`) e nota na tabela de reasons (`:63`) indicando a resposta dedicada; lista de pré-requisitos de canary (`:340-341`).
- `.env.example` e `deployment/config/prod.env` ganham `WA_MSG_AUDIO_DURATION_EXCEEDED`.

## Revisão Futura

Revisar junto com a ADR-001 (30 dias) ou antes, se a taxa de reenvio fora do limite indicar que a mensagem não está educando o usuário.
