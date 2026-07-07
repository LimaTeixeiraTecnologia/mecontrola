# Transacoes e Locking

## Fontes Oficiais
- MVCC: https://www.postgresql.org/docs/current/mvcc.html
- Transaction isolation: https://www.postgresql.org/docs/current/transaction-iso.html
- Explicit locking: https://www.postgresql.org/docs/current/explicit-locking.html
- App-level consistency checks: https://www.postgresql.org/docs/current/applevel-consistency.html

## Regras Mandatorias
- Escolher o menor nivel de isolamento capaz de preservar a corretude exigida pelo caso observado.
- Nao usar `SERIALIZABLE` por padrao; usar apenas quando a anomalia a evitar estiver claramente demonstrada.
- Quando usar `SERIALIZABLE`, exigir estrategia explicita de retry para falhas de serializacao.
- Manter transacoes curtas, com ordem consistente de acesso e sem trabalho externo prolongado dentro da transacao.
- Usar locks explicitos apenas quando MVCC e constraints nao resolverem o problema com seguranca.
- Em concorrencia sensivel, preferir garantias por constraint unica, `INSERT ... ON CONFLICT`, ou estrategia equivalente oficial antes de coordenacao manual mais cara.

## Bloqueios Obrigatorios
- Bloquear recomendacao de isolamento ou lock sem descrever a anomalia ou conflito que precisa ser evitado.
- Bloquear fluxo que mistura I/O remoto, chamadas HTTP ou processamento pesado dentro de transacao sem justificativa material.
- Bloquear uso de lock amplo em tabela quando o problema puder ser resolvido com lock mais fino ou regra de integridade declarativa.

## Evidencia Minima
- Fluxo transacional ou pseudocodigo.
- Tipo de concorrencia esperada.
- Invariante funcional que nao pode quebrar.
- Erros ou sintomas observados, quando existirem.
