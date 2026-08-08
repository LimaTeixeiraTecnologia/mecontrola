# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

## Visão Geral

O assistente financeiro do mecontrola no WhatsApp processa hoje cada mensagem válida do usuário com um loop de LLM via OpenRouter, sem nenhum teto de uso: um único usuário pode disparar interações ilimitadas no dia, cada uma com até 12 rodadas de ferramenta, gerando custo de provedor imprevisível e sem proteção contra abuso, erro de cliente ou loops de reenvio. Esta funcionalidade estabelece um limite de 30 interações por usuário por dia, com bloqueio antes de qualquer chamada ao LLM e resposta estática clara ao usuário, tornando o custo máximo por usuário calculável e protegendo a operação sem degradar a experiência de quem usa o produto dentro da faixa normal. A funcionalidade é para o negócio (previsibilidade de custo) e para o usuário (transparência sobre a regra quando ele a atinge).

## Objetivos

- Garantir que nenhum usuário dispare mais de 30 interações ao agente por dia, medido pela contagem de Runs em `mecontrola.platform_runs` por `resource_id` e dia em America/Sao_Paulo.
- Garantir custo zero de provedor em mensagem bloqueada: 100% das mensagens acima do limite bloqueadas sem chamada ao OpenRouter e sem abertura de Run, auditável pela métrica de bloqueio e pela ausência de novos registros em `platform_runs`.
- Manter zero regressão funcional: as primeiras 30 interações do dia, o pipeline de áudio, a deduplicação por WAMID e a retomada de workflows suspensos (onboarding e confirmação destrutiva) seguem bit a bit o comportamento atual de produção.
- Manter reversibilidade operacional: limite configurável por variável de ambiente com default 30, permitindo ajuste ou desativação sem novo deploy.
- Métrica chave a acompanhar desde o primeiro deploy: contador de bloqueios por limite, indicando quantos usuários atingem o teto e em que horário, subsidiando decisão futura de calibragem ou de limites por plano.

## Histórias de Usuário

- História origem: `docs/us/US-003-limite-30-interacoes-diarias-assistente.md`, validada pelo script `validar-historias-usuario.py`.
- Persona primária (negócio): como responsável pelo produto mecontrola, quero limitar a 30 as interações diárias por usuário com o assistente, para ter teto determinístico de custo de LLM por usuário por dia.
- Persona secundária (usuário final): como usuário do mecontrola no WhatsApp, quero receber um aviso claro quando atingir o limite diário, informando que o limite é de 30 interações e que renova à meia noite, para entender o bloqueio e saber quando posso voltar a usar.
- Caso de borda coberto: usuário com workflow suspenso (onboarding ou confirmação destrutiva) que atingiu o limite continua conseguindo responder ao fluxo pendente, pois essas retomadas não executam o loop de LLM e não são contadas nem bloqueadas.
- Caso de borda coberto: usuário em dia de instabilidade do provedor consome cota mesmo em Runs que falham após o dispatch, pois o custo já foi incorrido; essa regra é explícita para evitar expectativa de reembolso de cota.

## Funcionalidades Core

- Gate de limite diário no fluxo inbound: antes de despachar a mensagem ao agente, o sistema verifica quantas interações o usuário já consumiu no dia corrente e decide permitir ou bloquear. Importante porque é o ponto único onde o custo ainda pode ser evitado; funciona consultando a contagem persistida e decidindo de forma determinística, sem chamada ao LLM.
- Contagem por usuário e dia: cada interação despachada ao agente é registrada como Run em `mecontrola.platform_runs`, tabela e índice já existentes; a contagem do dia usa essa fonte, sem migration nova. Importante porque elimina estado paralelo a sincronizar e mantém a auditoria atual como fonte de verdade.
- Resposta de bloqueio estática e configurável: ao atingir o limite, o usuário recebe mensagem de texto informando o limite de 30 interações diárias e a renovação à meia noite, com texto configurável por variável de ambiente no padrão `WA_MSG_*` já existente. Importante porque dá transparência sem custo de LLM e sem promessa de funcionalidade inexistente.
- Janela do dia em America/Sao_Paulo: a virada da cota ocorre à meia noite no fuso local, reutilizando a configuração de fuso já injetada no runtime. Importante porque a base de usuários é brasileira e a renovação em UTC geraria confusão às 21h.
- Observabilidade do bloqueio: cada bloqueio incrementa contador de métrica com labels de cardinalidade controlada e gera log estruturado. Importante porque a taxa de bloqueio é o sinal de produto para calibrar o limite.

## Requisitos Funcionais

- RF-01: o sistema deve verificar o limite diário antes de despachar a mensagem ao agente, antes da abertura de Run e antes de qualquer chamada ao OpenRouter.
- RF-02: a contagem deve usar os Runs registrados em `mecontrola.platform_runs` para o `resource_id` do usuário com `started_at` dentro do dia corrente em America/Sao_Paulo, sem criar tabela ou migration nova.
- RF-03: a janela do dia deve ser definida pela meia noite no fuso America/Sao_Paulo, reutilizando a configuração de fuso existente.
- RF-04: a fronteira é inclusiva: a 30ª interação do dia é processada normalmente; somente a partir da 31ª a mensagem é bloqueada.
- RF-05: a mensagem de bloqueio deve ser estática, configurável por variável de ambiente no padrão `WA_MSG_*`, informando o limite de 30 interações diárias e a renovação à meia noite, sem uso de LLM e sem CTA de upgrade, pois não existe plano com cota maior.
- RF-06: a decisão de permitir ou bloquear deve viver na camada de aplicação de `internal/agents`; o consumer de WhatsApp permanece adapter fino, apenas consultando o resolvedor e enviando a resposta.
- RF-07: retomadas de workflows suspensos (onboarding e confirmação destrutiva) e o fluxo de onboarding não devem ser contados nem bloqueados, pois não executam o loop de LLM por mensagem.
- RF-08: o limite deve ser configurável por variável de ambiente com default 30, permitindo ajuste operacional ou desativação sem novo deploy.
- RF-09: cada bloqueio deve incrementar contador de métrica com labels de cardinalidade controlada (sem `user_id`) e gerar log estruturado do evento.
- RF-10: mensagem bloqueada não deve abrir Run, não deve chamar o LLM e não deve incrementar a contagem do dia.
- RF-11: Runs que falharem após o dispatch (erro de LLM, timeout, truncamento) devem consumir cota normalmente, pois o custo já foi incorrido.
- RF-12: indisponibilidade da consulta de contagem deve seguir o fluxo de erro de infraestrutura já existente do consumer, com registro de erro e métrica, sem resposta inventada ao usuário e sem liberação silenciosa acima do limite.
- RF-13: o pipeline de áudio permanece inalterado; áudio transcrito conta como interação no momento do dispatch ao agente, aceitando o custo de transcrição de áudio que cruza o limite.
- RF-14: mensagens duplicadas já tratadas pela deduplicação por WAMID não devem passar pelo gate, preservando a ordem atual de processamento.

## Experiência do Usuário

- Persona: usuário final do mecontrola no WhatsApp, base brasileira, interação por texto e áudio.
- Fluxo principal: o usuário envia mensagens normalmente e recebe respostas do assistente; até a 30ª interação do dia nada muda em relação ao comportamento atual.
- Fluxo de bloqueio: a partir da 31ª mensagem no mesmo dia, o usuário recebe imediatamente a mensagem estática informando o limite de 30 interações diárias e a renovação à meia noite; não há aviso prévio progressivo, decisão confirmada para manter a regra simples.
- Fluxo de exceção: se o usuário tem um workflow suspenso aguardando resposta (por exemplo, confirmação de exclusão), a resposta dele ao fluxo pendente é processada mesmo acima do limite, evitando deixar o usuário travado numa ação que ele já iniciou.
- Acessibilidade e clareza: a mensagem de bloqueio deve ser curta, em português claro, sem jargão técnico e sem prometer funcionalidade inexistente.

## Restrições Técnicas de Alto Nível

- Integração existente: o gate se apoia na tabela `mecontrola.platform_runs` e no consumer `whatsapp_inbound` já em produção; nenhum serviço externo novo é introduzido.
- O OpenRouter não oferece limite por usuário final (limites e erro 429 são por chave de API, conforme [documentação oficial de limites](https://openrouter.ai/docs/api-reference/limits)); o teto de 30 por usuário é obrigatoriamente regra da aplicação.
- Zero regressão é requisito não negociável: nenhum comportamento atual de texto, áudio, deduplicação, onboarding ou confirmação destrutiva pode mudar para usuários dentro da cota.
- Privacidade e cardinalidade: métricas de bloqueio não podem conter `user_id` ou identificadores de conversa, seguindo o padrão de cardinalidade controlada do repositório.
- Performance: a verificação adiciona uma consulta indexada por mensagem inbound; o índice `(resource_id, started_at DESC)` já existe e deve ser a única estrutura de suporte exigida.
- Tecnologia não negociável: monolito Go existente, camadas e padrões de `internal/agents` e `internal/platform`, sem novos frameworks; mensagens estáticas seguem o padrão de configuração `WA_MSG_*`.
- Conformidade com as skills obrigatórias do repositório na implementação: `go-implementation`, `mastra`, `domain-modeling-production` e `design-patterns-mandatory`, incluindo consumer como adapter fino e decisão de limite como regra determinística na camada de aplicação.

## Fora de Escopo

- Limites diferenciados por plano ou assinatura; as estruturas de billing e identity existem, mas a integração é fatia futura.
- Aviso prévio progressivo de cota restante (por exemplo, na 25ª interação), descartado por decisão de produto.
- Enfileiramento de mensagens bloqueadas para processamento no dia seguinte.
- Resposta de bloqueio composta por LLM ou personalizada por usuário.
- CTA de upgrade na mensagem de bloqueio, pois não existe plano com cota maior para oferecer.
- Limitação por minuto ou por hora, e alteração do limite de rodadas de ferramenta do agente.
- Tratamento novo para erro 429 do OpenRouter dentro do loop do agente.
- Painel, endpoint ou comando de consulta da cota restante pelo usuário.
- Reembolso de cota para Runs que falham após o dispatch.

## Suposições e Questões em Aberto

- Não restam questões em aberto: todas as decisões materiais foram confirmadas pelo usuário em rodadas de múltipla escolha durante a história de usuário e este PRD.
- Decisões confirmadas neste ciclo: critério de sucesso primário é o teto por usuário com custo zero em bloqueio; não há aviso prévio progressivo; a mensagem de bloqueio informa limite e renovação à meia noite, sem CTA de upgrade.
- Decisões herdadas da US-003: persona de negócio; interação contada como mensagem inbound válida que dispara o agente; bloqueio com resposta estática sem LLM; limite fixo de 30 para cada usuário com dia em America/Sao_Paulo; falha na consulta de contagem segue o fluxo de erro existente.
- Suposição registrada: a base de usuários é integralmente brasileira, o que justifica a janela única em America/Sao_Paulo; se usuários de outros fusos forem admitidos no futuro, a janela por usuário vira nova discussão de produto.
