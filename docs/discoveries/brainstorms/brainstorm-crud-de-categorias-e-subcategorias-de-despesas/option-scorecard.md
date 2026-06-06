# Scorecard de Alternativas

Escala: 1 = pior ou mais oneroso; 5 = melhor ou menos oneroso no contexto da decisão.

Eixo principal avaliado: **modelo de schema/agregado** para a hierarquia categoria↔subcategoria. As demais decisões (eventos, seed, cascata, kind, MVP scope) são parametrizações ortogonais já respondidas pelo usuário.

| Alternativa | Complexidade | Tempo de entrega | Custo | Escalabilidade | Segurança | Confiabilidade | Observabilidade | Manutenibilidade | Risco operacional | Total | Observação |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Alternativa 1 - Adjacency List (tabela única + parent_id) + validação no domínio | 5 | 5 | 5 | 4 | 4 | 4 | 4 | 5 | 4 | 40 | CRUD único, schema compreensível, performance adequada para ~50 categorias/usuário. Profundidade=2 garantida no agregado. Selecionada. |
| Alternativa 2 - Duas tabelas (categories + subcategories) | 3 | 3 | 4 | 4 | 5 | 4 | 4 | 3 | 4 | 34 | Schema garante profundidade no banco, mas duplica CRUD, repositórios, handlers e use cases. Maior superfície de teste sem ganho proporcional. |
| Alternativa 3 - Closure Table (suporta N níveis, limita 2 na app) | 2 | 2 | 3 | 5 | 4 | 4 | 3 | 3 | 3 | 29 | Permite hierarquia profunda no futuro, mas é overkill agora. Queries recursivas e migração inicial complexas. Risco operacional maior. |
| Alternativa 4 - Adjacency List + projetor de read-model category_tree_view | 3 | 3 | 3 | 5 | 4 | 4 | 5 | 4 | 3 | 34 | Mais alinhado com billing (consumer + projeção); excelente observabilidade. Adiciona consumer + tabela de projeção sem ganho proporcional para baixa cardinalidade. Reavaliar se relatórios pesados surgirem. |

## Leitura do Resultado
- Alternativa mais equilibrada: Alternativa 1 — melhor combinação de simplicidade, manutenibilidade e tempo de entrega.
- Alternativa mais rápida: Alternativa 1 — menor tempo de implementação dado cardinalidade e profundidade=2.
- Alternativa mais segura (schema enforce): Alternativa 2 — FK explícita evita subcategoria de subcategoria no schema, mas Alternativa 1 cobre o mesmo no domínio.
- Alternativa mais barata: Alternativa 1 — menos tabelas, menos código, menos testes.
- Alternativa com maior risco operacional: Alternativa 3 — closure table exige cuidado com consistência da tabela auxiliar e migrações.

## Decisões Ortogonais (também consolidadas)
| Decisão | Resposta | Trade-off aceito |
| --- | --- | --- |
| Seed | Migration SQL idempotente (`ON CONFLICT DO NOTHING`) | Editar seed exige nova migration; em troca, mudanças versionadas em git. |
| Customização seed | Endpoint `POST /v1/categories/{id}/clone` | Mais código no module; em troca, evita duplicação visual em listagens. |
| Eventos | Outbox completo (publisher + consumer-ready) | Custo de tabela outbox + jobs; em troca, contrato estável para futuras integrações. |
| Cascata soft-delete | Bloquear delete se houver filhos ativos (409) | UX exige 2 passos; em troca, previne deletes acidentais em árvore. |
| `kind` (receita/despesa) | Na raiz; filhas herdam por constraint | Constraint mais complexa; em troca, modelagem inequívoca. |
| Visual (color/icon) | **Fora** do MVP (canal exclusivo WhatsApp) | Front-end gráfico fica para o futuro; seed mantém só nome+kind+parent. |
| Audit log | Entregar na primeira onda | +1 tabela e captura de quem/quando/canal; em troca, atende H3 sem refactor posterior. |
| Escopo | Wave única (com sub-waves internas) | Prazo maior (~2-3 semanas); em troca, contrato estável e robusto. |
