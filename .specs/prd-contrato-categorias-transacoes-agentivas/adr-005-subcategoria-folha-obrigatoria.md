# Registro de Decisao Arquitetural (ADR)

## Metadados

- **Titulo:** Subcategoria folha obrigatoria
- **Data:** 2026-07-06
- **Status:** Aceita
- **Decisores:** Engenharia
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD exige 0 falso positivo conhecido e decidiu que toda transacao deve usar subcategoria folha. O codigo atual permite raiz sem subcategoria em alguns caminhos, especialmente receitas e templates recorrentes.

## Decisao

Bloquear toda escrita categorizada com raiz sem subcategoria. A regra vale para transacoes diretas, edicoes, templates recorrentes, fluxos agentivos e writes manuais nao agentivos.

## Alternativas Consideradas

- Permitir raiz sem folha para receitas: reduz friccao, mas cria contrato divergente entre despesa e receita.
- Permitir raiz sem folha quando a raiz nao tiver filhos: depende de estado editorial e enfraquece a regra de persistencia.
- Permitir lista manual de slugs terminais: adiciona excecao operacional e risco de drift.

## Consequencias

### Beneficios Esperados

- Contrato uniforme para todas as escritas.
- Auditoria mais precisa por categoria folha.
- Menor risco de categorizar transacao em nivel amplo demais.

### Trade-offs e Custos

- Catalogo precisa ter folhas suficientes para despesas e receitas.
- Mais pedidos de clarificacao quando a entrada do usuario apontar apenas para raiz.

### Riscos e Mitigacoes

- Risco: aumento de bloqueios por ausencia de folha cadastrada.
  Mitigacao: testes de catalogo devem garantir folhas para categorias usadas em fluxos principais.

## Plano de Implementacao

1. Tornar `subcategory_id` obrigatorio no baseline de transacoes e templates.
2. Alterar guards de use case para exigir folha em todas as direcoes.
3. Alterar `CategoryWriteGate` para rejeitar root igual a leaf e subcategoria ausente.
4. Cobrir create/update/recorrencia/manual em testes.

## Monitoramento e Validacao

Validar por testes de integracao e metricar bloqueios com `reason=root_without_leaf`.

## Impacto em Documentacao e Operacao

Documentar que toda categoria persistida em transacao/template deve ser uma folha canonica.

## Revisao Futura

Revisar apenas se um PRD futuro introduzir terminalidade explicita no catalogo.
