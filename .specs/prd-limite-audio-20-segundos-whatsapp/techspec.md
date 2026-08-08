<!-- spec-hash-prd: d876d06c905ac89f41a13356af3bb113b44b6aee400410ff4cd56edaa3998b96 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica

## Resumo Executivo

A solução é deliberadamente cirúrgica: o pipeline de áudio do WhatsApp já possui enforcement de duração dirigido por configuração, auditoria por motivo e seleção de mensagem de rejeição no use case. Esta especificação cobre duas mudanças de comportamento e uma de observabilidade: (a) reduzir o limite de aceite de 60s para 20s alterando o default de `AGENT_AUDIO_MAX_DURATION` e os arquivos de ambiente, sem endurecer a faixa de validação `[1s..60s]`, preservando rollback por variável de ambiente; (b) introduzir a configuração `WA_MSG_AUDIO_DURATION_EXCEEDED` e estender a seleção de mensagem em `replyFor` para decidir também por `AudioReason`, mantendo o consumer como adapter fino; (c) adicionar o alerta `mc-audio-duration-exceeded-rate` ao grupo `audio` de alertas Grafana existente, sobre métrica já emitida, sem nenhum código novo de instrumentação.

Nenhum tipo fechado novo, nenhuma migration, nenhum contrato de evento, nenhum endpoint e nenhuma dependência nova são introduzidos. A decisão de design patterns foi submetida ao seletor determinístico (`scripts/select_pattern.py`) e o resultado foi `reject`: solução direta vence em economia, eficiência e robustez (ADR-003).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes modificados (nenhum componente novo):

- `configs/config.go`: novo campo `AudioDurationExceededReply` em `AgentConfig` (após `configs/config.go:198`), nova entrada em `envKeys()` (após `configs/config.go:567`), validação de não vazio em `validateAgentAudio` (bloco `configs/config.go:1076-1082`) e em `validateProductionAudio` (bloco `configs/config.go:1135-1140`), alteração do default em `configs/config.go:1441` de `60*time.Second` para `20*time.Second` e novo `SetDefault` da mensagem após `configs/config.go:1444`.
- `internal/agents/application/usecases/process_audio_inbound.go`: nova constante `defaultAudioDurationExceededReply` (bloco `process_audio_inbound.go:22-32`), novo campo `DurationExceededReply` em `ProcessAudioInboundConfig` (`:39-47`), extensão de `resultFromRecord` (`:471-484`) para repassar `record.Reason` e de `replyFor` (`:486-493`) para aceitar `(outcome, reason)`.
- `internal/agents/module.go`: novo campo em `AudioConfig` (`:86-97`) e repasse no wiring do use case (`:404-412`).
- `cmd/server/server.go` (`:247-258`) e `cmd/worker/worker.go` (`:444-455`): repasse do novo campo nos dois entrypoints, que hoje duplicam o bloco de `AudioConfig`; esquecer um dos dois deixa server e worker divergentes.
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`: novo alerta no grupo `audio` (`:1279`).
- Arquivos de ambiente e documentação: `.env.example` (`:240`, `:244-245`), `deployment/config/prod.env` (`:218`, `:222-223`), `deployment/runbooks/audio-whatsapp-stt.md` (`:63`, `:80`, após `:85`, `:340-341`), `deployment/runbooks/audio-whatsapp-pos-deploy.md` (`:185-188`, cenário 3.4).

Componentes explicitamente NÃO tocados (fronteira de zero regressão):

- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`: continua enviando `result.ReplyText` cegamente (`:274-276`); nenhum branching por reason é adicionado.
- `internal/platform/whatsapp/media/duration.go` (`CheckMaxDuration`, `:19-24`) e os parsers de duração Ogg/M4A.
- `internal/agents/application/usecases/decide_audio_transcription.go`: `AudioOutcome` e `AudioReason` já contêm `duration_exceeded` (`:59`); nenhum valor novo.
- Migrations e tabela `mecontrola.agents_whatsapp_audio_messages`: o CHECK constraint já aceita `duration_exceeded` (`migrations/000016`, `000017`).
- Faixa de validação `[1s..60s]` em `configs/config.go:1062-1067`: permanece para permitir rollback operacional.

Fluxo de dados resultante: webhook Meta -> dispatcher -> outbox `whatsapp.inbound` -> `WhatsAppInboundConsumer.Handle` -> `ProcessAudioInbound.Execute` -> download -> `DetermineDuration` -> `CheckMaxDuration(duration, cfg.MaxDuration)` com `MaxDuration = 20s` -> em excedente, `terminal` persiste auditoria `rejected/duration_exceeded` com `duration_ms` -> `resultFromRecord` -> `replyFor(outcome, reason)` retorna a mensagem dedicada -> consumer envia o texto ao usuário. Nenhuma etapa nova; apenas o valor do limite e a seleção da mensagem mudam.

## Design de Implementação

### Interfaces Chave

Nenhuma interface nova. As alterações de assinatura são internas ao use case:

```go
// configs/config.go: struct AgentConfig (campo novo, apos AudioRejectedReply)
AudioDurationExceededReply string `mapstructure:"WA_MSG_AUDIO_DURATION_EXCEEDED"`

// internal/agents/application/usecases/process_audio_inbound.go
const defaultAudioDurationExceededReply = "esse áudio passou de 20 segundos 🎙️ me manda um mais curtinho, de até 20 segundos?"

// ProcessAudioInboundConfig: campo novo
DurationExceededReply string

// unico call-site: resultFromRecord (:482)
func (p *ProcessAudioInbound) replyFor(outcome AudioOutcome, reason AudioReason) string
```

`replyFor` estendido: quando `reason == AudioReasonDurationExceeded`, retorna `firstNonEmpty(p.cfg.DurationExceededReply, defaultAudioDurationExceededReply)`; os casos atuais por outcome (`TranscriptionUncertain`/`TranscriptionFailed` -> `UncertainReply`; demais -> `RejectedReply`) permanecem com a mesma precedência e semântica.

### Modelos de Dados

Nenhuma alteração de schema, entidade, value object ou evento. A auditoria `AudioAuditRecord` já persiste `Reason` e `DurationMs`; a leitura em `audio_audit_repository.go:160-168` já desserializa o reason.

### Endpoints de API

Nenhum endpoint novo ou alterado. O webhook `/api/v1/whatsapp` e o fluxo outbox/worker permanecem intactos.

## Pontos de Integração

Nenhuma integração nova. Meta Graph API (download de mídia e envio de texto) e OpenRouter STT permanecem com os mesmos clientes, timeouts e credenciais. A documentação oficial da OpenAI confirma que a API de transcrição aceita arquivos de até 25 MB sem teto de duração, portanto o limite de 20 segundos é regra de produto, sem dependência de provedor.

## Abordagem de Testes

### Testes Unitários

Componentes e cenários (todos determinísticos, sem rede, table-driven onde houver matriz):

1. `configs/config_test.go`:
   - Novo teste de default: sem env, `AudioMaxDuration == 20*time.Second` e `AudioDurationExceededReply` igual ao default informal com emoji (hoje não existe teste de default de duração; lacuna coberta nesta entrega).
   - Override por env: `AGENT_AUDIO_MAX_DURATION=30s` é aceito (prova de rollback operacional dentro da faixa).
   - Manter o teste de rejeição fora da faixa (`:949-959`, usa 90s; continua válido).
   - Espelhar os testes de obrigatoriedade de mensagem (`:981-1001`) para `WA_MSG_AUDIO_DURATION_EXCEEDED` vazia, nos blocos base e production (`:1135-1140`).
   - A fixture `newBaseConfig` (`:1861-1864`) ganha o campo novo para os testes de produção passarem.
2. `internal/agents/application/usecases/process_audio_inbound_test.go`:
   - No cenário de duração excedida (`:486-518`), adicionar asserção de `result.ReplyText` igual à mensagem configurada de duração, e variante sem config caindo no default `defaultAudioDurationExceededReply`.
   - Novo cenário de não regressão de mensagem: rejeição por outro motivo pré-STT (ex.: `media_unavailable`, `:474-483`) continua retornando `RejectedReply`; incerto continua retornando `UncertainReply`.
   - Fronteira: fixture com duração exata de 20s é aprovada com `MaxDuration: 20s`; fixture de 20s+1s é rejeitada (as fixtures sintéticas Ogg já existem, `buildOggOpusFixture`, `:414`).
3. `internal/platform/whatsapp/media/duration_test.go`: adicionar casos de fronteira em `TestCheckMaxDuration` (`:91-98`): 20s passa e 21s falha contra max 20s, com `errors.Is(err, media.ErrAudioDurationExceeded)`.

Mocks: apenas os já existentes (`SimulatedTranscriber`, `simulatedMediaClient`, `inMemoryAudioAuditRepository`); nenhum mock novo, nenhuma mudança em `mockery.yml`.

### Testes de Integração

O projeto já tem testes de integração com Postgres para a auditoria de áudio (`audio_audit_repository_integration_test.go`), e o critério do template se mantém atendido (fronteira de IO crítica já coberta). Nenhum teste de integração novo é necessário: o CHECK constraint de `duration_exceeded` já existe e já é exercido em `TestInsertTerminalAcceptsPreSTTRejectionReasons` (`:186-204`). A suíte de integração existente deve ser executada como prova de não regressão.

### Testes E2E e Golden

- `internal/agents/application/golden/audio_pipeline_test.go`: novo cenário negativo `duration_exceeded` com config de 20s e fixture acima de 20s, assertando `Outcome=Rejected`, `Reason=AudioReasonDurationExceeded`, `ReplyText` igual à mensagem dedicada e `transcriber.Calls == 0` (prova de que a rejeição ocorre antes do STT, cobrindo o objetivo primário do PRD).
- As fixtures golden de 1-2s continuam aprovadas com o limite de 20s, provando não regressão do caminho feliz.
- Gates reais (`RUN_REAL_STT`, `STT_REAL_AUDIO_FIXTURE`) permanecem inalterados e fora do escopo desta mudança.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. `configs`: campo, `envKeys()`, validações, defaults e testes de config primeiro, pois todo o resto depende da chave existir e ser lida; sem a entrada em `envKeys()` a env var é silenciosamente ignorada.
2. Use case `ProcessAudioInbound`: constante, campo de config, extensão de `replyFor`/`resultFromRecord` e testes do use case.
3. Wiring: `internal/agents/module.go`, `cmd/server/server.go` e `cmd/worker/worker.go` na mesma alteração, para impedir divergência entre entrypoints.
4. Ambiente e documentação: `.env.example`, `deployment/config/prod.env`, runbooks `audio-whatsapp-stt.md` e `audio-whatsapp-pos-deploy.md`.
5. Observabilidade: alerta `mc-audio-duration-exceeded-rate` em `rules.yaml`.
6. Validação proporcional final conforme matriz do AGENTS.md (escopo `application/` + `configs` + adapter não alterado).

### Dependências Técnicas

Nenhuma dependência bloqueante nova: sem infraestrutura nova, sem serviço externo novo, sem migration. A única dependência operacional é aplicar os valores de ambiente no deploy (prod.env já versionado neste repositório).

## Monitoramento e Observabilidade

- Métrica existente reutilizada: `agents_audio_inbound_total{outcome="rejected",reason="duration_exceeded"}` (já agregada no dashboard `deployment/dashboards/agent-audio-whatsapp.json:98`). Nenhuma instrumentação nova; cardinalidade inalterada.
- Novo alerta `mc-audio-duration-exceeded-rate` no grupo `audio` de `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` (`:1279`): razão entre rejeições `duration_exceeded` e o total de áudios inbound, janela de 1h, threshold inicial conservador de 0.30 (30%) com severidade informativa, explicitamente marcado como ajustável após o primeiro ciclo de observação, pois não existe baseline prévio da taxa.
- Logs: nenhum log novo; a auditoria estruturada existente (`duration_ms`, `outcome`, `reason`, `cost_microusd`) permanece a fonte de diagnóstico.
- Runbook: tabela de configs e tabela de reasons de `audio-whatsapp-stt.md` atualizadas; cenário 3.4 de `audio-whatsapp-pos-deploy.md` reescrito, pois o áudio de 55-60s passa a ser rejeitado por `duration_exceeded` (comportamento esperado novo, não falha).

## Considerações Técnicas

### Decisões Chave

Cada decisão material tem ADR própria neste diretório:

- ADR-001 (`adr-001-limite-20s-via-config-com-default-sem-endurecer-faixa.md`): limite de 20s aplicado por configuração com novo default, mantendo a faixa `[1s..60s]` para rollback sem deploy.
- ADR-002 (`adr-002-selecao-de-mensagem-por-reason-no-usecase.md`): seleção da mensagem de rejeição estendida para decidir por `AudioReason` dentro do use case, com consumer intacto.
- ADR-003 (`adr-003-nao-aplicar-design-pattern.md`): seletor determinístico retornou `reject`; solução direta sem pattern novo.
- ADR-004 (`adr-004-alerta-dedicado-duration-exceeded.md`): monitoramento ativo da taxa de `duration_exceeded` via alerta dedicado, em vez de observação manual.

### Riscos Conhecidos

- Chave nova ausente em `envKeys()`: a env var seria silenciosamente ignorada. Mitigação: a entrada em `envKeys()` é parte obrigatória do passo 1, coberta por teste de override por env; não existe gate de CI de paridade `.env.example` x `config.go`, e essa lacuna fica registrada como limitação conhecida, não como falso positivo.
- Drift entre o número na mensagem (20 segundos) e um eventual limite alterado via env: mitigado pelo fato de a mensagem também ser configurável por ambiente e por nota explícita de acoplamento no runbook; mudança operacional do limite exige revisar a mensagem no mesmo commit de configuração.
- Threshold do alerta sem baseline: mitigado por severidade informativa e revisão após o primeiro ciclo, conforme ADR-004.
- Divergência server x worker: os dois entrypoints duplicam o bloco de `AudioConfig`; mitigado aplicando o repasse nos dois no mesmo passo e cobrindo com build de ambos os binários.
- Falso positivo de regressão em documentação operacional: o cenário 3.4 do runbook de pós-deploy ficaria inválido silenciosamente; está no escopo (RF-08) e listado nos arquivos dependentes.

### Conformidade com Padrões

- AGENTS.md: consumer permanece adapter fino (R-ADAPTER-001); nenhum branching de domínio fora do use case; nenhum comentário em código Go de produção; mensagens novas seguem `log/slog` e erros existentes sem `panic`; nenhum identificador com prefixo `_`.
- DMMF (domain-modeling-production): `AudioOutcome`/`AudioReason` permanecem tipos fechados sem valor novo; a decisão por motivo vive no use case, não em string solta; nenhum agregado, comando ou evento novo, portanto sem materialização de bundle de domínio (fora dos gatilhos de materialização).
- design-patterns-mandatory: gate executado via `scripts/select_pattern.py` com sinais canónicos `prefer_direct_solution`, `single_variant_only`, `low_change_frequency`; resultado `reject` registrado na ADR-003.
- mastra: nenhum primitivo de `internal/platform/{agent,memory,workflow,tool,scorer}` é tocado ou reimplementado; o fluxo Thread-Run canônico é preservado.
- go-implementation: classificação `usecase-write` + `module-wiring` (config + entrypoints); validação proporcional `boundary`: `go build ./...`, `go vet ./...`, `go test -race -count=1 ./configs/... ./internal/agents/... ./internal/platform/whatsapp/media/...`, `golangci-lint run` no escopo, gates de governança existentes.
- R-ERR-001 e R-TEST-001 (agent-governance): mensagens ao usuário dizem o que falhou e a ação possível; testes determinísticos com doubles existentes, caminho feliz e de falha cobertos.

### Arquivos Relevantes e Dependentes

Modificados:

- `configs/config.go` (`:198`, `:567`, `:1062-1067` mantida, `:1076-1082`, `:1135-1140`, `:1441`, após `:1444`)
- `configs/config_test.go` (`:949-959` mantido, `:981-1001` espelhado, `:1861-1864`)
- `internal/agents/application/usecases/process_audio_inbound.go` (`:22-32`, `:39-47`, `:471-484`, `:486-493`)
- `internal/agents/application/usecases/process_audio_inbound_test.go` (`:486-518`, novo cenário de não regressão de mensagem)
- `internal/agents/application/golden/audio_pipeline_test.go` (novo cenário `duration_exceeded`)
- `internal/platform/whatsapp/media/duration_test.go` (`:91-98`, fronteira 20s/21s)
- `internal/agents/module.go` (`:86-97`, `:404-412`)
- `cmd/server/server.go` (`:247-258`), `cmd/worker/worker.go` (`:444-455`)
- `.env.example` (`:240`, após `:245`), `deployment/config/prod.env` (`:218`, após `:223`)
- `deployment/runbooks/audio-whatsapp-stt.md` (`:63`, `:80`, após `:85`, `:340-341`)
- `deployment/runbooks/audio-whatsapp-pos-deploy.md` (`:185-188`)
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` (grupo `audio`, `:1279`)

Somente leitura (fronteira de zero regressão, não tocar):

- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/platform/whatsapp/media/duration.go`, `duration_ogg.go`, `duration_m4a.go`, `client.go`
- `internal/agents/application/usecases/decide_audio_transcription.go`
- `internal/agents/infrastructure/persistence/audio_audit_repository.go` e migrations `000015`-`000018`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_test.go`
- `configs/config.go:1062-1067` (faixa de validação)

### Mapeamento Requisito -> Decisão -> Teste

- RF-01 e RF-02 (aceite até 20s, rejeição acima antes do STT): ADR-001; `TestCheckMaxDuration` com fronteira 20s/21s; cenário golden `duration_exceeded` com `transcriber.Calls == 0`.
- RF-03 (mensagem específica do limite): ADR-002; asserções de `ReplyText` no use case e no golden.
- RF-04 (demais motivos inalterados): ADR-002; cenário de não regressão de mensagem por motivo pré-STT e por incerteza.
- RF-05 (configurável, default 20s, faixa mantida): ADR-001; testes de default, override e rejeição fora da faixa em `config_test.go`.
- RF-06 (auditoria com reason e duração): comportamento existente; prova pela suíte de integração de auditoria e asserções de `FinalizeTerminal` no use case.
- RF-07 (texto e demais validações inalterados): suítes existentes de consumer, use case e golden executadas sem alteração de expectativa.
- RF-08 (documentação operacional atualizada): passo 4 do sequenciamento; revisão dos dois runbooks e dos arquivos de ambiente.
- RF-09 (sem comunicação proativa): nenhuma implementação; ausência de código novo de envio/template é a própria conformidade.
