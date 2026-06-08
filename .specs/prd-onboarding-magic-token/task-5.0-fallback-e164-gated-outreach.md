# Tarefa 5.0: Fallback de ativacao por E.164 condicionado a outreach

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar fallback de ativacao automatica por match do numero E.164 digitado no checkout, com pre-condicao obrigatoria de outreach ja enviado e reutilizando a mesma transacao de ativacao plena.

<requirements>
- Cobrir `RF-07`, `RF-10`, `RF-13`, `RF-14`, `RF-16`.
- Mensagem qualquer que nao seja `ATIVAR <token>` so pode ativar se houver token `PAID` para o mesmo E.164 com `outreach_sent_at IS NOT NULL`.
- Mensagem de numero casado sem outreach previo nao ativa automaticamente e deve orientar uso de `ATIVAR <token>`.
- Ativacao fallback deve registrar `activation_path='fallback_e164'` e seguir a mesma transacao de `RF-07`.
- Numeros fora do espaco E.164 BR devem ser rejeitados com mensagem amigavel e log mascarado.
- A execucao posterior deve carregar obrigatoriamente `go-implementation`, carregar exemplos apenas sob demanda, verificar `go.mod` antes de usar recursos da linguagem, partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nao usar `internal/platform/runtime` como ponto de partida e nao adicionar comentarios em arquivos Go.
</requirements>

## Subtarefas

- [ ] 5.1 Implementar `TryFallbackActivation` na camada de aplicacao.
- [ ] 5.2 Adicionar consulta de token pago por mobile com outreach ja enviado.
- [ ] 5.3 Integrar fallback ao `WhatsAppInboundHandler` para mensagens nao-ATIVAR.
- [ ] 5.4 Reutilizar fluxo transacional de consumo com activation path correto.
- [ ] 5.5 Emitir metricas e logs mascarados para fallback, recusas e pais nao suportado.
- [ ] 5.6 Testar casos com outreach, sem outreach, numero divergente e token expirado.

## Detalhes de Implementação

Referenciar `techspec.md` secoes 2, 5.2, 7.2, 8.5, 9.2 e ADR-008. A divergencia deliberada da discovery deve ser preservada: fallback sem outreach e proibido.

## Critérios de Sucesso

- Fallback so ativa token pago com outreach ja registrado.
- Sem outreach previo, nenhuma subscription e vinculada.
- A ativacao fallback publica o mesmo evento `onboarding.subscription_bound`.
- `onboarding_tokens_consumed_total{activation_path="fallback_e164"}` e incrementada no caminho correto.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitarios de `TryFallbackActivation`.
- [ ] Testes de handler inbound para mensagem comum com e sem outreach.
- [ ] `go test -race -count=1 ./internal/onboarding/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/application/usecases/try_fallback_activation.go`
- `internal/onboarding/application/usecases/consume_magic_token.go`
- `internal/onboarding/infrastructure/http/server/handlers/whatsapp_inbound_handler.go`
- `internal/onboarding/infrastructure/repositories/postgres/magic_token_repository.go`
