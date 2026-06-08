# Transcript do Discovery Técnico

## Contexto Inicial

- Tema: discovery técnico production-ready do módulo de orçamentos mensais por categoria.
- Direção decisória aprovada: registro canônico reconciliável com atualização transacional.
- Objetivo: estado financeiro atual correto e reconciliável, entradas idempotentes por API e evento, alertas persistentes e consulta imediatamente consistente.
- Escopo aprovado: orçamento único por usuário/mês, percentuais até 100%, orçamento imutável após ativação, categoria obrigatória e criação/edição/exclusão física de despesas.
- Trade-off aprovado: não haverá histórico de edições/exclusões; a reconciliação cobre somente o estado atual.
- Integração real com agente LLM/WhatsApp está fora do MVP; budgets deve preparar contrato/provider e alertas persistentes.

### Materiais de Apoio Identificados

- `docs/discoveries/brainstorms/brainstorm-modulo-de-orcamentos-mensais-por-categoria/`: bundle decisório validado com `SUCCESS`.
- `docs/discoveries/brainstorms/brainstorm-crud-de-categorias-e-subcategorias-de-despesas/`: decisão do futuro módulo de categorias e contrato pretendido para budgets.
- `AGENTS.md`: regras canônicas de arquitetura, DDD, outbox, workers, HTTP e validação.
- `go.mod`: monólito modular Go 1.26.4 com Chi, pgx/Postgres, OpenTelemetry e bibliotecas de teste.
- `internal/platform/outbox`: capacidade existente para side-effects resilientes.
- `internal/platform/worker`: capacidade existente para jobs e consumers.
- `internal/billing` e `internal/identity`: referências reais de composição, persistência, eventos e observabilidade.

### Evidências e Lacunas

- Não existe módulo `internal/budgets`, `internal/categories` ou `internal/transactions` implementado no working tree.
- A decisão de categorias é material de planejamento, não contrato executável existente.
- Não foi encontrada integração pronta de WhatsApp/agente LLM para alertas de budgets.
- Volumetria, SLO, RTO/RPO e guardrail financeiro ainda não foram definidos.

## Classificação da Demanda e Materiais

### Perguntas

**P1. Qual classificação principal deve orientar este discovery?**

- A. Nova capacidade de negócio: desenhar budgets como novo bounded context, aceitando custo inicial de fundação.
- B. Extensão de um módulo existente: incorporar budgets em outro bounded context para reduzir prazo, aceitando maior acoplamento.
- C. Experimento técnico: validar apenas consistência/idempotência antes de assumir entrega funcional completa.

**P2. Como os materiais identificados devem ser tratados?**

- A. Documentação robusta e suficiente para avançar: decisões existentes são mandatórias, e lacunas serão fechadas nas rodadas.
- B. Documentação parcial: decisões existentes orientam, mas podem ser revisadas se o código ou riscos técnicos contradisserem.
- C. Apenas contexto preliminar: reavaliar inclusive decisões do brainstorming, aumentando prazo e espaço de solução.

### Respostas

- P1: A. A iniciativa é uma nova capacidade de negócio e deve ser desenhada como novo bounded context `budgets`.
- P2: B. Os materiais existentes são parciais: orientam a solução, mas decisões podem ser revistas quando riscos técnicos ou evidências do código exigirem.

### Síntese

- `budgets` terá fronteira própria, seguindo o padrão modular obrigatório do repositório.
- O bundle decisório é a principal entrada, mas não substitui decisões de segurança, operação, volumetria e custo.
- O futuro módulo `categories` é dependência de contrato ainda não implementada.
- Capacidades existentes de outbox, workers, Postgres e observabilidade devem ser reutilizadas.

## Rodada 1 - Objetivo, Escopo e Criticidade

### Perguntas

**P1. Qual objetivo técnico deve prevalecer quando houver tensão entre prazo e robustez?**

- A. Integridade financeira acima do prazo: aceitar entrega mais lenta para impedir duplicidade e divergência não detectada.
- B. Equilíbrio: garantir idempotência e consistência essenciais, adiando reconciliação automatizada e hardening operacional.
- C. Velocidade: entregar fluxo funcional primeiro e corrigir divergências manualmente; reduz prazo, mas contradiz o risco inaceitável definido.

**P2. Qual criticidade operacional deve ser assumida para budgets no MVP?**

- A. Alta: indisponibilidade ou total incorreto é incidente crítico, exigindo SLO forte e resposta operacional rápida.
- B. Média: total incorreto é crítico, mas indisponibilidade temporária é tolerável; prioriza integridade sobre disponibilidade.
- C. Baixa: atrasos e divergências temporárias são toleráveis; reduz custo, mas conflita com consistência imediata.

**P3. Qual recorte deve compor a primeira entrega operacional?**

- A. Núcleo completo aprovado: orçamento, despesas por API/evento, consultas, reconciliação e alertas persistentes.
- B. Núcleo financeiro primeiro: orçamento, despesas por API/evento, consultas e reconciliação; alertas persistentes entram em fase posterior.
- C. API primeiro: orçamento, despesas via API e consultas; entrada por evento, reconciliação e alertas entram depois.
- D. Escrita primeiro: criação do orçamento e despesas; consultas consolidadas e capacidades operacionais entram depois.

**P4. Qual restrição domina a solução?**

- A. Capacidade do time: minimizar novos componentes e operação, reutilizando Postgres, outbox e workers existentes.
- B. Prazo: entregar rapidamente mesmo com mais dívida técnica explícita.
- C. Custo de infraestrutura: evitar qualquer capacidade que aumente consumo operacional.
- D. Dependências futuras: desenhar primeiro contratos de categories e agente LLM, mesmo que atrase budgets.

### Respostas

- P1: A. Integridade financeira prevalece sobre prazo; entrega mais lenta é aceitável para impedir duplicidade e divergência não detectada.
- P2: A. Criticidade alta; indisponibilidade e total incorreto são incidentes críticos.
- P3: A. A primeira entrega deve conter o núcleo completo: orçamento, despesas por API/evento, consultas, reconciliação e alertas persistentes.
- P4: A. A restrição dominante é a capacidade do time; a solução deve minimizar novos componentes e reutilizar Postgres, outbox e workers existentes.

### Síntese

- O MVP possui escopo funcional amplo, mas não pode reduzir garantias financeiras para acelerar prazo.
- A solução precisa operar com disciplina de alta criticidade mesmo sendo parte de um monólito modular.
- Postgres deve concentrar garantias transacionais; outbox e workers existentes devem atender alertas e reconciliação.
- Novas infraestruturas ou serviços dedicados exigem justificativa por limite comprovado, não por antecipação.

## Rodada 2 - Arquitetura, Dados, Volumetria e Custo

### Perguntas

**P1. Qual estratégia arquitetural deve orientar a primeira entrega?**

- A. Bounded context `internal/budgets` no monólito modular, compartilhando Postgres e plataforma existente; menor custo operacional e aderência ao repositório.
- B. Bounded context no monólito, mas com banco/schema isolado desde o início; melhora isolamento, porém aumenta migração e operação.
- C. Serviço dedicado de budgets desde o MVP; oferece autonomia, mas contradiz a restrição de capacidade do time sem evidência de escala.

**P2. Qual contrato deve ser a autoridade canônica para deduplicar API e evento?**

- A. Toda origem fornece `external_transaction_id` estável e único por usuário/origem; maior confiabilidade, mas exige contrato rígido dos produtores.
- B. Budgets gera ID na API e eventos devem referenciar esse ID; simplifica autoridade, mas eventos externos sem chamada prévia não entram diretamente.
- C. Budgets calcula fingerprint de usuário, categoria, valor e instante; reduz exigência dos produtores, mas pode colidir ou não detectar duplicatas reais.
- D. API e evento são autoridades separadas e nunca representam a mesma despesa; simplifica deduplicação, mas depende de premissa operacional forte.

**P3. Qual perfil de volumetria deve ser usado para dimensionar o MVP?**

- A. Baixo: até 10 mil usuários ativos, até 100 despesas/usuário/mês e pico de 10 escritas/s.
- B. Médio: até 100 mil usuários ativos, até 300 despesas/usuário/mês e pico de 100 escritas/s.
- C. Alto: até 1 milhão de usuários ativos, até 500 despesas/usuário/mês e pico de 1.000 escritas/s.
- D. Sazonal imprevisível: volume médio baixo, mas picos acima de 1.000 escritas/s; exige fila e absorção de rajadas desde o MVP.

**P4. Qual guardrail de custo deve orientar o discovery?**

- A. Sem nova infraestrutura gerenciada: usar Postgres, outbox, workers e observabilidade existentes; custo incremental mínimo.
- B. Custo incremental moderado permitido quando reduzir risco operacional, como réplica, capacidade adicional ou retenção de telemetria.
- C. Robustez acima do custo: autorizar novas capacidades gerenciadas quando melhorarem SLA/SLO.
- D. Limite financeiro mensal explícito deve ser definido antes de continuar; sem ele, não consolidar solução.

### Respostas

- P1: A. Criar `internal/budgets` no monólito modular, compartilhando Postgres e capacidades existentes da plataforma.
- P2: A. Toda origem deve fornecer `external_transaction_id` estável e único por usuário/origem.
- P3: A. Dimensionar para até 10 mil usuários ativos, até 100 despesas por usuário/mês e pico de 10 escritas por segundo.
- P4: A. Não adicionar infraestrutura gerenciada; usar Postgres, outbox, workers e observabilidade existentes, com custo incremental mínimo.

### Síntese

- O desenho deve permanecer dentro do monólito modular e usar uma única fronteira transacional Postgres.
- A identidade canônica de entrada será baseada em usuário, origem e `external_transaction_id`; entradas sem identificador válido devem ser rejeitadas.
- O volume selecionado não justifica serviço dedicado, stream externo, cache distribuído ou read-model assíncrono.
- O principal gargalo provável não é throughput global, mas contenção concorrente por orçamento/categoria e crescimento de alertas/reconciliações.
- O custo incremental deve vir principalmente de armazenamento, processamento do worker e telemetria já existentes.

## Rodada 3 - Segurança, Confiabilidade e Operação

### Perguntas

**P1. Qual baseline de segurança deve ser aplicado aos dados financeiros?**

- A. Reforçado: isolamento estrito por usuário, autorização em todo use case, mascaramento de payloads sensíveis e auditoria operacional mínima sem valores anteriores.
- B. Padrão: autenticação/autorização e criptografia da plataforma, sem controles adicionais; menor esforço, mas investigação limitada.
- C. Máximo: segregação adicional, trilha completa e retenção ampliada; conflita com a decisão de não manter histórico e aumenta custo.

**P2. Como o sistema deve degradar quando componentes auxiliares falharem?**

- A. Núcleo financeiro continua; alertas e reconciliações permanecem persistidos para retry, e consultas servem o último estado transacional confirmado.
- B. Bloquear novas despesas enquanto alertas ou reconciliação estiverem indisponíveis; reduz divergência operacional, mas aumenta indisponibilidade.
- C. Aceitar despesas em fila para processamento posterior; melhora disponibilidade, mas viola consistência imediata.

**P3. Qual profundidade de observabilidade é obrigatória no MVP?**

- A. Telemetria completa: métricas, logs estruturados, traces, alertas e runbook para duplicidade, divergência, fila de alertas e latência.
- B. Métricas e logs estruturados: menor custo operacional, mas diagnóstico de concorrência e fluxo cross-entry fica mais lento.
- C. Logs apenas: menor esforço, incompatível com criticidade alta.

**P4. Qual estratégia de rollout e rollback deve ser adotada?**

- A. Feature flag e piloto fechado por usuários internos; migrations aditivas e rollback desliga entradas sem apagar dados.
- B. Canary percentual automático; maior sofisticação operacional para baixa volumetria.
- C. Liberação geral controlada após testes; menor complexidade, mas maior raio de impacto.
- D. Big-bang com rollback de migration; rápido, mas arriscado para dados financeiros.

### Respostas

- P1: A. Aplicar baseline reforçado com isolamento estrito por usuário, autorização em todo use case, mascaramento e auditoria operacional mínima.
- P2: A. O núcleo financeiro continua quando alertas ou reconciliação falham. O usuário explicitou que não alertar é aceitável; o valor correto é prioritário.
- P3: A. O MVP exige telemetria completa com métricas, logs estruturados, traces, alertas e runbook.
- P4: C. Fazer liberação geral controlada após testes, sem feature flag ou canary.

### Síntese

- Alertas são best-effort operacional: falhas não afetam despesas e não constituem perda financeira, mas devem ser observáveis.
- Reconciliação é auxiliar para detectar/corrigir divergência; a transação principal deve manter o valor correto sem depender dela.
- Baseline reforçado precisa compensar parcialmente a ausência de histórico financeiro, sem reintroduzir valores anteriores.
- Liberação geral controlada reduz complexidade, mas aumenta o raio de impacto; rollback deverá desligar entradas por configuração/deploy e preservar dados/migrations aditivas.

## Rodada 4 - SLO, Reconciliação e Semântica Financeira

### Perguntas

**P1. Qual SLO deve orientar operações críticas do núcleo financeiro?**

- A. p95 até 300 ms e 99,9% de disponibilidade mensal para escrita/consulta; exige disciplina operacional compatível com criticidade alta.
- B. p95 até 1 s e 99,5% de disponibilidade mensal; reduz custo e pressão operacional, aceitando maior indisponibilidade.
- C. Sem SLO formal no MVP; medir primeiro e definir depois, incompatível com a criticidade alta escolhida.

**P2. Qual RPO/RTO deve ser assumido para dados financeiros?**

- A. RPO próximo de zero para operações confirmadas e RTO até 1 hora; exige backup/restauração validados e transações duráveis.
- B. RPO até 15 minutos e RTO até 4 horas; menor exigência operacional, aceitando perda potencial de despesas confirmadas.
- C. Herdar metas gerais da plataforma sem requisito específico; simplifica, mas pode não proteger o domínio financeiro.

**P3. Como a reconciliação deve operar na primeira entrega?**

- A. Recálculo periódico completo do mês corrente por usuário/orçamento e correção automática transacional quando houver divergência.
- B. Recálculo periódico que detecta e alerta, mas exige correção manual; reduz risco de correção indevida, porém aumenta operação humana.
- C. Recálculo sob demanda por API/job administrativo; menor custo, mas divergências podem permanecer invisíveis.
- D. Sem acumulado persistido: consultas sempre somam despesas existentes; elimina divergência de projeção, mas pode aumentar custo e latência.

**P4. Como limiares de alerta devem evitar mensagens repetidas após edições e exclusões?**

- A. Alertar uma vez por limiar cruzado em cada direção; se o gasto cair abaixo e cruzar novamente, novo alerta é permitido.
- B. Alertar uma vez por limiar durante todo o mês, mesmo após cair e cruzar novamente; menor ruído, mas pode omitir novo desvio.
- C. Criar alerta em toda alteração enquanto estiver acima do limiar; simples, mas gera ruído e volume operacional.
- D. Persistir somente estado do alerta, sem regra fechada agora; bloqueia desenho confiável do processamento.

### Respostas

- P1: A. Operações críticas devem buscar p95 até 300 ms e 99,9% de disponibilidade mensal.
- P2: B. Assumir RPO até 15 minutos e RTO até 4 horas.
- P3: D. Não persistir acumulados; consultas devem somar as despesas canônicas existentes. O usuário reforçou que orçamentos e metas financeiras são mensais.
- P4: C. Criar alerta em toda alteração enquanto o gasto permanecer acima do limiar.

### Síntese

- A fonte de verdade financeira será o conjunto atual de despesas canônicas, sem projeção acumulada persistida.
- A reconciliação deixa de comparar acumulado versus despesas; passa a concentrar-se em integridade referencial, idempotência e detecção de registros inválidos.
- O recorte mensal limita o conjunto somado, tornando a leitura calculada viável para a volumetria baixa selecionada, desde que existam índices adequados.
- O RPO de até 15 minutos aceita perda potencial de operações confirmadas em desastre, apesar da criticidade alta; este é um risco residual explícito.
- Alertar em toda alteração acima do limiar aumenta ruído e custo futuro de mensagens, mas foi escolhido explicitamente.

## Rodada 5 - Tempo, Valores, Categorias e Alertas

### Perguntas

**P1. Qual data determina a qual orçamento mensal uma despesa pertence?**

- A. Data de competência informada pela origem; representa quando o gasto ocorreu, mas exige validação de alterações retroativas.
- B. Data de criação no sistema; simplifica consistência, mas classifica incorretamente gastos lançados com atraso.
- C. Data de competência quando informada, senão criação; melhora flexibilidade, mas cria duas regras de atribuição.

**P2. Como tratar alteração da data ou categoria de uma despesa para outro mês/orçamento?**

- A. Permitir e recalcular ambos os meses na mesma transação; mantém estado atual correto, mas pode alterar orçamento passado.
- B. Bloquear mudança entre meses/categorias após criação; reduz complexidade, mas limita edição aprovada.
- C. Permitir apenas no mês corrente; protege períodos anteriores, mas exige conceito de fechamento mensal.

**P3. Qual representação monetária e percentual deve ser mandatória?**

- A. Valores em centavos inteiros de BRL e percentuais em basis points; evita arredondamento binário e suporta duas casas percentuais.
- B. Decimal no banco para dinheiro e percentual; flexível, mas exige regra explícita de escala e arredondamento em todas as fronteiras.
- C. Float para valores e percentuais; simples, mas inadequado para integridade financeira.

**P4. Considerando alerta em toda alteração acima do limiar, como evitar duplicação causada por retry da mesma entrada?**

- A. Alerta idempotente por despesa, versão da alteração e limiar; retries da mesma operação não criam novo alerta.
- B. Alerta idempotente somente por despesa e limiar no mês; reduz mensagens, mas pode omitir alterações posteriores acima do limite.
- C. Não deduplicar alertas; toda tentativa pode gerar alerta, aumentando ruído e custo futuro.

### Respostas

- P1: A. A data de competência informada pela origem determina o orçamento mensal. Cada orçamento pode ser lançado manualmente ou criado por recorrência limitada a 12 meses futuros, gerando registros mensais independentes e únicos por usuário e competência.
- P2: A. Permitir alteração de data ou categoria e refletir ambos os meses/orçamentos afetados na mesma transação lógica.
- P3: A. Representar valores em centavos inteiros de BRL e percentuais em basis points, com arredondamento half-even bancário.
- P4: B. Deduplicar alertas por despesa e limiar no mês; alterações posteriores da mesma despesa acima do mesmo limiar não geram novo alerta.

### Síntese

- Orçamento é entidade mensal independente, única por usuário e competência, mesmo quando originada por recorrência.
- Recorrência é uma operação de criação antecipada de até 12 orçamentos; não deve introduzir leitura implícita de série para garantir previsibilidade e unicidade.
- Alterações de competência/categoria exigem que a leitura de ambos os períodos reflita imediatamente o novo estado; como totais são calculados, a transação altera somente a despesa canônica.
- Dinheiro e percentuais não usarão ponto flutuante. O cálculo do valor planejado por categoria deve aplicar half-even e tratar sobras de centavos explicitamente.
- Alertas são deduplicados por despesa, limiar e mês, reduzindo retries e alterações repetidas da mesma despesa.

## Rodada 6 - Recorrência, Arredondamento e Dependência de Categorias

### Perguntas

**P1. Se a criação recorrente encontrar um mês que já possui orçamento para o usuário, qual comportamento deve prevalecer?**

- A. Falhar toda a operação atomicamente e não criar nenhum mês; máxima previsibilidade, mas exige correção antes de tentar novamente.
- B. Criar somente meses ausentes e retornar os conflitos; operação idempotente e prática, mas o resultado pode ser parcial.
- C. Sobrescrever meses existentes ainda não ativados; simplifica atualização, mas pode apagar configuração manual.

**P2. Após criar os orçamentos recorrentes, alterações na recorrência devem afetar quais meses?**

- A. Nenhum automaticamente: cada mês criado é independente; alterações futuras exigem nova operação explícita.
- B. Apenas meses futuros ainda não ativados; facilita manutenção, mas exige rastrear série e estado.
- C. Todos os meses futuros, inclusive ativados; maior automação, mas contradiz imutabilidade após ativação.

**P3. Como tratar centavos não distribuídos após aplicar percentuais e half-even?**

- A. Manter a sobra como valor não alocado do orçamento; soma planejada das categorias pode ficar abaixo do total e permanece explicável.
- B. Distribuir a sobra deterministicamente entre categorias; fecha exatamente o total, mas altera ligeiramente os percentuais efetivos.
- C. Rejeitar qualquer configuração que produza sobra; garante igualdade exata, mas torna configurações comuns difíceis.

**P4. O que ocorre se uma categoria referenciada pelo orçamento for alterada ou removida no módulo categories?**

- A. Budgets mantém referência e snapshot mínimo do nome; orçamentos existentes continuam legíveis, e novos usos dependem de categoria ativa.
- B. Bloquear alteração/remoção no módulo categories enquanto houver orçamento; preserva referência, mas cria forte acoplamento entre módulos.
- C. Remover automaticamente a categoria do orçamento; mantém somente categorias ativas, mas corrompe o planejamento histórico mensal.

### Respostas

- P1: B. Criar somente os meses ausentes e retornar conflitos para competências que já possuem orçamento.
- P2: B. Alterações na recorrência devem atualizar apenas meses futuros ainda não ativados.
- P3: B. Distribuir deterministicamente os centavos restantes entre categorias.
- P4: C. Remover automaticamente a categoria do orçamento quando ela for alterada ou removida no módulo categories.

### Síntese

- A recorrência precisa de identidade própria para localizar e atualizar somente orçamentos futuros não ativados pertencentes à série.
- A criação recorrente será parcialmente bem-sucedida e idempotente, retornando competências criadas e competências em conflito.
- A distribuição de centavos deve usar ordem determinística estável para que o mesmo orçamento sempre produza os mesmos valores planejados.
- Remover automaticamente categoria conflita com orçamento imutável após ativação, pode deixar despesas canônicas sem categoria orçamentária e pode reduzir a soma percentual sem decisão explícita sobre redistribuição.
- A decisão do brainstorming de categories prevê soft-delete preservando referências downstream; remoção automática em budgets contradiz esse material e exige clarificação.

## Rodada 7 - Integridade entre Categories, Budgets e Despesas

### Perguntas

**P1. A remoção automática de categoria deve afetar orçamentos já ativados?**

- A. Não: orçamentos ativados preservam a alocação e snapshot; remoção afeta somente orçamentos futuros não ativados.
- B. Sim: remover também de orçamentos ativados; mantém alinhamento com categories, mas viola imutabilidade e altera planejamento passado.
- C. Marcar alocação como categoria removida sem excluí-la; preserva valores e referência, mas não representa remoção automática literal.

**P2. O que fazer com despesas existentes vinculadas à categoria removida?**

- A. Preservar a referência e continuar somando na alocação histórica/snapshot; mantém total correto e legibilidade.
- B. Torná-las sem categoria; preserva valor total, mas quebra consultas por categoria e requisito de categoria obrigatória.
- C. Exigir reclassificação automática para outra categoria; mantém classificação, mas precisa de regra ou intervenção não definida.

**P3. Ao remover uma categoria de orçamento futuro não ativado, como tratar seu percentual?**

- A. Tornar o percentual não alocado; mantém as demais metas intactas e soma fica abaixo de 100%.
- B. Redistribuir proporcionalmente entre categorias restantes; fecha 100%, mas altera metas sem escolha explícita do usuário.
- C. Bloquear ativação até o usuário redistribuir; preserva intenção, mas adiciona etapa operacional.

**P4. Como budgets deve receber mudanças do módulo categories?**

- A. Consumir eventos `categories.v1.*` via outbox/consumer idempotente; desacoplado e resiliente, com consistência eventual apenas para metadados.
- B. Categories chama budgets sincronamente; efeito imediato, mas cria acoplamento e falha distribuída entre módulos.
- C. Budgets consulta categories em toda leitura; evita sincronização local, mas torna consultas financeiras dependentes de outro módulo.

### Respostas

- P1: A. Remoções de categoria afetam somente orçamentos futuros não ativados; orçamentos ativados preservam sua alocação.
- P2: B. Despesas existentes vinculadas à categoria removida tornam-se sem categoria.
- P3: C. Orçamentos futuros afetados não podem ser ativados até o usuário redistribuir manualmente o percentual.
- P4: B, com ressalva explícita: o usuário deseja efeito síncrono sem chamada direta entre módulos, respeitando o monólito modular.

### Síntese

- Orçamentos ativados preservam planejamento e snapshot, protegendo a imutabilidade mensal.
- Orçamentos futuros não ativados removem a alocação afetada e entram em estado inválido até redistribuição.
- Tornar despesas existentes sem categoria conflita com a regra anterior de categoria obrigatória e impede atribuir seus valores a uma meta específica.
- A opção de integração síncrona escolhida conflita com a proibição explícita de chamada direta. No monólito modular, as opções coerentes são contrato síncrono explícito consumido por budgets ou evento assíncrono/outbox.

## Rodada 8 - Contrato Cross-Module e Despesas Órfãs

### Perguntas

**P1. Após uma categoria ser removida, onde despesas órfãs devem aparecer no resumo mensal?**

- A. Em grupo técnico `Sem categoria`, fora das metas percentuais, mas incluído no total gasto; mantém valor correto e torna a inconsistência visível.
- B. Somente no total geral, sem linha própria; reduz exposição, mas dificulta explicar a divergência entre categorias e total.
- C. Excluir das consultas do orçamento; simplifica categorias, mas torna o total financeiro incorreto e é incompatível com a prioridade definida.

**P2. Novas despesas podem ser criadas sem categoria após a remoção?**

- A. Não: somente despesas previamente categorizadas podem tornar-se órfãs por remoção; novas entradas continuam exigindo categoria ativa.
- B. Sim: permitir categoria opcional em qualquer entrada; flexibiliza operação, mas revisa o requisito de categoria obrigatória.
- C. Bloquear remoção se houver despesas; evita órfãs, mas contradiz a escolha anterior de torná-las sem categoria.

**P3. Qual integração respeita sua intenção de efeito imediato sem acoplamento direto entre implementações?**

- A. Budgets declara uma interface de consulta de categoria ativa e recebe adapter no wiring; valida sincronamente ao criar/editar, sem importar implementação de categories.
- B. Categories publica evento outbox e budgets consome; preserva máximo desacoplamento, mas remoções refletem com consistência eventual.
- C. Uma aplicação/orquestrador externo chama categories e budgets; efeito síncrono, mas cria coordenação e rollback entre operações.

**P4. Quem é responsável por tornar despesas órfãs quando a categoria é removida?**

- A. Budgets consome evento de remoção de categories e atualiza suas próprias despesas idempotentemente; cada módulo preserva ownership, com atraso eventual controlado.
- B. Categories chama um contrato exposto por budgets durante a remoção; efeito imediato, mas categories passa a coordenar estado de outro módulo.
- C. Um job de budgets detecta periodicamente categorias removidas; menor acoplamento, mas órfãs demoram mais para refletir.

### Respostas

- P1: A, com revisão da premissa: categorias usadas por budgets são fixas do sistema e sempre existirão; portanto, não haverá despesas órfãs no fluxo esperado.
- P2: A. Novas despesas continuam exigindo categoria válida.
- P3: A. Budgets declara uma interface de consulta/validação de categoria e recebe adapter no wiring, sem importar a implementação de categories.
- P4: não aplicável após revisão da premissa; categorias usadas por budgets não serão removidas.

### Síntese

- Despesas órfãs, grupo `Sem categoria` e processamento de remoção de categoria saem do escopo.
- Budgets aceita somente categorias fixas do sistema e valida a referência sincronamente por interface declarada pelo consumidor.
- O material anterior do futuro módulo categories prevê categorias personalizadas e removíveis, mas budgets não consumirá essas categorias no escopo atual.
- A dependência cross-module fica limitada a um contrato de leitura/validação, implementado por adapter no composition root.

## Rodada 9 - Idempotência de Exclusão, Ativação e Categorias Fixas

### Perguntas

**P1. Qual conjunto de categorias budgets deve aceitar?**

- A. Somente categorias fixas de orçamento mantidas pelo sistema; maior previsibilidade e nenhuma remoção, mas usuários não personalizam metas.
- B. Categorias fixas mais categorias personalizadas, desde que personalizadas não possam ser removidas quando usadas; amplia flexibilidade e acoplamento.
- C. Snapshot livre informado na criação do orçamento; elimina dependência de categories, mas perde taxonomia governada.

**P2. Como impedir que retry de evento recrie uma despesa excluída fisicamente?**

- A. Manter tombstone idempotente sem valores financeiros, contendo origem, `external_transaction_id`, operação e expiração; preserva exclusão física e impede replay.
- B. Permitir recriação após exclusão; simplifica armazenamento, mas torna retry capaz de alterar o total incorretamente.
- C. Proibir exclusão de despesas criadas por evento; reduz risco, mas cria comportamento diferente por origem.

**P3. Qual condição permite ativar um orçamento mensal?**

- A. Soma percentual pode ser menor ou igual a 100%; diferença permanece não alocada.
- B. Soma deve ser exatamente 100%; garante distribuição integral, mas impede orçamento parcialmente planejado.
- C. Qualquer soma até 100% pode ser salva como rascunho, mas ativação exige exatamente 100%; separa planejamento de execução com regra mais rígida.

**P4. Após ativação, quando o orçamento mensal deixa de aceitar novas despesas ou edições?**

- A. Nunca fecha automaticamente; competências passadas continuam editáveis, preservando correções retroativas.
- B. Fecha automaticamente no fim do mês; protege passado, mas impede correções tardias.
- C. Fecha manualmente por ação do usuário; oferece controle, mas adiciona estado e operação.
- D. Fecha após janela fixa no mês seguinte; equilibra correção tardia e estabilidade histórica.

### Respostas

- P1: A. Budgets aceita somente categorias fixas do sistema.
- P2: B. Após exclusão física, permitir recriação da despesa quando a mesma identidade externa reaparecer.
- P3: B. Ativação exige soma percentual exatamente igual a 100%.
- P4: A. Competências nunca fecham automaticamente; correções retroativas permanecem permitidas.

### Síntese

- O conjunto de categorias de orçamento é governado pelo sistema, fixo e não removível; budgets não depende de categorias personalizadas.
- Orçamento pode existir como rascunho, mas só é ativado quando 100% do valor estiver distribuído.
- Competências passadas permanecem mutáveis, portanto consultas e alertas podem mudar retroativamente.
- Permitir recriação após exclusão física conflita com idempotência: um retry atrasado da operação original pode ser indistinguível de uma intenção legítima de recriação e alterar o total incorretamente.

## Rodada 10 - Recriação, Retenção e Limites Operacionais

### Perguntas

**P1. Como distinguir recriação intencional de retry atrasado após exclusão física?**

- A. Recriação exige novo `external_transaction_id`; retries do ID excluído permanecem ignorados por tombstone idempotente.
- B. Recriação usa o mesmo ID, mas exige comando explícito via API; eventos repetidos permanecem ignorados.
- C. Aceitar o mesmo ID por API ou evento sem distinção; reduz regras, mas viola idempotência e pode corromper o total.

**P2. Por quanto tempo despesas e orçamentos mensais devem permanecer consultáveis?**

- A. Indefinidamente no MVP; simplifica produto, mas crescimento contínuo exige monitoramento de armazenamento e índices.
- B. Cinco anos; reduz crescimento, mas exige política de expurgo e impacto em histórico.
- C. Dois anos; menor custo, mas limita visão financeira de longo prazo.
- D. Arquivar após dois anos mantendo consulta separada; melhor equilíbrio futuro, porém adiciona infraestrutura e fluxo fora do guardrail.

**P3. Qual origem de evento de despesa deve existir no MVP?**

- A. Um único contrato canônico interno, publicado por futuros produtores; budgets não integra brokers externos específicos.
- B. Eventos de qualquer produtor que atendam ao contrato; flexível, mas exige governança e autenticação por origem.
- C. Uma integração externa específica já no MVP; reduz abstração, mas nenhuma integração foi informada.

**P4. Qual limite operacional aplicar aos alertas gerados em toda alteração acima do limiar?**

- A. Persistir todos, com limite máximo configurado por usuário/mês e descarte observável após o limite; controla crescimento.
- B. Persistir todos sem limite; preserva sinal completo, mas permite crescimento e custo sem guardrail.
- C. Rever regra para um alerta por limiar/mês; reduz volume, mas contradiz a decisão anterior.

### Respostas

- P1: A. Recriação exige novo `external_transaction_id`; a identidade excluída permanece protegida por tombstone idempotente.
- P2: C. Despesas e orçamentos permanecem consultáveis por dois anos.
- P3: A. O MVP expõe um contrato canônico interno de eventos para futuros produtores, sem integração externa específica.
- P4: A. Alertas persistidos possuem limite configurado por usuário/mês e descartes devem ser observáveis.

### Síntese

- A exclusão permanece física para os valores financeiros, mas um tombstone mínimo preserva a garantia de idempotência.
- A retenção de dois anos exige job de expurgo, regras para tombstones e proteção contra remover dados ainda necessários.
- Eventos entram por contrato canônico interno; autenticação da origem e validação do envelope permanecem necessárias.
- Alertas têm guardrail de cardinalidade e podem ser descartados após o limite sem comprometer o núcleo financeiro.

## Rodada 11 - Parâmetros Operacionais Finais

### Perguntas

**P1. Quais limiares de alerta devem existir por padrão no MVP?**

- A. 80% e 100% por categoria; oferece prevenção e estouro com baixa complexidade.
- B. Somente 100%; menor ruído, mas reduz controle preventivo.
- C. Lista livre configurada pelo usuário; maior flexibilidade, mas amplia validação e volume.

**P2. Qual limite padrão de alertas persistidos por usuário/mês deve ser adotado?**

- A. 100 alertas; suficiente para baixa volumetria e contém abuso/ruído.
- B. 1.000 alertas; preserva mais eventos, mas aumenta armazenamento e processamento futuro.
- C. 10 alertas; custo mínimo, mas pode descartar alertas legítimos rapidamente.

**P3. Como aplicar a retenção de dois anos e tombstones?**

- A. Expurgo mensal após 24 meses; tombstones permanecem pelo mesmo período contado da exclusão.
- B. Expurgo diário após 24 meses; maior precisão, porém mais execução operacional.
- C. Expurgar despesas/orçamentos após 24 meses, mas manter tombstones indefinidamente; máxima idempotência histórica com crescimento contínuo.

**P4. O que ocorre quando chega uma despesa para competência sem orçamento?**

- A. Rejeitar a entrada; preserva vínculo obrigatório, mas produtores precisam criar orçamento antes.
- B. Persistir a despesa sem orçamento e incluí-la quando o orçamento for criado; evita perda, mas introduz estado pendente.
- C. Criar orçamento rascunho automaticamente; melhora fluidez, mas inventa configuração sem percentuais.

### Respostas

- P1: A. Limiares padrão de alerta em 80% e 100% por categoria.
- P2: C. Limite padrão de 10 alertas persistidos por usuário/mês.
- P3: A. Expurgo mensal após 24 meses; tombstones permanecem por 24 meses contados da exclusão.
- P4: C. Quando uma despesa chega para competência sem orçamento, criar automaticamente um orçamento rascunho.

### Síntese

- O limite de 10 alertas contém ruído e custo, mas pode descartar alertas legítimos; descartes precisam de métrica e log.
- O expurgo mensal atende o guardrail de custo sem nova infraestrutura.
- O orçamento rascunho automático evita rejeitar despesas, porém não possui valor total nem distribuição válida até intervenção do usuário.
- Alertas percentuais não podem ser avaliados enquanto o rascunho não tiver valor total e 100% distribuído.

## Rodada 12 - Rascunho Automático e Interação com Recorrência

### Perguntas

**P1. Qual valor total deve ter o orçamento rascunho criado automaticamente?**

- A. Nenhum valor definido; o rascunho não calcula metas nem alertas até o usuário configurar e ativar.
- B. Usar o valor da primeira despesa; permite cálculo imediato, mas inventa orçamento sem intenção do usuário.
- C. Copiar o orçamento do mês anterior; melhora conveniência, mas pode aplicar metas antigas indevidamente.

**P2. Como despesas de rascunho automático aparecem nas consultas?**

- A. Mostrar total gasto e detalhamento por categoria, com planejado/percentual indisponíveis até ativação.
- B. Ocultar até o orçamento ser ativado; evita painel incompleto, mas esconde gastos reais.
- C. Mostrar como orçamento estourado; força ação, mas produz sinal financeiramente enganoso.

**P3. Se uma recorrência posterior tentar criar orçamento para um mês que já possui rascunho automático, o que fazer?**

- A. Preencher o rascunho existente com a configuração recorrente, preservando as despesas; reduz conflito e mantém unicidade.
- B. Tratar como conflito e exigir resolução manual; máxima previsibilidade, mas cria fricção.
- C. Criar outro orçamento; viola unicidade por usuário e competência.

**P4. Um orçamento rascunho automático pode receber novas despesas indefinidamente?**

- A. Sim; preserva todos os gastos, mas o usuário pode nunca configurar metas.
- B. Sim, mas gerar sinal operacional após prazo configurado sem ativação; preserva gastos e torna abandono observável.
- C. Não; bloquear após a primeira despesa até configuração, reduzindo utilidade e disponibilidade.

### Respostas

- P1: A. O orçamento rascunho automático não possui valor total definido e não calcula metas ou alertas até configuração e ativação.
- P2: A. Consultas do rascunho mostram gastos e detalhamento por categoria; planejado e percentual permanecem indisponíveis.
- P3: A. Recorrência posterior preenche o rascunho existente, preservando despesas e unicidade mensal.
- P4: B. O rascunho automático pode receber despesas, mas deve gerar sinal operacional após prazo configurado sem ativação.

### Síntese

- Despesas nunca são escondidas nem rejeitadas pela ausência de orçamento configurado.
- Rascunho automático é um estado explícito sem metas; somente orçamento ativado avalia alertas percentuais.
- Recorrência pode completar rascunhos automáticos, mas não sobrescreve orçamentos ativados.
- Rascunhos abandonados exigem observabilidade, sem bloquear novas despesas.

## Rodada 13 - Contratos de Mutação e Concorrência

### Perguntas

**P1. Quais mutações o contrato canônico de eventos deve suportar no MVP?**

- A. Criar, atualizar e excluir despesas; mantém paridade com a API, mas exige versionamento e idempotência em todas as operações.
- B. Somente criar despesas; edições e exclusões ficam na API, reduzindo complexidade dos produtores.
- C. Criar e excluir; atualização exige exclusão mais nova criação, aumentando operações e alertas.

**P2. Como resolver duas atualizações concorrentes da mesma despesa?**

- A. Controle otimista por versão esperada; conflito é rejeitado e o chamador deve reler, evitando perda silenciosa.
- B. Última escrita vence por instante de processamento; simples, mas pode perder alteração válida.
- C. Serializar por usuário em worker; reduz conflitos, mas viola consistência imediata da API e adiciona fila.

**P3. Qual prazo deve sinalizar orçamento rascunho automático não ativado?**

- A. 24 horas após criação; reação rápida, com maior volume de sinais.
- B. 7 dias após criação; equilíbrio entre ação e ruído.
- C. No fim da competência mensal; menor ruído, mas sinal tardio para controle preventivo.

**P4. Como calcular alertas após uma mutação confirmada?**

- A. Na mesma transação financeira, somar a categoria e persistir a intenção de alerta; garante avaliação imediata, mas aumenta latência e contenção.
- B. Após commit, via outbox/worker; preserva transação financeira e aceita atraso ou perda tolerável do alerta.
- C. Somente durante consultas do resumo; reduz processamento, mas alertas dependem de o usuário consultar.

### Respostas

- P1: A. O contrato canônico de eventos suporta criar, atualizar e excluir despesas.
- P2: C. Atualizações concorrentes devem ser serializadas por usuário em worker.
- P3: C. Rascunhos automáticos não ativados são sinalizados no fim da competência mensal.
- P4: B. Após commit financeiro, alerts são avaliados por outbox/worker.

### Síntese

- Eventos possuem paridade funcional com a API e exigem versão/operação idempotente.
- Alertas são assíncronos e toleram atraso/perda, coerente com a prioridade do valor correto.
- Sinal de rascunho não ativado é tardio e operacional, sem interferir no controle preventivo.
- Serializar todas as atualizações por usuário em worker conflita com confirmação imediata da API e p95 de 300 ms; é necessário limitar ou revisar essa decisão.

## Rodada 14 - Serialização e Contrato de Resposta

### Perguntas

**P1. Onde a serialização por usuário deve ser aplicada?**

- A. Somente no consumer de eventos; API continua transacional e usa controle otimista/lock no Postgres.
- B. API e eventos passam pelo worker; todas as mutações são assíncronas, contrariando consistência imediata.
- C. Não usar worker para serialização; API e consumer usam locks/transações Postgres na mesma chave lógica.

**P2. Qual resposta a API deve fornecer após criar/editar/excluir despesa?**

- A. `200/201` somente após commit, retornando estado e versão atuais; atende consistência imediata.
- B. `202 Accepted` após enfileirar; melhora desacoplamento, mas consulta imediata pode não refletir a mudança.
- C. Resposta sem versão; simplifica contrato, mas dificulta conflito e diagnóstico.

**P3. Como ordenar eventos de atualização/exclusão da mesma despesa?**

- A. Exigir `version` monotônica por despesa; ignorar duplicatas e rejeitar/estacionar lacunas ou regressões.
- B. Usar `occurred_at`; simples, mas relógios e atrasos podem aplicar ordem incorreta.
- C. Ordem de chegada no consumer; menor contrato, mas eventos atrasados podem sobrescrever estado novo.

**P4. O que fazer quando evento de atualização/exclusão chega antes da criação?**

- A. Persistir como pendente e tentar novamente por janela limitada; resiliente, mas adiciona estado operacional.
- B. Rejeitar e enviar para dead-letter/estado de falha observável; simples, mas exige replay pelo produtor.
- C. Criar placeholder da despesa; evita falha, mas inventa estado financeiro incompleto.

### Respostas

- P1: A. A serialização por usuário aplica-se somente ao consumer de eventos; a API permanece síncrona e transacional no Postgres.
- P2: A. A API retorna `200/201` somente após commit, incluindo estado e versão atuais.
- P3: B. Eventos da mesma despesa são ordenados por `occurred_at`.
- P4: A. Atualização/exclusão antes da criação permanece pendente e é retentada por janela limitada.

### Síntese

- API e eventos compartilham a mesma lógica de aplicação, mas possuem estratégias de concorrência diferentes: transação síncrona na API e serialização no consumer.
- `occurred_at` como ordem de negócio exige política para relógios divergentes, empates e eventos muito atrasados.
- Eventos fora de ordem exigem armazenamento pendente, retry e expiração observável.
- A versão retornada pela API ajuda diagnóstico, embora não seja a autoridade escolhida para ordenar eventos.

## Rodada 15 - Ordenação Temporal e Falhas de Eventos

### Perguntas

**P1. Como desempatar eventos com o mesmo `occurred_at`?**

- A. Exigir `event_id` ordenável/único e usar ordem determinística por `occurred_at` + `event_id`; reproduzível, mas a ordem semântica pode não refletir intenção.
- B. Último recebido vence em empate; simples, mas reprocessamento pode produzir resultado diferente.
- C. Rejeitar empate como contrato inválido; preserva integridade, mas exige correção/replay do produtor.

**P2. Como tratar evento com `occurred_at` anterior ao último já aplicado?**

- A. Ignorar como regressão e registrar métrica/log; protege estado atual, mas pode descartar correção tardia legítima.
- B. Aplicar mesmo assim; preserva todos os eventos, mas evento atrasado pode sobrescrever estado novo.
- C. Estacionar para revisão/replay explícito; maior segurança, porém adiciona operação manual.

**P3. Qual janela de retry para eventos pendentes fora de ordem?**

- A. 24 horas, depois estado de falha observável; contém armazenamento e operação.
- B. 7 dias, depois estado de falha observável; tolera atrasos maiores com mais retenção.
- C. Indefinida até chegar a criação; máxima tolerância, mas crescimento sem limite.

**P4. Quem governa o contrato canônico interno de eventos de despesa?**

- A. Budgets publica a especificação consumida pelos produtores; interface/contrato pertence ao consumidor.
- B. Cada produtor define seu payload e budgets cria adapters; flexível, mas aumenta superfície e inconsistência.
- C. Um módulo compartilhado global define o contrato; centraliza, mas cria acoplamento transversal prematuro.

### Respostas

- P1: B. Em empate de `occurred_at`, o último evento recebido vence.
- P2: B. Evento anterior ao último aplicado deve ser aplicado mesmo atrasado.
- P3: A. Eventos pendentes são retentados por 24 horas e depois entram em estado de falha observável.
- P4: A. Budgets governa e publica o contrato canônico interno consumido pelos produtores.

### Síntese

- Budgets é dono do contrato de eventos e produtores devem aderir ao envelope definido pelo consumidor.
- A janela de pendência de 24 horas contém armazenamento e torna falhas operáveis.
- Aplicar eventos antigos e usar ordem de chegada em empate torna a ordem de recepção a autoridade efetiva, podendo produzir estados diferentes em replay.
- Essa semântica conflita com a decisão anterior de ordenar eventos por `occurred_at` e com o requisito de valor correto/reproduzível.

## Rodada 16 - Autoridade Final de Ordenação

### Pergunta

**P1. Qual regra final deve governar atualizações e exclusões concorrentes por evento?**

- A. Ordem de negócio determinística: exigir `version` monotônica; aplicar somente a próxima versão esperada e estacionar lacunas. Replays sempre produzem o mesmo estado.
- B. `occurred_at` determinístico: aplicar somente eventos mais novos; desempatar por `event_id`. Correções tardias exigem novo evento com instante posterior.
- C. Ordem de chegada: todo evento recebido pode sobrescrever o estado, inclusive eventos antigos. É simples, mas replays e atrasos podem alterar valores incorretamente.

### Resposta

- P1: B. A autoridade final é `occurred_at` determinístico: aplicar somente eventos mais novos, com desempate por `event_id`; correções tardias exigem novo evento com instante posterior.

### Síntese

- Regressões temporais são ignoradas e observadas; não sobrescrevem estado mais novo.
- Empates usam `event_id` como desempate determinístico, garantindo replay reproduzível.
- Eventos update/delete antes do create permanecem pendentes por até 24 horas.
- Produtores são responsáveis por emitir correções tardias como novos eventos com `occurred_at` posterior.

## Confirmação da Hipótese Consolidada

### Resumo

- Novo bounded context `internal/budgets` no monólito modular, usando Postgres, outbox, workers e observabilidade existentes.
- Orçamento único por usuário/competência; rascunho, ativado e recorrência de até 12 meses futuros.
- Ativação exige 100% distribuído entre categorias fixas do sistema; valores em centavos, percentuais em basis points e half-even.
- Despesas canônicas entram por API síncrona ou eventos internos, com identidade `(user_id, source, external_transaction_id)`.
- Totais não são persistidos; consultas somam despesas por competência/categoria, com meta p95 até 300 ms.
- Exclusão física mantém tombstone por 24 meses; recriação exige novo identificador externo.
- Eventos são ordenados por `occurred_at` + `event_id`; regressões são ignoradas e dependências ausentes aguardam até 24 horas.
- Alertas de 80% e 100% são avaliados após commit via outbox/worker, limitados a 10 por usuário/mês e tolerantes a perda.
- Retenção de despesas, orçamentos e tombstones por 24 meses, com expurgo mensal.
- Baseline reforçado, telemetria completa, disponibilidade alvo de 99,9%, RPO até 15 minutos e RTO até 4 horas.

### Pergunta

**P1. Como deseja prosseguir?**

- A. Materializar e validar o dossiê técnico com esta direção.
- B. Refinar mais um ponto específico antes de materializar.
- C. Cancelar o discovery sem materializar o dossiê final.

### Resposta

- P1: A. Materializar e validar o dossiê técnico com a direção consolidada.

### Decisão Final

- O usuário aprovou a hipótese consolidada para materialização.
- Status de viabilidade: viável com restrições.
- Riscos residuais aceitos: RPO de até 15 minutos, ausência de histórico financeiro de edições/exclusões, alertas best-effort e liberação geral sem canary/feature flag.
