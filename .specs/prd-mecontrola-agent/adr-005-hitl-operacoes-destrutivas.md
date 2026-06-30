# Registro de Decisão Arquitetural (ADR-005)

## Metadados

- **Título:** HITL de operações destrutivas — reemissão do contrato `R-AGENT-WF-001.7-A`
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-27, D-03/D-21), techspec.md; `.claude/rules/agent-workflows-tools.md` (Addendum R-AGENT-WF-001.7-A, marcado SUPERSEDED como caminho literal até reemissão por consumidor que reintroduza HITL)

## Contexto

O domínio não oferece proteção nem confirmação para operações destrutivas: deletes são imediatos; remover cartão com parcelas em aberto deixa órfãos (`card/.../soft_delete_card.go`); `delete_card_purchase` remove todas as parcelas (`transactions/.../delete_card_purchase.go:76-104`). O PRD exige confirmação humana explícita com aviso de impacto (RF-27/D-21). A regra `R-AGENT-WF-001.7-A` define o contrato HITL (estado de espera fechado, persistido antes de pedir confirmação, retomado por merge-patch antes do parse, limpeza determinística), porém está **SUPERSEDED como caminho literal** após a remoção do agente financeiro anterior — devendo ser **reemitida pelo consumidor que reintroduzir HITL**. Este é esse consumidor.

## Decisão

Reemitir o contrato HITL para `internal/agents`. Modelar `AwaitingKind` (`AwaitingNone`/`AwaitingConfirm`) e `OperationKind` (`OpDeleteEntry`/`OpEditEntry`/`OpDeleteCard`) como **tipos fechados** (state-as-type), com `ConfirmState` persistido no **`Snapshot` do kernel** (fonte única, sem side-store de domínio). Fluxo para operação destrutiva: a tool **resolve o alvo** (mês corrente por padrão, ampliando se citado — D-03), monta `ImpactNote` (parcelado → "remove N parcelas em todos os meses"; cartão → checa `HasOpenInstallments` e alerta órfãos), **persiste `ConfirmState{Awaiting:AwaitingConfirm}` antes de retornar a pergunta**. No turno seguinte, `continueDestructiveConfirm` roda **antes de qualquer parse**, aplicando merge-patch (`{"ResumeText":"sim"}`): confirmação explícita executa via binding e fecha o run; cancelamento descarta; resposta ambígua re-pergunta **uma vez** (`RepromptDone`), depois cancela; replay de `messageID` já processado → `ToolOutcomeReplay`. Limpeza determinística: o run nunca permanece suspenso após efetivar/cancelar. LLM **proibido** no passo de confirmação (R-AGENT-WF-001.4).

## Alternativas Consideradas

- **Deletes diretos sem confirmação** — viola RF-27; perigoso e irreversível na conversa. Rejeitada.
- **Confirmar via flag booleana/string solta** — viola DMMF state-as-type e o contrato HITL. Rejeitada.
- **Proteção no domínio (`internal/card` bloquear delete com parcelas)** — correto a longo prazo, mas fora do escopo deste PRD (mudaria contrato de outro módulo). Registrado como risco; mitigado no agente.

## Consequências

### Benefícios Esperados

- Segurança contra remoção indevida e órfãos; aderência ao contrato de governança HITL.

### Trade-offs e Custos

- Um ciclo extra de suspend/resume por operação destrutiva.

### Riscos e Mitigações

- **Órfão de cartão persiste no domínio** → aviso explícito ao usuário; não efetiva sem "sim". Sinalizar correção futura em `internal/card`.
- **Parse antes do resume** (regressão) → ordem determinística `continueDestructiveConfirm → parse`; teste de guarda.

## Plano de Implementação

1. `ConfirmState`/`AwaitingKind`/`OperationKind` + persistência no Snapshot.
2. Tools destrutivas resolvem alvo + montam `ImpactNote` + suspendem.
3. `continueDestructiveConfirm` (resume antes do parse, re-prompt único, TTL/cancelamento).
4. Testes: confirma/cancela/ambíguo/replay/limpeza.

## Monitoramento e Validação

- `agents_destructive_confirm_total{operation,result=confirmed|cancelled|ambiguous}`; nenhum run destrutivo permanece suspenso.

## Impacto em Documentação e Operação

- Atualizar `.claude/rules/agent-workflows-tools.md` reemitindo os gates literais do Addendum .7-A apontando para os arquivos reais deste consumidor; runbook com strings de confirmação verbatim.

## Revisão Futura

- Revisar quando/se `internal/card` ganhar proteção de integridade para delete com parcelas.
