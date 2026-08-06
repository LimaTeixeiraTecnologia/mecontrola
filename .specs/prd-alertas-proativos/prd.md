<!-- spec-version: 1 -->

# PRD: Alertas Proativos MeControla

## Visão Geral

Alertas proativos permitem que o MeControla avise usuários pelo WhatsApp quando houver risco financeiro relevante, orçamento ausente, perda de constância ou oportunidade de retomada de controle. A funcionalidade reduz atraso de reação do usuário sem depender de uma pergunta iniciada por ele.

O Release 1 é deliberadamente conservador: categoria em 80% e 100%, orçamento ausente no início do mês e orçamento não revisado até o terceiro dia. Envio real só ocorre depois de dry-run produtivo e template Meta `APPROVED`; templates `PENDING` bloqueiam envio daquele alerta, sem fallback por texto livre.

Fonte de verdade de refinamento: `docs/refin/2026-08-05-sdd-alertas-proativos.md`.

## Objetivos

- Detectar alertas financeiros elegíveis por regra determinística, sem LLM na decisão de disparo.
- Enviar alertas por WhatsApp apenas quando o usuário tiver canal elegível e template Meta aprovado.
- Rodar dry-run antes de envio real para medir volume, prioridade e supressões sem efeitos colaterais externos.
- Aplicar no máximo um alerta iniciado pelo sistema por usuário por rodada diária.
- Evitar duplicidade por usuário, tipo de alerta, alvo e período.
- Preservar zero regressão nos fluxos atuais de onboarding, WhatsApp inbound, budgets threshold e agente financeiro.

## Histórias de Usuário

- Como usuário com orçamento mensal, quero ser avisado quando uma categoria atingir limite relevante para ajustar próximos gastos antes de perder o controle.
- Como usuário sem orçamento do mês, quero receber um lembrete para cadastrar o orçamento e habilitar acompanhamento automático.
- Como usuário que parou de interagir, quero receber uma retomada simples somente se eu tiver dado opt-in adequado para esse tipo de mensagem.
- Como operador do produto, quero habilitar alertas em dry-run para validar volume e qualidade antes de disparar mensagens reais.
- Como responsável técnico, quero que falhas de Meta, templates pendentes e canais ausentes sejam tratados sem marcar envio como sucesso.

## Funcionalidades Core

- Avaliação de elegibilidade: identifica alertas por usuário, competência, categoria e estado de uso.
- Priorização e supressão: escolhe no máximo um alerta por usuário por rodada diária e registra por que os demais foram suprimidos.
- Deduplicação: impede repetição da mesma condição dentro do período definido.
- Templates Meta: envia mensagens proativas fora da janela WhatsApp somente por templates aprovados.
- Dry-run operacional: avalia alertas sem publicar outbox, marcar envio, chamar Meta ou consumir créditos.
- Quiet hours: impede envio entre 20:00 e 08:00 no timezone do usuário; fallback `America/Sao_Paulo`.
- Opt-in: exige consentimento explícito para alertas classificados como `MARKETING`.
- Follow-up agentivo: respostas curtas como "sim" usam contexto de alerta recente, sem inventar intenção.

## Requisitos Funcionais

- RF-01: O sistema deve avaliar alertas de categoria em 80% e 100% do orçamento planejado no Release 1.
- RF-02: O sistema deve manter alerta de 90% desabilitado até alteração explícita de domínio, constraint e migration PostgreSQL.
- RF-03: O sistema deve avaliar alerta de orçamento ausente no início do mês.
- RF-04: O sistema deve avaliar alerta de orçamento não revisado até o terceiro dia do mês.
- RF-05: O sistema deve permitir motivação semanal e retomada de uso após 3 dias somente quando template `APPROVED` e opt-in `MARKETING` estiverem válidos.
- RF-06: O sistema deve manter risco de abandono, fechamento mensal e meta atingida fora do Release 1.
- RF-07: O sistema deve aplicar no máximo um alerta proativo iniciado pelo sistema por usuário por rodada diária.
- RF-08: O sistema deve registrar supressões por prioridade, frequência, canal ausente, opt-in ausente, quiet hours e template ausente ou não aprovado.
- RF-09: O sistema deve impedir duplicidade por chave de usuário, tipo de alerta, alvo e período.
- RF-10: O sistema deve oferecer modo dry-run que não publique evento de envio, não marque alerta como enviado, não chame Meta e não altere estado de notificação.
- RF-11: O sistema deve enviar mensagens proativas fora da janela WhatsApp apenas usando templates Meta com status `APPROVED`.
- RF-12: O sistema deve preservar envio por texto livre existente dentro dos fluxos atuais sem regressão.
- RF-13: O sistema deve preservar o template de ativação atual e o fluxo de outreach existente.
- RF-14: O sistema deve permitir resposta do usuário ao alerta e rotear o follow-up pelo runtime agentivo existente quando houver contexto recente válido.
- RF-15: O sistema deve expor evidências operacionais de avaliação, supressão, fila, envio e falha com métricas e logs de baixa cardinalidade.
- RF-16: O sistema deve usar timezone do usuário quando disponível e fallback `America/Sao_Paulo`.
- RF-17: O sistema deve bloquear envio em quiet hours entre 20:00 e 08:00.
- RF-18: O sistema deve exigir opt-in explícito antes de enviar templates `MARKETING`.

## Experiência do Usuário

O usuário recebe uma mensagem objetiva no WhatsApp com o motivo do alerta e uma pergunta de continuidade. Quando responder positivamente, o MeControla usa contexto de alerta recente para abrir o fluxo correspondente: detalhamento de categoria, panorama de orçamento, criação de orçamento ou retomada. Se o contexto estiver expirado, o sistema pede esclarecimento em vez de presumir intenção.

## Restrições Técnicas de Alto Nível

- Elegibilidade, prioridade, dedup e supressão não dependem de LLM.
- Envio proativo WhatsApp fora da janela depende de template Meta `APPROVED`.
- Template `PENDING`, `REJECTED`, ausente ou não configurado bloqueia o envio daquele alerta.
- Texto livre não é fallback para proativo fora da janela WhatsApp.
- Nenhum segredo da Meta pode ser registrado em documentação, logs ou erros.
- Rollout começa em dry-run obrigatório.
- Implementação respeita monolito modular Go e fronteiras `infrastructure -> application -> domain`.
- Alterações de domínio Go seguem `go-implementation`, `domain-modeling-production` e `design-patterns-mandatory`.
- Alterações agentivas consomem `internal/platform/{agent,memory,workflow,tool,scorer}` sem recriar primitivos Mastra.

## Fora de Escopo

- Envio real de templates que ainda estejam `PENDING` na Meta.
- Threshold 90% sem migration e alteração explícita de VO/constraint.
- Meta atingida enquanto não houver entidade de meta individual provada no codebase.
- Fechamento mensal e risco de abandono no Release 1.
- Machine learning para churn, otimização automática de horário e experimentos A/B.
- Novo canal além de WhatsApp.
- Reescrever o runtime agentivo ou o kernel de workflow.

## Decisões Fechadas

- Um alerta por usuário por rodada diária no Release 1.
- Release 1 inclui categoria 80%, categoria 100%, orçamento ausente e orçamento não revisado.
- Threshold 90% fica condicionado a migration/modelagem posterior.
- Meta atingida fica fora até existir entidade de meta individual.
- Dry-run é obrigatório antes de envio real.
- Envio real exige template `APPROVED`; sem fallback por texto livre.
- Templates `MARKETING` exigem opt-in explícito.
- Quiet hours fixo: 20:00-08:00 no timezone do usuário, fallback `America/Sao_Paulo`.
