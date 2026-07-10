# US-001: Onboarding com apresentação de categorias, orçamento mensal e cartões

## Declaração
Como cliente nova do MeControla no WhatsApp, quero passar por um onboarding guiado que apresenta o MeControla, coleta minha meta, explica as 5 categorias do orçamento, solicita meu orçamento mensal e cadastra meus cartões um por vez, para começar meu planejamento financeiro com clareza, sem informar renda líquida e sem lacunas no fluxo.

## Contexto
- Problema: o onboarding atual combina boas-vindas com pergunta de objetivo, pergunta "renda mensal líquida", coleta cartão antes de apresentar as categorias e separa a apresentação da abordagem da coleta de orçamento, enquanto o fluxo desejado exige boas-vindas isolada, meta no segundo passo, apresentação das categorias no terceiro passo com passagem imediata, coleta de orçamento mensal, distribuição/ativação do orçamento e depois cadastro de um ou mais cartões.
- Resultado esperado: o workflow durável de onboarding deve seguir a sequência `bem-vindo -> meta/objetivo com valor opcional -> apresentação das categorias -> orçamento mensal -> distribuição/ativação -> cartões em loop`, usando os primitivos existentes de `internal/platform/workflow`, sem recriar o substrato agentivo e sem perguntar renda líquida.
- Fonte: solicitação do usuário nesta conversa, decisões de clarificação `1:A`, `2:C`, `3:A`, `4:A` e confronto com o codebase local em `/Users/jailtonjunior/Git/mecontrola`.

## Regras de Negócio
- O primeiro passo do onboarding deve ser apenas a mensagem de boas-vindas e apresentação do MeControla, sem perguntar meta, orçamento, renda ou cartão na mesma mensagem.
- O segundo passo deve coletar a meta ou objetivo financeiro do usuário, permitindo objetivo com valor monetário ou sem valor monetário.
- O terceiro passo deve apresentar as 5 categorias do orçamento com o texto aprovado pelo usuário e avançar de imediato para a coleta do orçamento mensal, sem exigir confirmação "Faz sentido?".
- A apresentação das categorias deve usar exatamente estas categorias de orçamento: Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira.
- A mensagem de apresentação deve preservar o conteúdo de negócio informado pelo usuário: "Antes de montar seu planejamento, deixa eu te mostrar como organizamos o dinheiro por aqui. Tudo vive em apenas 5 categorias: Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira."
- O onboarding não deve perguntar renda mensal líquida; deve perguntar qual é o orçamento mensal ou valor mensal que o usuário quer planejar.
- O modelo de estado, prompts, erros, resumo e WorkingMemory devem usar a semântica de orçamento mensal, não renda líquida.
- O cadastro de cartões deve ocorrer após a distribuição e ativação do orçamento, cadastrando um cartão por vez e repetindo a pergunta até o usuário informar que não deseja adicionar outro cartão.
- Cada cartão criado no onboarding deve respeitar os dados obrigatórios já praticados pelo codebase: apelido, banco emissor e dia de vencimento entre 1 e 31.
- A distribuição do orçamento deve usar o orçamento mensal como total planejado para sugestão, validação de valores em reais e criação do budget.
- Estados do workflow devem continuar fechados e parseáveis; nenhuma fase nova pode ser representada por string solta.
- A implementação deve preservar o workflow durável com suspensão e retomada por merge-patch; não deve substituir o onboarding por branching solto no agente, handler ou consumer.
- A implementação deve seguir `$go-implementation`, `$mastra`, `$domain-modeling-production` e `$design-patterns-mandatory`; a decisão de pattern é `nao aplicar padrao`, porque a solução direta sobre o workflow existente tem menor custo e menor superfície de falha.

## Critérios de Aceite
```gherkin
Cenário: cliente nova recebe primeiro passo apenas com boas-vindas
  Dado uma cliente nova com assinatura ativa e sem WorkingMemory de objetivo financeiro
  Quando o onboarding for iniciado por evento de ativação ou pelo primeiro inbound WhatsApp
  Então o sistema deve enviar uma mensagem de boas-vindas apresentando o MeControla
  E a mensagem não deve perguntar meta, objetivo, orçamento mensal, renda líquida ou cartão
  E o workflow deve ficar suspenso aguardando a resposta para avançar ao passo de meta

Cenário: segundo passo coleta meta com valor opcional
  Dado uma cliente que recebeu a mensagem de boas-vindas
  Quando a cliente responder que quer começar ou enviar uma intenção equivalente
  Então o sistema deve perguntar qual é a meta ou objetivo financeiro dela
  E deve aceitar resposta com valor monetário explícito
  E deve aceitar resposta sem valor monetário sem bloquear o avanço

Cenário: apresentação das categorias avança de imediato para orçamento mensal
  Dado uma cliente que informou uma meta válida com ou sem valor
  Quando o workflow concluir o passo de meta
  Então o sistema deve apresentar as 5 categorias do orçamento
  E deve incluir Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira
  E deve avançar de imediato para a pergunta de orçamento mensal sem exigir confirmação da apresentação
  E não deve perguntar renda mensal líquida

Cenário: orçamento mensal substitui renda líquida em coleta e validação
  Dado uma cliente que recebeu a apresentação das categorias
  Quando o sistema solicitar o valor mensal para planejamento
  Então a pergunta deve usar a expressão orçamento mensal ou valor mensal planejado
  E não deve usar a expressão renda mensal líquida
  E o valor informado deve ser persistido no estado como total planejado do onboarding

Cenário: valor inválido de orçamento mensal gera reprompt específico
  Dado uma cliente na etapa de orçamento mensal
  Quando ela responder sem valor monetário positivo identificável
  Então o sistema deve pedir novamente o orçamento mensal com exemplo em reais
  E o workflow deve permanecer suspenso na etapa de orçamento mensal
  E nenhum budget, cartão ou distribuição deve ser criado nesse turno

Cenário: distribuição personalizada em reais usa orçamento mensal como total
  Dado uma cliente com orçamento mensal de R$ 5.000,00
  Quando ela informar valores em reais para as cinco categorias
  Então a soma dos valores deve ser validada contra R$ 5.000,00
  E, se a soma fechar o orçamento mensal, o sistema deve converter os valores para basis points
  E, se a soma não fechar o orçamento mensal, o sistema deve pedir correção sem ativar orçamento parcial

Cenário: resumo e ativação acontecem antes do cadastro de cartões
  Dado uma cliente que concluiu meta, categorias, orçamento mensal e distribuição
  Quando o sistema apresentar o resumo antes da ativação
  Então o resumo deve exibir objetivo, orçamento mensal e distribuição por categoria
  E o resumo não deve exibir "renda mensal líquida"
  E a ativação só deve ocorrer após confirmação explícita da cliente no resumo

Cenário: cartões são cadastrados um por vez após ativação do orçamento
  Dado uma cliente que confirmou o resumo e teve o orçamento ativado com sucesso
  Quando o workflow chegar ao cadastro de cartões
  Então o sistema deve perguntar se ela deseja adicionar um cartão de crédito
  E, se ela informar um cartão com apelido, banco e vencimento válido, o sistema deve criar esse cartão
  E, após criar o cartão, deve perguntar se ela deseja adicionar outro cartão
  E deve repetir o ciclo até a cliente responder que não deseja adicionar outro cartão

Cenário: cartão incompleto mantém loop de cartões sem desfazer orçamento ativado
  Dado uma cliente com orçamento já ativado e na etapa de cadastro de cartões
  Quando ela tentar cadastrar um cartão sem apelido, banco ou dia de vencimento válido
  Então o sistema deve pedir somente os dados faltantes ou inválidos do cartão atual
  E não deve criar cartão parcial
  E não deve desfazer, recriar ou alterar o orçamento já ativado

Cenário: cliente recusa cartões e conclui onboarding
  Dado uma cliente com orçamento já ativado e na etapa de cadastro de cartões
  Quando ela responder que não deseja adicionar cartão
  Então o workflow deve marcar a etapa de cartões como concluída
  E deve concluir o onboarding com mensagem final de próximos passos
  E não deve reabrir distribuição, resumo ou ativação de orçamento

Cenário: erro técnico mantém resposta sem falso sucesso
  Dado uma falha ao listar cartões, criar cartão, sugerir alocação, criar budget, ativar budget ou gravar WorkingMemory
  Quando o step afetado executar
  Então o workflow deve retornar erro tipado no step correspondente
  E o sistema não deve afirmar que cartão, orçamento ou onboarding foram concluídos
  E a falha deve ser rastreável por workflow, step, status e erro sanitizado
```

## Dados e Permissões
- Dados obrigatórios: `user_id`, `peer_id`, fase do onboarding, texto de retomada, objetivo financeiro, valor opcional da meta em centavos, orçamento mensal em centavos, lista ou contagem de cartões cadastrados no onboarding, alocações por root slug, resumo final e mensagem final.
- Dados obrigatórios por cartão: apelido, banco emissor e dia de vencimento entre 1 e 31.
- Perfis/permissões: cliente autenticada via WhatsApp só pode conduzir o próprio onboarding e cadastrar cartões na própria conta; workers e consumers operam com credenciais de serviço já existentes.
- Privacidade: mensagens e WorkingMemory não devem expor termos internos como workflow, run, snapshot, correlação, plataforma ou infraestrutura.

## Dependências
- `internal›agents›application›workflows›onboarding_workflow.go` para fases, prompts, extração estruturada, suspensão, criação de budget, ativação e WorkingMemory.
- `internal›agents›application›agents›mecontrola_agent.go` para tom, regras de WhatsApp, emojis, catálogo de ferramentas e instruções do agente financeiro.
- `internal/platform/workflow` para workflow durável, `StepStatusSuspended`, `SuspendAwaitingInput` e retomada por merge-patch.
- `internal/agents/application/interfaces.BudgetPlanner` para `SuggestAllocation`, `CreateBudget`, `ActivateBudget`, `CreateRecurrence` e leitura de summary.
- `internal/agents/application/interfaces.CardManager` para `ListCards` e `CreateCard`.
- `internal/budgets/domain/valueobjects.RootSlug` para taxonomia oficial de Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira.
- OpenRouter via provider LLM já existente para extrações estruturadas no workflow; a feature não deve introduzir outro provider nem fallback chain.

## Fora de Escopo
- Criar novas categorias de orçamento ou alterar slugs canônicos.
- Reescrever `internal/platform/workflow`, `internal/platform/agent`, `internal/platform/memory` ou o runtime Thread -> Run.
- Criar nova tool financeira para onboarding quando o workflow existente puder resolver com menor mudança.
- Permitir cadastro de vários cartões em uma única mensagem.
- Perguntar renda líquida como dado separado do orçamento mensal.
- Alterar billing, assinatura, Kiwify, regras de entitlement ou integrações de pagamento.
- Publicar ticket em ferramenta externa ou abrir pull request sem comando explícito.

## Evidências
- Entrada: o usuário pediu uma nova US para `internal›agents›application›agents›mecontrola_agent.go` e `internal›agents›application›workflows›onboarding_workflow.go`, com fluxo decidido como boas-vindas isolada, meta no segundo passo, categorias com avanço imediato, orçamento mensal sem renda líquida, distribuição/ativação antes dos cartões e cartões um por vez.
- Base de código: `internal›agents›application›workflows›onboarding_workflow.go:23` define `OnboardingWorkflowID`; `internal›agents›application›workflows›onboarding_workflow.go:59` define `OnboardingPhase` fechado; `internal›agents›application›workflows›onboarding_workflow.go:62` inicia em `PhaseWelcome`; `internal›agents›application›workflows›onboarding_workflow.go:64` possui a fase atual `PhaseMonthlyIncome`; `internal›agents›application›workflows›onboarding_workflow.go:464` pergunta renda mensal líquida hoje; `internal›agents›application›workflows›onboarding_workflow.go:501` monta prompt de cartões com base em cartões existentes; `internal›agents›application›workflows›onboarding_workflow.go:508` monta a apresentação/sugestão atual das categorias; `internal›agents›application›workflows›onboarding_workflow.go:562` combina boas-vindas e pergunta de objetivo no step atual; `internal›agents›application›workflows›onboarding_workflow.go:628` implementa a coleta atual de renda; `internal›agents›application›workflows›onboarding_workflow.go:659` implementa cadastro atual de cartão em uma passagem; `internal›agents›application›workflows›onboarding_workflow.go:713` sugere distribuição a partir do valor mensal atual; `internal›agents›application›workflows›onboarding_workflow.go:760` cria draft de orçamento com `TotalCents`; `internal›agents›application›workflows›onboarding_workflow.go:804` exibe resumo antes de ativar; `internal›agents›application›workflows›onboarding_workflow.go:837` ativa orçamento e grava WorkingMemory; `internal›agents›application›workflows›onboarding_workflow.go:943` define a sequência atual dos steps; `internal›agents›application›interfaces›budget_planner.go:9` define a porta de orçamento usada pelo workflow; `internal›agents›application›interfaces›card_manager.go:9` define a porta de cartões; `internal/budgets/domain/valueobjects/root_slug.go:12` define as cinco categorias oficiais como enum fechado; `internal›agents›module.go:231` conecta `BuildOnboardingWorkflow` no módulo real.
- Inferências: como o workflow já é durável, tipado e conectado ao módulo real, a menor mudança robusta é refatorar fases, prompts e ordem dos steps existentes, preservando portas `BudgetPlanner` e `CardManager`; a decisão de não aplicar pattern é suportada pelo seletor de patterns com `status=reject`.
- Não evidenciado: não há evidência em código de produção de um passo isolado de boas-vindas; não há evidência de loop atual para vários cartões no onboarding; não há evidência de que o onboarding atual use "orçamento mensal" como semântica completa no lugar de renda líquida.

## Notas de Validação
- Skills aplicadas: `$user-stories`, `$go-implementation`, `$mastra`, `$domain-modeling-production` e `$design-patterns-mandatory`.
- Classificação Go para futura implementação: mudança em workflow/application de `internal/agents`, com perfil esperado `boundary`; deve validar build, vet, testes race no bounded context afetado e lint proporcional quando disponível.
- Decisão de design pattern: `select_pattern.py` retornou `status=reject`; a US exige solução direta/refactor local sobre o workflow existente, sem novo GoF pattern primário.
- Primitivo Mastra local: usar `BuildOnboardingWorkflow` como consumidor real de `internal/platform/workflow`; não mover regra para handler, consumer, prompt solto do agente ou substrato de plataforma.
- Modelagem de domínio: comandos esperados incluem `IniciarOnboarding`, `ApresentarMeControla`, `DefinirMetaFinanceira`, `ApresentarCategoriasOrcamento`, `DefinirOrcamentoMensal`, `CadastrarCartaoOnboarding`, `EncerrarCadastroCartoes`, `DefinirDistribuicaoOrcamento` e `AtivarOrcamento`.
- Invariantes esperadas: não perguntar renda líquida; orçamento mensal positivo é obrigatório antes de distribuição; apresentação de categorias sempre ocorre antes de orçamento mensal; distribuição e ativação do orçamento ocorrem antes do loop de cartões; cartão parcial não é persistido; múltiplos cartões são cadastrados um por vez; orçamento ativado não é desfeito por erro ou recusa no cadastro de cartões.
