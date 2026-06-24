# Registro de Decisão Arquitetural — ADR-002

## Metadados

- **Título:** Botão CTA do email de ativação aponta para deep link wa.me
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Time de produto + plataforma
- **Relacionados:** `techspec.md`, `prd.md` RF-01, RF-02, RF-04

## Contexto

O email de ativação atual tem um botão "Ativar MeControla" que leva o usuário para a página web
`/ativar?token=TOKEN`. A página exibia erro ("Algo deu errado"), obrigando o usuário a digitar
manualmente o comando `ATIVAR {token}` no WhatsApp. O fluxo funcional existe, mas a UX é ruim:
exposição do token como URL crua no fallback de texto, página com erro para tokens válidos e
etapas manuais desnecessárias.

A decisão foi tomada pelo usuário: o CTA do email deve ir diretamente ao WhatsApp via `wa.me`.

## Decisão

O campo `href` do botão CTA no template HTML passa a ser o deep link `wa.me`:
```
https://wa.me/{botNumber}?text=ATIVAR%20{clearToken}
```

O `SendActivationEmail` usecase passa a construir esse URL (recebe `botNumber` como dependência
injetada via constructor — já disponível como `waCfg.BotNumberE164` no wiring). O campo
`ActivateURL` (URL da página web) é removido do `ActivationTemplateInput`.

A linha de fallback "Se o botão não funcionar, copie e cole este link no navegador: `{URL}`" é
removida. No lugar, uma linha de suporte: "Dificuldades? Fale conosco pelo WhatsApp."

## Alternativas Consideradas

### 1. Manter CTA na página web + corrigir a página
- Vantagem: a página web existe e pode ser usada para validar o token antes de redirecionar.
- Desvantagem: adiciona um passo extra (web → WhatsApp) que não agrega valor em mobile; a página
  está com erro atual que assusta usuários; no mobile, wa.me é mais confiável que uma landing page.
- **Rejeitada**: a página bridge continua existindo como fallback; o email vai direto ao destino
  final (WhatsApp), eliminando o intermediário desnecessário no fluxo mobile principal.

### 2. Dois botões no email: WhatsApp (primário) + web (secundário)
- Vantagem: cobre desktop sem WhatsApp instalado.
- Desvantagem: wa.me no desktop abre WhatsApp Web — o usuário sem WhatsApp ativo não conseguiria
  ativar de qualquer forma (o produto requer WhatsApp). Dois CTAs aumentam a carga cognitiva.
- **Rejeitada**: o produto requer WhatsApp; usuário sem WhatsApp não consegue usar o produto
  independentemente do CTA. O segundo botão seria ruído.

### 3. Manter URL da página web no CTA, remover apenas o fallback de texto
- Vantagem: menor impacto no backend.
- Desvantagem: não resolve o problema principal — a página ainda é um intermediário que adiciona
  fricção e que apresentava erro.
- **Rejeitada**: não atinge o objetivo de "ativação em 1 clique".

## Consequências

### Benefícios Esperados

- Fluxo mobile reduzido de 3 passos (clicar → copiar/digitar → enviar) para 2 (clicar → enviar).
- Zero exposição de URL crua com token no corpo do email.
- Email tem aparência profissional: botão limpo + linha de suporte simples.
- Independência da página web: se a landing page tiver problema, o email continua funcional.

### Trade-offs e Custos

- O token claro continua exposto no `href` do botão (visível ao inspecionar o email).
  Isso não é regressão: o token estava igualmente exposto na URL do texto de fallback.
  O ataque de interceptação de email com token é o mesmo nível de risco anterior.
- O campo `activateURL` na config `EmailConfig` pode tornar-se órfão se não houver outro uso.
  Verificar antes de remover para não quebrar outros consumidores.

### Riscos e Mitigações

- **Risco**: token expirado no momento do clique.
  **Mitigação**: o backend rejeita o token ao processar `ATIVAR` no WhatsApp (comportamento atual,
  sem mudança). O usuário recebe mensagem de erro do bot e pode contatar suporte.
- **Risco**: usuário em desktop sem WhatsApp.
  **Mitigação**: `wa.me` no desktop redireciona para WhatsApp Web; se o usuário não tem conta,
  o produto não é funcional para ele de qualquer forma (limitação do produto, não do fluxo).

## Plano de Implementação

1. Adicionar `botNumber string` ao struct e constructor de `SendActivationEmail`.
2. Construir `WaMeURL` em `Execute()` usando `sanitizeE164(botNumber)` e `clearToken`.
3. Substituir `ActivateURL` por `WaMeURL` em `ActivationTemplateInput`.
4. Atualizar `activation.html.tmpl`: botão → `{{.WaMeURL}}`, remover fallback, adicionar suporte.
5. Atualizar wiring em `module.go`: passar `waCfg.BotNumberE164`, remover `emailCfg.ActivateURL`.
6. Verificar se `emailCfg.ActivateURL` é usado em outro lugar; remover se órfão.
7. Escrever suite de testes para `SendActivationEmail` (novo arquivo).

## Monitoramento e Validação

- Critério de sucesso: suite `SendActivationEmail` passa com cenário que verifica que
  `ActivationTemplateInput.WaMeURL` contém `wa.me/{botNumber}?text=ATIVAR%20{token}`.
- Inspecionar email renderizado manualmente antes de deploy: verificar ausência de URL crua.

## Revisão Futura

Se o produto expandir para outros canais (SMS, email magic link sem WhatsApp), revisar esta
decisão para verificar se o CTA do email deve ser channel-aware. Marco: quando o segundo canal
de ativação for implementado.
