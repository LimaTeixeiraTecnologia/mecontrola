# US-001: Nome de Tratamento do Usuário

## Declaração
Como usuário do MeControla que conversa pelo WhatsApp (no onboarding e no dia a dia), quero informar e alterar por linguagem natural o nome pelo qual desejo ser chamado, para que o agente se comunique comigo de forma próxima e personalizada usando esse nome, sem alterar meus dados cadastrais ou de cobrança.

## Contexto
- Problema: hoje o agente não pergunta nem armazena um nome de tratamento. O primeiro contato do onboarding é a mensagem combinada de boas-vindas + objetivo (`welcomeCombinedPrompt` em `internal/agents/application/workflows/onboarding_workflow.go:696`), exibida no passo de objetivo (`BuildGoalStep`, `onboarding_workflow.go:1017`); o passo de boas-vindas atual é um no-op (`BuildWelcomeStep`, `onboarding_workflow.go:1009`). O único "nome" existente é o `display_name` do módulo `identity` (nome civil/cadastral, `internal/identity/infrastructure/http/server/handlers/upsert_user_by_whatsapp_handler.go:38`), que é conceito distinto e não deve ser reaproveitado como nome de tratamento.
- Resultado esperado: o agente pergunta no início do onboarding como o usuário gostaria de ser chamado, extrai o nome/apelido de linguagem natural, persiste o valor de forma estruturada em `platform_resources` (metadata JSONB) e também em uma seção do `working_memory`, e passa a usar o nome vigente nas interações. O usuário pode trocar o nome a qualquer momento por linguagem natural, com aplicação imediata. Toda mensagem segue o Tom de Voz oficial do MeControla.
- Fonte: `US_Nome_de_Tratamento_do_Usuario_MeControla.md` (entrada do usuário) confrontada com a base de código do repositório `mecontrola`.

## Regras de Negócio
- RN-01 — Captura no início do onboarding: um novo passo dedicado (`step-name`) precede o passo de objetivo. Ele apresenta a mensagem de boas-vindas e pergunta, no Tom de Voz oficial: "Antes da gente começar, como você gostaria que eu te chamasse? 💚". O passo de objetivo deixa de duplicar a saudação de boas-vindas. Referência estrutural: passos suspensivos do `onboarding_workflow.go` (`suspendStep`, `onboarding_workflow.go:993`).
- RN-02 — Extração por linguagem natural: o nome de tratamento é identificado e extraído a partir de formas livres ("Stefany", "Pode me chamar de Stef", "Me chama de Stef", "Prefiro Stef", "Só Stef mesmo"), usando Structured Output com schema estrito, no mesmo padrão dos demais passos do onboarding (`a.Execute(ctx, agent.Request{... Schema: &llm.Schema{Strict: true ...}})`, `onboarding_workflow.go:1027`). O valor extraído é apenas o nome/apelido de tratamento, que não precisa ser o nome civil ou completo.
- RN-03 — Persistência dual-write (estruturada + utilizável): ao capturar ou alterar o nome, o sistema grava simultaneamente (a) a chave estruturada `metadata["nome_tratamento"]` em `platform_resources` via `WorkingMemory.UpsertMetadata` (merge JSONB `metadata || EXCLUDED.metadata`, `internal/platform/memory/infrastructure/postgres/working_memory_repository.go:75`) e (b) uma seção `## Nome de Tratamento` no `working_memory` TEXT via `WorkingMemory.Upsert`, espelhando o padrão já usado para `objetivo_financeiro` (`onboarding_workflow.go:1563-1572`). A seção no `working_memory` é obrigatória porque o runtime injeta no system prompt do agente somente a coluna `working_memory`, nunca o `metadata` JSONB (`internal/platform/agent/runtime.go:308-312`).
- RN-04 — Valor único e vigente por usuário: existe no máximo um nome de tratamento vigente por usuário (chave `resource_id` = `userID`, PK de `platform_resources`, `migrations/000001_initial_schema.up.sql:2340`). Uma alteração substitui o valor anterior tanto no `metadata` quanto na seção do `working_memory` (substituição de seção no padrão de `goalEditReplaceSection`, `internal/agents/application/workflows/goal_edit_workflow.go:241`).
- RN-05 — Opcionalidade no onboarding: se o usuário não informar um nome utilizável (ex.: "não", "tanto faz", ou responde diretamente sobre o objetivo), o onboarding prossegue sem nome de tratamento e nunca é bloqueado, espelhando o precedente do valor de meta opcional (`GoalValueAsked`, `onboarding_workflow.go:1049`). Nesse caso o agente trata o usuário sem nome, de forma neutra.
- RN-06 — Uso natural nas interações: quando houver nome de tratamento vigente, o agente o usa nas respostas de forma natural e coerente com o contexto, sem repetir o nome de forma excessiva ou artificial na mesma interação, preservando a fluidez e o Tom de Voz oficial. O nome fica disponível ao LLM por já estar na seção `## Nome de Tratamento` do `working_memory` injetado (`runtime.go:308-312`).
- RN-07 — Alteração por linguagem natural sem comando fixo: o agente reconhece a intenção de alterar o nome mesmo sem comando específico ("Quero trocar como você me chama", "Muda como você me chama", "Quero mudar meu apelido", "A partir de agora quero que me chame de outro nome"), no mesmo modelo de detecção de intenção do fluxo de edição existente (`goal_edit_workflow.go`).
- RN-08 — Alteração com nome já informado (sem re-pergunta): quando a mensagem de alteração já contém o novo nome ("Agora me chama de Stef", "Troca meu nome para Stef", "De agora em diante me chama de Tefy"), o agente não pergunta novamente; aplica a alteração e confirma com o novo nome ("Combinado, Stef! 💚 Vou te chamar assim daqui pra frente.").
- RN-09 — Alteração sem nome informado (pergunta única): quando a intenção de alteração não traz o novo nome, o agente pergunta uma vez "Claro! Como você gostaria que eu te chamasse a partir de agora? 💚" e, ao receber a resposta, aplica e confirma. Não há gate de confirmação sim/não, pois o nome de tratamento é personalização não-destrutiva, fora do escopo HITL de operações sensíveis do `R-AGENT-WF-001.7-A` (`.claude/rules/agent-workflows-tools.md`).
- RN-10 — Aplicação imediata: após a atualização, o novo nome é considerado imediatamente e todas as interações seguintes usam o valor vigente, pois a seção do `working_memory` é relida a cada execução (`runtime.buildMessages`, `runtime.go:304`).
- RN-11 — Isolamento de dados: a alteração do nome de tratamento não modifica o `display_name` do módulo `identity` nem quaisquer dados cadastrais ou de cobrança; o nome de tratamento vive exclusivamente em `platform_resources` (camada de agents/memory), separado do agregado `User` do `identity`.
- RN-12 — Validação do valor: o nome extraído é normalizado por `strings.TrimSpace`; valor vazio após trim, ou marcador de recusa, não é persistido (equivale a "sem nome utilizável", RN-05); o comprimento persistido respeita o limite da coluna `resource_id`-scoped em `platform_resources` sem violar as restrições existentes da tabela (`migrations/000001_initial_schema.up.sql:2340-2347`).

## Critérios de Aceite
```gherkin
Cenário: Captura do nome no início do onboarding e persistência dual-write
  Dado um usuário iniciando o onboarding pelo WhatsApp
  Quando o agente apresenta o passo inicial
  Então a primeira mensagem é a de boas-vindas seguida da pergunta "Antes da gente começar, como você gostaria que eu te chamasse? 💚"
  E quando o usuário responde "Pode me chamar de Stef"
  Então o sistema extrai "Stef" como nome de tratamento
  E grava metadata["nome_tratamento"]="Stef" em platform_resources
  E grava uma seção "## Nome de Tratamento" com valor "Stef" no working_memory do usuário
  E o onboarding avança para o passo de objetivo

Cenário: Uso do nome vigente nas interações seguintes
  Dado um usuário com nome de tratamento "Stefany" vigente na seção "## Nome de Tratamento" do working_memory
  Quando o usuário registra um lançamento pelo WhatsApp
  Então a resposta do agente utiliza o nome "Stefany" de forma natural (por exemplo "Prontinho, Stefany! Seu lançamento foi registrado. ✅")
  E o nome não é repetido de forma excessiva na mesma interação
  E a mensagem segue o Tom de Voz oficial

Cenário: Alteração com o novo nome já informado na mesma mensagem
  Dado um usuário com nome de tratamento "Stefany" vigente
  Quando o usuário envia "Agora me chama de Stef"
  Então o agente não pergunta novamente qual nome usar
  E substitui metadata["nome_tratamento"] e a seção "## Nome de Tratamento" para "Stef"
  E confirma com "Combinado, Stef! 💚 Vou te chamar assim daqui pra frente."
  E todas as interações seguintes usam "Stef"

Cenário: Alteração sem o novo nome informado (pergunta única, sem confirmação sim/não)
  Dado um usuário com nome de tratamento "Stefany" vigente
  Quando o usuário envia "Quero trocar como você me chama"
  Então o agente pergunta "Claro! Como você gostaria que eu te chamasse a partir de agora? 💚"
  E quando o usuário responde "Pode me chamar de Stef"
  Então o agente aplica a alteração e confirma com "Combinado, Stef! 💚 Vou te chamar assim daqui pra frente."
  E não solicita confirmação adicional do tipo sim/não

Cenário: Usuário não informa nome utilizável no onboarding (opcional, não bloqueia)
  Dado um usuário no passo inicial do onboarding
  Quando o usuário responde "tanto faz" ou já fala diretamente do objetivo
  Então o sistema não persiste nenhum nome de tratamento
  E o onboarding prossegue normalmente para o passo de objetivo
  E o agente passa a tratar o usuário sem nome, de forma neutra

Cenário: Isolamento — alterar nome de tratamento não altera dados cadastrais
  Dado um usuário com display_name cadastral "Stefany Lima" no módulo identity
  Quando o usuário troca o nome de tratamento para "Stef"
  Então metadata["nome_tratamento"] e a seção do working_memory passam a "Stef"
  E o display_name cadastral em identity permanece "Stefany Lima"
  E nenhum dado de cobrança é alterado

Cenário: Falha ao persistir o nome de tratamento
  Dado um usuário informando o nome de tratamento
  Quando a gravação em platform_resources (Upsert ou UpsertMetadata) falha
  Então o passo falha explicitamente (StepStatusFailed), sem confirmar sucesso ao usuário
  E o nome de tratamento anterior permanece inalterado
```

## Dados e Permissões
- Dados obrigatórios: `resource_id` (= `userID`) como chave em `platform_resources`; valor do nome de tratamento (string não-vazia após trim) quando informado.
- Estruturas afetadas: `platform_resources.metadata` (chave `nome_tratamento`) e `platform_resources.working_memory` (seção `## Nome de Tratamento`).
- Perfis/permissões: apenas o próprio usuário, autenticado pelo canal WhatsApp; não há perfil administrativo envolvido e não há alteração no módulo `identity`.

## Dependências
- `memory.WorkingMemory` com `Upsert` e `UpsertMetadata` já disponível (`internal/platform/memory/ports.go:18`) e implementado em Postgres (`internal/platform/memory/infrastructure/postgres/working_memory_repository.go:51-102`).
- Tabela `platform_resources` já criada (`migrations/000001_initial_schema.up.sql:2340`).
- Padrão de passo suspensivo/durável do onboarding e de substituição de seção do working_memory (`onboarding_workflow.go:993`; `goal_edit_workflow.go:241`).
- Structured Output do LLM via `agent.Agent.Execute` com `llm.Schema{Strict: true}` (`onboarding_workflow.go:1027`).
- Roteamento do fluxo de alteração pós-onboarding pelo agente diário via registry/tool/workflow, análogo ao `goal-edit` (`goal_edit_workflow.go:269` `ContinueGoalEdit`) — este wiring de roteamento é dependência de implementação a materializar, seguindo `R-AGENT-WF-001.1` (sem `switch case intent.Kind`).

## Fora de Escopo
- Alterar `identity.display_name` ou qualquer dado cadastral/de cobrança do usuário.
- Injetar a coluna `metadata` JSONB no system prompt do runtime (a utilização pelo LLM ocorre via seção do `working_memory`, não por mudança no kernel de plataforma).
- Histórico/versionamento de nomes de tratamento anteriores.
- Nome de tratamento distinto por canal ou por thread (o valor é escopo `resource`, único por usuário).
- Inferência de gênero, flexão ou tradução do nome informado.

## Evidências
- Entrada: `US_Nome_de_Tratamento_do_Usuario_MeControla.md` (seções 1 a 8: captura, identificação, uso, edição, fluxo de alteração, alteração direta, persistência, critérios de aceite).
- Base de código:
  - `internal/platform/agent/runtime.go:304-328` — `buildMessages` injeta apenas `working_memory` no system prompt; `metadata` JSONB não é injetado.
  - `migrations/000001_initial_schema.up.sql:2340-2347` — DDL de `platform_resources(resource_id PK, working_memory TEXT, metadata JSONB DEFAULT '{}', updated_at)`.
  - `internal/platform/memory/ports.go:18-22` — interface `WorkingMemory` com `Get`, `Upsert`, `UpsertMetadata`.
  - `internal/platform/memory/infrastructure/postgres/working_memory_repository.go:51-102` — `Upsert` e `UpsertMetadata` (merge JSONB `metadata || EXCLUDED.metadata`).
  - `internal/agents/application/workflows/onboarding_workflow.go:1563-1572` — padrão dual-write existente (`## Objetivo Financeiro` no working_memory + chave `objetivo_financeiro` no metadata).
  - `internal/agents/application/workflows/onboarding_workflow.go:696,1009,1017,1049,1643-1657` — welcome combinado, welcome no-op, passo de objetivo, opcionalidade de valor de meta e montagem sequencial do workflow.
  - `internal/agents/application/workflows/goal_edit_workflow.go:23-24,241,269` — padrão de edição pós-onboarding (heading de seção, `goalEditReplaceSection`, `ContinueGoalEdit`).
  - `internal/agents/application/scorers/mecontrola_scorers.go` e `internal/agents/application/scorers/behavioral_scorers.go` — Tom de Voz oficial codificado como scorer LLM-judged (`tone_adherence`) e scorer determinístico (negrito com asterisco simples, emojis oficiais).
  - `internal/identity/infrastructure/http/server/handlers/upsert_user_by_whatsapp_handler.go:38` — `display_name` cadastral do `identity`, conceito distinto do nome de tratamento.
  - `.claude/rules/agent-workflows-tools.md` (`R-AGENT-WF-001.1`, `R-AGENT-WF-001.7-A`) — roteamento por registry e escopo do gate HITL restrito a operações destrutivas/sensíveis.
- Inferências: (1) o novo nome de tratamento é net-new e não colide com chaves de metadata existentes; a chave `nome_tratamento` é proposta por consistência com `objetivo_financeiro`. (2) A validação de comprimento adota `strings.TrimSpace` e rejeição de vazio por consistência com os demais `Decide*` do onboarding; o limite exato de caracteres pode ser fixado na techspec sem alterar o comportamento observável desta história.
- Não evidenciado: nenhum campo, seção de working_memory ou chave de metadata de nome de tratamento existe hoje na base de código (busca por "nome de tratamento", "te chamasse", "PreferredName", "TreatmentName", "displayName" retornou apenas o `display_name` cadastral do `identity`).

## Notas de Validação
- História única e coesa (uma capacidade de personalização: capturar, usar e alterar o nome de tratamento), com cenários de fluxo feliz, fluxos alternativos (alteração com e sem nome informado; ausência de nome) e fluxos de erro/isolamento (falha de persistência; não-alteração de dados cadastrais).
- Todas as afirmações técnicas estão ancoradas em caminho e linha da base de código ou marcadas explicitamente como inferência.
- Decisões confirmadas pelo usuário (múltipla escolha): persistência dual-write (metadata + seção do working_memory); captura em passo dedicado antes do objetivo; nome opcional no onboarding (não bloqueia); alteração aplicada direto com confirmação, sem gate sim/não.
- Sem marcadores pendentes; sem lacunas materiais conhecidas.
