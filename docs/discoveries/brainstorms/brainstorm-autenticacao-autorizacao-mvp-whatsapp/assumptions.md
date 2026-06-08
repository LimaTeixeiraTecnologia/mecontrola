# Hipóteses e Premissas

## Hipóteses Confirmadas
| ID | Hipótese | Evidência | Impacto | Status |
| --- | --- | --- | --- | --- |
| H1 | LLM + WhatsApp bridge rodam in-process com o backend Go no MVP. | Arquitetura monolítica modular declarada em `AGENTS.md`; ADR-009 onboarding (deploy VPS único). | Permite descartar JWT no MVP — Principal flui via `context.Context`. | confirmada |
| H2 | `internal/identity` já expõe `FindUserByWhatsApp` apto a resolver `wa_id→user_id`. | Arquivo `internal/identity/application/usecases/find_user_by_whatsapp.go`. | Webhook reaproveita usecase existente. | confirmada |
| H3 | `RequireUser` via `X-User-ID` no PRD card é transitório por design. | Comentário explícito no PRD `.specs/prd-card-crud-mvp/prd.md` e nota equivalente em categories/budgets. | Card/categories/budgets bloqueiam até `RequireUser` definitivo existir. | confirmada |
| H4 | `outbox.Publisher` é o canal padrão para eventos de domínio com garantia transacional. | Regra documentada em `CLAUDE.md` seção Outbox. | Auth events seguem o mesmo contrato — sem publicação direta. | confirmada |
| H5 | VPS Hostinger não possui KMS/Secret Manager gerenciado. | ADR-009 e ADR-011 onboarding. | Justifica adiar JWT Ed25519 (gestão de kid/rotação manual é vetor de erro). | confirmada |
| H6 | `internal/identity/domain/pii` já oferece VOs com método `Masked()`. | Diretório `internal/identity/domain/pii` no codebase. | Reaproveitar para mascarar `wa_id` em logs/spans sem código novo. | confirmada |

## Hipóteses Não Validadas
| ID | Hipótese | Risco se falsa | Como validar | Dono |
| --- | --- | --- | --- | --- |
| H7 | 60 msg/min por user é teto adequado para WhatsApp humano + agente. | Sub-limite causa friction; super-limite permite abuso/scraping. | Medir P95 de mensagens/min durante 2 semanas pós-lançamento via métrica OTel `auth_messages_per_minute`. | Operações |
| H8 | Rate-limit in-memory sustenta única instância VPS por 12 meses. | Falha quando primeiro plano de escalar horizontal aparecer. | Acompanhar métrica `cpu_load`, `rps_total`; alertar quando RPS médio > 50 ou quando ADR de escala horizontal for proposto. | Engenharia |
| H9 | Cache TTL curto (60s) para `wa_id→user_id` não causa staleness em troca de número. | Usuário troca número, mensagem cai em user_id antigo durante TTL. | Invalidar cache no usecase `UpdateUserWhatsApp` (futuro) via outbox event. | Engenharia |
| H10 | LLM tool calls jamais sairão do mesmo processo Go. | Se LLM for hospedado externo (ex.: function calling remoto), contrato in-process quebra. | Reavaliar quando ADR de "LLM out-of-process" for considerado; até lá, premissa válida. | Arquitetura |
| H11 | App/web ficam fora do MVP e podem esperar PRD dedicado (≥3 meses). | Se prazo encurtar, JWT precisa entrar antes do esperado. | Confirmar roadmap com Produto a cada release. | Produto |

## Restrições Confirmadas
- Deploy permanece em VPS Hostinger; sem KMS gerenciado, sem managed identity provider.
- Postgres é o único storage; nada de Redis/Memcached/Vault.
- Outbox + projector é o único mecanismo de eventos de domínio com garantia.
- Zero dependência externa nova no MVP (sem Auth0/Clerk/Keycloak/Supabase/Vault).
- LGPD: audit log com retenção mínima 90 dias; PII mascarada (R7); revogação de sessões ao marcar usuário deletado.
- `wa_id`, `email`, `IP` são PII e nunca podem aparecer crus em logs estruturados ou spans OTel.

## Preferências Não Bloqueantes
- Preferência por contratos explícitos no domínio (`auth.Principal`) sobre primitivas de transporte.
- Preferência por reaproveitar `internal/identity` e `internal/onboarding` em vez de criar `internal/auth` separado — discovery técnico decidirá pacote final.
- Preferência por testes table-driven + property-based no validador HMAC.
