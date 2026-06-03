# Hipóteses e Premissas

## Hipóteses Confirmadas
| ID | Hipótese | Evidência | Impacto | Status |
| --- | --- | --- | --- | --- |
| H1 | Discoveries de billing, identity e onboarding já refletem a direção desejada de produto | Usuário citou as 3 como fonte canônica no prompt `docs/prompts/consolidar-arquitetura-core.md` | Bundle vai consolidar essas discoveries, não reabrir o conteúdo delas | confirmada |
| H2 | Scaffold dos módulos `internal/identity/` e `internal/finance/` está virgem | `ls` mostra somente `doc.go` em cada subpasta (`domain`, `application`, `infrastructure`) | Não há código herdado bloqueando reescrita das fronteiras | confirmada |
| H3 | `internal/platform/outbox` está pronto para uso transacional | Commit `4b7149e feat(outbox): implemente fundacao transacional de eventos` + arquivos em `platform/outbox` e `runtime/outbox_subsystem.go` | Webhook ingress pode publicar via outbox desde a primeira feature | confirmada |
| H4 | `internal/platform/events.Bus` é volátil (sem persistência) | Layout `bus.go` + AGENTS.md cita "Outbox vs events.Bus" como contrato dual | Escolha entre Bus e Outbox precisa entrar nas regras inegociáveis | confirmada |
| H5 | Há drift entre scaffold de identity e discovery | `internal/identity/domain/doc.go:3-4` declara "JWT/refresh, RBAC e audit de acesso"; discovery-identity-entitlement.md seção 0 rejeita RBAC e JWT no MVP | Consolidação precisa corrigir o `doc.go` e reescrever README/AGENTS do módulo | confirmada |

## Hipóteses Não Validadas
| ID | Hipótese | Risco se falsa | Como validar | Dono |
| --- | --- | --- | --- | --- |
| H6 | Landing page atual promete planos Mensal/Trimestral/Anual sem plano família/equipe | `src/lib/content.ts` mostra `plans = [mensal R$29,90, trimestral R$80,73, anual R$297,80]`. Copy 100% individual ("você", "seu dinheiro"). Sem menção a compartilhamento. CHECKOUT_URL ainda `#` (placeholder, landing não publicada com link real) | Decisão "sem RBAC no MVP" é coerente com promessa atual. Mas plano **trimestral** não estava na discovery de billing (que assumia só period_end = now+30d) — máquina de estados precisa generalizar `period_length` por plano | confirmada |
| H7 | Kiwify propaga query param `?s={token}` no webhook (custom field ou UTM) | Sem propagação, magic token quebra; precisa de fallback obrigatório por número e/ou campo custom em formulário | Testar com compra real de R$ 1 (Pix) em produto sandbox da Kiwify | Eng. Billing |
| H8 | Volume MVP esperado < 1k assinantes em 6 meses | Se volume for maior, Redis Streams pode não bastar e cache de entitlement precisa de invalidação distribuída agressiva | Confirmar com usuário (próximas rodadas) | Produto |
| H9 | Time é pequeno (1-2 devs) | Se for maior, faz sentido investir em mais abstração de pacote desde já | Confirmar nas rodadas seguintes | Produto |

## Restrições Confirmadas
- Stack obrigatória: Go (versão de `go.mod`), Postgres, Redis, WhatsApp Business API, Kiwify.
- Arquitetura hexagonal canônica (`domain/` puro, `application/` com ports, `infrastructure/` com adapters) imposta pelo CLAUDE.md + AGENTS.md + depguard.
- Outbox vs Bus: contrato dual obrigatório (`AGENTS.md` referencia a seção); idempotência por `event_id` é regra fixa.
- Comunicação cross-module só por interface declarada em `application/` do consumidor ou por Domain Event via `internal/platform/events`.
- **Planos contratados pela landing:** mensal (30d), trimestral (90d), anual (365d). Máquina de estados precisa carregar `period_length` por plano, não constante.
- **Sem trial nativo** prometido na landing — landing oferece preço direto, sem "teste grátis". Trial fica como flag opcional para experimentos, não como feature do MVP.
- **Modais de pagamento prometidos:** PIX e cartão (compatível com Kiwify).

## Preferências Não Bloqueantes
- Preferência de dossiê único que inclua regras inegociáveis + contratos Go canônicos + roadmap de épicos (Resposta 1.3 do usuário).
- Foco da consolidação em (a) eliminar drift, (b) materializar contratos, (c) validar funil — em paralelo (Resposta 1.1).
- Escopo end-to-end (Identity + Billing + Onboarding) sem incluir LLM gate/router como núcleo (Resposta 2.1).
- Bundle entrega **dossiê apenas**: regras + contratos Go (assinaturas) + roadmap; **código fica para PRD/techspec downstream** (Resposta 2.2).
- Tensão tempo × robustez deve resultar em roadmap enxuto/priorizado (fatia mínima production-proof primeiro), não cobertura ampla (Resposta 2.3).
- Cerco externo dominante: Kiwify + WhatsApp Business; stack interna (Postgres + Redis) é tida como fixa (Resposta 2.4).
