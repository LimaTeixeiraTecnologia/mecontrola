# Tarefa 7.0: Contact-point de e-mail + roteamento por severidade + runbooks

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar um contact-point de e-mail (`email-mecontrola`, receiver `email` NATIVO do Grafana via `GF_SMTP_*`) em `deployment/telemetry/grafana/provisioning/alerting/contact-points.yaml` e configurar uma política de roteamento por severidade: severidade alta (página) segue para o Telegram existente e severidade baixa (ticket) para o e-mail — sem remover o Telegram. Garantir que cada alerta que pagina humano referencia um runbook nas annotations. Cobre RF-09 e RF-10. Depende da Tarefa 1.0.

<requirements>
- Adicionar o contact-point `email-mecontrola` (receiver `email` nativo do Grafana, configurado via `GF_SMTP_*`) em `contact-points.yaml`, sem código na aplicação nem endpoint novo.
- Configurar política de roteamento por severidade: severidade alta (página) → Telegram existente; severidade baixa (ticket) → e-mail.
- NÃO remover o contact-point Telegram existente.
- Garantir que cada alerta que pagina humano referencia um runbook (em `docs/runbooks/` ou `deployment/runbooks/`) nas annotations `description`, seguindo o padrão já usado.
- Não regredir roteamento ou alertas válidos existentes (RF-20).
</requirements>

## Subtarefas

- [ ] 7.1 Adicionar o contact-point `email-mecontrola` (receiver `email` nativo via `GF_SMTP_*`) ao bloco `contactPoints[].receivers[]` de `contact-points.yaml`.
- [ ] 7.2 Configurar `policies[].routes[]` com `matchers` por `severity`: alta → Telegram, baixa → e-mail; preservar o Telegram como rota/contact-point existente.
- [ ] 7.3 Auditar cada alerta que pagina humano e garantir referência a um runbook (`docs/runbooks/` ou `deployment/runbooks/`) nas annotations; criar/apontar runbooks faltantes.
- [ ] 7.4 Validar o provisionamento (Grafana carrega `contact-points.yaml` sem erro).

## Detalhes de Implementação

Ver techspec.md, seção "Pontos de Integração" (receiver `email` NATIVO do Grafana via `GF_SMTP_*`, provisionado como `email-mecontrola`; webhook para o Resend descartado por acoplamento; schema de `contact-points.yaml` com `contactPoints[].receivers[]` + `policies[].routes[]` por `severity`), "Sequenciamento de Desenvolvimento → Ordem de Build" (item 5, e-mail + roteamento; item 1 = Tarefa 1.0) e "Dependências Técnicas" (SMTP configurado no Grafana via `GF_SMTP_*`). RF-09 (split por severidade sem remover Telegram) e RF-10 (todo alerta de página referencia runbook) no prd.md.

## Critérios de Sucesso

- `contact-points.yaml` contém o contact-point `email-mecontrola` (receiver `email` nativo) e a política de roteamento por severidade.
- Página de teste (severidade alta) roteia ao Telegram; ticket de teste (severidade baixa) roteia ao e-mail.
- O Telegram existente permanece intacto.
- Todo alerta que pagina humano referencia um runbook nas annotations.
- Grafana carrega `contact-points.yaml` sem erro de provisionamento.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Validar o provisionamento (Grafana carrega `contact-points.yaml` sem erro) e que uma página de teste roteia ao Telegram e um ticket de teste roteia ao e-mail.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/telemetry/grafana/provisioning/alerting/contact-points.yaml` — contact-point `email-mecontrola` + roteamento por severidade.
- `docs/runbooks/` — runbooks referenciados pelos alertas que paginam.
