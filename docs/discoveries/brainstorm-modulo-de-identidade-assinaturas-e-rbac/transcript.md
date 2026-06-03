# Transcript do Brainstorming Decisório

## Contexto Inicial
- Pedido original: decidir como criar um módulo de identidade robusta e production-ready para o MeControla, conectado ao work item 29 do Azure DevOps, com pagamento recorrente por assinatura, RBAC e isolamento para cada usuário acessar somente seus próprios recursos.
- Contexto de codebase observado: monolito Go com arquitetura hexagonal, módulos internos `identity`, `finance`, `conversation`, `agent`, `notifications` e `telemetry`; `identity` já é descrito como responsável por usuário, sessão, JWT/refresh, RBAC e audit de acesso; `finance` já prevê dados por usuário; infraestrutura existente inclui PostgreSQL, `UnitOfWork[T]`, outbox, eventos in-process, observabilidade OTel e runtime server/worker.
- Restrição operacional da sessão: o MCP do Azure DevOps falhou ao consultar o work item 29 com a mensagem de que a identidade precisa ser materializada via login interativo no navegador. O conteúdo específico do item 29 ainda não foi validado.
- Hipótese do usuário a ser desafiada: agrupar identidade, assinatura recorrente, RBAC e isolamento de recursos em uma única iniciativa pode acelerar o MVP, mas também pode acoplar autenticação, billing e autorização de domínio se não houver fronteiras explícitas.

## Rodada 1 - Entendimento do Problema
- Pergunta 1: Qual problema principal estamos resolvendo primeiro?
  - Resposta do usuário: C e D.
  - Registro: o problema inicial combina pacote completo de identidade + assinatura + RBAC e conformidade/auditoria. Implicação prática: a decisão tende a exigir fronteiras claras para não transformar uma iniciativa de MVP em uma plataforma ampla demais.
- Pergunta 2: Qual resultado mínimo torna essa decisão bem-sucedida?
  - Resposta do usuário: D.
  - Registro: o resultado mínimo desejado inclui autenticação, assinatura, RBAC, auditoria e webhooks idempotentes. Implicação prática: o mínimo aceitável pelo usuário já tem características production-ready e dependências externas críticas.
- Pergunta 3: O que torna essa decisão urgente agora?
  - Resposta do usuário: "Tidas".
  - Interpretação operacional: tratado como "Todas" (A, B, C e D), salvo correção posterior do usuário.
  - Registro: a urgência combina risco de segurança, risco comercial, risco arquitetural e necessidade de clareza para o work item 29.
- Pergunta 4: Qual risco é mais inaceitável se adiarmos ou escolhermos mal?
  - Resposta do usuário: A, B e D.
  - Registro: são inaceitáveis vazamento cross-user, cobrança incorreta/duplicada/não reconciliada e dependência forte de provedor externo sem plano de troca.

## Rodada 2 - Escopo e Restrições
- Pergunta 1: O que entra no escopo inicial do MVP?
  - Resposta do usuário: B.
  - Registro: identidade + assinatura recorrente + bloqueio por plano. Implicação prática: valida receita cedo, com RBAC inicialmente simples.
- Pergunta 2: O que deve ficar fora da primeira decisão?
  - Resposta do usuário: D.
  - Registro: nada deve ficar fora da primeira decisão. Implicação prática: há tensão direta com a resposta 1B, pois o escopo inicial é focado, mas o usuário não quer excluir organizações/equipes, múltiplos provedores ou SSO/OAuth avançado da decisão.
- Pergunta 3: Qual restrição domina a decisão?
  - Resposta do usuário: A, B e D.
  - Registro: segurança/isolamento, monetização rápida e operação/conformidade dominam a decisão. Implicação prática: a alternativa recomendada precisa provar anti-cross-user, suportar cobrança recorrente e prever idempotência/auditoria/runbooks.
- Pergunta 4: Qual dependência externa você prefere assumir no início?
  - Resposta do usuário: A, com pergunta adicional sobre custo das opções.
  - Registro: preferência inicial por provedor de pagamento gerenciado. Implicação prática: identidade/RBAC tendem a permanecer no monolito, enquanto billing integra por adapter e webhooks idempotentes.

## Rodada 3 - Alternativas
- Nota de custo solicitada pelo usuário na Rodada 2:
  - Opção 4A, pagamento gerenciado e identidade própria no monolito: custo direto externo fica concentrado em tarifa por pagamento recebido; custo interno fica em implementação de identidade, RBAC, isolamento, auditoria e operação de segurança.
  - Opção 4B, identidade gerenciada e pagamento ainda a definir: custo externo combina MAU/recursos de autenticação com integração posterior de cobrança; reduz esforço inicial de auth, mas aumenta lock-in e custo por usuário ativo.
  - Opção 4C, identidade gerenciada + pagamento gerenciado: maior aceleração inicial e maior custo recorrente/lock-in; exige conciliação entre dois provedores críticos.
  - Opção 4D, identidade própria + pagamento externo: custo direto similar à 4A para pagamentos, maior custo de engenharia em auth/RBAC, porém melhor controle sobre autorização, LGPD, auditoria e fronteiras do domínio.
  - Evidências consultadas em 2026-06-03: Stripe Brasil, Mercado Pago Brasil, Asaas, Auth0, Amazon Cognito, Supabase e Clerk. Tarifas específicas devem ser revalidadas antes de contratação.
- Alternativa A: Monolito modular com identidade/RBAC próprios e um PSP inicial por adapter.
  - Racional: preserva a arquitetura atual, mantém autorização e isolamento dentro do domínio `identity`, usa provedor de pagamento gerenciado para recorrência e registra webhooks críticos com idempotência/outbox.
- Alternativa B: Auth gerenciado + PSP gerenciado.
  - Racional: acelera login, MFA e recursos de segurança prontos, mas desloca parte da identidade para fora do monolito e aumenta custo por MAU/lock-in.
- Alternativa C: Billing-first incremental com RBAC mínimo.
  - Racional: entrega assinatura recorrente mais rápido, com isolamento obrigatório por `user_id` e bloqueio por plano, deixando RBAC avançado e auditoria profunda para evolução controlada.
- Alternativa D: Plataforma robusta multi-tenant desde o início.
  - Racional: inclui organizações, papéis, planos, auditoria completa, múltiplos provedores e políticas finas; maximiza flexibilidade futura, mas é a alternativa mais lenta e cara para MVP.
- Alternativa E: Híbrida em fases com contratos fortes.
  - Racional: implementar identidade própria, ownership por usuário, RBAC mínimo, assinatura com um PSP e webhooks idempotentes no MVP; desenhar portas internas para organizações, múltiplos provedores e SSO sem implementá-los agora.
- Decisão parcial registrada pelo usuário após discussão de custos:
  - Mercado Pago Pix definido como ideia/provedor/meio de pagamento inicial a validar.
  - Implicação prática: o brainstorming passa a tratar o pagamento recorrente inicial como integração com Mercado Pago Pix, com webhooks idempotentes, reconciliação de status e bloqueio/desbloqueio de plano. A decisão parcial não fecha, sozinha, se identidade/RBAC será própria, gerenciada ou híbrida.
- Estimativa adicional solicitada: custo real de pagamento + identidade gerenciada para 500 usuários/mês com crescimento.
  - Premissa de cálculo: 500 MAU para identidade; quando todos forem assinantes pagantes, 500 cobranças recorrentes/mês. Ticket exemplo: R$29,90/mês.
  - Identidade gerenciada em 500 MAU: Cognito Lite/Essentials tende a ficar em US$0 para login direto/social dentro do free tier de 10.000 MAU; Supabase pode ficar em US$0 no Free ou US$25/mês no Pro; Auth0 pode ficar em US$0 no Free, mas recursos production-ready como RBAC por organização/audit logs podem empurrar para plano pago a partir de US$35/mês; Clerk deve ser revalidado no pricing oficial antes de decisão por ter modelo/limites sujeitos a mudança.
  - Pagamento gerenciado em 500 cobranças de R$29,90: Stripe com cartão + Billing fica aproximadamente R$896/mês em tarifa; Mercado Pago cartão fica aproximadamente R$595 a R$745/mês conforme prazo de recebimento; Asaas cartão a partir de aproximadamente R$692/mês; Pix fica aproximadamente R$148/mês no Mercado Pago ou R$595 a R$995/mês em modelos com tarifa fixa/percentual conforme PSP; boleto tende a penalizar ticket baixo quando há tarifa fixa por cobrança.
  - Conclusão financeira: em 500 usuários, identidade gerenciada pode custar pouco ou zero se ficar no free tier, mas pagamento consome margem todo mês. O fator econômico dominante é ticket médio, meio de pagamento e taxa de sucesso/chargeback, não MAU.
- Estimativa adicional: contas novas/free tier em Azure, AWS e GCP.
  - AWS: Cognito tem free tier próprio que não expira com o free tier de 12 meses para Lite/Essentials, com 10.000 MAU/mês para login direto/social. Para 500 MAU, custo de identidade tende a ser US$0 se não usar Plus, SMS, M2M pago ou SAML/OIDC enterprise acima do limite.
  - Azure: conta gratuita pode ter crédito inicial e serviços gratuitos por prazo, mas a identidade de clientes deve ser avaliada por Microsoft Entra External ID. O core offering informa gratuidade para os primeiros 50.000 MAU. Para 500 MAU, custo de identidade tende a ser US$0, salvo recursos/licenças adicionais.
  - GCP/Firebase: Firebase Authentication e Identity Platform informam no-cost até 50.000 MAU para auth comum; Phone Auth/SMS é cobrado à parte. Para 500 MAU, custo de identidade tende a ser US$0 se não usar SMS/telefone como canal principal.
  - Risco registrado: "e-mail novo" não deve ser tratado como estratégia financeira production-ready. Elegibilidade depende de provedor, cartão/organização/conta, termos de uso e continuidade operacional. Free tier é bom para sandbox/MVP controlado, mas produção precisa budgets, alertas, conta corporativa, owner claro e plano pós-free-tier.

## Rodada 4 - Trade-offs
- Scorecard preenchido em `option-scorecard.md` usando Mercado Pago Pix como premissa de pagamento inicial.
- Recomendação preliminar para debate: Alternativa E - híbrida em fases com contratos fortes, usando Mercado Pago Pix no MVP e mantendo ownership, plano, permissões e auditoria no banco do MeControla.
- Análise adicional solicitada: recomendação OAuth/OIDC para a stack Go modular do MeControla.
  - Recomendação preliminar: usar OpenID Connect com Authorization Code + PKCE, evitando implementar um servidor OAuth próprio no MVP.
  - Provedor recomendado se a prioridade for robustez, custo inicial baixo e integração limpa com Go: Amazon Cognito User Pools como IdP OIDC gerenciado.
  - Alternativa forte se a estratégia de nuvem for Azure: Microsoft Entra External ID, especialmente se o produto evoluir para integrações corporativas, organizações e SSO.
  - Alternativa simples para MVP web/mobile: Firebase Auth, com atenção a Phone Auth/SMS e menor aderência natural a RBAC corporativo.
  - Decisão de fronteira: tokens OIDC provam identidade; RBAC, plano, auditoria, consentimento, ownership de recursos e bloqueio por assinatura continuam no banco e domínio do MeControla.
  - Bibliotecas Go candidatas para relying party/API: `golang.org/x/oauth2` para fluxo OAuth2/PKCE e `github.com/coreos/go-oidc/v3` para discovery OIDC, JWKS e validação de ID token/JWT.
- Análise adicional solicitada: integrar `https://www.mecontrola.app.br/` com cadastro, venda de plano, onboarding, repositório corrente e produto 100% WhatsApp.
  - Contexto observado no site: landing com promessa "Sua vida financeira organizada, direto no WhatsApp", CTA para planos, planos mensal/trimestral/anual, menção a PIX/cartão e FAQ dizendo que a experiência principal acontece no WhatsApp.
  - Recomendação preliminar: o site deve ser a camada de aquisição, checkout e ativação; o repositório Go deve ser o backend canônico de identidade, assinatura, onboarding, autorização e webhooks; o WhatsApp deve ser o canal principal de uso do produto após ativação.
  - Fluxo recomendado: landing -> seleção de plano -> login/cadastro OIDC -> criação de checkout Mercado Pago Pix -> webhook idempotente confirma pagamento -> backend ativa assinatura -> usuário vincula WhatsApp -> onboarding conversacional -> uso diário 100% WhatsApp.
  - Fronteira crítica: não autenticar por senha dentro do WhatsApp; usar link seguro de vinculação com expiração e confirmar posse do número antes de liberar recursos.

## Rodada 5 - Seleção de Direção
- Síntese apresentada ao usuário ao longo da rodada:
  - O site `https://www.mecontrola.app.br/` deve ser usado para aquisição, cadastro/login, escolha de plano e checkout.
  - O backend Go deste repositório deve ser a fonte canônica de usuários, assinatura, RBAC, ownership, auditoria, webhooks e onboarding.
  - A experiência principal do produto deve funcionar 100% no WhatsApp após ativação e vinculação segura.
  - Mercado Pago Pix foi definido como meio/provedor inicial de validação.
  - OAuth/OIDC gerenciado foi recomendado para autenticação, com RBAC e ownership mantidos no MeControla.
- Decisão explícita do usuário:
  - "documentar e seguir com esse plano, 100% robusto, eficiente, economico, production-ready/proof de forma inegociável"
- Alternativa selecionada:
  - Alternativa 5 - Híbrida em fases com contratos fortes e Mercado Pago Pix no MVP.
- Trade-off aceito:
  - Usar provedor externo para autenticação OIDC e Mercado Pago Pix para pagamento, sem transferir autorização, plano, ownership, auditoria e regras de acesso para esses provedores.
  - Implementar MVP robusto com portas para evolução futura, sem implementar SSO corporativo, múltiplos PSPs e multi-tenant organizacional completo na primeira entrega.

## Decisões Registradas
- D1: Seguir com a Alternativa 5 - Híbrida em fases com contratos fortes e Mercado Pago Pix no MVP.
- D2: Usar `https://www.mecontrola.app.br/` como camada de aquisição, cadastro/login, venda de plano, checkout e status mínimo de assinatura.
- D3: Manter o backend Go deste repositório como fonte canônica de identidade interna, assinatura, RBAC, ownership, auditoria, onboarding e autorização.
- D4: Garantir que a aplicação funcione 100% no WhatsApp para uso diário após pagamento, ativação e vinculação segura do número.
- D5: Validar Mercado Pago Pix como PSP/meio inicial, com webhook idempotente, reconciliação e bloqueio/desbloqueio por status da assinatura.
- D6: Usar OAuth/OIDC gerenciado para autenticação, preferencialmente Cognito se a decisão de nuvem ficar aberta, ou Entra External ID se Azure virar restrição estratégica; RBAC crítico permanece no MeControla.
- D7: Não fazer login por senha dentro do WhatsApp; vincular WhatsApp por token/código temporário, expiração, confirmação de posse do número e auditoria.
