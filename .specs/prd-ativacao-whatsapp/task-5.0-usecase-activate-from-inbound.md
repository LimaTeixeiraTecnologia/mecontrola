# Tarefa 5.0: Usecase `ActivateFromInbound` + DTO `Validate()`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o usecase que orquestra a ativação a partir da primeira mensagem: correlação por telefone (janela 24h) → fallback transicional por token (borda sem telefone) → no-match com throttle. NÃO envia boas-vindas (desacopladas na tarefa 6.0).

<requirements>
- RF-20/RF-22: qualquer primeira mensagem de número não vinculado dispara ativação (texto-agnóstico).
- RF-23: seleção da Activation Session `PAID` mais recente por telefone.
- RF-26: idempotente e resiliente a mensagens duplicadas (estado do token + throttle).
- RF-30/RF-31: borda sem telefone — interpretar texto como token (com/sem prefixo `ATIVAR`) e consumir por `ActivationPathDirect`; telefone-match usa `ActivationPathFallbackE164`.
- RF-24: no-match → resposta orientativa com throttle por telefone, métrica/audit.
- RF-36/RF-37: logs estruturados (telefone mascarado) e métricas de cardinalidade controlada.
- Pipeline DMMF (Princípio 5); DTO com `Validate()` (R-DTO-VALIDATE-001) logo após o span.
</requirements>

## Subtarefas

- [ ] 5.1 Criar `application/dtos/input/activate_from_inbound_input.go` (`PeerE164`, `Text`, `MessageID`) com `Validate()` (`errors.Join`, nomeia campos).
- [ ] 5.2 Criar `application/usecases/activate_from_inbound.go` com `Execute` decomposto em passos privados: `loadByPhone` (janela) → `tryToken` (borda) → `bind` (via `SubscriptionBindingService.BindAndConsume`) → `noMatch` (throttle + reply).
- [ ] 5.3 Marcar `activation_started_at` (set-once-if-null) no início da ativação.
- [ ] 5.4 Métrica `onboarding_activation_attempt_total{outcome}` e `onboarding_activation_window_expired_total`.

## Detalhes de Implementação

Ver techspec.md, seções "Interfaces Chave" (`ActivateFromInbound`, `ActivateOutcome`), "Fluxo de Dados" e ADR-001/ADR-003. Reusar `subscription_binding.go:41` (`BindAndConsume`) e `consume_magic_token.go` (caminho por token). Sem boas-vindas aqui.

## Critérios de Sucesso

- Cenários cobertos: phone-matched, token-matched (borda), already-active (replay), no-match (com throttle).
- No caminho de sucesso publica `onboarding.subscription_bound`; no no-match não ativa e responde no máximo 1x por telefone/janela.
- DTO `Validate()` rejeita `PeerE164`/`MessageID` vazios.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unitários (testify/suite, IIFE por mock, `fake.NewProvider()`) dos 4 outcomes + `Validate()`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/application/usecases/activate_from_inbound.go` (novo)
- `internal/onboarding/application/dtos/input/activate_from_inbound_input.go` (novo)
- `internal/onboarding/application/binding/subscription_binding.go` (reuso)
- `internal/onboarding/application/usecases/consume_magic_token.go` (reuso)
