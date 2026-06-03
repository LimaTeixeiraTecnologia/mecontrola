# Hipóteses e Premissas

## Hipóteses Confirmadas
| ID | Hipótese | Evidência | Impacto | Status |
| --- | --- | --- | --- | --- |
| H1 | O MeControla é um monolito Go com módulos internos e arquitetura hexagonal. | README do repositório e READMEs dos módulos `identity` e `finance`. | A decisão deve preservar fronteiras de módulo e evitar dependências circulares. | confirmada |
| H2 | O módulo `identity` é a fronteira natural para usuário, sessão, JWT/refresh, RBAC e auditoria de acesso. | README de `internal/identity`. | Identidade e autorização devem ser desenhadas primeiro como contratos de aplicação/domínio, com adapters externos isolados. | confirmada |
| H3 | O projeto já tem infraestrutura útil para produção, incluindo PostgreSQL, UnitOfWork, outbox e observabilidade. | README do repositório e arquivos em `internal/infrastructure`. | Side-effects críticos de billing/assinatura devem considerar outbox; sinais voláteis podem usar event bus. | confirmada |
| H4 | Mercado Pago Pix é o meio/provedor inicial definido para validação da assinatura. | Resposta do usuário: "Seguir para validar a ideia Mercado Pago Pix, definido". | A análise financeira e operacional deve priorizar Pix no Mercado Pago, sem eliminar portas para troca futura. | confirmada |

## Hipóteses Não Validadas
| ID | Hipótese | Risco se falsa | Como validar | Dono |
| --- | --- | --- | --- | --- |
| NV1 | O work item 29 contém requisitos de identidade, assinatura ou controle de acesso. | A decisão pode divergir do backlog oficial. | Reautenticar MCP do Azure DevOps e consultar o item 29. | Produto/Tech Lead |
| NV2 | A assinatura recorrente é requisito do MVP e não apenas evolução posterior. | Superdimensionar billing antes de validar aquisição e retenção. | Confirmar escopo da Rodada 2 e conteúdo do item 29. | Produto |
| NV3 | O produto precisa suportar apenas usuários finais individuais no início, sem organizações/equipes. | Modelo de RBAC e tenancy pode nascer estreito demais. | Confirmar persona e modelo comercial nas rodadas de clarificação. | Produto |
| NV4 | O provedor de pagamento ainda não está escolhido. | Decisão pode depender de Stripe, Mercado Pago, Asaas, Iugu ou outro PSP. | Confirmar restrições de país, moeda, nota fiscal, pix/cartão e custo. | Produto/Operação |
| NV5 | A resposta "Tidas" na Rodada 1 significa "Todas". | Priorização pode ser registrada mais ampla que a intenção real do usuário. | Confirmar implicitamente na próxima resposta ou corrigir se necessário. | Usuário |
| NV6 | O escopo inicial é identidade + assinatura recorrente + bloqueio por plano, mas sem excluir itens avançados da decisão. | Pode haver conflito entre MVP executável e plataforma completa. | Resolver na comparação de alternativas e trade-offs. | Produto/Tech Lead |

## Restrições Confirmadas
- Preservar monolito modular e fronteiras hexagonais existentes.
- Não gerar código, PRD, backlog, tasks, épico, user stories ou arquitetura final nesta skill.
- Segurança e isolamento cross-user são restrição dominante.
- Monetização rápida por assinatura recorrente é restrição dominante.
- Operação, conformidade, auditoria e idempotência são restrições dominantes.

## Preferências Não Bloqueantes
- Robustez production-ready.
- RBAC explícito.
- Cada usuário deve acessar somente seus próprios recursos.
- Pagamento recorrente por assinatura.
- Auditoria, conformidade e webhooks idempotentes aparecem como parte do resultado mínimo desejado.
- Mercado Pago Pix como meio/provedor inicial para validar pagamento recorrente.
