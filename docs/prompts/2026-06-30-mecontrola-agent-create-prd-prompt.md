# Prompt Original

Criar um prompt que sirva como input direto para a skill `create-prd` com o objetivo de substituir integralmente o módulo `internal/agents` atual, removendo toda e qualquer referência ao agente weather e a `internal/agents/application/agents/agent.go`, e definir o produto `MeControlaAgent`, um agente financeiro conversacional no WhatsApp. O novo agente deve usar como referência estrutural o weather-agent e a skill `mastra`, considerar `message history`, `working memory` para objetivo financeiro do usuário, onboarding obrigatório passo a passo, e consumir obrigatoriamente capacidades existentes de `internal/budgets`, `internal/card`, `internal/categories` e `internal/transactions`.

# Prompt Enriquecido

Use a skill `$create-prd` para criar um PRD completo, pronto para implementação futura, sem implementar código agora.

Objetivo do PRD:
Definir o produto e o escopo funcional do novo `MeControlaAgent`, que substituirá integralmente o agente atual de `internal/agents`.

Contexto mandatário do repositório a considerar antes de redigir:
- O módulo consumidor atual de agentes fica em `internal/agents` e hoje está ancorado no exemplo weather.
- O agente atual usa `internal/agents/application/agents/agent.go` como ponto central do weather-agent. Toda e qualquer referência ao weather-agent e a esse arquivo deve ser tratada como legado a ser removido na futura implementação.
- Interprete o pedido como uma substituição completa do módulo `internal/agents` por um novo consumidor sucessor chamado `MeControlaAgent`, e não como convivência entre weather-agent e MeControlaAgent.
- Use o weather-agent apenas como molde estrutural de referência do consumidor Mastra-equivalente, nunca como feature a preservar.
- Considere a arquitetura real do substrato agentivo em:
  - `internal/platform/agent`
  - `internal/platform/llm`
  - `internal/platform/memory`
  - `internal/platform/workflow`
- Considere também os pontos reais do consumidor atual:
  - `internal/agents/application/agents`
  - `internal/agents/application/scorers`
  - `internal/agents/application/workflows`
  - `internal/agents/module.go`

Capacidades reais já existentes no workspace que devem orientar o PRD:
- `internal/categories`
  - há casos de uso e rotas para listar categorias, obter categoria por id, listar dicionário e buscar no dicionário
  - rotas HTTP atuais: `/api/v1/categories`, `/api/v1/categories/{id}`, `/api/v1/category-dictionary`, `/api/v1/category-dictionary/search`
- `internal/card`
  - há casos de uso e rotas para criar, listar, obter, atualizar, atualizar limite, remover cartão e consultar faturas
  - rotas HTTP atuais em `/api/v1/cards`
- `internal/budgets`
  - há casos de uso e rotas para criar orçamento, ativar orçamento, criar recorrência, registrar/editar/remover despesas derivadas, listar alertas e obter resumo mensal
  - rotas HTTP atuais em `/api/v1/budgets`
- `internal/transactions`
  - há casos de uso e rotas para transações, compras no cartão, templates recorrentes, resumo mensal e listagem de lançamentos do mês
  - rotas HTTP atuais em `/api/v1/transactions`, `/api/v1/card-purchases`, `/api/v1/recurring-templates` e `/api/v1/months/{ref_month}`

Diretriz obrigatória de produto:
- O PRD deve deixar explícito que `MeControlaAgent` é um agente financeiro conversacional dentro do WhatsApp.
- O PRD deve deixar explícito que o produto não é aplicativo bancário, sistema contábil, plataforma de investimentos, ERP financeiro, planilha financeira ou ferramenta para especialistas.
- O valor central vendido é realização de objetivos; dinheiro é meio, objetivo é destino.
- A promessa central é: "Seu dinheiro organizado sem planilhas, sem complicação e direto no WhatsApp."

Persona principal:
- Homens e mulheres de 20 a 45 anos
- sentem que o dinheiro desaparece durante o mês
- não mantêm controle financeiro consistente
- abandonam planilhas e apps complexos
- possuem objetivos financeiros, mas não conseguem acompanhá-los

Dores obrigatórias a refletir no PRD:
- falta de clareza sobre para onde o dinheiro vai
- falta de controle sobre gastos
- falta de organização sustentável
- falta de realização dos objetivos financeiros

Objetivos principais do usuário:
- quitar dívidas
- fazer viagem
- comprar casa
- comprar carro
- construir reserva financeira
- organizar a vida financeira

Identidade do agente:
- parceiro financeiro
- simples
- claro
- próximo
- confiável
- motivador
- nunca julga
- nunca culpa
- nunca usa linguagem bancária, jurídica, agressiva ou fria

Tom de voz obrigatório:
- simples
- direto
- amigável
- leve
- motivacional
- profissional

Regras obrigatórias de comunicação:
- uma pergunta por vez
- perguntar apenas o que falta
- nunca pedir dado já fornecido
- priorizar ação em vez de interrogatório
- aceitar linguagem natural, sem comandos rígidos
- organizar respostas com boa clareza visual e uso moderado dos emojis oficiais

Emojis oficiais permitidos:
- 🎯 objetivo
- 💰 dinheiro
- 💳 cartão
- 📊 planejamento
- 📈 receita
- 📉 despesa
- ✅ sucesso
- ⚠️ atenção
- 🚨 alerta crítico
- 🔍 busca
- 🗑️ exclusão
- ✏️ edição
- 🎓 conhecimento
- 🎉 prazeres
- 🏦 liberdade financeira

Onboarding obrigatório e inegociável a refletir nos requisitos funcionais e na experiência do usuário:
1. boas-vindas
2. definição do objetivo financeiro
3. definição do orçamento mensal
4. cadastro ou recusa consciente de cartões
5. apresentação da metodologia das 5 categorias
6. distribuição monetária categoria por categoria
7. resumo final com confirmação ou ajuste
8. conclusão com exemplos de uso diário

Resultados obrigatórios ao final do onboarding:
- objetivo financeiro definido
- orçamento mensal definido
- cartão ou cartões cadastrados, ou ausência conscientemente confirmada
- distribuição financeira criada por categoria
- planejamento consolidado e confirmado

Categorias obrigatórias da metodologia do produto:
- 💰 Custo Fixo
- 🎓 Conhecimento
- 🎉 Prazeres
- 🎯 Metas
- 🏦 Liberdade Financeira

Regras inegociáveis do onboarding:
- nunca pular etapas
- nunca solicitar tudo de uma vez
- nunca pressionar o usuário
- sempre explicar a etapa atual
- sempre mostrar progresso
- sempre reforçar benefícios
- se o usuário tiver dúvida, responder e voltar exatamente para a etapa em andamento
- nunca reiniciar o onboarding por causa de dúvida no meio do fluxo

Operação diária obrigatória a refletir no PRD:
- registrar receitas por linguagem natural
- registrar despesas completas em linguagem natural
- quando faltar meio de pagamento, perguntar apenas isso
- suportar compra em cartão
- suportar compra parcelada com impacto automático nas competências futuras
- responder consultas de resumo mensal e planejamento
- permitir remoção/edição de lançamentos quando o usuário pedir algo como "Apaga aquele Uber"

Restrições funcionais obrigatórias entre módulos:
- working memory deve persistir o objetivo financeiro do usuário como memória de longo prazo
- a jornada deve usar `message history`; avalie e proponha no PRD uma janela de histórico suficiente para conversa fluida, robusta e coerente, justificando a escolha como requisito de produto ou restrição de alto nível
- o PRD deve exigir persistência robusta e eficiente, sem perder o objetivo do cliente
- ferramentas do futuro agente devem ser derivadas exclusivamente dos módulos reais existentes
- categorias devem usar `internal/categories`
- criação e consulta de orçamento devem usar `internal/budgets`
- listagem e criação de cartões devem usar `internal/card`
- lançamentos e consultas financeiras devem usar `internal/transactions`
- `internal/budgets` deve depender conceitualmente de categorias de `internal/categories`
- `internal/transactions` deve depender conceitualmente de categorias de `internal/categories`

Diretrizes obrigatórias para o documento:
- Produza um PRD, não uma especificação técnica e não um plano de implementação.
- Foque em problema, objetivo, escopo, comportamento esperado, jornada, requisitos funcionais e restrições de alto nível.
- Não desenhe APIs, structs, tabelas, interfaces Go, wiring, migrations ou detalhes de código.
- Quando precisar mencionar o repositório, trate os paths e módulos como restrições e integrações de alto nível.
- Numere os requisitos no formato `RF-01`, `RF-02`, etc.
- Inclua claramente:
  - visão geral
  - objetivos
  - histórias de usuário
  - funcionalidades core
  - requisitos funcionais
  - experiência do usuário
  - restrições técnicas de alto nível
  - fora de escopo
  - suposições e questões em aberto

Critérios de aceitação para o PRD:
- deve deixar inequívoco que o weather-agent será completamente substituído por `MeControlaAgent`
- deve refletir a jornada de onboarding obrigatória sem flexibilização
- deve refletir a operação diária de receitas, despesas, cartão, parcelamento e consultas
- deve refletir o uso de WhatsApp como canal primário
- deve refletir memory/history como capacidades essenciais do produto
- deve refletir as integrações obrigatórias com `budgets`, `card`, `categories` e `transactions`
- deve separar claramente o que é escopo e o que é fora de escopo
- deve ser suficientemente concreto para virar `techspec` depois, sem já virar desenho técnico agora

Se você identificar lacunas menores, registre em `Suposições e Questões em Aberto` sem desviar o documento para implementação. Só interrompa para pedir input se encontrar contradição material que impeça definir o produto.
