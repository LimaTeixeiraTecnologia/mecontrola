# Documento de Requisitos do Produto (PRD) — Onboarding: Cartão por Extenso, Exemplo de Cadastro e Resumo/Conclusão Final

<!-- spec-version: 1 -->

> Origem: `docs/us/2026-07-12-us-onboarding-cartao-resumo-conclusao.md` (US única, confrontada com codebase e com produção).
> Data: 2026-07-12.
> Base de código: `internal/agents/application/workflows/onboarding_workflow.go` (workflow durável de onboarding, consumidor Mastra sobre `internal/platform/{agent,workflow,memory}`), entregue por `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`.
> Evidência de produção: run `5eeaf6e8-8c50-461b-b2ce-442f7d7105cf` do usuário `8b0eeabb-293b-4837-a5a2-8418c63fce94` (WhatsApp +5511986896322), status `succeeded`, coletada por SSH em `root@187.77.45.48` (Postgres `workflow_runs`/`platform_messages`/`cards`, Loki, Prometheus `otel-lgtm`) em 2026-07-12.
> Decisões confirmadas (múltipla escolha, 2026-07-12): grafia **"cartão" com acento** sempre acompanhada do emoji 💳; **apenas** a palavra "outro" em negrito e minúscula no convite ao próximo cartão; resumo final **completo**; exemplo de vencimento em **ambos os formatos** ("dia 1" e "dia primeiro"); PRD **independente** e compatível com `prd-onboarding-sem-friccao`; exemplo no **convite inicial e no reprompt**; título de seção **"Resumo de Onboarding"**; prova de aceite por **eval golden real-LLM + testes**.

## Visão Geral

Esta funcionalidade corrige lacunas reais da etapa de cartões e do encerramento do onboarding conversacional do MeControla no WhatsApp, observadas numa jornada de produção. Na conversa real, todas as mensagens de cartão usaram apenas o emoji `💳` sem a palavra "cartão"; o convite ao segundo cartão veio como "Deseja cadastrar OUTRO 💳" (maiúsculo, sem negrito); ao responder "Sim" o cliente recebeu um reprompt genérico sem exemplo de preenchimento; e após recusar o segundo cartão o onboarding foi encerrado com uma frase que menciona só o objetivo, sem qualquer resumo do orçamento, da distribuição ou dos cartões.

O valor entregue é um encerramento de onboarding claro e fechado: o cliente sempre lê "cartão 💳" por extenso, é convidado de forma amigável a cadastrar *outro* cartão 💳 (com apenas "outro" em negrito), recebe um exemplo exato de como cadastrar — inclusive quando não informa o apelido — e, ao terminar a etapa de cartões (tendo cadastrado nenhum, um ou vários cartões), recebe um "Resumo de Onboarding" completo seguido da conclusão. A mudança é de copy e de montagem de mensagem no passo de conclusão do workflow, preservando integralmente o motor de workflow, as regras de orçamento/recorrência e a idempotência de escrita.

## Objetivos

- Eliminar a ambiguidade de linguagem na etapa de cartões: 100% das mensagens de cartão do onboarding contêm a palavra "cartão" (com acento) acompanhada do emoji 💳, com 0 mensagens usando 💳 isolado no lugar da palavra.
- Padronizar o convite ao próximo cartão: 100% das ocorrências no formato exato "Deseja cadastrar *outro* cartão 💳 agora?", com apenas "outro" em negrito e minúscula.
- Reduzir fricção no cadastro de cartão: exibir exemplo exato (com e sem apelido, "dia 1"/"dia primeiro") no convite inicial e no reprompt, buscando 0 loop de cartão por resposta válida.
- Fechar o onboarding com clareza: 100% dos onboardings que chegam ao passo de conclusão emitem um "Resumo de Onboarding" completo seguido da conclusão, tanto sem cartão quanto com um ou vários cartões, apresentado uma única vez ao final.
- Zero regressão: métricas e comportamento de `workflow_steps_total{workflow="onboarding-workflow"}`, `onboarding_workflow_total`, criação de cartão, ativação de orçamento, recorrência e idempotência de escrita permanecem inalterados em semântica.
- Prova de qualidade: eval golden real-LLM (`RUN_REAL_LLM=1`) cobrindo os quatro comportamentos, com score no limiar do projeto (features anteriores atingiram 1.0000, limiar mínimo ≥ 0,90), somada a testes unitários e de integração determinísticos verdes.

## Histórias de Usuário

- Como pessoa em onboarding no WhatsApp, quero que a etapa fale "cartão 💳" por extenso, para entender claramente do que se trata em vez de decifrar um emoji isolado.
- Como pessoa em onboarding, quero ser convidada de forma amigável a cadastrar *outro* cartão 💳 (com "outro" em destaque), para perceber que posso adicionar mais de um cartão sem esforço.
- Como pessoa em onboarding, quero um exemplo exato de como cadastrar o cartão — inclusive quando não informo o apelido —, para acertar de primeira e não ficar em idas e vindas.
- Como pessoa em onboarding, quero um resumo completo do que configurei junto da conclusão — tenha eu cadastrado nenhum, um ou vários cartões —, para terminar o onboarding com uma visão fechada do meu planejamento.

## Funcionalidades Core

1. **Linguagem de cartão por extenso com emoji** — toda referência a cartão nas mensagens do onboarding usa a palavra "cartão" (com acento) junto do emoji 💳, sem substituir a palavra pelo emoji isolado. Importa porque a jornada de produção mostrou mensagens só com 💳, ambíguas para o cliente. Em alto nível: ajuste das constantes/funções de prompt da etapa de cartões e do resumo.
2. **Convite ao próximo cartão com destaque** — quando já existe ao menos um cartão, o onboarding pergunta exatamente "Deseja cadastrar *outro* cartão 💳 agora?", com apenas a palavra "outro" em negrito e minúscula. Importa porque o convite atual está em maiúsculas e sem destaque, e sem a palavra "cartão".
3. **Exemplo exato de cadastro** — o convite inicial e o reprompt após um "sim" incompleto trazem um exemplo com apelido ("Roxinho, Nubank e vencimento dia 1"/"...dia primeiro") e um sem apelido ("Nubank e vencimento dia 1"/"...dia primeiro"), comunicando que sem apelido o apelido herda o nome do banco. Importa porque hoje o cliente recebe um reprompt genérico sem exemplo e pode errar repetidamente.
4. **Resumo + conclusão ao encerrar a etapa de cartões** — ao terminar os cartões (recusando qualquer cartão ou após cadastrar um/vários e recusar outro), o passo de conclusão apresenta um "Resumo de Onboarding" completo (objetivo, valor da meta quando houver, orçamento mensal, distribuição por categoria, cartões cadastrados ou "nenhum cartão", recorrência) seguido da frase de conclusão, uma única vez ao final. Importa porque hoje o encerramento cita só o objetivo, sem visão consolidada.

## Requisitos Funcionais

Linguagem de cartão (palavra + emoji):
- RF-01: Toda mensagem do onboarding que se refira a um cartão deve conter a palavra "cartão" (com acento) acompanhada do emoji 💳 na mesma frase. Aplica-se ao convite inicial, aos três reprompts de dados faltantes, ao convite a cadastrar outro cartão e ao "Resumo de Onboarding".
- RF-02: Nenhuma mensagem de cartão do onboarding pode usar o emoji 💳 isolado como substituto da palavra "cartão".
- RF-03: A funcionalidade não deve introduzir nenhum outro emoji para representar cartão além de 💳, preservando a convenção de emoji de cartão já vigente no produto.

Convite ao próximo cartão (destaque):
- RF-04: Quando o usuário já tiver ao menos um cartão cadastrado, a pergunta de cadastrar mais um deve ser exatamente "Deseja cadastrar *outro* cartão 💳 agora?", com apenas a palavra "outro" em negrito e em minúsculas; o restante da frase mantém a capitalização normal iniciando com "Deseja", e a palavra "cartão" permanece acompanhada do emoji 💳.

Exemplo exato de cadastro:
- RF-05: O convite inicial de cartão deve incluir um exemplo exato de como cadastrar, contendo uma forma com apelido e uma forma sem apelido.
- RF-06: Quando o usuário sinalizar que quer cadastrar um cartão mas não fornecer dados suficientes (sem apelido/banco e/ou sem dia de vencimento), o reprompt deve incluir o mesmo exemplo exato, com a forma com apelido e a forma sem apelido.
- RF-07: O exemplo deve apresentar o dia de vencimento em ambos os formatos, numérico ("dia 1") e por extenso ("dia primeiro"), e o onboarding deve aceitar as duas formas na resposta do usuário.
- RF-08: O exemplo sem apelido deve comunicar explicitamente que, quando o apelido não é informado, o apelido do cartão fica igual ao nome do banco.
- RF-09: Quando o usuário informar apenas o banco, sem apelido, o cartão deve ser criado com apelido igual ao nome do banco, preservando o comportamento já existente; esse comportamento não pode regredir.

Resumo + conclusão ao encerrar cartões:
- RF-10: Ao encerrar a etapa de cartões — seja recusando cadastrar qualquer cartão, seja após cadastrar um ou vários cartões e responder que não quer cadastrar outro — o onboarding deve apresentar um "Resumo de Onboarding" seguido da frase de conclusão e encerrar o onboarding.
- RF-11: O "Resumo de Onboarding" deve ser apresentado uma única vez, ao final da etapa de cartões, e nunca ser reapresentado a cada cartão cadastrado.
- RF-12: O "Resumo de Onboarding" deve conter: o objetivo financeiro; o valor da meta quando informado; o orçamento mensal; a distribuição confirmada por categoria (as 5 categorias canônicas com valor e percentual); a lista de cartões cadastrados (apelido, banco e dia de vencimento) ou a indicação explícita de que nenhum cartão foi cadastrado; e o estado da recorrência (repetição pelos próximos 12 meses ligada ou desligada).
- RF-13: A seção de resumo deve usar o título "Resumo de Onboarding", reaproveitando o comportamento do normalizador de saída que prefixa automaticamente o emoji 📊 a esse título.
- RF-14: Os valores exibidos no resumo devem refletir exatamente o estado persistido no onboarding: objetivo, valor da meta, orçamento mensal e distribuição vêm do estado do workflow; os cartões vêm da fonte de verdade de cartões do usuário; a recorrência reflete a escolha feita no passo de recorrência.
- RF-15: As categorias no resumo devem usar os rótulos padronizados com emoji já vigentes no onboarding (Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira), e os valores monetários devem usar a mesma formatação BRL já usada nos demais passos.
- RF-16: Quando nenhum cartão for cadastrado, o resumo deve indicar explicitamente que nenhum cartão foi cadastrado, mantendo objetivo, orçamento, distribuição e recorrência.

Não regressão e prova de aceite:
- RF-17: As mudanças devem se restringir a copy e à montagem de mensagem no passo de conclusão do onboarding, sem alterar a ordem das etapas, os gatilhos de suspend/resume, a extração via LLM, a criação de cartão, a ativação de orçamento, a recorrência ou a idempotência de escrita.
- RF-18: O aceite exige eval golden real-LLM (`RUN_REAL_LLM=1`) cobrindo os quatro comportamentos (palavra+emoji, convite com "outro" em negrito, exemplo exato, resumo+conclusão com e sem cartão) e testes unitários e de integração determinísticos verdes; nenhum comportamento pode ser declarado pronto apenas com mock.

## Experiência do Usuário

- Persona primária: cliente pagante recém-ativado fazendo onboarding pelo WhatsApp, que já configurou objetivo, orçamento, distribuição e recorrência e chega à etapa de cartões.
- Fluxo principal (com cartão): o onboarding convida a cadastrar cartão 💳 mostrando o exemplo; o cliente informa banco (e opcionalmente apelido) e vencimento; o cartão é criado (apelido herda o banco quando omitido); o onboarding pergunta "Deseja cadastrar *outro* cartão 💳 agora?"; ao recusar, recebe o "Resumo de Onboarding" completo e a conclusão.
- Fluxo alternativo (sem cartão): o cliente recusa cadastrar cartão 💳 no convite inicial; o onboarding segue direto para o "Resumo de Onboarding" (indicando "nenhum cartão cadastrado") e a conclusão.
- Fluxo alternativo (múltiplos cartões): o cliente cadastra dois ou mais cartões, respondendo ao convite de "outro" a cada um; ao recusar o próximo, recebe um único "Resumo de Onboarding" listando cada cartão, seguido da conclusão.
- Considerações de copy: negrito no WhatsApp é expresso por asterisco simples; a palavra "outro" é destacada com esse recurso; o título "Resumo de Onboarding" recebe o emoji 📊 automaticamente; os valores monetários seguem o formato BRL já usado; as categorias mantêm seus rótulos com emoji.
- Acessibilidade/robustez de conversa: as mensagens devem ser autoexplicativas (exemplo concreto reduz erro), e o resumo consolidado dá fechamento cognitivo à jornada.

## Restrições Técnicas de Alto Nível

- Compatibilidade com `prd-onboarding-sem-friccao-ate-primeiro-lancamento` (já implementado e mergeado): este PRD é independente, porém explicitamente compatível — ele adiciona a palavra "cartão" às mensagens sem remover o emoji 💳 e sem introduzir outro emoji, portanto não contradiz RF-07/RF-08 daquele PRD; refina apenas a copy do convite ao próximo cartão (antes "Deseja cadastrar OUTRO 💳").
- O motor genérico de workflow (`internal/platform/workflow`) não pode ser alterado (governança R-WF-KERNEL-001); as mudanças ficam no consumidor `internal/agents`.
- Governança de adaptador e consumidor: código Go alterado sem comentários (R-ADAPTER-001.1); comportamento novo no consumidor sem `switch case intent.Kind` (R-AGENT-WF-001); LLM apenas nas call-sites já sancionadas — a extração de dados de cartão/recorrência/orçamento permanece inalterada.
- A entrega ao usuário usa o caminho existente: a mensagem final de conclusão (com o resumo embutido) é entregue como resposta do onboarding pelo consumidor de WhatsApp; nenhuma nova integração externa é introduzida.
- Fonte de verdade: cartões vêm da interface de cartões já injetada no workflow; objetivo/meta/orçamento/distribuição/recorrência vêm do estado durável do onboarding; nenhuma nova tabela, migração de schema ou campo de domínio é necessário para atender aos requisitos.
- Sensibilidade de dados: o resumo exibe apenas dados do próprio usuário em sua própria thread; nenhuma exposição cross-usuário; nenhum dado sensível novo é coletado.

## Fora de Escopo

- Alterações no motor de workflow (`internal/platform/workflow`) ou nos primitivos de plataforma (`internal/platform/{agent,memory}`).
- Mudanças na ordem das etapas do onboarding, nos gatilhos de suspend/resume ou na lógica de extração via LLM (schemas e system prompts de extração permanecem inalterados).
- Mudanças nas regras de orçamento, distribuição, ativação ou recorrência; o resumo apenas reflete o que já foi decidido nesses passos.
- Persistência do resumo/conclusão como mensagem no histórico da thread (`platform_messages`); hoje o passo de conclusão não é suspenso e por isso a conclusão não é gravada no histórico — alterar isso é melhoria separada.
- Reprocessamento ou reenvio do resumo para usuários que concluíram o onboarding antes desta mudança.
- Introdução de novos estados/enum de domínio, novo emoji para cartão, ou novas frequências/valores de negócio.
- Redefinição da convenção de emoji de cartão do PRD `prd-onboarding-sem-friccao` (mantida intacta; este PRD apenas adiciona a palavra).

## Suposições e Questões em Aberto

Nenhuma questão em aberto. Todas as decisões materiais foram confirmadas com o solicitante em 2026-07-12 e estão refletidas nos requisitos:

- Grafia "cartão" com acento, sempre junto do emoji 💳 (RF-01, RF-02, RF-03).
- Apenas a palavra "outro" em negrito e minúscula no convite ao próximo cartão (RF-04).
- Exemplo exato no convite inicial e no reprompt, com e sem apelido, em ambos os formatos de dia (RF-05, RF-06, RF-07, RF-08).
- Resumo completo com título "Resumo de Onboarding" ao encerrar a etapa de cartões, com e sem cartão, uma única vez (RF-10 a RF-16).
- Relacionamento com `prd-onboarding-sem-friccao`: PRD independente e compatível (Restrições Técnicas de Alto Nível).
- Prova de aceite por eval golden real-LLM somada a testes determinísticos (RF-18).
</content>
