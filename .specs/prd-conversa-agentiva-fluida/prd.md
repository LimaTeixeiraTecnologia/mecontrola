<!-- spec-version: 2 -->

# Documento de Requisitos do Produto (PRD) — Conversa Agentiva Fluida para Registro Financeiro

## Visão Geral

O MeControla precisa evoluir a experiência conversacional do agente financeiro para lidar com fluxos incompletos, ambíguos ou retomados sem perder o contexto já informado pelo usuário. O caso motivador é um registro de despesa em que o usuário informa valor, local, data e pagamento, o agente pede clarificação de categoria, mas os turnos seguintes são interpretados como novas intenções soltas. O resultado é uma conversa quebrada: o usuário responde "custo fixo" e "sim e pix", mas o agente volta a pedir categoria para uma compra anterior ou reinicia a coleta de dados.

A funcionalidade deve tornar a conversa mais fluida e confiável para registros financeiros via WhatsApp, preservando a intenção pendente, os dados já extraídos e o motivo da clarificação até que o fluxo seja concluído, cancelado ou expirado. O produto deve usar as capacidades existentes dos módulos `internal/agents`, `internal/categories` e `internal/transactions`, sem permitir que o agente invente categoria, simule sucesso ou registre transações com evidência categorial insuficiente.

O objetivo de produto não é "perguntar menos a qualquer custo". O objetivo é perguntar apenas o dado faltante, uma pergunta por vez, e retomar corretamente a operação original quando o usuário responde de forma curta, natural ou parcialmente elíptica.

Esta versão fixa todas as decisões funcionais abertas: uma nova frase completa de lançamento substitui a pendência anterior e inicia nova operação; toda escolha categorial persistível exige categoria raiz canônica e subcategoria folha canônica, ambas com `id` e `slug`; e a medição oficial de retomada/confusão usa harness determinístico com evidência em Run auditável.

## Objetivos

- **O-01 — Continuidade conversacional.** O agente deve preservar uma operação financeira pendente entre turnos quando precisar de clarificação de categoria, pagamento, cartão, data ou confirmação.
- **O-02 — Menos atrito no registro.** O usuário não deve repetir valor, descrição, data ou forma de pagamento quando esses dados já foram informados em um turno anterior.
- **O-03 — Zero sucesso simulado.** O agente nunca deve afirmar que registrou, editou ou confirmou uma operação sem retorno real da tool/use case correspondente.
- **O-04 — Clarificação categorial segura.** Ambiguidade ou ausência de categoria deve bloquear escrita até que uma categoria canônica seja resolvida e validada.
- **O-05 — Fluidez em português natural.** Respostas curtas como "custo fixo", "sim", "pix", "essa mesmo", "não, era farmácia" ou "cancela" devem ser interpretadas dentro do contexto pendente quando houver uma operação aguardando input.
- **O-06 — Auditabilidade.** O produto deve permitir investigar por que o agente perguntou, retomou, registrou, cancelou ou expirou um fluxo pendente.

### Métricas-chave

- **M-01 Taxa de retomada correta de pendência:** percentual de respostas curtas corretamente associadas à operação pendente, medido por harness determinístico com verificação de estado, tool calls, resposta final e Run auditável. Meta: 100% no conjunto de cenários canônicos.
- **M-02 Repetição evitada:** percentual de fluxos pendentes concluídos sem solicitar novamente dados já informados. Meta: 100% nos cenários determinísticos.
- **M-03 Sucesso simulado:** respostas de sucesso sem escrita real comprovada. Meta: 0.
- **M-04 Escrita com categoria insegura:** transações persistidas sem categoria canônica validada e evidência mínima. Meta: 0.
- **M-05 Perguntas por pendência:** número de perguntas adicionais para concluir um registro parcialmente informado. Meta: no máximo uma pergunta por dado realmente faltante.
- **M-06 Confusão entre pendências:** casos em que resposta do usuário é aplicada a lançamento errado, medidos por harness determinístico com evidência em Run auditável. Meta: 0 nos cenários canônicos.

## Histórias de Usuário

- **US-01:** Como usuário, quero dizer "Gastei R$ 150,00 no mercado hoje, no pix" e, se faltar categoria, responder apenas "custo fixo" para o agente concluir ou validar o registro sem pedir tudo de novo.
- **US-02:** Como usuário, quero corrigir naturalmente uma pendência dizendo "não, era farmácia" para o agente atualizar a descrição/contexto antes de tentar categorizar novamente.
- **US-03:** Como usuário, quero responder "sim e pix" quando o agente pedir confirmação ou forma de pagamento, para que a operação pendente seja retomada sem iniciar uma nova conversa.
- **US-04:** Como usuário, quero que o agente faça uma pergunta por vez e mantenha a conversa curta, sem linguagem técnica nem explicação de arquitetura.
- **US-05:** Como usuário, quero poder cancelar uma operação pendente com "cancela" ou "deixa pra lá", sem risco de registro posterior.
- **US-06:** Como operador, quero auditar o estado pendente, a resposta recebida e o desfecho, para depurar conversas quebradas e medir qualidade.

## Funcionalidades Core

1. **Estado pendente de registro financeiro.** O produto deve representar quando uma operação está aguardando uma informação específica do usuário, como categoria, forma de pagamento, cartão, data, confirmação ou correção.
2. **Retomada por resposta curta.** O agente deve interpretar mensagens subsequentes à luz da pendência ativa antes de tratá-las como uma nova intenção independente.
3. **Preservação de slots extraídos.** Valor, descrição, data, forma de pagamento, cartão, parcelas, categoria candidata e evidência categorial já obtidos devem ser preservados até conclusão, cancelamento ou expiração.
4. **Clarificação categorial segura.** Quando `internal/categories` não resolver uma única categoria válida para escrita, o agente deve pedir clarificação e só persistir em `internal/transactions` após nova resolução canônica.
5. **Correção natural de contexto.** O usuário deve poder corrigir descrição, estabelecimento, pagamento ou data durante a pendência sem reiniciar todo o fluxo.
6. **Cancelamento e expiração.** Pendências devem poder ser canceladas explicitamente e devem expirar de forma previsível quando não houver resposta em tempo razoável.
7. **Resposta final objetiva.** Ao concluir, o agente deve confirmar valor, categoria e período/data quando aplicável, usando linguagem pronta para WhatsApp e sem detalhes internos.
8. **Substituição explícita por nova operação completa.** Quando o usuário enviar uma nova frase completa de lançamento durante uma pendência, o produto deve encerrar a pendência anterior como substituída e processar a nova operação.
9. **Escolha categorial canônica completa.** Toda opção de categoria apresentada para escrita deve conter raiz canônica e subcategoria folha canônica, com `id` e `slug` de ambas.

## Requisitos Funcionais

- **RF-01:** O produto DEVE detectar quando uma tool ou use case de registro financeiro retorna necessidade de clarificação e abrir uma pendência conversacional vinculada à operação original.
- **RF-02:** A pendência DEVE preservar, no mínimo, tipo de operação, valor, descrição, data/competência inferida, forma de pagamento, cartão quando houver, parcelas quando houver, candidatos de categoria quando existirem, motivo da pendência e identificador de correlação da conversa.
- **RF-03:** Quando houver pendência ativa, a próxima mensagem do usuário DEVE ser avaliada primeiro como possível resposta à pendência antes de ser roteada como nova intenção.
- **RF-04:** O agente DEVE fazer apenas uma pergunta por mensagem e perguntar somente o dado faltante ou ambíguo.
- **RF-05:** O agente NÃO DEVE solicitar novamente valor, descrição, data, forma de pagamento ou cartão quando esses dados já estiverem preservados e válidos na pendência.
- **RF-06:** O agente DEVE aceitar respostas curtas e elípticas como preenchimento de slot pendente quando forem compatíveis com o estado aguardado.
- **RF-07:** O agente DEVE permitir correção explícita de slots já coletados, como descrição, categoria pretendida, pagamento, cartão, data ou parcelas, antes da escrita final.
- **RF-08:** O agente DEVE cancelar a pendência quando o usuário expressar cancelamento inequívoco, sem executar escrita posterior.
- **RF-09:** O agente DEVE expirar pendências após 30 minutos de inatividade e informar ao usuário, em linguagem simples, que será necessário começar novamente quando a resposta chegar fora da janela válida.
- **RF-10:** O agente DEVE bloquear a escrita quando a categoria continuar ausente, ambígua, incompatível com despesa/receita, deprecated, sem subcategoria folha ou sem versão/evidência categorial suficiente.
- **RF-11:** O agente DEVE usar `internal/categories` como fonte canônica de classificação, listagem, candidatos e validação categorial para escrita.
- **RF-12:** O agente DEVE usar `internal/transactions` como autoridade de persistência e validação final para criação/edição de transações e templates recorrentes.
- **RF-13:** O agente NÃO DEVE usar prompt, scorer, resposta de LLM ou texto livre do usuário como autoridade final para escolher IDs de categoria.
- **RF-14:** Quando o usuário informar uma categoria por nome livre, o sistema DEVE resolvê-la novamente por contrato canônico antes de persistir.
- **RF-15:** Quando a resposta curta puder ser tanto nova intenção quanto resposta à pendência, o agente DEVE preferir a interpretação de pendência se houver compatibilidade clara com o slot aguardado.
- **RF-16:** Quando a resposta curta for incompatível com a pendência ativa, o agente DEVE pedir esclarecimento sem descartar automaticamente a operação original.
- **RF-17:** O agente DEVE diferenciar estados de pendência por tipo fechado, incluindo pelo menos: aguardando categoria, aguardando pagamento, aguardando cartão, aguardando data, aguardando confirmação, aguardando correção e cancelado/expirado/concluído.
- **RF-18:** O fluxo DEVE seguir o pipeline funcional `parse → validate → decide → persist → publish/respond`, mantendo decisões de negócio fora de handlers, consumers, jobs e tools finas.
- **RF-19:** A decisão de retomar, cancelar, pedir dado adicional ou persistir DEVE ser testável como regra determinística sempre que não depender de LLM.
- **RF-20:** Toda escrita financeira originada de conversa retomada DEVE preservar idempotência por identidade de inbound/correlação, evitando registro duplicado quando o usuário repete ou confirma a mesma operação.
- **RF-21:** O agente DEVE confirmar sucesso somente depois que a tool/use case de escrita retornar sucesso real, com identificador ou evidência de recurso criado/atualizado quando aplicável.
- **RF-22:** Em caso de erro de tool/use case, o agente DEVE informar falha sem declarar sucesso e sem perder a pendência quando ainda for possível corrigir o input.
- **RF-23:** O produto DEVE registrar evidência auditável do motivo da clarificação, do slot respondido, da decisão tomada e do desfecho da pendência.
- **RF-24:** O agente DEVE manter respostas finais compatíveis com WhatsApp: português do Brasil, texto curto, sem markdown incompatível e sem mencionar infraestrutura interna.
- **RF-25:** O produto DEVE cobrir pelo menos os fluxos de despesa via pix/débito/dinheiro/boleto, despesa em cartão de crédito, receita, edição de lançamento e criação de recorrência quando houver categoria.
- **RF-26:** A experiência DEVE impedir que uma resposta do usuário seja aplicada a uma pendência diferente da operação mais recente da mesma thread, salvo se houver identificação explícita.
- **RF-27:** O produto DEVE permitir listar ou apresentar opções de categoria quando houver múltiplos candidatos plausíveis, sem escolher automaticamente o primeiro candidato.
- **RF-28:** Toda opção categorial apresentada para destravar uma escrita DEVE conter categoria raiz canônica com `id` e `slug` e subcategoria folha canônica com `id` e `slug`; exemplo de contrato: raiz `66cb85a0-3266-5900-b8e3-13cdcd00ab62` + `custo-fixo` e subcategoria `c2fda6a3-c329-52c8-81ea-771b6ea4f365` + `aluguel`.
- **RF-29:** O usuário pode ver nomes legíveis, mas o contrato persistível e auditável DEVE carregar os IDs e slugs canônicos da raiz e da subcategoria folha escolhidas.
- **RF-30:** Categoria raiz sem subcategoria folha DEVE ser bloqueada em qualquer escrita financeira originada da conversa.
- **RF-31:** Quando o usuário enviar uma nova frase completa de lançamento durante uma pendência ativa, o produto DEVE encerrar a pendência anterior com status fechado de substituída e processar a nova frase como nova operação explícita.
- **RF-32:** A pendência substituída por nova operação NÃO DEVE gerar escrita posterior, mesmo que o usuário responda depois com uma palavra compatível com o slot antigo.
- **RF-33:** O harness determinístico DEVE ser a fonte oficial para medir M-01 e M-06, verificando estado pendente, transição decidida, tool calls esperadas, escrita real quando houver e Run auditável.
- **RF-34:** Scorers LLM-judged podem complementar observabilidade, mas NÃO PODEM substituir o harness determinístico como critério de aceite de retomada correta ou ausência de confusão entre pendências.
- **RF-35:** O produto DEVE rejeitar fallback para categoria genérica, categoria raiz sem folha, primeira categoria da lista ou categoria estimada pelo LLM.
- **RF-36:** A futura implementação DEVE usar as skills obrigatórias `go-implementation` e `mastra`, respeitando as regras DMMF do repositório para state-as-type, smart constructors, decisões puras e workflow pipeline.
- **RF-37:** A solução NÃO DEVE reimplementar primitivos de thread, run, working memory, workflow ou tool fora de `internal/platform/{agent,memory,workflow,tool}` e do consumidor `internal/agents`.

## Experiência do Usuário

### Fluxo principal esperado

1. Usuário: "Gastei R$ 150,00 no mercado hoje, no pix".
2. Agente identifica despesa, valor, descrição, data e pagamento.
3. Se a categoria estiver insegura, agente pergunta uma única coisa: "Qual categoria você quer usar para essa despesa no mercado?"
4. Usuário: "custo fixo".
5. Agente interpreta como resposta à categoria pendente, resolve de forma canônica e, se válida, registra sem pedir valor/pagamento novamente.
6. Agente confirma de forma curta: "Despesa de R$ 150,00 registrada em *Custo Fixo* para hoje no pix ✅".

### Fluxo com correção

1. Usuário: "Gastei R$ 150,00 no mercado hoje, no pix".
2. Agente pede categoria por ambiguidade.
3. Usuário: "na verdade foi farmácia".
4. Agente atualiza a descrição/contexto da pendência, reclassifica e pergunta apenas o que ainda faltar.

### Fluxo com nova operação explícita

1. Usuário: "Gastei R$ 150,00 no mercado hoje, no pix".
2. Agente pede categoria por ambiguidade.
3. Usuário: "Gastei R$ 150,00 na farmácia hoje, no pix".
4. Agente encerra a pendência de mercado como substituída, processa a frase de farmácia como nova operação e não reutiliza a pendência anterior.

### Fluxo com múltiplos candidatos de categoria

1. Usuário informa um lançamento que gera múltiplos candidatos plausíveis.
2. Agente apresenta opções persistíveis contendo raiz e subcategoria folha canônicas.
3. Cada opção possui internamente `rootCategoryId`, `rootSlug`, `subcategoryId` e `subcategorySlug`.
4. O usuário escolhe uma opção e o sistema revalida o par raiz + folha antes de persistir.

### Fluxo com cancelamento

1. Usuário inicia registro.
2. Agente pede clarificação.
3. Usuário: "cancela".
4. Agente encerra a pendência e confirma que nada foi registrado.

## Restrições Técnicas de Alto Nível

- A futura implementação deve seguir obrigatoriamente `go-implementation`, `mastra` e os princípios de Domain Modeling Made Functional definidos no repositório.
- O comportamento agentivo deve consumir o substrato existente `internal/platform/{agent,llm,memory,workflow,tool,scorer}` e implementar comportamento novo no consumidor `internal/agents`.
- Tools devem continuar sendo adapters finos: sem SQL direto, sem regra de negócio financeira e sem decisão categorial complexa dentro da tool.
- `internal/categories` deve permanecer como autoridade canônica para categorias, candidatos, ambiguidade, versionamento editorial e validação de escrita.
- `internal/transactions` deve permanecer como autoridade de persistência e validação final de transações/recorrências.
- Estados críticos de conversa pendente devem ser tipos fechados; string livre não pode governar transição crítica.
- Objetos com invariantes devem usar smart constructors e zero value inválido quando aplicável.
- Decisões de domínio devem ser puras quando possível, recebendo dados tipados e retornando estado/erro tipado, sem IO.
- A solução deve preservar idempotência de escrita financeira e auditabilidade por Thread/Run.
- A fonte oficial de aceite para retomada correta e ausência de confusão entre pendências é harness determinístico com Run auditável; scorers são complementares.
- Não é permitido criar fallback chain de LLM, provider paralelo ou chamada HTTP direta para LLM fora do provider oficial.
- Não é permitido relaxar o contrato categorial para ganhar fluidez; segurança de escrita prevalece sobre conveniência.

## Fora de Escopo

- Implementação de código, migrations, handlers, adapters, jobs, rotas ou desenho detalhado de storage neste PRD.
- Redesenho completo da taxonomia de categorias.
- Criação automática de novas categorias pelo usuário durante o registro.
- Recomendações financeiras, investimento, crédito, seguros, impostos complexos ou ações fora do domínio financeiro pessoal do MeControla.
- Substituição do contrato de categorias por julgamento do LLM.
- Mudança de canal além de WhatsApp/texto conversacional.
- Correção retroativa de conversas históricas já encerradas.
- Definição final de nomes de structs, campos, tabelas ou APIs, que pertence à especificação técnica.

## Critérios de Aceite

- **CA-01:** Dado um registro de despesa com valor, descrição, data e pix, quando a categoria exigir clarificação e o usuário responder "custo fixo", então o agente deve retomar a pendência original sem pedir valor ou pagamento novamente.
- **CA-02:** Dada uma pendência de categoria para "mercado", quando o usuário iniciar uma nova frase completa "Gastei R$ 150,00 na farmácia hoje, no pix", então o agente deve encerrar a pendência anterior como substituída e tratar a frase como nova operação explícita.
- **CA-03:** Dada uma pendência ativa, quando o usuário responder "sim e pix", então o agente deve preencher apenas os slots compatíveis com a pendência e pedir esclarecimento se "sim" não for uma confirmação válida naquele estado.
- **CA-04:** Dada uma categoria ambígua com múltiplos candidatos, quando o agente apresentar opções, então cada opção deve conter raiz canônica e subcategoria folha canônica com `id` e `slug`, e o sistema deve revalidar o par escolhido antes de persistir.
- **CA-05:** Dado cancelamento explícito, quando o usuário disser "cancela", então nenhuma escrita deve ocorrer depois desse turno.
- **CA-06:** Dada uma tool de escrita retornando erro, quando o agente responder, então a resposta não pode afirmar sucesso.
- **CA-07:** Dado replay idempotente da mesma operação, quando o usuário repetir a confirmação, então o sistema não deve duplicar a transação.
- **CA-08:** Dada expiração da pendência, quando o usuário responder tarde demais, então o agente deve explicar que precisa começar de novo.
- **CA-09:** Dado candidato de categoria raiz sem subcategoria folha, quando o usuário tentar registrar, então a escrita deve ser bloqueada e o agente deve pedir uma subcategoria válida.
- **CA-10:** Dado um fluxo de cartão de crédito sem cartão identificado, quando o usuário responde com o apelido do cartão, então o agente deve resolver o cartão e retomar o registro sem pedir novamente valor/descrição.
- **CA-11:** Dada uma pendência substituída por nova operação completa, quando o usuário responder depois com texto compatível com a pendência antiga, então nenhuma escrita da pendência antiga deve ocorrer.
- **CA-12:** Dado o harness determinístico de conversa, quando executar cenários de retomada, substituição e ambiguidade, então deve validar estado, tool calls, escrita real quando aplicável e Run auditável.

## Decisões Funcionais Fechadas

- **D-01:** O escopo inicial cobre registro de despesas/receitas e fluxos relacionados a categoria, pagamento, cartão e recorrência.
- **D-02:** Uma nova frase completa de lançamento durante uma pendência ativa substitui a pendência anterior e inicia nova operação explícita.
- **D-03:** A experiência deve manter uma única pendência ativa por thread para reduzir risco de aplicar resposta ao item errado.
- **D-04:** Toda escolha categorial persistível exige raiz canônica e subcategoria folha canônica, ambas com `id` e `slug`.
- **D-05:** O exemplo canônico de contrato categorial é raiz `66cb85a0-3266-5900-b8e3-13cdcd00ab62` + `custo-fixo` e subcategoria `c2fda6a3-c329-52c8-81ea-771b6ea4f365` + `aluguel`.
- **D-06:** A janela de expiração da pendência é de 30 minutos de inatividade; toda pendência expirada deve ter status fechado e resposta clara ao usuário.
- **D-07:** A fonte oficial para M-01 e M-06 é harness determinístico com verificação de estado, tool calls, escrita real quando aplicável e Run auditável.
- **D-08:** Scorers LLM-judged são permitidos apenas como observabilidade complementar, nunca como gate primário de aceite funcional.
- **D-09:** Não há questões de produto em aberto para este PRD; qualquer nova dúvida deve ser tratada como mudança de escopo com incremento de `spec-version`.
