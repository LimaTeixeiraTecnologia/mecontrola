# US-001: Polir copy e fluxo do onboarding conversacional (boas-vindas, confirmação do objetivo, cartão e resumo sem redundância)

## Declaração
Como usuário novo do MeControla concluindo o onboarding conversacional no WhatsApp, quero mensagens claras, bem organizadas, que confirmem meu objetivo com reforço positivo e não repitam informação, para me sentir ouvido e chegar ao fim do onboarding sem confusão nem ruído.

## Contexto
- Problema: o onboarding atual tem atritos de copy e redundância evidenciados em `internal/agents/application/workflows/onboarding_workflow.go`: o exemplo de meta nas boas-vindas usa "comprar uma casa, meta de R$ 400.000,00" (linha 527); a mensagem que segue a captura do objetivo não confirma nem reforça o objetivo do usuário (é apenas a pergunta do valor da meta em `goalValueReprompt`, linha 531); os textos de cartão são um parágrafo corrido sem blocos (`cardsPrompt`, linhas 613-625); após criar um cartão a próxima mensagem reaproveita `cardsPrompt(existing>0)` ("Você já tem N cartão cadastrado", linha 616) em vez de uma confirmação de sucesso; e o objetivo aparece duas vezes no resumo final (`conclusionSummaryMessage` cabeçalho linhas 693-697 e novamente via `conclusionFinalMessage` linhas 650-660).
- Resultado esperado: onboarding com boas-vindas usando exemplo de meta acessível (celular novo, R$ 5.000,00), segunda mensagem confirmando e reforçando o objetivo de forma determinística, mensagens de cartão organizadas em blocos, confirmação de sucesso ao registrar cartão, e o objetivo citado uma única vez no resumo final.
- Fonte: pedido direto do usuário (5 itens numerados) + confronto com a base de código em `internal/agents/application/workflows/onboarding_workflow.go` e `internal/agents/application/workflows/card_create_confirm_workflow.go`.

## Regras de Negócio
- RN-1 (Boas-vindas): no texto de boas-vindas `welcomeCombinedPrompt` (`onboarding_workflow.go:522-527`), o exemplo de meta "comprar uma casa, meta de R$ 400.000,00" é substituído por "comprar um celular novo, meta de R$ 5.000,00". Nenhum outro trecho da mensagem de boas-vindas muda.
- RN-2 (Confirmação + reforço do objetivo): na etapa `BuildGoalStep` (`onboarding_workflow.go:734-797`), a mensagem enviada logo após o usuário informar o objetivo deve, na mesma mensagem e nesta ordem: (a) confirmar o objetivo ecoando o texto exato informado pelo usuário (valor de `state.Goal`), (b) trazer um reforço positivo relacionado a esse objetivo e (c) manter a pergunta opcional do valor da meta hoje presente em `goalValueReprompt` (`onboarding_workflow.go:531`), incluindo a saída "não" para seguir sem informar valor.
- RN-3 (Reforço determinístico, sem LLM): a confirmação e o reforço da RN-2 são construídos de forma determinística por template que interpola `state.Goal`, sem nenhuma chamada adicional a `agent.Agent.Execute`/LLM, preservando a pureza do passo de decisão (DMMF Decide* puro) e a ausência de custo/latência extra.
- RN-4 (Cartão organizado): os textos de cartão do onboarding — `cardsPrompt` (`onboarding_workflow.go:613-625`) e os reprompts `cardsReprompt`, `cardsRepromptMissingName`, `cardsRepromptMissingDueDay`, `cardsRepromptMissingBoth` (`onboarding_workflow.go:547-561`) — são reorganizados em blocos legíveis (quebras de linha entre intenção, exemplo e a saída "não"), com emojis e texto organizados. O significado é preservado: cartão é opcional, há exemplo de preenchimento, apelido ausente assume o nome do banco, e responder "não" pula a etapa. No convite inicial quando já existem cartões de sessão anterior (`cardsPrompt` ramo `existing > 0`, `onboarding_workflow.go:614-620`), o informativo "Você já tem N cartão(ões) cadastrado(s)" é mantido; ele apenas é reorganizado em blocos, não substituído pela confirmação de sucesso da RN-5.
- RN-5 (Sucesso ao registrar cartão): no `BuildCardsStep`, após `cards.CreateCard` retornar sucesso (`onboarding_workflow.go:876-888`), a próxima mensagem começa com a confirmação "💳 Cartão registrado com sucesso ✅" seguida da pergunta "Quer registrar algum outro?", substituindo o reaproveitamento de `cardsPrompt(len(existingCards))` que hoje diz "Você já tem N cartão cadastrado(s)". Essa confirmação de sucesso ocorre exclusivamente após um cadastro concluído nesta sessão; o convite inicial da etapa (RN-4) não usa esse texto.
- RN-6 (Objetivo uma vez no resumo): em `conclusionSummaryMessage` (`onboarding_workflow.go:690-708`), o objetivo permanece apenas no cabeçalho ("🎯 Objetivo:", linhas 693-697) e deixa de aparecer no bloco final; `conclusionFinalMessage` (`onboarding_workflow.go:650-660`) deixa de mencionar o objetivo, preservando a chamada de ação final ("Tudo pronto! 🚀 ... me envie seus gastos e receitas ...").
- RN-7 (Consistência no cartão pós-onboarding): no fluxo conversacional avulso `card_create_confirm_workflow.go`, apenas a confirmação de sucesso de cadastro (`card_create_confirm_workflow.go:155`, hoje "✅ 💳 *%s* cadastrado com sucesso.") é alinhada ao tom de sucesso do onboarding. A linha de idempotência "✅ 💳 *%s* já estava cadastrado." (`card_create_confirm_workflow.go:153`) permanece intacta. Como esse fluxo é de cartão único (sem laço de "outro cartão"), o sufixo "Quer registrar algum outro?" não se aplica ali; apenas a confirmação de sucesso é padronizada.
- RN-8 (Sem regressão de comportamento): todas as mudanças são de copy e ordenação de texto; os estados fechados (`OnboardingPhase`, `reviewAwaitKind`, `allocationInputKind`), os `Decide*`, o esquema de suspensão/retomada do workflow e a criação de cartão/orçamento permanecem inalterados.

## Critérios de Aceite
```gherkin
Cenário: Boas-vindas com novo exemplo de meta e confirmação do objetivo com reforço
  Dado que um usuário novo inicia o onboarding no WhatsApp
  Quando ele recebe a mensagem de boas-vindas
  Então o exemplo de meta exibido é "comprar um celular novo, meta de R$ 5.000,00"
  E não é exibido o exemplo "comprar uma casa"
  Quando ele responde informando o objetivo "trocar de celular"
  Então a mensagem seguinte confirma o objetivo ecoando "trocar de celular"
  E inclui um reforço positivo relacionado a esse objetivo
  E ainda pergunta, de forma opcional, o valor da meta, oferecendo responder "não" para seguir sem valor
  E nenhuma chamada adicional de LLM é feita para gerar a confirmação e o reforço

Cenário: Cartão organizado em blocos e confirmação de sucesso ao registrar
  Dado que o usuário chegou à etapa de cartão do onboarding
  Quando ele recebe o convite para cadastrar um cartão
  Então o texto está organizado em blocos (intenção, exemplo e a saída "não" separados por quebras de linha)
  E deixa claro que o cartão é opcional e que "não" pula a etapa
  Quando ele informa apelido, banco emissor e dia de vencimento válido
  E o cartão é criado com sucesso
  Então a próxima mensagem inicia com "💳 Cartão registrado com sucesso ✅"
  E pergunta "Quer registrar algum outro?"
  E não repete o texto "Você já tem N cartão cadastrado(s)"

Cenário: Resumo final cita o objetivo uma única vez e cartão inválido é rejeitado com reprompt
  Dado que o usuário informou objetivo, orçamento mensal e confirmou a distribuição
  Quando ele conclui o onboarding e recebe o resumo final
  Então o objetivo aparece somente no cabeçalho "🎯 Objetivo:"
  E a mensagem final de conclusão não menciona novamente o objetivo
  E a chamada de ação final para enviar gastos e receitas é preservada
  Dado que, na etapa de cartão, o usuário informou um dia de vencimento fora do intervalo 1 a 31
  Quando o cartão é avaliado
  Então nenhum cartão é criado
  E ele recebe o reprompt de cartão organizado pedindo um dia de vencimento entre 1 e 31
```

## Dados e Permissões
- Dados obrigatórios: texto do objetivo informado pelo usuário (`state.Goal`); dados do cartão quando o usuário optar por cadastrar (apelido, banco emissor, dia de vencimento 1–31, validados por `DecideCardEntry`, `onboarding_workflow.go:382-394`); orçamento mensal e distribuição já coletados nas etapas anteriores para compor o resumo.
- Perfis/permissões: usuário autenticado no canal WhatsApp Meta em seu próprio contexto de onboarding; não há operação administrativa nem escopo de outro usuário — o passo de cartão opera sobre `state.UserID` (`onboarding_workflow.go:834, 872`).

## Dependências
- Testes que fixam as strings/funcs alteradas e precisam ser atualizados no mesmo trabalho: `internal/agents/application/workflows/onboarding_workflow_test.go` (asserts sobre `welcomeCombinedPrompt` linha 772, `goalValueReprompt` linhas 216-217/817/858/910, `cardsPrompt(...)` linhas 1704/1752/1772/1789/1854/1876/1952-1954); `internal/agents/application/workflows/onboarding_workflow_integration_test.go` (exemplo "comprar uma casa" linha 123); `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_test.go` (texto de boas-vindas linha 32; "Resumo de Onboarding" linhas 468/475) e `whatsapp_inbound_consumer_integration_test.go` (fluxo de resumo linhas 370/397). Evidência coletada por `grep` nesses arquivos.
- Governança obrigatória do repositório para a implementação Go: zero comentários em `.go` de produção (R-ADAPTER-001.1) e `Decide*` puro para a lógica determinística de RN-3 (`.claude/rules/transactions-workflows.md`, `.claude/rules/governance.md`).
- Não há dependência de serviço externo novo nem de migração de banco: todas as mudanças são em texto e ordenação de mensagens dentro dos workflows já existentes.

## Fora de Escopo
- Personalização do reforço via LLM (decidida como determinística nesta US).
- Remoção da pergunta opcional do valor da meta (decidida como mantida/prefixada nesta US).
- Alteração dos prompts de sistema do extrator, como `goalWithValueSystemPrompt` (`onboarding_workflow.go:587-595`), que usam valores só como exemplos internos de conversão e não são mensagens ao usuário.
- Alteração do objetivo exibido no `summaryPrompt` da revisão de orçamento (`onboarding_workflow.go:639-648`), que é outra mensagem, distinta do resumo de conclusão citado na RN-6.
- Adição de um laço "cadastrar outro cartão" ao fluxo avulso `card_create_confirm_workflow.go`; a RN-7 padroniza apenas a confirmação de sucesso, sem mudar a natureza single-shot desse workflow.
- Publicação da história em Jira, Azure DevOps ou GitHub Issues.

## Evidências
- Entrada: pedido do usuário com 5 itens (boas-vindas celular R$ 5.000,00; confirmação+reforço do objetivo na 2ª interação; cartão em blocos; confirmação de sucesso do cartão; objetivo uma vez no resumo) e 3 respostas de esclarecimento (reforço determinístico; prefixar mantendo o valor; escopo inclui `card_create_confirm_workflow.go`).
- Base de código: `internal/agents/application/workflows/onboarding_workflow.go` — `welcomeCombinedPrompt` linhas 522-527; `goalValueReprompt` linha 531 e uso em `BuildGoalStep` linhas 766-776; `cardsPrompt` linhas 613-625 e reprompts linhas 547-561; criação de cartão e reaproveitamento de `cardsPrompt` no sucesso linhas 876-888; `conclusionSummaryMessage` linhas 690-708 e `conclusionFinalMessage` linhas 650-660. `internal/agents/application/workflows/card_create_confirm_workflow.go:155` — confirmação de sucesso do cartão avulso.
- Inferências: a "segunda interação" corresponde à mensagem imediatamente após a captura do objetivo em `BuildGoalStep`, que hoje é `goalValueReprompt`; a persona é o usuário novo no canal WhatsApp por ser o fluxo de onboarding.
- Não evidenciado: busca por `grep` não encontrou uso de `conclusionFinalMessage` fora de `conclusionSummaryMessage`, confirmando que remover o objetivo dele afeta apenas o resumo de conclusão; nenhuma outra mensagem ao usuário reaproveita o exemplo "comprar uma casa" fora dos pontos listados.

## Notas de Validação
- Cobertura de cenários: feliz (boas-vindas + confirmação/reforço + valor opcional; cartão organizado + sucesso), alternativa (resumo com objetivo único preservando a CTA) e erro (dia de vencimento fora de 1–31 rejeitado com reprompt organizado, sem criar cartão).
- Decisões confirmadas pelo usuário: reforço determinístico (sem LLM); confirmação+reforço prefixados mantendo a pergunta de valor; escopo estendido à confirmação de sucesso do `card_create_confirm_workflow.go`.
- Verificação técnica esperada na implementação: build, vet, test race e lint do módulo `internal/agents`, com atualização dos asserts de copy listados nas Dependências.
