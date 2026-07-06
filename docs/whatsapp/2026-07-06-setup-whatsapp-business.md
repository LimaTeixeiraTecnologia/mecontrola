# WhatsApp Business — Setup e Próximos Passos

Data: 2026-07-06
Número: +55 11 93621-2870
Phone Number ID: 1223374060850702
WABA ID: 1678013276765840
App ID: 993579623250676

---

## Status Atual (confirmado via API)

| Item | Status |
|---|---|
| `account_mode` | LIVE |
| `quality_rating` | GREEN |
| `verified_name` | MeControla |
| `name_status` | PENDING_REVIEW |
| `is_official_business_account` | false |
| Verificação de empresa | Concluída |
| Foto de perfil | Definida via API (2026-07-06) |
| Template `mecontrola_ativacao` | PENDING (submetido 2026-07-06) |
| Template ID | 976808612134618 |
| Selo verde (OBA) | Não concedido |

---

## O que foi feito nesta sessão

### Foto de perfil (concluído)

Fonte: logo oficial de `https://www.mecontrola.app.br/images/logo-icon.png`

Arquivo gerado: `mecontrola-landingpage/logo whatsapp oficial/whatsapp-profile-oficial.png` (640x640px)

Upload via API (3 etapas):

1. Criar sessão de upload:
```bash
curl -X POST "https://graph.facebook.com/v20.0/app/uploads" \
  -H "Authorization: Bearer $META_ACCESS_TOKEN" \
  -d "file_length=FILE_SIZE&file_type=image/png"
# retorna: {"id": "upload:..."}
```

2. Enviar o arquivo:
```bash
curl -X POST "https://graph.facebook.com/v20.0/{upload-session-id}" \
  -H "Authorization: OAuth $META_ACCESS_TOKEN" \
  -H "file_offset: 0" \
  -H "Content-Type: image/png" \
  --data-binary "@whatsapp-profile-oficial.png"
# retorna: {"h": "handle..."}
```

3. Definir no perfil:
```bash
curl -X POST "https://graph.facebook.com/v20.0/$META_PHONE_NUMBER_ID/whatsapp_business_profile" \
  -H "Authorization: Bearer $META_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"messaging_product":"whatsapp","profile_picture_handle":"{handle}"}'
# retorna: {"success":true}
```

---

### Template de mensagem (pendente aprovação)

O template `activation_reminder` original não existia. Tentativa inicial com esse nome foi rejeitada por `INCORRECT_CATEGORY` (conteúdo de ativação classificado como autenticação pela Meta).

Template deletado e recriado com novo nome para evitar conflito de propagação de deleção no backend da Meta.

Template submetido:

```bash
curl -X POST "https://graph.facebook.com/v20.0/$META_WABA_ID/message_templates" \
  -H "Authorization: Bearer $META_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mecontrola_ativacao",
    "language": "pt_BR",
    "category": "UTILITY",
    "components": [
      {
        "type": "BODY",
        "text": "Olá! Você ainda não concluiu sua configuração no Me Controla. Acesse com o link {{1}} para continuar de onde parou.",
        "example": {
          "body_text": [["ATIVAR abc123"]]
        }
      }
    ]
  }'
# retornou: {"id":"976808612134618","status":"PENDING","category":"UTILITY"}
```

Observação: o código em `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go:26` envia o parâmetro `"ATIVAR " + token` como `{{1}}` do body. O template é compatível com o código atual sem nenhuma mudança.

---

## Próximos Passos

### 1. Verificar aprovação do template (~24h)

```bash
curl -s "https://graph.facebook.com/v20.0/976808612134618?fields=name,status,rejected_reason&access_token=$META_ACCESS_TOKEN"
```

Possíveis status:
- `APPROVED` → prosseguir para o passo 2
- `REJECTED` → verificar `rejected_reason` e recriar com ajuste no texto
- `PENDING` → aguardar mais

### 2. Atualizar .env em produção após aprovação

No servidor (`/opt/mecontrola/.env`):

```bash
META_OUTREACH_TEMPLATE_NAME=mecontrola_ativacao
ONBOARDING_OUTREACH_ENABLED=true
```

Reiniciar o worker após a alteração:
```bash
docker service update --force mecontrola_worker
```

### 3. Aguardar aprovação do nome de exibição

`name_status: PENDING_REVIEW` — a Meta revisa o nome "MeControla". Sem ação necessária.

Verificar:
```bash
curl -s "https://graph.facebook.com/v20.0/$META_PHONE_NUMBER_ID?fields=name_status,verified_name&access_token=$META_ACCESS_TOKEN"
```

### 4. Solicitar Selo Verde (OBA)

Pré-requisitos para solicitar:
- App LIVE: concluído
- Verificação de empresa: concluída
- Template aprovado: pendente (passo 1)
- Nome aprovado: pendente (passo 3)

Após os pré-requisitos:
1. Acessar `business.facebook.com/help`
2. Categoria: WhatsApp Business
3. Solicitar: "Request Official Business Account"
4. Submeter: link do site (`mecontrola.app.br`), redes sociais, CNPJ

Observação: a Meta pode negar na primeira tentativa para marcas novas. Reenviar após crescimento de volume de mensagens.

---

## Referências de Código

| Arquivo | Função |
|---|---|
| `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go` | `SendActivationTemplate` — envia o template com o token como `{{1}}` |
| `internal/onboarding/application/usecases/send_outreach.go` | `Execute` — usa `templateName` do config para envio de outreach |
| `internal/platform/notification/adapters/whatsapp.go` | `SendTemplate` — adapter fino entre gateway e client Meta |

## Variáveis de Ambiente Relevantes

```
META_PHONE_NUMBER_ID=1223374060850702
META_WABA_ID=1678013276765840
META_ACCESS_TOKEN=<ver .env>
META_OUTREACH_TEMPLATE_NAME=mecontrola_ativacao   # atualizar após aprovação
ONBOARDING_OUTREACH_ENABLED=false                  # habilitar após aprovação
```

## Assets

```
mecontrola-landingpage/logo whatsapp oficial/
├── logo-icon-original.png      → logo oficial (328x318, baixado do site)
├── logo-wordmark-original.png  → wordmark oficial (baixado do site)
└── whatsapp-profile-oficial.png → 640x640px, aplicado no perfil via API
```
