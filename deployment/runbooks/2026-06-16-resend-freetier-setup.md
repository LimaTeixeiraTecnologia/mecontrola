# Resend Free Tier — Setup de E-mail Transacional

**Domínio:** `mecontrola.app.br`
**Provider:** Resend (https://resend.com)
**Trigger:** e-mail de ativação disparado após pagamento Kiwify confirmado

---

## Pré-requisitos

- Acesso ao painel Cloudflare de `mecontrola.app.br`
- Acesso ao `.env` de produção na VPS Hostinger
- E-mail para cadastro no Resend

---

## 1. Criar conta no Resend

1. Acesse https://resend.com → **Sign Up**
2. Cadastre com e-mail (ex: `devops@mecontrola.app.br`) ou GitHub
3. Confirme o e-mail de verificação
4. Plano selecionado automaticamente: **Free** (3.000 e-mails/mês, 100/dia, 1 domínio)

---

## 2. Adicionar e verificar o domínio

1. No painel Resend: **Domains → Add Domain**
2. Digite `mecontrola.app.br` → **Add**
3. O Resend exibe os records DNS necessários — copie os valores exatos do painel (não use os abaixo como referência estática):

| Tipo | Nome | Valor (exemplo) |
|------|------|-----------------|
| TXT | `@` | `"v=spf1 include:amazonses.com ~all"` |
| CNAME | `resend._domainkey` | `resend._domainkey.resend.com` |
| TXT | `_dmarc` | `"v=DMARC1; p=quarantine; rua=mailto:dmarc@mecontrola.app.br"` |

---

## 3. Adicionar os records no Cloudflare

**Opção rápida (recomendada):** use o botão **Auto configure** no painel Resend → Domains. O Resend se conecta ao Cloudflare via OAuth e adiciona os records automaticamente (DKIM, SPF, MX).

**Opção manual:**
1. Acesse https://dash.cloudflare.com → selecione `mecontrola.app.br`
2. **DNS → Records → Add record**
3. Adicione cada record conforme tabela do passo 2
4. CNAME: **Proxy status = DNS only** (nuvem cinza, não laranja)
5. Salve cada record

> Se já existir TXT `v=spf1` em `@`, mescle o `include:amazonses.com` no valor existente em vez de criar novo.

---

## 4. Verificar o domínio no Resend

1. Painel Resend → **Domains → Verify** ao lado de `mecontrola.app.br`
2. Aguarde 5 a 30 minutos para propagação DNS
3. Status deve mudar para **Verified** (verde)

Se continuar pendente após 30 minutos:

```bash
dig CNAME resend._domainkey.mecontrola.app.br +short
dig TXT mecontrola.app.br +short | grep spf
```

---

## 5. Criar API Key

1. Painel Resend: **API Keys → Create API Key**
2. Nome: `mecontrola-prod`
3. Permission: **Sending access** (Full Access não é necessário)
4. Domain: selecione `mecontrola.app.br`
5. Copie a chave gerada — ela é exibida uma única vez: `re_xxxxxxxxxxxxxxxx`

---

## 6. Configurar variáveis de produção

Na VPS Hostinger, edite o `.env` de produção:

```bash
EMAIL_PROVIDER=resend
RESEND_API_KEY=re_xxxxxxxxxxxxxxxx
RESEND_BASE_URL=https://api.resend.com
EMAIL_FROM_ADDRESS=noreply@mecontrola.app.br
EMAIL_FROM_NAME=MeControla
EMAIL_REPLY_TO=
EMAIL_ACTIVATE_URL=https://mecontrola.app.br/activate
EMAIL_HTTP_TIMEOUT=10s
```

Reinicie os containers:

```bash
docker compose -f deployment/compose/compose.prod.yml up -d --force-recreate
```

---

## 7. Testar o envio

**Opção A — via painel Resend:**
1. Resend → **Emails → Send test email**
2. Use um e-mail real como destinatário
3. Verifique recebimento fora da pasta de spam

**Opção B — via fluxo real:**
1. Dispare um webhook Kiwify de teste (sandbox)
2. Monitore os logs: `docker compose logs -f worker`
3. Confirme a métrica: `onboarding_activation_email_dispatched_total{result="sent"}`
4. Painel Resend → **Emails** deve mostrar status `Delivered`

---

## 8. Verificar DMARC

```bash
dig TXT _dmarc.mecontrola.app.br +short
# Esperado: "v=DMARC1; p=quarantine; ..."
```

---

## Limites do Free Tier

| Limite | Valor |
|--------|-------|
| E-mails/mês | 3.000 |
| E-mails/dia | 100 |
| Domínios | 1 |
| API Keys | ilimitadas |
| Retenção de logs | 1 dia |

Quando o volume de usuários pagantes ultrapassar ~90/dia, migrar para Brevo SMTP ou Resend Pro ($20/mês — 50k e-mails). Para trocar para Brevo basta alterar `EMAIL_PROVIDER=smtp` e as variáveis SMTP correspondentes, sem mudança de código.

---

## Checklist de conclusão

- [ ] Domínio `mecontrola.app.br` com status **Verified** no painel Resend
- [ ] `dig CNAME resend._domainkey.mecontrola.app.br` retorna valor do Resend
- [ ] `RESEND_API_KEY` configurada no `.env` de produção
- [ ] Containers reiniciados sem erro
- [ ] E-mail de teste recebido fora da pasta de spam
- [ ] Métrica `onboarding_activation_email_dispatched_total{result="sent"}` incrementando
