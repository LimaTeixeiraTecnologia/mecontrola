# US-001: Operação Conversacional Diária do MeControla (reescrita completa do workflow do dia a dia)

## Declaração
Como assinante ativo do MeControla que já concluiu o onboarding e conversa pelo WhatsApp, quero registrar, editar e consultar minhas finanças e gerenciar orçamento, distribuição, cartões, objetivo, plano e suporte por linguagem natural, para manter o controle financeiro diário com confirmação explícita antes de cada gravação e respostas fiéis ao Tom de Voz oficial.

## Contexto
- Problema: o agente conversacional atual do dia a dia (`internal/agents`) cobre parte dos fluxos, mas diverge do documento oficial no Tom de Voz (confirmação e sucesso em linha única, sem frase motivacional), não cobre alterar o valor total do orçamento, alterar o objetivo depois do onboarding, cancelamento de plano conversacional nem suporte, e a edição de lançamento não busca candidatos nem permite trocar categoria/forma de pagamento pela tool. O prompt é monolítico e as mensagens ficam espalhadas em builders divergentes.
- Resultado esperado: uma reescrita da camada conversacional que cobre os 13 fluxos do documento com fidelidade verbatim ao Tom de Voz, confirmação universal antes de qualquer escrita, mensagens determinísticas geradas pelo sistema com frase motivacional rotacionada, e roteamento por registry de tools/workflows (sem `switch case intent.Kind`), preservando os primitivos de plataforma e os módulos de domínio.
- Fonte: documento de produto `US_Operacao_Conversacional_Diaria_MeControla_Versao_Final.md` (entrada do usuário) confrontado com a base de código deste repositório.

## Regras de Negócio

### RN-00 — Fronteira da reescrita
- A reescrita redesenha apenas a camada conversacional em `internal/agents` (agente, tools, workflows, prompt, mensagens, roteamento e guards). Preserva integralmente os primitivos de plataforma `internal/platform/{agent,workflow,memory,llm,scorer}` e os módulos de domínio `internal/{transactions,budgets,card,categories,identity,billing}`, conforme R-AGENT-WF-001 e R-WF-KERNEL-001.
- Mudanças em domínio, quando necessárias, são estritamente aditivas (novo usecase, novo valor de enum, nova tool de leitura), nunca reescrita de agregado, `Decide*` ou smart constructor existente.
- Esta US absorve e torna obsoletas as US anteriores de edição conversacional (Editar Transação, Editar Orçamento, Editar Cartão): o comportamento delas é reencarnado aqui com fidelidade ao documento vigente; nenhuma spec paralela de edição permanece autoritativa.

### RN-01 — Compreensão de linguagem natural
- O agente interpreta mensagens livres, sem formato fixo, para registrar, editar, consultar e gerenciar, cobrindo as variações de intenção do documento (ex.: "Gastei 20", "Mercado 100", "Recebi meu salário", "Como estou indo?").
- A extração de campos é feita pelo LLM nas call-sites sancionadas (loop de tool-calling do agente); a validação de cada campo é determinística nos smart constructors e DTOs, nunca no LLM.

### RN-02 — Normalização de valores monetários
- O agente reconhece valores em formatos numéricos, por extenso e gírias: `10`, `10 reais`, `dez`, `dez reais`, `10 conto`, `10 pila`, `10 mangos`, `mil`, `um mil`, `1.000`, `1000`, `R$ 1.000`, `R$1000,00`, `cem`, `duzentos`, `quinhentos`, `dois mil`, `dez mil`.
- O valor é convertido para centavos inteiros (`amount_cents`). O smart constructor rejeita valor menor ou igual a zero. Sem parser paralelo de domínio: a conversão vem do LLM e a validação é determinística.

### RN-03 — Taxonomia de forma de pagamento (extensão aditiva)
- O enum canônico de forma de pagamento em `internal/transactions` é estendido de forma aditiva para cobrir as formas do documento ainda ausentes: carteiras digitais (Apple Pay, Google Pay, PicPay, Mercado Pago), cheque, DOC e transferência, além das já suportadas (pix, débito, débito em conta, crédito, dinheiro, boleto, TED, vale-refeição, vale-alimentação).
- Menções a "Cartão <Banco>" (Nubank, Inter, C6, Itaú, Santander, Banco do Brasil, Caixa) não são forma de pagamento própria: resolvem para um cartão do usuário por apelido, com forma de pagamento crédito ou débito conforme a fala.
- Cada nova forma persiste como valor próprio; a classificação é determinística após extração do LLM.

### RN-04 — Confirmação explícita universal (HITL) antes de persistir
- Cada operação de escrita — registrar despesa, registrar receita, criar recorrência, editar despesa/receita, alterar valor total do orçamento, alterar distribuição/percentuais, alterar objetivo, cadastrar cartão, editar cartão, excluir cartão, excluir recorrência — apresenta um resumo e aguarda confirmação explícita antes de gravar.
- Nenhuma gravação ocorre sem confirmação positiva do usuário. Isso muda o comportamento atual em que trocar apelido ou banco de cartão gravava direto.
- O estado de espera é um tipo fechado persistido no `Snapshot` do kernel antes de a pergunta de confirmação ser enviada; a retomada aplica merge-patch (RFC 7386) sobre o estado antes de qualquer novo parse; o run é sempre finalizado (nunca suspenso órfão) após efetivar, cancelar ou expirar.

### RN-05 — Invariantes de produção reencarnadas
- Idempotência de escrita por `wamid + item_seq + operation` via ledger durável (`agents_write_ledger`), com `ON CONFLICT DO NOTHING` e reaper de retenção.
- Guarda anti-falso-sucesso: uma confirmação positiva que não resulte em recurso persistido finaliza o passo como `StepStatusFailed` e incrementa a métrica de falso-sucesso; nunca devolve mensagem de sucesso sem recurso gravado.
- Reclassificação de categoria por kind quando a categoria escolhida não for compatível com a direção do lançamento.
- Expiração por TTL avaliada na retomada e limpeza determinística do estado de espera; teto de reprompt em respostas ambíguas antes de cancelar.

### RN-06 — Mensagens determinísticas verbatim e Tom de Voz
- As mensagens de confirmação, sucesso, esclarecimento e as respostas informacionais são geradas pelo sistema (não pelo LLM) com estrutura e campos verbatim do documento; o LLM apenas repassa o texto da tool sem parafrasear (guard de relay verbatim).
- A frase motivacional de fechamento é sorteada de uma lista fixa por cenário; nenhuma resposta de escrita conclui sem frase motivacional coerente.
- Emojis obrigatórios seguem o documento (✅ 💰 💳 📂 📥 📊 ⚠️ 🚨 🎉 💚). Formatação WhatsApp com asterisco simples para negrito.
- Blocos verbatim de confirmação de despesa: `✅ Encontrei este lançamento:` seguido de `💰 Valor`, `💳 Pagamento`, `📂 Categoria` e `Posso registrar?`. Bloco de confirmação de receita: `✅ Encontrei esta entrada:` seguido de `💰 Valor`, `📥 Origem` e `Posso registrar?`. Sucesso de despesa: `Prontinho! ✅` com frase motivacional. Sucesso de receita: `Boa notícia! 🎉` com frase motivacional.

### RN-07 — Registro de despesa (Fluxo 1)
- O agente identifica intenção, valor, estabelecimento (descrição literal), categoria e forma de pagamento quando informada.
- Quando faltar apenas um campo, o agente solicita somente o campo ausente e nunca repergunta informação já identificada.
- A categoria é resolvida automaticamente; havendo ambiguidade, o agente lista candidatos para escolha; havendo direção incompatível, reclassifica por kind.

### RN-08 — Registro de receita (Fluxo 2)
- O agente identifica valor, origem e tipo da receita; não pergunta forma de pagamento.
- A confirmação usa o bloco de entrada com `📥 Origem`.

### RN-09 — Edição de despesa e receita (Fluxos 3 e 4)
- O agente busca lançamentos compatíveis do período vigente por valor, categoria, descrição e recência.
- Havendo mais de um candidato, lista as opções para escolha; havendo apenas um, apresenta o registro (valor anterior, categoria, forma de pagamento) e o novo valor, e pergunta "Posso atualizar?".
- A edição permite alterar valor, categoria/subcategoria e forma de pagamento, além de descrição e data, respeitando o guard de migração de forma de pagamento do domínio (algumas migrações envolvendo crédito com parcelas são restringidas).
- Receita e despesa compartilham o fluxo genérico de edição.

### RN-10 — Alteração do valor total do orçamento (Fluxo 5)
- O agente busca o orçamento ativo, pergunta o novo valor mensal, reescala proporcionalmente os percentuais atuais para o novo total, apresenta o resumo e persiste após confirmação.
- A distribuição existente é preservada em proporção; o usuário não é obrigado a redistribuir manualmente.

### RN-11 — Alteração da distribuição financeira (Fluxo 6)
- O agente busca a distribuição atual, exibe os percentuais por categoria, solicita a nova distribuição, apresenta o resumo e persiste após confirmação.
- A soma dos percentuais permanece coerente com as regras de domínio de orçamento; percentuais inválidos são rejeitados na validação determinística.

### RN-12 — Cadastro de cartão (Fluxo 7)
- O agente pergunta apelido e dia de vencimento (e banco quando ajudar a resolução), apresenta o resumo e persiste após confirmação, encerrando com frase motivacional.

### RN-13 — Edição e exclusão de cartão (Fluxo 8)
- O agente busca o cartão, apresenta os dados atuais, solicita a alteração (vencimento, apelido, banco), apresenta o resumo e persiste após confirmação.
- Exclusão de cartão passa pelo gate destrutivo com confirmação e aviso de impacto (parcelas em aberto), e só efetiva após confirmação positiva.

### RN-14 — Alteração do objetivo financeiro (Fluxo 9)
- O objetivo permanece como texto em WorkingMemory por recurso (onde o onboarding já o grava, incluindo o valor em centavos quando informado).
- O agente busca o objetivo atual, pergunta o novo objetivo, apresenta o resumo e reescreve a WorkingMemory após confirmação, encerrando com frase motivacional. Não há criação de agregado de domínio para objetivo.

### RN-15 — Cancelamento do plano (Fluxo 10)
- Resposta informacional determinística verbatim, servida por tool de leitura estática, com o passo a passo oficial da Kiwify (acessar conta, Minhas Compras, localizar a assinatura MeControla, Gerenciar Assinatura, Cancelar Assinatura e confirmar) e fechamento acolhedor.
- A tool apenas informa: não chama API da Kiwify nem o módulo de billing, e não altera o estado da assinatura.

### RN-16 — Suporte (Fluxo 11)
- Resposta informacional determinística verbatim, servida por tool de leitura estática, orientando o envio de e-mail para `contato@limateixeira.com.br` com prazo de resposta de até 24 horas.

### RN-17 — Resumo por categoria (Fluxo 12)
- Uma tool de leitura dedicada retorna os lançamentos do período filtrados pela categoria (resolvendo subcategoria para a categoria raiz quando o usuário cita apenas a subcategoria), com data, valor e subcategoria por lançamento, além de planejado, gasto e disponível ou excedente.
- O bloco segue a estrutura verbatim do documento e conclui com frase motivacional coerente com o cenário (disponível, próximo do limite, exatamente no limite, ultrapassado).

### RN-18 — Resumo geral do orçamento (Fluxo 13)
- O agente exibe cada categoria do planejamento com planejado, gasto e disponível, e o consolidado (total planejado, total gasto, total disponível), concluindo com mensagem contextualizada (positivo, atenção, crítico), na estrutura verbatim do documento.

### RN-19 — Mensagem com múltiplos lançamentos
- Quando a mensagem contém mais de um lançamento, o agente bloqueia e orienta a enviar um de cada vez, preservando a confirmação HITL individual e a idempotência por `wamid + item_seq`.

### RN-20 — Recorrência
- Recorrência (criar, listar, alterar, excluir) permanece no escopo como variante de registro, reencarnada com o mesmo Tom de Voz, confirmação e idempotência dos demais fluxos.

### RN-21 — Roteamento por registry
- O roteamento resolve tools e workflows por registry; é proibido `switch case intent.Kind` para decidir qual usecase chamar. Novo comportamento entra como nova tool/workflow, não como branch de domínio no agente.
- Estados de fronteira são tipos fechados: `agent.RunStatus`/`ToolOutcome`/`AwaitingKind`, `workflow.RunStatus`/`StepStatus`/`SuspendReason`, e os enums de operação e espera dos workflows; nunca string livre.

### RN-22 — Pendência conversacional em aberto
- Havendo um lançamento pendente aguardando confirmação, o agente conclui esse pendente ou aceita cancelamento antes de iniciar um novo, sem gravar de forma parcial ou silenciosa.

## Critérios de Aceite

```gherkin
Cenário: Registrar despesa completa com confirmação verbatim
  Dado que sou um assinante ativo com onboarding concluído no WhatsApp
  Quando envio "Comprei bloco no depósito foi 300 conto"
  Então o agente responde com o bloco "✅ Encontrei este lançamento:" contendo 💰 Valor R$ 300,00, 💳 Pagamento e 📂 Categoria e a pergunta "Posso registrar?"
  E nenhuma gravação ocorre antes da minha confirmação

Cenário: Confirmar despesa e receber sucesso motivacional
  Dado que o agente apresentou o resumo de uma despesa de R$ 300,00 aguardando confirmação
  Quando respondo "sim"
  Então o lançamento é persistido de forma idempotente
  E o agente responde "Prontinho! ✅" com uma frase motivacional sorteada da lista fixa

Cenário: Solicitar apenas o campo ausente sem reperguntar o conhecido
  Dado que envio "Gastei 40 no estacionamento"
  Quando a forma de pagamento não foi informada e a categoria foi resolvida
  Então o agente pergunta somente a forma de pagamento
  E não repergunta valor, estabelecimento ou categoria já identificados

Cenário: Registrar receita com origem
  Dado que envio "Recebi meu salário, entrou 2 mil"
  Quando o agente identifica valor e origem
  Então responde com o bloco "✅ Encontrei esta entrada:" contendo 💰 Valor R$ 2.000,00 e 📥 Origem
  E não pergunta forma de pagamento

Cenário: Editar despesa com múltiplos candidatos
  Dado que tenho mais de um lançamento compatível no período vigente
  Quando envio "Corrige aquele mercado, era 25"
  Então o agente lista as opções compatíveis para eu escolher
  E após a escolha apresenta valor anterior, categoria e forma de pagamento e pergunta "Posso atualizar?"

Cenário: Editar despesa trocando a categoria
  Dado que existe um único lançamento compatível
  Quando envio "Muda a categoria daquele lançamento para Casa"
  Então o agente apresenta o novo valor e a nova categoria e pede confirmação
  E ao confirmar a categoria e a forma de pagamento podem ser alteradas respeitando o guard de migração de forma de pagamento

Cenário: Alterar o valor total do orçamento preservando a distribuição
  Dado que tenho um orçamento ativo com distribuição por categoria
  Quando envio "Quero mudar meu orçamento, vou ganhar mais" e informo o novo valor mensal
  Então o agente reescala proporcionalmente os percentuais atuais para o novo total
  E persiste somente após a minha confirmação

Cenário: Alterar a distribuição financeira
  Dado que tenho uma distribuição ativa
  Quando envio "Quero mudar os percentuais das minhas categorias"
  Então o agente exibe a distribuição atual, solicita a nova, apresenta o resumo e persiste após confirmação

Cenário: Cadastrar cartão
  Dado que quero adicionar um cartão
  Quando envio "Cadastrar cartão" e informo apelido e dia de vencimento
  Então o agente apresenta o resumo do cartão e persiste após confirmação com frase motivacional

Cenário: Editar apelido de cartão agora exige confirmação
  Dado que possuo um cartão cadastrado
  Quando envio "Muda o apelido daquele cartão"
  Então o agente apresenta o resumo da alteração e pergunta a confirmação
  E a alteração de apelido só é gravada após a minha confirmação explícita

Cenário: Alterar o objetivo financeiro
  Dado que meu objetivo está registrado na WorkingMemory desde o onboarding
  Quando envio "Meu sonho mudou, quero trocar minha meta"
  Então o agente busca o objetivo atual, pergunta o novo, apresenta o resumo e reescreve a WorkingMemory após confirmação

Cenário: Orientar cancelamento de plano pela Kiwify
  Dado que envio "Quero cancelar minha assinatura"
  Quando o agente identifica a intenção de cancelamento de plano
  Então responde com o passo a passo verbatim da Kiwify e fechamento acolhedor
  E não chama a API da Kiwify nem o módulo de billing

Cenário: Orientar suporte por e-mail
  Dado que envio "Preciso de ajuda, deu um erro"
  Quando o agente identifica intenção de suporte
  Então orienta enviar e-mail para contato@limateixeira.com.br com prazo de até 24 horas

Cenário: Resumo por categoria a partir de subcategoria
  Dado que pergunto "Quanto gastei com Água esse mês?"
  Quando o agente resolve a subcategoria Água para a categoria raiz Custo Fixo
  Então exibe cada lançamento do período com data, valor e subcategoria
  E exibe planejado, gasto e disponível com frase motivacional coerente com o cenário

Cenário: Resumo geral do orçamento
  Dado que pergunto "Como estou indo?"
  Quando o agente identifica pedido de panorama geral
  Então exibe cada categoria com planejado, gasto e disponível e o consolidado geral
  E conclui com mensagem contextualizada ao cenário financeiro

Cenário: Mensagem com múltiplos lançamentos é bloqueada
  Dado que envio "Gastei 30 no ônibus e 50 no mercado"
  Quando o agente detecta mais de um lançamento na mesma mensagem
  Então orienta a enviar um de cada vez
  E não grava nenhum dos lançamentos parcialmente

Cenário: Confirmação positiva sem recurso persistido não devolve falso sucesso
  Dado que confirmei um registro e a gravação não retornou um recurso persistido
  Quando o passo avalia o resultado da escrita
  Então finaliza como falha, incrementa a métrica de falso-sucesso e não devolve mensagem de sucesso

Cenário: Pendência em aberto bloqueia novo registro
  Dado que existe um lançamento pendente aguardando minha confirmação
  Quando envio um novo lançamento antes de concluir o pendente
  Então o agente pede para concluir ou cancelar o pendente antes de seguir

Cenário: Expiração do estado de espera por TTL
  Dado que um registro ficou pendente além do TTL configurado
  Quando eu respondo após a expiração
  Então o agente informa que o registro expirou e finaliza o run sem gravar

Cenário: Resposta ambígua na confirmação segue política de reprompt
  Dado que o agente aguarda confirmação de um registro
  Quando respondo algo ambíguo além do teto de reprompt
  Então o agente cancela o registro sem efeito e finaliza o run
```

## Dados e Permissões
- Dados obrigatórios por operação de escrita: `resourceId` e `threadId` opacos (identidade do Thread), `messageId` (wamid) e `item_seq` para idempotência, e os campos de domínio de cada fluxo (valor em centavos, descrição, forma de pagamento, categoria/subcategoria, cartão, datas, percentuais, apelido e vencimento de cartão, texto de objetivo).
- Perfis/permissões: apenas assinante autenticado com `auth.Principal` no contexto; operações de escrita exigem `Principal` válido e recusam sem ele. Onboarding concluído é pré-condição do fluxo do dia a dia.
- Cardinalidade de métricas: labels restritos a enums fechados (`agent_id`, `channel`, `workflow`, `status`, `tool`, `outcome`); proibido `user_id`, `correlation_key` ou `category_id` como label.

## Dependências
- Primitivos de plataforma existentes e preservados: `internal/platform/agent` (runtime, loop de tool-calling, registry, RunStore), `internal/platform/workflow` (kernel `Engine[S]`, `Store`, merge-patch no resume, CAS otimista, reaper), `internal/platform/memory` (Thread, MessageStore, WorkingMemory), `internal/platform/llm` (provider OpenRouter), `internal/platform/scorer`.
- Módulos de domínio consumidos por binding: `internal/transactions` (criar/editar transação e recorrência), `internal/budgets` (criar/ativar orçamento, editar percentual, resumo mensal, sugerir alocação), `internal/card` (criar/editar/excluir/consultar cartão), `internal/categories` (resolver e classificar categoria).
- Ledger de idempotência existente reutilizado: `agents_write_ledger` via `write_ledger_repository.go` (mantido, não recriado).
- Extensões aditivas necessárias e ainda inexistentes: usecase de alteração do valor total do orçamento com reescala proporcional em `internal/budgets`; extensão aditiva do enum de forma de pagamento em `internal/transactions` para carteiras digitais, cheque, DOC e transferência; extensão da tool/binding de edição para expor categoria e forma de pagamento; tool de leitura de detalhe por categoria com lançamentos do período; tools de leitura estáticas de cancelamento de plano e suporte; usecase de leitura/reescrita do objetivo na WorkingMemory.

## Fora de Escopo
- Reescrita de qualquer primitivo de plataforma (`internal/platform/*`) ou de qualquer agregado, `Decide*` ou smart constructor existente nos módulos de domínio.
- Reescrita do fluxo de onboarding (é outro fluxo; aqui apenas o dia a dia pós-onboarding).
- Integração ativa com a API da Kiwify para cancelar assinatura pelo chat (cancelamento permanece informacional; a efetivação segue pelo webhook de billing já existente).
- Criação de agregado de domínio para objetivo/meta (permanece em WorkingMemory).
- Multi-turn de fila para múltiplos lançamentos numa só mensagem (permanece bloqueio com orientação de um de cada vez).
- Publicação desta história em Jira, Azure DevOps ou GitHub Issues.

## Evidências
- Entrada: `US_Operacao_Conversacional_Diaria_MeControla_Versao_Final.md` (13 fluxos, blocos verbatim de confirmação/sucesso/resumo, frases motivacionais, passo a passo Kiwify, e-mail de suporte contato@limateixeira.com.br em até 24 horas).
- Base de código:
  - Agente único e catálogo de tools: arquivo `mecontrola_agent.go` (pacote `agents`, em `internal/agents/application`), id `mecontrola-agent` na linha 13, build em :265, prompt em :18-262; registro em `internal/agents/module.go:216-217`; runtime com write-tool set em `internal/agents/module.go:234-236`.
  - Workflow de registro/edição atual (a reescrever): `internal/agents/application/workflows/pending_entry_workflow.go`; confirmação em linha única `:919-939` (`buildConfirmSummary`); sucesso sem frase motivacional `:845-877` (`buildWriteSuccessText`); idempotência e guarda de falso-sucesso `:87`, `:558-599`; roteamento de escrita `:647-656` (`callLedger`) cobre apenas criar/editar transação e recorrência.
  - Ledger de idempotência preservado: `internal/agents/infrastructure/persistence/write_ledger_repository.go:34-113` (chave `wamid+item_seq+operation`, `ON CONFLICT DO NOTHING`, reaper `DeleteBefore`).
  - Domínio de edição de transação já aceita categoria e forma de pagamento: `internal/transactions/application/usecases/update_transaction.go:53-100` (recebe `RawUpdateTransaction` com categoria/subcategoria/forma de pagamento/valor/versão) e guard de migração `:120`; a tool `internal/agents/application/tools/edit_entry.go` expõe apenas valor/descrição/data.
  - Distribuição/percentuais existente: tool `internal/agents/application/tools/adjust_allocation.go:28` → `BudgetPlanner.EditCategoryPercentage` (`internal/agents/application/interfaces/budget_planner.go`), usecase `internal/budgets/application/usecases/edit_category_percentage.go`.
  - Cadastro e edição de cartão: `internal/agents/application/tools/create_card.go`, `update_card.go` (confirma só quando muda vencimento), exclusão via `delete_entry.go` + `destructive_confirm_workflow.go`; usecases em `internal/card/application/usecases/{create_card.go,update_card.go,soft_delete_card.go}`.
  - Resumos: `internal/agents/application/tools/query_plan.go` → `internal/budgets/application/usecases/get_monthly_summary.go` (agregados por categoria raiz e consolidado); `internal/agents/application/tools/query_month.go` (lançamentos do mês).
  - Objetivo capturado só no onboarding: `internal/agents/application/workflows/onboarding_workflow.go` grava `## Objetivo Financeiro` e metadata de valor na WorkingMemory; leitura em `internal/agents/application/usecases/resolve_onboarding_or_agent.go`.
  - Primitivos preservados: `internal/platform/agent/runtime.go:87-159` (Execute/Thread/Run) e `:304-328` (WorkingMemory no system prompt); `internal/platform/workflow/engine.go:218-294` (Resume) e `codec.go:48-71` (merge-patch); `internal/platform/memory/ports.go` (Thread/WorkingMemory/MessageStore); `internal/platform/llm/openrouter.go` (provider); estados fechados em `internal/platform/agent/types.go` e `internal/platform/workflow/step.go`.
  - Guards de tom/verbatim atuais reaproveitáveis: arquivo `mecontrola_agent.go` (mesmo pacote `agents`), linhas 302-308 (relay verbatim, success_without_tool, internal_terms).
- Inferências: a busca de candidatos para edição (por valor/categoria/descrição/recência) e a reescala proporcional de percentuais ao alterar o total são desenhos novos derivados do documento; a extensão do enum de forma de pagamento e a tool de detalhe por categoria são inferidas como mínimas e aditivas para atingir a fidelidade verbatim dos blocos do documento.
- Não evidenciado: usecase de alteração do valor total do orçamento (buscado em `internal/budgets/application/usecases/` — inexistente; há apenas criar/ativar/excluir rascunho/editar percentual); tool ou texto conversacional de cancelamento de plano e de suporte (busca por `contato@` e por orientação Kiwify no agente retornou zero); tool de listagem de lançamentos filtrada por uma única categoria (inexistente; `query_plan` só devolve agregados e `query_month` devolve o mês inteiro); fluxo de alteração de objetivo pós-onboarding (inexistente).

## Notas de Validação
- Esta é uma história de nível épico por diretriz explícita do usuário ("uma única US" cobrindo a operação diária inteira). Os 13 fluxos do documento são fatias internas da mesma história; cada fatia possui cenário feliz, cenário alternativo e cenário de erro entre os critérios de aceite.
- Cobertura dos 3 Cs: cartão (declaração e fatias por fluxo), conversa (12 decisões de escopo resolvidas com o usuário antes da redação), confirmação (critérios de aceite verificáveis com blocos verbatim e invariantes de idempotência/falso-sucesso).
- As 12 decisões de escopo foram confirmadas pelo usuário: fronteira só na camada conversacional; mensagens determinísticas verbatim com motivacional rotacionado; confirmação universal; supersessão das US anteriores de edição; alterar total do orçamento preservando distribuição; edição com busca de candidatos e campos ampliados; extensão aditiva do enum de forma de pagamento; objetivo em WorkingMemory; informacionais como tools estáticas verbatim; bloqueio de múltiplos lançamentos; recorrência mantida; tool de detalhe por categoria.
- Conformidade de governança verificada contra R-AGENT-WF-001 (roteamento por registry, tool fina, estados fechados, LLM só nas call-sites sancionadas, Run auditável, HITL com estado de espera persistido antes da confirmação e resume por merge-patch), R-WF-KERNEL-001 (kernel sem domínio, preservado), R-ADAPTER-001 (adaptadores finos e zero comentários), R-DTO-VALIDATE-001 (Validate em cada input DTO) e R-TESTING-001 (suite canônica). A implementação e as tasks derivadas desta US devem executar os gates dessas regras.
- Sem marcadores pendentes, sem ressalvas em aberto e sem afirmação de suporte da base de código sem caminho e linha.
```
