# Prompt enriquecido — unificação de migrations PostgreSQL

## Contrato de carga base

- `AGENTS.md` lido como fonte canônica do repositório.
- Skill operacional usada: `prompt-enricher`.
- Este arquivo apenas materializa um prompt pronto para uso; não executa nenhuma implementação.

## Prompt original

> Eu quero unificar TODAS migrations utilizando as melhores e mais recomendas práticas de um DBA e documentação oficial do postgres: https://www.postgresql.org/docs/, quero que analise se tem alguma tabela que não é mais utilizada e que seja removida.
>
> O foco é robustez, eficiencia, production-ready/proof, 0 gaps, 0 lacunas, 0 falso positivo e realmente pronto para produção.
>
> NENHUM USUÁRIO IMPACTADO.

## Ambiguidades e riscos que o prompt enriquecido resolve

1. "Unificar TODAS migrations" pode significar squash total do histórico ou criação de novo baseline com estratégia de transição.
2. "Nenhum usuário impactado" precisa ser tratado como pré-condição verificável, não como suposição cega.
3. "Tabela não mais utilizada" precisa de critério objetivo e auditável para evitar falso positivo.
4. O repositório já usa `golang-migrate`, `cmd/migrate`, `taskfiles/migrate.yml` e migrations embedadas em `migrations/`; o prompt precisa apontar esses entrypoints.
5. Há registro histórico em `docs/runs/2026-06-29-unificar-migrations.md` de cenário com dados reais; o prompt precisa exigir bloqueio imediato se a premissa atual de zero impacto não puder ser comprovada.

## Prompt enriquecido — pronto para uso

```md
Quero que você execute uma análise e, se as pré-condições forem comprovadas, implemente a unificação das migrations PostgreSQL deste repositório com padrão DBA-grade, aderência à documentação oficial do PostgreSQL (https://www.postgresql.org/docs/) e nível production-ready real.

## Objetivo

Unificar todas as migrations do projeto em uma estratégia final robusta, segura, auditável e pronta para produção, com foco em:

- robustez
- eficiência
- zero gaps
- zero lacunas
- zero falso positivo
- zero impacto em usuários

Além disso, você deve identificar tabelas potencialmente obsoletas e remover apenas aquelas cuja obsolescência seja comprovada por evidência técnica suficiente.

## Regras inegociáveis

1. Não assuma nada sem verificar no working tree atual.
2. Use `AGENTS.md` como fonte canônica do repositório.
3. Siga a arquitetura e os limites do projeto antes de propor qualquer mudança.
4. Use como pontos obrigatórios de partida:
   - `cmd/server/server.go`
   - `cmd/worker/worker.go`
   - `cmd/migrate/migrate.go`
   - diretório `migrations/`
   - `migrations/migrations_integration_test.go`
   - `taskfiles/migrate.yml`
5. Considere que o projeto usa Go 1.26.4 e `github.com/golang-migrate/migrate/v4`.
6. Consulte explicitamente a documentação oficial do PostgreSQL relevante para as decisões tomadas e cite os tópicos/links usados.
7. Não invente tabelas obsoletas, fluxos, consumers, jobs, handlers, dependências ou cenários de produção.
8. Se houver qualquer evidência de que a premissa "nenhum usuário impactado" é falsa, incompleta ou não comprovável, pare imediatamente e retorne **BLOCKED**, sem implementar a unificação destrutiva.
9. Se houver dúvida razoável sobre remover uma tabela, não remova; classifique como candidata e documente a evidência faltante.
10. Não faça downgrade da segurança operacional só para viabilizar o squash.

## Contexto técnico mínimo a inspecionar

Faça inventário completo e correlacione:

- todas as migrations `up/down` em `migrations/`
- ordem, dependências e efeitos de cada migration
- schema final efetivo produzido pela cadeia atual
- tabelas, índices, constraints, extensões e seeds
- uso real das tabelas no código Go, incluindo:
  - repositories
  - queries SQL
  - handlers HTTP
  - jobs
  - consumers
  - producers
  - testes
- fluxo de bootstrap e execução do sistema a partir de `cmd/server/server.go`, `cmd/worker/worker.go` e `cmd/migrate/migrate.go`
- task de migrations existente em `taskfiles/migrate.yml`
- documentação histórica relevante, incluindo `docs/runs/2026-06-29-unificar-migrations.md`, apenas como insumo de risco; se divergir do working tree atual, prevalece o estado atual do repositório

## O que você precisa decidir

Antes de alterar qualquer arquivo, determine e justifique qual estratégia é a mais segura:

1. squash total para novo baseline único; ou
2. baseline novo + estratégia de compatibilidade/ponte; ou
3. bloqueio da unificação caso a premissa operacional não seja segura

A decisão deve ser baseada em evidência, não em preferência.

## Critério obrigatório para considerar uma tabela "não utilizada"

Uma tabela só pode ser considerada removível se você comprovar cumulativamente:

1. ausência de uso pelos entrypoints ativos;
2. ausência de uso em repositories, queries SQL, jobs, consumers, producers e handlers;
3. ausência de dependência funcional em testes relevantes;
4. ausência de necessidade para bootstrap, migrations futuras, outbox, deduplicação, auditoria ou integridade referencial;
5. ausência de impacto operacional e de dados no cenário atual;
6. possibilidade de reversão segura ou justificativa formal para não manter reversão.

Se qualquer item acima não puder ser comprovado, trate a tabela como **não elegível para remoção**.

## Saída esperada

Entregue exatamente nesta estrutura:

### 1. Executive summary
- decisão final
- motivo
- status: `APPROVED`, `BLOCKED` ou `APPROVED WITH RESTRICTIONS`

### 2. Inventário das migrations atuais
Tabela com:
- migration
- finalidade
- objetos criados/alterados/removidos
- dependências
- risco

### 3. Mapa do schema final atual
Tabela com:
- tabela
- origem da criação
- propósito
- principais constraints/índices
- evidência de uso
- status (`ativa`, `candidata à remoção`, `bloqueada para remoção`)

### 4. Análise formal de tabelas obsoletas
Para cada tabela candidata:
- evidências a favor da remoção
- evidências contra
- conclusão
- decisão final

### 5. Estratégia recomendada de unificação
Detalhe:
- abordagem escolhida
- por que ela é a mais segura
- como evita impacto em usuários
- como preserva integridade, rastreabilidade e rollback

### 6. Plano exato de implementação
Liste:
- arquivos a criar/alterar/remover
- sequência de execução
- cuidados com `up/down`
- tratamento de extensões, seeds, índices, constraints e schema `mecontrola`

### 7. Validação obrigatória
Liste os comandos exatos e o objetivo de cada um, incluindo no mínimo:
- `gofmt -w <arquivos alterados, se houver Go>`
- `go test -race -count=1 ./migrations/...`
- `go test -race -count=1 ./cmd/migrate/...`
- `go test ./...` se o risco justificar
- `go build ./cmd/...`
- `go vet ./cmd/... ./migrations/...`

Se algum comando não existir ou não fizer sentido no escopo final, explique com precisão.

### 8. Referências oficiais do PostgreSQL utilizadas
Liste os links e diga em qual decisão cada referência influenciou.

### 9. Riscos residuais
- riscos aceitos
- riscos bloqueantes
- premissas que ainda precisariam de confirmação externa

## Critérios de aceitação

Seu trabalho só será considerado aceito se:

1. a estratégia escolhida estiver justificada por evidência do repositório atual;
2. a premissa de zero impacto estiver comprovada ou o trabalho for bloqueado explicitamente;
3. não houver remoção de tabela sem prova suficiente de obsolescência;
4. a solução respeitar `golang-migrate`, o schema `mecontrola` e os entrypoints reais do projeto;
5. a proposta estiver alinhada a boas práticas oficiais do PostgreSQL e a práticas conservadoras de DBA;
6. houver plano claro de rollback ou justificativa técnica formal quando rollback não for seguro;
7. o resultado final estiver pronto para execução real em produção, sem lacunas narrativas ou técnicas.

## Modo de execução

Trabalhe nesta ordem:

1. inventário
2. prova da premissa de zero impacto
3. análise de obsolescência das tabelas
4. decisão da estratégia
5. implementação somente se aprovada
6. validação
7. relatório final

Se encontrar contradição entre histórico/documentação e o estado atual, registre a divergência e siga a opção mais segura.
```

## Justificativa curta das adições

| Adição | Motivo |
| --- | --- |
| Pré-condição verificável de zero impacto | Evita assumir um cenário operacional que pode não ser verdadeiro. |
| Entry points concretos do repositório | Reduz alucinação e força análise a partir da estrutura real. |
| Critério cumulativo para remoção de tabelas | Elimina falso positivo na identificação de obsolescência. |
| Estratégias possíveis de unificação | Resolve a ambiguidade entre squash total, baseline com ponte ou bloqueio seguro. |
| Formato de saída fechado | Garante resposta auditável, comparável e pronta para execução. |
| Referências oficiais do PostgreSQL | Obriga decisões sustentadas por documentação primária. |
| Critérios de aceitação mensuráveis | Torna o resultado verificável e menos subjetivo. |

## Variantes válidas

### Variante recomendada

Usar exatamente o prompt enriquecido acima para análise + implementação condicionada às pré-condições.

### Variante conservadora

Usar o mesmo prompt, mas trocar a seção `## Modo de execução` para exigir que a implementação só ocorra após uma primeira entrega exclusivamente analítica aprovada manualmente.
