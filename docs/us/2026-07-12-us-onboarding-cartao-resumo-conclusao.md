# Onboarding — Cartão por Extenso, Exemplo de Cadastro e Resumo/Conclusão Final — User Story

> Objetivo: fechar lacunas reais do onboarding conversacional na etapa de cartões e no encerramento, observadas em produção, de modo que (1) toda menção a cartão use a palavra "cartão" junto do emoji 💳, (2) o convite a cadastrar mais de um cartão apareça em negrito e minúsculo, (3) ao dizer "sim" o usuário receba um exemplo exato de como cadastrar (com e sem apelido), e (4) ao final — tendo cadastrado cartão ou não — o usuário receba um resumo completo do onboarding seguido da conclusão.
> Escopo confirmado com o solicitante: **melhorar os textos da etapa de cartões e o encerramento do workflow de onboarding**; nenhuma mudança em orçamento, recorrência, metas ou no motor de workflow.
> Decisões confirmadas com o solicitante (múltipla escolha, 2026-07-12): grafia **"cartão" com acento**; resumo final **completo** (objetivo + valor da meta, orçamento mensal, distribuição por categoria, cartões cadastrados ou "nenhum cartão", recorrência, e a frase de conclusão); exemplo de vencimento em **ambos os formatos** ("dia 1" e "dia primeiro"), aceitando as duas formas.
> Data de geração: 2026-07-12
> Nome do arquivo: `2026-07-12-us-onboarding-cartao-resumo-conclusao.md`
> Base: `internal/agents/application/workflows/onboarding_workflow.go` (workflow durável de onboarding, consumidor Mastra sobre `internal/platform/{agent,workflow,memory}`), entregue via `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`.
> Evidência de produção: run `5eeaf6e8-8c50-461b-b2ce-442f7d7105cf` do usuário `8b0eeabb-293b-4837-a5a2-8418c63fce94` (WhatsApp +5511986896322), status `succeeded`, coletada por SSH em `root@187.77.45.48` (Postgres `workflow_runs`/`platform_messages`, Loki, Prometheus `otel-lgtm`) em 2026-07-12.

---

## Declaração
Como pessoa que está fazendo o onboarding financeiro do MeControla pelo WhatsApp, quero que a etapa de cartão fale "cartão 💳" por extenso, me convide a cadastrar *outro* cartão 💳 destacando apenas a palavra "outro" em negrito, me dê um exemplo exato de como cadastrar (inclusive quando eu não informar o apelido) e, ao final, me mostre um resumo completo do que configurei junto da conclusão — tenha eu cadastrado um cartão ou não —, para que eu entenda claramente o que foi cadastrado, saiba exatamente como responder e termine o onboarding com uma visão fechada do meu planejamento.

## Contexto
- Problema: na conversa real de produção (run `5eeaf6e8`), todas as mensagens da etapa de cartão usaram apenas o emoji `💳`, nunca a palavra "cartão"; o convite ao segundo cartão veio como "Deseja cadastrar OUTRO 💳" (maiúsculo, sem negrito); ao responder "Sim" o usuário recebeu um reprompt genérico sem exemplo de preenchimento; e após recusar o segundo cartão ("Não") o onboarding foi encerrado com uma frase de conclusão que menciona só o objetivo, sem qualquer resumo do orçamento, da distribuição ou dos cartões.
- Resultado esperado: os quatro pontos acima corrigidos exclusivamente na camada de prompts e no passo de conclusão do workflow de onboarding, preservando o motor de workflow, as regras de orçamento/recorrência e a idempotência de escrita. O passo de conclusão passa a montar um "Resumo de Onboarding" completo seguido da frase de conclusão, cobrindo os dois desfechos (com e sem cartão).
- Fonte: pedido do solicitante em 2026-07-12 para produzir uma única história de usuário que feche lacunas, falso positivo e ressalvas do `onboarding_workflow.go`, com uso obrigatório de investigação em produção via SSH e das skills `user-stories`, `go-implementation`, `mastra`, `domain-modeling-production` e `design-patterns-mandatory`. Decisões de grafia, conteúdo do resumo e formato do exemplo confirmadas por múltipla escolha na mesma data.

## Regras de Negócio
- RN-01 (palavra + emoji sempre juntos): toda mensagem do onboarding que se refira a um cartão DEVE conter a palavra "cartão" (com acento) acompanhada do emoji `💳` na mesma frase — por exemplo "cartão 💳". Aplica-se ao convite inicial, aos três reprompts de dados faltantes, ao convite a cadastrar outro e ao resumo final. Nenhuma mensagem pode usar `💳` sozinho como substituto da palavra.
- RN-02 (destaque da palavra "outro" em negrito e minúscula): quando o usuário já tem ao menos um cartão cadastrado, a pergunta se ele quer cadastrar mais um DEVE ser exatamente "Deseja cadastrar *outro* cartão 💳 agora?", com apenas a palavra "outro" em negrito e em minúsculas; o restante da frase mantém a capitalização normal, iniciando com "Deseja", e a palavra "cartão" continua acompanhada do emoji 💳. O negrito é expresso no prompt com `**outro**`, que o normalizador de saída converte para o negrito do WhatsApp (`*outro*`).
- RN-03 (exemplo exato ao dizer "sim"): quando o usuário sinaliza que quer cadastrar um cartão mas ainda não forneceu os dados suficientes (apelido/banco e/ou dia de vencimento), a resposta DEVE conter um exemplo exato em duas formas: uma com apelido ("Roxinho, Nubank e vencimento dia 1" / "Roxinho, Nubank e vencimento dia primeiro") e uma sem apelido ("Nubank e vencimento dia 1" / "Nubank e vencimento dia primeiro"). Ambos os formatos de dia (numérico "dia 1" e por extenso "dia primeiro") DEVEM ser mostrados e aceitos.
- RN-04 (apelido opcional herda o banco): quando o usuário informar apenas o banco, sem apelido, o apelido do cartão DEVE ser assumido igual ao nome do banco. Esse comportamento já existe e não pode regredir; a novidade é comunicá-lo ao usuário por meio do exemplo sem apelido de RN-03.
- RN-05 (resumo + conclusão ao encerrar a etapa de cartões): assim que o usuário encerra a etapa de cartões — seja recusando cadastrar qualquer cartão, seja após cadastrar um ou vários cartões e responder que não quer cadastrar outro — o passo de conclusão DEVE apresentar um "Resumo de Onboarding" seguido da frase de conclusão, encerrando o onboarding. O resumo é apresentado uma única vez, ao final, e nunca a cada cartão cadastrado.
- RN-06 (conteúdo completo do resumo): o "Resumo de Onboarding" DEVE conter: o objetivo financeiro; o valor da meta quando informado; o orçamento mensal; a distribuição confirmada por categoria (as 5 categorias canônicas com valor e percentual); a lista de cartões cadastrados (apelido, banco e dia de vencimento) ou a indicação explícita de que nenhum cartão foi cadastrado; e o estado da recorrência (repetição pelos próximos 12 meses ligada ou não).
- RN-07 (fidelidade ao estado confirmado): os valores exibidos no resumo DEVEM refletir exatamente o que foi persistido no onboarding — objetivo, valor da meta, orçamento e distribuição vêm do estado do workflow; os cartões vêm da fonte de verdade de cartões do usuário; a recorrência reflete a escolha feita no passo de recorrência.
- RN-08 (rótulos e formatação monetária consistentes): as categorias no resumo DEVEM usar os mesmos rótulos com emoji já padronizados no onboarding (Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira) e os valores monetários DEVEM usar a mesma formatação BRL já usada nos demais passos.
- RN-09 (sem regressão de fluxo): as mudanças são de texto e de montagem de mensagem; não podem alterar a ordem das etapas, os gatilhos de suspend/resume, a criação de cartão, a ativação do orçamento, a recorrência nem a idempotência de escrita.

## Critérios de Aceite
```gherkin
Cenário: Convite inicial de cartão usa a palavra e o emoji juntos
  Dado um usuário que concluiu a etapa de recorrência do onboarding e ainda não tem cartão cadastrado
  Quando o onboarding pergunta se ele deseja cadastrar um cartão
  Então a mensagem contém a palavra "cartão" acompanhada do emoji 💳 na mesma frase
  E a mensagem não usa 💳 isolado no lugar da palavra "cartão"

Cenário: Usuário diz "sim" e recebe exemplo exato com e sem apelido
  Dado o convite inicial de cartão exibido ao usuário
  Quando o usuário responde apenas "sim", sem informar banco nem dia de vencimento
  Então a resposta contém um exemplo com apelido do tipo "Roxinho, Nubank e vencimento dia 1" ou "Roxinho, Nubank e vencimento dia primeiro"
  E a resposta contém um exemplo sem apelido do tipo "Nubank e vencimento dia 1" ou "Nubank e vencimento dia primeiro"
  E a resposta deixa claro que, sem apelido, o apelido do cartão fica igual ao nome do banco

Cenário: Cadastro apenas com banco herda o apelido do banco
  Dado que o usuário quer cadastrar um cartão
  Quando ele responde "Nubank, vencimento dia 1", sem informar apelido
  Então o cartão é cadastrado com apelido igual a "Nubank"
  E o onboarding prossegue para o convite de cadastrar outro cartão

Cenário: Convite ao próximo cartão destaca apenas a palavra outro em negrito
  Dado que o usuário já cadastrou pelo menos um cartão
  Quando o onboarding pergunta se ele quer cadastrar mais um
  Então a mensagem é "Deseja cadastrar *outro* cartão 💳 agora?"
  E apenas a palavra "outro" aparece em negrito e em minúsculas
  E a mensagem contém a palavra "cartão" acompanhada do emoji 💳

Cenário: Resumo e conclusão com cartão cadastrado
  Dado um usuário que cadastrou ao menos um cartão e recusou cadastrar outro
  Quando o passo de conclusão do onboarding é executado
  Então ele recebe um "Resumo de Onboarding" com objetivo, orçamento mensal, distribuição por categoria, os cartões cadastrados e o estado da recorrência
  E, na sequência, recebe a frase de conclusão do onboarding

Cenário: Resumo e conclusão após cadastrar mais de um cartão
  Dado um usuário que cadastrou dois ou mais cartões durante o onboarding
  Quando ele responde que não deseja cadastrar outro cartão 💳
  Então ele recebe um único "Resumo de Onboarding" que lista cada cartão cadastrado com apelido, banco e dia de vencimento
  E o resumo mantém objetivo, orçamento mensal, distribuição por categoria e estado da recorrência
  E, na sequência, recebe a frase de conclusão que encerra o onboarding
  E o resumo não é reapresentado a cada cartão cadastrado, apenas uma vez ao final

Cenário: Resumo e conclusão sem nenhum cartão cadastrado
  Dado um usuário que recusou cadastrar qualquer cartão
  Quando o passo de conclusão do onboarding é executado
  Então ele recebe um "Resumo de Onboarding" que indica explicitamente que nenhum cartão foi cadastrado
  E o resumo mantém objetivo, orçamento mensal, distribuição por categoria e estado da recorrência
  E, na sequência, recebe a frase de conclusão do onboarding

Cenário: Valor da meta aparece no resumo apenas quando informado
  Dado um usuário que informou o valor da meta durante o onboarding
  Quando o passo de conclusão monta o resumo
  Então o valor da meta aparece formatado em BRL junto do objetivo
  E, para um usuário que não informou valor de meta, o resumo omite o valor sem exibir campo vazio
```

## Dados e Permissões
- Dados obrigatórios (já presentes no estado do onboarding, `OnboardingState`, `internal/agents/application/workflows/onboarding_workflow.go:181-195`): `Goal`, `GoalValueCents`, `MonthlyBudgetCents`, `Allocations` (basis points por categoria), `Recurrence`, `CardsDone`, `UserID`.
- Fonte de verdade dos cartões para o resumo: a interface de cartões já injetada no passo de cartões (`interfaces.CardManager`, usada em `BuildCardsStep`, `onboarding_workflow.go:767,775,813,821`); o resumo lista os cartões do usuário a partir dessa mesma fonte.
- Rótulos e categorias canônicas: `canonicalSlugs` e `categoryLabels` (`onboarding_workflow.go:43-57`), reutilizados para o resumo de distribuição.
- Formatação monetária: `money.FromCents(...).BRL()`, já usada nos prompts de orçamento e no resumo pré-ativação (`onboarding_workflow.go:417,628,638`).
- Perfis/permissões: fluxo executado sob a identidade do próprio usuário do onboarding (`UserID` do estado); nenhuma nova permissão é introduzida. A entrega ao usuário ocorre pelo mesmo caminho já existente (`FinalMessage` retornado por `resolve_onboarding_or_agent.go:151` e enviado por `whatsapp_inbound_consumer.go:301,337-357`).

## Dependências
- Dependência técnica interna: o passo de conclusão hoje recebe apenas a working memory (`BuildConclusionStep(workingMem memory.WorkingMemory)`, `onboarding_workflow.go:1036`). Para montar o resumo com cartões, o passo de conclusão precisa de acesso à lista de cartões do usuário (via `interfaces.CardManager`, já disponível no builder do workflow `BuildOnboardingWorkflow`, `onboarding_workflow.go:1089-1115`) ou de os cartões serem carregados no `OnboardingState` durante o passo de cartões. Ambas as opções mantêm o kernel de workflow genérico intocado; a escolha é de implementação e não altera o comportamento observável desta história.
- Dependência de plataforma já satisfeita: o normalizador de saída `NormalizeOutboundText` converte `**` em negrito do WhatsApp e já possui um gancho que prefixa `📊` a um bloco "Resumo de Onboarding"/"Resumo do Onboarding" (`internal/platform/whatsapp/formatting/normalize.go:6,12-23`). Esse gancho existe mas nunca é acionado hoje, porque o onboarding nunca emite esse marcador — esta história passa a produzi-lo, aproveitando a plumbing existente sem criar nova.
- Governança aplicável e já existente no repositório: R-AGENT-WF-001 (`.claude/rules/agent-workflows-tools.md`) — comportamento novo no consumidor `internal/agents`, sem `switch case intent.Kind`, sem regra fora das call-sites sancionadas; R-ADAPTER-001.1 (`.claude/rules/go-adapters.md`) — zero comentários no código Go alterado; R-WF-KERNEL-001 (`.claude/rules/workflow-kernel.md`) — nenhuma mudança pode ser feita em `internal/platform/workflow`.

## Fora de Escopo
- Qualquer alteração no motor de workflow (`internal/platform/workflow`) ou nos primitivos de plataforma (`internal/platform/{agent,memory}`).
- Mudança na ordem das etapas do onboarding, nos gatilhos de suspend/resume ou na lógica de extração via LLM (schemas e system prompts de extração permanecem inalterados).
- Mudança nas regras de orçamento, distribuição, ativação ou recorrência; o resumo apenas reflete o que já foi decidido nesses passos.
- Persistência do resumo/conclusão como mensagem no histórico da thread (`platform_messages`): hoje o passo de conclusão não é suspenso, então a conclusão não é gravada no histórico; alterar isso é uma melhoria separada e não é requisito desta história.
- Reprocessamento ou reenvio do resumo para usuários que já concluíram o onboarding antes desta mudança.
- Introdução de novos estados/enum de domínio: não há necessidade — o resumo lê estado e dados já existentes (ver Notas de Validação, verdict de design pattern).

## Evidências
- Entrada: pedido do solicitante em 2026-07-12 para fechar lacunas do `onboarding_workflow.go` com investigação obrigatória em produção via `ssh root@187.77.45.48`, sem flexibilizar regras e sem inventar resposta; decisões de grafia, conteúdo do resumo e formato de exemplo confirmadas por múltipla escolha na mesma data.
- Base de código:
  - `internal/agents/application/workflows/onboarding_workflow.go:605-610` — `cardsPrompt` usa apenas `💳`, nunca a palavra "cartão"; o convite ao segundo cartão é "Deseja cadastrar OUTRO 💳 agora?" (maiúsculo, sem negrito).
  - `internal/agents/application/workflows/onboarding_workflow.go:547-566` — reprompts `cardsReprompt`, `cardsRepromptMissingName`, `cardsRepromptMissingDueDay`, `cardsRepromptMissingBoth` e `cardsRepromptFor`: usam `💳` sozinho em cada variante; a variante "faltando ambos" (`:552`) não traz nenhum exemplo de preenchimento.
  - `internal/agents/application/workflows/onboarding_workflow.go:370-380` — `normalizeCardExtract` já herda o apelido a partir do banco quando o apelido vem vazio (regra RN-04 já implementada).
  - `internal/agents/application/workflows/onboarding_workflow.go:355-368,382-394` — `classifyCardMissing`/`DecideCardEntry` classificam dados faltantes; ao responder só "sim" o fluxo cai em `cardMissingNameAndDueDay` e devolve o reprompt sem exemplo.
  - `internal/agents/application/workflows/onboarding_workflow.go:635-645` — `conclusionFinalMessage` monta a mensagem final apenas com o objetivo (e valor da meta quando houver); não há resumo de orçamento, distribuição ou cartões.
  - `internal/agents/application/workflows/onboarding_workflow.go:624-633` — `summaryPrompt` (revisão pré-ativação) mostra objetivo, orçamento e distribuição, mas nunca os cartões, porque o passo de cartões roda depois.
  - `internal/agents/application/workflows/onboarding_workflow.go:1102-1111` — ordem das etapas: `budget-review → activation → recurrence → cards → conclusion`; o passo de conclusão é o ponto natural do resumo pós-cartões e roda tanto com quanto sem cartão.
  - `internal/agents/application/workflows/onboarding_workflow.go:1036-1052` — `BuildConclusionStep` recebe apenas `workingMem`; não tem hoje acesso aos cartões (base da dependência técnica listada acima).
  - `internal/agents/application/usecases/resolve_onboarding_or_agent.go:151` e `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:301,337-357` — a `FinalMessage` do estado é o texto de conclusão efetivamente enviado ao usuário (confirmando que enriquecer a conclusão é suficiente para entregar o resumo).
  - `internal/platform/whatsapp/formatting/normalize.go:6,12-23` — `NormalizeOutboundText` converte `**` no negrito do WhatsApp e possui gancho `📊 Resumo de Onboarding` latente, nunca acionado hoje pelo onboarding.
- Evidência de produção (SSH `root@187.77.45.48`, 2026-07-12):
  - `workflow_runs` — run `5eeaf6e8-8c50-461b-b2ce-442f7d7105cf`, `workflow=onboarding-workflow`, `correlation_key=8b0eeabb-293b-4837-a5a2-8418c63fce94`, `status=succeeded`, `state` com `goal="Comprar uma casa"`, `goalValueCents=80000000`, `monthlyBudgetCents=1387440`, `recurrence=true`, `cardsDone=true`, `finalMessage` contendo apenas objetivo e valor da meta (sem resumo).
  - `platform_messages` (thread do usuário) — mensagens verbatim de cartão: "O 💳 é opcional. Você deseja cadastrar um 💳 agora?…", "Para adicionar o 💳, preciso do apelido/banco emissor e do dia de vencimento…" (após o "Sim", sem exemplo), "Você já tem 1 💳 cadastrado(s). Deseja cadastrar OUTRO 💳 agora?…"; após o "Não" do usuário não há nenhuma mensagem de resumo/conclusão registrada na thread.
  - `cards` — cartão `d69b24b3-407e-43da-9ce3-5daf0d4d1750` criado com `bank=Nubank` e `nickname=Nubank` a partir de "Nubank, vencimento dia 1", confirmando em produção a herança de apelido pelo banco (RN-04).
  - Prometheus (`otel-lgtm`) — `workflow_steps_total{workflow="onboarding-workflow"}` mostra `step-cards` e `step-conclusion` com `status=completed`, confirmando que a conclusão executou e é o ponto correto para o resumo.
- Inferências: como o passo de conclusão sempre roda ao final (após `cards`) e sua `FinalMessage` é entregue ao usuário, enriquecer esse passo com um bloco de resumo cobre os dois desfechos (com e sem cartão) sem tocar em outras etapas; o gancho `📊 Resumo de Onboarding` do normalizador indica que essa seção já era esperada pela plataforma.
- Não evidenciado: nenhuma mensagem de "Resumo de Onboarding"/"Resumo do Onboarding" é produzida pelo onboarding — a busca por esses marcadores em `internal/` fora do normalizador e de testes não retornou ocorrências, confirmando que a seção de resumo é hoje inexistente no fluxo; e a tabela `whatsapp_message_status` não possui registros para o destinatário `+5511986896322`, portanto os webhooks de status de entrega da Meta não estão capturados — a entrega da `FinalMessage` foi confirmada pelo caminho de código (`whatsapp_inbound_consumer.go:301`), não por esse registro.

## Notas de Validação
- Cobertura de cenários: a história inclui fluxo feliz (convite com palavra+emoji; resumo+conclusão com cartão), fluxos alternativos válidos (exemplo ao dizer "sim"; herança de apelido pelo banco; convite ao próximo em negrito/minúsculo; resumo sem nenhum cartão) e variação de borda (valor da meta presente vs. ausente no resumo).
- Verdict de design pattern (skill `design-patterns-mandatory`): **não aplicar padrão**. A mudança é localizada — constantes/funções de prompt e um montador de mensagem de resumo no passo de conclusão. A alternativa simples (funções puras de formatação de string reutilizando `categoryLabels`, `canonicalSlugs` e `money.FromCents`) vence em economia, eficiência e robustez; introduzir Builder, Strategy ou qualquer indireção seria overengineering sem sinal forte que o justifique.
- Conformidade DMMF (skill `domain-modeling-production`): nenhum novo estado de domínio é necessário; o resumo é uma projeção de estado e dados já existentes. Os estados fechados atuais (`OnboardingPhase`, `reviewAwaitKind`, `allocationInputKind`, `cardMissingField`) permanecem inalterados. Não se introduz `string` solta como estado nem se colapsa comportamento novo em CRUD.
- Conformidade Mastra/governança (skills `mastra`, `go-implementation`): a mudança fica inteiramente no consumidor `internal/agents` (workflow de onboarding) consumindo o substrato; não se altera `internal/platform/workflow` (R-WF-KERNEL-001) nem se reimplementam primitivos de plataforma; sem `switch case intent.Kind`; código Go alterado permanece sem comentários (R-ADAPTER-001.1). Recursos de linguagem limitados à versão declarada em `go.mod`.
- Não regressão obrigatória: a criação de cartão, a ativação do orçamento, a recorrência e a idempotência de escrita não podem mudar; a herança de apelido pelo banco (`normalizeCardExtract`) precisa continuar coberta por teste; a ordem das etapas do onboarding precisa permanecer `budget-review → activation → recurrence → cards → conclusion`.
- Validação automatizada: `python3 .agents/skills/user-stories/scripts/validar-historias-usuario.py docs/us/2026-07-12-us-onboarding-cartao-resumo-conclusao.md` deve retornar sucesso antes de considerar esta história pronta.
</content>
</invoke>
