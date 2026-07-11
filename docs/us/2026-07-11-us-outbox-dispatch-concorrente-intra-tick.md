# Despacho Concorrente Intra-Tick do Outbox — User Story única, pronta para desenvolvimento

> Fonte: pedido do usuário ("analisar `internal/platform`, achar gap/lacuna e criar UMA única US") confrontado com a base de código real do repositório `mecontrola` e com os diagnósticos de capacidade já registrados.
> Data de geração: 2026-07-11
> Nome do arquivo: `2026-07-11-us-outbox-dispatch-concorrente-intra-tick.md`

## Resumo e decisão de escopo

A auditoria de `internal/platform` (quatro subagentes de exploração cobrindo agent/workflow/memory/scorer/tool/llm, outbox/idempotency/events/worker, cluster WhatsApp e primitivos de infra) produziu vários achados. Depois de filtrar os falsos positivos — por exemplo, a suposta "corrida de claim" do outbox é neutralizada pelo índice parcial único `outbox_events_user_inflight_uidx` (`migrations/000001_initial_schema.up.sql:70-72`) e pelo tratamento explícito de `23505` (`internal/platform/outbox/storage_postgres.go:96-101`); e os claims "presos" são recuperados pelo reaper (`internal/platform/outbox/reaper.go:40-54`) — o gap mais material, verificado e autocontido em `internal/platform` é o **processamento estritamente serial das linhas de um batch do outbox dispatcher**.

Cada tick do dispatcher reivindica até 50 linhas (default `OUTBOX_DISPATCHER_BATCH_SIZE`, `configs/config.go:752`), e o desenho do claim garante **no máximo uma linha por usuário** por batch (a subconsulta de menor-pendente-por-usuário em `internal/platform/outbox/storage_postgres.go:77-81` seleciona apenas a linha mais antiga de cada `aggregate_user_id`, e a exclusão de in-flight em `storage_postgres.go:70-76` remove usuários que já têm linha em processamento). Ou seja, as linhas de um mesmo batch pertencem a **usuários distintos e são independentes entre si**. Mesmo assim, o laço `for _, row := range rows` (`internal/platform/outbox/dispatcher.go:93-97`) processa cada linha em sequência, e o handler roda **inline** (`dispatcher.go:106-114`) — para a inbound do WhatsApp isso inclui a chamada LLM síncrona. Com latência de LLM na casa de segundos por linha, um batch cheio consome dezenas de segundos de forma serial, sustentando o teto documentado de ~0,33–1,0 msg/s por instância e o veredito "não atende em pico" dos diagnósticos de capacidade.

Esta única US foca em **executar concorrentemente, com paralelismo limitado, as linhas independentes de um batch do dispatcher**, preservando FIFO por usuário, idempotência, cancelamento cooperativo e agregação de erros. É complementar — e não sobreposta — à evolução futura de sharding cross-instância por hash (ADR-001, fase 2.000–10.000), que permanece fora de escopo.

## Confronto com o Codebase

Comportamento **atual** (evidenciado):

- O tick reivindica um batch e processa cada linha em sequência: `internal/platform/outbox/dispatcher.go:93-97`; não há goroutine, `errgroup`, `sync.WaitGroup` nem semáforo no arquivo (verificado por busca).
- `processClaimed` roda o handler inline e depois marca o resultado em transação própria: `internal/platform/outbox/dispatcher.go:106-114`.
- O handler recebe timeout individual (default 10s, `configs/config.go:753`): `internal/platform/outbox/dispatcher.go:136-138`.
- Os erros das linhas são agregados com `errors.Join`: `internal/platform/outbox/dispatcher.go:103`.
- O backoff de retry já é protegido por mutex (`d.mu`) sobre o `rng`: `internal/platform/outbox/dispatcher.go:197-199` — sinal de que o código foi desenhado antecipando acesso concorrente, sem que o despacho concorrente tenha sido implementado.
- A marcação de resultado (`MarkPublished`, `MarkPendingRetry`, `MarkFailed`, contador de dead-letter e histograma de lag) vive em `internal/platform/outbox/dispatcher.go:146-181`, com labels de métrica restritos a `event_type`.
- Não existe knob de concorrência: a struct `OutboxConfig` (`configs/config.go:331-343`) tem `DispatcherBatchSize` mas nenhum `DispatcherConcurrency`.

Garantia de ordenação por usuário que torna a concorrência segura (evidenciada):

- Claim FIFO por usuário com desempate total `(occurred_at, created_at, id)` e `FOR UPDATE SKIP LOCKED`: `internal/platform/outbox/storage_postgres.go:63-91`.
- Índice parcial único que impede duas linhas in-flight do mesmo usuário no nível do banco: `outbox_events_user_inflight_uidx` em `(aggregate_user_id) WHERE status = 2` — `migrations/000001_initial_schema.up.sql:70-72`.
- Recuperação de linhas presas em processamento (após shutdown/crash) pelo reaper via `ResetStuck`: `internal/platform/outbox/reaper.go:40-54`.

Contexto de produção que amplia o impacto (evidenciado):

- A chamada LLM roda dentro do handler inbound, e o timeout de inbound default é 90s (`configs/config.go:1351`), envolvido por `context.WithTimeout` no consumidor (`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:179`); somado ao processamento serial, isso reduz o throughput por instância.

## Análise de padrão de projeto (skill design-patterns-mandatory)

O seletor determinístico foi executado com sinais canônicos (`prefer_direct_solution`, `performance_hot_path`) e restrições (`preserve_public_contract`, `minimize_class_count`, `minimize_indirection`, `team_needs_low_cognitive_load`) e retornou `status = reject` → "Usar solução direta, módulo pequeno ou refactor local". Conclusão registrada: paralelismo limitado por semáforo ou `errgroup` com limite é solução idiomática de concorrência em Go, **não** um padrão do catálogo clássico. A US, portanto, não prescreve padrão de projeto; especifica a capacidade e as invariantes de robustez, deixando a forma concreta (semáforo com buffer, `errgroup.SetLimit`, worker pool fixo) para a implementação, desde que respeitadas as regras abaixo.

---

## Declaração
Como usuário do MeControla no WhatsApp, quero que minhas mensagens sejam processadas sem atraso mesmo quando muitos usuários escrevem ao mesmo tempo, para que eu receba a resposta em segundos e não em minutos durante os horários de pico.

## Contexto
- Problema: o outbox dispatcher processa as linhas de cada batch de forma estritamente serial, com o handler LLM rodando inline; como as linhas de um batch pertencem a usuários distintos e independentes, essa serialização desperdiça capacidade e sustenta um teto de throughput por instância que não atende picos.
- Resultado esperado: as linhas independentes de um batch passam a ser processadas concorrentemente, com um limite configurável de paralelismo, reduzindo o lag do outbox em pico sem violar a ordem por usuário, a idempotência, o cancelamento cooperativo ou a agregação de erros hoje existentes.
- Fonte: pedido do usuário e confronto com a base de código (`internal/platform/outbox/dispatcher.go`, `internal/platform/outbox/storage_postgres.go`, `internal/platform/outbox/reaper.go`, `configs/config.go`) mais os diagnósticos de capacidade registrados em `docs/runs`.

## Regras de Negócio
- Concorrência intra-tick limitada: as linhas reivindicadas em um mesmo tick são processadas concorrentemente respeitando um teto configurável de linhas em processamento simultâneo, via nova configuração `OUTBOX_DISPATCHER_CONCURRENCY`. O default é 1 (comportamento atual preservado) e o valor é validado dentro de um intervalo fechado, análogo ao já existente para `OUTBOX_DISPATCHER_BATCH_SIZE` ([1..500], `configs/config.go:911-913`); valor fora do intervalo falha a validação de configuração explicitamente.
- Ordem por usuário inviolável: a concorrência só é permitida porque o claim entrega no máximo uma linha por `aggregate_user_id` por batch (`internal/platform/outbox/storage_postgres.go:77-81`) e o índice parcial único `outbox_events_user_inflight_uidx` (`migrations/000001_initial_schema.up.sql:70-72`) impede duas linhas in-flight do mesmo usuário. A implementação não pode introduzir qualquer caminho que processe duas linhas do mesmo usuário no mesmo tick nem reordene linhas do mesmo usuário entre ticks.
- Idempotência intocada: cada linha continua sendo marcada em sua própria transação (`internal/platform/outbox/dispatcher.go:111-113`); a mudança não altera o contrato de `MarkPublished`, `MarkPendingRetry`, `MarkFailed` nem a política de retry/dead-letter (`dispatcher.go:146-181`). Reprocessar a mesma linha continua seguro.
- Timeout individual por handler preservado: cada linha mantém seu próprio `context.WithTimeout` de handler (`internal/platform/outbox/dispatcher.go:136-138`); a concorrência não pode fazer uma linha herdar ou compartilhar o deadline de outra.
- Agregação de erros preservada: o erro de uma linha não interrompe as demais; ao final do tick, os erros das linhas continuam agregados com `errors.Join` (`internal/platform/outbox/dispatcher.go:103`). Uma linha que falha é marcada para retry ou dead-letter exatamente como hoje, sem afetar as linhas concorrentes.
- Cancelamento cooperativo sem leak: ao receber cancelamento de contexto (shutdown), o dispatcher para de iniciar novo trabalho, aguarda o encerramento das linhas em voo dentro do timeout do job (`dispatcher.go:74`) e retorna; linhas ainda reivindicadas mas não marcadas permanecem em `status = 2` e são recuperadas pelo reaper via `ResetStuck` (`internal/platform/outbox/reaper.go:40-54`). Nenhuma goroutine pode sobreviver ao retorno de `Run`.
- Cardinalidade de métrica controlada (R-TXN-004): os labels de métrica permanecem restritos a `event_type`; é proibido adicionar `user_id`, `aggregate_user_id` ou `correlation_key` como label ao instrumentar a concorrência. Métricas de saturação do pool, se adicionadas, usam apenas labels de baixa cardinalidade.
- Contrato público estável: a assinatura pública de `DispatcherJob` (`Name`, `Schedule`, `Timeout`, `Run`) e os construtores `NewDispatcherJob`/`NewObservableDispatcherJob` permanecem compatíveis; a concorrência é um detalhe interno parametrizado por configuração.
- Zero comentários em Go de produção (R-ADAPTER-001.1): o código novo em `internal/platform/outbox` não introduz comentários fora das exceções permitidas.
- Escopo por instância: esta capacidade eleva o throughput de uma instância; não substitui nem antecipa o sharding cross-instância por hash (ADR-001), que continua sendo a evolução separada para além de ~2.000 usuários simultâneos.

## Critérios de Aceite
```gherkin
Cenário: Batch de usuários distintos é processado concorrentemente
  Dado um tick que reivindica um batch com várias linhas, cada uma de um usuário distinto
  E uma configuração de concorrência maior que 1
  Quando o dispatcher processa o batch
  Então as linhas são executadas em paralelo respeitando o teto configurado
  E o tempo total do tick é menor que a soma dos tempos individuais das linhas

Cenário: Concorrência padrão preserva o comportamento serial atual
  Dado a configuração de concorrência com o valor default igual a 1
  Quando o dispatcher processa um batch
  Então as linhas são executadas uma por vez, como hoje
  E nenhuma diferença de comportamento observável é introduzida

Cenário: Ordem por usuário é preservada entre ticks
  Dado que um usuário tem várias mensagens pendentes em ordem
  Quando os ticks sucessivos processam esse usuário
  Então no máximo uma linha desse usuário está em processamento por vez
  E as linhas do mesmo usuário são publicadas na ordem de ocorrência

Cenário: Falha de uma linha não afeta as linhas concorrentes
  Dado um batch em que o handler de uma linha retorna erro e as demais têm sucesso
  Quando o dispatcher processa o batch concorrentemente
  Então a linha com erro é marcada para retry ou dead-letter conforme a política atual
  E as demais linhas são publicadas normalmente
  E o erro é agregado ao final do tick sem interromper as outras linhas

Cenário: Shutdown durante processamento concorrente não vaza goroutine nem perde evento
  Dado um batch em processamento concorrente
  Quando o contexto do dispatcher é cancelado por shutdown
  Então o dispatcher para de iniciar novas linhas e aguarda as linhas em voo dentro do timeout do job
  E as linhas ainda não marcadas permanecem reivindicadas e são recuperadas depois pelo reaper
  E nenhuma goroutine sobrevive ao retorno da execução do tick

Cenário: Configuração de concorrência inválida falha na inicialização
  Dado um valor de concorrência fora do intervalo permitido
  Quando a configuração do outbox é validada na inicialização
  Então a validação falha explicitamente com mensagem que nomeia a variável
  E o serviço não sobe com concorrência inválida
```

## Dados e Permissões
- Dados obrigatórios: nova configuração `OUTBOX_DISPATCHER_CONCURRENCY` (inteiro, default 1, intervalo fechado validado), somada às configurações existentes do outbox (`OUTBOX_DISPATCHER_BATCH_SIZE`, `OUTBOX_DISPATCHER_HANDLER_TIMEOUT`, `OUTBOX_DISPATCHER_TICK_INTERVAL`, `configs/config.go:333-335`); as linhas reivindicadas carregam `aggregate_user_id`, `occurred_at`, `created_at`, `id` usados no claim e no desempate (`internal/platform/outbox/storage_postgres.go:82,90-91`).
- Perfis/permissões: capacidade interna de plataforma executada pelo processo worker; não expõe endpoint novo e não altera perfis de acesso de usuário final. Não há mudança de permissão de banco além das já concedidas ao dispatcher.

## Dependências
- Claim FIFO por usuário com uma linha por usuário por batch: `internal/platform/outbox/storage_postgres.go:63-91` (pré-condição que torna a concorrência segura).
- Índice parcial único de in-flight por usuário: `migrations/000001_initial_schema.up.sql:70-72` (backstop de banco).
- Reaper de recuperação de linhas presas: `internal/platform/outbox/reaper.go:40-54`.
- Unit of Work e pool de conexões que suportam transações concorrentes: `internal/platform/database/uow/uow.go`, `internal/platform/database/postgres/postgres.go` (o pool precisa comportar `OUTBOX_DISPATCHER_CONCURRENCY` transações simultâneas de marcação).
- Carregamento e validação de configuração do outbox: `configs/config.go:331-343,752-753,900-913`.

## Fora de Escopo
- Sharding cross-instância por hash do outbox (ADR-001, fase 2.000–10.000): evolução separada, não construída aqui.
- Desacoplar a chamada LLM do handler do dispatcher (processamento assíncrono do LLM fora do caminho do tick): mudança arquitetural distinta.
- Endurecer o timeout do provider LLM e honrar o campo `llm.Config.RequestTimeout` hoje declarado e não aplicado (`internal/platform/llm/openrouter.go:39,75`): gap separado, não tratado nesta US.
- Alterações no middleware de idempotência HTTP, no cluster WhatsApp ou em qualquer outro subpacote de `internal/platform` fora do outbox dispatcher.
- Mudança na semântica de retry, backoff, dead-letter ou housekeeping do outbox.

## Evidências
- Entrada: pedido do usuário para analisar `internal/platform`, identificar gap/lacuna e produzir uma única US salva em `docs/us` com o prefixo da data de hoje.
- Base de código: processamento serial em `internal/platform/outbox/dispatcher.go:93-97`; handler inline em `dispatcher.go:106-114`; timeout de handler em `dispatcher.go:136-138`; agregação de erro em `dispatcher.go:103`; mutex de backoff pré-existente em `dispatcher.go:197-199`; marcação/dead-letter em `dispatcher.go:146-181`; claim uma-linha-por-usuário em `storage_postgres.go:63-91` (menor-pendente em `:77-81`, exclusão de in-flight em `:70-76`); tratamento de `23505` em `storage_postgres.go:96-101`; índice único de in-flight em `migrations/000001_initial_schema.up.sql:70-72`; reaper em `reaper.go:40-54`; ausência de knob de concorrência em `configs/config.go:331-343`; defaults e bounds do batch em `configs/config.go:752,911-913`; LLM inline com inbound timeout 90s em `configs/config.go:1351` e `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:179`.
- Inferências: a redução de latência em pico é proporcional ao teto de concorrência escolhido e à latência média do handler; o ganho exato depende do perfil de carga real e não foi medido nesta análise.
- Não evidenciado: não há teste de carga executado que quantifique o throughput resultante por valor de `OUTBOX_DISPATCHER_CONCURRENCY`; a fronteira empírica de ~2.000 usuários citada vem dos diagnósticos registrados, não de nova medição feita aqui.

## Notas de Validação
- Skills obrigatórias aplicadas: `go-implementation` (concorrência com `context`, goroutines canceláveis sem leak, agregação com `errors.Join`, zero comentários R-ADAPTER-001.1), `domain-modeling-production` (invariante de ordem por usuário e estados de linha preservados como contrato) e `design-patterns-mandatory` (seletor determinístico retornou `reject` → solução direta idiomática de concorrência, sem padrão do catálogo). US produzida via skill `user-stories`.
- Governança respeitada: cardinalidade de métrica controlada (R-TXN-004, labels restritos a `event_type`); contrato público de `DispatcherJob` estável; alterações contidas em `internal/platform/outbox` e na configuração do outbox.
- Segurança de concorrência comprovada por desenho: a base de dados garante no máximo uma linha in-flight por usuário (índice único parcial) e o claim entrega uma linha por usuário por batch, de modo que processar o batch em paralelo não pode violar FIFO por usuário.
- Cobertura de cenários: fluxo feliz (concorrência efetiva), fluxo alternativo (default 1 preserva o comportamento atual e preservação de ordem entre ticks), e cenários de erro/bloqueio (falha isolada de uma linha, shutdown cooperativo sem leak, configuração inválida barrada na inicialização).
- Verificação de higiene da história: sem marcadores pendentes, sem placeholders, sem termos não resolvidos; evidências separam entrada, base de código, inferência e não evidenciado.
