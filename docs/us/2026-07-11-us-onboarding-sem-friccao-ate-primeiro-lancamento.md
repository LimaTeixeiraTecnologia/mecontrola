# US-001: Onboarding sem fricção até o primeiro lançamento financeiro

## Declaração
Como cliente pagante recém-ativado no WhatsApp, quero receber o onboarding completo sem mensagens artificiais, cadastrar ou reutilizar meu 💳 sem loop e registrar meu primeiro lançamento financeiro, para sair da ativação com orçamento, 💳 e transações funcionando em produção.

## Contexto
- Problema: o onboarding de produção exigiu uma mensagem "Oi" para sair da saudação inicial, exibiu a explicação das categorias em texto corrido, não cadastrou nenhum 💳 apesar de três respostas com banco e vencimento, e depois não salvou transações do usuário. A evidência de produção do usuário `3819be07-8877-4efe-8944-807908f3681e` mostra onboarding concluído, orçamento criado, `0` cartões ativos, `0` transações ativas e um `pending-entry` suspenso para despesa pix que recebeu mensagem pedindo 💳.
- Resultado esperado: o usuário recebe a saudação e a pergunta de objetivo na mesma primeira resposta, entende as 5 categorias com emojis e descrições curtas, consegue cadastrar ou reutilizar 💳 sem loop, e consegue persistir receita/despesa após a confirmação necessária, sem pergunta de 💳 quando o pagamento é pix.
- Fonte: solicitação do usuário em 2026-07-11, conversa real de produção via WhatsApp, consulta SSH somente leitura em `root@187.77.45.48`, Prometheus/Tempo em produção e confronto com a base de código local.

## Regras de Negócio
- RN-01: A primeira resposta enviada após "Ativar o meu plano" deve combinar, em uma única mensagem outbound, a saudação e a pergunta de objetivo financeiro; o usuário não deve precisar enviar "Oi" para receber a pergunta de objetivo.
- RN-02: A primeira mensagem do onboarding deve usar exatamente este conteúdo, preservando quebras de linha e emojis permitidos:

```text
🎉 Bem-vindo ao MeControla! 🎉

Estou aqui para te ajudar a organizar suas finanças e conquistar seus objetivos. 💪💰

Vamos começar? Qual é o seu principal objetivo financeiro para este mês?
(por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva; se quiser, já pode me contar o valor da meta, tipo "comprar uma casa, meta de R$ 400.000,00")
```

- RN-03: A mensagem de orçamento mensal deve apresentar as 5 categorias em linhas separadas, com o emoji de cada categoria e uma breve descrição antes de perguntar o orçamento.
- RN-04: A mensagem de orçamento mensal deve usar este conteúdo funcional:

```text
📊 Antes de montar seu planejamento, deixa eu te mostrar como organizamos o dinheiro por aqui.

O dinheiro vive em apenas 5 categorias:

💰 Custo Fixo: contas essenciais e compromissos recorrentes, como moradia, mercado, transporte e saúde.
🎓 Conhecimento: cursos, livros, mentorias e aprendizados que aumentam sua capacidade de ganhar ou administrar dinheiro.
🎉 Prazeres: lazer, restaurantes, viagens, presentes e escolhas que trazem qualidade de vida.
🎯 Metas: objetivos com prazo e valor definidos, como quitar dívida, comprar algo importante ou montar uma reserva específica.
🏦 Liberdade Financeira: reserva de emergência, investimentos e aportes para independência financeira.

Qual é o seu orçamento mensal? (por exemplo: R$ 3.500,00)
```

- RN-05: Toda mensagem, critério, teste e copy funcional que falar de cartão deve usar o emoji `💳`; não deve usar outro emoji para cartão.
- RN-06: Quando o usuário já tiver pelo menos um 💳 ativo cadastrado, o onboarding deve reconhecer que há 💳 existente e perguntar explicitamente se deseja cadastrar OUTRO 💳; se a resposta for negativa, o fluxo deve prosseguir usando os cartões existentes para lançamentos futuros.
- RN-07: Quando o usuário não tiver 💳 ativo e responder com banco/apelido e vencimento em formato natural, como "Santander, vencimento dia 1", "Nubank, vencimento dia 1" ou "XP, vencimento dia 1", o onboarding deve criar o 💳 com o banco/apelido informado e dia de vencimento entre 1 e 31, sem repetir a mesma pergunta em loop.
- RN-08: Após criar um 💳 válido, o onboarding pode perguntar se o usuário deseja cadastrar OUTRO 💳; se o usuário responder "não", o fluxo conclui a etapa de 💳 e segue para a conclusão, preservando o 💳 criado.
- RN-09: Se o usuário informar pagamento `pix`, `dinheiro`, `boleto`, `ted`, `débito` ou outra forma de pagamento que não seja `credit_card`, o fluxo de lançamento financeiro não pode pedir 💳.
- RN-10: Para uma despesa pix com valor, descrição, categoria resolvida e data resolvida, o fluxo deve pedir confirmação do lançamento e, após confirmação positiva, persistir uma linha em `mecontrola.transactions` com `payment_method` correspondente a pix, `origin_wamid`, `origin_operation` e categoria decidida.
- RN-11: Para uma receita simples em mensagem única, como "Recebi R$ 13.874,40 de salário", o agente não pode classificá-la como múltiplos lançamentos; deve registrar a receita ou iniciar a pendência mínima necessária para confirmação, preservando o termo literal "salário".
- RN-12: A conclusão do onboarding só pode afirmar que registra gastos e receitas do dia a dia se o caminho de primeiro lançamento estiver funcional e testado para receita e despesa pix.
- RN-13: Os ajustes devem preservar o workflow durável e auditável: `workflow_runs`, `workflow_steps`, `platform_runs`, `platform_messages`, `outbox_events` e métricas existentes devem continuar sendo gravados.

## Critérios de Aceite
```gherkin
Cenário: Primeira resposta já inclui boas-vindas e pergunta de objetivo
  Dado um cliente pagante ativo sem working memory de objetivo financeiro
  Quando o evento de ativação inicia o onboarding pelo WhatsApp
  Então o cliente recebe uma única mensagem com "🎉 Bem-vindo ao MeControla! 🎉"
  E a mesma mensagem contém "Vamos começar? Qual é o seu principal objetivo financeiro para este mês?"
  E nenhuma mensagem "Oi" do cliente é necessária para avançar da saudação para a pergunta de objetivo
  E a mensagem fica registrada em platform_messages como uma única resposta do assistente

Cenário: Categorias são explicadas com emojis e descrições antes do orçamento
  Dado um cliente que informou objetivo financeiro válido no onboarding
  Quando o onboarding solicitar o orçamento mensal
  Então a mensagem começa com "📊 Antes de montar seu planejamento"
  E lista em linhas separadas "💰 Custo Fixo", "🎓 Conhecimento", "🎉 Prazeres", "🎯 Metas" e "🏦 Liberdade Financeira"
  E cada categoria tem uma descrição curta verificável
  E a pergunta "Qual é o seu orçamento mensal?" aparece depois da lista e das descrições

Cenário: Usuário sem 💳 cadastra um 💳 em linguagem natural sem loop
  Dado um cliente sem registros ativos em cards
  E o onboarding está na etapa de 💳
  Quando o cliente responde "Santander, vencimento dia 1"
  Então o sistema cria um 💳 ativo para o cliente com banco ou apelido Santander e due_day igual a 1
  E a etapa não repete a mensagem genérica de dados do 💳 para a mesma resposta válida
  E o cliente recebe uma pergunta de continuidade usando o texto "Deseja cadastrar OUTRO 💳?"

Cenário: Usuário com 💳 existente não é obrigado a cadastrar novo 💳
  Dado um cliente com pelo menos um 💳 ativo cadastrado
  E o onboarding está na etapa de 💳
  Quando a etapa inicia
  Então o sistema informa que já existe 💳 cadastrado
  E pergunta se o cliente deseja cadastrar OUTRO 💳
  E quando o cliente responde "não"
  Então nenhum 💳 duplicado é criado
  E o onboarding segue para a conclusão usando os cartões já existentes como disponíveis para lançamentos futuros

Cenário: Despesa pix não pede 💳 e persiste após confirmação
  Dado um cliente onboardado sem 💳 cadastrado
  Quando ele envia "gastei R$ 50,00 no supermercado no pix"
  E informa a data "hoje" quando solicitado
  Então o sistema não pergunta qual 💳 foi utilizado
  E exibe uma confirmação contendo supermercado, R$ 50,00, categoria resolvida e pix
  E quando o cliente confirma com "sim"
  Então uma transação ativa é criada em mecontrola.transactions para o cliente
  E a transação contém amount_cents igual a 5000, description igual a "supermercado", payment_method de pix, category_path compatível com "Custo Fixo > Supermercado" e origin_wamid preenchido

Cenário: Receita simples de salário não vira falso multi-lançamento
  Dado um cliente onboardado
  Quando ele envia "Recebi R$ 13.874,40 de salário"
  Então o sistema não responde que percebeu mais de um lançamento
  E registra a receita ou solicita apenas a confirmação mínima necessária para gravar uma receita única
  E após confirmação positiva existe uma transação ativa de receita com amount_cents igual a 1387440 e descrição literal "salário"

Cenário: Falha de parsing de 💳 não produz sucesso falso nem conclusão silenciosa
  Dado um cliente sem 💳 cadastrado
  E o onboarding está na etapa de 💳
  Quando o cliente envia uma resposta realmente incompleta para cadastro de 💳, sem banco/apelido ou sem vencimento
  Então nenhum 💳 parcial é criado
  E o sistema explica exatamente qual dado falta usando 💳
  E o workflow permanece suspenso na etapa de 💳 sem marcar cardsDone como verdadeiro
```

## Dados e Permissões
- Dados obrigatórios do usuário afetado em produção: `id=3819be07-8877-4efe-8944-807908f3681e`, `whatsapp_number=+5511986896322`, `email=jailton.junior94@outlook.com`, `status=ACTIVE`, `created_at=2026-07-11 09:44:03 -0300`.
- Dados obrigatórios para 💳: `user_id`, `nickname`, `bank`, `due_day`; `closing_day` pode continuar sendo derivado pelo caso de uso existente quando não informado.
- Dados obrigatórios para lançamento: `user_id`, direção, forma de pagamento, valor em centavos, descrição literal, categoria ou pendência de categoria, data de ocorrência, `origin_wamid` e operação de origem.
- Perfis/permissões: apenas cliente ativo e autenticado pelo inbound WhatsApp do próprio `user_id`; o fluxo deve continuar passando pela identidade inbound e pelo principal de aplicação já existentes.

## Dependências
- Workflow de onboarding durável em `internal/agents/application/workflows/onboarding_workflow.go`, incluindo fases `welcome`, `goal`, `monthly_budget`, `budget_review`, `activation`, `recurrence`, `cards` e `conclusion`.
- Adapter de 💳 já disponível via `interfaces.CardManager`, com `ListCards` e `CreateCard` chamados pelo onboarding.
- Adapter de transações já disponível via `interfaces.TransactionsLedger`, com `CreateTransaction` delegando para `transactions.CreateTransaction`.
- Consumer WhatsApp já roteia pendências antes do agente geral, então correções de onboarding, 💳 e primeiro lançamento precisam respeitar a ordem `pending-entry`, confirmação destrutiva, criação de 💳, criação de orçamento e onboarding.
- Observabilidade de produção disponível em PostgreSQL, Prometheus e Tempo; os critérios devem permanecer rastreáveis em `workflow_runs`, `workflow_steps`, `platform_runs`, `platform_messages`, `outbox_events` e métricas.

## Fora de Escopo
- Criar novo canal além de WhatsApp.
- Criar novo endpoint HTTP público para onboarding, 💳 ou transações.
- Reprocessar manualmente os dados do usuário de produção como parte desta história; a história define o comportamento correto e os critérios de correção.
- Alterar percentuais padrão da distribuição do orçamento.
- Alterar as categorias canônicas além da copy explicativa solicitada.
- Criar nova política de autorização; a autorização existente por usuário/principal deve ser preservada.

## Evidências
- Entrada:
  - O usuário informou a conversa real em que "Ativar o meu plano" recebeu apenas a saudação, e a pergunta de objetivo só veio depois de "Oi".
  - O usuário solicitou a nova primeira mensagem com saudação e pergunta de objetivo unificadas.
  - O usuário rejeitou a copy compacta de categorias e forneceu a estrutura com `📊`, lista das 5 categorias e descrição breve.
  - O usuário informou que nenhum 💳 foi cadastrado, que houve loop na etapa de 💳, que 💳 existente deve ser usado e que a pergunta deve ser "deseja cadastrar OUTRO cartão?".
  - O usuário informou que nenhuma transação foi salva.
  - O usuário definiu que, ao falar de cartão, deve ser usado o emoji `💳`.
- Base de código:
  - `internal/agents/application/workflows/onboarding_workflow.go:486` define `welcomePrompt` isolado com a saudação.
  - `internal/agents/application/workflows/onboarding_workflow.go:489` define `welcomeGoalPrompt` separado com a pergunta de objetivo.
  - `internal/agents/application/workflows/onboarding_workflow.go:598` suspende `BuildWelcomeStep` na saudação quando `ResumeText` está vazio.
  - `internal/agents/application/workflows/onboarding_workflow.go:609` suspende `BuildGoalStep` na pergunta de objetivo quando `ResumeText` está vazio.
  - `internal/agents/application/workflows/onboarding_workflow.go:496` contém a copy compacta rejeitada para orçamento mensal.
  - `internal/agents/application/workflows/onboarding_workflow.go:51` contém os labels das 5 categorias com emojis.
  - `internal/agents/application/workflows/onboarding_workflow.go:540` já diferencia o prompt de cartões existentes pelo total retornado.
  - `internal/agents/application/workflows/onboarding_workflow.go:705` lista 💳 existentes antes de perguntar pelo cadastro.
  - `internal/agents/application/workflows/onboarding_workflow.go:746` chama `cards.CreateCard` quando o extrator retorna 💳 válido.
  - `internal/agents/application/workflows/onboarding_workflow.go:754` lista novamente e suspende após criar 💳, comportamento hoje coberto como loop desejado em teste.
  - `internal/agents/application/workflows/onboarding_workflow.go:1033` define a sequência atual `welcome -> goal -> monthly_budget -> budget_review -> activation -> recurrence -> cards -> conclusion`.
  - `internal/agents/application/workflows/onboarding_workflow_test.go:722` espera que a primeira entrada suspenda com boas-vindas isolada sem objetivo; esse contrato deve ser atualizado.
  - `internal/agents/application/workflows/onboarding_workflow_test.go:770` espera que a primeira mensagem do goal venha sem preâmbulo de boas-vindas; esse contrato deve ser atualizado.
  - `internal/agents/application/workflows/onboarding_workflow_test.go:1729` espera que 💳 válido crie e re-suspenda perguntando por outro; o novo contrato deve preservar a pergunta por OUTRO 💳 sem loop para respostas válidas.
  - `internal/agents/application/workflows/onboarding_workflow_test.go:1798` considera inválida a resposta "vencimento dia 10, banco Nubank" quando não há nickname; a nova regra precisa aceitar banco como apelido padrão quando só um nome de banco for informado.
  - `internal/agents/application/workflows/pending_entry_decisions.go:34` só pede 💳 quando `paymentMethod == "credit_card"` e não há `CardID`; esse contrato deve ser preservado para pix.
  - `internal/agents/application/workflows/pending_entry_workflow.go:648` monta `RawTransaction` para `ledger.CreateTransaction`.
  - `internal/agents/infrastructure/binding/transactions_ledger_adapter.go:72` delega `CreateTransaction` ao use case de transações e injeta `OriginWamid`.
  - `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:190` define a cadeia de retomada antes do agente geral.
  - `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:286` envia a resposta do onboarding quando o resolver retorna `Handled=true`.
- Produção, banco de dados:
  - Consulta em `mecontrola.users` confirmou o usuário ativo criado em `2026-07-11 12:44:03 UTC`.
  - Consulta em `mecontrola.cards` retornou `cards_count=0` para `user_id=3819be07-8877-4efe-8944-807908f3681e`.
  - Consulta em `mecontrola.transactions` retornou `transactions_count=0` para o mesmo usuário.
  - Consulta em `mecontrola.workflow_runs` mostrou `onboarding-workflow` com `status=succeeded`, `phase=8`, `cardsDone=true`, `goal="Comprar uma casa"`, `goalValueCents=80000000`, `monthlyBudgetCents=1387440` e `recurrence=true`.
  - Consulta em `mecontrola.workflow_steps` mostrou `step-cards` concluído entre `2026-07-11 09:47:18.011 -0300` e `2026-07-11 09:47:18.844 -0300`, apesar de `cards_count=0`.
  - Consulta em `mecontrola.platform_messages` mostrou a saudação enviada às `2026-07-11 09:44:04 -0300`, o usuário enviando "Oi" às `09:44:21 -0300` e a pergunta de objetivo sendo enviada apenas depois do "Oi".
  - Consulta em `mecontrola.platform_messages` mostrou a copy compacta de categorias enviada às `2026-07-11 09:44:38 -0300`.
  - Consulta em `mecontrola.platform_messages` mostrou respostas "Santander, vencimento dia 1", "Nubank, vencimento dia 1" e "XP, vencimento dia 1" seguidas sempre do mesmo reprompt de 💳, sem criação de registro em `cards`.
  - Consulta em `mecontrola.platform_messages` mostrou "Recebi R$ 13.874,40 de salário" recebendo resposta de múltiplos lançamentos, apesar de ser uma receita única.
  - Consulta em `mecontrola.workflow_runs` mostrou `pending-entry` suspenso para a despesa "supermercado", com `amountCents=5000`, `paymentMethod="pix"`, categoria "Custo Fixo > Supermercado", `awaiting=5` e `responseText` de confirmação.
  - Consulta em `mecontrola.platform_messages` mostrou que, após o usuário responder "hoje" para a despesa pix, a mensagem enviada foi "Antes de continuar, preciso saber qual cartão você quer usar", incompatível com `paymentMethod="pix"` e com o estado de confirmação persistido.
  - Consulta em `mecontrola.agents_write_ledger` para o usuário não retornou linhas de escrita de lançamento.
  - Consulta em `mecontrola.outbox_events` no intervalo confirmou eventos inbound e de memória publicados com `status=3` e sem `last_error` para o usuário.
- Produção, métricas e tracing:
  - Prometheus em `mecontrola_otel-lgtm` retornou `onboarding_workflow_total{outcome="started"}=1`, `onboarding_workflow_total{outcome="resumed"}=8` e `onboarding_workflow_total{outcome="completed"}=1` no snapshot consultado.
  - Prometheus retornou `workflow_runs_total{workflow="onboarding-workflow",status="suspended"}=8`, `workflow_runs_total{workflow="onboarding-workflow",status="succeeded"}=1` e `workflow_runs_total{workflow="pending-entry",status="suspended"}=1`.
  - Prometheus retornou `agents_whatsapp_inbound_total{channel="whatsapp",outcome="success"}=9` no snapshot consultado.
  - Prometheus não retornou série para `agents_write_total` no snapshot consultado, consistente com ausência de escrita em `agents_write_ledger` para o usuário.
  - Tempo pesquisado por `user_id=3819be07-8877-4efe-8944-807908f3681e` entre `2026-07-11 09:43:00 -0300` e `2026-07-11 09:49:30 -0300` retornou traces de inbound WhatsApp, incluindo `traceID=915a6f53afa7a18f671d3537dfda42d4` com `rootServiceName=mecontrola-worker`, `rootTraceName=agents.consumer.whatsapp_inbound.handle` e `durationMs=1750`.
  - O trace `915a6f53afa7a18f671d3537dfda42d4` contém spans de retomada de `pending-entry`, `card-create-confirm`, `budget-creation`, execução de agente, chamada LLM via OpenRouter e operações de orçamento, comprovando rastreabilidade operacional no período.
- Inferências:
  - Persona "cliente pagante recém-ativado" deriva do evento de ativação e do usuário ativo em produção.
  - A necessidade de aceitar "Santander, vencimento dia 1" como cadastro de 💳 válido deriva da conversa de produção e do fato de a etapa solicitar apelido, banco e vencimento sem explicar que banco sozinho não serve como apelido.
  - A incompatibilidade entre despesa pix e pergunta de 💳 deriva da combinação entre estado persistido `paymentMethod="pix"` e mensagem real enviada pedindo cartão.
- Não evidenciado:
  - Logs textuais filtrados de `mecontrola_worker` e `mecontrola_server` entre `2026-07-11 09:43:00 -0300` e `2026-07-11 09:49:30 -0300` não retornaram linhas com `3819be07`, telefone, onboarding, pending-entry, card, transaction, erro ou termos da conversa; a trilha útil ficou em banco, métricas e Tempo.
  - Nenhuma transação persistida para o usuário em `mecontrola.transactions` no momento da consulta.
  - Nenhum 💳 ativo persistido para o usuário em `mecontrola.cards` no momento da consulta.

## Notas de Validação
- A história cobre fluxo feliz, alternativo e erro: primeira mensagem e categorias, cadastro/reuso de 💳, lançamento pix, receita simples e falha real de parsing de 💳.
- A história diferencia fatos de produção, evidências do codebase e inferências; nenhuma afirmação técnica sobre o codebase foi usada sem caminho e linha.
- A história não exige alteração de schema PostgreSQL; as correções previstas ficam em workflow, prompts, parsing, roteamento e testes.
- A validação de implementação deve incluir testes unitários do onboarding, testes de pending-entry para pix sem 💳, testes de agente para receita simples, testes de integração do consumer WhatsApp e verificação de persistência em `transactions`.
