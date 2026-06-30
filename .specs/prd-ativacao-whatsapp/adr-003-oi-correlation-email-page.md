# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Correlação por mensagem de ativação + telefone, e-mail/página via `/ativar`, sem Telegram
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Plataforma / autor da feature
- **Relacionados:** PRD `.specs/prd-ativacao-whatsapp/prd.md` (RF-07, RF-13..RF-22, RF-30, RF-31), techspec, ADR-001, ADR-002

## Contexto

A UX atual exige código visível: a mensagem do WhatsApp é `ATIVAR <token>` (`activation_command.go`) e o e-mail aponta **direto** para `wa.me?text=ATIVAR <token>` (`send_activation_email.go:107`), pulando a página `/ativar`. O `Customer.mobile` da Kiwify não é normalizado (`kiwifypayload/commands.go:18`), enquanto o `msg.From` da Meta vira `+E164` garantido (`parser.go:24`, `whatsapp_number.go`). A página `/ativar` já consome `wa_me_url` as-is do backend e oferece um botão Telegram condicional (que o backend nunca popula).

O PRD exige mensagem **"Ativar o meu plano"** sem código, correlação invisível por telefone, e-mail levando à página `/ativar`, e Telegram fora.

## Decisão

1. **Mensagem "Ativar o meu plano" + correlação por telefone**: `get_token_state` constrói `wa_me_url = wa.me/<bot>?text=Ativar+o+meu+plano` quando o token tem telefone esperado. A correlação ocorre por número (ADR-001/002, query por `customer_mobile_e164`).
2. **Normalização única E.164**: novo pacote `internal/platform/phone` como fonte de verdade; `whatsapp_number.go` (identity) delega a ele e `kiwifypayload/commands.go` normaliza `Customer.Mobile` antes de persistir no token (RF-07/RF-21). Telefone não-normalizável → string vazia → caso de borda.
3. **E-mail → página `/ativar`**: o CTA passa a ser `${ONBOARDING_ACTIVATION_PAGE_URL}/ativar?token=<clear>` (não mais `wa.me` direto), preservando validação de estado (expirado/pendente/consumido) antes de abrir o WhatsApp.
4. **Caso de borda sem telefone (RF-30/RF-31)**: quando o token não tem telefone esperado, `wa_me_url = wa.me/<bot>?text=<token-cru>`; o consumer, ao não achar match por telefone, tenta interpretar o texto como token válido (`TokenFromClear`) e consome por token (`ActivationPathDirect`). No fluxo normal a mensagem é "Ativar o meu plano".
5. **Remoção do Telegram**: backend nunca retorna `telegram_deep_link`; a landing remove o botão `#activate-tg-btn` e a lógica associada; teste Playwright correspondente é removido.
6. **Remoção do legado `ATIVAR` com cutover transicional**: o dispatcher deixa de casar `ATIVAR <token>` e `WA_MSG_PLEASE_USE_ATIVAR_COMMAND` deixa de ser usado. Para não quebrar e-mails/links em voo (tokens PAID já enviados com `wa.me?text=ATIVAR <token>`), o `ActivationAttemptConsumer` aceita, de forma transicional, texto que contenha um token válido **com ou sem** o prefixo `ATIVAR` e consome por token (`ActivationPathDirect`). O parse de token é removido após a base antiga expirar. `ConsumeMagicToken` permanece como porta interna.
7. **ActivationPath**: match por telefone usa `ActivationPathFallbackE164` (semântica "por número, sem token"); caminho por token usa `ActivationPathDirect`. Sem alteração da CHECK constraint `('direct','fallback_e164','outreach','admin')`.

## Alternativas Consideradas

- **Token oculto no `wa.me` (zero-width/sufixo)**: − insere dado técnico na mensagem, frágil entre clientes WhatsApp. Rejeitada (decisão do usuário por mensagem simples sem código).
- **Manter `ATIVAR <token>`**: − viola "sem código visível". Rejeitada.
- **E-mail direto para `wa.me` com mensagem pré-preenchida**: − perde validação de estado e contraria o fluxo do PRD (e-mail→/activate→WhatsApp). Rejeitada.
- **Exportar `normalizeRaw` da identity em vez de novo pacote**: − cria dependência `billing→identity/domain` (cross-module). Rejeitada em favor de pacote de plataforma neutro.

## Consequências

### Benefícios Esperados

- UX premium sem código visível; e-mail com validação de estado antes do WhatsApp.
- Fonte única de normalização elimina divergência Kiwify×Meta.
- Landing simplificada (sem Telegram morto).

### Trade-offs e Custos

- Caso de borda ainda carrega o token na mensagem (aceito por RF-31, apenas quando não há telefone).
- Nova config de base-URL da página por ambiente.

### Riscos e Mitigações

- **Telefone Kiwify inválido**: vira string vazia → borda por token, nunca ativação errada.
- **Deriva entre normalizadores**: evitada pela delegação de `whatsapp_number.go` ao pacote único (testes de tabela cobrem os formatos).
- Rollback: reverter as URLs e reintroduzir o ramo `ATIVAR` (mantido em git) restaura o comportamento anterior; índice/colunas são aditivos.

## Plano de Implementação

1. `internal/platform/phone` + refactor `whatsapp_number.go` + normalização em `commands.go`.
2. `get_token_state`: `Ativar+o+meu+plano`/token-cru conforme telefone; remover `ATIVAR`.
3. `send_activation_email`: URL para `/ativar` + config nova; `email_sent_at`.
4. Remover Telegram na landing + ajustar Playwright.
5. Conclusão: e2e backend + Playwright verdes; e-mail abre `/ativar`; mensagem é "Ativar o meu plano".

## Monitoramento e Validação

- `onboarding_activation_attempt_total{outcome=phone_matched|token_matched|no_match}`.
- Telefone sempre mascarado em logs (`payload.MaskMobile`).
- Sucesso: 100% das ativações felizes via telefone; borda por token só quando telefone ausente.

## Impacto em Documentação e Operação

- `.env.example`/runbook: `ONBOARDING_ACTIVATION_PAGE_URL`, `WA_MSG_ACTIVATION_NOT_FOUND`, depreciação de `WA_MSG_PLEASE_USE_ATIVAR_COMMAND`.
- Landing: README de deploy (sem Telegram).

## Revisão Futura

Revisitar se o checkout passar a capturar telefone (eliminaria a borda por token) ou se um segundo canal voltar ao escopo.
