# Prompt Enriquecido - Auditoria Exaustiva 7 Dias

Data de preparacao: 2026-07-03
Idioma: pt-BR
Status: pronto para uso

## Prompt original

```text
Eu quero que analise em: ssh root@187.77.45.48, logs, tracing, metrics e banco de dados os ultimos 7 dias e veja se o codebase atual resolveu 100% dos bugs, lacunas, ressalvas, gaps que existiam, faça uma analise exaustiva com comprovações que realmente foi resolvido com base no usuário:
select * from users u where u.id = '06edc407-4f63-42e8-b07c-946b9ef0a19c'; e o e-mail: jailton.junior94@outlook.com

eu quero no resultado evidencias com 0 gaps, sem desvios, sem flexividade, 0 gaps, 0 lacunas, 0 falso positivo e realmente production-ready/proof pronto para produção.

NÃO IMPLEMENTE NADA, APENAS CRIE/ENRIQEUÇA O PROMPT E DEIXE PRONTO PARA USO
```

## Prompt enriquecido

```text
Voce deve executar uma auditoria tecnica exaustiva, read-only e orientada a evidencias para validar se o codebase atual eliminou integralmente os bugs, lacunas, ressalvas, gaps e riscos anteriormente existentes para um caso real de producao.

Escopo e alvo:
- Ambiente remoto: `ssh root@187.77.45.48`
- Fontes obrigatorias de evidencia: logs, traces, metrics, banco de dados e codebase atual em producao
- Janela temporal obrigatoria: ultimas 168 horas contadas retroativamente a partir do inicio da execucao; registrar no relatorio a faixa exata em UTC e no fuso local do servidor
- Usuario de referencia:
  - `user_id = 06edc407-4f63-42e8-b07c-946b9ef0a19c`
  - `email = jailton.junior94@outlook.com`
- Consulta base obrigatoria:
  - `select * from users u where u.id = '06edc407-4f63-42e8-b07c-946b9ef0a19c';`

Objetivo:
Determinar, com prova tecnica verificavel, se o estado atual do sistema resolveu 100% dos bugs, gaps, lacunas, ressalvas e riscos observaveis para esse usuario e para os fluxos diretamente relacionados a ele dentro da janela analisada.

Mandatos inegociaveis:
1. Nao implemente, nao corrija, nao altere configuracao, nao rode migracoes, nao reinicie servicos, nao mude dados e nao faça nenhuma acao destrutiva. Auditoria somente em modo leitura.
2. Nao assuma que algo foi resolvido. Toda afirmacao precisa de evidencia cruzada entre pelo menos 2 fontes quando isso for tecnicamente possivel.
3. Nao use linguagem vaga. Proibido dizer "parece", "provavelmente", "possivelmente", "deve estar", "aparenta" sem qualificador de evidencia.
4. Se nao houver prova suficiente para concluir "100% resolvido", a conclusao obrigatoria deve ser "nao comprovado".
5. Nao produza falso positivo. Quando houver incerteza, classifique como pendencia, risco residual, lacuna de observabilidade ou ausencia de evidencia.
6. O criterio de "0 gaps", "0 lacunas", "0 desvios" e "production-ready" so pode ser declarado se todas as verificacoes obrigatorias forem aprovadas com evidencia objetiva, reproduzivel e sem contradicoes.

Procedimento obrigatorio:

Fase 1 - Preparacao e contexto
1. Registrar data e hora de inicio da auditoria.
2. Identificar hostname, timezone do servidor, versao da aplicacao em execucao, hash/versao do deploy atual e principais servicos ativos.
3. Mapear onde estao logs, traces, metrics e credenciais de leitura do banco.
4. Registrar quaisquer limitacoes de acesso, retencao de logs, lacunas de tracing, ausencia de metricas, tabelas inacessiveis ou dados faltantes.

Fase 2 - Validacao do usuario-alvo
1. Executar a consulta base do usuario.
2. Confirmar se o registro encontrado corresponde exatamente ao `user_id` e ao e-mail informados.
3. Levantar entidades relacionadas ao usuario que impactem o fluxo de negocio analisado: onboarding, billing, tokens, eventos, jobs, mensagens, webhooks, subscriptions, claims e outros artefatos diretamente ligados ao caso real.
4. Registrar IDs, timestamps, status, correlacoes e chaves tecnicas relevantes para rastrear a jornada ponta a ponta.

Fase 3 - Auditoria de banco de dados
1. Inspecionar tabelas e registros ligados ao usuario dentro da janela de 168 horas e tambem os registros historicos necessarios para explicar o estado atual.
2. Verificar consistencia de estados, duplicidade, retries, eventos pendentes, falhas de outbox, dead letters, inconsistencias de relacionamento, status impossiveis e efeitos colaterais incompletos.
3. Comparar o estado persistido com o comportamento esperado do codebase atual.
4. Se detectar anomalia, registrar a consulta, o resultado, o horario e o impacto concreto.

Fase 4 - Auditoria de logs
1. Buscar logs por `user_id`, e-mail, correlation IDs, trace IDs, request IDs, subscription IDs e outros identificadores encontrados.
2. Identificar erros, warnings, retries excessivos, timeouts, panics, quedas de throughput, jobs nao executados, consumidores travados, eventos reprocessados e respostas HTTP inesperadas.
3. Relacionar cada anomalia ao fluxo de negocio e ao trecho do sistema responsavel.
4. Registrar evidencias textuais curtas e objetivas, com timestamp e origem.

Fase 5 - Auditoria de tracing
1. Localizar traces e spans vinculados ao caso real.
2. Verificar latencia, erros, spans faltantes, quebras de correlacao, retries anormais, dependencia externa instavel e fluxos incompletos.
3. Confirmar se o tracing suporta fechar o circuito causal entre entrada, decisao, persistencia e side effects.
4. Se o tracing nao permitir essa comprovacao, tratar isso como lacuna de observabilidade.

Fase 6 - Auditoria de metrics
1. Inspecionar metricas relevantes da janela: erros por endpoint/job/consumer, latencia, taxa de sucesso, filas, retries, backlog, outbox, throughput e saturacao.
2. Correlacionar picos, regressões ou instabilidades com o usuario ou com o fluxo ao qual ele pertence.
3. Validar se o comportamento agregado das metricas contradiz ou confirma logs, traces e banco.

Fase 7 - Auditoria do codebase atual
1. Identificar no servidor o commit, tag ou artefato exatamente em execucao.
2. Comparar o comportamento observado com o codebase atual correspondente, sem presumir que a branch local e igual ao deploy.
3. Mapear no codigo quais componentes deveriam impedir cada bug, gap ou ressalva relevante ao caso real.
4. Verificar se as protecoes esperadas existem de fato no codigo em execucao e se ha evidencia operacional de que funcionaram nos ultimos 7 dias.
5. Se existir protecao no codigo, mas sem evidencia operacional suficiente, marcar como "implementado mas nao comprovado em producao".

Fase 8 - Prova de resolucao
1. Construir uma matriz de verificacao com cada bug, gap, lacuna, ressalva ou risco identificado.
2. Para cada item, preencher obrigatoriamente:
   - descricao objetiva do problema
   - evidencia historica ou operacional do problema
   - mecanismo no codebase atual que supostamente resolve
   - evidencia tecnica de que o mecanismo esta em producao
   - evidencia tecnica de que funcionou corretamente na janela auditada
   - status final: `resolvido`, `nao resolvido`, `parcial`, `nao comprovado`
3. Nao consolidar itens diferentes em uma unica linha para esconder pendencias.
4. Qualquer contradicao entre banco, logs, traces, metrics e codigo derruba a conclusao de "100% resolvido".

Formato de saida obrigatorio em Markdown:

# Relatorio de Auditoria de Producao

## 1. Escopo auditado
- alvo
- janela exata analisada
- fontes consultadas
- versao/commit auditado

## 2. Metodologia aplicada
- como os dados foram coletados
- limites da auditoria
- validacoes cruzadas executadas

## 3. Identificacao do usuario e correlacoes
- resultado da consulta base
- entidades correlatas encontradas
- IDs tecnicos usados na investigacao

## 4. Evidencias por fonte
### 4.1 Banco de dados
### 4.2 Logs
### 4.3 Tracing
### 4.4 Metrics
### 4.5 Codebase/deploy em execucao

## 5. Matriz exaustiva de bugs, gaps, lacunas e ressalvas
Tabela obrigatoria com colunas:
`item | descricao | fonte_historica_ou_sintoma | evidencia_no_codigo | evidencia_em_producao | contradicoes | status_final`

## 6. Achados negativos
- tudo o que prova que ainda existe falha, incerteza, ausencia de prova, observabilidade insuficiente ou risco residual

## 7. Veredito final
Escolher exatamente uma opcao:
- `100% resolvido e comprovado`
- `nao 100% resolvido`
- `nao foi possivel comprovar 100%`

## 8. Criterios de aceite
Responder explicitamente:
- ha 0 gaps comprovados?
- ha 0 lacunas comprovadas?
- ha 0 desvios comprovados?
- ha 0 falso positivo nesta analise?
- esta production-ready com prova suficiente?

Regras finais de decisao:
- So responda `100% resolvido e comprovado` se nenhum item da matriz estiver como `nao resolvido`, `parcial` ou `nao comprovado`, e se nao existir nenhuma lacuna de observabilidade ou limitacao de acesso que comprometa a conclusao.
- Se faltar qualquer evidencia, a resposta obrigatoria e `nao foi possivel comprovar 100%`.
- Seja rigoroso, deterministico e auditavel. Sem flexibilidade interpretativa.
```

## Justificativas das adicoes

- Transformei "ultimos 7 dias" em "ultimas 168 horas" para remover ambiguidade temporal e forcar a faixa exata auditada.
- Converti "0 gaps" em criterio de aceite verificavel, nao em conclusao presumida, para evitar falso positivo.
- Impus modo somente leitura porque voce pediu analise, nao implementacao.
- Exigi correlacao entre banco, logs, tracing, metrics e deploy real para evitar conclusao baseada em uma unica fonte.
- Adicionei matriz de prova por item para impedir sumario superficial e obrigar rastreabilidade de cada bug ou lacuna.
- Forcei o status `nao comprovado` quando faltar evidencia, preservando o objetivo sem permitir declaracao vazia de pronto para producao.
