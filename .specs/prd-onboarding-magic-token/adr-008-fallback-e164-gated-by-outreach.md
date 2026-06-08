# ADR-008 — Fallback de ativação por match E.164 gated por outreach já enviado

## Metadados

- **Título:** Pré-condição obrigatória "outreach já enviado" para ativação por match de número
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §5.2, RF-10, S-09; discovery `docs/discoveries/discovery-onboarding-flow.md`

## Contexto

O PRD (RF-10, S-09) **diverge deliberadamente** da discovery primária (`discovery-onboarding-flow.md` §5 `tryFallbackActivation`), que descreve fallback E.164 como ativação automática a partir de qualquer mensagem do número casado, **sem pré-condição**.

O risco da versão sem gating: cliente A digita engano o número de WhatsApp do cliente B no checkout. B nunca recebeu mensagem alguma, mas envia uma mensagem espontânea ao bot. Sistema casa E.164 e ativa a conta de A para B. Resultado: B recebe acesso indevido a uma assinatura paga por outra pessoa.

O épico não fixa essa pré-condição, então não há violação do roadmap; é refinamento de produto.

## Decisão

A ativação por match E.164 (`TryFallbackActivation`) **só** pode ocorrer quando:
1. Existe token em estado `PAID` cujo `customer_mobile_e164` casa com o `from` da mensagem inbound.
2. `outreach_sent_at IS NOT NULL` no mesmo token (outreach já foi disparado pelo job de E3).

Quando outreach ainda não foi enviado:
- `TryFallbackActivation` retorna sem efeito.
- Bot responde mensagem `please_use_ativar_command` orientando o cliente a usar `ATIVAR <token>`.

Quando outreach já foi enviado:
- Executa mesma transação atômica de `ConsumeMagicToken` com `activation_path='fallback_e164'`.

A pré-condição faz com que a ativação automática só ocorra **após contato intencional do bot** com aquele número. Como o outreach só dispara depois de `paid_at + 2h` (RF-09), há uma janela mínima de 2 horas em que apenas `ATIVAR` funciona — o que reforça que o token deve ser usado intencionalmente.

## Alternativas Consideradas

1. **Sem gating (discovery original).** Recusada — risco de ativação de terceiro descrito acima.
2. **Gating por janela temporal (ex: `paid_at + 24h`) em vez de outreach.** Recusada — janela arbitrária; não comprova contato com o cliente; outreach é evidência empírica de comunicação.
3. **Gating por outreach OU 7d (TTL do token).** Recusada — após 7d o token já expirou; redundante.
4. **Exigir confirmação interativa do bot ("é você?").** Recusada — adiciona fricção; PRD explicita "máximo 1 mensagem para ativar"; outreach + gating já entrega segurança suficiente.

## Consequências

### Benefícios
- Reduz risco de ativação não solicitada.
- Outreach passa a ter dupla função: lembrar o cliente E habilitar fallback.
- Auditável: presença de `outreach_sent_at` é prova de pré-condição.

### Trade-offs
- Janela inicial (0–2h pós-pagamento) sem fallback. Cliente que enviar mensagem nesse intervalo recebe orientação textual, não ativação automática. Aceitável dado o caminho feliz (`ATIVAR` via deep link).
- Se a Meta atrasar muito o template (S-04 — outreach desligado), fallback E.164 nunca funciona. Aceitável: caminho feliz permanece operacional via `ATIVAR`.

### Riscos e Mitigações
- **R:** Cliente legítimo envia mensagem qualquer ao bot dentro da janela 0–2h e recebe orientação para usar `ATIVAR`. **M:** Mensagem clara `please_use_ativar_command` inclui o número do token (ou orienta a abrir o link `wa.me` da página de agradecimento).
- **R:** Outreach desligado em produção (toggle) → fallback permanentemente inativo. **M:** Métrica `onboarding_tokens_consumed_total{activation_path="fallback_e164"}` em zero alerta operação.

## Plano de Implementação
1. Use case `TryFallbackActivation` consulta `WHERE status='PAID' AND customer_mobile_e164=? AND outreach_sent_at IS NOT NULL`.
2. Test unitário com 4 casos: (a) sem outreach → no-op com mensagem orientativa; (b) com outreach → ativação; (c) número diverge → não dispara; (d) token EXPIRED → não dispara.

## Monitoramento
- `onboarding_tokens_consumed_total{activation_path}` segmenta direto vs fallback vs outreach.
- Comparar `outreach_sent_total` vs `fallback_e164` consumption para entender conversão.
