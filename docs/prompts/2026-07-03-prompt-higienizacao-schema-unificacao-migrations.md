# 2026-07-03 — Prompt enriquecido para higienização do schema e unificação de migrations

> Nota histórica: este documento registra o contexto pré-squash usado no pedido original de 2026-07-03. O working tree atual já foi consolidado para baseline único `000001_initial_schema`, então referências abaixo a `000002_card_simplification` e `000003_claim_particionado_indices` descrevem o estado anterior do repositório, não a trilha vigente.

## Prompt original

> Eu quero higienizar todas as tabelas de banco de dados unificar migrations, seguir nomenclaruturas corretas de coluna que realmente represente o dado com base em: https://www.postgresql.org/docs/16/index.html e https://www.postgresql.org/docs/current/ddl.html , analisar no codebase o que realmente são utilizadas e remover as legadas de forma efetiva, robusto, economico, eficiente, 0 gaps, 0 falso positivo, 0 ressalvas, 0 lacunas.

## Ambiguidades eliminadas

| Ponto | Decisão aplicada no prompt |
|---|---|
| O trabalho é só diagnóstico ou também execução? | O prompt enriquecido manda implementar a higienização completa do schema e da trilha de migrations, não parar em análise superficial. |
| O que significa “unificar migrations”? | O prompt fecha como alvo preferencial um baseline canônico mínimo e coerente com o schema final, preservando `up/down`, `migrate` e `migrate-down`. |
| O que conta como “legado”? | O prompt exige evidência objetiva de não uso no código, testes, deploy e operação antes de remover tabela, coluna, índice, constraint, seed ou migration. |
| Como evitar renomear coluna por gosto pessoal? | O prompt obriga justificar cada rename por semântica real do dado, uso efetivo no codebase e aderência às práticas do PostgreSQL, não por cosmética. |
| Quais pontos do repositório são fonte da verdade? | O prompt obriga partir de `cmd/server/server.go`, `cmd/worker/worker.go`, `cmd/migrate/migrate.go`, `migrations/*.sql` e do working tree atual. |
| Como tratar drift já existente entre docs e migrations? | O prompt obriga reconciliar código, migrations, testes, runbooks e documentação para eliminar versão, tabela ou coluna descrita de forma incorreta. |

## Prompt enriquecido — versão pronta para uso

```text
Atue como um engenheiro sênior de PostgreSQL, modelagem relacional, migrations, Go e higiene de codebase. Execute a higienização completa do schema de banco de dados deste repositório, eliminando tabelas/colunas/objetos legados, corrigindo nomes de colunas que não representam corretamente o dado e unificando a trilha de migrations para deixá-la canônica, coerente e economicamente sustentável.

Você deve IMPLEMENTAR a higienização. Não pare em diagnóstico, plano ou sugestão.

Base obrigatória de decisão:
- Documentação oficial PostgreSQL 16: https://www.postgresql.org/docs/16/index.html
- Documentação oficial PostgreSQL DDL: https://www.postgresql.org/docs/current/ddl.html
- Estado atual do working tree deste repositório

Contexto mandatário do repositório:
- Repositório: `mecontrola`
- Stack principal: Go `1.26.4`
- Arquitetura: monólito modular em Go
- Fonte canônica de regras: `AGENTS.md`
- Ponto de partida obrigatório da investigação: `cmd/server/server.go` e `cmd/worker/worker.go`
- Entrypoint de migrations: `cmd/migrate/migrate.go`
- As migrations SQL ficam em `migrations/`, são embutidas por `migrations/embed.go` e aplicadas com `golang-migrate`
- O schema alvo usado pelas migrations é `mecontrola`
- Existe suporte operacional e de desenvolvimento para migrations em `taskfiles/migrate.yml`
- Existem testes/referências de consistência de migrations em:
  - `migrations/migrations_integration_test.go`
  - `internal/platform/database/postgres/test_helper.go`
- O working tree atual contém pelo menos estas migrations SQL:
  - `000001_initial_schema`
  - `000002_card_simplification`
  - `000003_claim_particionado_indices`
- Há drift real já detectável no repositório: `README.md` e `docs/diagrams/architecture.md` ainda descrevem apenas `000001` como baseline único e ainda falam em expectativa de versão `1`, embora o diretório `migrations/` já contenha `000002` e `000003`

Restrições mandatórias:
1. Leia e siga `AGENTS.md` antes de qualquer alteração.
2. Carregue obrigatoriamente `.agents/skills/agent-governance/SKILL.md`.
3. Como haverá alteração em projeto Go com impacto em SQL, migrations, testes e documentação, carregue também `.agents/skills/go-implementation/SKILL.md`.
4. Trabalhe em cima do working tree atual. Se código, docs antigas ou prompts históricos divergirem, o working tree atual prevalece.
5. Não use `internal/platform/runtime` como ponto de partida.
6. Não invente tabelas, colunas, constraints, índices, seeds, views, triggers, extensões, handlers, repositórios ou fluxos ausentes.
7. Não trate como “usado” um objeto de banco que só aparece em documentação antiga, comentário, prompt velho, migration histórica já superada ou teste obsoleto sem consumidor real.
8. Não trate como “legado” um objeto que tenha consumidor real em runtime, teste válido, deploy, operação, seed necessária, lock/idempotência, outbox ou fluxo de observabilidade ainda suportado.
9. Se um objeto não fizer mais sentido no schema final, remova por completo; não deixe compatibilidade morta, coluna ociosa, tabela fantasma, índice sem consumidor, rename parcial ou ressalva vaga.
10. Não renomeie coluna por preferência estética. Só renomeie quando o nome atual não representar corretamente o dado, a unidade, o papel relacional ou a semântica de domínio.
11. Toda renomeação de tabela/coluna/constraint/index deve ser acompanhada da atualização completa do código consumidor, testes, docs, exemplos operacionais e migrations.
12. Toda remoção deve ser sustentada por evidência objetiva de não uso.
13. Toda migration resultante deve manter `up` e `down` coerentes, executáveis e seguras.
14. A unificação de migrations deve resultar em uma trilha mínima, canônica e livre de drift. Alvo preferencial: um baseline único do schema final. Só mantenha migrations adicionais se houver necessidade técnica comprovada e explicitamente justificada.
15. Não preserve histórico redundante “por via das dúvidas”.
16. Não deixe docs operacionais, runbooks, compose files, testes ou helpers apontando para versão/tabela/coluna que já não existe no estado final.
17. Zero falso positivo: nenhuma remoção ou rename sem prova.
18. Zero lacunas: ao final, não pode sobrar objeto legado no schema, SQL, Go, testes, deploy ou documentação.

Escopo exato da investigação:
1. Inventariar todas as migrations e todos os objetos de schema definidos em `migrations/*.sql`, incluindo:
   - schemas
   - extensões
   - tabelas
   - colunas
   - constraints
   - índices
   - seeds de referência
   - FKs
   - checks
   - objetos auxiliares relevantes
2. Mapear o consumo real desses objetos começando obrigatoriamente por:
   - `cmd/server/server.go`
   - `cmd/worker/worker.go`
   - `cmd/migrate/migrate.go`
   - `migrations/*.sql`
   - `migrations/migrations_integration_test.go`
   - `internal/platform/database/postgres/test_helper.go`
3. Revisar repositórios, queries inline, scans, DTOs, fixtures, testes e quaisquer pontos que toquem o banco, para descobrir:
   - quais tabelas realmente são lidas/escritas
   - quais colunas realmente são consumidas
   - quais índices/constraints sustentam comportamento real
   - quais objetos existem apenas por legado histórico
4. Cruzar obrigatoriamente a análise de schema com:
   - código Go em `internal/`
   - entrypoints em `cmd/`
   - `README.md`
   - `docs/diagrams/architecture.md`
   - `deployment/compose/*`
   - `deployment/runbooks/*`
   - `taskfiles/migrate.yml`
5. Identificar drift entre:
   - migrations atuais
   - schema final efetivo esperado pelos testes
   - documentação de operação
   - helpers de banco para testes
   - código consumidor
6. Avaliar nomenclatura de colunas com foco em representação correta do dado, por exemplo:
   - nomes que não deixam claro se o valor é ID externo, hash, status, origem, total, limite, versão ou timestamp
   - colunas quantitativas sem unidade explícita quando a unidade é relevante
   - nomes ambíguos que conflitam com o uso real no código
   - colunas herdadas de escopo antigo que já não representam o domínio atual
7. Definir o estado final canônico do schema e da trilha de migrations.

Objetivo operacional:
Deixar o repositório com uma camada de persistência mínima, correta e comprovada, onde:
- toda tabela existente tenha função real;
- toda coluna existente represente corretamente o dado que armazena;
- todo índice/constraint existente tenha papel técnico ou de integridade comprovado;
- todo objeto legado seja removido do SQL, do código, dos testes e da documentação;
- a trilha de migrations fique unificada, coerente e pronta para bootstrap limpo do zero.

Como executar:
1. Monte um inventário autoritativo de todos os objetos de banco definidos nas migrations atuais.
2. Classifique cada objeto em uma e apenas uma categoria:
   - runtime real
   - integridade/infra obrigatória
   - seed de referência necessária
   - usado apenas em teste válido
   - usado apenas em operação/deploy
   - legado/sem uso real
3. Para cada tabela e cada coluna mantida, anote evidência exata de consumo com arquivo e linha.
4. Para cada objeto removido, anote evidência exata de ausência de uso real ou de substituição consolidada.
5. Corrija nomes de colunas apenas onde houver desvio semântico comprovado e atualize integralmente:
   - migrations
   - queries
   - repositórios
   - DTOs/serialização
   - testes
   - docs/runbooks
6. Remova de forma efetiva:
   - tabelas legadas
   - colunas legadas
   - índices/constraints órfãos
   - seeds obsoletas
   - referências antigas em código ou documentação
7. Unifique a trilha de migrations para refletir somente o estado final correto:
   - prefira colapsar em um baseline canônico único se isso puder ser feito com segurança
   - se precisar manter mais de um arquivo, mantenha apenas o conjunto mínimo necessário e documente a razão técnica
   - elimine drift entre nomes de arquivos, conteúdo real, docs e expectativa de `schema_migrations`
8. Garanta que `migrate`, `migrate-down`, setup de testes e bootstrap de banco do zero continuem coerentes com o estado final.
9. Atualize documentação e runbooks para refletirem exatamente:
   - quais migrations existem
   - qual versão final passa a ser a referência
   - como resetar/reaplicar o banco
10. Ao terminar, garanta que nenhuma referência residual a tabela/coluna/migration legada permaneceu no repositório.

Critérios de aceitação obrigatórios:
1. Toda migration SQL vigente do repositório foi analisada.
2. Toda tabela do schema final foi mantida por evidência real ou removida por evidência real.
3. Toda coluna do schema final foi mantida por evidência real ou removida/renomeada por evidência real.
4. Nenhum objeto legado permaneceu em:
   - migrations
   - código Go
   - testes
   - docs
   - runbooks
   - compose/deploy
5. A trilha de migrations final ficou unificada e coerente com o schema final pretendido.
6. O bootstrap de banco vazio a partir das migrations finais cria apenas o schema correto e ativo.
7. O fluxo down→up continua funcional e coerente com a nova trilha.
8. `README.md`, diagramas/arquitetura e runbooks não continuam afirmando versão, baseline ou objeto de schema incorreto.
9. Toda coluna renomeada passou a representar melhor o dado real, sem deixar mismatch entre SQL e código.
10. Não sobrou query, scan, fixture, seed ou teste apontando para tabela/coluna removida.
11. O resultado final não contém TODO, TBD, “avaliar depois”, compatibilidade morta, ressalva vaga ou meia-consolidação.

Formato obrigatório da sua resposta final:
1. **Resumo do que foi higienizado**
2. **Tabela de inventário final** com colunas:
   - objeto
   - tipo
   - categoria
   - evidência de uso real
   - ação tomada
3. **Trilha final de migrations**
4. **Arquivos alterados/removidos**
5. **Drifts eliminados**
6. **Comandos de validação executados**

Validação mínima obrigatória:
1. `gofmt -w` nos arquivos Go alterados
2. `go test -race -count=1 ./cmd/migrate/... ./internal/platform/database/postgres/...`
3. `go test -tags=integration ./migrations/...`
4. `go test -race -count=1` nos pacotes de domínio/aplicação/infra afetados pela mudança
5. `go build ./cmd/...`
6. `go vet ./cmd/... ./internal/...` no escopo proporcional da alteração
7. `golangci-lint run` no escopo alterado, se disponível no repositório

Regras de qualidade da execução:
1. Faça a menor mudança segura que resolva por completo a causa raiz.
2. Não preserve tabela, coluna ou migration “por precaução” se não houver consumidor real.
3. Não remova tabela, coluna ou índice “por aparência” se ainda houver função comprovada.
4. Toda decisão deve estar ancorada no codebase atual e nas referências oficiais do PostgreSQL.
5. Se docs antigas divergirem do working tree, o working tree prevalece e o drift deve ser corrigido.
6. Se houver trade-off real entre colapsar histórico e preservar segurança operacional, escolha a opção mais segura e mais simples de manter e explique objetivamente a razão.
7. Seja econômico: reduza redundância real, não apenas ruído visual.
8. Entregue o trabalho pronto, sem gaps, sem falso positivo e sem desvios do objetivo.
```

## O que foi adicionado e por quê

| Adição | Justificativa |
|---|---|
| Contexto explícito de `cmd/server`, `cmd/worker`, `cmd/migrate` e `migrations/embed.go` | Força o prompt a partir do bootstrap e da trilha real de banco do repositório. |
| Drift já identificado entre `README`/diagramas e `migrations/` | Obriga o executor a reconciliar documentação e versão real, em vez de só mexer no SQL. |
| Definição operacional de “unificar migrations” | Remove ambiguidade e reduz o risco de uma resposta parar em recomendações vagas. |
| Exigência de inventário por objeto com evidência | Minimiza falso positivo na remoção de tabelas, colunas e índices. |
| Regra de rename semântico, não cosmético | Evita churn gratuito e foca só em nomes que realmente distorcem o dado. |
| Cobertura obrigatória de código, testes, deploy e runbooks | Fecha gaps comuns em mudanças de schema que deixam resíduos fora das migrations. |
| Critérios de aceitação e validação fechados | Torna o prompt reutilizável como instrução de execução real, não só brainstorming. |

## Variante compacta

Use apenas se quiser uma versão menor e aceitar menos contexto inline:

```text
Leia `AGENTS.md`, carregue `.agents/skills/agent-governance/SKILL.md` e `.agents/skills/go-implementation/SKILL.md`, e execute a higienização completa do schema e a unificação das migrations deste repositório.

Parta obrigatoriamente de `cmd/server/server.go`, `cmd/worker/worker.go`, `cmd/migrate/migrate.go`, `migrations/*.sql`, `migrations/migrations_integration_test.go` e `internal/platform/database/postgres/test_helper.go`. Trabalhe sobre o working tree atual, usando como base obrigatória a documentação oficial do PostgreSQL (`https://www.postgresql.org/docs/16/index.html` e `https://www.postgresql.org/docs/current/ddl.html`).

Inventarie todas as tabelas, colunas, índices, constraints e seeds do schema `mecontrola`; prove com evidência o que realmente é usado; remova de forma efetiva tudo que for legado; renomeie apenas colunas cujo nome não represente corretamente o dado; e unifique a trilha de migrations para um baseline canônico mínimo, mantendo `up/down`, `migrate`, `migrate-down`, testes e documentação coerentes.

Considere explicitamente o drift atual: o repositório já possui `000001`, `000002` e `000003`, mas `README.md` e `docs/diagrams/architecture.md` ainda descrevem apenas `000001` como baseline único.

Não invente contexto. Não preserve compatibilidade morta. Não remova por aparência. Não deixe sobrar referências residuais no Go, SQL, testes, compose, runbooks ou docs.

Entregue:
1. resumo do que foi higienizado;
2. tabela por objeto com categoria, evidência e ação tomada;
3. trilha final de migrations;
4. arquivos alterados/removidos;
5. drifts eliminados;
6. comandos de validação executados.
```
