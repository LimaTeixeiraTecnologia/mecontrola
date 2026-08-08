# US-002: Aceite de áudio de voz limitado a 20 segundos no WhatsApp

## Declaração
Como usuário do mecontrola que registra gastos por mensagens de voz no WhatsApp, quero que áudios de até 20 segundos sejam aceitos e transcritos, recebendo um aviso claro quando o áudio passar do limite, para ter resposta rápida do assistente e saber exatamente como corrigir quando exceder a duração permitida.

## Contexto
- Problema: o pipeline de áudio já existe e está ativo em produção (`deployment/config/prod.env:215`), mas o limite de duração aceito hoje é de 60 segundos (`configs/config.go:1441`, `deployment/config/prod.env:218`). Áudios longos elevam o custo de transcrição STT via OpenRouter, aumentam a latência até a resposta e ampliam a janela de falhas de download e processamento, sem benefício proporcional para o registro de gastos, que é uma interação curta por natureza.
- Resultado esperado: apenas áudios de até 20 segundos seguem para transcrição; áudios acima de 20 segundos são rejeitados com mensagem específica que informa o limite; o restante do fluxo de áudio e de texto permanece idêntico ao comportamento atual.
- Fonte: pedido direto do usuário com limite explícito de 20 segundos, documentação oficial da OpenAI sobre transcrição e confronto com o codebase.

## Regras de Negócio
- RN1: a duração máxima aceita passa a ser 20 segundos, aplicada pela configuração existente `AGENT_AUDIO_MAX_DURATION` com enforcement já implementado em `internal/agents/application/usecases/process_audio_inbound.go:303` via `media.CheckMaxDuration` (`internal/platform/whatsapp/media/duration.go:19`). Nenhum mecanismo novo de validação de duração é criado.
- RN2: o default do código muda de 60s para 20s em `configs/config.go:1441` e os ambientes são alinhados: `deployment/config/prod.env:218` e `.env.example:240` passam a `AGENT_AUDIO_MAX_DURATION=20s`. A faixa de validação `[1s..60s]` de `configs/config.go:1062` é mantida, permitindo rollback ou ajuste operacional por variável de ambiente sem novo deploy.
- RN3: a rejeição por duração excedida (`AudioReasonDurationExceeded`) passa a responder com mensagem específica que informa o limite de 20 segundos, via nova configuração `WA_MSG_AUDIO_DURATION_EXCEEDED` com default em texto claro. A seleção ocorre no use case, estendendo `replyFor` (`process_audio_inbound.go:486`) para decidir também por `Reason`, sem branching de domínio no consumer, que continua apenas enviando `result.ReplyText` (`whatsapp_inbound_consumer.go:275`). Os demais motivos de rejeição mantêm as mensagens atuais `WA_MSG_AUDIO_UNCERTAIN_RETRY` e `WA_MSG_AUDIO_REJECTED_RETRY` inalteradas.
- RN4: a rejeição por duração continua auditada na tabela `mecontrola.agents_whatsapp_audio_messages` com outcome `rejected`, reason `duration_exceeded` e `duration_ms` preenchido (`process_audio_inbound.go:302-309`, `migrations/000015_agents_whatsapp_audio_messages.up.sql`), preservando a idempotência por WAMID e a observabilidade existente.
- RN5: zero regressão funcional fora do limite: o fluxo de mensagens de texto, os formatos aceitos (OGG/Opus e M4A/AAC em `process_audio_inbound.go:614-624`), os limites de tamanho (`AGENT_AUDIO_MAX_BYTES`), custo (`AGENT_AUDIO_MAX_COST_MICROUSD`) e confiança (`AGENT_AUDIO_MIN_CONFIDENCE`), a feature flag `AGENT_AUDIO_ENABLED` e o reinject do texto transcrito no fluxo canônico `HandleInbound` permanecem inalterados.
- RN6: a fronteira de aceite é inclusiva: áudio com duração de exatamente 20 segundos é aceito; somente duração estritamente acima de 20 segundos é rejeitada, conforme a semântica já existente de `media.CheckMaxDuration`.

## Critérios de Aceite
```gherkin
Cenário: áudio curto dentro do limite é transcrito e processado
  Dado que a feature flag AGENT_AUDIO_ENABLED está ligada
  E que AGENT_AUDIO_MAX_DURATION está configurado em 20 segundos
  Quando o usuário envia um áudio de voz de 15 segundos em formato OGG
  Então o áudio é baixado, a duração é verificada e o áudio segue para transcrição
  E o texto transcrito é reinjetado no fluxo canônico do assistente como mensagem de texto

Cenário: áudio na fronteira exata de 20 segundos é aceito
  Dado que AGENT_AUDIO_MAX_DURATION está configurado em 20 segundos
  Quando o usuário envia um áudio de voz com duração de exatamente 20 segundos
  Então a verificação de duração aprova o áudio
  E o fluxo de transcrição e resposta ocorre normalmente

Cenário: áudio acima de 20 segundos é rejeitado com aviso do limite
  Dado que AGENT_AUDIO_MAX_DURATION está configurado em 20 segundos
  Quando o usuário envia um áudio de voz de 35 segundos
  Então o áudio é rejeitado antes da transcrição
  E o usuário recebe a mensagem específica informando que o limite é de 20 segundos
  E a auditoria registra outcome rejected com reason duration_exceeded e duration_ms preenchido

Cenário: ambiente sem a variável explícita aplica o novo default de 20 segundos
  Dado que a variável AGENT_AUDIO_MAX_DURATION não está definida no ambiente
  Quando a aplicação sobe com a configuração padrão
  Então o limite efetivo de duração é 20 segundos
  E um valor de ambiente dentro da faixa de 1s a 60s continua sendo aceito pelo boot para rollback operacional

Cenário: mensagem de texto permanece inalterada
  Dado que a nova configuração de 20 segundos está ativa
  Quando o usuário envia uma mensagem de texto válida
  Então o fluxo de resposta é idêntico ao comportamento atual de produção
  E nenhuma validação de duração é executada
```

## Dados e Permissões
- Dados obrigatórios: duração do áudio derivada dos bytes do container de mídia, pois o payload do webhook da Meta não traz duração; o parse já existente cobre Ogg em `internal/platform/whatsapp/media/duration_ogg.go` e M4A em `internal/platform/whatsapp/media/duration_m4a.go`.
- Perfis/permissões: nenhum perfil novo. Nenhuma credencial nova; o pipeline reutiliza as credenciais Meta e OpenRouter já configuradas.

## Dependências
- Nenhuma dependência nova de código, biblioteca ou serviço: o enforcement de duração, a auditoria e o envio de resposta já existem em produção.
- Transcrição via OpenRouter com modelo `openai/whisper-large-v3` já operante (`.env.example:238`); a documentação oficial da OpenAI confirma que a API de transcrição aceita arquivos de até 25 MB e não impõe teto de duração, portanto o limite de 20 segundos é regra de produto do mecontrola, não restrição do provedor.
- Atualização documental do runbook `deployment/runbooks/audio-whatsapp-stt.md`, que hoje registra o default de 60s na tabela de configurações (linha 80), para refletir o novo default.

## Fora de Escopo
- Suporte a novos formatos de áudio como MP3 ou WAV.
- Redução do teto de validação de `[1s..60s]` para `[1s..20s]` ou qualquer endurecimento que impeça ajuste operacional por variável de ambiente.
- Mudança nos limites de tamanho, custo ou confiança, na feature flag ou nas mensagens dos demais motivos de rejeição.
- Rejeição de áudio longo antes do download, pois a duração só é conhecida após o parse dos bytes baixados; o consumo de download segue mitigado por `AGENT_AUDIO_MAX_BYTES`.
- Envio de áudio outbound pelo assistente.
- Suporte a imagem, vídeo ou documento no inbound.

## Ganhos e Perdas
- Ganhos: custo de STT previsível e limitado por mensagem, já que a cobrança de transcrição cresce com a duração do áudio; latência de ponta a ponta menor e mais estável para o usuário; diff mínimo e cirúrgico, pois o mecanismo de enforcement, a auditoria e os testes do pipeline já existem; rollback instantâneo por variável de ambiente sem deploy; rejeições por duração já são mensuráveis na tabela de auditoria, permitindo monitorar o impacto da mudança desde o primeiro deploy.
- Perdas: usuários que hoje enviam áudios entre 20 e 60 segundos passam a ser rejeitados, o que é uma mudança deliberada de comportamento e precisa de acompanhamento via métrica de `duration_exceeded`; a duração só é conhecida após o download completo, então um áudio longo consome banda de download até o teto de `AGENT_AUDIO_MAX_BYTES` antes da rejeição; a mensagem específica exige estender `replyFor` para decidir por `Reason`, um branching pequeno mas novo no use case; o runbook e os exemplos de ambiente precisam ser atualizados no mesmo ciclo para não documentar o default antigo.

## Skills Obrigatórias de Implementação
- `go-implementation`: entrypoint canônico da alteração Go, com classificação `usecase-write` para a extensão de `replyFor` e validação proporcional dos pacotes `configs` e `internal/agents/application/usecases`.
- `mastra`: a decisão de resposta vive no use case do fluxo Thread-Run, e o consumer permanece adapter fino apenas enviando `result.ReplyText`; nenhum primitivo de `internal/platform/{agent,memory,workflow}` é reimplementado.
- `domain-modeling-production`: aplicação dos princípios DMMF, mantendo `AudioOutcome` e `AudioReason` como tipos fechados e a decisão por motivo dentro do use case; não há novo agregado, comando ou evento, portanto sem materialização de bundle.
- `design-patterns-mandatory`: gate de desenho com decisão explícita; a expectativa, dada a natureza de configuração mais seleção de mensagem por motivo, é `não aplicar padrão`, mantendo código direto sem novas abstrações.

## Evidências
- Entrada: pedido do usuário para aceitar áudio de no máximo 20 segundos, com foco em zero regressão, production-ready e sem respostas inventadas.
- Base de código: pipeline de áudio completo e ativo em produção com `AGENT_AUDIO_ENABLED=true` (`deployment/config/prod.env:215`); limite atual de 60s no default (`configs/config.go:1441`) e em produção (`deployment/config/prod.env:218`); validação da faixa `[1s..60s]` em `configs/config.go:1062`; enforcement de duração em `internal/agents/application/usecases/process_audio_inbound.go:303` com rejeição auditada em `:302-309`; seleção atual de mensagem por outcome em `process_audio_inbound.go:486-492`; envio da resposta pelo consumer em `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:275`; tabela de auditoria em `migrations/000015_agents_whatsapp_audio_messages.up.sql`; formatos aceitos em `process_audio_inbound.go:614-624`; teste de validação da faixa em `configs/config_test.go:949`.
- Documentação oficial: [guia oficial de speech-to-text da OpenAI](https://developers.openai.com/api/docs/guides/speech-to-text), que define limite de 25 MB por arquivo e nenhum teto de duração, confirmando que o limite de 20 segundos é regra de produto; a estrutura do payload de áudio do webhook da Meta é evidenciada pelo parser do repositório em `internal/platform/whatsapp/payload/types.go:64-85`.
- Inferências: a extensão de `replyFor` para decidir por `Reason` é inferência de design para entregar a mensagem específica sem branching no consumer, confirmada como direção pelo usuário na rodada de múltipla escolha junto com a estratégia config mais default 20s.
- Não evidenciado: duração do áudio no payload do webhook da Meta; a busca no parser e no cliente de mídia confirmou que a duração é derivada apenas dos bytes baixados, portanto a rejeição antes do download é tecnicamente inviável nesta integração e ficou fora de escopo.

## Notas de Validação
- Decisões confirmadas pelo usuário em rodada de múltipla escolha: aplicação do limite por configuração com novo default de 20s no código, mantendo a faixa de validação `[1s..60s]`, e mensagem de rejeição específica informando o limite para o motivo de duração excedida.
- Documentação oficial consultada: guia de speech-to-text da OpenAI, lido na íntegra, corroborando que o provedor não impõe limite de duração e que a regra de 20 segundos é decisão de produto do mecontrola.
- Arquivo validado com `python3 .agents/skills/user-stories/scripts/validar-historias-usuario.py` com resultado de sucesso.
