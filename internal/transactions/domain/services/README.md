# Domain Services — Transactions

As funcoes `Decide*` neste pacote sao **puras**: dado o mesmo input (comando, entidade atual, IDs pre-gerados e instante `now`), produzem o mesmo output sem efeitos colaterais, sem IO e sem dependencia de estado externo. Essa pureza garante testabilidade total sem mocks e torna o comportamento do dominio auditavel e previsivel. Os efeitos (persistencia, publicacao de eventos, spans) vivem exclusivamente nas camadas `application/usecases/` e `infrastructure/` que consomem o resultado dos `Decide*`.
