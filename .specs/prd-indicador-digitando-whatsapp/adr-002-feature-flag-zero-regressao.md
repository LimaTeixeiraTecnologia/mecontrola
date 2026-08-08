# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Rollout por feature flag com default desligado e operação separada como contrato de zero regressão
- **Data:** 2026-08-07
- **Status:** Aceita
- **Decisores:** usuário (rodada de múltipla escolha na US), agente de especificação
- **Relacionados:** `.specs/prd-indicador-digitando-whatsapp/prd.md` (RF-05, RF-06, RF-09), `techspec.md`, `adr-001`, `adr-003`

## Contexto

O requisito não negociável do pedido é zero regressão: a feature não pode alterar nenhum comportamento observável até ser ligada deliberadamente. O codebase tem um precedente exato de flag booleana de canal: `AGENT_AUDIO_ENABLED` com default `false` (`configs/config.go:192`, `:561`, `:1440`). Os testes existentes (unitários com asserts exatos de chamadas, integração com `.Once()` e e2e com contagem de mensagens) quebrariam se o typing fosse implementado como envio pelo mesmo caminho de `SendTextMessage`.

## Decisão

A emissão do indicador é controlada pela configuração `AGENT_WHATSAPP_TYPING_INDICATOR_ENABLED` em `AgentConfig`, com `SetDefault` `false`, propagada por `cmd/worker/worker.go` em `agents.Deps` até a option `WithTypingIndicator` do consumer. Com a flag desligada, o consumer nem sequer chama o sender. A operação de typing é separada de `SendTextMessage` em todos os níveis (método próprio no client, no gateway e na interface do consumidor), de modo que nenhuma contagem de mensagens existente seja afetada. A interface pública de onboarding `interfaces.WhatsAppGateway` e seus mocks gerados não são alterados, pois nenhum consumidor de onboarding usa typing.

## Alternativas Consideradas

- Sempre ligado, sem flag.
  - Vantagens: menos configuração.
  - Desvantagens: qualquer problema vira incidente sem botão de desligar; viola o requisito de zero regressão na ativação.
  - Motivo da rejeição: rejeitado pelo usuário na rodada da US.
- Estender `SendTextMessage` ou enviar o typing como mensagem comum pelo gateway existente.
  - Vantagens: nenhum método novo.
  - Desvantagens: quebra mocks com asserts exatos, contagens de e2e e o contrato de resposta do client; typing não é mensagem de negócio.
  - Motivo da rejeição: falso positivo estrutural de regressão.
- Adicionar o método na interface pública de onboarding e regenerar mocks via mockery.
  - Vantagens: interface única do módulo onboarding.
  - Desvantagens: nenhum use case de onboarding consome o método; interface inflada contra R6.3 (interface no consumidor) e regeneração de mock sem necessidade.
  - Motivo da rejeição: custo sem consumidor real.

## Consequências

### Benefícios Esperados

- Rollback instantâneo sem deploy, apenas desligando a variável de ambiente.
- A suíte existente passa sem nenhuma alteração de assert, servindo como evidência objetiva de zero regressão.
- Ativação gradual por ambiente: desenvolvimento, staging e produção em momentos independentes.

### Trade-offs e Custos

- Uma variável de configuração a mais para operar e documentar.
- O caminho com flag ligada só é exercitado em produção após ativação deliberada; até lá, a cobertura vem de testes unitários e de integração.

### Riscos e Mitigações

- Risco: flag ligada por engano em ambiente errado. Impacto: baixo, apenas bolhas de digitação a mais. Mitigação: default false e revisão de `.env` por ambiente. Rollback: desligar a variável.
- Risco: drift entre o caminho testado (flag ligada em testes) e o caminho de produção (flag desligada). Mitigação: ambos os caminhos cobertos por testes; ativação em staging antes de produção.

## Plano de Implementação

1. Adicionar a chave em `AgentConfig`, na lista de chaves conhecidas e nos defaults de `configs/config.go`, com teste de config.
2. Propagar em `agents.Deps` e na option do consumer.
3. Documentar a variável em `.env.example`.
4. Critério de conclusão: suíte existente verde sem alteração de asserts e testes novos cobrindo os dois estados da flag.

## Monitoramento e Validação

- Após ativação por ambiente: contador `agents_whatsapp_inbound_typing_total` passa a registrar emissões; antes da ativação, o contador permanece em zero, o que também valida que o caminho desligado está inerte.
- Critério de sucesso: nenhuma variação em `agents_whatsapp_inbound_total` (latência e outcome) após ligar a flag.

## Impacto em Documentação e Operação

- `.env.example` e `configs/config_test.go` atualizados.
- Operação: procedimento de rollout e rollback é apenas alterar a variável e reiniciar o worker.

## Revisão Futura

- Revisar a remoção da flag quando a feature estiver estável em produção por um ciclo completo de observação e a métrica de saúde estiver dentro da meta.
