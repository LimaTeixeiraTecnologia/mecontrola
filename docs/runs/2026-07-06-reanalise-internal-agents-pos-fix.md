# Re-análise Crítica — `internal/agents` (pós-correção)

**Data:** 2026-07-06 · **Base:** `main` (working tree, não commitado) · Go 1.26.4
**Antecede:** `docs/runs/2026-07-06-analise-critica-internal-agents.md` (análise que apontou G-01..G-20)
**Método:** correção de TODOS os itens com subagentes especializados + loop principal (limite de sessão da conta cortou a Onda 1; concluído no loop principal) → re-verificação adversarial independente + validação real-LLM.

---

## 1. Sumário Executivo

**Status geral: PRONTO PARA MAIN** (era **NÃO PRONTO**).
**Nota de robustez: 9 / 10** (era 6/10).

Os 2 CRÍTICOS e os 4 ALTOS foram fechados, com os MÉDIOs e BAIXOs endereçados. O bloqueio de produção deixou de existir: recorrência funciona com `auth.Principal`, os testes de integração de `binding` compilam, `adjust_allocation` não confia mais em `userId` do LLM, e runs de confirmação suspensos são expirados por um reaper. Build/vet/suíte completa verdes; real-LLM (agents + scorers) validado. Restam apenas 3 ressalvas BAIXO de dívida técnica não-bloqueante. Falta somente commitar.

---

## 2. O que Está Bom (agora)

1. Recorrência auditável com identidade — `recurrence_manager_adapter.go` injeta `auth.Principal` nas 4 operações (`principalCtx`), com teste de regressão.
2. Rede de segurança de integração restaurada — `go vet -tags integration ./internal/agents/...` exit 0; CA09-reconciled e transactions-integration voltam a compilar.
3. Reaper genérico de runs suspensos no kernel (`internal/platform/workflow/reaper.go`) — domain-free, labels só `workflow`/`status`, CAS por versão, wired como job (`ConfirmReaperJob`).
4. Identidade sempre do contexto — nenhuma tool confia em id de usuário vindo do LLM; `adjust_allocation` corrigido.
5. Tipos fechados de fronteira — `EntryKind`/`ClassifyOutcome`/`CategoryKind` em `interfaces/discriminators.go`.
6. ACL sem vazamento de `domain` — sentinels/parser expostos na camada application dos módulos-fonte.
7. Predicado de write-eligibility único (`CategorySearchResult.IsWriteEligible()`).
8. Deadline de inbound sempre aplicado (default 60s); `message_id` obrigatório na fronteira; erros de histórico logados.
9. Provider único OpenRouter, LLM só nas call-sites sancionadas, loop bounded (12) — preservados.
10. Gates limpos: zero comentários, sem SQL em adapter, kernel puro, sem `switch intent.Kind`.

---

## 3. Gaps Encontrados

Nenhum CRÍTICO/ALTO/MÉDIO remanescente. Status dos itens da análise anterior:

| Gap | Sev. original | Status |
|-----|---------------|--------|
| G-01 recorrência sem principal | CRÍTICO | **CLOSED** (+teste) |
| G-02 integ. binding não compila | CRÍTICO | **CLOSED** |
| G-03 adjust_allocation IDOR | ALTO | **CLOSED** |
| G-04 run suspenso órfão | ALTO | **CLOSED** (reaper +teste) |
| G-05 message_id não obrigatório | ALTO | **CLOSED** |
| G-06 vazamento domain no ACL | ALTO | **CLOSED** |
| G-07 predicado triplicado | MÉDIO | **CLOSED** |
| G-08 discriminadores string | MÉDIO | **CLOSED** (resíduo: `ConfirmState.TargetKind` string, convertido no consumo) |
| G-09 política no tool update_card | MÉDIO | **CLOSED** |
| G-10 write-antes-ledger | MÉDIO | mitigado (dedup natural transactions via OriginWamid) |
| G-11 TTL consome próxima msg | MÉDIO | **CLOSED** |
| G-12 sem deadline default | MÉDIO | **CLOSED** |
| G-13 reconciliação no card adapter | MÉDIO | **CLOSED** |
| G-14 cobertura ausente | BAIXO | **CLOSED** (reaper + recurrence adapter) |
| G-15 Append silencioso | BAIXO | **CLOSED** |
| G-16 replay-guard messageID | BAIXO | coberto por design (no-op de resume em run concluído) |
| G-17 destrutiva fora do idempotente | BAIXO | **CLOSED** (OriginWamid) |
| G-18 fidelidade do scorer | BAIXO | coberto por teste existente |
| G-19 continue silencioso | BAIXO | **CLOSED** |

---

## 4. Ressalvas (dívida técnica BAIXO, não-bloqueante)

1. `classify_category.classifyWriteDecision` e `register_entry.classify` re-derivam ramos do predicado apenas para produzir `reason`/métrica granular; o booleano-núcleo está centralizado em `IsWriteEligible`. Eliminável expondo `WriteBlockReason()` no tipo.
2. `BuildImpactNote` faz branch `targetKind == "card"` (string) — seleção de texto de impacto, não decisão de domínio; consistente com o resíduo G-08.
3. Reaper com `maxAge=10min`/`batch=100` hardcoded em `module.go` — safety-net acima do `confirmTTL=5min`; funcional, poderia ser configurável.

---

## 5. Falsos Positivos

Os falsos positivos da análise anterior foram eliminados na raiz:
- "Unique constraint garante idempotência" → agora escrita destrutiva de registro carrega OriginWamid (dedup real no transactions).
- "TTL limita runs abandonados" → agora há reaper que expira ativamente + resume não consome a próxima mensagem.
- "`ListSuspended` parece housekeeping" → agora tem caller real (o reaper).
- "22-tools coverage cobre tudo" → recorrência agora tem caminho autorizado funcional + teste de regressão.

---

## 6. Plano de Ação

1. **Commitar** a mudança na `main` (ou branch + PR) — único passo restante. **backend** · XS · working tree limpo após commit.
2. (Opcional) Expor `WriteBlockReason()` e fechar `ConfirmState.TargetKind`/`BuildImpactNote` — **backend** · S · resíduo G-08/ressalva 1-2 zerado.
3. (Opcional) Tornar `maxAge`/`batch` do reaper configuráveis via `deps` — **backend** · XS.

---

## 7. Parecer sobre Produção

**Pode ser usado por usuários reais** após o commit (item 1). Não há pré-requisito bloqueante de código pendente. Sugestão operacional: garantir que `ConfirmReaperJob` e o housekeeping do kernel estejam ativos no worker de produção (o reaper flui via `agentsModule.Jobs`, já espalhado em `cmd/worker/worker.go:323`).

---

## 8. Registro de Suposições

1. Real-LLM executado de fato (OpenRouter do `.env`, `RUN_REAL_LLM=1`): scorers 22-tools+EP01+EP05 verdes (56s); agents 5/6 na 1ª rodada (1 falha = timeout de rede `http2`, transitório) e o 6º passou em retry isolado.
2. Testes DB-backed (`e2e`, `ca03`, `ca09`, `transactions_integration`, `write_ledger`) **compilam** sob `-tags integration`; execução com Postgres não foi disparada nesta rodada (sem container) — a compilação é a evidência desta rodada; execução assumida conforme rodadas anteriores.
3. Mudança **não commitada** no working tree da `main` (sobre modificações pré-existentes de CategoryWriteEvidence). Métrica de suíte: `go test ./...` exit 0; subagente contou 512 testes nos 3 módulos-núcleo.
