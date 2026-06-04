## Descrição

<!-- Descreva o que esta PR faz e por que. Inclua contexto relevante de produto ou técnico. -->

## Tipo de Mudança

- [ ] Nova funcionalidade (feature)
- [ ] Correção de bug
- [ ] Refatoração (sem mudança de comportamento)
- [ ] Documentação / governança
- [ ] Infraestrutura / CI/CD
- [ ] Configuração / dependências

## Testes

- [ ] Testes unitários adicionados/atualizados
- [ ] Testes de integração adicionados/atualizados
- [ ] Validação manual realizada (descrever abaixo se aplicável)

## Breaking Changes

- [ ] Esta PR introduz breaking changes
  <!-- Se sim, descrever impacto e plano de migração -->

---

## Outbox / Event Handler

<!-- Preencher esta seção APENAS se a PR toca `internal/platform/outbox`, registra um handler de outbox, ou altera comportamento transacional de publicação de eventos. Caso contrário, remover esta seção. -->

- [ ] O Handler é idempotente por `event.ID` (upsert ou tabela de deduplicação — RF-04/RF-38)
- [ ] `event.ID` é usado como chave de deduplicação (não `aggregate_id` isolado)
- [ ] O Handler está registrado via `Registry.Register` com `Subscription.Name` único no bootstrap (RF-06)
- [ ] O payload não contém segredos, PII não necessário ou dados sensíveis em texto claro (RF-30)
