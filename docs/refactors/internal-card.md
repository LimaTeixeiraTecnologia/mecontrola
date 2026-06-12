# Prompt enriquecido para `internal/card`

## Prompt enriquecido

```text
Objetivo
Refatorar o modulo `internal/card` em modo `advisory` por padrao, preservando a API, o wiring atual e os contratos do workspace. So entre em `execution` se eu solicitar explicitamente.
Mandatorio: nao mudar nenhum comportamento ja existente, nem publico, nem interno observavel, nem operacional. A refatoracao deve ser estritamente sem mudanca de comportamento.

Skills obrigatorias
1. Carregue `.agents/skills/refactor/SKILL.md`.
2. Carregue `.agents/skills/go-implementation/SKILL.md`.
3. Carregue referencias de ambas apenas sob demanda e conforme a superficie real alterada.

Contrato de exploracao
1. Confirme o baseline via `AGENTS.md`, `go.mod`, `internal/card/module.go` e arquivos reais do modulo.
2. Nao invente casos de uso, endpoints, repositorios ou observabilidade inexistentes.
3. Considere explicitamente as superficies reais: `application/dtos`, `application/interfaces`, `application/usecases`, `domain/entities`, `domain/services`, `domain/valueobjects`, `infrastructure/http/server`, `infrastructure/repositories/postgres`, `infrastructure/observability` e `module.go`.

Objetivo tecnico da refatoracao
1. Melhorar modelagem e clareza do fluxo de CRUD de cartoes e `invoice_for`.
2. Siga fielmente SOMENTE os principios de `Domain Modeling Made Functional: Tackle Software Complexity with Domain-Driven Design and F#` que tragam beneficio objetivo no contexto Go.
3. Priorize quando fizer sentido:
   - smart constructors para value objects como nomes, apelidos e billing cycle;
   - services pequenos e deterministas para calculos de fatura e timezone;
   - use cases com workflow mais explicito e menos branching incidental;
   - domain events tipados apenas se houver fato de dominio relevante que justifique isso no modulo;
   - erros de dominio e validacao mais semanticos;
   - fronteiras mais nidas entre DTO, dominio e persistencia.
4. Quando eventos fizerem sentido, a decisao do evento deve ocorrer na camada de dominio/aplicacao, com producers finos cuidando apenas do mapeamento/publicacao.
5. Use como inspiracao estrutural, se necessario, `.specs/prd-transactions-monthly`, principalmente a adocao seletiva de smart constructors, `Decide*` puro e domain events tipados. Nao replique complexidade de workflows que o modulo nao precisa.
6. Nao introduza modelagem sofisticada demais se o ganho nao superar o custo para um modulo CRUD relativamente enxuto.

Focos especificos para `card`
1. Revisar se handlers HTTP continuam finos e se a regra de negocio esta concentrada em use cases e dominio.
2. Verificar se `invoice_for` e calculos de ciclo/fuso estao modelados no lugar certo.
3. Avaliar se algum evento de dominio faz sentido para o modulo e, se fizer, se ele e decidido fora do producer.
4. Verificar se repositorio Postgres e camada de observabilidade estao desacoplados do dominio.
5. Revisar o wiring de `module.go` e o contrato dos DTOs de entrada/saida sem quebrar a API.

Restricoes obrigatorias
1. Preserve contratos publicos e semantica de endpoints existentes.
2. Nao mover regra de negocio para handler ou repositorio.
3. Nao criar interfaces sem consumidor real.
4. Se houver eventos de dominio, producer deve ser apenas adapter fino.
5. E proibido alterar qualquer comportamento existente; se a melhoria exigir mudanca de API, calculo ou fluxo, pare e declare fora de escopo.
6. Nao usar `panic`, `init()` ou padrões artificiais so para "parecer DDD".

Saida esperada
1. Classifique o `task_type`.
2. Liste dores atuais, especialmente duplicacao, validacao dispersa e acoplamento indevido.
3. Proponha um plano incremental pequeno.
4. Marque onde DMMF ajuda e onde nao ajuda.
5. Liste validacoes proporcionais e testes de regressao relevantes.
6. Em `execution`, exija review final e relatorio de refactor.

Criterios de aceitacao
- O plano deve referenciar apenas caminhos reais de `internal/card`.
- A refatoracao proposta deve manter o modulo simples, sem overengineering.
- Cada recomendacao de DMMF deve ter ganho claro de invariantes, legibilidade ou previsibilidade.
- A resposta final deve terminar com `done`, `needs_input`, `blocked` ou `failed`.
```

## Justificativas curtas

- Ajustei o prompt para o perfil mais enxuto de `card`, evitando incentivar overengineering.
- Chamei `invoice_for`, billing cycle e timezone porque aparecem como pontos reais de modelagem do modulo.
- Mantive a exigencia de adapters finos e preservacao de API por ser um modulo fortemente exposto por HTTP.
