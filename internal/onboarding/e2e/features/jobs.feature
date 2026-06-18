# language: pt
Funcionalidade: Jobs de onboarding

  Cenário: Enviar outreach para token pago elegível
    Dado existe um token pago elegível para outreach via WhatsApp
    Quando o job de outreach é executado
    Então deve ter sido enviado 1 template(s) de outreach

  Cenário: Expirar token pago órfão
    Dado existe um token pago expirado
    Quando o job de expiração de tokens é executado
    Então deve existir um support signal do tipo "orphan_expired_subscription"

  Cenário: Limpar registros antigos de deduplicação e lookup
    Dado existem registros antigos de deduplicação e lookup
    Quando o job de limpeza do onboarding é executado
    Então os registros antigos devem ter sido removidos
