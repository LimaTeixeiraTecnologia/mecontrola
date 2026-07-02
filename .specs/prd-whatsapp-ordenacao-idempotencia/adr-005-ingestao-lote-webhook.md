# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Ingestão em lote do webhook WhatsApp — 1 evento outbox por mensagem
- **Data:** 2026-07-02
- **Status:** Aceita
- **Decisores:** time de plataforma (autor), owner do produto (decisões D-07/D-13 do PRD)
- **Relacionados:** PRD `.specs/prd-whatsapp-ordenacao-idempotencia/prd.md` (RF-17/18), techspec `techspec.md`,
  ADR-001 (claim particionado), ADR-002 (idempotência), regras R-ADAPTER-001, R-WF-KERNEL-001

## Contexto

O parser do webhook (`internal/platform/whatsapp/payload/parser.go` `ExtractFirstMessage`) processa
**apenas** `c.Value.Messages[0]` e descarta as demais mensagens do mesmo webhook (verificado no código
em 2026-07-02). A Meta pode entregar múltiplas mensagens num único POST; hoje todas exceto a primeira
são perdidas silenciosamente. Além disso, o `OccurredAt` do evento outbox usa `time.Now().UTC()`
(`dispatcher.go`), não o timestamp da mensagem da Meta — o que impede o FIFO por usuário de refletir a
ordem real de envio quando duas mensagens caem no mesmo lote ou em servers diferentes com skew.

## Decisão

1. **Extrair todas as mensagens do webhook** (não só `Messages[0]`): iterar `Entry[].Changes[].Value.Messages[]`.
2. **Emitir 1 evento outbox por mensagem inbound**, cada um com:
   - seu próprio `wamid` (id da mensagem da Meta) como `aggregate_id`;
   - `aggregate_user_id` do usuário resolvido (para o claim particionado, ADR-001);
   - `OccurredAt = msg.Timestamp` da Meta (epoch → `time.Time`), critério primário do FIFO (RF-18);
   - `created_at` do outbox como desempate dentro do mesmo segundo (D-08).
3. **`item_seq`** permanece como índice de escrita **dentro do turno de uma mensagem** (uma mensagem
   pode gerar N escritas de domínio), compondo a chave de idempotência `(wamid, item_seq, operation)`
   do `agents_write_ledger` e a chave natural de domínio (ADR-002 emenda v3).
4. A ordem entre as N mensagens do mesmo usuário é garantida pelo **claim particionado** (1 evento em
   voo por usuário, `ORDER BY occurred_at`), não por processá-las no mesmo Run.

O handler/produtor permanece **adapter fino** (R-ADAPTER-001): apenas parseia, resolve principal e
publica N eventos; sem regra de negócio nem branching de domínio.

## Alternativas Consideradas

1. **1 evento outbox carregando o array de mensagens (Run único itera):** menos linhas no outbox, mas
   mistura múltiplos `wamid` num evento, complica replay/idempotência (a chave é por `wamid`) e o FIFO
   por mensagem. Rejeitada — quebra o modelo `(wamid, item_seq, operation)`.
2. **Manter só a primeira mensagem (status quo):** perda silenciosa de dados do usuário. Rejeitada.
3. **Deduplicar/reordenar no consumer:** empurra ordenação para depois do claim, reintroduzindo o
   problema que o claim particionado resolve. Rejeitada.

## Consequências

### Benefícios Esperados

- Nenhuma mensagem do usuário é perdida; conversa reflete tudo que foi enviado.
- FIFO por usuário correto por `occurred_at` (timestamp da Meta), robusto a skew entre server-1/2.
- Chave de idempotência limpa (1 `wamid` por evento), alinhada ao `agents_write_ledger` existente.

### Trade-offs e Custos

- Mais linhas no outbox por webhook multi-mensagem (volume baixo; turnos humanos).
- Conversão epoch→`time.Time` precisa tratar `msg.Timestamp` ausente/zero (fallback para `created_at`).

### Riscos e Mitigações

- **Risco:** `msg.Timestamp` nulo/inválido quebra o FIFO. **Mitigação:** validar no parser; ausência →
  usar `now()` como `occurred_at` e registrar métrica (não falhar a ingestão).
- **Risco:** duas mensagens com o mesmo `occurred_at` (mesmo segundo). **Mitigação:** desempate por
  `created_at` do outbox (D-08).

## Plano de Implementação

1. `payload/parser.go`: substituir `ExtractFirstMessage` por extração de todas as mensagens (nova
   função retornando `[]Message`), preservando `msg.Timestamp` (string epoch).
2. `module.go` `buildWhatsAppAgentRoute` / `dispatcher.go`: publicar 1 evento por mensagem com
   `OccurredAt = msg.Timestamp` convertido; fallback para `now()` se ausente.
3. Testes unitários: webhook com N mensagens → N eventos, ordem por `occurred_at`; timestamp ausente →
   fallback; mensagem única mantém comportamento.
4. Integração (CA-07): webhook multi-mensagem processa todas na ordem do timestamp da Meta sob claim
   particionado.

Concluído quando: CA-07 verde; nenhuma mensagem descartada; FIFO por `occurred_at` observado.

## Monitoramento e Validação

- Métricas: mensagens por webhook (distribuição), `occurred_at` ausente (fallback), ordem observada.

## Impacto em Documentação e Operação

- Runbook de ingestão do WhatsApp (comportamento multi-mensagem), dashboards de ordenação.

## Revisão Futura

- Revisar se a Meta introduzir agrupamento/ordem própria de mensagens, ou se o volume multi-mensagem
  justificar batch de publicação transacional.
