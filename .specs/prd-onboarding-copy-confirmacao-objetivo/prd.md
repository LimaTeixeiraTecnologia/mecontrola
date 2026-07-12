# Documento de Requisitos do Produto (PRD) — Onboarding: Boas-vindas, Confirmação do Objetivo, Emoji de Cartão, Sucesso de Cartão e Objetivo Único no Resumo

<!-- spec-version: 3 -->

> Origem: `docs/us/2026-07-12-us-onboarding-copy-confirmacao-cartao-resumo.md` (US única, validada por `scripts/validar-historias-usuario.py`, confrontada com o codebase).
> Data: 2026-07-12.
> Base de código: `internal/agents/application/workflows/onboarding_workflow.go` (workflow durável de onboarding, consumidor Mastra sobre `internal/platform/{agent,workflow,memory}`), entregue por `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`; fluxo avulso de cartão em `internal/agents/application/workflows/card_create_confirm_workflow.go`; normalizador de saída em `internal/platform/whatsapp/formatting/normalize.go`.
> Escopo deliberadamente enxuto: 5 refinamentos de copy/UX do onboarding conversacional + regra de emoji de cartão aplicada aos 2 fluxos determinísticos de cartão. PRD **independente**, sem misturar assuntos com os PRDs já entregues; a relação de supersession com eles está explícita na seção "Relação com PRDs Existentes".
> Decisões confirmadas (múltipla escolha, 2026-07-12): reforço do objetivo **determinístico** (eco do objetivo, sem LLM); confirmação+reforço **prefixados** mantendo a pergunta opcional do valor da meta; boas-vindas por **supersession cirúrgico do fragmento** de exemplo do RF-03 de `prd-onboarding-sem-friccao`; mensagem pós-cadastro de cartão no onboarding = **selo "💳 Cartão registrado com sucesso ✅" + "Quer registrar algum outro?"**; exemplo de formato de valor **alinhado a "R$ 5.000,00"/"5 mil"**; layout de cartão em **bullets**.
> Decisão de emoji de cartão (mandatória e inegociável, 2026-07-12): em cada um dos dois fluxos determinísticos de cartão (onboarding e avulso `card_create_confirm`), o emoji 💳 aparece **apenas** (a) na primeira mensagem de cartão do fluxo, na primeira menção, e (b) no selo de sucesso do cadastro; **proibido** em qualquer outra mensagem/ocorrência. Escopo confirmado: **onboarding + avulso apenas**; o system prompt do agente (`mecontrola_agent.go`), as tools de cartão e os golden cases **não** são alterados.

## Visão Geral

Esta funcionalidade refina a copy e a montagem de mensagens do onboarding conversacional do MeControla no WhatsApp em cinco pontos observados na jornada real, e aplica uma regra mandatória de uso do emoji 💳 aos dois fluxos determinísticos de cartão (onboarding e o fluxo avulso de cadastro), sem alterar o motor de workflow, as regras de orçamento/recorrência, a extração via LLM, o system prompt do agente ou a idempotência de escrita. As cinco melhorias são: (1) usar um exemplo de meta mais acessível na mensagem de boas-vindas ("comprar um celular novo, meta de R$ 5.000,00" no lugar de "comprar uma casa, meta de R$ 400.000,00"); (2) fazer com que a mensagem seguinte à captura do objetivo confirme o objetivo do usuário e traga um reforço positivo relacionado a ele, de forma determinística, antes de manter a pergunta opcional do valor da meta; (3) organizar as mensagens de cartão em blocos legíveis (bullets) e aplicar a regra de emoji 💳; (4) apresentar uma confirmação de sucesso ao registrar um cartão; e (5) citar o objetivo uma única vez no "Resumo de Onboarding" da conclusão.

O valor entregue é um onboarding mais acolhedor e menos ruidoso e uma linguagem de cartão consistente: o usuário vê um exemplo de meta próximo do dia a dia, sente-se ouvido logo após declarar seu objetivo (confirmação + incentivo), recebe mensagens de cartão organizadas e sem poluição de emoji repetido, uma confirmação clara de que o cartão foi registrado, e encerra a jornada com um resumo sem repetição do objetivo. Todas as mudanças são de copy e de montagem de mensagem no consumidor `internal/agents`, preservando integralmente o motor de workflow e as regras de negócio já entregues.

## Objetivos

- Boas-vindas mais acessível: 100% das primeiras mensagens de onboarding exibem o exemplo "comprar um celular novo, meta de R$ 5.000,00" e 0% exibem "comprar uma casa", mantendo íntegro todo o restante do texto travado (asserts de substring existentes permanecem verdes).
- Confirmação e reforço do objetivo: 100% das mensagens seguintes à captura do objetivo confirmam o objetivo ecoando o texto do usuário e incluem um reforço positivo relacionado a ele, com 0 novas call-sites de LLM introduzidas (a confirmação/reforço é determinística).
- Emoji de cartão sob controle: em cada fluxo determinístico de cartão (onboarding e avulso), o emoji 💳 aparece no máximo 2 vezes — a primeira menção da primeira mensagem de cartão e o selo de sucesso do cadastro; 0 mensagens repetem 💳 a cada ocorrência da palavra "cartão" (hoje há 18 ocorrências de 💳 no onboarding e 12 no avulso).
- Mensagens de cartão organizadas: 100% das mensagens de cartão do onboarding ficam organizadas em blocos legíveis (bullets), com 0 regressão nos fragmentos obrigatórios não-emoji já entregues (exemplo com e sem apelido, dia em ambos os formatos, nota de apelido herdando o banco).
- Confirmação de sucesso de cartão: 100% dos cadastros de cartão bem-sucedidos no onboarding emitem "💳 Cartão registrado com sucesso ✅" seguido de "Quer registrar algum outro?".
- Objetivo único no resumo: 0 ocorrências do objetivo repetido na frase de conclusão; o objetivo aparece exatamente 1 vez, no cabeçalho do "Resumo de Onboarding".
- Zero regressão e prova de qualidade: semântica de `workflow_steps_total{workflow="onboarding-workflow"}`, criação de cartão, ativação de orçamento, recorrência e idempotência permanecem inalteradas; aceite por testes unitários e de integração determinísticos verdes cobrindo os comportamentos, com os asserts de copy atualizados, e o gate golden real-LLM agregado por categoria (`CategoryOnboarding`) mantido ≥ 0,90.

## Histórias de Usuário

- Como usuário novo no onboarding do WhatsApp, quero que o exemplo de meta seja algo próximo do meu dia a dia (um celular novo), para entender rapidamente o que devo responder.
- Como usuário que acabou de declarar meu objetivo, quero que a resposta confirme o objetivo que informei e me incentive, para sentir que fui ouvido antes de continuar.
- Como usuário na etapa de cartão, quero mensagens organizadas em blocos e sem emoji repetido, para ler com clareza o que é opcional, o exemplo e como pular.
- Como usuário que acabou de cadastrar um cartão, quero uma confirmação de que deu certo, para ter segurança de que foi registrado antes de decidir cadastrar outro.
- Como usuário concluindo o onboarding, quero ver meu objetivo uma única vez no resumo, para não achar a mensagem repetitiva.

## Funcionalidades Core

1. **Exemplo de meta acessível nas boas-vindas** — a primeira mensagem troca o exemplo "comprar uma casa, meta de R$ 400.000,00" por "comprar um celular novo, meta de R$ 5.000,00", preservando todo o restante do texto.
2. **Confirmação do objetivo com reforço positivo (determinístico)** — a mensagem que segue a captura do objetivo confirma o objetivo ecoando o texto exato do usuário, adiciona um reforço positivo e mantém a pergunta opcional do valor da meta (com exemplo de formato alinhado a R$ 5.000,00).
3. **Mensagens de cartão organizadas com emoji sob controle** — convite inicial, reprompts e convite ao próximo cartão reorganizados em bullets; o emoji 💳 aparece apenas na primeira menção da primeira mensagem de cartão do fluxo (mais o selo de sucesso), nunca repetido.
4. **Confirmação de sucesso ao registrar cartão** — após criar um cartão no onboarding, a próxima mensagem é "💳 Cartão registrado com sucesso ✅" seguida de "Quer registrar algum outro?".
5. **Objetivo único no resumo de conclusão** — no "Resumo de Onboarding", o objetivo aparece só no cabeçalho; a frase de conclusão deixa de repeti-lo, preservando a chamada de ação final.

## Requisitos Funcionais

Boas-vindas (item 1):
- RF-01: A primeira mensagem do onboarding deve substituir exclusivamente o fragmento de exemplo "comprar uma casa, meta de R$ 400.000,00" por "comprar um celular novo, meta de R$ 5.000,00", preservando integralmente todas as demais linhas, emojis e quebras de linha do texto de boas-vindas.
- RF-02: Nenhuma outra parte da primeira mensagem pode ser alterada; as substrings já asseguradas por `prd-onboarding-sem-friccao` ("🎉 Bem-vindo ao MeControla! 🎉" e "Vamos começar? Qual é o seu principal objetivo financeiro para este mês?") devem permanecer presentes e íntegras.

Confirmação e reforço do objetivo (item 2):
- RF-03: Após o usuário informar o objetivo, a mensagem seguinte deve, na mesma mensagem e nesta ordem: (a) confirmar o objetivo ecoando o texto exato informado pelo usuário; (b) apresentar um reforço positivo relacionado a esse objetivo; (c) manter a pergunta opcional do valor da meta, incluindo a saída explícita para seguir sem informar valor.
- RF-04: A confirmação e o reforço do RF-03 devem ser gerados de forma determinística, por template que interpola o objetivo do usuário, sem introduzir nenhuma nova chamada de LLM no fluxo.
- RF-05: O comportamento de coleta do valor da meta (opcionalidade, controle de "já perguntou o valor", conversão de valor informado) não pode regredir; a nova mensagem apenas acrescenta a confirmação e o reforço antes da pergunta opcional já existente.
- RF-06: O exemplo de formato de valor exibido na pergunta opcional do valor da meta (hoje "R$ 400.000,00"/"400 mil") deve ser alinhado a "R$ 5.000,00"/"5 mil", para coerência com o novo exemplo de boas-vindas.

Emoji de cartão — mandatório e inegociável (item 3):
- RF-07: Em cada um dos dois fluxos determinísticos de cartão — o onboarding (`onboarding_workflow.go`) e o avulso (`card_create_confirm_workflow.go`) —, o emoji 💳 deve aparecer apenas em duas situações, e em nenhuma outra: (a) na primeira mensagem de cartão do fluxo, na primeira menção; e (b) no selo de sucesso do cadastro. É proibido acompanhar o emoji 💳 a qualquer outra mensagem/ocorrência de cartão — reprompts, convite ao próximo cartão, cancelamento, erros, idempotência, seção de cartões do "Resumo de Onboarding" ou qualquer outra frase. Este requisito supersede as decisões de copy de emoji de `prd-onboarding-cartao-resumo-conclusao` (RF-01/RF-02/RF-03) e de `prd-cadastro-conversacional-cartao` na parte de repetição do emoji.
- RF-08: O escopo da regra de emoji da RF-07 é estritamente os dois fluxos determinísticos citados. O system prompt do agente (`mecontrola_agent.go`), as tools de cartão (`create_card`, `resolve_card`, `register_expense`, `update_card`, etc.), o fluxo `pending_entry`, o `destructive_confirm` e os golden cases não são alterados por esta funcionalidade.

Mensagens de cartão organizadas (item 3):
- RF-09: As mensagens de cartão do onboarding — convite inicial, reprompts de dados faltantes e convite ao próximo cartão — devem ser reorganizadas em blocos legíveis no estilo bullets, separando a intenção, os exemplos e a saída "não", com o emoji 💳 posicionado conforme RF-07.
- RF-10: A reorganização de layout deve preservar integralmente os fragmentos obrigatórios não-emoji já entregues em `prd-onboarding-cartao-resumo-conclusao`: o exemplo com apelido e o exemplo sem apelido; o dia de vencimento em ambos os formatos ("dia 1" e "dia primeiro"); e a comunicação de que, sem apelido, o apelido do cartão herda o nome do banco. Nenhum desses fragmentos pode ser removido pela mudança de layout ou pela regra de emoji.

Confirmação de sucesso ao registrar cartão (item 4):
- RF-11: Após a criação bem-sucedida de um cartão no onboarding, a próxima mensagem deve ser, nesta ordem: a linha de selo de sucesso exatamente "💳 Cartão registrado com sucesso ✅" seguida da pergunta "Quer registrar algum outro?". Este selo de sucesso é a aparição autorizada pela RF-07(b) do emoji 💳 no fluxo de onboarding.
- RF-12: A confirmação de sucesso do RF-11 ocorre exclusivamente após um cadastro concluído na sessão atual. O convite inicial da etapa de cartão — inclusive em sessão retomada com cartões pré-existentes, antes de o usuário cadastrar um cartão nesta interação — não usa a linha de sucesso e mantém o texto/contagem vigentes (por exemplo, o informativo de quantidade de cartões já existentes), aplicando a regra de emoji da RF-07.

Objetivo único no resumo (item 5):
- RF-13: No "Resumo de Onboarding" apresentado na conclusão, o objetivo financeiro deve aparecer uma única vez, no cabeçalho do resumo (com o valor da meta quando houver).
- RF-14: A frase de conclusão que segue o resumo não pode repetir o objetivo financeiro; deve preservar a chamada de ação final (orientação para o usuário enviar gastos e receitas no dia a dia). Todo o restante do "Resumo de Onboarding" travado em `prd-onboarding-cartao-resumo-conclusao` (conteúdo do resumo, título "Resumo de Onboarding" prefixado por 📊 no normalizador, formatação BRL, rótulos de categoria, lista de cartões, recorrência) permanece inalterado, exceto pela aplicação da regra de emoji da RF-07 à seção de cartões.

Fluxo avulso e não regressão (item 4 estendido + prova):
- RF-15: No fluxo conversacional avulso `card_create_confirm_workflow.go`, a regra de emoji da RF-07 deve ser aplicada: o emoji 💳 permanece apenas na primeira mensagem do fluxo (a pergunta de confirmação) e no selo de sucesso do cadastro ("✅ 💳 *<apelido>* cadastrado com sucesso."); as demais mensagens (reprompt de ambiguidade, cancelamento, erros de domínio, erros de infraestrutura, idempotência "já estava cadastrado") deixam de usar 💳. O fluxo permanece single-shot (sem laço de "cadastrar outro cartão").
- RF-16: Todas as mudanças devem se restringir a copy e à montagem de mensagem no consumidor `internal/agents`, sem alterar a ordem das etapas do onboarding, os gatilhos de suspend/resume, os schemas e system prompts de extração via LLM, a criação de cartão, a ativação de orçamento, a recorrência ou a idempotência de escrita.
- RF-17: O aceite exige testes unitários e de integração determinísticos verdes cobrindo os comportamentos (exemplo de boas-vindas, confirmação+reforço do objetivo, cartão em bullets com emoji sob controle nos dois fluxos, sucesso de cartão + "Quer registrar algum outro?", objetivo único no resumo), com os asserts de copy afetados atualizados; e o gate golden real-LLM agregado (`CategoryOnboarding`, threshold ≥ 0,90) mantido verde. Nenhum comportamento pode ser declarado pronto apenas com mock quando houver caminho real coberto por teste de integração.

## Experiência do Usuário

- Persona primária: usuário recém-ativado fazendo onboarding pelo WhatsApp, na primeira jornada de configuração (objetivo, orçamento, distribuição, cartões, conclusão); e o mesmo usuário cadastrando um cartão avulso após o onboarding.
- Fluxo de boas-vindas e objetivo: o usuário recebe a saudação com o exemplo de meta "comprar um celular novo, meta de R$ 5.000,00"; ao responder o objetivo, recebe uma mensagem que confirma o objetivo declarado, o incentiva e ainda pergunta, de forma opcional, o valor da meta (com exemplo de formato "R$ 5.000,00"/"5 mil" e a saída "não").
- Fluxo de cartão (onboarding): a primeira mensagem de cartão traz o emoji 💳 (uma vez, na primeira menção) e o convite/exemplos em bullets; os reprompts e o convite ao próximo cartão vêm sem 💳; ao cadastrar um cartão com sucesso, o usuário lê "💳 Cartão registrado com sucesso ✅" seguido de "Quer registrar algum outro?"; ao recusar, segue para o resumo e a conclusão.
- Fluxo de cartão (avulso): a pergunta de confirmação inicial traz 💳; o selo de sucesso traz 💳; reprompt, cancelamento, erros e idempotência não trazem 💳.
- Fluxo de conclusão: o usuário recebe o "Resumo de Onboarding" (título prefixado por 📊 pelo normalizador) com o objetivo no cabeçalho e uma frase de conclusão que não repete o objetivo, mas mantém a orientação de começar a registrar gastos e receitas; a seção de cartões do resumo não usa 💳.
- Considerações de copy: negrito no WhatsApp é expresso por asterisco (o normalizador converte `**` em `*`); a palavra "outro" permanece destacada quando presente no convite inicial já travado; o emoji 💳 obedece à RF-07; valores monetários seguem o formato BRL já usado; a confirmação e o reforço do objetivo são determinísticos e ecoam o texto do usuário.

## Restrições Técnicas de Alto Nível

- O motor genérico de workflow (`internal/platform/workflow`) não pode ser alterado (governança R-WF-KERNEL-001); as mudanças ficam no consumidor `internal/agents` e nas constantes/funções de montagem de mensagem.
- Governança de adaptador e consumidor: código Go alterado sem comentários (R-ADAPTER-001.1); comportamento novo no consumidor sem `switch case intent.Kind` (R-AGENT-WF-001); LLM apenas nas call-sites já sancionadas. A confirmação/reforço do objetivo (RF-03/RF-04) é determinística e não adiciona nenhuma call-site de LLM.
- Modelagem de domínio (DMMF): a lógica determinística de confirmação/reforço do objetivo é um cálculo puro (sem IO, sem `context.Context`), preservando a pureza dos passos de decisão do onboarding.
- Fonte de verdade inalterada: objetivo/meta/orçamento/distribuição/recorrência vêm do estado durável do onboarding; cartões vêm da interface de cartões já injetada; nenhuma nova tabela, migração de schema, campo de domínio, estado/enum ou emoji é introduzido.
- Canal e entrega: WhatsApp Meta como canal único; a entrega usa o caminho existente `Suspend.Prompt → usecase → consumer.sendReply → NormalizeOutboundText → gateway`; o normalizador (`internal/platform/whatsapp/formatting/normalize.go`) permanece responsável por `** → *`, prefixo 📊 em "Resumo de Onboarding" e ✅ na confirmação de orçamento.
- Sensibilidade de dados: todas as mensagens exibem apenas dados do próprio usuário em sua própria thread; nenhuma exposição cross-usuário; nenhum dado sensível novo é coletado.

## Relação com PRDs Existentes

Este PRD é independente e não mistura assuntos; ele explicita como se relaciona com o que já foi entregue:

- `prd-onboarding-sem-friccao-ate-primeiro-lancamento` (entregue): o RF-03 daquele PRD trava o texto exato da primeira mensagem, incluindo o exemplo "comprar uma casa, meta de R$ 400.000,00". Este PRD **supersede exclusivamente o fragmento de exemplo** (RF-01/RF-02 aqui), preservando todo o restante do texto e as substrings asseguradas.
- `prd-onboarding-cartao-resumo-conclusao` (entregue): este PRD **supersede o RF-01/RF-02/RF-03** daquele PRD (que exigiam a palavra "cartão" acompanhada de 💳 em toda mensagem de cartão), substituindo-os pela regra de emoji da RF-07. Os fragmentos **não-emoji** de linguagem e exemplo de cartão daquele PRD são **preservados** (RF-10 aqui). A mensagem pós-cadastro do onboarding **supersede o convite do RF-04** daquele PRD, usando "💳 Cartão registrado com sucesso ✅" + "Quer registrar algum outro?" (RF-11 aqui). A frase de conclusão daquele PRD é **refinada de forma compatível** (RF-13/RF-14 aqui). O título "Resumo de Onboarding" e o prefixo 📊 do normalizador permanecem intactos.
- `prd-cadastro-conversacional-cartao` (entregue): este PRD **refina a copy de emoji** do fluxo avulso `card_create_confirm_workflow.go` para obedecer à RF-07, sem alterar a lógica de confirmação, idempotência, TTL, reprompt único ou o caráter single-shot do fluxo.
- Este PRD não altera o system prompt do agente (`mecontrola_agent.go`), as tools de cartão, o `pending_entry`, o `destructive_confirm`, os golden cases nem nenhuma regra de orçamento, distribuição, ativação, recorrência ou idempotência.

## Fora de Escopo

- Aplicação da regra de emoji 💳 fora dos dois fluxos determinísticos citados: o system prompt do agente (`mecontrola_agent.go`, 45× 💳), as tools de cartão, o `pending_entry_workflow`, o `destructive_confirm_workflow` e os golden cases (`cases_card.go`) permanecem inalterados.
- Personalização do reforço do objetivo via LLM (decidido: reforço determinístico, sem LLM).
- Remoção da pergunta opcional do valor da meta (decidido: mantida/prefixada pela confirmação e reforço).
- Adição de um laço "cadastrar outro cartão" ao fluxo avulso `card_create_confirm_workflow.go` (permanece single-shot).
- Alteração da ordem das etapas do onboarding, dos gatilhos de suspend/resume ou da lógica de extração via LLM (schemas e system prompts de extração inalterados).
- Alteração do objetivo exibido em outras mensagens que não a conclusão (ex.: a mensagem de revisão de distribuição de orçamento `summaryPrompt`), que é distinta do "Resumo de Onboarding".
- Alteração dos prompts de sistema do extrator (exemplos internos de conversão), que não são mensagens ao usuário.
- Mudanças no motor de workflow (`internal/platform/workflow`) ou nos primitivos de plataforma (`internal/platform/{agent,memory}`).
- Introdução de novas tabelas, migrações, campos de domínio, estados/enum, emojis, frequências ou regras de negócio.
- Persistência do resumo/conclusão como mensagem no histórico da thread e reprocessamento retroativo para usuários que já concluíram o onboarding.
- Publicação da funcionalidade ou de seus itens em Jira, Azure DevOps ou GitHub Issues.

## Suposições e Questões em Aberto

Nenhuma questão em aberto. Todas as decisões materiais foram confirmadas com o solicitante e refletem confronto com o codebase:

- Boas-vindas: supersession cirúrgico do fragmento de exemplo do RF-03 de `prd-onboarding-sem-friccao` (RF-01, RF-02).
- Confirmação + reforço do objetivo: determinístico (eco do objetivo, sem LLM), prefixado, mantendo a pergunta opcional do valor da meta, com o exemplo de formato alinhado a R$ 5.000,00 (RF-03, RF-04, RF-05, RF-06).
- Emoji de cartão (mandatório e inegociável): 💳 apenas na primeira mensagem de cartão de cada fluxo (onboarding e avulso) e no selo de sucesso; proibido em qualquer outra ocorrência; escopo restrito aos 2 fluxos determinísticos (RF-07, RF-08).
- Mensagens de cartão em bullets preservando os fragmentos obrigatórios não-emoji já entregues (RF-09, RF-10).
- Sucesso de cartão no onboarding: selo "💳 Cartão registrado com sucesso ✅" + "Quer registrar algum outro?", apenas após cadastro concluído; convite inicial mantém o texto/contagem vigentes ajustado à regra de emoji (RF-11, RF-12).
- Objetivo único no resumo, com refino compatível da frase de conclusão (RF-13, RF-14).
- Fluxo avulso: regra de emoji aplicada mantendo 💳 na confirmação inicial e no selo de sucesso, sem laço de novo cartão (RF-15).
- Não regressão restrita a copy/montagem de mensagem e prova por testes determinísticos + gate golden agregado (RF-16, RF-17).
