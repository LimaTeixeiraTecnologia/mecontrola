# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 24 -->

<!--
Histórico de versões:
- v1 (2026-06-06): versão inicial derivada do brainstorming e discovery técnico validados.
- v2 (2026-06-06): catálogo fixo de budgets fechado com as cinco categorias oficiais do MeControla.
- v3 (2026-06-06): ordem determinística de distribuição dos centavos residuais fechada.
- v4 (2026-06-06): confiança e autorização dos produtores internos de eventos fechadas; PRD pronto para especificação técnica.
- v5 (2026-06-06): alertas redefinidos para ocorrer somente no cruzamento real de limiares.
- v6 (2026-06-06): limite de alertas separado por categoria e limiar, protegendo alertas críticos de 100%.
- v7 (2026-06-06): API e eventos unificados sob controle otimista por versão monotônica.
- v8 (2026-06-06): consultas e correções retroativas limitadas à retenção de 24 meses.
- v9 (2026-06-06): exclusão física definida como remoção das consultas/totais, mantendo somente tombstone técnico invisível.
- v10 (2026-06-06): limiares são rearmados imediatamente após o gasto cair abaixo deles.
- v11 (2026-06-06): sequência monotônica fechada com criação na versão 1 e mutações incrementais unitárias.
- v12 (2026-06-06): recorrência definida como operação parcial explícita por competência.
- v13 (2026-06-06): expurgo condicionado à ausência de dependências de integridade e idempotência.
- v14 (2026-06-06): rascunho automático condicionado ao commit bem-sucedido da primeira despesa válida.
- v15 (2026-06-06): correções retroativas não geram alerta ao usuário; somente sinal operacional.
- v16 (2026-06-06): fuso de negócio do MVP fixado em America/Sao_Paulo.
- v17 (2026-06-06): despesas restritas a valores positivos maiores que zero.
- v18 (2026-06-06): ativação restrita a orçamento total positivo maior que zero.
- v19 (2026-06-06): alerta assíncrono deve revalidar o estado atual e suprimir sinais obsoletos.
- v20 (2026-06-06): catálogo de despesas migrado para subcategorias de `internal/categories`; auth E1 obrigatório na API; eventos pendentes e alertas persistidos em tabelas dedicadas; exclusão de rascunho permitida; tombstone libera reuso após 24m; sinal de rascunho abandonado via cronjob 03:00 BR; allowlist estática de produtores; identidade API com `external_transaction_id` no body e `source="api"` fixo pelo servidor.
- v21 (2026-06-06): contrato de leitura com `internal/categories` formalizado por slugs editoriais imutáveis; estado terminal de evento pendente desambiguado entre `failed` (erro permanente) e `expired` (timeout); versão monotônica fixada explicitamente para criação (v=1 imposta pelo servidor) e exclusão (tombstone congela a próxima versão); avaliação de alerta declarada como assíncrona via outbox interno; formato canônico de `external_transaction_id` (UUID v4 ou ULID) e chave de competência (`YYYY-MM` em America/Sao_Paulo) fixados no PRD; cardinalidade de métricas operacionais limitada; recorrência exige fonte com 100% alocados; demais ajustes editoriais para prontidão de especificação técnica.
- v22 (2026-06-08): estouro de orçamento explicitamente permitido; despesas podem levar o percentual utilizado acima de 100% sem bloqueio, rejeição, reversão ou truncamento do resumo.
- v23 (2026-06-08): bump pós-`prd-auth-foundation` task 9.0. Endpoints autenticados da API usarão o `RequireUser` canônico de `internal/identity/infrastructure/http/server/middleware` (via `auth.Principal` no `context.Context` injetado pelo `EstablishPrincipal`), removendo dependência do header transitório `X-User-ID`. Referência: `prd-auth-foundation`.
- v24 (2026-06-09): decisões de production-readiness fechadas antes do handoff para techspec — (1) endpoints administrativos HTTP (RF-39b original e RF-64c expansivo) movidos para fora do MVP via OUT-16, mantendo persistência e observabilidade por SQL/Grafana; (2) estado de cruzamento de limiar formalizado em tabela dedicada `budgets_threshold_state` com versão monotônica (novos RF-60e/RF-60f); (3) allowlist de produtores fixada como constante Go versionada em `internal/budgets/infrastructure/config/` (RT-28); (4) resumo mensal calculado on-demand com índice composto obrigatório (RT-29); (5) cronjobs de RF-18b e RF-66 executados pelo scheduler in-process via `internal/budgets/infrastructure/jobs/handlers/` (RT-30); (6) cache de categories com raízes resolvidas no boot e subcategorias com TTL 60s + bust por `editorial_version` (RT-31).
-->

> **Origem**: brainstorming decisório em `docs/discoveries/brainstorms/brainstorm-modulo-de-orcamentos-mensais-por-categoria/` e discovery técnico em `docs/discoveries/technical-modulo-de-orcamentos-mensais-por-categoria/`, ambos validados com `SUCCESS` em 2026-06-06.

## Visão Geral

O `mecontrola` ainda não permite que uma pessoa defina quanto pretende gastar em um mês, distribua esse valor entre categorias financeiras e acompanhe o consumo dessas metas conforme despesas são lançadas.

Este PRD define o MVP production-ready de **orçamentos mensais por categoria**. Cada usuário poderá possuir um único orçamento por competência mensal, informar o valor total, distribuir exatamente 100% entre as cinco categorias raiz oficiais de despesa providas por `internal/categories` (Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira) e acompanhar valor planejado, valor gasto e percentual utilizado. Cada despesa carrega a `subcategory_id` informada pelo cliente; budgets agrega gasto pela raiz correspondente.

Despesas poderão entrar por API ou por eventos internos. O estado financeiro atual deverá permanecer correto mesmo com retries, duplicidades, eventos atrasados, edições, exclusões e mudanças retroativas de competência ou categoria.

O MVP privilegia eficiência e integridade financeira. Alertas são auxiliares e não podem comprometer lançamentos. Para evitar falso positivo, um alerta somente poderá ser criado quando existir orçamento ativado, cálculo válido e limiar realmente atingido por uma mutação financeira nova e aplicável.

## Objetivos

- **OBJ-01**: Permitir planejamento financeiro mensal com distribuição integral do orçamento entre as cinco raízes oficiais de despesa fornecidas por `internal/categories`.
- **OBJ-02**: Exibir uma visão mensal correta do planejado, gasto e percentual utilizado por categoria.
- **OBJ-03**: Aceitar despesas por API e evento sem duplicar ou regredir o estado financeiro.
- **OBJ-04**: Permitir correções retroativas de despesas sem manter acumulados divergentes.
- **OBJ-05**: Preparar alertas confiáveis de aproximação e estouro de meta, sem falso positivo e sem bloquear despesas.
- **OBJ-06**: Reduzir configuração repetitiva por recorrência de até 12 competências futuras.
- **OBJ-07**: Entregar o núcleo completo usando capacidades existentes, sem introduzir infraestrutura sem evidência de necessidade.

### Métricas de Sucesso

- **M-01**: Zero duplicidade financeira em testes de retry concorrente pela mesma identidade externa.
- **M-02**: Zero aplicação de mutação com versão duplicada, regressiva ou com lacuna em testes de aceitação.
- **M-03**: 100% dos resumos mensais conciliam com a soma das despesas canônicas existentes.
- **M-04**: Zero alertas criados para orçamento rascunho, cálculo inválido, retry duplicado ou evento ignorado.
- **M-05**: p95 de até 300 ms para escritas confirmadas e consulta do resumo mensal no perfil de até 100 despesas por usuário/mês.
- **M-06**: Disponibilidade mensal de 99,9% para o núcleo financeiro.
- **M-07**: 100% das exclusões impedem recriação por retry do mesmo identificador externo durante a retenção.
- **M-08**: 100% dos dados elegíveis são expurgados após 24 meses sem remover registros ainda dentro da retenção.
- **M-09**: Alertas persistidos respeitam limites independentes por usuário, competência, categoria e limiar; alertas de 100% nunca são descartados por consumo do limite de 80% ou de outra categoria.
- **M-10**: Recorrência nunca cria mais de um orçamento para o mesmo usuário e competência.

## Histórias de Usuário

- **US-01 — Planejar o mês**
  Como usuário, quero informar meu orçamento total mensal e distribuir 100% entre categorias, para saber quanto devo gastar em cada finalidade.

- **US-02 — Acompanhar consumo**
  Como usuário, quero consultar planejado, gasto e percentual utilizado por categoria, para decidir se ainda posso gastar.

- **US-03 — Registrar despesas por diferentes entradas**
  Como usuário, quero que despesas lançadas pela API ou por integrações internas apareçam uma única vez, para confiar no total apresentado.

- **US-04 — Corrigir uma despesa**
  Como usuário, quero editar valor, categoria ou competência de uma despesa, para que os meses afetados reflitam o estado correto.

- **US-05 — Excluir e recriar corretamente**
  Como usuário, quero excluir uma despesa e poder registrar uma nova despesa equivalente, sem que retries da despesa excluída a recriem.

- **US-06 — Planejar meses futuros**
  Como usuário, quero repetir uma configuração mensal por até 12 meses, para reduzir configuração manual sem alterar meses já ativados.

- **US-07 — Não perder despesas sem orçamento**
  Como usuário, quero que uma despesa recebida antes da configuração do mês permaneça visível, para não perder controle financeiro.

- **US-08 — Receber alertas confiáveis**
  Como usuário, quero alertas quando atingir 80% ou 100% de uma categoria, sem mensagens falsas causadas por retries ou orçamento incompleto.

- **US-09 — Operar o produto**
  Como operador, quero identificar duplicidades, regressões temporais, eventos pendentes e falhas operacionais, para agir sem alterar valores corretos.

- **US-10 — Integrar produtores internos**
  Como produtor interno, quero um contrato canônico de eventos de despesas, para criar, atualizar e excluir despesas de forma determinística.

## Funcionalidades Core

### F-01 — Orçamento Mensal

Permite criar e excluir orçamento em rascunho por usuário e competência. O orçamento começa como rascunho e somente pode ser ativado quando possuir valor total e exatamente 100% distribuído entre as cinco raízes oficiais. Após ativação, o orçamento é imutável e somente expira por retenção.

### F-02 — Distribuição por Categorias Raiz Oficiais

Permite definir percentuais em basis points para as cinco categorias raiz oficiais de despesa providas por `internal/categories`: **Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira**. Valores planejados são calculados em centavos BRL com half-even e distribuição determinística de centavos residuais. budgets não mantém catálogo próprio nem expõe mutação editorial.

### F-03 — Recorrência Limitada

Permite criar ou atualizar uma série de até 12 competências futuras a partir de uma competência fonte (`source_competence`) explícita, cujo valor total e alocações somam exatamente 100%. A recorrência completa rascunhos automáticos existentes nas competências futuras e altera somente meses futuros ainda não ativados.

### F-04 — Despesas Canônicas

Mantém o estado atual de cada despesa, aceitando criação, atualização e exclusão por API ou evento. Cada despesa carrega `subcategory_id` (FK ao catálogo global de `internal/categories`); a identidade externa `(user_id, source, external_transaction_id)` impede duplicidade entre retries.

### F-05 — Rascunho Automático

Quando uma despesa chega para uma competência sem orçamento, cria um rascunho sem valor planejado. Gastos permanecem visíveis; metas e alertas ficam indisponíveis até ativação.

### F-06 — Resumo Mensal Calculado

Calcula valores gastos diretamente das despesas existentes, evitando acumulados persistidos que possam divergir após correções retroativas.

### F-07 — Alertas sem Falso Positivo

Após uma mutação confirmada, avalia limiares de 80% e 100%. Um alerta ocorre somente quando o gasto cruza de abaixo para igual ou acima do limiar. Novo alerta do mesmo limiar somente pode ocorrer depois que o gasto cair abaixo dele e cruzá-lo novamente. Alertas são auxiliares, limitados, não bloqueiam despesas e não existem para rascunhos.

### F-08 — Retenção e Operação

Mantém orçamentos, despesas e tombstones por 24 meses, sinaliza eventos fora de ordem e rascunhos abandonados e fornece telemetria operacional.

## Requisitos Funcionais

### Orçamento Mensal e Ativação

- **RF-01**: O sistema DEVE permitir criar orçamento mensal informando usuário, competência e valor total em centavos BRL.
- **RF-02**: O sistema DEVE garantir no máximo um orçamento por usuário e competência.
- **RF-03**: Um orçamento criado manualmente DEVE iniciar como rascunho.
- **RF-04**: O sistema DEVE aceitar alocações exclusivamente nas cinco categorias raiz oficiais de despesa providas pelo módulo `internal/categories` (`kind=expense`, `parent_id=NULL`).
- **RF-04a**: As cinco raízes oficiais DEVEM ser identificadas pelos slugs editoriais imutáveis (caso, hifenização e prefixo cravados) fornecidos por `internal/categories`: `expense.custo_fixo`, `expense.conhecimento`, `expense.prazeres`, `expense.metas` e `expense.liberdade_financeira`. budgets DEVE consumir os IDs resolvidos a partir desses slugs e referenciar internamente as raízes por slug, nunca por nome humano.
- **RF-04b**: budgets NÃO DEVE manter catálogo próprio das raízes nem duplicar slugs ou nomes; a resolução de slug para PK DEVE ocorrer via interface de leitura exposta por `internal/categories` (RT-23).
- **RF-04c**: budgets NÃO DEVE expor criação, edição, ocultação ou remoção de categorias raiz ou subcategorias; mutações editoriais são responsabilidade exclusiva de `internal/categories`.
- **RF-04d**: budgets DEVE validar, antes de persistir alocação ou despesa, que o identificador informado pertence a uma raiz oficial (em alocação) ou a uma subcategoria com `kind=expense` cujo `parent_id` pertence ao conjunto das cinco raízes oficiais (em despesa).
- **RF-04e**: A validação de RF-04d NÃO DEVE rejeitar identificadores com `deprecated_at` definido; despesas e alocações em subcategorias descontinuadas permanecem válidas, consultáveis e exibem o caminho completo histórico.
- **RF-05**: Cada alocação DEVE informar percentual em basis points inteiros.
- **RF-06**: A soma dos percentuais DEVE ser menor ou igual a 100% durante edição do rascunho.
- **RF-07**: O sistema DEVE permitir ativação somente quando o valor total estiver definido e a soma dos percentuais for exatamente 100%.
- **RF-07a**: O valor total necessário para ativação DEVE ser inteiro em centavos e maior que zero.
- **RF-07b**: Orçamento com valor total zero ou negativo NÃO DEVE ser ativado.
- **RF-08**: Após ativação, valor total e alocações do orçamento mensal NÃO DEVEM ser alterados.
- **RF-09**: Despesas da competência DEVEM continuar criáveis, editáveis e excluíveis após ativação e após o fim do mês, enquanto a competência estiver dentro da retenção de 24 meses.
- **RF-09a**: Após o término da retenção e expurgo, orçamento e despesas da competência NÃO DEVEM permanecer consultáveis ou corrigíveis.
- **RF-09b**: Orçamento em estado de rascunho (manual ou automático) DEVE ser exclusível pelo usuário autenticado via API. A exclusão DEVE remover rascunho e alocações sem afetar despesas da competência.
- **RF-09c**: Orçamento ativado NÃO DEVE ser exclusível por API; permanece consultável e corrigível até o expurgo da retenção.
- **RF-09d**: Após exclusão de rascunho, nova despesa válida na mesma competência DEVE acionar a regra RF-12 de criação de rascunho automático.
- **RF-10**: Valores planejados por categoria DEVEM ser calculados em centavos, com arredondamento half-even.
- **RF-11**: Centavos residuais da distribuição DEVEM ser atribuídos por ordem determinística, garantindo que a soma planejada seja igual ao valor total.
- **RF-11a**: A ordem determinística para atribuir centavos residuais DEVE seguir a sequência de slugs editoriais imutáveis: `expense.custo_fixo`, `expense.conhecimento`, `expense.prazeres`, `expense.metas`, `expense.liberdade_financeira`. Cada passagem atribui no máximo um centavo por raiz até eliminar a diferença. A ordem NÃO DEVE depender de PKs, ordem de seed nem nomes humanos.

### Rascunho Automático

- **RF-12**: Quando uma despesa válida chegar para competência sem orçamento, o sistema DEVE criar automaticamente um orçamento rascunho único.
- **RF-12a**: O rascunho automático DEVE ser criado somente na mesma operação confirmada que persiste a primeira despesa válida da competência.
- **RF-12b**: Tentativa rejeitada, evento inválido, evento duplicado, conflito de versão ou mutação não persistida NÃO DEVE criar rascunho automático.
- **RF-13**: O rascunho automático NÃO DEVE possuir valor total ou alocações inventadas pelo sistema.
- **RF-14**: A consulta do rascunho automático DEVE mostrar despesas e gastos por categoria.
- **RF-15**: A consulta do rascunho automático DEVE indicar planejado e percentual utilizado como indisponíveis.
- **RF-16**: O rascunho automático NÃO DEVE gerar alertas percentuais.
- **RF-17**: O sistema DEVE permitir novas despesas em rascunho automático sem bloqueio.
- **RF-18**: Ao fim da competência, rascunhos automáticos não ativados DEVEM gerar sinal operacional observável, sem notificar obrigatoriamente o usuário.
- **RF-18a**: Competência corrente, fim do mês, recorrência e classificação de correção retroativa DEVEM usar o fuso `America/Sao_Paulo`.
- **RF-18b**: O sistema DEVE executar tarefa diária às 03:00 em `America/Sao_Paulo` que avalia rascunhos não ativados cuja competência tenha terminado antes da data corrente; cada rascunho identificado DEVE produzir métrica e log estruturado.
- **RF-18c**: A tarefa de RF-18b DEVE ser idempotente; o mesmo rascunho NÃO DEVE gerar métrica/log duplicado entre execuções consecutivas (controle por flag persistente ou checkpoint observável).
- **RF-18d**: A tarefa de RF-18b NÃO DEVE notificar o usuário final e NÃO DEVE alterar o estado financeiro.

### Recorrência

- **RF-19**: O usuário DEVE poder solicitar recorrência por no máximo 12 competências futuras.
- **RF-20**: Cada competência criada por recorrência DEVE possuir orçamento mensal independente.
- **RF-21**: Ao criar recorrência, o sistema DEVE criar meses ausentes e retornar explicitamente competências em conflito.
- **RF-21a**: A criação ou alteração de recorrência DEVE retornar resultado individual por competência, distinguindo criado, atualizado, completado a partir de rascunho, conflito e falha.
- **RF-21b**: Sucesso parcial DEVE ser informado explicitamente; conflitos ou falhas em uma competência NÃO DEVEM ser apresentados como sucesso nem desfazer competências aplicadas com sucesso.
- **RF-22**: A recorrência DEVE completar rascunhos automáticos existentes, preservando suas despesas.
- **RF-23**: Alterações da recorrência DEVEM afetar somente competências futuras ainda não ativadas.
- **RF-23a**: A operação de recorrência DEVE receber explicitamente o `source_competence` consumido como template e DEVE rejeitar fontes inválidas: rascunho automático sem alocações, rascunho manual com soma de alocações diferente de 100%, competência sem valor total positivo ou competência expurgada. Orçamento ativado e rascunho com 100% distribuídos são fontes válidas.
- **RF-24**: A recorrência NÃO DEVE sobrescrever orçamento ativado.

### Despesas por API

- **RF-25**: A API DEVE permitir criar despesa informando `external_transaction_id`, `subcategory_id`, valor em centavos e competência. O `user_id` DEVE ser derivado do JWT autenticado de `internal/identity` e o `source` DEVE ser fixado pelo servidor como `"api"`; ambos NÃO DEVEM ser aceitos no payload.
- **RF-25a**: O valor da despesa DEVE ser inteiro em centavos e maior que zero.
- **RF-25b**: Valor zero ou negativo DEVE ser rejeitado; correções e estornos DEVEM ocorrer por edição ou exclusão explícita.
- **RF-25c**: O `external_transaction_id` DEVE ser gerado pelo cliente conforme o formato canônico definido em RT-26; o servidor NÃO DEVE gerar, substituir ou normalizar esse identificador.
- **RF-25d**: O `subcategory_id` informado em criação DEVE atender RF-04d antes do commit; rejeição NÃO DEVE alterar o estado financeiro.
- **RF-26**: A API DEVE permitir editar valor, `subcategory_id` e competência de uma despesa existente. O `subcategory_id` editado DEVE atender RF-04d.
- **RF-27**: A API DEVE permitir excluir fisicamente os valores financeiros de uma despesa existente.
- **RF-27a**: Após exclusão física, a despesa NÃO DEVE aparecer em consultas nem contribuir para totais.
- **RF-28**: A API DEVE confirmar criação/edição/exclusão somente após a operação financeira ser persistida.
- **RF-29**: A resposta de criação/edição DEVE retornar o estado e a versão monotônica atuais da despesa.
- **RF-29a**: Toda edição ou exclusão via API DEVE informar a versão esperada da despesa.
- **RF-29b**: Mutação via API com versão esperada divergente da versão atual DEVE ser rejeitada como conflito, sem alterar o estado.
- **RF-29c**: Toda despesa DEVE ser criada na versão 1; cada edição ou exclusão válida DEVE exigir a versão atual e produzir exatamente a próxima versão inteira.
- **RF-29d**: Criação de despesa via API ou evento NÃO DEVE aceitar campo `version` no payload. O servidor DEVE impor `version=1` no commit; tentativa de envio explícito de `version` na criação DEVE ser rejeitada como payload inválido sem alterar o estado.
- **RF-29e**: Exclusão válida DEVE produzir a próxima versão inteira (`current_version + 1`) e registrá-la no tombstone como `tombstone_version`. Retries de exclusão pela mesma identidade canônica DEVEM ser idempotentes ao reapresentar a mesma versão esperada que produziu o tombstone, retornando sucesso sem nova mutação.
- **RF-30**: Alterações de competência ou `subcategory_id` DEVEM refletir no resumo dos meses e raízes afetados no mesmo commit transacional da mutação.
- **RF-31**: Novas despesas DEVEM sempre informar `subcategory_id` válido conforme RF-04d.

### Despesas por Evento

- **RF-32**: Budgets DEVE publicar e governar um contrato canônico interno para eventos de criação, atualização e exclusão de despesas.
- **RF-32a**: Somente produtores internos previamente registrados e autorizados DEVEM poder publicar eventos aceitos por budgets.
- **RF-32b**: Cada produtor registrado DEVE possuir `source` estável e não reutilizável, associado à sua identidade operacional.
- **RF-32c**: Evento de `source` ausente, desconhecido ou não autorizado DEVE ser rejeitado sem alterar o estado financeiro e registrado operacionalmente.
- **RF-33**: Todo evento DEVE informar `event_id`, `source`, `external_transaction_id`, `occurred_at`, usuário, operação e versão monotônica.
- **RF-34**: Eventos sem os campos obrigatórios ou com valores inválidos DEVEM ser rejeitados sem alterar o estado financeiro.
- **RF-35**: Eventos duplicados pela mesma identidade externa/operação NÃO DEVEM produzir nova mutação.
- **RF-36**: Para uma despesa existente, somente mutação que informe exatamente a próxima versão esperada DEVE alterar o estado.
- **RF-36a**: Evento de criação DEVE informar versão 1; criação com qualquer outra versão DEVE ser rejeitada sem alterar o estado.
- **RF-37**: Evento com versão já aplicada ou regressiva DEVE ser tratado como duplicado/obsoleto sem alterar o estado.
- **RF-38**: Evento com lacuna de versão ou recebido antes da criação DEVE permanecer pendente por até 24 horas em estado `pending`.
- **RF-39**: Evento ainda inaplicável após a janela de 24 horas DEVE transitar para estado terminal `expired`, observável, sem criar estado financeiro incompleto e sem agir sobre o agregado de despesa.
- **RF-39a**: Eventos pendentes (lacuna de versão ou recebidos antes da criação) DEVEM ser persistidos em tabela dedicada `budgets_expense_events_pending`, com máquina de estados `pending → applied | failed | expired`. O estado `failed` DEVE ser usado exclusivamente para erro permanente (validação de schema, autorização, identidade canônica inválida, regra de versão definitivamente impossível); o estado `expired` DEVE ser usado exclusivamente para timeout de 24h sem aplicabilidade. As transições DEVEM ser idempotentes e auditáveis.
- **RF-39b**: No MVP, a inspeção de eventos pendentes DEVE permanecer consultável exclusivamente via SQL e dashboards de observabilidade (sem endpoint HTTP administrativo, conforme OUT-16). A tabela `budgets_expense_events_pending` DEVE expor colunas indexadas suficientes para filtros por `source`, `user_id`, estado e janela temporal direto no banco.
- **RF-39c**: Métricas DEVEM cobrir taxa de eventos por estado, idade do pendente mais antigo e contagem de transições para `failed` e `expired`.
- **RF-40**: API e eventos DEVEM compartilhar a mesma regra de versão monotônica e conflito.
- **RF-41**: `occurred_at` DEVE ser preservado como data de negócio/auditoria, mas NÃO DEVE definir a autoridade concorrente da mutação.

### Idempotência, Exclusão e Recriação

- **RF-42**: A identidade canônica de despesa DEVE considerar usuário, origem e `external_transaction_id`.
- **RF-43**: Retry da mesma criação por API ou evento DEVE retornar/manter a despesa existente sem duplicar valor.
- **RF-44**: Após exclusão física, o sistema DEVE manter tombstone sem valores financeiros por 24 meses.
- **RF-45**: Retry usando identidade presente em tombstone NÃO DEVE recriar a despesa.
- **RF-46**: Recriação intencional após exclusão DEVE utilizar novo `external_transaction_id`.
- **RF-47**: Tombstone NÃO DEVE ser apresentado como despesa nem contribuir para qualquer total.
- **RF-47a**: Tombstone é metadado técnico interno e NÃO DEVE ser exposto em consultas do usuário.
- **RF-47b**: Após expurgo do tombstone (24 meses contados da exclusão), a identidade `(user_id, source, external_transaction_id)` DEVE ser considerada livre; nova despesa com o mesmo trio DEVE ser criada normalmente sem rejeição por idempotência.

### Resumo e Cálculos

- **RF-48**: O sistema DEVE oferecer resumo por usuário e competência mensal.
- **RF-49**: O resumo DEVE calcular valor gasto diretamente das despesas canônicas existentes.
- **RF-50**: O resumo de orçamento ativado DEVE mostrar, por raiz oficial: percentual definido, valor planejado, valor gasto agregado de todas as subcategorias filhas e percentual utilizado.
- **RF-51**: O resumo DEVE mostrar total gasto, total planejado e percentual total utilizado.
- **RF-52**: Gastos acima do planejado DEVEM permanecer válidos e visíveis.
- **RF-52a**: Percentuais utilizados por categoria e no total DEVEM poder ultrapassar 100% quando o gasto exceder o planejado; o sistema NÃO DEVE truncar, limitar, rejeitar, bloquear ou reverter despesas por estouro de orçamento.
- **RF-53**: O resumo NÃO DEVE depender da execução de alertas, jobs ou providers externos.
- **RF-54**: O sistema NÃO DEVE persistir acumulado financeiro como fonte de verdade no MVP.
- **RF-54a**: O resumo mensal DEVE expor agregação somente por raiz oficial; granularidade por subcategoria permanece como dado interno e NÃO DEVE compor o contrato do resumo no MVP.

### Alertas sem Falso Positivo

- **RF-55**: O sistema DEVE avaliar limiares de 80% e 100% por categoria após mutação financeira confirmada.
- **RF-56**: A avaliação de alertas DEVE ocorrer após o commit e NÃO DEVE bloquear ou reverter a despesa.
- **RF-56a**: Antes de persistir ou disponibilizar um alerta, o processamento assíncrono DEVE recalcular o estado financeiro atual da categoria.
- **RF-56b**: Se o limiar não permanecer cruzado no estado atual, o alerta DEVE ser suprimido como obsoleto e registrado operacionalmente.
- **RF-57**: Um alerta somente DEVE ser criado quando o orçamento estiver ativado, o valor planejado for válido e o gasto calculado atingir ou ultrapassar o limiar.
- **RF-58**: Retry duplicado, evento em estado não terminal (`pending`) ou terminal não aplicado (`failed`, `expired`), mutação rejeitada por validação ou autorização, conflito de versão e orçamento em rascunho NÃO DEVEM gerar alerta.
- **RF-59**: Um alerta DEVE ser criado somente quando o gasto da categoria cruzar de abaixo para igual ou acima do limiar.
- **RF-60**: Enquanto o gasto permanecer igual ou acima do limiar, novas despesas ou alterações NÃO DEVEM gerar novo alerta daquele limiar.
- **RF-60a**: Após o gasto cair abaixo do limiar, um novo cruzamento de baixo para cima PODE gerar novo alerta daquele limiar.
- **RF-60b**: Edição ou exclusão que reduza o gasto abaixo do limiar DEVE rearmar imediatamente aquele limiar para futuros cruzamentos.
- **RF-60c**: Cruzamento de limiar causado por correção em competência anterior à competência corrente NÃO DEVE gerar alerta ao usuário.
- **RF-60d**: Cruzamento retroativo suprimido DEVE gerar sinal operacional observável, sem afetar o estado financeiro.
- **RF-60e**: O estado de cruzamento de cada limiar DEVE ser persistido em tabela dedicada `budgets_threshold_state` com chave `(user_id, competence, root_slug, threshold)` e colunas mínimas `currently_crossed` (boolean), `last_crossed_at` (timestamptz, UTC), `last_uncrossed_at` (timestamptz nullable, UTC), `version` (bigint monotônico incrementado a cada transição) e `last_evaluated_committed_at` (timestamptz, UTC). A transição DEVE ser atualizada exclusivamente pelo avaliador assíncrono em UPSERT idempotente.
- **RF-60f**: O avaliador DEVE ler `budgets_threshold_state` antes de decidir emissão; nova linha em `budgets_alerts` SOMENTE DEVE ser criada quando `currently_crossed` transitar de `false` para `true` no recálculo do estado financeiro atual da categoria. Evento de outbox cuja avaliação resulte em "permanece cruzado" ou "permanece abaixo" DEVE atualizar apenas `last_evaluated_committed_at` (ou nada) sem criar alerta.
- **RF-61**: O sistema DEVE aplicar limite independente de 10 alertas por usuário, competência, categoria e limiar.
- **RF-61a**: Alertas de 80% e 100% DEVEM possuir contadores independentes; alertas de uma categoria NÃO DEVEM consumir o limite de outra categoria.
- **RF-62**: Alertas excedentes do mesmo usuário, competência, categoria e limiar DEVEM ser descartados de forma observável e NÃO DEVEM afetar o estado financeiro.
- **RF-63**: Falha no processamento ou futura entrega de alerta NÃO DEVE impedir novas despesas ou consultas.
- **RF-64**: O MVP DEVE preparar contrato/provider para futura integração, sem exigir envio real por WhatsApp.
- **RF-64a**: Alertas confirmados DEVEM ser persistidos em tabela `budgets_alerts` com estado mínimo `pending_delivery | delivered | suppressed_stale | suppressed_retroactive | rate_limited`.
- **RF-64b**: A API DEVE expor `GET /v1/budgets/alerts` paginado por cursor, com filtros por competência, raiz oficial e limiar, restrito ao `user_id` autenticado.
- **RF-64c**: A listagem ao usuário final (`GET /v1/budgets/alerts`) DEVE retornar somente alertas com estado relevante (`pending_delivery` e `delivered`). No MVP, estados auxiliares (`suppressed_stale`, `suppressed_retroactive`, `rate_limited`) DEVEM permanecer persistidos em `budgets_alerts` e observáveis via SQL e dashboards, sem endpoint HTTP administrativo (OUT-16). O contrato público da API NÃO DEVE expor esses estados.
- **RF-64d**: No MVP, sem provider externo (OUT-01), `delivered` DEVE ser atribuído quando o alerta é persistido com sucesso em `budgets_alerts`. A introdução futura do provider WhatsApp DEVE estender a máquina com estado `delivery_failed` sem quebrar contrato dos estados existentes; o MVP NÃO DEVE expor `delivery_failed` no contrato público.

### Retenção e Operação

- **RF-65**: Orçamentos e despesas existentes DEVERÃO permanecer consultáveis e corrigíveis por 24 meses; tombstones DEVERÃO preservar idempotência pelo mesmo período contado da exclusão.
- **RF-66**: O sistema DEVE executar expurgo mensal em lotes para dados elegíveis com mais de 24 meses.
- **RF-67**: O expurgo NÃO DEVE remover dados dentro da retenção ou necessários para preservar idempotência vigente.
- **RF-67a**: Dados financeiros com eventos pendentes em estado não terminal (`pending`) ou outras dependências de integridade vigentes NÃO DEVEM ser expurgados até a dependência ser resolvida ou expirar. Tombstones possuem ciclo de retenção independente de 24 meses contados da exclusão e DEVEM ser expurgados separadamente; tombstone vigente bloqueia reuso de identidade canônica (RF-47b) mas NÃO bloqueia expurgo de outros dados financeiros que não o referenciem.
- **RF-67b**: Adiamentos de expurgo por dependência DEVEM ser observáveis e NÃO DEVEM ser tratados como sucesso de expurgo.
- **RF-68**: O sistema DEVE registrar métricas, logs estruturados e traces para operações financeiras, duplicidades, regressões temporais, eventos pendentes, alertas e expurgo.
- **RF-69**: Logs e traces NÃO DEVEM expor payload financeiro completo nem valores anteriores excluídos/editados.
- **RF-70**: Operadores DEVEM conseguir distinguir falha do núcleo financeiro de falha auxiliar de alertas/jobs.

### Autenticação e Autoridade

- **RF-71**: Todos os endpoints HTTP de budgets DEVEM exigir JWT emitido pelo módulo `internal/identity` (E1).
- **RF-71a**: O `user_id` operado por toda requisição DEVE ser derivado do JWT autenticado e NÃO DEVE ser aceito no payload, query string ou header.
- **RF-71b**: Tentativa de consultar ou mutar dado pertencente a outro `user_id` DEVE retornar `403 Forbidden` sem revelar a existência do recurso.
- **RF-72**: A allowlist de produtores internos autorizados (RF-32a) DEVE ser definida em configuração revisada do módulo budgets; alterações exigem deploy.
- **RF-72a**: budgets NÃO DEVE expor endpoint runtime para cadastrar, editar ou remover produtores autorizados.

## Experiência do Usuário

### Usuário Final

1. O usuário cria o orçamento de uma competência, informa o valor total e distribui 100% entre categorias.
2. O sistema apresenta planejado por categoria antes da ativação.
3. Após ativar, o usuário registra ou recebe despesas por integrações.
4. O resumo mensal mostra gastos reais e metas imediatamente após operações confirmadas.
5. Gastos acima da meta permanecem registrados, são destacados e podem exibir percentual utilizado acima de 100%.
6. Caso despesas cheguem antes da configuração, elas aparecem em rascunho automático sem metas falsas.
7. O usuário pode corrigir despesas de meses anteriores e consultar o estado atual recalculado.

### Regras de Clareza

- Rascunho e orçamento ativado DEVEM ser visualmente/semanticamente distintos.
- Planejado e percentual utilizado não podem aparecer como zero quando estiverem indisponíveis; devem ser explicitamente indisponíveis.
- Alertas descartados ou falhos não devem alterar o resumo financeiro.
- O usuário não deve receber indicação de estouro baseada em orçamento incompleto.
- Estouro de orçamento não deve impedir novas despesas; a experiência deve sinalizar excesso sem bloquear registro, edição ou correção dentro da retenção.
- A exclusão de rascunho (manual ou automático) NÃO impede recriação automática por nova despesa válida na mesma competência (RF-09d); a UI DEVE comunicar essa expectativa para evitar percepção de bug em ciclos delete → próxima despesa → novo rascunho.

## Restrições Técnicas de Alto Nível

- **RT-01**: A funcionalidade deve respeitar o monólito modular e as fronteiras definidas em `AGENTS.md`.
- **RT-02**: Postgres, outbox, workers e observabilidade existentes devem ser reutilizados; nenhuma infraestrutura gerenciada nova faz parte do MVP.
- **RT-03**: Integridade financeira e idempotência são não negociáveis; alertas são auxiliares.
- **RT-04**: Valores monetários usam centavos inteiros BRL; percentuais usam basis points; cálculos usam half-even.
- **RT-05**: Categorias aceitas por budgets são fornecidas por `internal/categories`; budgets opera somente sobre as cinco raízes oficiais de despesa e suas subcategorias `kind=expense`.
- **RT-06**: Dados financeiros são isolados por usuário e tratados como sensíveis ao negócio.
- **RT-07**: O núcleo financeiro deve buscar 99,9% de disponibilidade mensal e p95 de até 300 ms.
- **RT-08**: Dimensionamento inicial: até 10 mil usuários ativos, 100 despesas por usuário/mês e pico de 10 escritas/s.
- **RT-09**: RPO aceito de até 15 minutos e RTO de até 4 horas são riscos residuais explícitos.
- **RT-10**: Retenção de dados do módulo é de 24 meses.
- **RT-11**: Competências não fecham automaticamente dentro da retenção; correções retroativas são permitidas somente durante os 24 meses de retenção.
- **RT-12**: Liberação será geral e controlada após testes; não haverá canary ou feature flag no MVP.
- **RT-13**: Ausência de histórico de valores anteriores de despesas editadas/excluídas é trade-off aprovado.
- **RT-14**: As cinco raízes oficiais de despesa (Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira) e suas subcategorias são fornecidas pela seed editorial de `internal/categories`; budgets resolve os IDs em runtime e DEVE poder cachear conforme a versão editorial exposta pelo catálogo.
- **RT-15**: O contrato versionado de eventos deve ser fechado antes da implementação.
- **RT-16**: O MVP não aceita produtores externos nem publicação pública de eventos de despesas.
- **RT-17**: Timestamps técnicos DEVEM ser persistidos em UTC; regras mensais de negócio DEVEM usar `America/Sao_Paulo`.
- **RT-18**: O módulo budgets DEPENDE em runtime do módulo `internal/categories` para leitura de raízes e subcategorias; indisponibilidade de leitura categorial DEVE degradar somente operações de validação de novas despesas/alocações e novas ativações, sem afetar consultas, resumos e alertas baseados em dados já persistidos.
- **RT-19**: A ordem determinística de distribuição de centavos residuais (RF-11a) DEVE referenciar os slugs editoriais imutáveis das raízes oficiais, resolvidos para PKs via `internal/categories`, e NÃO DEVE depender de nomes textuais, ordem de seed nem PKs diretas.
- **RT-20**: Endpoints HTTP de budgets DEVEM viver sob o prefixo `/v1/budgets`; o contrato externo expõe `subcategory_id` em despesas e `category_id` (raiz) em alocações.
- **RT-21**: O cutoff de competência corrente para supressão de alerta retroativo (RF-60c) DEVE ser computado em `America/Sao_Paulo` no instante do commit da mutação financeira, não no instante da avaliação assíncrona do alerta.
- **RT-22**: A identidade canônica de despesa via API DEVE usar `source="api"` fixado pelo servidor e `external_transaction_id` (conforme RT-26) fornecido pelo cliente no body; não há header Idempotency-Key separado no MVP.
- **RT-23**: O módulo `internal/categories` é pré-requisito de produção de budgets e DEVE estar disponível em runtime antes da liberação do MVP, expondo, no mínimo: (a) leitura das cinco raízes oficiais por slug imutável retornando `category_id`, slug, nome e versão editorial corrente; (b) validação de `subcategory_id` confirmando `kind=expense`, raiz pertencente ao conjunto oficial e estado `deprecated_at` (consulta aceita identificador descontinuado); (c) versão editorial do catálogo para invalidação de cache local em budgets. budgets NÃO DEVE ser liberado em produção sem `internal/categories` operacional cumprindo este contrato.
- **RT-24**: A avaliação de alertas (RF-55–RF-64) DEVE ser executada de forma assíncrona via outbox interno do módulo budgets e worker dedicado; a request HTTP ou consumo de evento que aplica a mutação financeira NÃO DEVE aguardar o resultado da avaliação. O instante de commit (`committed_at`) e a competência BR de cutoff (RT-21) DEVEM ser propagados pelo evento de outbox para o avaliador, garantindo correção de RF-60c independente do atraso de processamento.
- **RT-25**: Métricas operacionais de budgets DEVEM ter cardinalidade limitada; `user_id`, `external_transaction_id` e `subcategory_id` NÃO DEVEM compor labels de métricas. Granularidade aceita inclui módulo, raiz oficial (por slug), competência (`YYYY-MM`), estado de máquina, fonte (`source`) restrita à allowlist e limiar. Tracing pode reter atributos de alta cardinalidade conforme política de PII vigente.
- **RT-26**: `external_transaction_id` DEVE ser validado no boundary como UUID v4 canônico (lowercase com hyphens, 36 caracteres) OU ULID canônico (26 caracteres Crockford base32 uppercase). Identificadores em outros formatos DEVEM ser rejeitados antes do commit, sem normalização pelo servidor; identidade canônica é case-sensitive para evitar colisão entre formatos.
- **RT-27**: A chave canônica de competência DEVE ser uma string `YYYY-MM` (ISO 8601 truncado), com o mês computado em `America/Sao_Paulo` conforme RT-17. Persistência interna PODE usar tipo equivalente desde que a serialização externa preserve o formato `YYYY-MM`.
- **RT-28**: A allowlist de produtores internos autorizados (RF-32a/RF-72) DEVE ser definida como constante Go versionada em `internal/budgets/infrastructure/config/producers.go`, exportando o conjunto canônico de `source` aceitos. Mudanças DEVEM ocorrer via PR revisado + deploy; budgets NÃO DEVE expor mutação runtime nem leitura de arquivo/env para essa lista.
- **RT-29**: O resumo mensal (RF-48–RF-54) DEVE ser calculado on-demand por agregação SQL sobre `budgets_expenses` filtrada por `(user_id, competence)` e agrupada por raiz oficial. A tabela DEVE possuir índice composto `(user_id, competence, subcategory_id)` com cláusula `WHERE deleted_at IS NULL` para suportar o p95 ≤ 300 ms (M-05/RT-07). Acumulado persistido permanece proibido (RF-54).
- **RT-30**: As tarefas agendadas RF-18b (varredura diária de rascunhos abandonados às 03:00 BR) e RF-66 (expurgo mensal de retenção) DEVEM ser implementadas como handlers em `internal/budgets/infrastructure/jobs/handlers/`, disparados pelo scheduler in-process já adotado por `internal/identity` e `internal/billing`. NÃO DEVE haver introdução de cron externo, `pg_cron` ou serviço dedicado.
- **RT-31**: O cache local do contrato de `internal/categories` (RT-14/RT-23) em budgets DEVE resolver as cinco raízes oficiais uma única vez no boot do processo, mantendo `category_id ↔ slug` em memória pelo lifetime do binário (raízes são imutáveis por contrato). Subcategorias DEVEM ser cacheadas com TTL máximo de 60 segundos e bust explícito quando a `editorial_version` exposta por `internal/categories` mudar. Falha ao resolver raízes no boot DEVE impedir o startup do módulo.

## Fora de Escopo

- **OUT-01**: Envio real de alertas por WhatsApp.
- **OUT-02**: Implementação ou integração com agente LLM.
- **OUT-03**: Categorias personalizadas, removíveis ou criadas pelo usuário dentro de budgets.
- **OUT-04**: Classificação automática de despesas.
- **OUT-05**: Despesas novas sem categoria.
- **OUT-06**: Histórico/auditoria dos valores anteriores de despesas editadas ou excluídas.
- **OUT-07**: Event sourcing, ledger imutável ou projeções financeiras assíncronas.
- **OUT-08**: Acumulados persistidos como fonte de verdade.
- **OUT-09**: Cache distribuído, broker externo ou serviço dedicado de budgets.
- **OUT-10**: Multi-moeda.
- **OUT-11**: Bloqueio de despesa por estouro de orçamento.
- **OUT-12**: Fechamento automático ou manual de competência.
- **OUT-13**: Planejamento anual além da recorrência limitada a 12 meses.
- **OUT-14**: Front-end gráfico dedicado; o PRD define contratos e comportamento de produto.
- **OUT-15**: Arquivamento consultável separado após 24 meses.
- **OUT-16**: Endpoints HTTP administrativos para inspeção de eventos pendentes (RF-39b) e de estados auxiliares de alertas (RF-64c). No MVP, inspeção é feita exclusivamente via SQL e dashboards de observabilidade; introdução de endpoints administrativos depende de roles/claims no E1 e fica para pós-MVP.

## Suposições e Questões em Aberto

- **QA-04 — Recuperação**: comprovar capacidade real de backup/restauração para RPO de 15 minutos e RTO de 4 horas.
- **QA-05 — Alertas futuros**: definir o contrato do provider de agente LLM/WhatsApp sem adicioná-lo ao MVP.

## Decisões Fechadas (v24)

| Decisão | Resultado |
| --- | --- |
| Catálogo de despesa | Fornecido por `internal/categories`; budgets opera sobre 5 raízes oficiais e suas subcategorias `kind=expense` |
| Identificação das raízes | Slugs editoriais imutáveis: `expense.custo_fixo`, `expense.conhecimento`, `expense.prazeres`, `expense.metas`, `expense.liberdade_financeira` |
| FK em despesa | `subcategory_id` (subcategoria); agregação do resumo por raiz |
| Validação na ingestão | FK + `kind=expense` + raiz ∈ {5 oficiais}; aceita `deprecated_at` para preservar histórico |
| Resumo mensal | Somente por raiz; subcategoria é dado interno |
| Estouro de orçamento | Permitido; percentual utilizado pode ultrapassar 100% sem bloqueio, rejeição, reversão ou truncamento |
| Auth API | JWT do E1; `user_id` derivado do token; nunca no payload |
| Auth produtores eventos | Allowlist estática em configuração revisada do módulo |
| Identidade API | `external_transaction_id` (UUID v4 ou ULID canônicos, RT-26) no body; `source="api"` fixo pelo servidor |
| Versão monotônica | Criação força `v=1` (servidor); edição/exclusão exigem versão esperada; exclusão grava `tombstone_version` |
| Recorrência | `source_competence` explícita com soma de alocações = 100% (ativado ou rascunho válido); aplica a até 12 competências futuras ainda não ativadas |
| Ciclo do orçamento | Rascunho é exclusível pelo usuário; ativado é imutável até expurgo de 24m |
| Avaliação de alerta | Assíncrona via outbox interno do módulo + worker; commit financeiro não aguarda |
| Exposição de alertas | Tabela `budgets_alerts` + `GET /v1/budgets/alerts` (cursor); MVP sem `delivery_failed` |
| Eventos pendentes | Tabela `budgets_expense_events_pending` com `pending → applied | failed | expired` (failed = erro permanente; expired = timeout 24h) + endpoint admin |
| Cutoff retroativo | Competência BR no commit (não na avaliação assíncrona) |
| Reuso de tombstone | Liberado após expurgo de 24 meses; tombstone tem retenção independente dos demais dados |
| Sinal de rascunho abandonado | Cronjob diário 03:00 BR, métrica + log, sem notificação ao usuário |
| Subcategoria descontinuada | Despesa histórica mantém referência e exibe caminho completo |
| Chave de competência | `YYYY-MM` (RT-27), mês computado em America/Sao_Paulo |
| Cardinalidade de métricas | `user_id`, `external_transaction_id` e `subcategory_id` proibidos como labels |
| Endpoints admin HTTP | Fora do MVP (OUT-16); inspeção via SQL + Grafana até roles no E1 |
| Estado de cruzamento de limiar | Tabela `budgets_threshold_state` com versão monotônica, atualizada apenas pelo avaliador assíncrono (RF-60e/RF-60f) |
| Allowlist de produtores | Constante Go versionada em `internal/budgets/infrastructure/config/producers.go` (RT-28) |
| Resumo mensal | Agregação SQL on-demand com índice composto `(user_id, competence, subcategory_id) WHERE deleted_at IS NULL` (RT-29) |
| Scheduler de jobs | Handlers em `internal/budgets/infrastructure/jobs/handlers/` no scheduler in-process (RT-30) |
| Cache de categories | Raízes resolvidas 1x no boot; subcategorias com TTL 60s + bust por `editorial_version` (RT-31) |

## Critério de Prontidão para Especificação Técnica

O PRD está pronto para especificação técnica. A especificação DEVE coordenar com a techspec de `internal/categories` para fechar o contrato de leitura consumido (RT-23) — resolução de slugs para IDs, validação de subcategoria (`kind=expense`, raiz oficial, `deprecated_at` aceito) e versão editorial para invalidação de cache — e DEVE detalhar o esquema das tabelas `budgets_alerts`, `budgets_expense_events_pending` e `budgets_threshold_state` (RF-60e/RF-60f), o evento de outbox interno que aciona a avaliação assíncrona de alerta (RT-24), o ciclo de vida de `tombstone_version`, o índice composto exigido por RT-29, os handlers de scheduler (RT-30), a estratégia de cache de categories (RT-31) e a estratégia de cardinalidade de métricas (RT-25). A entrega de budgets em produção DEPENDE da entrega prévia de `internal/categories` cumprindo RT-23. QA-04 deve ser comprovada antes da liberação produtiva. QA-05 não bloqueia o MVP.
