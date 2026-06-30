# Tarefa 8.0: E-mail para `/ativar`, estado do token e timestamps server-side

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Redirecionar o CTA do e-mail para a página `/ativar`, suprimir e-mail de recompra já vinculada, e ajustar o endpoint de estado do token para entregar a mensagem "Oi" (ou token-cru na borda) sem Telegram. Registrar timestamps server-side.

<requirements>
- RF-06/RF-12/RF-13/RF-14: e-mail com CTA único para `${ONBOARDING_ACTIVATION_PAGE_URL}/ativar?token=<clear>`.
- RF-11/RF-16: token não exposto; página valida e mostra apenas "Abrir WhatsApp".
- RF-17/RF-19: sem Telegram no backend (nunca retornar `telegram_deep_link`).
- RF-29: `get_token_state` constrói `wa_me_url` com "Oi" (telefone presente) ou token-cru (borda sem telefone); sem `ATIVAR`.
- RF-35: `email_sent_at` e `activation_started_at` (set-once-if-null) no servidor.
- Supressão de e-mail quando a assinatura já está vinculada a um usuário (recompra).
</requirements>

## Subtarefas

- [ ] 8.1 `send_activation_email.go`: trocar a construção de URL (`wa.me?text=ATIVAR...`) por `${ActivationPageURL}/ativar?token=<escaped>`; manter support URL; injetar a base via config (3.0).
- [ ] 8.2 Suprimir envio quando a assinatura/n.º já tem usuário vinculado (consultar binding); manter o skip atual por token CONSUMED/EXPIRED.
- [ ] 8.3 `get_token_state.go`: `wa_me_url` com "Oi" quando `CustomerMobileE164` presente; com token-cru quando ausente; remover `ATIVAR`. Garantir ausência de `telegram_deep_link`.
- [ ] 8.4 Persistir `email_sent_at` (em envio com sucesso) e `activation_started_at` (set-once-if-null) via métodos de repo idempotentes.

## Detalhes de Implementação

Ver techspec.md, "Endpoints de API", "Fluxo de Dados" e ADR-003. `GET /state` permanece sem escrita (page_opened fica no beacon, tarefa 9.0).

## Critérios de Sucesso

- E-mail aponta para `/ativar?token=`; recompra vinculada não recebe e-mail.
- `wa_me_url` = "Oi" (telefone presente) ou token-cru (borda); sem `ATIVAR`; sem campo Telegram.
- `email_sent_at` gravado no envio; idempotência preservada.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unitários (testify/suite) de `send_activation_email` (URL, supressão, idempotência) e `get_token_state` (Oi/token/sem Telegram).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/application/usecases/send_activation_email.go`
- `internal/onboarding/infrastructure/messaging/database/consumers/activation_email_consumer.go`
- `internal/onboarding/application/usecases/get_token_state.go`
- `internal/onboarding/application/dtos/output/get_token_state_output.go`
