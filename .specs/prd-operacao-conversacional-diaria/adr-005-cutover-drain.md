# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Cutover do legado com drenagem de runs suspensos por janela de graça
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Solicitante do produto, engenharia de plataforma
- **Relacionados:** PRD `prd.md` (RF-28); techspec `techspec.md`; reapers de workflow

## Contexto

A reescrita substitui a camada conversacional do dia a dia. No momento do corte, podem existir runs de workflow suspensos em produção aguardando resposta do usuário (registro em confirmação, confirmação destrutiva, cadastro de cartão, edição de orçamento). Descartá-los abruptamente quebraria usuários no meio de uma operação e perderia confirmações em aberto.

## Decisão

Cutover com **drenagem por janela de graça**: antes de desativar o legado, novos inbounds já entram no fluxo novo, enquanto os runs suspensos existentes continuam podendo concluir ou expiram pelo TTL/reaper de cada workflow. O legado só é desativado após a janela de graça (dimensionada pelo maior TTL dos workflows suspensos). Nenhuma confirmação em aberto é encerrada à força.

## Alternativas Consideradas

- **Descartar/expirar imediatamente no corte**: simples; rejeitada por quebrar quem estava confirmando um lançamento (má experiência, risco de retrabalho).
- **Migrar o estado dos runs para o novo formato**: máxima continuidade; rejeitada por exigir migração arriscada de snapshots para estados que serão redesenhados.

## Consequências

### Benefícios Esperados

- Sem perda de confirmações em aberto; transição sem quebra para o usuário.
- Sem migração de dados de estado arriscada.

### Trade-offs e Custos

- Janela de convivência em que legado e novo coexistem para resumes antigos.

### Riscos e Mitigações

- Risco: run suspenso nunca respondido. Mitigação: reaper por workflow expira pelo TTL; a janela de graça cobre o maior TTL.
- Risco: divergência de comportamento entre legado e novo durante a janela. Mitigação: novos inbounds já usam o novo fluxo; apenas resumes de runs pré-existentes tocam o legado, por tempo limitado.

## Plano de Implementação

1. Garantir reaper ativo para cada workflow suspenso, com TTL definido.
2. Ativar o roteamento novo para novos inbounds (feature de corte no consumer).
3. Manter os resolvers de resume do legado apenas durante a janela de graça.
4. Após a janela (maior TTL decorrido), remover o legado e seus resolvers.

## Monitoramento e Validação

- Métrica de runs suspensos por workflow (contagem decrescente durante a janela).
- Sucesso: contagem de runs suspensos do legado chega a zero antes da remoção.

## Impacto em Documentação e Operação

- Runbook de cutover com a sequência de ativação, a duração da janela de graça e o critério de remoção do legado.

## Revisão Futura

- Revisitar a duração da janela se os TTLs dos workflows mudarem.
