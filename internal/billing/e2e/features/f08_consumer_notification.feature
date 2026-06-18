# language: pt
Funcionalidade: Billing — consumer NotificationHandler

  Cenário: Handler de past_due recebe envelope válido e retorna nil
    Quando o handler do consumer "billing.subscription.past_due" recebe um envelope válido
    Então o handler retorna nil sem erro

  Cenário: Handler de refunded recebe envelope válido e retorna nil
    Quando o handler do consumer "billing.subscription.refunded" recebe um envelope válido
    Então o handler retorna nil sem erro

  Cenário: Handler de expired_after_grace recebe envelope válido e retorna nil
    Quando o handler do consumer "billing.subscription.expired_after_grace" recebe um envelope válido
    Então o handler retorna nil sem erro

  Cenário: Handler recebe payload inválido e retorna nil sem erro
    Quando o handler do consumer "billing.subscription.past_due" recebe um payload inválido
    Então o handler retorna nil sem erro
