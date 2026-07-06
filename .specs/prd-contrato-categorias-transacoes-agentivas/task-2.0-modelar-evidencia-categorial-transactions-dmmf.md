# Tarefa 2.0: Modelar Evidencia Categorial em Transactions com DMMF

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar os value objects e tipos fechados de `internal/transactions` que representam evidencia categorial aprovada. Esta tarefa cria a base DMMF para impedir string livre critica, source invalido, manual escape hatch e zero value valido por acidente.

<requirements>
RF-04, RF-05, RF-17, RF-19, RF-20, RF-21, RF-22, RF-30, RF-33, RF-34.
RNF-01, RNF-05.
CA-16, CA-18, CA-19, CA-21, CA-22.
</requirements>

## Subtarefas

- [ ] 2.1 Criar `CategoryDecisionSource` com enum fechado: `auto_matched`, `user_selected_candidate`, `manual_canonical_id`, `system_migration`.
- [ ] 2.2 Criar `CategoryWriteEvidence` com campos privados e smart constructor.
- [ ] 2.3 Validar outcome `matched`, score `[0,1]`, confidence, quality, signal type, UUIDs, kind, path, version e decided at.
- [ ] 2.4 Validar regra manual deterministica: `score=1.0`, `confidence=manual_confirmed`, `quality=manual_canonical`, `source=manual_canonical_id`, `signal_type=manual_canonical`, `matched_term=<subcategory_slug>`, `match_reason=manual canonical id validated`.
- [ ] 2.5 Criar erros tipados ou sentinels quando o caller precisar distinguir invalidade funcional.
- [ ] 2.6 Garantir que nenhuma API publica aceite outcome/source/kind criticos como string livre sem parse.

## Detalhes de Implementação

Seguir `techspec.md`, secoes "Contrato de transactions", "Persistencia" e "Conformidade com Padroes". Aplicar DMMF de forma obrigatoria: state-as-type, smart constructors, zero value invalido, transicoes puras e erros discriminaveis por `errors.Is`/`errors.As`.

## Critérios de Sucesso

- `CategoryWriteEvidence` nao pode ser construido em estado invalido.
- Source manual nao pode mascarar baixa evidencia.
- O dominio de `transactions` nao importa `categories`, `agents`, banco ou HTTP.
- Go production code nao inclui comentarios comuns, conforme AGENTS.md.
- Todos os campos necessarios para persistencia normalizada sao acessiveis por metodos seguros.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/transactions/domain/...`
- [ ] Unit tests table-driven para source invalido, outcome diferente de `matched`, score fora de `[0,1]`, root igual leaf, version `<=0`, path vazio e kind invalido.
- [ ] Unit tests para manual deterministico completo e para cada campo manual faltante.
- [ ] Unit tests garantindo zero value invalido para VOs novos.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/domain/valueobjects/category_decision_source.go`
- `internal/transactions/domain/valueobjects/category_write_evidence.go`
- `internal/transactions/domain/valueobjects/category_write_evidence_test.go`
- `internal/transactions/domain/valueobjects`
