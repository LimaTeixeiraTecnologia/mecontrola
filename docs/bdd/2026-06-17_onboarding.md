# BDD: Módulo Onboarding
**Data:** 2026-06-17
**Status:** MVP Robust / Production-Ready
**Referência:** Domain Modeling Made Functional (DMMF)

## Objetivo
Orquestrar a entrada de novos usuários, desde a recepção do token de pagamento até a configuração inicial do Telegram e do orçamento.

## Fluxo 1: Ciclo de Vida do Magic Token
**Funcionalidade:** Acesso seguro via link único pós-pagamento.

**Cenário:** Consumo de Magic Token válido
- **Dado** que um usuário possui um `MagicToken` com status `Paid`
- **Quando** o usuário acessa o link e consome o token
- **Então** o token deve ser marcado como `Consumed`
- **E** uma sessão de onboarding deve ser iniciada para o usuário.

## Fluxo 2: Ativação via Telegram
**Funcionalidade:** Vinculação de canal de comunicação.

**Cenário:** Ativação de bot via token de ativação
- **Dado** que o usuário recebeu um código de ativação no Telegram
- **Quando** o sistema valida o código enviado pelo usuário no bot
- **Então** o ID do Telegram deve ser vinculado permanentemente à conta do usuário
- **E** o status de onboarding deve progredir para `TelegramLinked`.

## Fluxo 3: Configuração Inicial de Orçamento
**Funcionalidade:** Setup rápido para novos usuários.

**Cenário:** Usuário define primeiros limites no onboarding
- **Dado** que o usuário está na etapa de configuração de orçamento
- **Quando** ele informa seus limites básicos
- **Então** o módulo `Budgets` deve ser acionado para criar os registros iniciais
- **E** o onboarding deve ser marcado como `Complete`.

## Regras de Domínio (DMMF)
- **State-as-Type:** O `MagicToken` transiciona entre `Pending`, `Paid`, `Consumed` e `Expired`.
- **Workflows:** O processo de ativação é uma pipeline que valida o token, verifica expiração e vincula o canal.
- **Invariantes:** Um token consumido não pode ser reutilizado.

## Validação de Produção
- [ ] Garantir expiração automática de tokens não pagos ou não consumidos após X horas.
- [ ] Validar a segurança da geração de tokens (entropia suficiente).
- [ ] Verificar se mensagens de outreach são enviadas corretamente em caso de abandono.
