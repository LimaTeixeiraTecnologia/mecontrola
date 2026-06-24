<!-- spec-version: 1 -->

# Documento de Requisitos do Produto (PRD) — Ativação UX

## Visão Geral

O fluxo de ativação atual expõe o token bruto ao usuário de formas que comprometem a experiência:
o email exibe a URL completa com o token como texto de fallback (visualmente feio e potencialmente
confuso), a página web `/ativar` apresenta erro ao tentar validar o acesso em vez de guiar o
usuário ao WhatsApp, e o usuário é forçado a digitar manualmente o comando `ATIVAR {token}` sem
nenhuma automação.

Esta melhoria redesenha o fluxo de ativação em três pontos de contato — **email**, **página de
ativação (bridge)** e **backend** — para que o usuário clique uma vez no email e chegue ao
WhatsApp com o comando de ativação já pré-preenchido, pronto para enviar.

**Para quem é**: novos assinantes que acabaram de pagar e precisam ativar a conta.

**Por que é valioso**: elimina a URL crua do email, conserta a página de ativação que hoje exibe
erro, e reduz o atrito entre o clique no email e o início do onboarding de 3–4 passos para 1 clique.

## Objetivos

- **Email limpo**: 0 exposições de URL crua com token visível ao usuário.
- **Ativação em 1 clique**: o botão do email abre o WhatsApp com `ATIVAR {token}` pré-preenchido.
- **Página bridge funcional**: a página `/ativar?token=TOKEN` valida o token via API e redireciona
  para WhatsApp automaticamente, com estados de erro humanizados em vez de "Algo deu errado".
- **Métricas-chave a acompanhar**:
  - Taxa de clique no botão do email → abertura do WhatsApp (deve aumentar vs. hoje).
  - Taxa de erros na página `/ativar` (deve cair para 0 para tokens válidos).
  - Tempo médio entre envio do email e primeiro envio do comando ATIVAR no WhatsApp.

## Histórias de Usuário

### Caminho principal — Ativação via email no mobile

- Como **novo assinante no celular**, quero clicar no botão do email e ser levado diretamente ao
  WhatsApp com o comando de ativação já digitado, para que eu só precise pressionar "Enviar".
- Como **novo assinante**, quero que o email não exiba código ou URL longa com caracteres
  aleatórios, para que o email pareça profissional e confiável.

### Caminho alternativo — Acesso via desktop

- Como **novo assinante no desktop** (sem WhatsApp instalado no computador), quero que a página
  de ativação me informe o que fazer e me ofereça um QR code ou link para usar no celular, para
  que eu consiga ativar mesmo sem app desktop.

### Caminho de exceção — Token inválido ou expirado

- Como **assinante com token expirado**, quero ver uma mensagem clara sobre o problema e um link
  direto para o suporte, para que eu saiba o próximo passo sem precisar entrar em contato por
  tentativa e erro.
- Como **assinante que já ativou a conta**, quero ver uma confirmação de que a conta já está ativa
  e um link para abrir o WhatsApp com o bot, sem mensagem de erro assustadora.

## Funcionalidades Core

### 1. Botão de email com link wa.me (deep link WhatsApp)

O CTA principal do email de ativação passa a ser um link `https://wa.me/{bot}?text=ATIVAR%20{token}`
que abre o WhatsApp no celular com o comando pré-preenchido. O fallback de URL crua é removido;
se o usuário precisar de ajuda, o email orienta a contatar o suporte.

### 2. Remoção do fallback de URL crua no email

A linha "Se o botão não funcionar, copie e cole este link no navegador: `{URL}`" é removida do
template. Em seu lugar, uma linha simples: "Em caso de dificuldade, entre em contato pelo suporte:
suporte@mecontrola.app.br".

### 3. Página bridge `/ativar` funcional com redirecionamento automático

A página de ativação no repositório `mecontrola-landingpage`:
- Lê o token do query param `?token=`.
- Chama a API `GET /api/v1/onboarding/tokens/{token}/state` do backend.
- Se o token é válido (`ready_to_activate: true`): exibe confirmação visual e inicia contagem
  regressiva de 3 segundos para redirecionamento automático ao `wa_me_url` retornado pela API.
- Disponibiliza botão manual "Abrir WhatsApp" para usuários que não queiram aguardar.
- Exibe o logo oficial do MeControla (sem inventar ou substituir assets).

### 4. Estados de erro humanizados na página bridge

Cada estado negativo do token tem uma mensagem clara:

| Estado da API | Mensagem ao usuário |
|---|---|
| Token não encontrado | "Link inválido. Verifique o link do email ou entre em contato com o suporte." |
| Token expirado | "Seu link de ativação expirou. Entre em contato com o suporte para receber um novo link." |
| Token já consumido | "Sua conta já está ativa! Abra o WhatsApp e envie uma mensagem para o MeControla." |
| Pagamento pendente | "Seu pagamento ainda está sendo processado. Aguarde alguns minutos e tente novamente." |

Todos os estados de erro incluem link wa.me para o bot do MeControla como canal de suporte.

### 5. WaMeURL construída pelo backend no envio do email

O usecase `SendActivationEmail` passa a construir o `WaMeURL` (`https://wa.me/{bot}?text=ATIVAR%20{clearToken}`)
e incluí-lo em `ActivationTemplateInput`. O template HTML usa esse campo como `href` do botão CTA.

## Requisitos Funcionais

- RF-01: O email de ativação DEVE ter o botão "Ativar MeControla" apontando para `wa_me_url`
  (link `https://wa.me/{botNumber}?text=ATIVAR%20{clearToken}`), não para a URL da página web.
- RF-02: O template HTML do email NÃO DEVE conter nenhum texto exibindo a URL completa do token.
  O fallback de "copie e cole este link" DEVE ser removido.
- RF-03: O template HTML do email DEVE incluir, no lugar do fallback removido, uma linha de suporte
  com link wa.me para o bot do MeControla: "Em caso de dificuldade, fale conosco pelo WhatsApp."
- RF-04: O usecase `SendActivationEmail` DEVE construir o `WaMeURL` com o número de bot configurado
  e o `clearToken`, e passá-lo ao `ActivationTemplateInput`.
- RF-05: A página `/ativar?token=TOKEN` no repositório `mecontrola-landingpage` DEVE ler o token
  do query param e chamar `GET /api/v1/onboarding/tokens/{token}/state`.
- RF-06: Se a API retornar `ready_to_activate: true`, a página DEVE exibir estado de sucesso e
  iniciar redirecionamento automático para o `wa_me_url` após 3 segundos.
- RF-07: A página DEVE exibir o botão "Abrir WhatsApp" disponível imediatamente, sem aguardar o
  redirecionamento automático.
- RF-08: Se a API retornar erro ou `ready_to_activate: false`, a página DEVE exibir mensagem
  humanizada conforme tabela de estados (token não encontrado, expirado, já consumido, pagamento
  pendente).
- RF-09: A página de ativação DEVE usar o logo oficial do MeControla provido pelo repositório
  `mecontrola-landingpage`, sem criar ou alterar assets de logo.
- RF-10: A URL canônica da página de ativação usada no email E no roteamento do frontend DEVE ser
  unificada em um único formato (ex.: `/ativar?token=TOKEN`), eliminando a divergência entre
  `/ativar` (email) e `/activate/` (frontend atual).
- RF-11: O endpoint `GET /api/v1/onboarding/tokens/{token}/state` DEVE retornar estado distinto para
  token já consumido (`ConsumeOutcomeAlreadyActive`) com `wa_me_url` incluído, para que a página
  bridge possa oferecer o link do WhatsApp mesmo nesse estado.

## Experiência do Usuário

### Fluxo feliz — Mobile

```
Email recebido
  → Clique em "Ativar MeControla"
  → WhatsApp abre com texto pré-preenchido "ATIVAR {token}"
  → Usuário pressiona Enviar
  → Onboarding inicia automaticamente
```

### Fluxo alternativo — Desktop

```
Email recebido no desktop
  → Clique em "Ativar MeControla"
  → Página bridge abre no navegador
  → Página valida token (spinner de carregamento)
  → Exibe "Conta verificada! Abrindo WhatsApp em 3s..."
  → Botão "Abrir WhatsApp" visível imediatamente
  → Auto-redirect ou clique manual no botão
```

### Fluxo de exceção — Token expirado

```
Clique no link do email (link expirado)
  → Página bridge abre
  → API retorna token expirado
  → Página exibe: "Seu link expirou. Clique aqui para falar com o suporte."
  → Link direto para canal de suporte
```

### Visual e identidade

- Usar exclusivamente o logo oficial presente no repositório `mecontrola-landingpage`.
- Paleta de cores consistente com a identidade existente do projeto (verde `#16a34a` / dark `#0f172a`).
- Layout mobile-first: a maioria dos usuários acessará pelo celular após clicar no email.
- Sem criar novos assets visuais ou ícones não presentes no repositório.

## Restrições Técnicas de Alto Nível

- **[HARD] Zero regressão funcional**: o fluxo de ativação atual funciona 100% — o comando
  `ATIVAR {token}` é processado corretamente pelo WhatsApp. Toda mudança é exclusivamente de UX
  (email, template, página bridge). Nenhuma alteração pode tocar a lógica de `consume_magic_token`,
  `whatsapp_message_processor`, `dispatcher`, regex de ativação ou qualquer outro componente do
  caminho de processamento do token. Regressão funcional é bloqueante e inaceitável.

- **Dois repositórios**: mudanças de backend (email, usecase) neste repositório Go; mudanças de
  frontend (página bridge) no repositório `mecontrola-landingpage`. O PRD cobre os dois, mas
  implementações são independentes e coordenadas pelo contrato da API `get_token_state`.
- **API existente**: o endpoint `GET /api/v1/onboarding/tokens/{token}/state` já existe
  (`get_token_state.go`) e já retorna `wa_me_url`. Ajustes mínimos podem ser necessários para
  incluir o estado `already_consumed` com wa_me_url.
- **Configuração de bot**: o número do bot WhatsApp já é configurado no backend (usado por
  `get_token_state.go`). O mesmo valor deve ser acessível em `SendActivationEmail`.
- **Token no wa.me**: o token claro (não hash) é exposto no link wa.me, o que é equivalente
  à exposição atual na URL do email — não representa regressão de segurança.
- **Expiração**: o link wa.me construído no email não expira por si só, mas o backend rejeita o
  token ao processar o comando ATIVAR se ele estiver expirado ou já consumido — sem mudança nesse
  comportamento.
- **Logo oficial**: a página bridge DEVE usar o logo oficial já presente no repositório
  `mecontrola-landingpage`. Nenhum novo asset de logo deve ser criado ou inventado.

## Fora de Escopo

- Mudança no processamento do comando `ATIVAR {token}` no WhatsApp (backend de ativação funciona).
- Alteração na lógica de geração, expiração ou consumo do token mágico.
- Adição de canal de ativação novo (Telegram, SMS, etc.).
- Fluxo de reenvio automático de email (usuário solicitar novo link).
- Autenticação ou login na página bridge — a página é pública e stateless.
- Internacionalização (i18n) da página bridge — apenas português por ora.
- Qualquer mudança no fluxo de onboarding conversacional pós-ativação (`prd-onboarding-v2`).

## Decisões Confirmadas

- **Canal de suporte**: o link de suporte no email (RF-03) e na página bridge (RF-08) aponta para o
  mesmo número de bot do MeControla via wa.me. Não há email ou número humano separado.
- **Rota canônica**: `/ativar?token=TOKEN` é a rota definitiva. O caminho `/activate/` atual no
  frontend deve virar redirect 301 para `/ativar`.
- **Timeout da API no frontend**: 5 segundos; exibir estado de erro de rede distinto caso a API
  não responda — será detalhado na techspec.
