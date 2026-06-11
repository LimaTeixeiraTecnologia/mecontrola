# Use Case Read

<!-- TL;DR
Leitura em application deve permanecer fina, sem efeito colateral persistente, com interface no consumidor e validacao proporcional ao pacote alterado.
Keywords: usecase, query, read, application
Load complete when: tarefa altera caso de uso de leitura em application.
-->

## Objetivo
Padronizar use cases de leitura com baixo acoplamento e sem ceremony desnecessaria.

## Regras
- Use case de leitura pode receber parametros primitivos quando isso mantiver a linguagem ubiqua clara.
- Nao criar Command Object para leitura so por simetria com escrita.
- Orquestrar apenas leitura, composicao simples e traducao de erros.
- Nao escrever em banco, emitir evento ou disparar side effect persistente.
- Interface fica no consumidor quando houver fronteira real.
- DTO/output deve expor apenas o contrato necessario ao caller.

## Validacao Minima
- `go test -count=1` no pacote alterado.
- `go build` e `go vet` no pacote ou modulo afetado quando houver codigo de producao alterado.

## Proibido
- Side effect persistente.
- Branching de negocio complexo no handler em vez do use case.
- Interface prematura sem fronteira real.
