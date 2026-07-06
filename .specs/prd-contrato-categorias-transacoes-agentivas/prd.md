<!-- spec-version: 1 -->

# PRD — Contrato Deterministico de Categorias para Transacoes Agentivas

## 1. Problema e Objetivo

O produto precisa garantir assertividade maxima na categorizacao de transacoes criadas, alteradas ou confirmadas por fluxos agentivos. O comportamento atual ja consulta `internal/categories` e valida parte da relacao raiz/subcategoria em `internal/transactions`, mas ainda perde evidencias decisivas entre classificacao, decisao, validacao e persistencia.

O problema principal e que `internal/categories` calcula evidencias ricas de classificacao, incluindo candidatos, score, ambiguidade, qualidade, motivo, outcome e versao editorial, mas `internal/agents` e `internal/transactions` consomem subconjuntos diferentes desse contrato. Isso cria risco de falso positivo quando existe apenas um candidato, mas a classificacao nao e suficientemente inequivoca para escrita automatica.

Objetivo: definir uma feature de produto para estabelecer um contrato funcional unico, deterministico, auditavel e bloqueante para categorias em transacoes, usando `internal/categories` como fonte canonica, `internal/agents` como consumidor agentivo/classificador e `internal/transactions` como consumidor validador/persistente.

A prioridade absoluta e 0 falso positivo conhecido na escrita. Quando houver qualquer duvida material, a escrita deve ser bloqueada e o usuario deve clarificar explicitamente.

## 2. Usuario e Atores

Ator principal: pessoa usuaria que registra, edita ou confirma despesas e receitas por interface conversacional.

Ator secundario: agente financeiro em `internal/agents`, que sugere, consulta, classifica e pede clarificacao, mas nao e autoridade final sobre categorias.

Autoridade canonica: `internal/categories`, responsavel pela semantica de catalogo, dicionario, candidatos, ambiguidade, outcome e versao editorial.

Validador de escrita: `internal/transactions`, responsavel por impedir persistencia quando categoria, subcategoria, tipo da transacao ou evidencia de classificacao nao forem compativeis com o contrato canonico.

## 3. Diagnostico do Codebase Atual

Fatos provados:

- `internal/categories` ja expoe use cases reais para `SearchDictionary`, `ResolveBySlug`, `ValidateSubcategory`, `ListCategories` e leitura de versao editorial.
- `internal/categories/domain/services/CandidateResolver` calcula `RootCategoryID`, `CategoryID`, `Path`, `MatchedTerm`, `SignalType`, `Confidence`, `Quality`, `Score`, `IsAmbiguous` e `MatchReason`.
- `internal/categories/application/usecases/SearchDictionary` calcula `Outcome` por score e retorna `Version`.
- `internal/agents/application/tools/classify_category.go` chama `SearchDictionary`, mas sua saida publica nao preserva `Outcome`, `Version`, `SignalType`, `Confidence`, `MatchQuality`, `MatchedTerm` nem `MatchReason`.
- `internal/agents/application/usecases/register_entry.go` nao depende da tool `classify_category`; ele classifica despesas e receitas internamente com `RegisterEntry.classify`.
- `RegisterEntry.classify` aceita o primeiro candidato quando existe exatamente um e `IsAmbiguous=false`, sem avaliar explicitamente `Outcome`, `Score`, `MatchQuality` ou `Version`.
- `internal/agents/application/workflows/destructive_confirm_workflow.go` possui fluxo de registro que busca categoria com `kind="expense"` e usa o primeiro candidato sem a mesma protecao de ambiguidade/root/subcategoria/outcome.
- `internal/transactions/application/interfaces/CategoryValidator` recebe apenas `categoryID` e `subcategoryID` e retorna snapshot limitado.
- `internal/transactions/infrastructure/config/CategoriesCache` valida se `categoryID` e raiz oficial e se `subcategoryID` pertence a raiz esperada, mas nao preserva evidencia editorial/classificatoria no contrato de produto.
- `CreateTransaction` e `UpdateTransaction` validam categoria e direcao, mas templates recorrentes nao demonstram a mesma paridade de guard funcional antes de persistir o template.
- A persistencia atual guarda IDs e snapshots de nomes, mas nao guarda evidencia funcional suficiente para auditoria robusta da decisao de categoria.

Conclusao: melhorar apenas `classify_category` nao resolve assertividade em transacoes. A feature deve consolidar o contrato de categorizacao no fluxo completo de escrita e confirmacao.

## 4. Escopo Incluido

- Definir o contrato funcional de resolucao de categoria para escrita de despesa e receita.
- Exigir uso de `internal/categories` como fonte canonica unica para catalogo, dicionario, candidatos, outcome e versao editorial.
- Exigir que qualquer escrita ou alteracao de transacao use apenas categoria canonicamente resolvida, inequivoca, ativa e compativel com o tipo da transacao.
- Exigir que fluxos agentivos tratem `Outcome` como decisao funcional obrigatoria, nao apenas `IsAmbiguous`.
- Exigir que match unico com score/qualidade insuficiente seja tratado como ambiguidade ou necessidade de clarificacao.
- Exigir bloqueio para `no_match`, `ambiguous`, categoria inexistente, categoria deprecated, categoria raiz invalida, subcategoria que nao pertence a raiz, incompatibilidade entre `kind` e direcao, ou ausencia de evidencia minima.
- Exigir paridade entre criacao, edicao, confirmacao destrutiva/sensivel e templates recorrentes quando houver categoria.
- Exigir que `classify_category` seja coerente com o mesmo contrato usado pela escrita, mesmo sendo ferramenta de apoio.
- Exigir rastreabilidade funcional suficiente para explicar por que uma categoria foi aceita, bloqueada ou enviada para clarificacao.
- Exigir cenarios de aceite que cubram despesas e receitas.

## 5. Escopo Excluido

- Implementacao de codigo, migrations, handlers, adapters, jobs, rotas, schema de banco ou wiring.
- Redesenho completo da taxonomia de categorias.
- Criacao automatica de novas categorias durante registro de transacao.
- Curadoria de novos termos de dicionario ou mudanca editorial do catalogo.
- Uso de LLM, embeddings, scorer ou avaliacao pos-fato como autoridade de escrita.
- Fallback silencioso para categoria generica, primeira categoria, maior similaridade textual ou valor livre.
- Mudanca de UI final, copy conversacional final ou desenho visual.

## 6. Restricoes e Conformidade

- `internal/categories` e a fonte canonica obrigatoria.
- `internal/agents` pode sugerir, listar, classificar e pedir clarificacao, mas nao pode inventar categoria, subcategoria, kind ou IDs.
- `internal/transactions` deve bloquear persistencia quando o contrato canonico nao provar uma unica categoria valida.
- A feature deve cobrir `expense` e `income`; qualquer outro `kind` deve ser invalido para escrita financeira.
- A solucao deve priorizar seguranca e exatidao sobre conveniencia.
- Scorers e avaliacoes LLM-judged podem medir qualidade, mas nao podem desbloquear escrita.
- DMMF deve orientar o contrato: estados fechados para outcomes, smart constructors para objetos com invariantes, decisao pura para aceite/bloqueio e pipeline explicito de classificar, validar, decidir, persistir.
- Mastra deve orientar a fronteira agentiva: tool como adapter fino, use case para orquestracao e workflow apenas quando houver fluxo retomavel/confirmavel.
- Go implementation deve orientar a arquitetura: interface no consumidor, dependencia para dentro, adapter fino, sem interfaces ficticias e sem reimplementar primitivos de plataforma.

## 7. Decisoes Funcionais Fixadas

- A feature cobre despesas e receitas desde o inicio.
- A escrita so pode ocorrer quando existir exatamente uma categoria canonica aceita pelo contrato.
- A evidencia minima obrigatoria para escrita automatica e: `Outcome` canonico de aceite, categoria ativa, `kind` compativel, raiz valida, subcategoria folha valida e versao editorial registrada.
- Ambiguidade, baixa confianca, match por token/fuzzy insuficiente, candidato unico sem outcome aceito, no match e incompatibilidade bloqueiam escrita.
- O usuario deve clarificar explicitamente para destravar qualquer bloqueio escolhendo um candidato canonico apresentado; texto livre nunca destrava escrita sem nova resolucao canonica.
- `Outcome` do modulo de categorias deve ser tratado como insumo obrigatorio para decisao de escrita.
- A evidencia funcional da decisao de categoria deve ser persistida junto da transacao ou template recorrente, nao apenas registrada em logs ou traces.
- A versao editorial usada na decisao deve fazer parte da evidencia funcional persistida.
- Categoria raiz sem subcategoria nao deve ser permitida em transacoes nesta feature.
- Clarificacao do usuario deve passar por novo gate completo antes da persistencia, incluindo `Outcome`, raiz, folha, `kind`, status ativo e versao editorial.
- Mudanca de versao editorial entre classificacao e persistencia deve bloquear a escrita, recarregar candidatos e exigir nova confirmacao ou clarificacao antes de qualquer persistencia.
- Evidencia persistida e obrigatoria para qualquer escrita categorizada de transacao ou template recorrente, incluindo fluxos agentivos e nao agentivos.
- Banco novo: a feature nao precisa remediar legado historico; o contrato bloqueante deve valer para todos os novos writes desde a primeira entrega.
- Toda transacao deve usar subcategoria folha.
- A fonte da decisao persistida deve ser enum fechado com valores funcionais: `auto_matched`, `user_selected_candidate`, `manual_canonical_id` e `system_migration`.
- Escrita manual nao agentiva com IDs fornecidos deve passar pelo mesmo gate completo e persistir evidencia com fonte `manual_canonical_id`.
- Escrita manual nao agentiva aprovada deve persistir evidencia deterministica com `score=1.0`, `confidence=manual_confirmed`, `quality=manual_canonical` e `source=manual_canonical_id`.
- Escrita manual nao agentiva aprovada deve persistir `signal_type=manual_canonical`, `matched_term=<subcategory_slug>` e `match_reason=manual canonical id validated`.
- Todo update de transacao ou template recorrente deve revalidar e atualizar evidencia da categoria atual antes de persistir, mesmo quando categoria e subcategoria nao mudarem.
- Fluxos de transacao direta, edicao, recorrencia e confirmacao devem obedecer ao mesmo criterio de aceite.
- `classify_category` deve refletir o contrato canonico, mas nao e a unica superficie da feature.

## 8. Criterios de Sucesso Mensuraveis

- 100% das escritas de despesa e receita com categoria inexistente sao bloqueadas antes da persistencia.
- 100% das escritas com mais de um candidato plausivel sao bloqueadas e pedem clarificacao.
- 100% das escritas com candidato unico mas outcome nao aceito sao bloqueadas e pedem clarificacao.
- 100% das escritas com `kind` incompativel com a direcao da transacao sao bloqueadas.
- 100% das escritas com subcategoria que nao pertence a raiz selecionada sao bloqueadas.
- 100% das escritas com raiz sem folha sao bloqueadas.
- 100% das escritas com mudanca de versao editorial entre classificacao e persistencia sao bloqueadas e reavaliadas.
- 100% das escritas manuais com IDs fornecidos passam pelo mesmo gate completo antes da persistencia.
- 100% dos fluxos de criacao, edicao, confirmacao e recorrencia seguem o mesmo criterio funcional de aceite.
- 100% dos aceites de categoria possuem evidencia persistida minima: raiz, subcategoria folha, kind, path, outcome canonico de aceite, score, confidence, quality, fonte da decisao, motivo da decisao e versao editorial.
- 0 escrita e autorizada por fallback para primeira categoria, categoria generica, similaridade textual isolada ou sugestao LLM.
- Testes de aceite derivados deste PRD cobrem match exato aceito, no match, ambiguidade multi-candidato, token/fuzzy abaixo do limiar, despesa vs receita, subcategoria de outra raiz, categoria deprecated e clarificacao bem-sucedida.

## 9. Requisitos Funcionais Preliminares

- RF-01: O produto deve tratar `internal/categories` como unica autoridade canonica para categorias financeiras.
- RF-02: O produto deve suportar categorizacao robusta para despesas e receitas.
- RF-03: O produto deve aceitar somente `expense` e `income` como kinds validos para escrita financeira categorizada.
- RF-04: O produto deve exigir que toda escrita de transacao com categoria passe por resolucao canonica antes da persistencia.
- RF-05: O produto deve distinguir outcome aceito, ambiguidade, ausencia de match, incompatibilidade e erro operacional como estados funcionais diferentes.
- RF-06: O produto deve bloquear escrita quando `Outcome` indicar ambiguidade, ausencia de match ou qualquer estado diferente de aceite canonico.
- RF-07: O produto deve bloquear escrita quando houver candidato unico com score, qualidade ou confianca insuficiente para aceite automatico.
- RF-08: O produto deve bloquear escrita quando houver mais de um candidato plausivel, mesmo que exista candidato ordenado em primeiro lugar.
- RF-09: O produto deve bloquear escrita quando a categoria raiz nao for oficial, ativa e compativel com o kind da transacao.
- RF-10: O produto deve bloquear escrita quando a subcategoria nao for folha valida da raiz selecionada.
- RF-11: O produto deve bloquear escrita quando a categoria estiver deprecated ou semanticamente indisponivel para novas transacoes.
- RF-12: O produto deve pedir clarificacao explicita ao usuario sempre que a escrita for bloqueada por ambiguidade, baixa evidencia, no match ou incompatibilidade.
- RF-13: O produto deve permitir que a clarificacao destrave escrita apenas quando o usuario escolher uma opcao canonica entre candidatos apresentados; texto livre deve disparar nova resolucao canonica antes de qualquer persistencia.
- RF-14: O produto deve revalidar completamente uma categoria escolhida por clarificacao antes da persistencia, incluindo `Outcome`, raiz, folha, `kind`, status ativo e versao editorial.
- RF-15: O produto deve bloquear categoria raiz sem subcategoria em transacoes nesta feature.
- RF-16: O produto deve bloquear persistencia quando a versao editorial mudar entre classificacao e escrita, recarregando candidatos antes de nova tentativa.
- RF-17: O produto deve exigir evidencia persistida para qualquer escrita categorizada de transacao ou template recorrente, independentemente da superficie de entrada.
- RF-18: O produto deve bloquear raiz sem folha em todos os writes.
- RF-19: O produto deve persistir a fonte da decisao como enum fechado: `auto_matched`, `user_selected_candidate`, `manual_canonical_id` ou `system_migration`.
- RF-20: O produto deve aplicar o mesmo gate completo a escritas manuais nao agentivas com IDs fornecidos, persistindo evidencia com fonte `manual_canonical_id`.
- RF-21: O produto deve persistir evidencia deterministica para escrita manual aprovada com `score=1.0`, `confidence=manual_confirmed`, `quality=manual_canonical` e `source=manual_canonical_id`.
- RF-22: O produto deve persistir evidencia manual aprovada com `signal_type=manual_canonical`, `matched_term=<subcategory_slug>` e `match_reason=manual canonical id validated`.
- RF-23: O produto deve revalidar e atualizar evidencia da categoria atual em todo update de transacao ou template recorrente, mesmo quando categoria e subcategoria nao mudarem.
- RF-24: O produto deve declarar que nao ha escopo de remediacao de legado porque o banco e novo; todos os novos writes devem cumprir o contrato desde a entrega.
- RF-25: O produto deve garantir que `RegisterEntry.classify`, `destructive_confirm_workflow` e demais fluxos de escrita categorizada usem o mesmo criterio funcional de aceite.
- RF-26: O produto deve garantir que `classify_category` exponha ao agente informacao suficiente para explicar candidatos, ambiguidade e necessidade de clarificacao.
- RF-27: O produto deve garantir que fluxos de confirmacao retomavel nao gravem categoria usando apenas o primeiro candidato.
- RF-28: O produto deve garantir paridade funcional entre criacao e edicao de transacoes.
- RF-29: O produto deve garantir paridade funcional entre transacoes diretas e templates recorrentes quando categoria for persistida.
- RF-30: O produto deve persistir evidencia funcional minima da decisao de categoria junto da transacao ou template recorrente para auditoria e debug.
- RF-31: O produto deve impedir que scorers, prompts ou respostas de LLM sejam usados como autoridade para desbloquear escrita.
- RF-32: O produto deve permitir diagnostico diferenciado para categoria inexistente, categoria raiz invalida, subcategoria fora da raiz, kind incompativel, deprecated e ambiguidade.
- RF-33: O produto deve rejeitar qualquer contrato que dependa de string livre para outcome critico de escrita.
- RF-34: O produto deve definir criterios de aceite reproduziveis para match exato, alias inequivoco, token match, fuzzy match e candidato manualmente clarificado.
- RF-35: O produto deve impedir regressao onde teste com mock aceite kind invalido que a integracao real rejeitaria.

## 10. Requisitos Nao Funcionais

- RNF-01: A decisao de aceite/bloqueio deve ser deterministica e reproduzivel.
- RNF-02: A latencia adicional da validacao de categoria nao deve degradar materialmente o fluxo de registro conversacional.
- RNF-03: O comportamento deve ser observavel por logs, traces ou eventos funcionais suficientes para investigar bloqueios.
- RNF-04: A feature deve ser segura contra catalogo editorial atualizado durante execucao, usando versao editorial como evidencia.
- RNF-05: A feature deve evitar duplicacao de regras de categoria em `internal/agents` e `internal/transactions`.

## 11. Cenarios de Aceite Obrigatorios

- CA-01: Dada uma despesa com termo que resolve para um match inequivoco de despesa, quando o usuario registra a transacao, entao a escrita e aceita com raiz, subcategoria, kind e evidencia canonica.
- CA-02: Dada uma receita com termo que resolve para um match inequivoco de receita, quando o usuario registra a transacao, entao a escrita e aceita com raiz, subcategoria, kind e evidencia canonica.
- CA-03: Dado um termo sem match, quando o usuario tenta registrar uma transacao, entao a escrita e bloqueada e o sistema pede clarificacao.
- CA-04: Dado um termo com multiplos candidatos, quando o usuario tenta registrar uma transacao, entao nenhum candidato e escolhido automaticamente.
- CA-05: Dado um candidato unico com outcome ambiguo por score, qualidade ou confianca, quando o usuario tenta registrar uma transacao, entao a escrita e bloqueada.
- CA-06: Dada uma subcategoria pertencente a outra raiz, quando a transacao e validada, entao a escrita e bloqueada.
- CA-07: Dada uma categoria de receita em uma despesa, ou categoria de despesa em uma receita, quando a escrita e solicitada, entao a escrita e bloqueada.
- CA-08: Dada uma categoria deprecated, quando a escrita e solicitada, entao a escrita e bloqueada para nova transacao.
- CA-09: Dado um fluxo de confirmacao retomado, quando a categoria ainda estiver ambigua, entao o workflow nao persiste usando o primeiro candidato.
- CA-10: Dada uma clarificacao explicita do usuario selecionando candidato canonico valido, quando a validacao confirmar compatibilidade, entao a escrita e permitida.
- CA-11: Dado um template recorrente com categoria invalida ou incompativel, quando o template for criado ou alterado, entao a persistencia do template e bloqueada.
- CA-12: Dada a tool `classify_category`, quando ela retornar candidatos, entao a resposta deve conter informacao suficiente para explicar por que a escrita pode ou nao prosseguir.
- CA-13: Dada uma categoria raiz sem subcategoria, quando a escrita for solicitada, entao a escrita e bloqueada.
- CA-14: Dada uma escolha por clarificacao, quando qualquer gate canonico falhar na revalidacao completa, entao a escrita e bloqueada.
- CA-15: Dada mudanca de versao editorial entre classificacao e persistencia, quando a escrita for solicitada, entao o sistema bloqueia, recarrega candidatos e exige nova confirmacao ou clarificacao.
- CA-16: Dado um fluxo nao agentivo de escrita categorizada, quando a transacao ou template for persistido, entao a evidencia minima da decisao de categoria tambem e persistida.
- CA-17: Dado um write manual nao agentivo com IDs de categoria fornecidos, quando qualquer gate canonico falhar, entao a persistencia e bloqueada.
- CA-18: Dado um write manual nao agentivo aprovado, quando a transacao ou template for persistido, entao a fonte da decisao persistida e `manual_canonical_id`.
- CA-19: Dado qualquer write categorizado aprovado, quando a evidencia for persistida, entao a fonte da decisao pertence ao enum fechado permitido.
- CA-20: Dado o banco novo, quando a feature entrar em producao, entao todos os novos writes categorizados devem cumprir o contrato sem etapa de remediacao de legado.
- CA-21: Dado um write manual nao agentivo aprovado, quando a evidencia for persistida, entao `score=1.0`, `confidence=manual_confirmed`, `quality=manual_canonical` e `source=manual_canonical_id`.
- CA-22: Dado um write manual nao agentivo aprovado, quando a evidencia for persistida, entao `signal_type=manual_canonical`, `matched_term=<subcategory_slug>` e `match_reason=manual canonical id validated`.
- CA-23: Dado um update de transacao ou template recorrente sem troca de categoria, quando a persistencia for solicitada, entao a categoria atual e revalidada e a evidencia persistida e atualizada antes do write.

## 12. Prompt Para Handoff ao `$create-prd`

Crie um PRD production-ready para a feature "Contrato Deterministico de Categorias para Transacoes Agentivas".

Use o seguinte escopo mandatorio: estabelecer um contrato funcional unico e auditavel para categorizacao de despesas e receitas entre `internal/categories`, `internal/agents` e `internal/transactions`. `internal/categories` e a fonte canonica; `internal/agents` classifica, sugere e pede clarificacao; `internal/transactions` valida e bloqueia persistencia quando o contrato nao provar uma unica categoria valida.

O PRD deve focar em 0 falso positivo conhecido na escrita. Ambiguidade, no match, score/qualidade/confianca insuficiente, kind invalido, incompatibilidade despesa/receita, categoria deprecated, raiz invalida, subcategoria fora da raiz ou ausencia de versao/evidencia devem bloquear escrita e exigir clarificacao explicita.

Inclua requisitos para `RegisterEntry.classify`, `classify_category`, fluxos de confirmacao retomavel, criacao/edicao de transacoes e templates recorrentes obedecerem ao mesmo criterio funcional de aceite. Nao implemente nada, nao defina migrations, nao invente rotas, nao desenhe codigo. O documento deve produzir requisitos funcionais, criterios de sucesso mensuraveis e cenarios de aceite prontos para especificacao tecnica posterior.
