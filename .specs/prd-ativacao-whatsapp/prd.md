# Documento de Requisitos do Produto (PRD) — Jornada de Ativação via WhatsApp

<!-- spec-version: 2 -->

## Visão Geral

Esta funcionalidade entrega a **jornada completa de ativação do Me Controla após a confirmação de pagamento da Kiwify**, terminando no momento em que a conta do usuário é ativada pelo WhatsApp e a mensagem de boas-vindas é entregue.

O usuário deve sentir que comprou um produto premium: tudo acontece automaticamente, sem copiar códigos, sem login, sem conceitos técnicos. Basta clicar em um botão e, em menos de 30 segundos, a conta está ativa. A ativação deve ser praticamente invisível.

A jornada cruza dois repositórios e reaproveita módulos já existentes:

- **Backend Go** (`/Users/jailtonjunior/Git/mecontrola`): `internal/billing` (webhook Kiwify — `process_sale_approved.go`, `kiwifypayload/`), `internal/onboarding` (Activation Session = entidade `magic_token`; e-mail via `send_activation_email.go`; consumo via `consume_magic_token.go` e `WhatsAppMessageProcessor`), `internal/identity` (binding por WhatsApp — `establish_principal.go`, `whatsapp_number.go`), `internal/platform/whatsapp` (webhook inbound, `dispatcher.go`, `payload/parser.go`), `internal/platform/notification` (canal WhatsApp), `internal/onboarding/infrastructure/email` (provider SMTP/Resend) e `internal/onboarding/infrastructure/http/client/meta` (client da Graph API).
- **Landing page** (`/Users/jailtonjunior/Git/mecontrola-landingpage`, Astro/Cloudflare): página `/ativar?token=...` (`src/pages/ativar.astro`) e script `public/js/activate.js`.

**Problema que resolve:** hoje a jornada está incompleta e com fricção. Pagamento, criação da assinatura, criação da Activation Session (`magic_token`), envio do e-mail e a página `/ativar` funcionam. Porém:

1. O **consumo da ativação por WhatsApp está 0% integrado em produção**: ao receber a mensagem, o `dispatcher.go:141` apenas classifica `ATIVAR <token>` e retorna `OutcomeNoRoute`; o `inbound_handler.go:37` descarta o resultado. O `ConsumeMagicToken` e o `WhatsAppMessageProcessor.HandleActivation` existem completos, mas só são chamados em testes. Resultado: paga → recebe e-mail → manda a mensagem → **nada acontece**.
2. A UX atual exige código visível: a mensagem é `ATIVAR <token>` e o e-mail aponta direto para `wa.me` com esse código (`send_activation_email.go:107`), violando a experiência sem-fricção.

**Para quem é:** novos assinantes do Me Controla que acabaram de pagar pela Kiwify e precisam conectar o WhatsApp para começar a usar o assistente financeiro.

**Modelo de dados existente (fato verificado, não suposição):** a "Activation Session" do PRD de origem é materializada pela entidade `magic_token` (`internal/onboarding/domain/entities/magic_token.go`). Ciclo de vida real: criada no **checkout** como `PENDING` (`create_checkout_session.go:74`) → marcada `PAID` no **webhook de pagamento** (`subscription_paid_consumer.go` → `mark_token_paid.go`) → marcada `CONSUMED` na **ativação por WhatsApp** (`consume_magic_token.go`). O estado "pronta para ativar" do PRD corresponde a `PAID`.

## Objetivos

- Fechar a lacuna de integração: tornar a ativação por WhatsApp **100% funcional em produção**, do pagamento à mensagem de boas-vindas.
- Eliminar a fricção: a mensagem enviada pelo usuário no WhatsApp deve ser apenas **"Ativar o meu plano"**, sem nenhum código visível; o e-mail leva à página `/ativar`, não a um link com código.
- Garantir correlação confiável e invisível entre a primeira mensagem do usuário e a Activation Session correta, **pelo número de telefone**.
- Ativar a conta de forma idempotente, auditável e resiliente a mensagens duplicadas e reentrega de webhook.
- Entregar a sequência de boas-vindas que conclui a jornada e convida o usuário a começar.

**Critérios de sucesso mensuráveis:**

- Ativação ponta a ponta (do clique em "Abrir WhatsApp" à mensagem de boas-vindas) concluída em **menos de 30 segundos**, medido pelos timestamps persistidos.
- Janela de ativação válida por **24 horas a partir do pagamento confirmado** (`paidAt`); ativações fora da janela caem em caminho de expiração amigável, nunca em silêncio.
- 100% das ativações bem-sucedidas resultam em: token `CONSUMED`, telefone associado ao usuário, assinatura vinculada, sessão invalidada e boas-vindas entregue.
- Zero ativação dupla ou efeito colateral em mensagens duplicadas/reentrega de webhook (idempotência por `event_id` e por WAMID).
- Taxa de correlação bem-sucedida da primeira mensagem rastreável; falhas de correlação (no-match) caem em fallback de suporte explícito e geram métrica/auditoria.

## Histórias de Usuário

- Como **novo assinante**, quero que, após pagar, eu receba um e-mail com um único botão "Ativar no WhatsApp", para que eu não precise entender nada técnico.
- Como **novo assinante**, quero clicar no botão, ver "Tudo certo!" e abrir o WhatsApp com a mensagem já pronta ("Ativar o meu plano"), para que eu só precise apertar enviar.
- Como **novo assinante**, quero que minha conta seja ativada automaticamente ao mandar a primeira mensagem, para que eu não precise digitar códigos nem fazer login.
- Como **novo assinante**, quero receber boas-vindas confirmando que minha conta está ativa e um convite para começar, para que eu saiba que deu certo.
- Como **assinante que abriu o link tarde demais**, quero uma mensagem clara de que o link expirou e como obter ajuda, para que eu não fique travado em silêncio.
- Como **assinante que já ativou**, quero que reenviar mensagens não cause efeito duplicado, para que minha conta permaneça consistente.
- Como **assinante cujo número não foi reconhecido**, quero uma resposta amigável orientando o próximo passo, para que eu não fique sem retorno.

## Funcionalidades Core

### 1. Confirmação de pagamento sem ativação prematura (Webhook Kiwify)
Ao receber pagamento aprovado, o sistema valida assinatura e pagamento, localiza/cria o cliente, persiste a assinatura, marca a Activation Session como `PAID` (pronta para ativar) e dispara o e-mail. A conta do usuário **nunca é considerada ativada neste momento** — a ativação só ocorre quando o usuário envia a primeira mensagem no WhatsApp.

### 2. Activation Session com janela de ativação curta (`magic_token`)
Entidade própria já existente, com identidade, vínculo à assinatura/cliente, telefone esperado (normalizado E.164), e-mail, expiração e status (`PENDING → PAID → CONSUMED`/`EXPIRED`). Token de uso único. A **janela de ativação é de 24 horas medida a partir do pagamento confirmado (`paidAt`)**, configurável por ambiente.

### 3. E-mail de ativação premium apontando para `/ativar`
E-mail enviado automaticamente após o pagamento, com um único CTA "Ativar no WhatsApp" que abre a **página `/ativar?token=...`** (não mais um link direto `wa.me` com código). O token viaja apenas para validar a página e nunca aparece visualmente.

### 4. Página `/ativar` sem fricção, WhatsApp-only
Página que valida o token via backend e mostra apenas o botão "Abrir WhatsApp". Sem login, senha, formulários ou códigos. Trata estados de erro (expirado, pendente, inválido) e de conta já ativa de forma amigável. **Sem Telegram.**

### 5. Mensagem "Ativar o meu plano" e correlação invisível por telefone
O botão abre o WhatsApp com a mensagem pré-preenchida **"Ativar o meu plano"** (sem código). O backend correlaciona a primeira mensagem do usuário à Activation Session `PAID` correta **pelo número de telefone do remetente**, de forma invisível.

### 6. Ativação automática na primeira mensagem
Ao receber a primeira mensagem de um número ainda não vinculado que corresponda a uma Activation Session `PAID`, o sistema ativa: associa o telefone ao cliente, vincula a assinatura ao usuário, marca a sessão como `CONSUMED` e a invalida — tudo idempotente.

### 7. Sequência de boas-vindas
Após a ativação, o usuário recebe a confirmação de boas-vindas e, em seguida, a apresentação do assistente financeiro com o convite textual para começar.

## Requisitos Funcionais

**Webhook e confirmação de pagamento**

- RF-01: Ao receber um evento de pagamento aprovado da Kiwify, o sistema DEVE validar a assinatura do webhook (HMAC) e o status do pagamento antes de qualquer efeito.
- RF-02: O sistema DEVE localizar ou criar o registro de cliente/assinatura a partir dos dados do webhook (incluindo e-mail e telefone do cliente).
- RF-03: No webhook de pagamento, o sistema DEVE marcar a Activation Session correspondente como `PAID` (pronta para ativar) e registrar `paidAt`, sem ativar a conta do usuário.
- RF-04: O sistema NUNCA DEVE marcar a conta do usuário como ativada no momento do webhook; a ativação ocorre exclusivamente após a primeira mensagem no WhatsApp.
- RF-05: O processamento do webhook DEVE ser idempotente: reentregas do mesmo evento não podem marcar `PAID` em duplicidade, enviar e-mails duplicados nem causar efeitos colaterais repetidos.
- RF-06: Após marcar a sessão como `PAID`, o sistema DEVE disparar o envio do e-mail de ativação.
- RF-07: O telefone do cliente recebido da Kiwify (`Customer.mobile`) DEVE ser **normalizado para E.164** ao ser persistido na Activation Session, reutilizando a mesma regra de normalização aplicada ao número de WhatsApp do inbound (hoje em `internal/identity/domain/valueobjects/whatsapp_number.go`), de modo a permitir correlação por número.

**Activation Session e janela de ativação**

- RF-08: A Activation Session DEVE conter, no mínimo: identificador, vínculo ao cliente/assinatura, token de ativação de uso único (armazenado como hash), telefone esperado em E.164, e-mail, `paidAt`, instante de consumo e status.
- RF-09: Os estados da Activation Session DEVEM ser tipos fechados cobrindo `PENDING`, `PAID`, `CONSUMED` e `EXPIRED` (sem string livre).
- RF-10: A **janela de ativação** DEVE ser de **24 horas a partir de `paidAt`**, configurável por ambiente; após expirar, a sessão é tratada como expirada e não pode ser consumida.
- RF-11: O token de ativação DEVE ser de **uso único** e nunca ser exposto visualmente ao usuário em nenhum canal (e-mail, página, mensagem do WhatsApp).
- RF-12: Sessões expiradas DEVEM ser tratadas explicitamente (caminho de erro amigável + suporte na página e em resposta no WhatsApp), nunca falhar em silêncio.

**E-mail**

- RF-13: O e-mail de ativação DEVE conter um único CTA "Ativar no WhatsApp" cujo destino é a **página `/ativar?token=...`** da landing page (canônica `https://mecontrola.app.br/ativar`), carregando o token apenas como parâmetro de validação, não visível ao usuário.
- RF-14: A base de URL da página de ativação DEVE ser configurável por ambiente (nova configuração dedicada), substituindo a construção atual de link direto `wa.me` no e-mail.
- RF-15: O envio do e-mail DEVE ser idempotente em relação à Activation Session (não reenviar para a mesma sessão por reentrega de evento).

**Página `/ativar`**

- RF-16: A página `/ativar` DEVE validar o token consultando o backend (`GET /api/v1/onboarding/tokens/{token}/state`) e exibir apenas o botão "Abrir WhatsApp" quando o token estiver pronto para ativação.
- RF-17: A página DEVE exibir mensagens amigáveis e distintas para token expirado, pagamento ainda em processamento, link inválido e conta já ativa.
- RF-18: A página NÃO DEVE exigir login, senha, formulário ou código, e NÃO DEVE expor o token nem parâmetros técnicos.
- RF-19: A página NÃO DEVE oferecer ativação via Telegram; o backend NÃO DEVE retornar `telegram_deep_link` na jornada de ativação e a UI nunca exibe o botão de Telegram.

**Mensagem e correlação por telefone**

- RF-20: O backend DEVE construir o link de WhatsApp consumido pela página e pelo e-mail de modo que a mensagem pré-preenchida seja exatamente **"Ativar o meu plano"**, sem token, código ou parâmetros técnicos.
- RF-21: Ao receber a primeira mensagem de um número ainda não vinculado, o sistema DEVE correlacioná-la à Activation Session `PAID` **pelo número de telefone do remetente** (normalizado E.164), via consulta dedicada por telefone, sem depender de outreach prévio.
- RF-22: A ativação DEVE ser disparada por **qualquer primeira mensagem** de um número não vinculado que case uma Activation Session `PAID` (texto-agnóstico, com normalização de caixa/acentos), e não apenas pelo literal "Ativar o meu plano".
- RF-23: Quando houver mais de uma Activation Session `PAID` ativável para o mesmo telefone, o sistema DEVE selecionar a **mais recente por `paidAt`** (`ORDER BY paidAt DESC LIMIT 1`).
- RF-24: Quando a primeira mensagem chegar de um número que NÃO corresponde a nenhuma Activation Session `PAID` válida, o sistema DEVE responder com mensagem amigável orientando o usuário (usar o link do e-mail ou falar com o suporte), registrar métrica/auditoria de no-match e NÃO ativar nenhuma conta nem permanecer em silêncio.

**Ativação por WhatsApp (lacuna a fechar)**

- RF-25: O recebimento da mensagem inbound do WhatsApp DEVE estar integrado em produção ao consumo da Activation Session — a mensagem que dispara a ativação NÃO pode ser apenas classificada e descartada (corrigir o caminho `dispatcher.go` → `inbound_handler.go` e wirar o `ConsumeMagicToken`/`WhatsAppMessageProcessor` em produção).
- RF-26: Ao ativar, o sistema DEVE, de forma consistente: associar o telefone (do `msg.From`) ao cliente, vincular a assinatura ao usuário (binding por WhatsApp), marcar a Activation Session como `CONSUMED` e invalidá-la.
- RF-27: A ativação DEVE ser idempotente e resiliente a mensagens duplicadas: reenviar mensagens após a ativação não pode causar segunda ativação nem efeito colateral; deve resultar em resposta consistente (ex.: conta já ativa).
- RF-28: Mensagens duplicadas de webhook (mesmo WAMID) DEVEM ser deduplicadas antes de qualquer efeito de ativação (reutilizando a dedup existente do dispatcher).
- RF-29: O caminho legado de ativação por código `ATIVAR <token>` DEVE ser **removido da jornada e da UX** (incluindo o match no dispatcher e o texto `WA_MSG_PLEASE_USE_ATIVAR_COMMAND`); o usecase de consumo permanece interno, sem expor código ao usuário.

**Caso de borda — telefone ausente na Kiwify**

- RF-30: Quando a Kiwify não fornecer um telefone válido/normalizável, a Activation Session DEVE ser criada normalmente, **sem telefone esperado**; o telefone é persistido a partir do `msg.From` no momento da ativação.
- RF-31: Nesse caso de borda, a página `/ativar` DEVE poder anexar o token ao abrir o WhatsApp **exclusivamente quando não houver telefone esperado**, permitindo correlacionar a ativação por token; no fluxo normal (com telefone esperado) a mensagem permanece apenas "Ativar o meu plano".

**Boas-vindas**

- RF-32: Após a ativação bem-sucedida, o sistema DEVE enviar a mensagem de boas-vindas confirmando que a conta foi ativada e que o WhatsApp está conectado.
- RF-33: Em seguida, o sistema DEVE enviar a apresentação do assistente financeiro com um convite **textual** para começar ("Vamos começar?"), entregue como texto livre dentro da janela de sessão de 24h aberta pelo usuário (o client Meta atual suporta apenas `text`/`template`; botão interativo nativo está fora do escopo deste MVP).
- RF-34: A jornada DEVE terminar exatamente quando: o usuário envia a primeira mensagem, a conta é ativada e as boas-vindas são entregues — sem prosseguir para onboarding, cadastro de cartão, salário, objetivos ou lançamentos.

**Observabilidade, idempotência e auditoria**

- RF-35: O sistema DEVE persistir os timestamps da jornada: pagamento confirmado, e-mail enviado, página de ativação aberta, WhatsApp aberto (quando possível), ativação iniciada e ativação concluída.
- RF-36: Todas as etapas DEVEM produzir logs estruturados e trilha de auditoria suficientes para reconstruir a jornada de um cliente, incluindo o resultado da correlação (match/no-match).
- RF-37: Métricas da jornada DEVEM usar cardinalidade controlada (sem `user_id`, telefone ou e-mail como label).

## Experiência do Usuário

**Fluxo principal (obrigatório, nenhuma etapa pode ser removida):**

1. Landing Page → Checkout Kiwify (Activation Session criada como `PENDING`) → Pagamento aprovado.
2. Webhook recebido → localiza/cria cliente → persiste assinatura → marca sessão `PAID` (+`paidAt`, telefone normalizado E.164) → envia e-mail.
3. Usuário clica em "Ativar no WhatsApp" no e-mail → abre `/ativar?token=...` (token só para validação).
4. Página `/ativar` valida o token → mostra botão "Abrir WhatsApp".
5. WhatsApp abre com a mensagem **"Ativar o meu plano"** pré-preenchida.
6. Usuário envia a mensagem → webhook recebe → correlação invisível por telefone → conta ativada (sessão `CONSUMED`, telefone e assinatura vinculados).
7. Boas-vindas entregues + apresentação do assistente com convite textual "Vamos começar?".

**Conteúdo de UX (tom premium, linguagem amigável):**

- E-mail: "🎉 Pagamento confirmado! Seu acesso está pronto. Agora falta apenas um passo. [Ativar no WhatsApp]".
- Página `/ativar` (token válido): "🎉 Tudo certo! Agora vamos conectar seu WhatsApp. [Abrir WhatsApp]".
- Primeira resposta no WhatsApp: "🎉 Bem-vindo ao Me Controla! Sua conta foi ativada com sucesso. Seu WhatsApp agora está conectado ao Me Controla."
- Segunda resposta (após poucos instantes): "Sou seu assistente financeiro. Vou ajudar você a controlar seus gastos, cartões e orçamento. Vamos começar?".
- No-match (número não reconhecido): mensagem amigável orientando a usar o link do e-mail ou falar com o suporte.

**Requisitos de UX obrigatórios:** zero fricção; nenhum código visível; nenhuma autenticação manual; linguagem amigável; ativação em menos de 30 segundos.

## Restrições Técnicas de Alto Nível

- **Integração Kiwify:** webhook oficial com validação de assinatura (HMAC SHA-1, rotação de secrets) já existente; o fluxo de pagamento aprovado deve continuar compatível com os demais eventos de assinatura (renovação, atraso, cancelamento, reembolso).
- **Canal WhatsApp:** WhatsApp oficial (Meta Cloud API) como canal único; sem Telegram. Número do bot configurado por ambiente (`META_BOT_NUMBER_E164`/`META_BOT_NUMBER_DISPLAY`). O client atual envia apenas `text` e `template` — boas-vindas usam texto livre dentro da janela de 24h.
- **E-mail:** provedor já configurado (SMTP via Resend); reutilizar a infraestrutura de e-mail e templates existentes, alterando apenas o destino do CTA para a página `/ativar`.
- **Dois repositórios:** alterações coordenadas entre o backend Go e a landing page (`mecontrola-landingpage`); o contrato é o endpoint `GET /api/v1/onboarding/tokens/{token}/state`, que entrega o link de WhatsApp pronto (agora com "Ativar o meu plano") e os estados de UI.
- **Correlação por telefone:** requer nova consulta dedicada `por telefone` em estado `PAID` sem exigência de outreach prévio (a query existente `FindPaidByMobileForFallback` exige `outreach_sent_at IS NOT NULL` e não serve ao fluxo de correlação por telefone); requer normalização do `Customer.mobile` da Kiwify para E.164 (a regra de normalização hoje é privada em `whatsapp_number.go` e precisa ser reutilizável).
- **Janela de ativação a partir de `paidAt`:** a expiração atual da entidade é medida de `createdAt + TTL` (criação no checkout); a janela de ativação de 24h deve ser avaliada a partir de `paidAt`, sem quebrar o job de expiração existente (`expire_tokens.go`).
- **Idempotência e auditoria:** idempotência por `event_id` (webhook/outbox) e por WAMID (mensagem); trilha de auditoria e timestamps persistidos.
- **Estados como tipos fechados:** status da Activation Session e resultados de ativação/correlação devem ser tipos fechados (sem string livre), conforme padrão DMMF do projeto.
- **Rota canônica:** `/ativar?token=` é a rota canônica da landing; `/activate` permanece apenas como redirect que preserva a query.

## Fora de Escopo

- Onboarding conversacional, cadastro de cartão, salário, objetivos e lançamentos financeiros — a jornada termina na mensagem de boas-vindas.
- Ativação via Telegram (removida desta jornada).
- Botão interativo nativo do WhatsApp para "Começar" (o client Meta não suporta `interactive` hoje; "Começar" é CTA textual neste MVP).
- Alterações na Landing Page de marketing (apenas a página `/ativar` e o contrato de ativação fazem parte do escopo na landing).
- Reenvio automático de novo e-mail/link após expiração da janela de 24h (tratado apenas como caminho de suporte amigável neste MVP).
- Mudanças nos demais eventos de assinatura da Kiwify além do necessário para preservar compatibilidade.

## Decisões Resolvidas (substituem as questões anteriormente em aberto)

Todas as ambiguidades materiais foram confrontadas com o código e decididas:

1. **Janela de ativação:** 24 horas a partir de `paidAt` (configurável), em vez de 10 min a partir do checkout — este último é inviável porque o token é criado antes do pagamento. (RF-10)
2. **Gatilho de ativação:** qualquer primeira mensagem de número não vinculado que case uma sessão `PAID` (texto-agnóstico), pré-preenchendo "Ativar o meu plano". (RF-22)
3. **No-match:** resposta amigável + orientação/suporte, com métrica/auditoria; nunca ativa conta errada nem silencia. (RF-24)
4. **Botão "Começar":** texto livre dentro da janela de 24h; sem estender o client Meta neste MVP. (RF-33)
5. **Destino do e-mail:** página `/ativar?token=` (não mais link direto `wa.me` com código). (RF-13, RF-14)
6. **Legado `ATIVAR <token>`:** removido da jornada e da UX; usecase de consumo permanece interno. (RF-29)
7. **Telefone ausente na Kiwify:** criar sessão sem telefone esperado; correlacionar via token anexado pela página `/ativar` apenas nesse caso de borda; persistir telefone a partir do `msg.From` na ativação. (RF-30, RF-31)
8. **Múltiplas sessões `PAID` por telefone:** ativar a mais recente por `paidAt`. (RF-23)
9. **Escopo:** jornada completa nos dois repositórios (backend Go + landing `mecontrola-landingpage`).
10. **Normalização de telefone:** reutilizar/expor a regra E.164 hoje privada em `whatsapp_number.go` para normalizar tanto o inbound quanto o `Customer.mobile` da Kiwify.

Não há questões em aberto remanescentes para o desenvolvimento; os detalhes de design (assinaturas de função, nomes de query, migrations, wiring exato em `cmd/`) serão definidos na Especificação Técnica.
