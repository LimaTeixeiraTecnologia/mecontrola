# Documento de Requisitos do Produto (PRD) — Onboarding sem Fricção até o Primeiro Lançamento Financeiro

<!-- spec-version: 1 -->

> Origem: `docs/us/2026-07-11-us-onboarding-sem-friccao-ate-primeiro-lancamento.md` (US única, confrontada com codebase).
> Data: 2026-07-11.
> Persona confirmada: cliente pagante recém-ativado no WhatsApp.
> Decisões confirmadas: `💳` opcional e contextual; primeiro lançamento por pix/dinheiro/boleto/débito/ted/receita não depende de cartão; `💳` só é bloqueante para `credit_card`; não aplicar novo design pattern; preservar workflow durável e auditável existente.

## Visão Geral

O onboarding atual do MeControla no WhatsApp falhou numa jornada real de produção: após a ativação, o cliente precisou enviar "Oi" para sair da saudação inicial, recebeu a explicação das categorias em texto corrido, não conseguiu cadastrar `💳` apesar de responder com banco e vencimento três vezes, concluiu o onboarding com `0` cartões ativos, e depois não teve nenhum lançamento persistido. A trilha de produção do usuário `3819be07-8877-4efe-8944-807908f3681e` mostrou onboarding concluído, orçamento criado, `cardsDone=true`, `0` cartões ativos, `0` transações ativas e um `pending-entry` de despesa pix que recebeu pergunta indevida de `💳`.

Esta funcionalidade fecha a jornada do cliente pagante recém-ativado até o primeiro lançamento financeiro funcional. O usuário deve sair da ativação com objetivo, orçamento e capacidade real de registrar receitas e despesas em produção, sem mensagens artificiais, sem loop de `💳`, sem falso múltiplo lançamento e sem pergunta de cartão para pagamentos que não usam cartão de crédito.

## Objetivos

- Eliminar a fricção inicial: a primeira resposta após "Ativar o meu plano" deve conter boas-vindas e pergunta de objetivo financeiro na mesma mensagem.
- Tornar a explicação das 5 categorias clara e escaneável, com emojis e descrições curtas antes da pergunta de orçamento mensal.
- Permitir cadastrar ou reutilizar `💳` sem loop, tratando cartão como opcional e contextual.
- Garantir que despesas pix e receitas simples consigam chegar à confirmação e persistência real sem dependência indevida de `💳`.
- Impedir falso sucesso: o sistema não pode concluir onboarding ou afirmar registro financeiro quando a capacidade correspondente não estiver funcional e testada.
- Métricas de sucesso:
  - 100% das ativações novas recebem a primeira mensagem combinada sem exigir "Oi".
  - 100% dos prompts de orçamento mensal exibem as 5 categorias canônicas com emoji e descrição curta.
  - 0 loops para respostas válidas de `💳` com banco/apelido e vencimento.
  - 0 perguntas de `💳` para despesa pix, dinheiro, boleto, débito, ted ou receita.
  - 100% dos primeiros lançamentos cobertos nos critérios de aceite persistem uma transação ativa após confirmação positiva.
  - 0 respostas de falso múltiplo lançamento para valor BRL único com separador de milhar, como `R$ 13.874,40`.

## Histórias de Usuário

- Como cliente pagante recém-ativado no WhatsApp, quero receber a saudação e a pergunta de objetivo financeiro na primeira resposta, para começar o onboarding sem enviar mensagem artificial.
- Como cliente pagante recém-ativado no WhatsApp, quero entender as 5 categorias do MeControla com emojis e descrições curtas, para informar meu orçamento mensal com clareza.
- Como cliente pagante recém-ativado no WhatsApp, quero cadastrar um `💳` em linguagem natural ou reutilizar meus cartões existentes, para não ficar preso em loop.
- Como cliente pagante recém-ativado no WhatsApp, quero concluir o onboarding mesmo sem cadastrar `💳`, para registrar despesas pix, dinheiro, boleto, débito, ted e receitas sem bloqueio indevido.
- Como cliente onboardado, quero registrar uma despesa pix e uma receita simples com confirmação antes da escrita, para confiar que o MeControla realmente salvou meu primeiro lançamento.

## Funcionalidades Core

1. **Primeira resposta sem mensagem artificial** — combina boas-vindas e pergunta de objetivo financeiro numa única mensagem outbound, eliminando a necessidade de o usuário enviar "Oi".
2. **Explicação clara das categorias** — apresenta as 5 categorias canônicas em linhas separadas, com emoji e descrição curta, antes de solicitar o orçamento mensal.
3. **Etapa de `💳` opcional e contextual** — reconhece cartões existentes, pergunta se o usuário quer cadastrar outro `💳`, aceita banco/apelido mais vencimento em linguagem natural e permite prosseguir sem cartão quando o usuário recusar.
4. **Primeiro lançamento financeiro funcional** — despesa pix e receita simples entram no fluxo de confirmação e persistem em `transactions` após aceite, com origem e categoria rastreáveis.
5. **Correção de classificação de receita simples** — valor monetário BRL com separador de milhar não pode ser interpretado como múltiplos lançamentos.
6. **Rastreabilidade operacional fim a fim** — onboarding, pendência de lançamento, mensagens, runs, eventos de outbox, métricas e traces continuam auditáveis.

## Requisitos Funcionais

Primeira mensagem e objetivo:
- RF-01: A primeira resposta enviada após "Ativar o meu plano" deve combinar, em uma única mensagem outbound, a saudação e a pergunta de objetivo financeiro.
- RF-02: O usuário não deve precisar enviar "Oi" ou qualquer mensagem artificial para receber a pergunta de objetivo financeiro.
- RF-03: A primeira mensagem do onboarding deve usar exatamente o conteúdo abaixo, preservando quebras de linha e emojis:

```text
🎉 Bem-vindo ao MeControla! 🎉

Estou aqui para te ajudar a organizar suas finanças e conquistar seus objetivos. 💪💰

Vamos começar? Qual é o seu principal objetivo financeiro para este mês?
(por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva; se quiser, já pode me contar o valor da meta, tipo "comprar uma casa, meta de R$ 400.000,00")
```

Categorias e orçamento:
- RF-04: A mensagem de orçamento mensal deve apresentar as 5 categorias canônicas em linhas separadas, com o emoji de cada categoria e uma descrição curta.
- RF-05: A mensagem de orçamento mensal deve perguntar o orçamento apenas depois da explicação das 5 categorias.
- RF-06: A mensagem de orçamento mensal deve usar o conteúdo funcional abaixo:

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

Uso obrigatório do emoji de cartão:
- RF-07: Toda mensagem, critério, teste e copy funcional que falar de cartão deve usar o emoji `💳`.
- RF-08: A funcionalidade não deve introduzir outro emoji para representar cartão.

`💳` opcional e contextual:
- RF-09: Quando o usuário já tiver pelo menos um `💳` ativo, o onboarding deve reconhecer a existência de `💳` cadastrado e perguntar explicitamente se deseja cadastrar OUTRO `💳`.
- RF-10: Quando o usuário com `💳` existente responder negativamente à pergunta de cadastrar outro, nenhum `💳` duplicado deve ser criado e o onboarding deve prosseguir usando os cartões existentes como disponíveis para lançamentos futuros.
- RF-11: Quando o usuário não tiver `💳` ativo e responder com banco/apelido e vencimento em linguagem natural, o onboarding deve criar um `💳` ativo com banco ou apelido informado e `due_day` entre 1 e 31.
- RF-12: Respostas como "Santander, vencimento dia 1", "Nubank, vencimento dia 1" e "XP, vencimento dia 1" devem ser válidas; quando só houver um nome de banco/apelido, esse valor deve poder preencher banco e apelido sem gerar loop.
- RF-13: Após criar um `💳` válido, o onboarding pode perguntar se o usuário deseja cadastrar OUTRO `💳`; se a resposta for negativa, a etapa de `💳` deve ser concluída preservando o cartão criado.
- RF-14: Quando o usuário sem `💳` recusar cadastro, o onboarding deve poder concluir sem cartão ativo, porque `💳` não é requisito para pix, dinheiro, boleto, débito, ted ou receita.
- RF-15: Resposta realmente incompleta para cadastro de `💳`, sem banco/apelido ou sem vencimento, não deve criar cartão parcial; o sistema deve explicar exatamente qual dado falta usando `💳` e manter o workflow suspenso sem marcar `cardsDone=true`.

Primeiro lançamento por despesa pix:
- RF-16: Quando o usuário informar pagamento `pix`, `dinheiro`, `boleto`, `ted`, `débito`, `debit_card`, `debit_in_account`, `cash`, `vale_refeicao` ou `vale_alimentacao`, o fluxo de lançamento financeiro não pode pedir `💳`.
- RF-17: Para uma despesa pix com valor, descrição, categoria resolvida e data resolvida, o sistema deve pedir confirmação do lançamento antes de persistir.
- RF-18: Após confirmação positiva de uma despesa pix, o sistema deve persistir uma transação ativa com direção de despesa, valor em centavos, descrição literal, forma de pagamento pix, categoria decidida, `origin_wamid` e `origin_operation`.
- RF-19: Para despesa pix sem `💳`, a ausência de cartão ativo não deve impedir confirmação nem persistência.

Primeiro lançamento por receita:
- RF-20: Para uma receita simples em mensagem única, como "Recebi R$ 13.874,40 de salário", o agente não pode classificar a mensagem como múltiplos lançamentos.
- RF-21: O termo literal da receita, como "salário", deve ser preservado como descrição sem parafrasear.
- RF-22: Receita simples deve registrar uma única intenção de receita ou iniciar apenas a confirmação mínima necessária para gravar uma receita única.
- RF-23: Após confirmação positiva, deve existir uma transação ativa de receita com valor em centavos correto e descrição literal.

Confirmação e no-false-success:
- RF-24: O sistema não deve afirmar que registra gastos e receitas do dia a dia na conclusão do onboarding enquanto o caminho de primeiro lançamento por receita e despesa pix não estiver funcional e testado.
- RF-25: Nenhuma resposta ao usuário pode afirmar sucesso de cadastro de `💳` ou lançamento financeiro sem retorno real da ferramenta/use case correspondente.
- RF-26: Toda escrita financeira coberta por este PRD deve passar por confirmação humana antes da persistência, preservando o fluxo durável de pendência conversacional.
- RF-27: Reenvio ou retomada da mesma pendência não deve criar escrita duplicada; a origem deve permanecer rastreável por `origin_wamid` e operação.

Roteamento e interoperabilidade:
- RF-28: A cadeia de retomada do WhatsApp deve continuar priorizando pendências antes do agente geral, especialmente `pending-entry`, confirmação destrutiva, criação de `💳`, criação de orçamento e onboarding.
- RF-29: Correções de onboarding, `💳` e primeiro lançamento devem preservar a identidade inbound do usuário ativo e autenticado pelo próprio WhatsApp.
- RF-30: O fluxo não deve criar novo canal, novo endpoint HTTP público nem nova política de autorização.

Auditoria, observabilidade e operação:
- RF-31: Onboarding, pendência de lançamento, mensagens e respostas devem continuar rastreáveis em `workflow_runs`, `workflow_steps`, `platform_runs`, `platform_messages` e `outbox_events`.
- RF-32: Métricas de onboarding, workflow, inbound WhatsApp e escrita financeira devem continuar disponíveis com cardinalidade controlada, sem `user_id`, telefone ou identificador de mensagem como label.
- RF-33: Traces de inbound WhatsApp devem continuar permitindo correlacionar retomada de workflow, execução de agente, chamada LLM, criação de orçamento, criação de `💳` e escrita financeira quando aplicável.
- RF-34: Logs, métricas e traces não devem expor dados sensíveis do cliente como rótulos de alta cardinalidade.

Qualidade e validação:
- RF-35: A implementação deve atualizar testes unitários do onboarding para a primeira mensagem combinada, copy de categorias, cadastro/reuso/recusa de `💳` e falha de parsing de `💳`.
- RF-36: A implementação deve cobrir pending-entry para pix sem `💳`, garantindo que só `credit_card` exige cartão.
- RF-37: A implementação deve cobrir agente financeiro para receita simples com valor BRL usando separador de milhar, impedindo falso múltiplo lançamento.
- RF-38: A implementação deve cobrir integração do consumer WhatsApp para a ordem de retomada e envio de resposta quando onboarding ou pending-entry tratam a mensagem.
- RF-39: A implementação deve incluir verificação de persistência em `transactions` para receita simples e despesa pix confirmadas.

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

Cenário: Usuário sem 💳 pode concluir onboarding se recusar cadastrar cartão
  Dado um cliente sem registros ativos em cards
  E o onboarding está na etapa de 💳
  Quando o cliente responde "não"
  Então nenhum 💳 é criado
  E o onboarding conclui sem marcar a ausência de 💳 como erro
  E a conclusão não bloqueia lançamentos pix, dinheiro, boleto, débito, ted ou receita

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

## Experiência do Usuário

O usuário inicia a ativação e recebe uma primeira resposta completa, sem precisar "acordar" o agente. A sequência deve parecer uma conversa contínua: objetivo financeiro, orçamento mensal, revisão/distribuição, ativação, recorrência, `💳` opcional e conclusão.

Na etapa de `💳`, o tom deve ser explícito: se já houver cartão, perguntar se deseja cadastrar OUTRO `💳`; se não houver, oferecer cadastro sem tornar isso obrigatório para quem não usa cartão de crédito. Respostas naturais com banco/apelido e vencimento devem funcionar de primeira. Quando faltar dado, a mensagem deve indicar exatamente o que falta.

Depois da conclusão, o usuário deve conseguir enviar um lançamento real. Para despesa pix, a conversa deve chegar à confirmação e persistência sem mencionar cartão. Para receita simples, o agente deve tratar o valor BRL como um único valor e preservar a descrição literal.

## Restrições Técnicas de Alto Nível

- Canal alvo: WhatsApp inbound já existente no consumidor `internal/agents`.
- O fluxo deve consumir o substrato agentivo existente em `internal/platform/{agent,memory,workflow,tool,scorer,llm}`; não deve recriar primitvos de Thread, Run, WorkingMemory, PendingStep ou kernel de workflow.
- O workflow durável deve ser preservado; suspend/resume continua sendo a forma de coletar dados do usuário.
- Tools e adapters continuam finos, delegando para use cases e interfaces existentes.
- LLM só pode aparecer nas call-sites sancionadas do consumidor agentivo; o kernel de workflow permanece genérico, sem regra financeira, SQL ou LLM.
- Structured output e schemas de tool devem permanecer estritos, com enums fechados quando houver decisão.
- `💳` é opcional no onboarding e obrigatório apenas para lançamento `credit_card`.
- Não há exigência de migration PostgreSQL neste PRD.
- Métricas devem manter cardinalidade controlada; dados de usuário, telefone, `wamid`, categoria e IDs de entidade não podem virar labels de métrica.
- Go alvo confirmado pelo `go.mod`: `go 1.26.5`.

## Fora de Escopo

- Criar novo canal além de WhatsApp.
- Criar novo endpoint HTTP público para onboarding, `💳` ou transações.
- Reprocessar manualmente os dados do usuário de produção como parte desta funcionalidade.
- Alterar percentuais padrão da distribuição do orçamento.
- Alterar as categorias canônicas além da copy explicativa solicitada.
- Criar nova política de autorização.
- Criar novo bounded context, novo kernel de workflow ou novo design pattern estrutural.
- Tornar `💳` obrigatório para concluir onboarding quando o usuário não pretende usar cartão de crédito.
- Alterar schema PostgreSQL.

## Evidências de Codebase

- `internal/agents/application/workflows/onboarding_workflow.go:486` define `welcomePrompt` apenas com saudação.
- `internal/agents/application/workflows/onboarding_workflow.go:489` define `welcomeGoalPrompt` separado com pergunta de objetivo.
- `internal/agents/application/workflows/onboarding_workflow.go:496` contém a copy compacta de categorias para orçamento mensal.
- `internal/agents/application/workflows/onboarding_workflow.go:540` diferencia prompt de cartões por quantidade existente.
- `internal/agents/application/workflows/onboarding_workflow.go:598` suspende `BuildWelcomeStep` na saudação quando `ResumeText` está vazio.
- `internal/agents/application/workflows/onboarding_workflow.go:609` suspende `BuildGoalStep` na pergunta de objetivo quando `ResumeText` está vazio.
- `internal/agents/application/workflows/onboarding_workflow.go:705` inicia a etapa durável de cartões consultando cartões existentes.
- `internal/agents/application/workflows/onboarding_workflow.go:746` cria `💳` por `cards.CreateCard` quando a extração é válida.
- `internal/agents/application/workflows/onboarding_workflow.go:758` suspende novamente com prompt de cartão depois da criação.
- `internal/agents/application/workflows/onboarding_workflow.go:1022` monta o workflow durável de onboarding por `workflow.Sequence`.
- `internal/agents/application/workflows/pending_entry_decisions.go:34` só exige cartão quando `paymentMethod == credit_card` e não há cartão.
- `internal/agents/application/workflows/pending_entry_workflow.go:624` delega criação para `ledger.CreateTransaction`.
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:190` prioriza retomadas antes do agente geral.
- `internal/agents/application/agents/mecontrola_agent.go:18` já contém regra anti falso múltiplo lançamento para valores BRL com separador de milhar.
- `internal/agents/application/agents/mecontrola_agent.go:91` lista os códigos válidos de forma de pagamento, incluindo `pix` e `credit_card`.

## Evidências de Produção

- Usuário afetado: `3819be07-8877-4efe-8944-807908f3681e`, cliente ativo criado em `2026-07-11 12:44:03 UTC`.
- `mecontrola.cards` retornou `cards_count=0` para o usuário afetado.
- `mecontrola.transactions` retornou `transactions_count=0` para o usuário afetado.
- `workflow_runs` mostrou `onboarding-workflow` com `status=succeeded`, `phase=8`, `cardsDone=true`, `goal="Comprar uma casa"`, `goalValueCents=80000000`, `monthlyBudgetCents=1387440` e `recurrence=true`.
- `workflow_steps` mostrou `step-cards` concluído apesar de `cards_count=0`.
- `platform_messages` mostrou saudação enviada, usuário enviando "Oi" e pergunta de objetivo enviada apenas depois do "Oi".
- `platform_messages` mostrou respostas "Santander, vencimento dia 1", "Nubank, vencimento dia 1" e "XP, vencimento dia 1" seguidas do mesmo reprompt de `💳`, sem criação de registro em `cards`.
- `platform_messages` mostrou "Recebi R$ 13.874,40 de salário" recebendo resposta de múltiplos lançamentos, apesar de ser receita única.
- `workflow_runs` mostrou `pending-entry` suspenso para despesa "supermercado", com `amountCents=5000`, `paymentMethod="pix"`, categoria "Custo Fixo > Supermercado" e resposta de confirmação.
- Após o usuário responder "hoje" para a despesa pix, `platform_messages` mostrou pergunta de cartão incompatível com `paymentMethod="pix"`.
- `agents_write_ledger` não retornou linhas de escrita de lançamento para o usuário.
- Prometheus e Tempo confirmaram rastreabilidade operacional via inbound WhatsApp, workflow de onboarding, `pending-entry`, OpenRouter e operações de orçamento.

## Decisões de Produto e Domínio

- `💳` é opcional e contextual: o usuário pode concluir onboarding sem cartão ativo.
- `💳` só é dependência bloqueante quando o lançamento for `credit_card`.
- Pagamentos pix, dinheiro, boleto, débito, ted, vale-refeição, vale-alimentação e receitas não podem pedir `💳`.
- Resposta válida de cartão em linguagem natural deve usar banco como apelido quando só um nome for informado.
- A conclusão do onboarding só pode prometer registro do dia a dia quando receita simples e despesa pix estiverem cobertas por testes e persistência.
- Gate `design-patterns-mandatory`: decisão determinística de `nao aplicar padrao`; a entrega deve corrigir diretamente workflow, prompts, parsing, roteamento e testes existentes, preservando menor custo e menor indireção.

## Suposições e Questões em Aberto

Nenhuma questão material em aberto.

Decisões fechadas:
- Cartão é opcional/contextual, confirmado pelo solicitante em 2026-07-11.
- Não há PRD existente para este slug, portanto não há drift downstream a aceitar.
- Não há exigência de alteração de schema PostgreSQL.
- O PRD não inclui remediação manual do usuário afetado em produção.
