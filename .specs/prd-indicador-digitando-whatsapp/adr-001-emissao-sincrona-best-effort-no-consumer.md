# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Emissão síncrona e best-effort do typing indicator no início do consumer do worker
- **Data:** 2026-08-07
- **Status:** Aceita
- **Decisores:** usuário (rodadas de múltipla escolha na US e no PRD), agente de especificação
- **Relacionados:** `.specs/prd-indicador-digitando-whatsapp/prd.md`, `.specs/prd-indicador-digitando-whatsapp/techspec.md`, `docs/us/US-001-indicador-digitando-whatsapp.md`, `adr-002`, `adr-003`

## Contexto

O indicador de digitação precisa aparecer antes da resposta final e nunca pode degradar o fluxo de resposta. O processamento inbound é assíncrono: o webhook responde 200 no processo server e o trabalho real acontece no worker via outbox (`internal/agents/module.go:585-604` e `whatsapp_inbound_consumer.go:173`). Dois pontos de emissão eram possíveis (server ou worker) e dois modos de execução (síncrono ou assíncrono em goroutine). A chamada à Meta usa um client sem retry (`client.go:123`) e uma falha dela não pode atrasar nem bloquear a resposta (restrição do PRD).

## Decisão

Emitir de forma síncrona no `WhatsAppInboundConsumer.Handle`, em um ponto único logo após o sucesso do dedup por WAMID (`whatsapp_inbound_consumer.go:214`) e antes do `WithTimeout` de processamento, cobrindo texto, áudio, retomada de workflow e onboarding com um único elo. A emissão usa contexto com timeout dedicado curto (3 segundos), independente do timeout de inbound de 60 a 90 segundos, para limitar o acréscimo máximo de latência. Qualquer falha é tratada uma única vez no ponto de emissão: log `Warn` com `message_id`, incremento do contador `agents_whatsapp_inbound_typing_total` com `outcome="error"` e continuação do fluxo, sem retry e sem propagação de erro.

## Alternativas Consideradas

- Emissão no processo server, no momento do webhook.
  - Vantagens: menor latência até a bolha aparecer.
  - Desvantagens: exige wiring novo de envio no server, que hoje só publica no outbox; maior acoplamento e superfície de regressão.
  - Motivo da rejeição: rejeitada pelo usuário na rodada da US; o ganho de latência não compensa o novo acoplamento.
- Emissão assíncrona em goroutine.
  - Vantagens: zero acréscimo de latência no caminho da resposta.
  - Desvantagens: a bolha pode chegar depois da resposta em mensagens rápidas, invertendo o sinal; exige goroutine cancelável integrada ao shutdown coordenado (regra do AGENTS.md) em um consumer que hoje não tem lifecycle próprio; mais testes de concorrência.
  - Motivo da rejeição: o timeout dedicado curto já limita o custo do modo síncrono, com complexidade muito menor.
- Reutilizar `SendTextMessage` ou o fluxo `doSend` do client Meta.
  - Vantagens: menos código novo.
  - Desvantagens: o payload e a resposta do typing são diferentes (sem campo `to`, resposta `{"success": true}` sem `messages[]`); reutilizar quebraria o contrato de resposta de `doSend` (client.go:148-152) e inflaria contagens de mensagens em mocks e e2e.
  - Motivo da rejeição: violação de contrato e falso positivo de contagem.

## Consequências

### Benefícios Esperados

- Um único ponto de emissão cobre todos os fluxos com resposta, sem branching por tipo de mensagem.
- Latência máxima adicionada fica limitada pelo timeout dedicado, mesmo com a Meta lenta.
- Falha do indicador é impossível de propagar para a resposta por construção.

### Trade-offs e Custos

- A bolha chega com o atraso do polling do outbox entre server e worker.
- Em processamentos acima de 25 segundos a bolha expira antes da resposta (limite da Meta, aceito no PRD).
- Se o processamento falhar após a emissão e o dedup for compensado, a reentrega emite a bolha de novo; aceito por ser um sinal honesto de nova tentativa.

### Riscos e Mitigações

- Risco: Meta lenta adiciona latência até o timeout dedicado. Impacto: baixo e limitado. Mitigação: timeout de 3 segundos e ausência de retry. Rollback: desligar a flag (adr-002).
- Risco: versão pinada da Graph API rejeitar o campo. Mitigação: gate RF-07 antes da ativação.

## Plano de Implementação

1. Adicionar os tipos de payload e o método no client Meta com testes httptest.
2. Adicionar o método no gateway de onboarding.
3. Adicionar interface de consumidor, option, contador e ponto de emissão no consumer com testes unitários.
4. Estender a interface local do módulo agents e o stub de boot.
5. Critério de conclusão: suíte dos pacotes alterados verde com a flag desligada e ligada.

## Monitoramento e Validação

- Contador `agents_whatsapp_inbound_typing_total` com `outcome` em {success, error}; meta de sucesso acima de 99%.
- `agents_whatsapp_inbound_total` existente sem variação de latência ou de erro após a ativação.
- Critério de reversão: taxa de erro do typing sustentada acima de 1% ou qualquer aumento de erro da resposta final.

## Impacto em Documentação e Operação

- `.env.example` documenta a variável nova.
- Runbooks: nenhum novo; o rollback é a flag.

## Revisão Futura

- Revisar ao medir a latência real inbound até resposta em produção; se processamentos longos forem frequentes, avaliar renovação periódica da bolha em nova ADR.
- Revisar se a Meta mudar o contrato do endpoint ou se houver bump de versão da Graph API.
