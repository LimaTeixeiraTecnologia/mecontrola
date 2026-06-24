# Registro de Decisão Arquitetural — ADR-001

## Metadados

- **Título:** Expor `reason` na resposta JSON do endpoint de estado do token
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** `techspec.md`, `prd.md` RF-08, RF-11

## Contexto

O endpoint `GET /api/v1/onboarding/tokens/{token}/state` já calcula internamente o motivo pelo
qual um token não está pronto (`TokenStateReason`: `not_found`, `pending`, `expired`, `consumed`).
Essa informação é usada em `h.invalidAccess(reason)` para métricas, mas **não é retornada** ao
cliente. O frontend (`activate.js`) recebe apenas `{ ready_to_activate: false }` e exibe a
mensagem genérica "Não foi possível validar seu acesso" para todos os estados negativos.

O resultado é que um usuário com pagamento em processamento vê a mesma mensagem de erro que um
usuário com token inválido — experiência confusa e que gera suporte desnecessário.

## Decisão

Incluir o campo `reason` (string) na resposta JSON para estados `ready_to_activate: false`.
Para o estado `consumed`, incluir também `wa_me_url` e `bot_number_display`, permitindo que
a página bridge ofereça acesso ao bot mesmo quando o token já foi consumido.

Valores possíveis do campo `reason`: `not_found`, `pending`, `expired`, `consumed`.

## Alternativas Consideradas

### 1. Manter resposta opaca (sem `reason`) e tratar no servidor
- Vantagem: frontend não sabe o estado exato do token — mais opaco.
- Desvantagem: requer lógica server-side (redirect, render de página diferente) que introduz
  complexidade no backend sem benefício proporcional. A página bridge é estática (Cloudflare Pages)
  e não pode renderizar estado dinâmico sem JS.
- **Rejeitada**: complexidade desproporcional para uma landing page estática.

### 2. Usar HTTP status codes distintos (404, 409, 410, 402)
- Vantagem: semântica HTTP clara.
- Desvantagem: a decisão anterior de usar `200 + ready_to_activate: false` com jitter de timing
  já foi tomada para mitigar timing attacks. Mudar para status codes distintos quebraria essa
  proteção — um atacante poderia distinguir tokens existentes de inexistentes pela latência.
- **Rejeitada**: regride a proteção de timing já implementada.

### 3. Expor `reason` apenas em header HTTP
- Vantagem: corpo da resposta permanece uniforme.
- Desvantagem: headers são menos ergonômicos para parsing no JS do frontend; CORS headers extras.
- **Rejeitada**: sem vantagem real; `reason` no body JSON é mais simples.

## Consequências

### Benefícios Esperados

- Frontend pode exibir mensagens contextuais específicas por estado.
- Usuário com pagamento pendente sabe que deve aguardar, não que o link está errado.
- Usuário com conta já ativa recebe confirmação positiva + acesso direto ao bot.
- Redução de tickets de suporte para estados claramente identificáveis.

### Trade-offs e Custos

- O campo `reason` informa ao cliente o estado exato do token. Isso é um leve aumento de
  informação exposta, mas não representa regressão de segurança:
  - O jitter de timing continua ativo para estados não-ready, preservando a proteção contra
    enumeração de tokens por latência.
  - O `reason` não ajuda um atacante a descobrir tokens válidos — apenas descreve por que um
    token específico não está pronto.
- Mudança backward-compatible: `reason` é `omitempty`; clientes que não leem o campo continuam
  funcionando.

### Riscos e Mitigações

- **Risco**: cliente malicioso usa `reason` para distinguir tokens existentes de inexistentes.
  **Mitigação**: o jitter permanece, e `not_found` é retornado também para tokens com hash inválido
  (sem lookup ao banco) — o atacante não consegue distinguir "token nunca existiu" de "token
  com hash inválido".

## Plano de Implementação

1. Adicionar `Reason string` a `GetTokenStateOutput` (DTO).
2. Preencher `Reason` em `get_token_state.go` para todos os estados não-ready.
3. Para `consumed`: preencher `WaMeURL` e `BotNumberDisplay` no output.
4. Atualizar `tokenStateResponse` no handler com campo `Reason` (`json:"reason,omitempty"`).
5. Serializar `Reason`, `WaMeURL`, `BotNumberDisplay` no JSON de resposta para não-ready.
6. Adicionar cenários de teste para consumed e por reason no handler e usecase.

## Monitoramento e Validação

- `h.invalidAccess(reason)` já existente continua funcionando sem mudança.
- Critério de sucesso: testes `token_state_handler_test.go` cobrem todos os 4 reasons com
  verificação de campo `reason` no JSON.

## Revisão Futura

Revisar se `reason` deve ser tipado (enum) no contrato OpenAPI quando uma especificação formal
for adotada. Data sugerida: próxima revisão de contratos de API do módulo `onboarding`.
