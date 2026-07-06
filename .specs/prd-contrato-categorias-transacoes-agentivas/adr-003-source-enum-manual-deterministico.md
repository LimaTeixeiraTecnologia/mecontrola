# Registro de Decisao Arquitetural (ADR)

## Metadados

- **Titulo:** Fonte da decisao como enum fechado e manual deterministico
- **Data:** 2026-07-06
- **Status:** Aceita
- **Decisores:** Engenharia
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD fixa as fontes permitidas: `auto_matched`, `user_selected_candidate`, `manual_canonical_id` e `system_migration`. Tambem fixa que escrita manual aprovada deve persistir evidencia deterministica com score, confidence e quality especificos.

## Decisao

Modelar `CategoryDecisionSource` como enum fechado em Go e constraint no banco. Writes manuais com IDs fornecidos nao pulam o gate: eles validam raiz, folha, kind, ativo e versao, e persistem `score=1.0`, `confidence=manual_confirmed`, `quality=manual_canonical`, `source=manual_canonical_id`, `signal_type=manual_canonical`, `matched_term=<subcategory_slug>` e `match_reason=manual canonical id validated`.

## Alternativas Consideradas

- Tratar manual como excecao sem score/quality: menor friccao, mas vira escape hatch.
- Usar string livre para source: flexivel, mas viola DMMF e dificulta auditoria.
- Reusar `high/exact/canonical_name` para manual: evita novos valores, mas confunde match editorial com escolha humana validada.

## Consequencias

### Beneficios Esperados

- Auditoria clara da origem da decisao.
- Comportamento manual reproduzivel.
- Bloqueio uniforme para fluxos agentivos e nao agentivos.

### Trade-offs e Custos

- Necessita novos valores fechados para confidence/quality de evidencia persistida.
- Mapeamento entre enums de `categories` e enums de evidencia precisa ser explicito.

### Riscos e Mitigacoes

- Risco: `system_migration` ser usado indevidamente.
  Mitigacao: bloquear uso em writes runtime comuns; permitir apenas por tarefa operacional explicitamente autorizada.

## Plano de Implementacao

1. Criar VOs de source, confidence persistida, quality persistida e signal type persistido.
2. Aplicar smart constructors.
3. Adicionar constraints SQL.
4. Cobrir casos manuais em tests de use case e repositorio.

## Monitoramento e Validacao

Metricas por `source` devem mostrar volume de manual vs auto sem expor IDs ou texto do usuario.

## Impacto em Documentacao e Operacao

Documentar que `manual_canonical_id` exige evidencia deterministica completa.

## Revisao Futura

Revisar somente se surgir demanda operacional explicita para importacao ou migracao de dados.
