# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Reutilizar o workflow `destructive-confirm` com novos `OperationKind` fechados
- **Data:** 2026-07-02
- **Status:** Aceita
- **Decisores:** Autor da techspec, time de plataforma
- **Relacionados:** PRD `.specs/prd-mecontrola-agent-tools/prd.md` (RF-16, RF-17, RF-18, RF-22, RF-23);
  techspec `.specs/prd-mecontrola-agent-tools/techspec.md`; R-AGENT-WF-001 (.7 / addendum .7-A)

## Contexto

O PRD adiciona 3 operações destrutivas/sensíveis (`update_recurrence`, `delete_recurrence`,
`update_card` quando altera dia de vencimento). Já existe um workflow único de confirmação HITL
`destructive-confirm` (`internal/agents/application/workflows/destructive_confirm_workflow.go`) com
estado fechado `ConfirmState`, enum `OperationKind` (`OpDeleteEntry`/`OpEditEntry`/`OpDeleteCard`),
semântica estrita (sim/não), reprompt único, TTL de 5 min, persistência antes da confirmação e
resume por merge-patch — tudo já testado. O dispatch atual em `executeOperation` usa `switch`.

## Decisão

Reutilizar o workflow único, estendendo o enum fechado `OperationKind` com `OpUpdateRecurrence`,
`OpDeleteRecurrence` e `OpUpdateCard`, e migrando o dispatch de `switch` para
`map[OperationKind]func(ctx, ConfirmState, deps) error`. As novas tools destrutivas seguem o idioma de
`edit_entry`/`delete_entry`: montam `ConfirmState` (com `UpdatePayload` serializado quando aplicável),
chamam `engine.Start(confirmDef, key, state)` e retornam `needsConfirmation=true` sem efetivar. A
efetivação ocorre só no resume, após confirmação explícita.

Escopo: apenas `internal/agents`. O kernel `internal/platform/workflow` permanece intocado.

## Alternativas Consideradas

- **Um workflow dedicado por operação.** Vantagem: isolamento. Desvantagem: duplica semântica de
  TTL/reprompt/limpeza já resolvida; multiplica superfície de teste; diverge do padrão vigente.
  Rejeitada por custo/risco sem benefício.
- **Executar destrutivas direto com confirmação inline no exec da tool.** Desvantagem: viola
  R-AGENT-WF-001.7 (estado de espera deve ser persistido no snapshot antes de perguntar) e o contrato
  do addendum .7-A. Rejeitada por não-conformidade.
- **Manter `switch` no dispatch.** Desvantagem: R-AGENT-WF-001.1 prefere resolução por mapa a switch
  de domínio. Rejeitada em favor do mapa.

## Consequências

### Benefícios Esperados

- Reuso de um gate HITL já endurecido (semântica estrita, TTL, limpeza determinística).
- Aderência a R-AGENT-WF-001 (estados fechados, resolução por mapa, resume antes do parse).
- Menor superfície de teste incremental (só novos executores + casos).

### Trade-offs e Custos

- Um enum e um mapa crescem; o workflow único concentra mais operações (coesão vs. tamanho).
- `UpdatePayload` como JSON serializado exige cuidado de contrato entre tool e executor.

### Riscos e Mitigações

- **Risco:** payload de update malformado. **Mitigação:** serialização/desserialização tipada
  (`RawUpdateRecurrence`/`CardUpdate`), teste unitário de round-trip.
- **Rollback:** as operações novas são aditivas; remover as constantes/entradas do mapa reverte sem
  afetar as 3 operações existentes.

## Plano de Implementação

1. Estender `OperationKind` + `String()`/`IsValid()`/`ParseOperationKind()` em `confirm_state.go`.
2. Migrar `executeOperation` para mapa; adicionar `executeUpdateRecurrence`, `executeDeleteRecurrence`,
   `executeUpdateCard`; estender `successMessage` e `BuildImpactNote` (`TargetKind` `recurring_template`).
3. Criar as 3 tools destrutivas seguindo `edit_entry`/`delete_entry`.
4. Registrar em `buildFinancialTools`.

## Monitoramento e Validação

- Testes unitários e de integração do gate para as 3 operações (confirm/cancel/ambíguo/TTL).
- Métrica `agents_write_total{operation}` para as novas operações; nenhum run deve ficar suspenso órfão.
- Critério de sucesso: RF-22/RF-23 verdes; 0 efetivação sem confirmação (M-06 = 0).

## Impacto em Documentação e Operação

- Atualizar runbook do agente com as novas operações destrutivas e suas mensagens de confirmação.

## Revisão Futura

- Revisar se o número de `OperationKind` no workflow único ultrapassar ~8 (sinal para segmentar).
