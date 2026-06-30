# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 5 -->

# `MeControlaAgent` — Agente financeiro conversacional no WhatsApp (sucessor de `internal/agents`)

> **Histórico de decisões.** spec-version 2 fechou as questões em aberto da v1; spec-version 3 aprofundou a integração entre módulos; spec-version 4 fechou riscos de correção/segurança de escrita (D-19…D-24). **spec-version 5** ajusta a UX do onboarding e reafirma limites: distribuição das 5 categorias em **mensagem única** (RF-14), cadastro de cartão coletando **apenas apelido e vencimento** com defaults para campos obrigatórios do domínio (RF-15), recorrência de **12 meses** (RF-16.1) e parcelamento de **até 24 parcelas** (RF-24). Todas as decisões estão consolidadas em **Decisões de Produto (resolvidas)**.

## Visão Geral

O `mecontrola` é um monolito modular em Go cujo comportamento agentivo já vive como substrato genérico e reutilizável em `internal/platform` (agent, llm, memory, workflow). O consumidor atual em `internal/agents` é um **port do exemplo weather do Mastra** — útil para provar a plataforma end-to-end, mas sem valor de produto para o usuário final.

Este PRD define o produto **`MeControlaAgent`**: um **agente financeiro conversacional dentro do WhatsApp** que **substitui integralmente** o consumidor `internal/agents` atual. O weather-agent — e em especial o arquivo central `internal/agents/application/agents/agent.go` — é **legado a ser removido** na implementação futura. O weather serve **apenas como molde estrutural de referência** de como um consumidor Mastra-equivalente é montado sobre `internal/platform`; nunca como funcionalidade a preservar. Não há convivência entre weather-agent e `MeControlaAgent`: é uma sucessão completa.

O `MeControlaAgent` ajuda pessoas comuns a organizarem o dinheiro e realizarem objetivos de vida conversando em linguagem natural pelo WhatsApp. O valor central vendido é **realização de objetivos**: dinheiro é o meio, o objetivo é o destino. A promessa do produto é:

> **"Seu dinheiro organizado sem planilhas, sem complicação e direto no WhatsApp."**

O produto **não** é app bancário, sistema contábil, plataforma de investimentos, ERP financeiro, planilha financeira nem ferramenta para especialistas. Ele é um **parceiro financeiro** que acolhe, organiza e motiva, sem julgar e sem jargão.

As capacidades de negócio do agente **não são reimplementadas**: elas derivam **exclusivamente** dos módulos de domínio reais já existentes no workspace (`internal/categories`, `internal/card`, `internal/budgets`, `internal/transactions`). O agente é a camada conversacional que entende a intenção do usuário em linguagem natural e a traduz para essas capacidades, mantendo memória do objetivo do usuário e do histórico da conversa.

### Estado atual relevante (grounding)

- O substrato agentivo está em `internal/platform/agent`, `internal/platform/llm`, `internal/platform/memory` e `internal/platform/workflow` — primitivos genéricos (Thread, Run auditável, WorkingMemory, message history, tool, workflow/kernel, provider LLM) que o consumidor monta por DI.
- O consumidor atual `internal/agents` é o port weather, com pontos reais em `internal/agents/application/agents`, `internal/agents/application/scorers`, `internal/agents/application/workflows` e `internal/agents/module.go`. Toda referência ao weather-agent (incluindo `internal/agents/application/agents/agent.go`) é legado a substituir.
- O canal oficial de entrada e saída é o **WhatsApp**: mensagens de texto livre do usuário chegam ao runtime do agente e a resposta volta pelo gateway WhatsApp existente.
- Capacidades de domínio já implementadas e expostas (a serem consumidas como ferramentas do agente, nunca reescritas):
  - `internal/categories` — listar categorias, obter categoria por id, listar dicionário de categorias, buscar no dicionário. Rotas: `/api/v1/categories`, `/api/v1/categories/{id}`, `/api/v1/category-dictionary`, `/api/v1/category-dictionary/search`. **As 5 categorias da metodologia já existem como seed global** (Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira), com subcategorias; o **dicionário** mapeia termos humanos (ex.: "mercado") para categorias via busca exact/token/fuzzy com nível de confiança.
  - `internal/card` — criar, listar, obter, atualizar, atualizar limite, remover cartão e consultar faturas. Rotas em `/api/v1/cards`. Cada cartão exige nome, apelido (único por usuário), dia de fechamento e limite; dia de vencimento é opcional. **Não há limite de quantidade de cartões por usuário.**
  - `internal/budgets` — criar orçamento (rascunho), ativar orçamento, criar recorrência, registrar/editar/remover despesas, listar alertas e obter resumo mensal. Rotas em `/api/v1/budgets`. O orçamento é **mensal (por competência)** e tem **distribuição por categoria que deve fechar exatamente 100%** na ativação; alertas disparam por limiar de gasto (ex.: 80%/100%).
  - `internal/transactions` — transações (receita/despesa), compras no cartão (parcelamento de **1 a 24 parcelas**, com impacto automático nas competências futuras), templates recorrentes, resumo mensal e listagem de lançamentos do mês. Rotas em `/api/v1/transactions`, `/api/v1/card-purchases`, `/api/v1/recurring-templates` e `/api/v1/months/{ref_month}`. **Receita e despesa exigem categoria e meio de pagamento**; lançamentos são identificados por id e suportam soft-delete e edição versionada; **não há restrição temporal** de qual mês pode ser editado/removido.
- Identidade e ativação: o usuário precisa estar **ativado/identificado** (via `internal/identity` + `internal/onboarding`, magic token por WhatsApp) antes de conversar com o agente; o dispatcher roteia usuário desconhecido para a ativação e usuário conhecido para o agente. Mensagens de **mídia (áudio/imagem)** não são processadas hoje — apenas texto — e a resposta ao usuário é somente texto.

## Objetivos

- Entregar um produto que faça o usuário **sair do caos financeiro** (sensação de "o dinheiro some") para um estado de **dinheiro organizado e objetivo em vista**, sem planilhas e sem apps complexos, conversando pelo WhatsApp.
- Garantir que **todo novo usuário passe por um onboarding obrigatório de 8 etapas** que termina com um planejamento financeiro consolidado e confirmado (objetivo, orçamento, cartões, distribuição por categoria).
- Permitir **operação diária por linguagem natural**: registrar receitas e despesas, compras no cartão (à vista e parceladas), consultar resumos e planejamento, e editar/remover lançamentos com frases naturais.
- Manter **continuidade e coerência conversacional**: o agente lembra do objetivo financeiro do usuário (memória de longo prazo) e do histórico recente da conversa, nunca repetindo perguntas já respondidas.
- **Substituir integralmente** o weather-agent por `MeControlaAgent`, sem deixar referência, fluxo ou ativo órfão do exemplo weather.
- Reaproveitar **exclusivamente** as capacidades dos módulos de domínio existentes, sem duplicar regra de negócio na camada do agente.

### Métricas de sucesso (produto)

- **Taxa de conclusão do onboarding**: % de usuários que iniciam e chegam à ETAPA 8 com planejamento consolidado e confirmado.
- **Tempo até o primeiro planejamento consolidado**: mediana de tempo entre a primeira mensagem e a confirmação do plano no onboarding.
- **Adoção da operação diária**: % de usuários ativos que registram ao menos um lançamento por linguagem natural por semana após o onboarding.
- **Retenção do objetivo**: % de usuários cujo objetivo financeiro permanece definido e acessível ao agente ao longo do tempo (memória de longo prazo preservada).
- **Esforço por interação**: número médio de mensagens trocadas para completar uma ação (registrar despesa, consultar resumo); meta de minimizar interrogatório.

## Histórias de Usuário

Atores: **Usuário final WhatsApp** (persona principal); **Operador/Suporte** (acompanha jornada e saúde do agente).

Persona principal — homens e mulheres de 20 a 45 anos que sentem que o dinheiro desaparece durante o mês, não mantêm controle financeiro consistente, já abandonaram planilhas e apps complexos, e têm objetivos financeiros (quitar dívidas, viajar, comprar casa, comprar carro, construir reserva, organizar a vida) mas não conseguem acompanhá-los.

- Como **novo usuário**, quero ser acolhido e guiado passo a passo na configuração inicial, para que ao final eu tenha meu objetivo, meu orçamento e meu planejamento prontos sem precisar de planilha.
- Como **usuário**, quero declarar meu objetivo financeiro (ex.: "quero juntar para uma viagem") e que o agente o **lembre sempre**, para sentir que estou caminhando em direção a algo que importa.
- Como **usuário**, quero registrar uma receita ou despesa em linguagem natural (ex.: "recebi 3000 de salário", "gastei 80 no mercado"), para anotar gastos sem fricção, no momento em que acontecem.
- Como **usuário**, quero registrar uma compra no cartão, inclusive parcelada (ex.: "comprei uma TV de 2400 em 12x no Nubank"), para que o impacto apareça automaticamente nos meses futuros sem eu calcular nada.
- Como **usuário**, quando eu esquecer de dizer a forma de pagamento, quero que o agente **pergunte apenas isso**, sem refazer todas as perguntas.
- Como **usuário**, quero consultar como está meu mês e meu planejamento (ex.: "como tá meu mês?", "quanto sobrou em prazeres?"), para saber para onde meu dinheiro está indo.
- Como **usuário**, quero corrigir um engano falando naturalmente (ex.: "apaga aquele Uber", "muda o mercado de 80 pra 60"), para manter meus registros certos sem menus.
- Como **usuário**, quando eu tiver uma dúvida no meio do onboarding, quero que o agente responda e **volte exatamente para a etapa onde eu estava**, sem reiniciar nem me fazer repetir o que já falei.
- Como **operador**, quero que cada interação seja auditável (quem, quando, qual ação, qual resultado), para acompanhar a saúde do agente e a jornada do usuário sem expor dados sensíveis em métricas.

## Funcionalidades Core

1. **Produto `MeControlaAgent` no WhatsApp** — agente financeiro conversacional, parceiro do usuário, montado sobre `internal/platform` no lugar do weather-agent. Entende linguagem natural, conduz o onboarding e atende a operação diária, com identidade e tom de voz definidos. Substitui integralmente `internal/agents` (weather).

2. **Onboarding obrigatório de 8 etapas** — jornada inegociável que leva o usuário de "primeiro contato" a "planejamento consolidado e confirmado": boas-vindas → objetivo financeiro → orçamento mensal → cartões (cadastro ou recusa consciente) → metodologia das 5 categorias → distribuição por categoria → resumo final com confirmação/ajuste → conclusão com exemplos de uso diário. Uma etapa por vez, sempre com explicação, progresso e reforço de benefício.

3. **Operação diária por linguagem natural** — registro de receitas e despesas, compras no cartão à vista e parceladas (com impacto automático nas competências futuras), consultas de resumo mensal e planejamento, e edição/remoção de lançamentos por frases naturais. O agente pergunta apenas o que falta.

4. **Memória do usuário (working memory de longo prazo)** — o objetivo financeiro e o contexto de planejamento do usuário persistem como memória de longo prazo, acessível em toda interação para manter foco no destino (objetivo) e nunca pedir dado já fornecido.

5. **Histórico de conversa (message history)** — janela de histórico recente que sustenta conversa fluida e coerente, resolução de referências ("aquele Uber") e continuidade de fluxo (retomar a etapa do onboarding após uma dúvida).

6. **Ferramentas derivadas dos módulos de domínio reais** — as ações do agente são adaptadores finos sobre as capacidades existentes: categorias via `internal/categories`, orçamento via `internal/budgets`, cartões via `internal/card`, lançamentos e consultas via `internal/transactions`. Nenhuma regra de negócio nova é criada na camada do agente.

7. **Substituição completa do legado weather** — remoção total das referências ao weather-agent (incluindo `internal/agents/application/agents/agent.go`, scorers e workflows do exemplo weather), sem deixar ativo, fluxo ou wiring órfão.

## Requisitos Funcionais

### Produto, substituição e canal

- RF-01: O produto DEVE ser `MeControlaAgent`, um **agente financeiro conversacional cujo canal primário é o WhatsApp**, montado como consumidor sobre `internal/platform` (agent, llm, memory, workflow).
- RF-02: O `MeControlaAgent` DEVE **substituir integralmente** o consumidor `internal/agents` atual (weather-agent), sem convivência. Toda referência ao weather-agent, incluindo `internal/agents/application/agents/agent.go`, scorers e workflows do exemplo weather, DEVE ser tratada como legado a remover.
- RF-03: Após a entrega, **apenas o `MeControlaAgent` responde no WhatsApp**; não pode restar ativo, fluxo, rota, scorer, workflow ou wiring órfão do exemplo weather.
- RF-04: O weather-agent DEVE servir **apenas como molde estrutural de referência** de como um consumidor Mastra-equivalente é montado sobre a plataforma; nenhuma funcionalidade de clima é preservada.
- RF-05: O produto DEVE comunicar e operar sob a promessa "Seu dinheiro organizado sem planilhas, sem complicação e direto no WhatsApp", com foco em **realização de objetivos** (dinheiro é meio, objetivo é destino).

### Identidade, tom e comunicação

- RF-06: O agente DEVE assumir a identidade de **parceiro financeiro**: simples, claro, próximo, confiável e motivador; **nunca** julga, **nunca** culpa e **nunca** usa linguagem bancária, jurídica, agressiva ou fria.
- RF-07: O tom de voz DEVE ser simples, direto, amigável, leve, motivacional e profissional.
- RF-08: A comunicação DEVE seguir as regras: **uma pergunta por vez**; perguntar **apenas o que falta**; nunca pedir dado já fornecido; priorizar ação em vez de interrogatório; aceitar linguagem natural sem exigir comandos rígidos.
- RF-09: As respostas DEVEM ter boa clareza visual com **uso moderado** apenas dos emojis oficiais do produto: 🎯 objetivo, 💰 dinheiro, 💳 cartão, 📊 planejamento, 📈 receita, 📉 despesa, ✅ sucesso, ⚠️ atenção, 🚨 alerta crítico, 🔍 busca, 🗑️ exclusão, ✏️ edição, 🎓 conhecimento, 🎉 prazeres, 🏦 liberdade financeira.

### Onboarding obrigatório (inegociável)

- RF-10: O onboarding DEVE ter **8 etapas em ordem fixa**: (1) boas-vindas; (2) definição do objetivo financeiro; (3) definição do orçamento mensal; (4) cadastro ou recusa consciente de cartões; (5) apresentação da metodologia das 5 categorias; (6) distribuição monetária categoria por categoria; (7) resumo final com confirmação ou ajuste; (8) conclusão com exemplos de uso diário.
- RF-11: O onboarding é **obrigatório para todo usuário ainda não onboardado** e DEVE ser concluído antes de o usuário operar livremente no dia a dia; o agente **nunca pula etapas** e **nunca solicita tudo de uma vez**.
- RF-11.1: O onboarding financeiro de 8 etapas é responsabilidade **exclusiva do `MeControlaAgent`**. A **ativação/identificação de conta** (magic token por WhatsApp, hoje em `internal/identity` + `internal/onboarding`) é **etapa anterior e separada**, **não absorvida** pelo agente: o usuário chega ao agente já ativado, e o agente conduz a jornada financeira. O `MeControlaAgent` não trata ativação de conta.
- RF-12: Em cada etapa o agente DEVE **explicar a etapa atual**, **mostrar o progresso** e **reforçar o benefício**, sem pressionar o usuário.
- RF-13: A ETAPA 5 DEVE apresentar a metodologia das **5 categorias do produto**, que correspondem às categorias-raiz já existentes em `internal/categories`: 💰 Custo Fixo, 🎓 Conhecimento, 🎉 Prazeres, 🎯 Metas, 🏦 Liberdade Financeira.
- RF-13.1: Na **ETAPA 3**, o "orçamento mensal" coletado é a **renda mensal líquida** do usuário, que será **distribuída 100%** entre as 5 categorias na ETAPA 6 — incluindo Metas e Liberdade Financeira (poupança/objetivo). O valor "que sobra" da renda já é representado como alocação nessas categorias de poupança, não como resíduo não planejado.
- RF-14: A ETAPA 6 DEVE apresentar e coletar a **distribuição das 5 categorias em uma única mensagem** (todas de uma vez, não uma por vez), exibindo as 5 categorias e pedindo os percentuais/valores num só turno, coerente com a renda definida na ETAPA 3. A **distribuição entre as 5 categorias DEVE fechar exatamente 100%** (regra do domínio de orçamento). Se a soma não fechar, o agente DEVE sinalizar (⚠️) e reapresentar a distribuição completa para ajuste numa nova mensagem única, **sem concluir** com distribuição inconsistente. O orçamento é **consolidado com as alocações ao final da ETAPA 6** e **ativado na ETAPA 7**.
- RF-15: A ETAPA 4 DEVE permitir **cadastrar cartões um de cada vez** — coletando do usuário **apenas o apelido e o dia de vencimento** de cada cartão — e oferecendo "adicionar outro?" ao final, ou registrar a **ausência conscientemente confirmada** de cartão, sem forçar o cadastro e sem impor limite de quantidade.
- RF-15.2: Como o domínio de cartão exige campos além de apelido e vencimento (`Name` não-vazio, `ClosingDay` 1–31 obrigatórios; `LimitCents` ≥0; vencimento/`DueDay` opcional no domínio), o agente DEVE **preencher os obrigatórios não coletados com defaults**: `Name` := apelido; `LimitCents` := 0 (sem limite informado); `ClosingDay` **derivado do vencimento** por uma regra determinística (a definir na techspec). **Atenção**: `ClosingDay` influencia a competência das parcelas (ciclo de fatura); a regra de derivação é uma decisão técnica que afeta o parcelamento e DEVE ser explicitada na techspec.
- RF-15.1: **Reaproveitamento de estado pré-existente.** Para um usuário ainda não onboardado que já possua cartões ou orçamento (criados via API/app), o onboarding DEVE **rodar as 8 etapas reaproveitando o que existe**: na ETAPA 4, listar os cartões já cadastrados (sem duplicar) e oferecer usar/adicionar; na ETAPA 3/6, **reutilizar/ativar/ajustar** o orçamento da competência em vez de criar um duplicado (o orçamento é único por competência). Objetivo e distribuição são definidos normalmente.
- RF-16: A ETAPA 7 DEVE apresentar um **resumo final** do planejamento (objetivo, orçamento, cartões, distribuição) e permitir **confirmação ou ajuste** antes de concluir; a confirmação **ativa** o orçamento do mês.
- RF-16.1: Durante o onboarding (na definição/confirmação do orçamento), o agente DEVE **perguntar ao usuário se o orçamento e a distribuição devem se repetir automaticamente nos meses futuros** (recorrência). Se o usuário aceitar, o planejamento é **replicado para 12 meses** (ano completo — máximo permitido pelo domínio); caso contrário, vale apenas o mês corrente. O usuário pode reajustar meses futuros depois.
- RF-17: Ao final do onboarding, o usuário DEVE ter, de forma consolidada e confirmada: **objetivo financeiro definido** (memória de longo prazo), **orçamento mensal definido e ativo**, **cartão(ões) cadastrados ou ausência confirmada**, **distribuição financeira por categoria criada (fechando 100%)** e **planejamento consolidado e confirmado** (com a escolha de recorrência registrada).
- RF-18: Se o usuário fizer uma **dúvida no meio do onboarding**, o agente DEVE respondê-la e **retornar exatamente à etapa em andamento**, **sem reiniciar** o onboarding e sem repetir perguntas já respondidas.
- RF-19: O onboarding DEVE preservar o estado de progresso de forma robusta e durável entre mensagens, de modo que o usuário possa pausar e **retomar exatamente na etapa em que parou** quando voltar — inclusive após dias de inatividade. (Sem reinício e sem perda do que já informou.)
- RF-19.1: Se o usuário **abandonar o onboarding no meio**, a retomada acontece **quando ele enviar nova mensagem** (retoma na etapa exata). **Lembrete proativo de abandono está fora do escopo do MVP.**

### Operação diária por linguagem natural

- RF-20: O agente DEVE **registrar receitas** declaradas em linguagem natural (ex.: "recebi 3000 de salário"), atribuindo uma **categoria de receita** (já existem categorias income, ex.: salário, renda variável, investimentos). Receitas são registradas como entradas (📈) **à parte**: elas **não consomem** as 5 categorias de despesa e **não entram** no cálculo de planejado×gasto do orçamento.
- RF-21: O agente DEVE **registrar despesas** declaradas em linguagem natural de forma completa (valor, descrição, categoria e meio de pagamento). A **categoria DEVE ser inferida** a partir do texto usando o **dicionário de categorias** de `internal/categories` (busca exact/token/fuzzy); em caso de baixa confiança ou ambiguidade, o agente confirma a categoria com o usuário.
- RF-21.1: **Data do lançamento.** Quando o usuário não informar a data, o agente DEVE assumir **a data de hoje** (o campo de data é obrigatório no domínio). Quando o usuário citar a data em linguagem natural ("ontem", "dia 5", "semana passada"), o agente DEVE usar a data indicada. Não interrogar a data quando ela puder ser assumida como hoje.
- RF-21.2: **Múltiplos lançamentos por mensagem.** Quando o usuário declarar mais de um lançamento numa única mensagem (ex.: "gastei 80 no mercado e 30 no uber"), o agente DEVE **registrar cada um** (chamadas sequenciais, pois não há operação em lote) e **devolver um resumo consolidado** (✅) do que foi registrado. Cada escrita respeita a garantia de idempotência (RF-38).
- RF-22: Quando faltar **apenas o meio de pagamento** para concluir um lançamento, o agente DEVE perguntar **somente isso**, sem refazer as demais perguntas. (Meio de pagamento é obrigatório no domínio de transações.)
- RF-23: O agente DEVE suportar **compra no cartão**, associando o lançamento ao cartão indicado pelo usuário (resolvido por nome/apelido do cartão).
- RF-24: O agente DEVE suportar **compra parcelada em até 24 parcelas** (1 a 24, máximo permitido pelo domínio), com **impacto automático nas competências (meses) futuras** conforme o ciclo de fatura do cartão, sem exigir cálculo manual do usuário. Pedidos acima de 24 parcelas DEVEM ser tratados com mensagem clara de limite (⚠️).
- RF-25: O agente DEVE responder **consultas de resumo mensal e de planejamento** (ex.: "como tá meu mês?", "quanto sobrou em prazeres?"), incluindo planejado × gasto por categoria e alertas de limiar.
- RF-25.1: O agente DEVE permitir **reajustar a distribuição por categoria em linguagem natural** após o onboarding (ex.: "põe 20% em prazeres"). O ajuste **rebalanceia** as demais categorias para fechar 100% (regra do domínio, válido sobre orçamento ativo); o agente confirma o resultado com o usuário.
- RF-25.2: **Fronteira de ferramentas (anti-dupla-contagem).** Todo **lançamento** (receita, despesa, compra no cartão à vista ou parcelada) é registrado **exclusivamente via `internal/transactions`**. O agente **não** registra despesas diretamente em `internal/budgets`; o resumo mensal do orçamento, o "spent" por categoria e os alertas de limiar são **atualizados automaticamente** pela integração já existente entre os módulos (eventos de transações → orçamento), de forma idempotente. O agente usa `internal/budgets` apenas para **planejamento**: criar/ativar orçamento, recorrência e ajuste de distribuição.
- RF-26: O agente DEVE permitir **remoção e edição de lançamentos** a partir de pedidos em linguagem natural (ex.: "apaga aquele Uber", "muda o mercado de 80 pra 60"). A resolução da referência DEVE usar o **histórico recente** e, por padrão, a **listagem do mês corrente**; se o usuário citar outro mês, a busca DEVE se ampliar para a competência indicada.
- RF-27: **Operações destrutivas exigem confirmação humana explícita** (o domínio não oferece proteção nem confirmação; deletes são imediatos). Antes de **remover/editar lançamento, remover compra parcelada ou remover cartão**, o agente DEVE confirmar o alvo (🔍) e só efetivar após "sim" explícito (🗑️). O agente DEVE **avisar o impacto**: ao remover uma **compra parcelada**, deixar claro que **todas as parcelas, em todos os meses**, serão removidas; ao remover um **cartão**, alertar (⚠️) se houver **parcelas em aberto** que ficariam órfãs. Referência ambígua (mais de um candidato) → o agente DEVE pedir desambiguação antes de agir.

### Memória, histórico e persistência

- RF-28: O usuário tem **um objetivo financeiro principal** (MVP), persistido como **memória de longo prazo (working memory)** escopada ao usuário e disponível ao agente em todas as interações, mantendo o foco no objetivo. O usuário pode **trocar** o objetivo depois; o produto não mantém múltiplos objetivos simultâneos no MVP.
- RF-29: A jornada DEVE usar **message history**. O produto exige uma **janela de histórico recente suficiente para conversa fluida e coerente** — proposta de **~20 mensagens recentes (~10 turnos)**, alinhada ao default atual da plataforma — justificada para resolver referências ("aquele Uber"), reter a etapa corrente do onboarding ao responder dúvidas e evitar repetir perguntas já respondidas. O objetivo de longo prazo **não** depende dessa janela (vive na working memory).
- RF-30: A persistência de memória e histórico DEVE ser **robusta e eficiente**, **sem perder o objetivo do cliente** ao longo do tempo, mesmo entre sessões e reinícios.
- RF-30.1: O agente DEVE saber se o usuário **já concluiu o onboarding** para decidir entre conduzir a jornada de 8 etapas ou operar o dia a dia. Esse estado é **derivado de sinais já existentes** (orçamento ativo do usuário + objetivo registrado na working memory), sem exigir uma nova fonte de verdade dedicada.

### Integrações obrigatórias com módulos de domínio

- RF-31: As ferramentas do agente DEVEM ser **derivadas exclusivamente dos módulos de domínio reais existentes**; o agente **não cria regra de negócio nova** nem duplica cálculo de domínio.
- RF-32: Tudo que envolver **categorias** (listar, obter por id, dicionário, busca no dicionário) DEVE usar `internal/categories`.
- RF-33: **Criação, ativação e consulta de orçamento, recorrências, despesas derivadas, alertas e resumo mensal de orçamento** DEVEM usar `internal/budgets`.
- RF-34: **Cadastro, listagem, consulta, atualização, limite, remoção de cartões e consulta de faturas** DEVEM usar `internal/card`.
- RF-35: **Lançamentos financeiros e consultas de lançamento** (transações, compras no cartão, templates recorrentes, resumo mensal de transações e listagem do mês) DEVEM usar `internal/transactions`. **Despesas e parcelas registradas em `internal/transactions` propagam-se automaticamente para o orçamento** (resumo e alertas) via a integração já existente entre os módulos — o agente não duplica esse registro.
- RF-36: O produto DEVE respeitar a dependência conceitual de que **`internal/budgets` e `internal/transactions` apoiam-se nas categorias de `internal/categories`**; o agente não reconcilia categorias por conta própria fora dessas fronteiras. A inferência de categoria a partir do texto usa o **dicionário de categorias** (que cobre termos de despesa e de receita).

### Auditabilidade

- RF-37: Cada interação relevante (etapa de onboarding concluída, lançamento criado/editado/removido, consulta atendida) DEVE ser **auditável** (ação, momento e resultado), sem expor dados pessoais sensíveis em métricas de alta cardinalidade.

### Robustez e qualidade de escrita

- RF-38: **Escrita exatamente-uma-vez por intenção do usuário.** Todo lançamento (receita, despesa, compra à vista/parcelada) DEVE ser **idempotente** em relação à intenção que o originou: retries do agente, loops de tool dentro de um mesmo Run e reentregas do canal **não podem** criar lançamento duplicado. A garantia DEVE ser ancorada em uma chave estável derivada da mensagem/decisão do usuário (ex.: identificador do inbound/Run auditável). É um requisito de **correção financeira inegociável**; o mecanismo concreto é responsabilidade da techspec. (Contexto: a idempotência atual do create só persiste respostas de erro, não de sucesso — logo a garantia precisa ser explicitada e implementada.)
- RF-39: **Avaliação de qualidade (scorers).** O produto DEVE incluir, já no MVP, um **conjunto mínimo de scorers/evals** sobre as interações do agente — por exemplo: seleção correta de ferramenta/intenção, categorização plausível e coerência do lançamento com a mensagem — com **amostragem configurável** e resultados persistidos, aproveitando o primitivo de scorer da plataforma. Serve como prova contínua de qualidade ("production-proof"), sem bloquear o caminho principal da conversa.

## Experiência do Usuário

A experiência acontece **inteiramente no WhatsApp**, em texto livre, como uma conversa com um parceiro que entende de dinheiro e nunca complica.

### Jornada de onboarding (primeira experiência)

O usuário é acolhido (ETAPA 1) e convidado a falar do que quer conquistar (ETAPA 2 — 🎯 objetivo). O agente reforça que o objetivo é o destino e o dinheiro é o meio. Em seguida, uma pergunta de cada vez, define-se o orçamento mensal (ETAPA 3 — 💰), os cartões ou a ausência consciente deles (ETAPA 4 — 💳), e apresenta-se a metodologia das 5 categorias (ETAPA 5 — 📊): 💰 Custo Fixo, 🎓 Conhecimento, 🎉 Prazeres, 🎯 Metas, 🏦 Liberdade Financeira. A distribuição é feita categoria por categoria (ETAPA 6), respeitando o orçamento. O agente mostra um resumo final para confirmação ou ajuste (ETAPA 7 — ✅) e conclui ensinando como usar no dia a dia (ETAPA 8 — 🎉). A cada passo: explica a etapa, mostra progresso, reforça o benefício e nunca pressiona. Dúvidas no meio do caminho são respondidas e o fluxo retoma na etapa exata, sem reinício.

### Operação diária (uso recorrente)

Depois do onboarding, o usuário simplesmente fala: "gastei 80 no mercado" (📉), "recebi 3000 de salário" (📈), "comprei uma TV de 2400 em 12x no cartão" (💳, parcelado com impacto automático nos meses futuros). Quando falta só a forma de pagamento, o agente pergunta apenas isso. Consultas como "como tá meu mês?" ou "quanto sobrou em prazeres?" (🔍/📊) trazem resumos claros. Correções acontecem por linguagem natural: "apaga aquele Uber" (🗑️) ou "muda o mercado de 80 pra 60" (✏️), sempre com confirmação do item-alvo. O agente nunca repete o que já sabe e mantém o objetivo do usuário em vista, motivando o progresso (🏦).

### Princípios de UX inegociáveis

- Uma pergunta por vez; ação antes de interrogatório.
- Clareza visual com uso moderado dos emojis oficiais; nunca poluir a resposta.
- Tom sempre acolhedor e motivador; jamais julgar, culpar ou usar jargão.
- Continuidade: o agente lembra do objetivo (longo prazo) e do contexto recente (histórico).

## Restrições Técnicas de Alto Nível

- **Canal primário**: WhatsApp como canal de entrada e saída do produto. Mensagens de texto livre do usuário são a interface; respostas voltam pelo gateway WhatsApp existente.
- **Substrato agentivo**: o `MeControlaAgent` é um **consumidor** de `internal/platform` (agent, llm, memory, workflow) e **não reimplementa** mecanismos da plataforma. O weather-agent atual em `internal/agents` é o **molde estrutural** a seguir e o legado a remover.
- **Integrações de domínio obrigatórias** (tratadas como fronteiras de alto nível, sem desenho de API nesta fase): `internal/categories`, `internal/card`, `internal/budgets`, `internal/transactions`. As ações do agente são adaptadores finos sobre essas capacidades; nenhuma regra de negócio nova é introduzida na camada do agente.
- **Dependência conceitual de categorias**: `internal/budgets` e `internal/transactions` apoiam-se nas categorias de `internal/categories`; o produto preserva essa fronteira.
- **Identificação prévia obrigatória**: o usuário precisa estar **ativado/identificado** (magic token por WhatsApp, via `internal/identity` + `internal/onboarding`) antes de falar com o agente. O `MeControlaAgent` recebe a identidade pronta e **não** trata ativação de conta; assume usuário já estabelecido.
- **Canal somente texto (MVP)**: o inbound do WhatsApp hoje processa apenas texto (mídia é descartada) e a resposta é apenas texto. O `MeControlaAgent` opera em **texto puro**; recebimento de áudio/imagem (ex.: foto de comprovante) está **fora de escopo** e exigiria evolução do canal.
- **Onboarding como fluxo durável**: o onboarding de 8 etapas é modelado sobre o mecanismo de estado durável (suspende/retoma) já existente na plataforma, garantindo retomada na etapa exata entre mensagens e após dias de inatividade.
- **Distribuição fecha 100%**: a distribuição por categoria do orçamento é validada pelo domínio para somar exatamente 100% na ativação; o produto não conclui onboarding com distribuição inconsistente.
- **Parcelamento limitado a 1–24**: compras parceladas respeitam o limite de 24 parcelas do domínio de transações.
- **Memória de longo prazo**: o objetivo financeiro (único, MVP) e o contexto de planejamento do usuário persistem como working memory escopada ao usuário, acessível em toda interação.
- **Message history — janela proposta**: propõe-se uma janela de **histórico recente equivalente às últimas ~20 mensagens (aprox. 10 turnos), alinhada ao default atual da plataforma**, como requisito de produto. Justificativa: é suficiente para (a) resolver referências como "aquele Uber" e "muda o mercado de 80 pra 60"; (b) reter a etapa corrente do onboarding ao responder uma dúvida intermediária; (c) evitar repetir perguntas já respondidas dentro de uma mesma sessão; mantendo a conversa fluida e coerente sem inflar custo/latência de contexto. O objetivo de longo prazo **não** depende dessa janela — ele vive na working memory e persiste além do histórico recente. O valor exato é calibrável na techspec, mas o produto exige que seja suficiente para os três comportamentos acima.
- **Persistência robusta e eficiente**: memória e histórico devem sobreviver a sessões e reinícios sem perder o objetivo do cliente; o estado de progresso do onboarding deve ser preservado de forma confiável entre mensagens.
- **Auditabilidade e privacidade**: cada interação relevante é um **Run auditável**; métricas não carregam identificadores de alta cardinalidade nem dados pessoais sensíveis.
- **Não negociável de produto**: o onboarding de 8 etapas não pode ser flexibilizado, pulado ou reiniciado por dúvida intermediária.

## Fora de Escopo

- **Qualquer funcionalidade do exemplo weather** (clima, geocoding, sugestão de atividades): será removida, não preservada nem estendida.
- **App bancário, sistema contábil, ERP financeiro, plataforma de investimentos, planilha financeira ou ferramenta para especialistas**: o produto é um parceiro conversacional simples, não nenhuma dessas categorias.
- **Movimentação real de dinheiro / integração bancária (Open Finance, transferências, pagamentos)**: o agente organiza e planeja; não movimenta fundos.
- **Ativação/identificação de conta**: tratada por `internal/identity` + `internal/onboarding` (magic token), **fora** do `MeControlaAgent`.
- **Mídia no inbound (áudio, imagem de comprovante) e respostas com mídia**: o canal hoje é texto puro; suportar mídia exigiria evolução do canal e está fora de escopo no MVP.
- **Lembrete proativo de abandono de onboarding**: a retomada ocorre quando o usuário volta a escrever; nudges proativos ficam fora do MVP.
- **Múltiplos objetivos financeiros simultâneos**: o MVP mantém um objetivo principal por usuário.
- **Novos módulos de domínio ou novas regras de negócio financeiras**: todas as capacidades derivam dos módulos existentes (`categories`, `card`, `budgets`, `transactions`); este PRD não cria capacidade de domínio nova.
- **Desenho técnico**: APIs, structs, tabelas, interfaces Go, wiring, migrations, esquema de prompts, escolha de modelo LLM e formato de tools são responsabilidade da techspec, não deste PRD.
- **Canais além do WhatsApp** (app próprio, web, e-mail, voz): fora de escopo nesta entrega.
- **Relatórios analíticos avançados, exportações, dashboards para o usuário final** além das consultas conversacionais de resumo e planejamento.
- **Aconselhamento financeiro regulado / recomendação de produtos financeiros**: o agente motiva e organiza, não presta consultoria de investimento.

## Decisões de Produto (resolvidas)

Todas as questões em aberto da v1 foram fechadas, confrontadas com o código real. As decisões abaixo são vinculantes para a techspec.

| # | Decisão | Resolução | Fundamento no código |
|---|---------|-----------|----------------------|
| D-01 | Relação com a ativação de conta (`internal/onboarding`) | **Ativação permanece separada**; o agente só conduz as 8 etapas financeiras (RF-11.1). | Dispatcher já roteia usuário desconhecido → ativação (magic token) e conhecido → agente. |
| D-02 | Recorrência do orçamento ao concluir onboarding | **Perguntar ao usuário** durante o onboarding se o planejamento deve repetir nos meses futuros (RF-16.1). | `internal/budgets` possui `create_recurrence` (clona orçamento+alocações para N meses). |
| D-03 | Janela de edição/remoção por linguagem natural | **Mês corrente por padrão**, ampliando se o usuário citar outro mês; sempre confirmar o alvo (RF-26/RF-27). | `internal/transactions` não impõe restrição temporal; listagem por `/months/{ref_month}` retorna id+descrição. |
| D-04 | Quantidade de objetivos financeiros | **Um objetivo principal por usuário** (MVP), na working memory (RF-28). | Working memory é um documento único por usuário (escopo resourceID); não existe entidade "objetivo". |
| D-05 | Cadastro de cartões na ETAPA 4 | **Um cartão por vez**, coletando do usuário **apenas apelido + vencimento**; demais campos por default (RF-15/RF-15.2). | `create_card.go:18-35` exige `Name` e `ClosingDay`; `LimitCents`≥0; `DueDay` opcional → defaults: Name=apelido, Limit=0, ClosingDay derivado do vencimento (techspec). |
| D-25b | Distribuição (ETAPA 6) | **Mensagem única** com as 5 categorias de uma vez (não uma por vez) (RF-14). | Regra de fechar 100% inalterada (`budget.go:132-150`); muda só a forma de coleta. |
| D-06 | Abandono de onboarding | **Retomar na etapa exata ao voltar**; sem lembrete proativo no MVP (RF-19/RF-19.1). | Kernel de workflow suporta suspend/resume durável; sem nudge outbound wired ao agente. |
| D-07 | Distribuição por categoria | **Deve fechar 100%**; estouro/sobra → ajuste guiado (RF-14). | Domínio de orçamento valida soma de alocações = 100% na ativação. |
| D-08 | Limite de cartões | **Sem limite imposto pelo agente**; respeita o módulo (RF-15). | `internal/card` não tem teto de quantidade. |
| D-09 | Mídia no canal | **Texto puro no MVP**; mídia fora de escopo. | Inbound descarta mídia hoje; gateway só envia texto. |
| D-10 | Parcelamento | **1 a 24 parcelas** (RF-24). | Domínio de transações valida `installments in 1..24`. |
| D-11 | Sinal de "onboarding concluído" | **Derivado** de orçamento ativo + objetivo na working memory (RF-30.1). | Estados já existentes (budget Draft→Active; WM por usuário). |
| D-12 | Inferência de categoria na operação diária | Usar o **dicionário de categorias** (busca exact/token/fuzzy); confirmar quando ambíguo (RF-21). | `internal/categories` expõe `category-dictionary/search` com confiança. |
| D-13 | Fronteira de ferramentas lançamento × orçamento | Lançamentos **só via `internal/transactions`**; orçamento (resumo/alertas) atualiza **automaticamente**; agente nunca chama `upsert_expense` direto (RF-25.2/RF-35). | Consumers `transaction.created.v1` / `card_purchase.created.v1` em `budgets/module.go:134-149` alimentam `budgets_expenses` com idempotência + unique constraint (zero dupla contagem). |
| D-14 | Semântica do "orçamento mensal" (ETAPA 3) | Coletar a **renda mensal líquida**, distribuída 100% entre as 5 categorias (incl. poupança) (RF-13.1). | `budget.totalCents` é distribuído 100% via basis points; Metas e Liberdade são `allocation_type=asset_allocation`. |
| D-15 | Horizonte de recorrência | **12 meses** (ano completo) quando o usuário aceita (RF-16.1). | `create_recurrence` aceita 1–12 meses como parâmetro. |
| D-16 | Ajuste de distribuição pós-onboarding | **Permitido por conversa**, com rebalanceamento (RF-25.1). | `edit_category_percentage` opera em orçamento ativo e rebalanceia para 100%. |
| D-17 | Tratamento de receitas no resumo | Registradas **à parte** (📈); não consomem as 5 categorias de despesa (RF-20). | Consumer do orçamento ignora income de propósito; "spent" vem só de `budgets_expenses` (outcome). |
| D-18 | Categorização de receita | Existem **categorias income seed** + termos no dicionário; sem gap (RF-20). | `migrations/000002:141-192` (salário, renda-variável, etc.) e dicionário com termos income. |
| D-19 | Idempotência de escrita | **Exatamente-uma-vez por intenção** como requisito (RF-38). | Middleware de idempotência só persiste 4xx, não 2xx (`idempotency/middleware.go:136-154`) → garantia precisa ser explícita; `soft_delete_card` já persiste sucesso (referência correta). |
| D-20 | Data não informada | **Assumir hoje**; usar data citada quando houver (RF-21.1). | `OccurredAt`/`PurchasedAt` obrigatórios, sem default no domínio (`raw_create_transaction.go:36-38`). |
| D-21 | Operações destrutivas | **Sempre confirmar + avisar impacto** (RF-27). | Sem HITL no domínio; `soft_delete_card` não valida parcelas em aberto (órfãos); `delete_card_purchase` remove todas as parcelas (`delete_card_purchase.go:76-104`). |
| D-22 | Múltiplos lançamentos por mensagem | **Registrar cada um** (sequencial) + resumo (RF-21.2). | Não há endpoint de batch; uma transação por chamada (`transactions_router.go:96-110`). |
| D-23 | Onboarding com estado pré-existente | **Rodar 8 etapas reaproveitando** cartões/orçamento existentes (RF-15.1). | `list_cards` existe; orçamento é único por competência (reutilizar/ativar em vez de duplicar). |
| D-24 | Scorers/evals no MVP | **Conjunto mínimo de scorers** como requisito de qualidade (RF-39). | Primitivo de scorer da plataforma já exercitado pelo weather (code-based + LLM-judged). |

## Suposições Remanescentes

- As capacidades atuais dos módulos `categories`, `card`, `budgets` e `transactions` cobrem as ações exigidas pelo onboarding e pela operação diária. Eventuais consultas específicas ainda não expostas (ex.: um filtro de listagem) serão tratadas como **dependência pontual a confirmar na techspec**, não como nova regra de negócio no agente.
- O substrato `internal/platform` (Thread/Run auditável, working memory, message history, tools, workflow durável, structured output) já oferece os primitivos necessários — hoje exercitados pelo weather-agent — e o `MeControlaAgent` os reaproveita sem reimplementar mecanismo.
- O valor final da janela de histórico (~20 como ponto de partida) é parâmetro de calibração da techspec, sem alterar os comportamentos exigidos por este PRD.
- A identidade do usuário chega ao agente já resolvida (resourceID), permitindo escopar working memory, threads e Runs por usuário.
