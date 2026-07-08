# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

## Visão Geral

Hoje o `step-goal` do onboarding conversacional do MeControla (WhatsApp) captura apenas o texto livre da meta financeira do usuário (`state.Goal`) e o persiste em `platform_resources.metadata` sob a chave `"objetivo_financeiro"`. Não existe nenhum campo, parser ou persistência para um valor monetário associado à meta.

Esta funcionalidade permite que o usuário, opcionalmente, informe **quanto** (em R$) representa a sua meta financeira ao descrevê-la no `step-goal`, para que o MeControla registre esse valor junto ao objetivo e o usuário tenha, mais adiante, um número concreto para acompanhar o progresso.

O valor é sempre opcional: sua ausência nunca bloqueia o onboarding. Quando o usuário já informa meta e valor juntos, ambos são extraídos em uma única interação; quando não informa valor, o sistema pergunta uma única vez e avança independentemente da resposta.

## Objetivos

- Capturar e persistir, de forma opcional, o valor monetário da meta financeira informada no onboarding, sem introduzir nenhum ponto de bloqueio no fluxo.
- Preservar a experiência conversacional atual do `step-goal`: quando meta e valor vêm juntos, uma única extração resolve ambos; quando o valor falta, no máximo uma repergunta.
- **Métrica de sucesso (gate de merge)**: um harness de casos rotulados executado com LLM real (real-LLM) deve atingir **≥ 0.90** de acerto na extração combinada meta+valor, cobrindo os três cenários (valor junto, valor ausente, valor inválido) e os formatos monetários suportados. Merge só é permitido com o gate satisfeito.
- Zero regressão no comportamento existente de `step-goal`, `step-income` e `DecideIncomeCents`.

## Histórias de Usuário

- Como usuário em onboarding, quero informar meta e valor na mesma mensagem (ex.: "Eu quero comprar uma casa, e tenho a meta de R$ 400.000,00"), para que o sistema registre ambos sem me fazer repetir nada.
- Como usuário em onboarding que descreveu a meta sem valor (ex.: "quero quitar minhas dívidas"), quero ser perguntado uma única vez sobre o valor, podendo informar um número ou recusar, para que o onboarding não me prenda nem me obrigue a inventar um número.
- Como usuário em onboarding que não sabe ou não quer dizer o valor (ex.: "quero viajar, mas não sei quanto vou gastar"), quero que o sistema não trave nem me mostre erro técnico, avançando normalmente após uma única tentativa.

## Funcionalidades Core

- **Extração combinada meta + valor**: quando a mensagem do `step-goal` contém meta e valor, ambos são extraídos numa única chamada ao parser LLM-assisted, sem repergunta — seguindo o padrão já usado para meta (`DecideGoal`/`goalSchema`) e para valor monetário (`DecideIncomeCents`/`incomeSchema`).
- **Objetivo permanece obrigatório; valor sempre opcional**: a captura do objetivo textual mantém o comportamento atual (o `step-goal` só avança com um objetivo válido, sob o `MaxAttempts` do workflow); o valor nunca bloqueia. Zero regressão na captura do objetivo.
- **Repergunta combinada quando faltam ambos**: quando a mensagem não produz objetivo válido, a única repergunta do `step-goal` é **combinada** — convida a informar o objetivo e, opcionalmente, o valor numa só pergunta (substitui o texto atual de `_goalReprompt`). Quando o objetivo é válido mas falta valor, a repergunta é **específica de valor**.
- **Repergunta de valor no máximo uma vez**: o valor é perguntado **no máximo uma vez** por execução do `step-goal` (seja via repergunta combinada, seja via repergunta específica de valor). Uma vez "gasta" essa repergunta, o valor nunca é pedido de novo: se ainda ausente/inválido, o fluxo avança sem valor.
- **Tratamento de valor inválido como ausência**: valor negativo, zero ou não numérico ("não sei quanto") é tratado como "valor não informado", sem repergunta adicional de valor além da única permitida, e sem mensagem de erro técnico.
- **Constructor puro dedicado ao valor da meta**: nova smart-constructor pura (distinta de `DecideIncomeCents`), na qual ausência/zero significam "não informado" (válido), e não erro obrigatório.
- **Persistência opcional no metadata**: quando informado, o valor sobrevive no estado do workflow (`OnboardingState`) até o `step-conclusion` e é persistido em `platform_resources.metadata` via merge JSONB sob a chave `"objetivo_financeiro_valor_centavos"` (inteiro em centavos), ao lado de `"objetivo_financeiro"`.
- **Eco apenas na mensagem final**: o valor não é ecoado por campo no `step-goal` (avanço direto ao próximo step, como hoje); quando capturado, ele é mencionado uma única vez na mensagem final de conclusão (`conclusionFinalMessage`), fechando o loop com o usuário. A working memory markdown (`"## Objetivo Financeiro"`) permanece só com o texto do objetivo — o valor não é injetado no system prompt de conversas futuras.

## Requisitos Funcionais

- RF-01: No `step-goal`, quando a mensagem contém meta e valor monetário, o sistema deve extrair ambos em uma única chamada ao parser LLM-assisted, sem repergunta.
- RF-02: O valor da meta é opcional; sua ausência (não informado, recusado ou inválido) nunca deve bloquear o avanço do onboarding.
- RF-03: O objetivo textual permanece obrigatório para avançar o `step-goal`, mantendo o comportamento atual de repergunta/loop até um objetivo válido (bounded pelo `MaxAttempts` do workflow). Zero regressão nesse comportamento.
- RF-03.1: Quando a mensagem não produzir objetivo válido, a única repergunta do `step-goal` deve ser **combinada** — pedindo objetivo e, opcionalmente, o valor numa mesma pergunta (substituindo o texto atual de `_goalReprompt`).
- RF-03.2: Quando o objetivo for válido mas não houver valor válido junto, o sistema deve fazer **uma única** repergunta específica pelo valor da meta.
- RF-03.3: O valor deve ser perguntado **no máximo uma vez** por execução do `step-goal` (contando repergunta combinada OU repergunta específica de valor). Após essa única repergunta, o valor nunca é pedido de novo.
- RF-04: Após a repergunta de valor (combinada ou específica), o fluxo deve avançar independentemente da resposta de valor: valor numérico válido é salvo; recusa ("não") ou resposta não numérica avança sem valor; nunca deve haver uma segunda repergunta de valor.
- RF-05: Valor claramente inválido (negativo, zero ou texto não numérico) deve ser tratado como "valor não informado", sem repergunta de valor além da única permitida por RF-03.3, e sem mensagem de erro técnico.
- RF-06: Mesmo que o usuário recuse explicitamente informar o valor já na primeira mensagem da meta, o sistema deve aplicar a regra uniforme (no máximo uma repergunta de valor, RF-03.3), avançando em seguida conforme RF-04.
- RF-07: A validação do valor da meta (quando informado e não recusado) deve convertê-lo para inteiro em centavos e aceitar qualquer valor positivo; ausência/zero/negativo representam "não informado", sem teto máximo de sanidade.
- RF-08: A validação de RF-07 deve residir em uma smart-constructor pura, nova e distinta de `DecideIncomeCents`, com semântica de "ausência é válida".
- RF-09: A extração de valor deve suportar valores em dígitos com máscara monetária ("R$ 400.000,00", "400000") e formas abreviadas coloquiais ("10 mil reais", "400 mil", "1,5 milhão"), convertendo-os corretamente para centavos.
- RF-10: O valor da meta deve sobreviver no estado do workflow (`OnboardingState`) desde o `step-goal` até o `step-conclusion`, da mesma forma que a meta textual sobrevive hoje.
- RF-11: Quando um valor válido for capturado, ele deve ser persistido no `metadata` do recurso do usuário via merge JSONB, sem exigir migration nova, sob a chave `"objetivo_financeiro_valor_centavos"` (inteiro em centavos, espelhando `IncomeCents`), ao lado da chave `"objetivo_financeiro"`.
- RF-12: Quando nenhum valor válido for capturado ao final do fluxo, a persistência não deve gravar chave de valor (omissão), preservando o comportamento atual de gravação apenas do objetivo textual.
- RF-13: A mensagem inicial do `step-goal` deve mencionar o valor opcional como exemplo/convite, sem torná-lo obrigatório; o texto exato da repergunta de valor é decisão de implementação.
- RF-13.1: No `step-goal`, o valor capturado inline não deve gerar mensagem de eco por campo; o fluxo avança direto ao próximo step, preservando o comportamento atual (nenhum step ecoa campo capturado).
- RF-14: Um harness de casos rotulados executado com LLM real deve validar a extração combinada meta+valor, cobrindo os três cenários e os formatos de RF-09, com acerto **≥ 0.90** como gate de merge. A medição do gate deve usar o modelo de produção `openai/gpt-4o-mini` (default do harness via `AGENT_HARNESS_MODEL`); modelos mais fortes não são aceitos para satisfazer o gate, pois não refletem a experiência real e já mascararam falhas de extração neste projeto.
- RF-15: Quando um valor válido for capturado, a mensagem final de conclusão (`conclusionFinalMessage`) deve mencioná-lo junto do objetivo textual (ex.: `Seu objetivo "comprar uma casa" (meta de R$ 400.000,00) está registrado.`); quando não houver valor, a mensagem final permanece idêntica à atual (só objetivo). O formato exato do valor exibido é decisão de implementação.
- RF-16: A working memory markdown gravada na conclusão (`"## Objetivo Financeiro\n\n" + Goal`) permanece **inalterada** — o valor não é incluído nesse markdown nem injetado no system prompt de conversas futuras. O valor estruturado vive exclusivamente no `metadata` (RF-11).

## Experiência do Usuário

- Fluxo feliz (valor junto): usuário descreve meta e valor na mesma mensagem → sistema avança direto ao próximo step, sem eco por campo e sem pergunta adicional sobre valor; o valor é confirmado ao usuário apenas na mensagem final de conclusão (RF-15).
- Fluxo alternativo (só meta, sem valor): usuário descreve só a meta válida → sistema faz uma pergunta amigável específica pedindo o valor, deixando claro que é opcional → usuário informa número (salvo) ou recusa (avança) → onboarding continua.
- Fluxo alternativo (nem meta nem valor): usuário manda algo sem objetivo identificável → sistema faz **uma repergunta combinada** pedindo objetivo e, opcionalmente, o valor → captura o objetivo (obrigatório para avançar) e o valor se vier; o valor não é pedido de novo.
- Caso de borda (valor inválido/recusa): usuário responde algo não numérico ou recusa → tratado como "não informado", sem erro técnico → nenhuma repergunta de valor além da única permitida → avanço garantido.
- A mensagem inicial do `step-goal` convida o usuário a incluir o valor como exemplo, reduzindo a probabilidade de precisar da repergunta.

## Restrições Técnicas de Alto Nível

- Reutiliza o parser LLM-assisted existente (`agent.Agent.Execute` com `llm.Schema` strict, padrão de `goalSchema`/`incomeSchema`); nenhuma dependência externa nova, apenas extensão de schema e/ou segunda chamada.
- OpenRouter permanece como único provider LLM; o parsing é a única call-site LLM sancionada envolvida — nenhuma regra de negócio de extração deve viver fora dos constructors puros (DMMF: `Decide*` puro, validação em smart constructor).
- Persistência via `WorkingMemoryRepository.UpsertMetadata` (merge JSONB `||`), na coluna `metadata JSONB` de `platform_resources`, que já aceita novas chaves — **sem migration nova**.
- O valor trafega no `OnboardingState` serializado no `Snapshot.State` do kernel de workflow (`internal/platform/workflow`), correlacionado por `state.UserID`; nenhuma nova estrutura de correlação.
- Estados e validações seguem os gates de governança do projeto (R-AGENT-WF-001 para o consumidor `internal/agents`; zero comentários em Go de produção; `Decide*` puro/determinístico sem IO).
- Nenhuma permissão nova: mantém o controle de acesso atual do onboarding, escopado por `state.UserID`.

## Fora de Escopo

- Exibir, usar ou calcular o valor da meta em outras partes do produto (relatórios, alertas proativos, orçamento) — esta entrega captura o valor, salva no `metadata` e o menciona apenas na mensagem final de conclusão do onboarding (RF-15); nenhum outro uso.
- Injetar o valor na working memory / system prompt para conversas futuras (RF-16 mantém a WM markdown inalterada) — fica para uma entrega futura, se necessário.
- Criar um tipo/VO de domínio "Goal"/"Meta" — `internal/agents/domain/` não existe hoje e não foi solicitado.
- Editar o valor da meta fora do fluxo de onboarding (ex.: comando posterior via WhatsApp).
- Alterar o comportamento de `step-income` ou de `DecideIncomeCents`, que permanecem como estão.
- Suportar valores por extenso ("quatrocentos mil", "meio milhão") — fora do conjunto de formatos suportado (RF-09).
- Impor teto máximo de sanidade ao valor da meta (RF-07 aceita qualquer valor positivo).

## Suposições e Questões em Aberto

- Decisão (chave e unidade de persistência): quando informado, o valor é gravado em `platform_resources.metadata` sob `"objetivo_financeiro_valor_centavos"`, inteiro em centavos (espelha `IncomeCents`); quando nenhum valor válido for informado, a chave é omitida (RF-12), preservando o payload atual. O nome do campo em `OnboardingState` é decisão de nomenclatura da techspec, mas a unidade é fixada como centavos (`int64`).
- Decisão (composição da repergunta): objetivo permanece obrigatório (zero regressão); o valor é opcional e perguntado no máximo uma vez por execução do `step-goal` — via repergunta **combinada** quando falta o objetivo, ou repergunta específica de valor quando só falta o valor (RF-03.1/RF-03.2/RF-03.3).
- Decisão (modelo do gate): o gate ≥ 0.90 de RF-14 é medido no modelo de produção `openai/gpt-4o-mini`; modelos mais fortes não satisfazem o gate.
- Decisão (surfacing conversacional): sem eco por campo no `step-goal` (RF-13.1); o valor é mencionado apenas na mensagem final de conclusão quando presente (RF-15); a working memory markdown permanece inalterada (RF-16).
- Suposição: o número de casos e a composição exata do harness de RF-14 (incluindo variações de formato de RF-09) serão definidos na especificação técnica; o PRD fixa o alvo ≥ 0.90 e o modelo `openai/gpt-4o-mini`.
- Sem questões em aberto pendentes de decisão do usuário: as ambiguidades materiais (métrica/modelo de sucesso, formatos suportados, teto de valor, recusa na primeira mensagem, composição das reperguntas, chave/unidade de persistência, eco no step-goal, menção na mensagem final, inclusão na working memory) foram todas resolvidas nesta sessão.
