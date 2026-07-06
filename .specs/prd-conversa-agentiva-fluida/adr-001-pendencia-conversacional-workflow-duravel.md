# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Pendência conversacional como workflow durável em `internal/agents`
- **Data:** 2026-07-06
- **Status:** Aceita
- **Decisores:** Engenharia / Produto MeControla
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O agente perde contexto quando uma tool de registro retorna necessidade de clarificação. O usuário responde com texto curto, mas o agente trata o turno como nova intenção ou mistura com uma pendência anterior. O repositório já possui `internal/platform/workflow` com snapshot durável, resume por merge-patch, CAS e reaper.

## Decisão

Criar um workflow durável `pending-entry` no consumidor `internal/agents`, com estado próprio `PendingEntryState`, key `<resourceID>:<threadID>:pending-entry`, TTL funcional de 30 minutos e resolução antes do agente aberto no consumer de WhatsApp. O kernel de workflow permanece genérico e sem semântica financeira.

Emenda `spec-version 3`: toda escrita financeira originada da conversa (registro, edição, recorrência) passa por este workflow, inclusive lançamentos totalmente especificados e sem ambiguidade, que abrem diretamente no estado terminal `AwaitingSlotConfirmation`. Nenhuma tool escreve de forma síncrona; a persistência só ocorre no resume que carrega o aceite humano explícito (ver ADR-004).

## Alternativas Consideradas

- Prompt-only: menor custo inicial, mas não preserva estado durável nem garante 0 falso positivo.
- Tabela própria de pendências: controle total, mas duplica capacidades já existentes no kernel.
- Expandir `destructive-confirm`: reaproveita wiring, mas mistura confirmação destrutiva com coleta de slots financeiros.

## Consequências

### Benefícios Esperados

- Retomada auditável e determinística.
- Sem reimplementar primitivos de plataforma.
- Conflitos tratados por CAS do workflow store.
- Isolamento correto por thread/canal, evitando aplicar resposta curta em conversa errada.

### Trade-offs e Custos

- Mais tipos e testes no consumidor `internal/agents`.
- Requer wiring adicional no módulo e consumer.

### Riscos e Mitigações

- Risco de regressão na ordem de resolvers. Mitigação: teste unitário do consumer validando ordem pending -> confirm -> onboarding -> agent.
- Risco de pendência presa. Mitigação: reaper e TTL de 30 minutos.
- Risco de key com dado semântico em métrica. Mitigação: key só em storage/log diagnóstico controlado; métricas usam apenas labels enum.

## Plano de Implementação

1. Criar tipos fechados e `PendingEntryState`.
2. Criar workflow e continuer.
3. Integrar no consumer antes dos resolvers existentes.
4. Adicionar reaper no módulo.
5. Cobrir com harness e testes de integração.

## Monitoramento e Validação

Monitorar `agents_pending_entry_total{outcome}` e `workflow_*{workflow="pending-entry"}`. A decisão é válida quando os cenários canônicos do PRD passam com 100% de retomada correta e 0 escrita indevida.

## Impacto em Documentação e Operação

Atualizar techspec, tasks e runbook operacional de conversas pendentes após implementação.

## Revisão Futura

Revisar se houver múltiplas pendências por thread como requisito de produto.
