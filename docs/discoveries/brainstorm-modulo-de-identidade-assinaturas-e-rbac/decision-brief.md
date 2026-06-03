# DECISION BRIEF

## Problema
O MeControla precisa transformar a landing `https://www.mecontrola.app.br/` em uma jornada de cadastro, venda de plano, ativação e onboarding conectada ao backend Go atual, preservando a promessa de que o produto funcionará 100% no WhatsApp para uso diário. A decisão precisa evitar três riscos inaceitáveis: vazamento cross-user, cobrança incorreta ou não reconciliada e dependência externa forte sem plano de troca.

## Objetivo
Definir uma direção robusta, eficiente, econômica e production-ready/proof para identidade, assinatura recorrente, RBAC, ownership de recursos, onboarding e integração WhatsApp. O critério de sucesso é permitir que um usuário compre via site, pague por Mercado Pago Pix, tenha assinatura ativada por webhook idempotente, vincule WhatsApp com segurança e acesse somente os próprios recursos.

## Escopo Inicial
Inclui:
- Site como camada de aquisição, cadastro/login, escolha de plano, checkout Mercado Pago Pix e status mínimo de assinatura.
- Backend Go como fonte canônica de usuário interno, assinatura, plano, RBAC, ownership, auditoria, onboarding e autorização.
- OAuth/OIDC gerenciado para autenticação, sem implementar servidor OAuth próprio no MVP.
- Mercado Pago Pix como meio/provedor inicial de validação de pagamento.
- Webhooks idempotentes, reconciliação de pagamento, bloqueio/desbloqueio por assinatura e eventos críticos via outbox quando houver side-effect que precise sobreviver a restart.
- WhatsApp como canal principal de uso após pagamento, ativação e vinculação segura.
- Vinculação WhatsApp por token/código temporário com expiração, confirmação de posse do número e auditoria.

Exclui:
- Login por senha dentro do WhatsApp.
- Transferir RBAC crítico, ownership, plano e auditoria para o provedor OAuth ou para o Mercado Pago.
- Implementar múltiplos PSPs no MVP; a arquitetura deve prever porta interna para troca futura.
- Implementar SSO corporativo, organizações/equipes e multi-tenant organizacional completo no MVP; a decisão deve preservar caminho de evolução.
- Implementar checkout ou conciliação de pagamento fora do backend canônico.

## Restrições
- Preservar o monolito modular Go com arquitetura hexagonal.
- Respeitar fronteiras entre `identity`, `finance`, `conversation`, `notifications`, `telemetry` e infraestrutura.
- Segurança e isolamento cross-user são inegociáveis.
- Monetização por assinatura recorrente deve ser validada cedo.
- Operação, conformidade, auditoria e idempotência são requisitos de produção.
- O produto deve funcionar 100% no WhatsApp para uso diário, mas cadastro, pagamento e gestão mínima de assinatura podem ocorrer no site.
- O work item 29 do Azure DevOps ainda não foi consultado nesta sessão por bloqueio de login interativo no MCP.

## Hipóteses
- O módulo `identity` é a fronteira natural para usuário, sessão, RBAC, vínculo WhatsApp e auditoria de acesso; evidência: README de `internal/identity`.
- O backend já possui infraestrutura útil para produção, incluindo PostgreSQL, `UnitOfWork[T]`, outbox, eventos, observabilidade e runtime; evidência: README e `internal/infrastructure`.
- Mercado Pago Pix atende a validação econômica inicial por ter custo de pagamento menor que cartão para ticket baixo; evidência: comparação de tarifas consultada durante o brainstorming.
- OAuth/OIDC gerenciado reduz risco de segurança frente a implementar servidor OAuth próprio no MVP.
- RBAC, plano e ownership no banco do MeControla reduzem lock-in e preservam controle do domínio.

## Alternativas Avaliadas
### Alternativa 1 - Monolito modular com identidade/RBAC próprios e Mercado Pago Pix por adapter
Resumo:
Implementar identidade, RBAC e ownership no MeControla, integrando Mercado Pago Pix por adapter e webhooks idempotentes.

Viabilidade:
Alta viabilidade técnica pela aderência ao codebase. Boa viabilidade operacional se houver testes de autorização e observabilidade. Boa viabilidade financeira por evitar custo de identidade gerenciada paga e concentrar custo no Pix.

### Alternativa 2 - Auth gerenciado + Mercado Pago Pix
Resumo:
Usar um IdP OIDC gerenciado para autenticação e manter billing com Mercado Pago Pix.

Viabilidade:
Alta velocidade para autenticação, mas exige disciplina para manter autorização, plano e ownership no MeControla. Viável financeiramente em 500 MAU por free tier, com risco de lock-in e custos futuros por recurso avançado.

### Alternativa 3 - Billing-first incremental com Mercado Pago Pix e RBAC mínimo
Resumo:
Priorizar checkout, cobrança e bloqueio por assinatura, com RBAC mínimo e evolução posterior de auditoria/autorização.

Viabilidade:
Mais rápida para validar receita, mas fraca para o risco inaceitável de vazamento cross-user e auditoria robusta. Deve ser evitada como direção principal.

### Alternativa 4 - Plataforma multi-tenant robusta desde o início
Resumo:
Implementar organizações, múltiplos papéis, SSO, múltiplos PSPs, auditoria completa e políticas finas no primeiro ciclo.

Viabilidade:
Robusta em tese, mas lenta, cara e operacionalmente pesada para MVP. Aumenta risco de atraso antes de validar pagamento e uso real no WhatsApp.

### Alternativa 5 - Híbrida em fases com contratos fortes e Mercado Pago Pix no MVP
Resumo:
Usar site para aquisição/checkout, OIDC gerenciado para autenticação, Mercado Pago Pix para assinatura inicial, backend Go como fonte canônica de plano/RBAC/ownership/auditoria e WhatsApp como canal de uso diário. Prever portas para SSO, organizações e múltiplos PSPs sem implementá-los agora.

Viabilidade:
Melhor equilíbrio técnico, operacional e financeiro. Preserva segurança e evolução futura, reduz custo inicial e valida monetização sem superdimensionar plataforma.

## Trade-offs
- Alternativa 5: aceita dependência externa de OIDC e Mercado Pago Pix, mas não delega autorização crítica nem ownership dos recursos a esses provedores.
- Alternativa 5: aceita não implementar SSO, organizações e múltiplos PSPs no MVP, mas exige contratos internos claros para evolução futura.
- Mercado Pago Pix: aceita menor fricção de custo em troca de possível fricção de pagamento versus cartão recorrente automático.
- WhatsApp 100% para uso diário: aceita site mínimo para aquisição e gestão de assinatura, mantendo a experiência principal no canal conversacional.

## Riscos
- Risco: Vazamento cross-user por filtro de ownership ausente ou inconsistente.
  Impacto: Crítico, com exposição de dados financeiros e perda de confiança.
  Probabilidade: Média se autorização ficar espalhada em handlers/adapters.
  Mitigação: Centralizar `AuthenticatedUser`, `owner_user_id`, policies de aplicação, testes de autorização e consultas sempre escopadas por usuário.
- Risco: Pagamento confirmado mais de uma vez ou status divergente entre Mercado Pago e MeControla.
  Impacto: Alto, com ativação indevida, bloqueio incorreto ou suporte manual.
  Probabilidade: Média por natureza de webhooks at-least-once.
  Mitigação: Webhook idempotente por `event_id`/payment id, tabela de deduplicação, reconciliação periódica e outbox para side-effects críticos.
- Risco: Lock-in em IdP ou PSP.
  Impacto: Médio a alto em custo e migração futura.
  Probabilidade: Média.
  Mitigação: Portas de aplicação para IdP e PSP, armazenar `external_subject` separado de `user_id`, manter plano/RBAC/ownership no MeControla.
- Risco: Free tier usado como estratégia de produção.
  Impacto: Médio por surpresa de custo, suspensão ou limite operacional.
  Probabilidade: Média.
  Mitigação: Conta corporativa, budgets, alertas, runbook de custo e decisão explícita de provedor antes da implementação.

## Custos
Estimativa relativa:
Baixa para identidade em 500 MAU quando usando free tier de Cognito, Entra External ID ou Firebase/Auth; baixa a média para pagamento com Mercado Pago Pix conforme volume e ticket.

Drivers de custo:
- Tarifa Mercado Pago Pix por pagamento confirmado.
- Ticket médio e número de assinantes pagantes.
- Recursos pagos de IdP, como SMS, MFA avançado, SAML/OIDC enterprise, logs avançados ou MAU acima do free tier.
- Observabilidade, banco, worker e tráfego em produção.
- Suporte operacional para pagamentos pendentes, chargeback quando houver cartão futuro, cancelamento e reativação.

## Impactos Operacionais
- Necessário runbook para webhook Mercado Pago, reconciliação, reprocessamento idempotente e suporte a pagamento pendente.
- Necessário painel ou endpoint administrativo mínimo para consultar usuário, assinatura, status de pagamento e vínculo WhatsApp.
- Necessário monitorar falhas de webhook, latência de ativação, divergência de status, falhas de vinculação WhatsApp e bloqueios por assinatura.
- Deploy deve preservar migrations, rollback e compatibilidade de eventos.
- Conta de nuvem/free tier deve ter owner corporativo, budget alert e plano pós-free-tier.

## Segurança
- OAuth/OIDC deve usar Authorization Code + PKCE; não implementar servidor OAuth próprio no MVP.
- API deve validar issuer, audience, expiração, assinatura JWKS, state/nonce quando aplicável e rotação de chaves.
- RBAC crítico, plano e ownership devem ser consultados no MeControla, não confiados apenas em claims externas.
- WhatsApp não deve receber senha; vinculação deve usar token/código temporário, expiração, confirmação de posse do número e auditoria.
- Dados financeiros e PII precisam de redaction em logs e trilha de auditoria para eventos sensíveis.

## Observabilidade
- Métricas: logins, criação de checkout, Pix pendente/pago/expirado, webhooks recebidos, webhooks duplicados, ativação de assinatura, falhas de vinculação WhatsApp e bloqueios por assinatura.
- Logs: correlação por `request_id`, `user_id`, `subscription_id`, `payment_id` e `webhook_event_id`, sem vazar PII sensível.
- Traces: fluxo site -> backend -> Mercado Pago -> webhook -> outbox -> WhatsApp/onboarding.
- Alertas: queda de webhook, aumento de pagamentos pendentes, falha de reconciliação, erro de validação OIDC e erro de autorização.

## Escalabilidade
- 500 MAU e 500 assinantes/mês cabem em monolito modular com PostgreSQL e workers.
- Crescimento deve ser tratado com índices por `owner_user_id`, `subscription_id`, `payment_id` e `whatsapp_phone`.
- Webhooks e eventos críticos devem ser processados de forma idempotente e reprocessável.
- O desenho com portas permite trocar Mercado Pago ou IdP sem reescrever domínio de usuário, plano e autorização.

## Alternativa Recomendada
Alternativa 5 - Híbrida em fases com contratos fortes e Mercado Pago Pix no MVP

## Justificativa
A Alternativa 5 é a melhor no contexto atual porque combina robustez production-ready com custo inicial baixo e entrega progressiva. Ela valida venda e pagamento com Mercado Pago Pix, mantém o backend Go como fonte canônica de autorização e ownership, usa OIDC gerenciado para reduzir risco de segurança e preserva o WhatsApp como canal principal sem colocar senha ou lógica crítica dentro da conversa.

## Decisões Pendentes
- Validar o conteúdo do work item 29 no Azure DevOps após login interativo do MCP.
- Escolher explicitamente o IdP OIDC: Cognito, Entra External ID ou Firebase/Auth.
- Confirmar ticket, planos finais e regra de recorrência Pix no Mercado Pago.
- Confirmar provedor/API oficial de WhatsApp e limites operacionais.
- Definir se o site atual será mantido como landing estática com backend separado ou integrado em aplicação web com rotas autenticadas.

## Próximo Passo Recomendado
technical-discovery-production com objetivo de detalhar arquitetura, fluxos, contratos, tabelas, eventos, segurança, observabilidade, custos e plano de implementação da Alternativa 5.
