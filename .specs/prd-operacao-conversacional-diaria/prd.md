# Documento de Requisitos do Produto (PRD) — Operação Conversacional Diária do MeControla

<!-- spec-version: 1 -->

## Visão Geral

O MeControla é um parceiro financeiro pessoal que opera integralmente por conversa no WhatsApp. Após concluir o onboarding, o assinante passa a viver o "dia a dia": registra despesas e receitas, corrige lançamentos, ajusta orçamento e distribuição, gerencia cartões, revê seu objetivo e consulta resumos — sempre em linguagem natural.

Hoje a camada conversacional do dia a dia (`internal/agents`) cobre parte dos fluxos, mas diverge do documento oficial de comportamento e Tom de Voz: as confirmações e mensagens de sucesso saem em linha única, sem frase motivacional; não existe alterar o valor total do orçamento, alterar o objetivo depois do onboarding, cancelamento de plano conversacional nem suporte; e a edição de lançamento não busca candidatos nem permite trocar categoria ou forma de pagamento pela ferramenta. O prompt é monolítico e as mensagens ficam espalhadas em geradores divergentes.

Este PRD define a reescrita completa da experiência conversacional do dia a dia, eliminando o legado dessa camada e reconstruindo os 13 fluxos do documento oficial com fidelidade verbatim ao Tom de Voz, confirmação explícita antes de qualquer gravação e roteamento por registry de ferramentas e workflows. A reescrita preserva integralmente os primitivos de plataforma e os módulos de domínio; mudanças de domínio, quando necessárias, são estritamente aditivas.

Fonte de comportamento: `docs/us/us-operacao-conversacional-diaria.md` (história de usuário única, já validada) e o documento de produto `US_Operacao_Conversacional_Diaria_MeControla_Versao_Final.md`.

## Objetivos

Sucesso é medido por fidelidade e confiabilidade da operação conversacional, não por engajamento bruto:

- **Confirmação universal**: 100% das operações de escrita são precedidas de confirmação explícita do usuário; nenhuma gravação ocorre sem confirmação positiva.
- **Zero falso-sucesso**: a métrica de falso-sucesso (`agents_pending_entry_false_success_total` e equivalentes por workflow) permanece em zero em produção; nenhuma mensagem de sucesso é emitida sem recurso persistido.
- **Resolução correta de intenção e categoria**: a taxa de resolução correta de intenção e de categoria/subcategoria nos 13 fluxos fica igual ou acima de 0,90 por fluxo na suíte de jornada (golden), com resolução automática de subcategoria para a categoria raiz.
- **Aderência verbatim ao Tom de Voz**: os blocos de confirmação, sucesso, esclarecimento e resumo dos 13 fluxos seguem a estrutura e os campos do documento oficial, verificados por scorers dedicados; cada operação de escrita conclui com frase motivacional coerente com o cenário.
- **Gate de liberação real-LLM**: o release exige a suíte de jornada com LLM real cobrindo os 13 fluxos com score igual ou superior a 0,90 em cada fluxo e zero falso-sucesso, além dos gates de governança do repositório.

## Histórias de Usuário

Persona primária: assinante ativo do MeControla que já concluiu o onboarding e interage exclusivamente pelo WhatsApp.

- Como assinante ativo, quero registrar um gasto escrevendo naturalmente (por exemplo "Comprei bloco no depósito foi 300 conto"), para lançar minhas despesas sem preencher formulário, confirmando antes de gravar.
- Como assinante ativo, quero registrar uma receita ("Recebi meu salário, entrou 2 mil"), para acompanhar minhas entradas com origem identificada.
- Como assinante ativo, quero corrigir um lançamento errado ("Corrige aquele mercado, era 25" ou "Muda a categoria"), para manter meus registros corretos, escolhendo entre candidatos quando houver mais de um.
- Como assinante ativo, quero alterar o valor total do meu orçamento mensal quando minha renda muda, para replanejar sem redistribuir tudo manualmente.
- Como assinante ativo, quero mudar a distribuição/percentuais das minhas categorias, para reorganizar meu planejamento.
- Como assinante ativo, quero cadastrar, editar e excluir cartões por conversa, para gerenciar meus meios de pagamento.
- Como assinante ativo, quero atualizar meu objetivo financeiro quando meu sonho muda, para manter minha motivação alinhada.
- Como assinante ativo, quero saber como cancelar meu plano e como obter suporte, para resolver questões administrativas sem sair do WhatsApp.
- Como assinante ativo, quero consultar o resumo de uma categoria e o panorama geral do meu orçamento, para entender minha situação financeira do mês.

## Funcionalidades Core

- **Registro conversacional de despesas e receitas**: interpreta valor, estabelecimento/origem, categoria e forma de pagamento em linguagem natural; solicita apenas o campo ausente; confirma antes de gravar; conclui com sucesso motivacional. Importante porque é o uso mais frequente e a base do controle financeiro.
- **Edição conversacional de lançamentos**: busca lançamentos compatíveis do período, lista candidatos ou apresenta o único encontrado, e permite alterar valor, categoria/subcategoria, forma de pagamento, descrição e data, com confirmação. Importante para manter os dados corretos.
- **Gestão de orçamento e distribuição**: altera o valor total do orçamento preservando a distribuição em proporção, e altera os percentuais por categoria. Importante para manter o planejamento aderente à realidade do usuário.
- **Gestão de cartões**: cadastra, edita e exclui cartões, com confirmação universal e aviso de impacto na exclusão. Importante para meios de pagamento e faturas.
- **Objetivo financeiro**: consulta e atualiza o objetivo mantido como memória de trabalho do usuário. Importante para a camada motivacional da marca.
- **Respostas informacionais**: cancelamento de plano (passo a passo oficial da Kiwify) e suporte (e-mail com prazo de resposta), servidos com texto fixo verbatim. Importantes para questões administrativas.
- **Consultas de resumo**: resumo por categoria (com lançamentos detalhados, planejado, gasto e disponível/excedente) e resumo geral do orçamento (categorias e consolidado), com conclusão contextualizada. Importantes para visibilidade financeira.
- **Fidelidade de Tom de Voz e confiabilidade**: mensagens determinísticas verbatim com frase motivacional rotacionada, confirmação universal antes de escrever, idempotência e guarda anti-falso-sucesso. Importantes para a identidade da marca e para a confiança no produto.

## Requisitos Funcionais

Requisitos transversais:

- RF-01: O agente compreende linguagem natural livre para registrar, editar, consultar e gerenciar, cobrindo as variações de intenção do documento oficial, sem exigir formato fixo do usuário.
- RF-02: O agente reconhece valores monetários em formatos numéricos, por extenso e gírias (por exemplo `10`, `dez`, `10 conto`, `mil`, `R$ 1.000`, `duzentos`, `dois mil`), convertendo para centavos inteiros; valores menores ou iguais a zero são rejeitados na validação determinística.
- RF-03: A taxonomia de forma de pagamento é estendida de forma aditiva para cobrir carteiras digitais (Apple Pay, Google Pay, PicPay, Mercado Pago), cheque, DOC e transferência, além das já suportadas; menções a "Cartão <Banco>" resolvem para um cartão do usuário por apelido, com forma de pagamento crédito ou débito conforme a fala.
- RF-04: Cada operação de escrita apresenta um resumo e aguarda confirmação explícita do usuário antes de gravar; nenhuma gravação ocorre sem confirmação positiva.
- RF-05: As mensagens de confirmação, sucesso, esclarecimento e as respostas informacionais são geradas pelo sistema com estrutura e campos verbatim do documento oficial; o LLM apenas repassa o texto sem parafrasear; a frase motivacional de fechamento é sorteada de uma lista fixa por cenário; os emojis e a formatação seguem o documento.
- RF-06: Toda escrita é idempotente por identificador de mensagem do WhatsApp e sequência de item, reutilizando o ledger durável existente; uma mesma mensagem nunca gera gravação duplicada.
- RF-07: Uma confirmação positiva que não resulte em recurso persistido é tratada como falha, incrementa a métrica de falso-sucesso e não devolve mensagem de sucesso.
- RF-08: O roteamento resolve ferramentas e workflows por registry; é proibido decidir o fluxo por ramificação de intenção de domínio; estados de fronteira são tipos fechados enumerados, nunca string livre.
- RF-09: Havendo um registro pendente aguardando confirmação, o agente exige concluir ou cancelar esse pendente antes de iniciar um novo, sem gravação parcial.
- RF-10: O estado de espera é persistido no snapshot do kernel antes de a pergunta de confirmação ser enviada; a retomada aplica delta por merge-patch antes de qualquer novo parse; há expiração por TTL avaliada na retomada, teto de reprompt em respostas ambíguas e limpeza determinística do estado.
- RF-11: Quando faltar apenas um campo obrigatório, o agente solicita somente o campo ausente e nunca repergunta informação já identificada.
- RF-12: Quando a mensagem contém mais de um lançamento, o agente bloqueia e orienta o usuário a enviar um de cada vez, preservando a confirmação individual e a idempotência.

Requisitos por fluxo:

- RF-13: Registro de despesa — identifica intenção, valor, estabelecimento (descrição literal), categoria e forma de pagamento quando informada; resolve categoria automaticamente, lista candidatos quando ambíguo e reclassifica por direção quando incompatível; apresenta o bloco de confirmação de despesa e, após confirmação, persiste e responde com sucesso motivacional.
- RF-14: Registro de receita — identifica valor, origem e tipo, sem perguntar forma de pagamento; apresenta o bloco de entrada com origem e, após confirmação, persiste e responde com sucesso motivacional.
- RF-15: Edição de despesa e receita — busca lançamentos compatíveis do período por valor, categoria, descrição e recência; lista candidatos quando houver mais de um e apresenta o único encontrado; permite alterar valor, categoria/subcategoria, forma de pagamento, descrição e data, respeitando o guard de migração de forma de pagamento do domínio; confirma antes de atualizar.
- RF-16: Recorrência — criar, listar, alterar e excluir recorrências permanece disponível como variante de registro, com o mesmo Tom de Voz, confirmação e idempotência dos demais fluxos.
- RF-17: Alteração do valor total do orçamento — busca o orçamento ativo, pergunta o novo valor mensal, reescala proporcionalmente os percentuais atuais para o novo total, apresenta o resumo e persiste após confirmação, sem obrigar o usuário a redistribuir manualmente.
- RF-18: Alteração da distribuição financeira — busca a distribuição atual, exibe os percentuais por categoria, solicita a nova distribuição, apresenta o resumo e persiste após confirmação; percentuais inválidos são rejeitados na validação determinística.
- RF-19: Cadastro de cartão — pergunta apelido e dia de vencimento (e banco quando ajudar a resolução), apresenta o resumo e persiste após confirmação, encerrando com frase motivacional.
- RF-20: Edição de cartão — busca o cartão, apresenta os dados atuais, solicita a alteração (vencimento, apelido, banco), apresenta o resumo e persiste após confirmação; toda alteração de cartão exige confirmação explícita, inclusive apelido e banco.
- RF-21: Exclusão de cartão — passa pelo gate destrutivo com confirmação e aviso de impacto (parcelas em aberto) e só efetiva após confirmação positiva.
- RF-22: Alteração do objetivo financeiro — o objetivo permanece como texto em memória de trabalho por usuário; o agente busca o objetivo atual, pergunta o novo, apresenta o resumo e reescreve a memória de trabalho após confirmação, encerrando com frase motivacional, sem criar agregado de domínio.
- RF-23: Cancelamento do plano — resposta informacional determinística verbatim, servida por ferramenta de leitura estática, com o passo a passo oficial da Kiwify e fechamento acolhedor; a resposta apenas informa e não altera o estado da assinatura nem chama a Kiwify ou o billing.
- RF-24: Suporte — resposta informacional determinística verbatim, servida por ferramenta de leitura estática, orientando o envio de e-mail para `contato@limateixeira.com.br` com prazo de resposta de até 24 horas.
- RF-25: Resumo por categoria — ferramenta de leitura dedicada retorna os lançamentos do período filtrados pela categoria (resolvendo subcategoria para a categoria raiz quando o usuário cita apenas a subcategoria), com data, valor e subcategoria por lançamento, além de planejado, gasto e disponível ou excedente; o bloco segue a estrutura verbatim do documento e conclui com frase motivacional coerente com o cenário (disponível, próximo do limite, exatamente no limite, ultrapassado).
- RF-26: Resumo geral do orçamento — exibe cada categoria com planejado, gasto e disponível, e o consolidado (total planejado, total gasto, total disponível), concluindo com mensagem contextualizada ao cenário (positivo, atenção, crítico), na estrutura verbatim do documento.

Requisitos de reescrita, corte e liberação:

- RF-27: A camada conversacional do dia a dia é substituída integralmente (eliminação do legado dessa camada), preservando os primitivos de plataforma e os módulos de domínio; mudanças de domínio limitam-se a adições (novo usecase de alteração de total de orçamento, novo valor de enum de forma de pagamento, extensão da ferramenta de edição, ferramenta de detalhe por categoria, ferramentas informacionais e usecase de leitura/reescrita do objetivo).
- RF-28: No corte de ativação do novo fluxo, os runs de workflow suspensos existentes em produção são drenados por uma janela de graça: concluem ou expiram pelo TTL antes da desativação do legado, enquanto novos inbounds já entram no novo fluxo; nenhuma confirmação em aberto é perdida abruptamente.
- RF-29: A liberação exige a suíte de jornada com LLM real cobrindo os 13 fluxos com score igual ou superior a 0,90 em cada fluxo e zero falso-sucesso, além dos gates de governança do repositório (adaptadores finos e zero comentários, validação de input DTO, padrão de testes canônico, e o padrão de agente/workflow/kernel). Os invariantes binários (confirmação prévia em 100% das escritas e falso-sucesso igual a zero) são gates absolutos avaliados à parte do score de jornada.
- RF-30: O produto acompanha e reporta os KPIs de sucesso: percentual de escritas com confirmação prévia (meta 100%), contagem de falso-sucesso (meta zero), taxa de resolução correta de intenção e categoria (acima da meta na jornada) e aderência verbatim ao Tom de Voz (scorers).

Requisitos de acesso:

- RF-31: Operações de escrita exigem usuário autenticado com principal válido no contexto e recusam sem ele; onboarding concluído é pré-condição do fluxo do dia a dia.

## Experiência do Usuário

- Canal único WhatsApp, idioma português do Brasil, formatação com asterisco simples para negrito e emojis do documento oficial (por exemplo ✅ 💰 💳 📂 📥 📊 ⚠️ 🚨 🎉 💚).
- Registro de despesa: bloco de confirmação `✅ Encontrei este lançamento:` com `💰 Valor`, `💳 Pagamento`, `📂 Categoria` e a pergunta `Posso registrar?`; sucesso `Prontinho! ✅` com frase motivacional.
- Registro de receita: bloco `✅ Encontrei esta entrada:` com `💰 Valor` e `📥 Origem`; sucesso `Boa notícia! 🎉` com frase motivacional.
- Edição: apresenta valor anterior, categoria e forma de pagamento e o novo valor, e pergunta `Posso atualizar?`; lista de opções quando houver múltiplos candidatos.
- Resumo por categoria e resumo geral: blocos com os campos e a ordem do documento, concluídos por mensagem motivacional coerente com o cenário financeiro.
- Confirmação universal: qualquer operação que grava dados só é efetivada após confirmação explícita; respostas ambíguas seguem política de reprompt única antes de cancelar; registros pendentes bloqueiam novos até conclusão ou cancelamento.
- Respostas informacionais (cancelamento e suporte) entregam o texto oficial fixo, sem efeito colateral.

## Restrições Técnicas de Alto Nível

- Preservar os primitivos de plataforma (`internal/platform/{agent,workflow,memory,llm,scorer}`) e os módulos de domínio (`internal/{transactions,budgets,card,categories,identity,billing}`); a reescrita atua apenas na camada conversacional (`internal/agents`).
- Provider de LLM único (OpenRouter); LLM apenas nas call-sites sancionadas (loop de tool-calling do agente, passo que gera texto e scorer com julgamento por LLM); proibido LLM no kernel de workflow e dentro de ferramentas de domínio.
- Governança obrigatória e não negociável: padrão de agente, workflow e kernel (roteamento por registry, ferramenta fina sem regra/SQL/branching de domínio, estados fechados, run auditável, HITL com estado de espera persistido antes da confirmação e retomada por merge-patch); adaptadores finos e zero comentários em Go de produção; validação de input DTO na fronteira; padrão canônico de testes.
- Idempotência de escrita por identificador de mensagem e sequência de item; guarda anti-falso-sucesso obrigatória.
- Modelagem de domínio com estado como tipo (enums fechados) para estados ilegais irrepresentáveis; validação em smart constructors; funções de decisão puras onde aplicável.
- Cardinalidade de métricas controlada: labels restritos a enums fechados; proibido usar identificadores de usuário, chave de correlação ou identificador de categoria como label.
- A resposta de cancelamento de plano é informacional e não integra com a API da Kiwify; a efetivação de cancelamento continua pelo webhook de billing já existente.

## Fora de Escopo

- Reescrita de qualquer primitivo de plataforma ou de qualquer agregado, função de decisão ou smart constructor existente nos módulos de domínio.
- Reescrita do fluxo de onboarding (este PRD cobre apenas o dia a dia pós-onboarding).
- Integração ativa com a API da Kiwify para cancelar assinatura pelo chat (cancelamento permanece informacional).
- Criação de agregado de domínio para objetivo/meta (permanece em memória de trabalho).
- Fila multi-turn para múltiplos lançamentos em uma única mensagem (permanece bloqueio com orientação de um de cada vez).
- Novos canais além do WhatsApp e novos idiomas.
- Métricas de engajamento (DAU, retenção) como critério de sucesso desta entrega.

## Suposições e Questões em Aberto

- Nenhuma suposição e nenhuma questão em aberto. As 12 decisões de escopo e desenho da história de usuário, as 3 decisões de produto (KPIs, gate de release e corte do legado) e o limiar definitivo do gate de jornada real-LLM (igual ou superior a 0,90 por fluxo) foram confirmados com o solicitante.
