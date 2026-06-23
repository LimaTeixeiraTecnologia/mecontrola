# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Síntese de WorkingMemory assíncrona via consumidor de `OnboardingCompleted`
- **Data:** 2026-06-23
- **Status:** Aceita
- **Decisores:** Dono do produto, time de plataforma
- **Relacionados:** PRD (RF-26, RF-34, DR-02, EB-11), techspec.md, ADR-002; Mastra working memory / memory processor
- **Inspiração:** https://github.com/mastra-ai/mastra — `docs/.../memory/working-memory.mdx` (escopo resource), processadores de memória

## Contexto

A WorkingMemory de perfil do usuário (markdown injetado no system prompt do agente principal) é hoje
sintetizada **inline** no `onboarding_tool_dispatcher` (`synthesizeAndStoreWM`), em best-effort
(erros engolidos), durante o turno que conclui o onboarding. Isso acopla a síntese ao turno, não tem
retry e mistura responsabilidade no adapter (tensão com R-ADAPTER-001). No modelo Mastra, working
memory é escopo `resource` (por usuário) e atualizada fora do caminho crítico da resposta.

## Decisão

Extrair a síntese para um novo consumidor `OnboardingCompletedConsumer` em `internal/agent`, que
reage ao evento `OnboardingCompleted` (já publicado na conclusão — ADR-002). O consumidor: lê o
contexto do onboarding via binding (`GetOnboardingContext`), monta o markdown por função pura e faz
`WorkingMemoryRepository.Upsert`. É idempotente (se já há WM com conteúdo, no-op) e retorna **erro**
em falha para forçar retry do outbox. A síntese inline e as dependências `wmWriter`/`contextReader`
são removidas do dispatcher.

## Alternativas Consideradas

- **Manter inline best-effort**: simples, mas sem garantia de presença, sem retry e com
  responsabilidade indevida no adapter. Rejeitada (não production-ready).
- **Bloquear conclusão até WM persistida**: garante presença, mas uma falha de WM trava o onboarding
  do usuário e acopla handoff a um passo não essencial. Rejeitada.

## Consequências

### Benefícios Esperados

- Desacopla síntese do turno; ganha retry e idempotência via outbox (RF-34/EB-11).
- Adapter fino; síntese passa a ser observável como passo próprio.
- Degradação graciosa: agente principal opera sem WM até a retentativa persistir (RF-26).

### Trade-offs e Custos

- Eventual consistency: há janela curta entre conclusão e WM disponível. Aceitável (WM é contexto, não
  gate de handoff).
- Mais um consumidor para operar/observar.

### Riscos e Mitigações

- **Risco:** loop de retry se `GetOnboardingContext` falhar persistentemente. **Mitigação:** backoff do
  outbox; métrica de falha; o handoff não depende da WM.
- **Rollback:** reabilitar a síntese inline (mantida em branch) enquanto o consumidor é corrigido.

## Plano de Implementação

1. Criar `OnboardingCompletedConsumer` (adapter fino) + função pura `buildWorkingMemory(context)`.
2. Registrar no `buildEventHandlers` para `OnboardingCompleted.EventType()`.
3. Remover `synthesizeAndStoreWM` e deps do dispatcher.

## Monitoramento e Validação

- `agent_onboarding_completed_consumer_total{result}` e `..._decode_failed_total`.
- Teste de integração: consumir `OnboardingCompleted` persiste WM; reprocesso não duplica.

## Impacto em Documentação e Operação

- Runbook: WM é eventual pós-conclusão; ausência temporária não é incidente.
- Dashboard: painel do consumidor de conclusão.

## Revisão Futura

- Revisar se a WM precisar ser sintetizada também em atualizações incrementais (pós-MVP).
