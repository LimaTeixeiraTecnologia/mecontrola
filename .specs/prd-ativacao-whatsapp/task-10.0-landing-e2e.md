# Tarefa 10.0: Landing (sem Telegram + beacon) e E2E ponta a ponta

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ajustar a landing page (`mecontrola-landingpage`) removendo o Telegram e chamando o beacon, e validar a jornada inteira ponta a ponta no backend, incluindo as etapas existentes do webhook que devem permanecer corretas.

<requirements>
- RF-15/RF-16/RF-17/RF-19: página `/ativar` amigável, só "Abrir WhatsApp", sem Telegram.
- RF-35: chamada do beacon ao exibir o CTA e ao acionar "Abrir WhatsApp".
- RF-01/RF-02/RF-03/RF-04/RF-05: validar (e2e) que o webhook valida assinatura/pagamento, localiza/cria cliente, marca PAID com `paidAt`, NÃO ativa no webhook e é idempotente.
- RF-34: jornada termina na boas-vindas.
</requirements>

## Subtarefas

- [ ] 10.1 (Landing) Remover o bloco do botão Telegram em `src/pages/ativar.astro:78-93` e a lógica em `public/js/activate.js:25-26,175-184`.
- [ ] 10.2 (Landing) Chamar `POST /tokens/{token}/opened` ao renderizar o CTA (`page_opened`) e ao clicar em "Abrir WhatsApp" (`whatsapp_opened`).
- [ ] 10.3 (Landing) Remover/ajustar o teste Playwright de Telegram (`tests/playwright/activate.spec.ts:39-61`); manter os demais.
- [ ] 10.4 (Backend) E2E da jornada: paga → PAID → `/state` retorna "Oi" → inbound não-vinculado → ativado → `subscription_bound` → 2 boas-vindas; e idempotência de reentrega.
- [ ] 10.5 (Backend) E2E de regressão do webhook (RF-01..05): assinatura inválida rejeitada, PAID sem ativar conta, reentrega idempotente.

## Detalhes de Implementação

Ver techspec.md, "Abordagem de Testes" (E2E) e "Pontos de Integração". A landing consome `wa_me_url` as-is; o backend já entrega "Oi". Deploy: backend primeiro (com beacon), landing depois.

## Critérios de Sucesso

- Landing sem Telegram; beacon chamado nos dois momentos; Playwright verde.
- E2E backend cobre a jornada feliz + idempotência + gate PAID-sem-ativar.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Playwright (landing) atualizado e verde.
- [ ] E2E backend da jornada completa + regressão do webhook.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `mecontrola-landingpage/src/pages/ativar.astro`
- `mecontrola-landingpage/public/js/activate.js`
- `mecontrola-landingpage/tests/playwright/activate.spec.ts`
- `internal/onboarding/e2e/` (e2e backend)
