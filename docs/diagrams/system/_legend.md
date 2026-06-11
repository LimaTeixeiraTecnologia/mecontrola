# Legend

## Elementos

- `Person`: ator humano direto.
- `Software System`: sistema externo conectado ao MeControla.
- `Container`: processo executavel, API, worker ou datastore logico.
- `ContainerDb`: armazenamento persistente.
- `ContainerQueue`: armazenamento/filas logicas para troca assincrona.

## Estilo de relacao

- Relacao sem tag especial: uso padrao.
- Tag `sync`: chamada sincrona.
- Tag `async`: chamada assincrona via outbox/event dispatcher.

## Leitura recomendada

1. Leia `mecontrola-container.svg` para entender fronteiras, processos e integracoes.
2. Leia `mecontrola-async-relations.svg` para entender eventos e jobs.
3. Leia o arquivo `*-flows.md` do modulo para seguir o percurso completo.
