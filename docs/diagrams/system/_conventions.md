# Conventions

## Tipos de fluxo

- `sync`: chamada no mesmo request ou no mesmo ciclo de job, com expectativa imediata de retorno.
- `async`: publicacao e consumo desacoplados via `outbox_events` + `outbox dispatcher` + `events.Dispatcher`.
- `sync + async`: entrada sincrona que persiste estado e tambem agenda um efeito posterior assincrono.

## Origem canonica de um fluxo

Todo fluxo deve nascer explicitamente em um destes pontos:

- endpoint HTTP
- webhook HTTP
- consumer de evento
- job agendado
- producer/outbox publication

## Nomenclatura de relacoes

- Prefixo `[sync]` para relacao sincrona.
- Prefixo `[async]` para relacao assincrona.
- Sempre rotular a intencao e a tecnologia/protocolo.

Exemplos:

- `[sync] POST /api/v1/cards`
- `[sync] consulta sales atualizadas`
- `[sync] persiste projection`
- `[async] publica billing.subscription.activated via outbox`
- `[async] consome onboarding.subscription_bound`

## Datastores logicos

Um mesmo PostgreSQL aparece em subdominios logicos quando isso melhora o entendimento:

- `core tables`
- `outbox_events`
- `processed_events`
- `kiwify_events`
- `subscription projections`
- `auth_events`
- `whatsapp dedup`
- `idempotency keys`
- `magic_tokens`
- `budgets/expenses/alerts/pending_events`

## Regras de leitura dos percursos

- `handler -> usecase -> repository/service/client` indica percurso sincrono de entrada.
- `producer -> outbox_events -> dispatcher job -> event handler` indica percurso assincrono.
- Em jobs, o "origem" sempre e o nome do job exposto em `Name()` e o gatilho e o `Schedule()`.
- Em consumers, o "origem" sempre e o `event_type` literal registrado no modulo.

## Limites intencionais

- Os diagramas nao descrevem deployment por ambiente, replicas, pods ou balanceadores.
- Os arquivos `*-flows.md` citam handlers, use cases e repositorios reais, mas nao substituem leitura de componente ou codigo.

## Padrao production-ready de detalhamento

Todo fluxo deve ser lido tambem sob estas lentes:

- `Eficiencia`
  - qual datastore dominante e tocado;
  - se ha fan-out assincrono ou paginação externa;
  - se o custo cresce por request, por item ou por backlog.
- `Robustez`
  - onde existe idempotencia, dedup ou controle de versao;
  - se o fluxo pode ser reexecutado sem dano;
  - quais erros sao transientes versus definitivos.
- `Operacao`
  - qual metrica, contador ou log estruturado indica saude do fluxo;
  - qual backlog ou checkpoint governa retomada;
  - qual dependencia externa pode degradar latencia.
- `Seguranca`
  - autenticacao, assinatura, allowlist, validacao de payload, PII mascarada.
- `Capacidade`
  - quais fluxos sao bound por CPU, IO de banco, API externa ou polling de worker.

## Taxonomia de falhas

- `falha transiente`
  - indisponibilidade temporaria de banco, rede ou API externa;
  - deve privilegiar retry, backlog ou reprocessamento.
- `falha definitiva`
  - payload invalido, assinatura invalida, versao invalida, recurso inexistente por contrato;
  - deve parar o fluxo sem retry infinito.
- `falha ordenacional`
  - evento fora de ordem, duplicate delivery, replay;
  - deve usar idempotencia, dedup, expected_version ou pending backlog.

## Checklist de leitura final

- Ha persistencia antes de side-effect externo quando o fluxo precisa sobreviver a crash?
- O fluxo deixa rastros suficientes para auditoria e replay?
- O erro e tratado no ponto correto, sem dupla interpretacao?
- O request sincrono evita trabalho pesado desnecessario que poderia ir para async?
