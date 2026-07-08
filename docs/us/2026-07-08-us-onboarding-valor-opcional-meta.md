# US-01: Informar valor opcional da meta financeira no onboarding

## Declaração
Como usuário do MeControla em processo de onboarding, quero opcionalmente informar quanto (em R$) representa a minha meta financeira ao descrevê-la no `step-goal`, para que o MeControla registre esse valor junto ao meu objetivo e eu tenha um número concreto para acompanhar meu progresso mais adiante.

## Contexto
- Problema: Hoje o `step-goal` do onboarding (`internal/agents/application/workflows/onboarding_workflow.go:492-521`) captura apenas o texto livre da meta (`state.Goal`, validado por `DecideGoal` em `onboarding_workflow.go:161-167`) e o persiste em `platform_resources.metadata` sob a chave `"objetivo_financeiro"` (`onboarding_workflow.go:774-779`). Não existe hoje nenhum campo, parser ou persistência para um valor monetário associado à meta.
- Resultado esperado: quando o usuário mencionar um valor junto da meta (ex.: "Eu quero comprar uma casa, e tenho a meta de R$ 400.000,00" ou "quero quitar minhas dívidas, preciso de 10 mil reais"), o MeControla extrai e salva esse valor. Quando o usuário não mencionar valor algum, o fluxo pergunta uma única vez se ele quer informar um valor, sem bloquear o onboarding.
- Fonte: solicitação direta do usuário via `/user-stories`, confrontada com o mapeamento de código feito nesta sessão sobre `onboarding_workflow.go`, `onboarding_workflow_test.go`, `working_memory_repository.go` e migrations de `platform_resources`.

## Regras de Negócio
- O valor da meta é **opcional**: sua ausência nunca deve bloquear o avanço do onboarding (decisão confirmada pelo usuário).
- Extração em uma única mensagem: quando o usuário já informa meta e valor juntos, ambos devem ser extraídos em uma única chamada ao parser LLM-assisted, sem repergunta — seguindo o padrão de extração já usado em `DecideGoal`/`goalSchema` (`onboarding_workflow.go:161-167,359-366`) e no precedente de valor monetário `DecideIncomeCents`/`incomeSchema` (`onboarding_workflow.go:169-174,368-379`, usado no `step-income`).
- Quando o usuário **não menciona nenhum valor** junto da meta, o sistema deve repergunter **uma única vez**, pedindo especificamente o valor (podendo o usuário responder com um número ou recusar, ex.: "não"). Após essa única repergunta, o fluxo avança independentemente da resposta (decisão confirmada pelo usuário).
- Quando o usuário informa um valor **claramente inválido** (negativo, zero, ou texto não numérico como "não sei quanto"), o sistema deve tratá-lo **como se nenhum valor tivesse sido informado** — ou seja, cai na mesma regra de repergunta única acima, sem bloquear e sem mensagem de erro técnico (decisão confirmada pelo usuário).
- Nunca deve haver mais de uma repergunta pelo valor da meta: se a resposta à repergunta única também for inválida ou for uma recusa, o onboarding avança sem valor, sem novo loop.
- Validação de valor monetário (quando informado e não recusado) deve seguir o mesmo princípio de `DecideIncomeCents` (converter para inteiro em centavos, validar valor positivo), mas como constructor **novo e distinto**, pois `DecideIncomeCents` hoje trata ausência/zero como erro obrigatório (`onboarding_workflow.go:169-174`), enquanto o valor da meta deve tratar ausência/zero como "não informado", não como erro.
- O valor da meta deve sobreviver no estado do workflow (`OnboardingState`, `onboarding_workflow.go:146-159`) até o `step-conclusion`, da mesma forma que `state.Goal` sobrevive hoje, pois a persistência final ocorre apenas em `BuildConclusionStep` (`onboarding_workflow.go:774-779`).

## Critérios de Aceite
```gherkin
Cenário: Usuário informa meta e valor na mesma mensagem
  Dado que o usuário está no step-goal do onboarding (Phase = goal)
  Quando ele responde "Eu quero comprar uma casa, e tenho a meta de R$ 400.000,00"
  Então o sistema extrai o objetivo "comprar uma casa" e o valor de R$ 400.000,00 (400000000 centavos)
  E não faz nenhuma repergunta sobre o valor
  E avança para o próximo step do onboarding com ambos os dados retidos no estado do workflow

Cenário: Usuário informa apenas a meta, sem valor
  Dado que o usuário está no step-goal do onboarding (Phase = goal)
  Quando ele responde "quero quitar minhas dívidas"
  Então o sistema extrai o objetivo "quitar minhas dívidas" sem valor associado
  E repergunta uma única vez pedindo o valor da meta
  E, se o usuário responder com um valor numérico válido (ex.: "10 mil reais"), o sistema salva 1000000 centavos e avança
  E, se o usuário responder "não" ou qualquer coisa não numérica, o sistema avança sem valor, sem nova repergunta

Cenário: Usuário informa um valor inválido para a meta
  Dado que o usuário está no step-goal do onboarding (Phase = goal)
  Quando ele responde "quero viajar, mas não sei quanto vou gastar"
  Então o sistema trata a ausência de um valor numérico como "valor não informado"
  E repergunta uma única vez pedindo o valor da meta, exatamente como no cenário de meta sem valor
  E, independentemente da resposta à repergunta, o onboarding avança sem bloquear
```

## Dados e Permissões
- Dados obrigatórios: `Goal` (string, já validado por `DecideGoal`, não alterado por esta história).
- Dados opcionais (novos): valor da meta em centavos (ex.: novo campo `GoalAmountCents int64` em `OnboardingState`, seguindo o padrão de `IncomeCents int64` já existente na linha 153 de `onboarding_workflow.go`); ausência representada como `0`/não setado.
- Perfis/permissões: nenhuma permissão nova; segue o mesmo controle de acesso do onboarding hoje, escopado por `state.UserID` como chave de correlação do workflow (`internal/platform/workflow/store.go:17-32`, campo `CorrelationKey`).

## Dependências
- Depende do parser LLM-assisted já existente (`agent.Agent.Execute` com `llm.Schema` strict) usado em `goalSchema`/`incomeSchema` (`onboarding_workflow.go:359-410`) — nenhuma dependência externa nova, apenas extensão do schema/segunda chamada.
- Depende de `WorkingMemoryRepository.UpsertMetadata` (`internal/platform/memory/infrastructure/postgres/working_memory_repository.go:75-102`), que já faz merge JSONB (`||`) e não exige migration nova, pois a coluna `metadata JSONB` de `platform_resources` já aceita novas chaves livremente.
- Depende da criação de uma nova smart-constructor pura (ex.: `DecideGoalAmount`) distinta de `DecideIncomeCents`, pois as regras de "ausência é válida" e "zero não é erro, e sim ausência" divergem da regra de renda existente.

## Fora de Escopo
- Não inclui exibir, usar ou calcular o valor da meta em outras partes do produto (relatórios, alertas proativos, orçamento) — apenas capturar e salvar no metadata, conforme solicitado.
- Não inclui criar um tipo/VO de domínio "Goal" — não existe hoje (`internal/agents/domain/` nem sequer existe como diretório) e não foi solicitado.
- Não inclui edição posterior do valor da meta fora do fluxo de onboarding (ex.: comando para atualizar depois via WhatsApp).
- Não inclui alterar o comportamento de `step-income` ou de `DecideIncomeCents`, que permanecem como estão.
- Não inclui alterar a mensagem `_welcomeGoalPrompt`/`_goalReprompt` além do necessário para mencionar o valor opcional como exemplo — o texto exato da nova repergunta de valor é decisão de implementação (técnica), não desta história.

## Evidências
- Base de código:
  - `internal/agents/application/workflows/onboarding_workflow.go:24-32` — constantes dos 7 steps do onboarding, incluindo `step-goal`.
  - `internal/agents/application/workflows/onboarding_workflow.go:146-159` — struct `OnboardingState`, sem campo de valor de meta hoje.
  - `internal/agents/application/workflows/onboarding_workflow.go:161-167` — `DecideGoal`, valida apenas texto não vazio.
  - `internal/agents/application/workflows/onboarding_workflow.go:169-174` — `DecideIncomeCents`, precedente de smart constructor para valor monetário obrigatório (>0).
  - `internal/agents/application/workflows/onboarding_workflow.go:359-410` — `goalSchema` e `incomeSchema`, padrão de JSON Schema strict usado na extração LLM-assisted.
  - `internal/agents/application/workflows/onboarding_workflow.go:412-417` — prompts atuais `_welcomeGoalPrompt` e `_goalReprompt`.
  - `internal/agents/application/workflows/onboarding_workflow.go:492-521` — corpo do `step-goal` (suspensão, parsing, decisão de avanço).
  - `internal/agents/application/workflows/onboarding_workflow.go:774-779` — persistência de `state.Goal` em `platform_resources.metadata` sob a chave `"objetivo_financeiro"`, via `UpsertMetadata`.
  - `internal/platform/memory/infrastructure/postgres/working_memory_repository.go:75-102` — `UpsertMetadata`, merge JSONB (`||`) que permite adicionar novas chaves sem migration.
  - `internal/platform/workflow/store.go:17-32` — `Snapshot`, campo `State []byte` (serialização JSON do `OnboardingState`) e `CorrelationKey` (userID).
  - `internal/agents/application/workflows/onboarding_workflow_test.go:60-94` — `TestDecideGoal`, 3 cenários (vazio, espaços, texto válido).
  - `internal/agents/application/workflows/onboarding_workflow_test.go:345-417` — `TestBuildGoalStep`, 3 cenários (primeira mensagem, resume válido, resume vazio).
- Entrada: solicitação do usuário especificando exemplos "Eu quero comprar uma casa, e tenho a meta de R$ 400.000,00" e "quero quitar minhas dívidas e preciso de 10 mil reais".
- Entrada (decisões confirmadas via pergunta de múltipla escolha nesta sessão): (1) valor ausente → repergunta única antes de avançar; (2) valor inválido → tratado como ausente, caindo na mesma repergunta única.
- Inferências: nome do novo campo em `OnboardingState` (`GoalAmountCents`) e nome da nova chave de metadata (ex.: `meta_valor_cents`) são propostos por analogia ao padrão existente (`IncomeCents`, `objetivo_financeiro`), mas a nomenclatura final é decisão de implementação/techspec.
- Não evidenciado: não existe PRD específico de onboarding em `.specs/` cobrindo o `step-goal` (buscado em `.specs/`, apenas menções genéricas a "onboarding" em `.specs/prd-alertas-proativos/prd.md:191,219-220,238`); não existe tipo de domínio "Goal"/"Meta" em `internal/agents/domain/` (diretório não existe).

## Notas de Validação
- Persona única (usuário em onboarding) e capacidade única (informar valor opcional da meta) mantidas em uma só história, pois os três cenários (com valor, sem valor, valor inválido) formam um único incremento de valor indivisível — não podem ser entregues parcialmente sem quebrar a consistência do fluxo de repergunta única.
- Os 3 cenários Gherkin cobrem fluxo feliz (valor informado corretamente), fluxo alternativo (valor ausente com repergunta) e um caso de borda equivalente a erro de entrada (valor inválido tratado como ausente) — não há bloqueio/erro técnico distinto a cobrir além desses, pois a regra de negócio confirmada é justamente nunca bloquear por causa do valor.
