# Prompt enriquecido para `internal/categories`

## Prompt enriquecido

```text
Objetivo
Refatorar o modulo `internal/categories` em modo `advisory` por padrao, preservando contratos de leitura, payloads e comportamento observavel. So execute mudancas se eu pedir explicitamente.
Mandatorio: nao mudar nenhum comportamento ja existente, nem publico, nem interno observavel, nem operacional. Toda sugestao deve preservar exatamente a semantica atual de leitura.

Skills obrigatorias
1. Carregue `.agents/skills/refactor/SKILL.md`.
2. Carregue `.agents/skills/go-implementation/SKILL.md`.
3. Carregue referencias sob demanda de acordo com a superficie real tocada e sem exceder a carga de contexto definida em `AGENTS.md`.

Contrato de exploracao
1. Valide `AGENTS.md`, `go.mod`, `internal/categories/infrastructure/http/server/router.go` e os arquivos realmente necessarios.
2. Nao invente fluxos de escrita, eventos ou integracoes que nao existam.
3. Considere explicitamente estas superficies reais: `application/usecases`, `application/interfaces`, `application/dtos`, `domain/entities`, `domain/services`, `domain/factories`, `domain/valueobjects`, `infrastructure/http/server`, `infrastructure/repositories/postgres` e `openapi.yaml`.

Objetivo tecnico da refatoracao
1. Melhorar legibilidade, separacao de responsabilidades e robustez do modulo de leitura de categorias e dicionario.
2. Siga fielmente SOMENTE os principios de `Domain Modeling Made Functional: Tackle Software Complexity with Domain-Driven Design and F#` que tragam ganho real para um modulo majoritariamente read-oriented.
3. Priorize quando fizer sentido:
   - smart constructors ou factories mais explicitas para IDs/slugs e entidades com invariante;
   - services puros para resolucao e busca;
   - tratamento de erros e resultados mais semanticamente claros;
   - reducao de logica acidental em handlers e repositorios.
4. So considere domain events se houver um fato de dominio real e um fluxo que realmente os justifique; caso contrario, deixe explicitamente fora do escopo.
5. Use como inspiracao estrutural, se necessario, `.specs/prd-transactions-monthly`, mas aplique apenas a parte seletiva de smart constructors, `Decide*` puro e domain events tipados quando houver aderencia real. Nao force workflow ou eventos em modulo de leitura.
6. Evite state-as-type ou modelagem de workflow se nao houver maquina de estados real no modulo.

Focos especificos para `categories`
1. Verifique se os use cases de listagem, busca e lookup mantem a logica em application/domain e deixam handlers finos.
2. Revise `candidate_resolver`, factories e modelagem de categoria/dicionario para invariantes e nomeacao ubiqua.
3. Verifique se `etag`, metricas e adaptacao HTTP nao vazam para dominio/aplicacao.
4. Revise repositorios Postgres e contratos de leitura para manter semantica previsivel e baixo acoplamento.
5. Se o modulo nao pedir eventos de dominio, exija que a analise diga isso explicitamente em vez de inventa-los.

Restricoes obrigatorias
1. Preserve semantica de leitura e compatibilidade de API.
2. Nao crie complexidade de dominio que o modulo nao precise.
3. Nao mover regra de negocio para handlers ou repositories.
4. Nao inventar escrita, eventos ou jobs onde o workspace nao mostra isso.
5. E proibido alterar qualquer comportamento existente; se a melhoria exigir mudanca de semantica de leitura, busca ou payload, pare e declare fora de escopo.
6. Se eventos de dominio nao fizerem sentido aqui, declare a nao adocao explicitamente.

Saida esperada
1. Classifique o `task_type` mais especifico.
2. Liste hotspots de acoplamento, validacao e semantica de leitura.
3. Proponha o menor plano seguro de refatoracao.
4. Explique quais principios de DMMF entram e quais ficam de fora.
5. Liste validacoes proporcionais e testes relevantes.
6. Em `execution`, inclua review final e relatorio de refactor.

Criterios de aceitacao
- O plano deve citar apenas artefatos reais de `internal/categories`.
- As sugestoes devem respeitar que o modulo e predominantemente de leitura.
- DMMF so deve ser aplicado onde houver ganho concreto de modelagem ou invariantes.
- A resposta final deve terminar com `done`, `needs_input`, `blocked` ou `failed`.
```

## Justificativas curtas

- O modulo `categories` tem perfil mais read-oriented, entao limitei o uso de DMMF a invariantes e clareza, nao a workflows ficticios.
- Citei `etag`, metricas e factories porque aparecem como superfices reais do modulo.
- Fixei a preservacao de API e semantica de leitura para evitar refatoracao que mude comportamento sem necessidade.
