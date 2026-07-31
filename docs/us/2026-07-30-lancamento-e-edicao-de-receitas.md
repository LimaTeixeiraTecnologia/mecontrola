# Lançamento e Edição de Receitas em Linguagem Natural — 2026-07-30

> Épico conversacional do agente MeControla (WhatsApp). Duas histórias independentes e entregáveis em separado, com a mesma persona e o mesmo tom de voz oficial.

## Decisões Confirmadas (rodadas de esclarecimento 2026-07-30)

- **D1 — Bloco de confirmação:** o catálogo oficial (`IncomeConfirmationBlock`) permanece a fonte de verdade; o texto da pergunta continua `Posso registrar?`. A linha `📅 Data` passa a ser exibida **apenas quando** a data for informada pelo usuário ou derivada da mensagem. Não se adota o texto alternativo do briefing (`Posso registrar essa receita?`).
- **D2 — Mensagem de sucesso:** a fonte de verdade é o catálogo oficial `WriteSuccess(WriteKindIncome, …)` (`Boa notícia! 🎉\n\n<motivacional> 💚`). O texto de exemplo do briefing (`🎉 Prontinho! Sua receita foi registrada com sucesso…`) é tratado apenas como exemplo e **não** substitui o catálogo (Tom de Voz oficial é inegociável).
- **D3 — Compreensão de linguagem natural:** LLM-first. A amplitude semântica (as famílias de intenção de receita) é responsabilidade do LLM via `register_income`, provada por casos golden real-LLM (≥ 0,90, 0 falso-sucesso). O guard determinístico permanece apenas como atalho de otimização do subconjunto já coberto — não é o mecanismo de comprensão.
- **D4 — Formas de valor:** obrigatório compreender numérico (`100`, `55 reais`), separador brasileiro (`1.000`, `R$ 13.874,40`) e gíria (`conto`, `pila`, `mangos`) — as três já cobertas deterministicamente por `reMoney` (write_shared.go:97) — **mais** por-extenso (`cem`, `mil`, `um mil`, `dois mil`, `dez mil`), que hoje **não** tem regex no caminho de escrita e é responsabilidade do LLM, exigido com prova golden real-LLM.
- **D5 — Datas sem suporte determinístico:** `semana passada`, `mês passado` e `dia 10` (dia-do-mês isolado) **não** são cobertos por `ParseInputDate` hoje e caem em fallback silencioso para o dia corrente (`resolveEntryDate`, register_entry.go:111). A US **exige estender `ParseInputDate`** para reconhecer essas formas deterministicamente, com gate de aceite, eliminando o fallback silencioso.
- **D6 — Categoria de receita:** o fluxo de escrita resolve categoria/subcategoria também para receita (`transactionDirection`→`income`, transaction_write_workflow.go:831) e o projeto exige subcategoria folha para income; quando a resolução por texto não fecha, o agente pede clarify de categoria. A US **modela esse turno** de clarify (reuso das mensagens de opções de categoria do catálogo).
- **D7 — Forma de pagamento em receita:** o sistema **não** captura canal de pagamento para receita (`register_income` não tem `paymentMethod`; `IncomeConfirmationBlock` só exibe Valor/Origem). Frases com canal (`recebi no cartão/débito/crédito`) são reconhecidas como receita, mas o canal **não** é persistido nem confirmado. A US fixa esse limite explicitamente; nenhuma mudança de modelo.

---

# US-01: Lançamento de receita em linguagem natural

## Declaração
Como usuário do MeControla no WhatsApp (assalariado, autônomo, comerciante ou prestador de serviço), quero informar qualquer entrada de dinheiro escrevendo do meu jeito — sem decorar comandos —, para que o agente entenda a intenção, o valor, a origem e a data, confirme comigo e registre a receita no meu controle financeiro.

## Contexto
- Problema: hoje o agente reconhece receitas com segurança apenas para um subconjunto de frases (o guard determinístico dispara só em `recebi`, `ganhei`, `caiu`, `entrou`, `salário`, `vendi`, `venda`, `serviço`). Famílias inteiras de intenção (`atendi cliente`, `prestei/fiz um freela`, `comissão caiu`, `recebi aluguel/dividendos/restituição/cashback`, `PLR`, `bônus`) dependem do LLM sem prova de regressão dedicada; valores por-extenso (`cem`, `dois mil`) não têm tratamento no caminho de escrita; e três formas de data (`semana passada`, `mês passado`, `dia 10`) caem em fallback silencioso para o dia corrente.
- Resultado esperado: o usuário informa a receita em linguagem natural, o agente extrai valor/origem/data, pede apenas o que faltar (inclusive categoria quando não resolver por texto), mostra o bloco de confirmação oficial e — após o `sim` — registra a receita e responde com a mensagem de sucesso oficial.
- Fonte: briefing do usuário (2026-07-30) + confronto com a base de código (`internal/agents`).

## Regras de Negócio
- RN-01: O agente deve identificar automaticamente a intenção de **registrar receita** a partir de linguagem natural, independentemente da forma de escrita, cobrindo no mínimo as famílias: recebimentos (`recebi`, `ganhei`, `entrou`, `entrou PIX`, `recebi uma TED/DOC/transferência`, `recebi no cartão/débito/crédito/dinheiro`), trabalho (`trabalhei`, `fiz/prestei/peguei/fechei/concluí um serviço`, `fiz/peguei um freela`, `fiz um trampo`), autônomos (`atendi cliente/paciente/consultoria`, `atendi unha/manicure/cabeleireiro/maquiagem/sobrancelha/depilação/fisioterapia/psicologia/personal/aula particular`), comércio e produção artesanal (`vendi`/`venda de …`, `fechei/concluí uma venda`, `vendi ovo de Páscoa/trufa/brownie/bolo/artesanato/crochê/caneca/camiseta`), comissões e bônus (`recebi/entrou comissão`, `comissão caiu`, `recebi/ganhei bônus`, `premiação`, `PLR`, `participação nos lucros`), salário (`recebi salário`, `salário/pagamento/folha caiu`, `recebi meu pagamento`), aluguéis (`recebi/entrou aluguel`, `recebi locação`) e investimentos (`recebi dividendos/juros/rendimento/lucro/cashback/restituição/reembolso`).
- RN-02: O agente deve extrair da mensagem, quando presentes: valor recebido, origem da receita (descrição da atividade), data. A origem registrada deve ser o **termo literal** usado pelo usuário (ex.: `salário`, `freela`), nunca uma paráfrase.
- RN-03: O agente deve compreender valor em formato numérico (`100`, `1000`, `55 reais`), com separador brasileiro (`1.000`, `R$ 13.874,40`) e gíria (`100 conto`, `100 pila`, `100 mangos`, `500 conto`) — formas já resolvidas deterministicamente por `reMoney` (write_shared.go:97) — e também por-extenso (`cem`, `mil`, `um mil`, `dois mil`, `dez mil`), que é resolvido pelo LLM e deve ser provado por golden real-LLM. O valor final é convertido para centavos.
- RN-04: O agente deve compreender datas em linguagem natural. Formas já cobertas deterministicamente por `ParseInputDate`: `hoje`, `ontem`, `anteontem`, dia da semana (`na sexta`, `no sábado`) e `DD/MM` (`12/08`). Formas a passar a cobrir deterministicamente (D5), sem fallback silencioso: `semana passada`, `mês passado` e `dia 10` (dia-do-mês isolado). Na ausência de data, a receita é do dia corrente.
- RN-05: Se faltar valor ou origem, o agente deve solicitar **apenas** o dado ausente, usando a pergunta determinística oficial do catálogo (`ClarificationQuestion`), e nunca pedir novamente um dado já identificado.
- RN-06: Quando a categoria/subcategoria da receita não puder ser resolvida pelo texto da origem, o agente deve pedir clarify de categoria antes de confirmar, usando as mensagens oficiais de opções de categoria do catálogo (`CategoryRootOptions`/`CategorySubcategoryOptions`/`CategoryLeafOptions`); income exige subcategoria folha (D6).
- RN-07: Nenhuma receita é gravada sem confirmação explícita do usuário. Antes de gravar, o agente apresenta o bloco de confirmação oficial `IncomeConfirmationBlock` (`💰 Valor` / `📥 Origem` / `Posso registrar?`), acrescentando a linha `📅 Data` somente quando a data for informada/derivada (D1).
- RN-08: Após confirmação (`sim`/`confirmar`/`ok`/`pode`), a receita é persistida uma única vez e o agente responde com a mensagem de sucesso oficial `WriteSuccess(WriteKindIncome, …)` — `Boa notícia! 🎉` seguida de frase motivacional do catálogo (D2). Cancelamento (`não`/`cancelar`) descarta sem efeito.
- RN-09: O registro é idempotente por `wamid`/`itemSeq`; reenvio da mesma mensagem não cria receita duplicada (replay).
- RN-10: Valor fora do intervalo permitido (≤ 0 ou > R$ 10.000.000,00) não grava e retorna orientação de correção; origem vazia/ilegível retorna pedido de origem em uma palavra, sem gravar.
- RN-11: A amplitude semântica de RN-01/RN-03 (por-extenso)/RN-04 é responsabilidade do LLM via `register_income` (arquitetura LLM-first, D3); o guard determinístico `register_income_shortcut` permanece apenas como atalho de otimização e não pode ser o único caminho de comprensão.
- RN-12: Receita **não** captura forma de pagamento; frases com canal (`recebi no cartão/débito/crédito`) são reconhecidas como receita, mas o canal não é persistido nem exibido na confirmação (D7).
- RN-13: As mensagens seguem rigorosamente o Tom de Voz oficial materializado em `internal/agents/application/messages/catalog.go`; nenhuma mensagem verbatim é reescrita fora do catálogo.

## Critérios de Aceite
```gherkin
Cenário: registra salário informado por extenso com confirmação
  Dado que sou um usuário ativo do MeControla no WhatsApp
  Quando eu envio "recebi meu salário de dois mil e quinhentos hoje"
  Então o agente identifica a intenção de receita
  E extrai valor R$ 2.500,00, origem "salário" e data de hoje
  E apresenta o bloco de confirmação oficial com "💰 Valor", "📥 Origem" e a pergunta "Posso registrar?"
  E não grava a receita até eu confirmar

Cenário: registra receita de família fora do guard, com gíria monetária
  Dado que sou um usuário ativo do MeControla no WhatsApp
  Quando eu envio "atendi uma cliente de unha de gel e recebi 100 pila"
  Então o agente identifica a intenção de receita via LLM
  E extrai valor R$ 100,00 e origem com o termo literal informado
  E apresenta o bloco de confirmação oficial de receita
  E ao eu responder "sim" grava a receita uma única vez
  E responde com a mensagem de sucesso oficial "Boa notícia! 🎉" seguida de frase motivacional do catálogo

Cenário: data por linguagem natural sem suporte hoje não cai silenciosamente para o dia corrente
  Dado que sou um usuário ativo do MeControla no WhatsApp
  Quando eu envio "recebi 300 de freela semana passada"
  Então o agente resolve a data para a semana anterior deterministicamente
  E a data exibida/registrada não é a do dia corrente
  E apresenta o bloco de confirmação oficial com a linha "📅 Data"

Cenário: pede clarify de categoria quando a origem não resolve
  Dado que sou um usuário ativo do MeControla no WhatsApp
  Quando eu envio "recebi 500 de dividendos hoje"
  E a origem não resolve uma subcategoria folha de receita
  Então o agente pergunta a categoria usando a mensagem oficial de opções de categoria
  E só apresenta a confirmação após eu escolher a subcategoria

Cenário: solicita apenas a origem ausente sem repetir dado já dado
  Dado que sou um usuário ativo do MeControla no WhatsApp
  Quando eu envio "caiu 100 conto na conta"
  Então o agente reconhece o valor R$ 100,00
  E pergunta apenas a origem usando a pergunta determinística oficial "Qual foi a origem dessa entrada? 📥"
  E não pergunta novamente o valor

Cenário: valor inválido não grava receita
  Dado que sou um usuário ativo do MeControla no WhatsApp
  Quando eu envio "recebi 0 reais de comissão hoje"
  Então o agente não grava a receita
  E responde com orientação verbatim de que o valor deve ser positivo e não ultrapassar R$ 10.000.000,00

Cenário: reenvio da mesma mensagem não duplica a receita
  Dado que eu já confirmei e registrei "recebi 300 de freela hoje"
  Quando a mesma mensagem (mesmo wamid) chega novamente
  Então o agente trata como replay
  E não cria uma segunda receita
```

## Dados e Permissões
- Dados obrigatórios: identidade inbound resolvida no contexto (`resourceId`/`userId`, `threadId`, `wamid`, `itemSeq`); valor em centavos (> 0 e ≤ 1.000.000.000); descrição/origem literal não vazia; subcategoria folha de receita (resolvida por texto ou por clarify); data opcional (default: dia corrente).
- Perfis/permissões: usuário com assinatura ativa habilitada ao agente (gate de entitlement do inbound WhatsApp já existente, fora do escopo desta US); nenhuma permissão administrativa envolvida.

## Dependências
- `register_income` (tool) e `usecases.RegisterIncomeCommand` — caminho de escrita de receita existente. Evidência: `internal/agents/application/tools/register_income.go:33`, `:113`.
- `transaction_write_workflow` — gate de confirmação HITL, direção income, mensagens de confirmação/sucesso e resolução/clarify de categoria. Evidência: `internal/agents/application/workflows/transaction_write_workflow.go:831`, `:1017`, `:1023`, `:998`.
- Parser de datas e resolução de data no caminho de escrita. Evidência: `internal/agents/application/workflows/write_shared.go:295`, `:270`; `internal/agents/application/usecases/register_entry.go:111`.
- Parser de valor (numérico/BR/gíria). Evidência: `internal/agents/application/workflows/write_shared.go:97`.
- Catálogo de mensagens oficial (`IncomeConfirmationBlock`, `WriteSuccess`, `ClarificationQuestion`, opções de categoria). Evidência: `internal/agents/application/messages/catalog.go:251`, `:258`, `:376`, `:347`.
- Harness golden real-LLM (`RUN_REAL_LLM=1` com credenciais OPENROUTER). Evidência: `internal/agents/application/golden/harness_realllm_test.go`.

## Fora de Escopo
- Registro de **múltiplos** lançamentos numa única mensagem (permanece a orientação verbatim de "um lançamento por vez"). Evidência: `internal/agents/application/golden/cases_expense_income.go:39`.
- Captura/confirmação de forma de pagamento para receita (D7): reconhecida como receita, mas o canal não é persistido.
- Despesas, cartões, recorrências, orçamento e onboarding (fluxos próprios).
- Alteração do gate de entitlement/assinatura do inbound WhatsApp.
- Receita recorrente (marcadores como `mensalmente`, `semanalmente`, `diariamente`): a mensagem com marcador de recorrência não é tratada como lançamento único aqui. Evidência: `internal/agents/.../agents/guards/register_income_shortcut.go:78`.

## Evidências
- Entrada: briefing do usuário "US — Lançamento e Edição de Receitas" (2026-07-30).
- Base de código:
  - Tool de receita com resolução de categoria por texto e descrição literal obrigatória; sem campo de forma de pagamento: `internal/agents/application/tools/register_income.go:15`, `:33`, `:107`.
  - Guard determinístico cobre só subconjunto de frases; valor numérico/BR no guard: `internal/agents/.../agents/guards/register_income_shortcut.go:75`, `:150`.
  - Parser de valor com gíria (`contos?|pilas?|mangos?`) já suportada: `internal/agents/application/workflows/write_shared.go:97`.
  - Parser de data: formas suportadas hoje (`hoje`/`ontem`/`anteontem`/dia-da-semana/`DD-MM`/ISO) e ausência das formas `semana passada`/`mês passado`/`dia N`: `internal/agents/application/workflows/write_shared.go:295`, `:270`.
  - Fallback silencioso de data para o dia corrente: `internal/agents/application/usecases/register_entry.go:111`.
  - Direção income no mesmo motor de categoria/candidatos: `internal/agents/application/workflows/transaction_write_workflow.go:831`, `:838`.
  - Bloco de confirmação de receita (Valor/Origem, sem pagamento; campo `DateFormatted` disponível): `internal/agents/application/messages/catalog.go:251`, `:106`.
  - Mensagem de sucesso oficial de receita e frases motivacionais: `internal/agents/application/messages/catalog.go:258`, `:151`.
  - Pergunta determinística de origem e opções de categoria: `internal/agents/application/messages/catalog.go:384`, `:347`.
  - Gate de confirmação e sucesso de receita no workflow: `internal/agents/application/workflows/transaction_write_workflow.go:1017`, `:1023`, `:998`.
  - Golden existente de receita (salário + separador de milhar), sem vocabulário amplo nem por-extenso: `internal/agents/application/golden/cases_expense_income.go:30`, `:88`.
- Inferências: a comprensão de por-extenso pode se apoiar no padrão já existente no prompt de onboarding (`onboarding_workflow.go:953`–`:987`); a cobertura ampla de intenção depende do LLM porque o guard não dispara para a maioria das famílias.
- Não evidenciado (busca executada, sem achado): golden real-LLM cobrindo `atendi/prestei/comissão caiu/aluguel/dividendos/restituição/cashback/PLR/bônus`; parsing por-extenso (`cem`/`mil`/`dois mil`) no caminho de escrita; suporte determinístico a `semana passada`/`mês passado`/`dia N` em `ParseInputDate`; linha `📅 Data` renderizada no `IncomeConfirmationBlock`.

## Notas de Validação
- Cobre fluxo feliz (registro por-extenso), variações válidas (família fora do guard + gíria; data NL sem suporte hoje; clarify de categoria) e erro (valor inválido) + replay.
- Fonte de verdade de confirmação/sucesso = catálogo oficial (D1/D2); a US não reescreve mensagens verbatim.
- Correção aplicada após re-auditoria 2026-07-30: gíria monetária (`conto`/`pila`/`mangos`) já é suportada por `reMoney` (write_shared.go:97) — retirada a afirmação anterior de "não evidenciado".
- Prova de regressão exigida: casos golden real-LLM (≥ 0,90, 0 falso-sucesso) para cada família de intenção de RN-01, para por-extenso de RN-03, e para as formas de data novas de RN-04, executados com `RUN_REAL_LLM=1`.

---

# US-02: Edição de receita em linguagem natural

## Declaração
Como usuário do MeControla no WhatsApp, quero corrigir uma receita já lançada escrevendo em linguagem natural (mudar valor, origem ou data), para que o agente localize o lançamento certo, confirme comigo e atualize sem eu precisar de comandos ou identificadores técnicos.

## Contexto
- Problema: correções de receita ("corrige aquela receita", "era 150", "na verdade foi 180", "recebi mais/menos", "muda a data", "foi ontem/semana passada") dependem inteiramente do LLM acionando `edit_entry` com busca por valor/termo, sem prova de regressão dedicada a essas frases; e a mudança de data herda a mesma limitação de RN-04 (formas `semana passada`/`mês passado`/`dia N` sem suporte determinístico hoje).
- Resultado esperado: o usuário descreve a correção; o agente localiza receitas compatíveis, apresenta opções se houver mais de uma (ou uma para confirmar), atualiza somente após o `sim` e responde com sucesso + motivação.
- Fonte: briefing do usuário (2026-07-30) + confronto com a base de código (`internal/agents`).

## Regras de Negócio
- RN-14: O agente deve identificar a intenção de **editar receita** a partir de linguagem natural, cobrindo no mínimo: `corrige aquela receita`, `corrige o lançamento`, `altera aquela entrada`, `atualiza a receita`, `o valor estava errado`, `era 150`, `na verdade foi 180`, `recebi mais`, `recebi menos`, `troca o valor`, `muda a data`, `foi ontem`, `foi semana passada`.
- RN-15: O agente deve localizar receitas compatíveis a partir dos critérios informados (valor atual e/ou termo/origem), usando a busca do `edit_entry` (`searchAmountCents`/`searchTerm`) quando o identificador do lançamento não é conhecido.
- RN-16: Havendo mais de um candidato compatível, o agente apresenta as opções para o usuário escolher; havendo apenas um, apresenta esse candidato para confirmação.
- RN-17: A atualização só ocorre após confirmação explícita do usuário; antes disso o agente mostra a nota de impacto (`needsConfirmation`) descrevendo o que será alterado.
- RN-18: O valor novo e o critério de busca são distintos: `amountCents` é o valor que a receita passará a ter; `searchAmountCents` é o valor atual usado só para localizar — nunca confundidos.
- RN-19: Mudança de data na edição usa o mesmo parser `ParseInputDate`; as formas `semana passada`/`mês passado`/`dia N` seguem o requisito determinístico de D5 (sem fallback silencioso para o dia corrente).
- RN-20: Após a confirmação, a atualização é persistida uma única vez (idempotente por `wamid`/`itemSeq`) e o agente responde com a mensagem oficial de sucesso de edição `EditSuccess` seguida de frase motivacional do catálogo.
- RN-21: Se nenhuma receita compatível for encontrada, o agente informa isso sem alterar nada; se a correção informada for inválida (ex.: novo valor ≤ 0), o agente não persiste e orienta a correção.
- RN-22: As mensagens seguem o Tom de Voz oficial do catálogo (`internal/agents/application/messages/catalog.go`); nenhuma mensagem verbatim é reescrita fora do catálogo.

## Critérios de Aceite
```gherkin
Cenário: corrige o valor de uma única receita compatível
  Dado que registrei "recebi 150 de freela hoje"
  Quando eu envio "corrige aquela receita, na verdade foi 180"
  Então o agente localiza a receita compatível pelo termo/valor atual
  E apresenta a nota de impacto indicando o novo valor R$ 180,00
  E não persiste até eu confirmar
  E ao eu responder "sim" atualiza a receita uma única vez
  E responde com a mensagem oficial de sucesso de edição seguida de frase motivacional do catálogo

Cenário: desambigua quando há mais de uma receita compatível
  Dado que registrei duas receitas de R$ 200,00 no mês
  Quando eu envio "troca o valor daquela receita de 200 pra 250"
  Então o agente apresenta as opções compatíveis para eu escolher
  E só prossegue com a edição após eu indicar qual receita
  E atualiza apenas após minha confirmação

Cenário: muda a data para forma sem suporte determinístico hoje
  Dado que registrei "recebi comissão de 300 hoje"
  Quando eu envio "muda a data daquela comissão, foi semana passada"
  Então o agente localiza a receita compatível
  E resolve a data para a semana anterior deterministicamente, sem cair para o dia corrente
  E apresenta a nota de impacto com a nova data
  E atualiza somente após minha confirmação

Cenário: nenhuma receita compatível encontrada
  Dado que não tenho receitas de "aluguel" no período
  Quando eu envio "corrige a receita de aluguel, era 1600"
  Então o agente informa que não encontrou receita compatível
  E não altera nenhum lançamento
```

## Dados e Permissões
- Dados obrigatórios: identidade inbound no contexto (`resourceId`/`userId`, `threadId`, `wamid`, `itemSeq`); ao menos um critério de localização (identificador conhecido, ou `searchAmountCents`/`searchTerm`); o(s) campo(s) novo(s) a aplicar (valor, origem/descrição, data).
- Perfis/permissões: usuário com assinatura ativa habilitada ao agente; edição restrita às próprias receitas (isolamento por `userId` já garantido no use case). Nenhuma permissão administrativa envolvida.

## Dependências
- `edit_entry` (tool) e `usecases.EditEntryCommand` com busca por valor/termo e gate de confirmação. Evidência: `internal/agents/application/tools/edit_entry.go:35`, `:72`, `:127`.
- `transaction_write_workflow` — gate de confirmação e mensagem de sucesso de edição. Evidência: `internal/agents/application/workflows/transaction_write_workflow.go:994`, `:1019`.
- Parser de data compartilhado (mesma extensão exigida por D5). Evidência: `internal/agents/application/workflows/write_shared.go:295`.
- `search_transactions` (tool) para casos em que a busca precisa expor candidatos antes da edição. Evidência: `internal/agents/application/tools/search_transactions.go`.
- Harness golden real-LLM (`RUN_REAL_LLM=1`). Evidência: `internal/agents/application/golden/harness_realllm_test.go`.

## Fora de Escopo
- Exclusão de receita (fluxo destrutivo próprio, `destructive_manage_workflow`).
- Edição de despesa, cartão, recorrência e orçamento (fluxos próprios).
- Edição em lote de múltiplas receitas numa única mensagem.
- Captura/edição de forma de pagamento em receita (D7).

## Evidências
- Entrada: briefing do usuário "US — Lançamento e Edição de Receitas" (2026-07-30).
- Base de código:
  - Tool de edição aceita valor/descrição/categoria/data e busca por `searchAmountCents`/`searchTerm` quando o id é desconhecido; persistência só após confirmação: `internal/agents/application/tools/edit_entry.go:35`, `:72`, `:127`.
  - Distinção explícita entre valor novo e critério de busca no schema da tool: `internal/agents/application/tools/edit_entry.go:43`, `:50`.
  - Sucesso de edição e mapeamento por operação no workflow: `internal/agents/application/workflows/transaction_write_workflow.go:994`, `:1019`.
  - Parser de data compartilhado com as limitações de D5: `internal/agents/application/workflows/write_shared.go:295`.
- Inferências: as frases de edição de receita são resolvidas pelo LLM acionando `edit_entry`; a desambiguação de múltiplos candidatos usa a busca da própria tool/`search_transactions`.
- Não evidenciado (busca executada, sem achado): casos golden real-LLM cobrindo especificamente as frases de edição de receita ("corrige aquela receita", "era 150", "recebi mais/menos", "muda a data", "foi ontem/semana passada").

## Notas de Validação
- Cobre fluxo feliz (edição de valor de candidato único), variações válidas (desambiguação de múltiplos candidatos e mudança de data sem suporte hoje) e erro/bloqueio (nenhum candidato compatível).
- Fonte de verdade de sucesso/confirmação = catálogo oficial; a US não reescreve mensagens verbatim.
- Prova de regressão exigida: casos golden real-LLM (≥ 0,90, 0 falso-sucesso) para as frases de edição de RN-14 e para a mudança de data de RN-19, executados com `RUN_REAL_LLM=1`.
