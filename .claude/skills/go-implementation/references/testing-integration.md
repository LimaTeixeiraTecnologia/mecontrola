# Testing Integration

<!-- TL;DR
Teste de integracao deve ser acionado apenas quando a fronteira real justificar o custo, com isolamento deterministico e sem depender de ambiente externo compartilhado.
Keywords: testing, integration, testcontainers, io
Load complete when: tarefa altera ou cria teste de integracao.
-->

## Objetivo
Definir quando testes de integracao realmente elevam a confiabilidade.

## Regras
- Usar integracao quando mocks nao cobrem confiavelmente a fronteira alterada.
- Preferir `testcontainers-go` ou infraestrutura efemera equivalente quando o projeto suportar.
- Separar testes de integracao com build tag quando o custo operacional justificar.
- Tear down e isolamento devem ser deterministicos.

## Validacao Minima
- `go test -count=1` no pacote alterado.
- `go test -tags=integration -count=1` apenas quando o proprio diff introduzir ou alterar esse tipo de teste.

## Proibido
- Depender de banco de dev, API de staging ou recurso compartilhado.
- Rodar integracao global por padrao em mudanca localizada sem ganho real.
