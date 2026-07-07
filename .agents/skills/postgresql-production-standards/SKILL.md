---
name: postgresql-production-standards
version: 1.0.0
description: Projeta, revisa e endurece o uso de PostgreSQL em desenvolvimento de software com foco mandatário em economia, eficiencia, robustez, seguranca, escalabilidade e operacao confiavel. Usa apenas documentacao oficial do PostgreSQL para orientar modelagem, SQL, indices, transacoes, migrations, privilegios, observabilidade, manutencao, backup, restore e replicacao. Use quando o pedido envolver criar, revisar, validar ou corrigir uso de PostgreSQL em qualquer linguagem. Nao use para bancos nao-PostgreSQL, brainstorming sem evidencia, ou recomendacoes baseadas em ferramentas e fontes nao oficiais.
---

# PostgreSQL Production Standards

<critical>Toda saida DEVE estar em PT-BR.</critical>
<critical>Toda recomendacao DEVE ter dupla evidencia obrigatoria: fato observado no projeto ou no input, e regra oficial do PostgreSQL correspondente.</critical>
<critical>Sem dupla evidencia, a skill DEVE bloquear a resposta e retornar `needs_input`.</critical>
<critical>A skill DEVE privilegiar economia, eficiencia e robustez. Solucoes mais caras ou mais complexas so sao aceitaveis quando a evidencia tecnica exigir.</critical>
<critical>A skill DEVE usar apenas documentacao oficial do PostgreSQL em `postgresql.org/docs`.</critical>
<critical>Quando a versao detectada for 14, 15, 16 ou 17, a skill DEVE resolver as URLs oficiais para a major correspondente e NAO usar `docs/current` como base normativa.</critical>
<critical>Versoes suportadas por padrao: PostgreSQL 14, 15, 16, 17 e 18. Fora disso, bloquear por incompatibilidade de baseline.</critical>

## Procedimentos

**Step 1: Detectar contexto PostgreSQL**
1. Execute `python3 scripts/detect-postgres-context.py <project_root>` para identificar versao, stack, artefatos SQL, migrations, ORM ou driver e evidencias de uso de PostgreSQL.
2. Se o script falhar, interrompa e use `assets/needs-input-template.md` para pedir a evidencia minima faltante.
3. Registre como contexto canonico:
   - versao observada ou inferida
   - linguagem, framework e driver/ORM
   - superficie principal do pedido: schema, query, indice, transacao, seguranca, manutencao, observabilidade, backup ou replicacao
   - arquivos e trechos que sustentam a analise
4. Leia `references/version-policy.md` para aplicar gates de versao e resolver a major version normativa antes de qualquer recomendacao.

**Step 2: Classificar o pedido em um dominio principal**
1. Classifique o pedido em exatamente um dominio principal:
   - modelagem e migrations
   - queries e indices
   - transacoes e locking
   - seguranca e controle de acesso
   - manutencao e observabilidade
   - backup, restore e replicacao
2. Se o pedido atravessar varios dominios, escolha o dominante e trate os demais como restricoes auxiliares materialmente necessarias.
3. Se nao houver dominio dominante claro, bloqueie e retorne `needs_input` pedindo o artefato ou objetivo principal.

**Step 3: Carregar as referencias obrigatorias**
1. Leia a referencia do dominio principal:
   - `references/modeling-and-migrations.md`
   - `references/queries-and-indexes.md`
   - `references/transactions-and-locking.md`
   - `references/security-and-access.md`
   - `references/maintenance-and-observability.md`
   - `references/backup-and-replication.md`
2. Leia tambem qualquer referencia auxiliar sem a qual a resposta ficaria sem base oficial suficiente.
3. Leia `references/checklist.md` antes de finalizar qualquer saida.
4. Registre explicitamente, na propria saida, quais referencias oficiais foram usadas.
5. Nao use conhecimento implicito, preferencia pessoal ou pratica de ecossistema que nao esteja ancorada na referencia oficial aplicavel.

**Step 4: Escolher o modo de saida**
1. Use `recommendation` quando o usuario pedir direcao tecnica, design, correcao ou padrao para implementar.
2. Use `review-findings` quando o usuario pedir auditoria, review, validacao ou diagnostico sobre artefatos existentes.
3. Use `needs_input` quando faltar versao, SQL, migration, configuracao, objetivo, volumetria ou outra evidencia material.
4. Leia o template correspondente em `assets/` antes de responder:
   - `assets/recommendation-template.md`
   - `assets/review-findings-template.md`
   - `assets/needs-input-template.md`

**Step 5: Emitir a decisao mandatória**
1. Para `recommendation`, entregue:
   - contexto observado
   - regra oficial aplicada
   - URLs oficiais utilizadas
   - decisao objetiva
   - impacto em custo, eficiencia, robustez e escalabilidade
   - validacao minima obrigatoria
2. Para `review-findings`, reporte apenas achados com evidencia concreta em `arquivo:linha`, migration, SQL, plano ou configuracao observavel.
3. Para `needs_input`, liste somente lacunas objetivas e a evidencia minima exigida para destravar a analise.
4. Quando houver mais de uma opcao tecnicamente valida, recomende uma unica opcao. So apresente alternativa quando a referencia oficial exigir trade-off explicito.

**Step 6: Aplicar regras por dominio**
1. Em modelagem e migrations:
   - exija tipos corretos, constraints reais e migrations reversiveis ou com rollback planejado
   - rejeite modelagem redundante, falta de `PRIMARY KEY`, `UNIQUE`, `CHECK` ou `FOREIGN KEY` quando a integridade exigir
2. Em queries e indices:
   - rejeite indice sem predicado de consulta observavel
   - prefira o menor conjunto de indices que cubra os planos de acesso relevantes
   - trate `EXPLAIN` ou `EXPLAIN ANALYZE` como evidencia obrigatoria quando o pedido for tuning
3. Em transacoes e locking:
   - escolha o menor isolamento que preserve corretude comprovada
   - exija estrategia explicita de retry para `serialization_failure` e conflitos equivalentes quando usar `SERIALIZABLE`
4. Em seguranca e acesso:
   - aplique least privilege com roles separadas por funcao
   - rejeite uso desnecessario de superuser, owner compartilhado ou grants amplos
5. Em manutencao e observabilidade:
   - trate autovacuum, `ANALYZE`, estatisticas e monitoracao como obrigatorios
   - rejeite tuning manual arbitrario sem evidencia operacional
6. Em backup, restore e replicacao:
   - exija objetivo operacional observavel para escolher dump logico, backup fisico ou replicacao
   - rejeite estrategia sem prova de restore testado

**Step 7: Validar antes de finalizar**
1. Confirme que a saida:
   - esta em PT-BR
   - cita apenas URLs oficiais do PostgreSQL resolvidas para a major version aplicavel
   - possui dupla evidencia
   - escolhe uma decisao unica
   - prioriza economia, eficiencia e robustez
2. Execute `python3 scripts/validate-doc-map.py .` ao alterar referencias ou URLs da skill.
3. Se qualquer gate falhar, nao improvise; bloqueie com `needs_input`.

## Error Handling
* Se `scripts/detect-postgres-context.py` nao detectar PostgreSQL, interrompa e solicite prova objetiva de que o projeto usa PostgreSQL.
* Se a versao detectada for inferior a 14 ou superior a 18, bloqueie por baseline fora do escopo e peça alinhamento explicito de compatibilidade.
* Se o pedido envolver ferramenta externa, extensao ou servico sem cobertura na documentacao oficial usada pela skill, recuse a recomendacao e informe a lacuna.
* Se o usuario pedir tuning sem SQL, plano de execucao, cardinalidade ou sinais operacionais minimos, retorne `needs_input`.
* Se houver conflito entre evidencia do projeto e a regra oficial aplicavel, priorize a regra oficial e reporte o conflito como risco.
