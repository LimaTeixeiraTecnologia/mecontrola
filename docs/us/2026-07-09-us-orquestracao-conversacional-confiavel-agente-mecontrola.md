# US-001: Orquestracao conversacional confiavel do agente MeControla

## Declaração
Como usuario do MeControla no WhatsApp, quero que o agente financeiro interprete, confirme, consulte e registre minhas solicitacoes com roteamento deterministico, validacao forte e respostas naturais em portugues, para controlar meu dinheiro com confianca sem registros duplicados, respostas inventadas, lacunas de confirmacao ou sensacao de chatbot mecanico.

## Contexto
- Problema: o agente tem arquitetura solida de Thread, Run, tools, workflows e memoria, mas concentra muitas regras criticas em um prompt monolitico e a producao mostra 23 runs em 7 dias com 19 sucessos, 4 falhas de usecaseError e scorers medios baixos de completude, acuracia de tool call e categorizacao.
- Resultado esperado: transformar regras criticas de roteamento, preenchimento de campos, confirmacao, anti-alucinacao, fallback e avaliacao em cadeia explicita, observavel e testavel antes e depois da chamada LLM, preservando fluidez conversacional e contratos publicos atuais.
- Fonte: solicitacao do usuario, codigo local em internal›agents e internal›platform, consultas agregadas ao Postgres de producao via SSH em 2026-07-09, logs recentes de containers e validacao local com go test -race.

## Regras de Negócio
- O agente deve manter linguagem de dominio financeiro pessoal atual: lancamentos, receitas, despesas, cartoes, orcamento, recorrencias, consulta mensal, confirmacao e pendencia conversacional.
- Cada acao financeira de escrita deve passar por tool e workflow de confirmacao quando aplicavel; o LLM nao pode afirmar sucesso sem retorno real de tool.
- O roteamento deve proteger operacoes criticas antes da geracao livre: multiplos lancamentos na mesma mensagem, compra em cartao com resolve_card obrigatorio, competencia de mes via monthRefKind, orcamento inexistente com offerCreatePrompt verbatim e follow-up sempre com nova consulta.
- A resposta ao usuario deve ser natural, curta, brasileira e pronta para WhatsApp, sem termos internos como workflow, thread, run, correlation, infraestrutura ou sistema interno.
- O agente deve registrar evidencia operacional por run, tool, outcome, duracao, erro sanitizado, scorer e fluxo pendente, sem labels de alta cardinalidade com user_id, thread_id, resource_id ou conteudo de mensagem.
- A evolucao deve preservar contrato publico de BuildMeControlaAgent, ferramentas existentes, AgentRuntime, schemas strict de tools e fluxos duraveis ja conectados no modulo.
- A evolucao nao pode remover, degradar ou alterar sem decisao explicita as funcionalidades existentes de registro de despesa, registro de receita, consulta mensal, consulta de orcamento, consulta de fatura, ultima transacao, busca de transacoes, cartoes, recorrencias, categorias, onboarding, pendencias conversacionais, confirmacao destrutiva, criacao de cartao, criacao de orcamento, memoria, scorers e entrega via WhatsApp.
- O padrao primario autorizado pela analise e Chain of Responsibility para organizar validadores e roteadores conversacionais sequenciais; State foi rejeitado como padrao primario porque os workflows de estado ja existem e o problema principal e encadear checks antes e depois do LLM.

## Plano de Evolução de Robustez e Eficiência
- Retirar regra critica do prompt monolitico e levar para codigo executavel: multi-item, competencia de mes, origem de cardId, confirmacao pendente, resposta verbatim, fallback seguro e bloqueio de sucesso sem tool devem ser validadores ou roteadores deterministas antes ou depois do LLM.
- Criar uma cadeia explicita de guardas conversacionais no padrao Chain of Responsibility, com handlers pequenos, ordenados, observaveis e testaveis, preservando a chamada publica de BuildMeControlaAgent e a composicao atual do modulo.
- Transformar roteamento em contrato verificavel: cada intencao financeira material deve ter caso golden com mensagem de entrada, tool esperada, argumentos esperados, outcome esperado e resposta esperada ou propriedade verificavel da resposta.
- Melhorar scorers para medir comportamento real em vez de keywords genericas: expected_tool, required_args, no_hallucination, verbatim_required, whatsapp_format, no_internal_terms, no_empty_answer, no_duplicate_write e month_reference_correctness.
- Fechar gaps operacionais do runtime: resposta vazia, truncamento por length, erro de append de mensagem, erro de update de run, run sem scorer, run sem mensagem persistida e tool de escrita com erro devem gerar metrica, log estruturado e criterio de alerta.
- Endurecer workflows e pendencias: pending entry, onboarding, confirmacao destrutiva, cadastro de cartao e criacao de orcamento devem cobrir retomada apos deploy, expiracao, cancelamento, mensagem repetida, WAMID duplicado, concorrencia e replay idempotente.
- Corrigir debito de governanca dos identificadores prefixados com underscore em Go de producao dentro de internal›agents›application›workflows, porque isso viola regra hard do repositorio e enfraquece a confianca nos gates.
- Reduzir custo e latencia evitando chamada LLM quando um guard deterministico puder responder com seguranca, especialmente multi-item, confirmacao pendente, cancelamento explicito, expiracao e dados obrigatorios ausentes conhecidos pela tool.
- Estabelecer thresholds iniciais de qualidade a partir da linha base produtiva coletada: taxa de run failed/usecaseError menor que a linha base de 4 falhas em 23 runs, tool-call accuracy acima da linha base 0,304, completeness acima da linha base 0,149 e categorization acima da linha base 0,565.
- Manter as funcionalidades existentes como contrato de regressao: a mudanca so e aceitavel quando o comportamento atual suportado por tools, workflows, memory, scorers e WhatsApp continuar coberto por teste ou evidencia operacional equivalente.
- Tornar o caminho production-ready apenas quando os gates locais, golden set, scorers e agregados produtivos confirmarem melhora objetiva sem regressao de contrato publico, privacidade, observabilidade ou fluidez conversacional.

## Critério Production-Ready
- Golden set deve passar cobrindo, no minimo, registro de despesa, registro de receita, consultas C1 a C7, cartoes, orcamento, recorrencias, onboarding, pendencias conversacionais, confirmacoes, follow-up, erro de tool, ambiguidade, formato WhatsApp e ausencia de termos internos.
- Funcionalidades existentes nao podem regredir: cada tool e workflow ja conectado no modulo deve manter contrato, nome, schema, outcome e comportamento observavel, salvo mudanca explicitamente aprovada em historia propria.
- Scorers devem medir comportamento verificavel, nao apenas presenca de palavras: tool esperada, argumentos obrigatorios, resposta verbatim quando exigida, formato WhatsApp, ausencia de alucinacao, ausencia de termos internos, resposta nao vazia, escrita nao duplicada e competencia de mes correta.
- Producao deve demonstrar melhora contra a linha base coletada: menos falhas que 4 em 23 runs, tool-call accuracy acima de 0,304, completeness acima de 0,149, categorization acima de 0,565, sem aumento de truncamento, escrita duplicada, resposta vazia ou falha silenciosa de persistencia de mensagem/run.
- Observabilidade deve permitir decisao operacional sem ler conteudo sensivel: run_id, agent_id, status, outcome, stage, tool, duracao, erro sanitizado, scorer_id, score, workflow e estado de pendencia devem estar disponiveis em log, metrica, trace ou consulta operacional.
- Gaps de governanca devem estar fechados no escopo afetado: identificadores prefixados com underscore em Go de producao de internal›agents devem ser corrigidos ou formalmente removidos do escopo antes de declarar production-ready.
- O uso produtivo so pode ser classificado como production-ready monitorado quando gates locais, golden set, scorers, logs, metricas e agregados produtivos confirmarem melhora objetiva sem perda de funcionalidades existentes, sem degradacao de privacidade e sem depender do prompt como unica defesa de comportamento critico.

## Critérios de Aceite
```gherkin
Cenário: bloqueio deterministico de multiplos lancamentos antes do LLM
  Dado uma mensagem de WhatsApp com dois valores monetarios ou dois itens de gasto distintos
  Quando o inbound chegar ao agente MeControla
  Então o agente deve responder exatamente a orientacao de lancamento unico
  E nao deve chamar register_expense, register_income, create_budget nem outra tool de escrita
  E deve registrar o outcome como clarify ou equivalente auditavel sem conteudo sensivel

Cenário: roteamento correto de consulta financeira com dados atualizados
  Dado uma mensagem como "como estou indo?" ou "qual foi minha ultima transacao?"
  Quando o agente decidir a acao
  Então deve chamar as tools exigidas pela matriz C1 a C7 antes de responder
  E deve usar somente dados retornados pelas tools na resposta final
  E deve reinvocar tool em follow-up financeiro em vez de responder de memoria

Cenário: escrita financeira exige confirmacao e idempotencia
  Dado uma mensagem com valor, descricao, data e forma de pagamento suficientes para uma despesa
  Quando register_expense retornar outcome=clarify com mensagem de confirmacao
  Então a resposta enviada ao usuario deve ser exatamente o campo message retornado pela tool
  E uma confirmacao posterior do usuario deve ser resolvida pelo workflow pendente sem nova chamada LLM de escrita duplicada
  E uma repeticao idempotente deve informar confirmacao sem criar novo registro

Cenário: compra no cartao nao usa cardId fabricado
  Dado uma mensagem de compra no cartao com apelido de cartao informado
  Quando o agente preparar a escrita ou consulta de fatura
  Então deve chamar resolve_card antes de register_expense ou query_card_invoice
  E deve usar exclusivamente o cardId retornado por resolve_card ou list_cards
  E deve pedir escolha quando resolve_card retornar found=false

Cenário: orcamento e competencia de mes nao inferem ano indevido
  Dado uma mensagem que cita um mes por nome sem ano
  Quando o agente chamar query_month, query_plan ou create_budget
  Então deve enviar monthRefKind=named_without_year com month preenchido e year ausente
  E deve repassar verbatim o clarifyPrompt quando a tool pedir o ano
  E deve exibir meses por extenso em portugues na resposta final

Cenário: falha de tool ou LLM nao vira sucesso aparente
  Dado que uma tool de escrita ou consulta retorne erro, resposta vazia ou truncamento por length
  Quando o runtime finalizar o run
  Então o run deve ficar failed ou com outcome de erro compativel
  E o WhatsApp deve receber fallback seguro, curto e sem detalhe tecnico
  E a resposta nao deve conter confirmacao de sucesso, valor inventado, categoria inventada ou status inventado

Cenário: avaliacao operacional detecta regressao conversacional
  Dado um conjunto golden de prompts reais e sinteticos cobrindo registro, consulta, orcamento, cartao, recorrencia, follow-up, erro e ambiguidade
  Quando testes e scorers forem executados em CI e ambiente produtivo
  Então tool-call accuracy, completude, categorizacao, taxa de falha, duracao p95 e truncamento devem ser medidos por versao do agente
  E o deploy deve bloquear quando qualquer threshold acordado cair abaixo da linha base aprovada
  E os resultados devem ser rastreaveis por run_id sem expor mensagem do usuario em metrica

Cenário: regra critica sai do prompt e vira guarda executavel
  Dado uma regra critica de seguranca conversacional hoje descrita apenas nas instructions do agente
  Quando a evolucao for implementada
  Então a regra deve existir como guarda, roteador, workflow ou validacao de tool com teste unitario ou golden correspondente
  E a instruction do agente deve ficar como reforco de linguagem, nao como unica defesa do comportamento

Cenário: funcionalidades existentes continuam funcionando
  Dado o comportamento atual de registros, consultas C1 a C7, cartoes, recorrencias, categorias, onboarding, pendencias, confirmacoes, memoria, scorers e entrega WhatsApp
  Quando a cadeia de guardas e scorers for introduzida
  Então os fluxos existentes devem continuar cobertos por teste automatizado, golden set ou evidencia operacional equivalente
  E nenhuma tool existente deve ser removida, renomeada, ocultada ou ter contrato alterado sem decisao explicita em historia propria

Cenário: gaps operacionais ficam observaveis e acionaveis
  Dado uma execucao com resposta vazia, truncamento, erro de persistencia de mensagem, erro de update de run ou scorer ausente
  Quando o runtime finalizar ou degradar a execucao
  Então deve existir metrica e log estruturado com agent_id, status, outcome, stage e erro sanitizado
  E a resposta ao usuario deve permanecer segura e sem detalhe tecnico

Cenário: workflows pendentes resistem a repeticao, expiracao e concorrencia
  Dado uma pendencia conversacional de lancamento, cartao, orcamento ou operacao destrutiva
  Quando o usuario repetir mensagem, confirmar duas vezes, cancelar, responder apos expiracao ou o sistema retomar depois de deploy
  Então o workflow deve produzir apenas um efeito financeiro valido
  E deve responder com texto deterministico para sucesso, cancelamento, expiracao ou repeticao idempotente

Cenário: thresholds produtivos comprovam melhora objetiva
  Dado a linha base produtiva de 19 runs succeeded, 4 failed, tool-call accuracy media 0,304, completeness media 0,149 e categorization media 0,565
  Quando a nova versao for observada em producao com volume minimo acordado
  Então a taxa de falha deve cair, os scorers devem subir e nenhum novo alerta critico de privacidade, truncamento ou escrita duplicada deve aparecer
  E a decisao de manter rollback ou promover a versao deve ser tomada por evidencia operacional, nao por impressao subjetiva
```

## Dados e Permissões
- Dados obrigatorios: resourceId opaco do usuario, threadId opaco do canal, messageId/WAMID, texto inbound, run_id, agent_id, tool calls, outcomes, duracao, status do workflow, scorers e erros sanitizados.
- Perfis/permissoes: usuario final autenticado via WhatsApp; agente so acessa dados financeiros do proprio resourceId; operadores tecnicos podem ver metricas, traces e agregados sanitizados; conteudo de mensagem em producao nao deve aparecer em dashboard de eficiencia sem politica explicita de privacidade.

## Dependências
- AgentRuntime, RunStore, ThreadGateway, MessageStore e WorkingMemory ja conectados.
- Tools financeiras existentes em internal›agents›application›tools e wiring em internal›agents›module.go.
- Workflows duraveis de pending entry, onboarding, confirmacao destrutiva, cadastro de cartao e criacao de orcamento.
- OpenRouter como provider unico via internal›platform›llm, sem fallback chain existente.
- Postgres de producao com platform_runs, platform_messages, workflow_runs e platform_scorer_results disponivel para agregados.
- Testes de pacote e gates de governanca Go devem continuar passando.
- Golden set versionado para intencoes financeiras materiais e regressao de formato WhatsApp.
- Dashboards ou consultas operacionais para runs, tool calls, scorers, workflows pendentes, truncamento e falhas sanitizadas.

## Fora de Escopo
- Trocar provider LLM, criar fallback chain ou adicionar outro vendor de LLM.
- Reescrever o substrato internal›platform›agent, memory, workflow, tool ou scorer.
- Alterar schema estrutural do Postgres sem historia propria de persistencia.
- Criar recomendacao bancaria, investimento, emprestimo, seguro ou imposto complexo fora do dominio financeiro pessoal do MeControla.
- Publicar ticket em Jira, Azure DevOps ou GitHub Issue nesta entrega.

## Evidências
- Entrada: o usuario pediu analise criteriosa de internal›agents›application›agents›mecontrola_agent.go, nota de eficiencia e robustez, uso real de producao por SSH, e uma unica US para elevar eficiencia, robustez, confiabilidade, anti-alucinacao e fluidez.
- Base de código: internal›agents›application›agents›mecontrola_agent.go:17 concentra regras criticas em prompt monolitico; internal›agents›application›agents›mecontrola_agent.go:253 monta o agente com MaxToolRounds e MaxTokens; internal›agents›application›agents›mecontrola_agent.go:271 aplica MultiItemGuard quando ha tool de registro; internal›agents›application›agents›multi_item_guard.go:63 bloqueia multiplos valores antes do LLM; internal›platform›agent›runtime.go:95 resolve Thread antes de Run; internal›platform›agent›runtime.go:167 persiste mensagens e fecha Run; internal›platform›agent›runtime.go:201 falha run com resposta vazia ou erro de tool; internal›agents›module.go:212 registra tools financeiras; internal›agents›module.go:233 cria runtime com write tool set; internal›agents›infrastructure›messaging›database›consumers›whatsapp_inbound_consumer.go:190 encadeia resumers antes do agente; internal›agents›application›scorers›mecontrola_scorers.go:172 registra scorers reais; internal›platform›tool›tool.go:95 valida schema antes do exec.
- Producao: SSH root@187.77.45.48 em 2026-07-09 confirmou containers server, worker, Postgres, pgbouncer e OTel saudaveis; Postgres mecontrola_db retornou em 7 dias 19 runs succeeded/routed e 4 failed/usecaseError, 37 mensagens user e 37 assistant, workflows onboarding e pending-entry concluidos, scorer categorization media 0,565, completeness media 0,149 e tool-call-accuracy media 0,304.
- Validacao local: go test -race -count=1 passou para ./internal›agents›application›agents, ./internal›platform›agent, ./internal›platform›tool e ./internal›platform›scorer; go build e go vet nos escopos centrais executados sem saida de erro.
- Inferências: a nota tecnica e eficiencia 7,5 de 10 e robustez 7,0 de 10; a arquitetura de runtime e boa, mas a efetividade conversacional observada e o volume de regras em prompt impedem nota maior.
- Não evidenciado: conteudo bruto de mensagens de producao nao foi consultado para compor esta historia; Prometheus direto via host nao retornou series na coleta, entao os agregados operacionais usados vieram do Postgres de producao e logs Docker recentes.

## Notas de Validação
- A historia foi mantida unica por pedido explicito do usuario, embora contenha fatias que normalmente poderiam virar historias menores.
- A decisao de design-patterns-mandatory retornou Chain of Responsibility como padrao primario e rejeitou State como primario por custo estrutural e sobreposicao com workflows existentes.
- Gate de governanca encontrou identificadores prefixados com underscore em Go de producao dentro de workflows de internal›agents, o que deve ser tratado na implementacao ou em tarefa tecnica acoplada a qualidade.
- A producao nao sustenta alegacao de zero falhas hoje; a historia define criterios para reduzir falhas e tornar regressoes detectaveis sem inventar garantia absoluta.
- A evolucao prioriza remover dependencia probabilistica do prompt em pontos criticos, medir a melhora com dados de producao e manter a conversa fluida usando respostas deterministicas apenas onde seguranca, idempotencia ou anti-alucinacao exigirem.
