# US-001: Dicionário pessoal — aprender apelidos de categoria por usuário

## Declaração
Como usuário do MeControla no WhatsApp, quero que o app aprenda como eu chamo meus gastos e receitas no momento em que eu confirmo a categoria de um lançamento, para que da próxima vez o mesmo termo seja categorizado automaticamente sem repetir a mesma pergunta.

## Contexto
- Problema: o catálogo e o dicionário de categorias são globais e editoriais, sem escopo de usuário (`migrations/000001_initial_schema.up.sql:432` cria `mecontrola.categories` sem coluna `user_id`; `:485` cria `mecontrola.category_dictionary` também sem `user_id`). Quando o termo do usuário não casa com a taxonomia curada, `SearchDictionary` retorna `no_match` (`internal/categories/application/usecases/search_dictionary.go:61-67`) e o agente pede a categoria manualmente. A escolha confirmada é apenas carimbada como evidência no lançamento (`CategorySource="user_selected_candidate"`, `CategoryMatchedTerm`, `CategoryScore` em `internal/agents/application/workflows/pending_entry_workflow.go:663-671`), mas nada é persistido como conhecimento reaproveitável. Na mensagem seguinte o mesmo texto passa de novo pela mesma busca global (`internal/agents/application/usecases/register_attempt.go:352` → `search_dictionary.go:61`) e cai novamente em `no_match`. O usuário repete a mesma correção indefinidamente.
- Resultado esperado: após o usuário confirmar uma categoria para um termo que não teve correspondência automática inequívoca, o app grava um apelido pessoal (por usuário) associando aquele termo à categoria/subcategoria escolhida; em mensagens futuras desse mesmo usuário, o termo passa a resolver automaticamente para a categoria aprendida, reduzindo a fricção de reperguntar.
- Fonte: análise do módulo `internal/categories` (somente leitura) e do consumidor `internal/agents`, solicitada em 2026-07-11; direção confirmada pelo usuário em rodada de esclarecimento (gap = dicionário pessoal / aprender apelidos; persona = usuário final via WhatsApp).

## Regras de Negócio
- RN-01: O aprendizado é implícito e só ocorre quando o termo do lançamento não teve correspondência automática inequívoca — ou seja, quando a busca global retornou `no_match`, quando o casamento foi apenas por token/fuzzy, ou quando o usuário escolheu categoria diferente da proposta de maior score. Termos que já resolvem por match exato de alta confiança no dicionário global não geram apelido, para não duplicar conhecimento já existente (`internal/categories/domain/valueobjects/match_quality.go`, `search_outcome.go`; proposta em `register_attempt.go:352`).
- RN-02: O apelido é estritamente por usuário (escopo `resourceId`/`user_id`). Ele nunca altera, cria ou deprecia entradas do dicionário global nem afeta a categorização de outros usuários (`category_dictionary` global permanece intocado — `migrations/000001_initial_schema.up.sql:485`).
- RN-03: A chave do apelido é `(user_id, kind, termo_normalizado)` e o valor é `(root_category_id, subcategory_id)`. A normalização do termo reutiliza a mesma regra do dicionário global: minúsculo sem acento (`term_normalized GENERATED ALWAYS AS lower(immutable_unaccent(term))` em `migrations/000001_initial_schema.up.sql:490`).
- RN-04: Reensinar o mesmo termo faz upsert do apelido existente (a última confirmação vence); não são criados apelidos duplicados para a mesma chave.
- RN-05: Ao classificar uma nova mensagem, o apelido pessoal do usuário é consultado e tem precedência de match exato sobre o dicionário global. Havendo apelido válido, o agente propõe diretamente a categoria aprendida (fluxo de confirmação) em vez de retornar `no_match`.
- RN-06: O apelido referencia uma categoria/subcategoria que precisa ser válida na versão editorial vigente. Se o alvo estiver deprecado ou a versão editorial tiver mudado, o apelido é ignorado e revalidado, e o fluxo normal (busca global + pergunta ao usuário) assume o controle (validação existente em `internal/categories/application/usecases/resolve_category_for_write.go:53` `ErrVersionDrift` e `:120` categoria inativa).
- RN-07: O apelido sempre aponta para uma subcategoria folha, nunca para a categoria raiz sozinha, coerente com o gate de escrita atual que rejeita subcategoria igual à raiz (`internal/agents/application/workflows/pending_entry_workflow.go:482`; `resolve_category_for_write.go:98-107` `ErrRootWithoutLeaf`).
- RN-08: O apelido só é gravado após a confirmação explícita do lançamento pelo usuário. Cancelamento, expiração ou reprompt não geram aprendizado (ponto de confirmação em `internal/agents/application/workflows/pending_entry_workflow.go:370` `ConfirmActionAccept`).
- RN-09: Apenas o próprio usuário dono lê e escreve seus apelidos; toda operação exige gateway auth mais principal de usuário, como já vigora no módulo de categorias (`internal/categories/openapi.yaml:6-7`; `internal/categories/module.go:29`).

## Critérios de Aceite
```gherkin
Cenário: Aprender apelido ao confirmar categoria de um termo não reconhecido
  Dado que sou um usuário autenticado no WhatsApp
  E que envio "gastei 50 no petshop do bairro"
  E que o dicionário global retorna no_match para "petshop do bairro"
  E que o agente me pergunta a categoria e eu escolho "Pets > Serviços"
  Quando eu confirmo o lançamento com "sim"
  Então a despesa é registrada em "Pets > Serviços"
  E é gravado um apelido pessoal associando "petshop do bairro" à subcategoria "Pets > Serviços" apenas para o meu usuário

Cenário: Termo aprendido resolve automaticamente na próxima mensagem
  Dado que eu já ensinei que "petshop do bairro" é "Pets > Serviços"
  Quando envio "gastei 80 no petshop do bairro"
  Então o agente propõe diretamente "Pets > Serviços" para confirmação
  E não retorna no_match nem repete a pergunta de categoria

Cenário: Reensinar o mesmo termo atualiza o apelido sem duplicar
  Dado que "mercadinho" está aprendido como "Alimentação > Supermercado"
  Quando envio um novo lançamento com "mercadinho" e corrijo para "Alimentação > Feira"
  E confirmo o lançamento
  Então o apelido pessoal de "mercadinho" passa a apontar para "Alimentação > Feira"
  E continua existindo apenas um apelido para "mercadinho" no meu usuário

Cenário: Apelido apontando para categoria deprecada é ignorado
  Dado que eu tenho o apelido "vr do mês" apontando para uma subcategoria que foi deprecada editorialmente
  Quando envio "recebi 600 de vr do mês"
  Então o apelido é ignorado por referenciar categoria inválida
  E o agente segue o fluxo normal de busca global e me pergunta a categoria

Cenário: Não confirmar o lançamento não gera aprendizado
  Dado que o agente me pergunta a categoria de "assinatura xyz" e eu escolho "Lazer > Streaming"
  Quando eu respondo "cancelar" em vez de confirmar
  Então o lançamento não é registrado
  E nenhum apelido pessoal é gravado para "assinatura xyz"
```

## Dados e Permissões
- Dados obrigatórios: `user_id` (header `X-User-ID`), `kind` (`income` ou `expense`), termo original e termo normalizado, `root_category_id`, `subcategory_id` e a versão editorial usada na validação.
- Perfis/permissões: usuário final autenticado via gateway (headers `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp`); cada usuário acessa exclusivamente os próprios apelidos.

## Dependências
- Fluxo de classificação e ponto de confirmação existentes no agente: `searchForPending` (`internal/agents/application/usecases/register_attempt.go:347-364`) e `handleConfirmationResume`/`ConfirmActionAccept` (`internal/agents/application/workflows/pending_entry_workflow.go:363-410`).
- Binding de leitura de categorias do agente para consultar dicionário e validar escrita (`internal/agents/infrastructure/binding/categories_reader_adapter.go:37-120`).
- Validação de versão editorial e deprecação já existente em `ResolveCategoryForWrite` (`internal/categories/application/usecases/resolve_category_for_write.go`).
- Normalização de termo reutilizando `immutable_unaccent` do schema atual (`migrations/000001_initial_schema.up.sql:490`).
- Decisão de arquitetura para a techspec: escolher o módulo dono da nova persistência de apelidos pessoais — extensão user-scoped do módulo `internal/categories` (que hoje é somente leitura, `internal/categories/module.go:36-42`) ou tabela própria do consumidor `internal/agents` consultada via binding, seguindo o padrão de o agente ter persistência própria e chamar módulos por binding.
- Governança obrigatória de plataforma para a implementação: adaptadores finos e zero comentários (R-ADAPTER-001), estados como tipos fechados / DMMF state-as-type, e primitivos de agent/memory em `internal/platform` (R-AGENT-WF-001).

## Fora de Escopo
- Criar categorias ou subcategorias personalizadas pelo usuário; a taxonomia global permanece fixa e editorial.
- Trilha editorial/administrativa de escrita do catálogo global (inserir, depreciar ou versionar categorias e termos).
- Comando conversacional explícito de gerenciamento de apelidos (listar, renomear ou apagar apelidos); pode ser história futura.
- Compartilhar aprendizado entre usuários ou promover apelidos pessoais para o dicionário global.
- Aprendizado adicional baseado em LLM fora do fluxo determinístico de confirmação já existente.

## Evidências
- Entrada: pedido de análise do módulo `internal/categories` em 2026-07-11 e escolha do usuário por dicionário pessoal com persona de usuário final via WhatsApp.
- Base de código: catálogo e dicionário globais sem `user_id` (`migrations/000001_initial_schema.up.sql:432`, `:485`, `:490`); módulo somente leitura sem caso de uso de escrita (`internal/categories/module.go:36-42`); `no_match` na busca (`internal/categories/application/usecases/search_dictionary.go:61-67`); reclassificação a cada mensagem sem persistir aprendizado (`internal/agents/application/usecases/register_attempt.go:347-364`); evidência da escolha do usuário carimbada apenas no lançamento (`internal/agents/application/workflows/pending_entry_workflow.go:663-671`); confirmação explícita como gatilho de escrita (`pending_entry_workflow.go:370`); validação de versão/deprecação disponível (`internal/categories/application/usecases/resolve_category_for_write.go:53`, `:120`); auth de gateway exigida (`internal/categories/openapi.yaml:6-7`, `internal/categories/module.go:29`).
- Inferências: a precedência do apelido pessoal como match exato sobre o global é a forma mais direta de eliminar o `no_match` recorrente; a decisão implícita de aprendizado (sem comando explícito) reduz fricção mas exige a guarda da RN-01 para não poluir com termos já globais.
- Não evidenciado: não existe hoje qualquer tabela, coluna, caso de uso, binding ou fluxo que persista mapeamento de termo por usuário; a busca varre apenas o dicionário global editorial.

## Notas de Validação
- Estrutura conforme `assets/modelo-historia-usuario.md`; validada com `python3 .agents/skills/user-stories/scripts/validar-historias-usuario.py docs/us/2026-07-11-us-dicionario-pessoal-apelidos-categoria.md`.
- Cenários cobrem fluxo feliz (aprender e reaplicar), alternativos (upsert e alvo deprecado) e de erro/bloqueio (não confirmar não aprende).
- Ambiguidades materiais de persona e escopo foram resolvidas por rodada de múltipla escolha antes da redação; a única decisão remanescente (módulo dono da persistência) está registrada como dependência de techspec, não como lacuna da história.
