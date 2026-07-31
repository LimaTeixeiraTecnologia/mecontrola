# Documento de Requisitos do Produto (PRD) — Lançamento e Edição de Receitas em Linguagem Natural

<!-- spec-version: 1 -->

## Visão Geral

O agente MeControla (WhatsApp) deve permitir que o usuário registre e corrija **receitas** escrevendo em linguagem natural, sem decorar comandos. O agente identifica a intenção (registrar ou editar receita), extrai valor, origem e data, pede apenas o que faltar, confirma com o usuário e persiste no ledger financeiro — sempre com o Tom de Voz oficial do MeControla.

O problema atual: a compreensão confiável de receita cobre só um subconjunto de frases (o guard determinístico dispara em `recebi`, `ganhei`, `caiu`, `entrou`, `salário`, `vendi`, `venda`, `serviço`); famílias inteiras de intenção (autônomos, comissões, aluguéis, investimentos) dependem do LLM sem prova de regressão dedicada; o valor por-extenso não é tratado no caminho de escrita; e três formas de data (`semana passada`, `mês passado`, `dia 10`) caem em fallback silencioso para o dia corrente, gravando data errada sem avisar.

Público: usuários pagantes do MeControla no WhatsApp (assalariados, autônomos, comerciantes, prestadores de serviço). Valor: reduzir atrito e erro no registro de entradas financeiras, aumentando fidelidade do histórico e confiança no controle.

## Objetivos

- Reconhecer receita escrita em linguagem natural em todas as famílias de intenção listadas, com prova de regressão via golden real-LLM (limiar ≥ 0,90, 0 falso-sucesso).
- Eliminar o fallback silencioso de data: `semana passada`, `mês passado` e `dia 10` passam a resolver deterministicamente para a data correta.
- Compreender valor em numérico, separador brasileiro, gíria (`conto`/`pila`/`mangos`) e por-extenso (simples e composto).
- Nunca gravar ou alterar receita sem confirmação humana explícita.
- Manter 100% das mensagens conformes ao catálogo oficial de mensagens (Tom de Voz inegociável).
- Métricas de sucesso: taxa de sucesso do gate golden por grupo de intenção ≥ 0,90; 0 caso de falso-sucesso; 0 receita gravada com data futura para entrada já recebida; 0 gravação/alteração sem confirmação.

## Histórias de Usuário

Fonte canônica (apêndice de rastreabilidade): `docs/us/2026-07-30-lancamento-e-edicao-de-receitas.md`.

- US-01 — Como usuário do MeControla no WhatsApp, quero informar qualquer entrada de dinheiro escrevendo do meu jeito, para que o agente entenda intenção, valor, origem e data, confirme e registre a receita.
- US-02 — Como usuário do MeControla no WhatsApp, quero corrigir uma receita já lançada em linguagem natural (valor, origem ou data), para que o agente localize o lançamento certo, confirme e atualize.

## Funcionalidades Core

1. **Compreensão ampla de intenção de receita (LLM-first).** O agente entende as famílias: recebimentos, trabalho, autônomos, comércio, produção artesanal, comissões/bônus, salário, aluguéis e investimentos. A amplitude é responsabilidade do LLM via a ferramenta de registro de receita; o guard determinístico permanece apenas como atalho de otimização do subconjunto já coberto.
2. **Extração de valor multi-forma.** Numérico, separador brasileiro, gíria e por-extenso (simples e composto), convertidos para centavos.
3. **Resolução de data determinística.** Formas já suportadas mais as três novas (`semana passada`, `mês passado`, `dia N`) com semântica fixa, sem fallback silencioso.
4. **Clarify mínimo.** Pede apenas o dado ausente (valor, origem, ou categoria/subcategoria) usando as mensagens oficiais; nunca repete dado já identificado.
5. **Confirmação obrigatória + sucesso motivacional.** Bloco de confirmação oficial de receita antes de gravar; mensagem de sucesso oficial após o `sim`.
6. **Edição conversacional de receita.** Localiza receitas compatíveis por valor/termo, desambigua quando há mais de um candidato, e atualiza somente após confirmação.

## Requisitos Funcionais

### Lançamento de receita (US-01)

- RF-01: O agente deve identificar a intenção de registrar receita a partir de linguagem natural, independentemente da forma de escrita, cobrindo no mínimo as nove famílias: (1) recebimentos, (2) trabalho, (3) profissionais autônomos, (4) comércio, (5) produção artesanal, (6) comissões e bônus, (7) salário, (8) aluguéis, (9) investimentos.
- RF-02: O agente deve extrair da mensagem, quando presentes, valor recebido, origem da receita e data, registrando a origem com o termo literal do usuário, nunca uma paráfrase.
- RF-03: O agente deve compreender valor numérico (`100`, `55 reais`), com separador brasileiro (`1.000`, `R$ 13.874,40`) e gíria (`100 conto`, `100 pila`, `100 mangos`), convertendo para centavos.
- RF-04: O agente deve compreender valor por-extenso simples (`cem`, `mil`, `um mil`, `dois mil`, `dez mil`) e composto (`mil e quinhentos`, `dois mil e quinhentos`), convertendo para centavos.
- RF-05: O agente deve resolver datas já suportadas deterministicamente: `hoje`, `ontem`, `anteontem`, dia da semana (`na sexta`, `no sábado`) e `DD/MM` (`12/08`).
- RF-06: O agente deve resolver deterministicamente, sem fallback silencioso para o dia corrente, as formas: `semana passada` = dia corrente menos 7 dias; `mês passado` = mesmo dia-do-mês do mês anterior, ajustado ao último dia válido quando o dia não existir; `dia N` (dia-do-mês isolado, ex.: `dia 10`) = a ocorrência mais recente do dia N que não seja futura (se o dia N do mês corrente ainda não chegou, usa o dia N do mês anterior).
- RF-07: Na ausência de data informada, a receita deve ser registrada com a data do dia corrente.
- RF-08: Se faltar valor ou origem, o agente deve solicitar apenas o dado ausente usando a pergunta determinística oficial do catálogo, e nunca pedir novamente um dado já identificado.
- RF-09: Quando a categoria/subcategoria da receita não puder ser resolvida pelo texto da origem, o agente deve pedir clarify de categoria antes de confirmar, usando as mensagens oficiais de opções de categoria; receita exige subcategoria folha.
- RF-10: Antes de gravar, o agente deve apresentar o bloco de confirmação oficial de receita (`💰 Valor`, `📥 Origem`, pergunta `Posso registrar?`), acrescentando a linha `📅 Data` somente quando a data for informada ou derivada.
- RF-11: O agente não deve gravar nenhuma receita sem confirmação explícita do usuário; confirmação (`sim`/`confirmar`/`ok`/`pode`) grava a receita uma única vez; cancelamento (`não`/`cancelar`) descarta sem efeito.
- RF-12: Após a gravação, o agente deve responder com a mensagem de sucesso oficial de receita (`Boa notícia! 🎉` seguida de frase motivacional do catálogo).
- RF-13: O registro deve ser idempotente por `wamid`/`itemSeq`; o reenvio da mesma mensagem não cria receita duplicada (replay).
- RF-14: Valor fora do intervalo permitido (≤ 0 ou > R$ 10.000.000,00) não deve gravar e deve retornar orientação de correção; origem vazia ou ilegível não deve gravar e deve pedir a origem em uma palavra.
- RF-15: Receita não deve capturar forma de pagamento; frases com canal (`recebi no cartão/débito/crédito`) são reconhecidas como receita, mas o canal não é persistido nem exibido na confirmação.
- RF-16: A compreensão ampla de RF-01, do por-extenso de RF-04 e das datas de RF-05/RF-06 deve seguir a arquitetura LLM-first pela ferramenta de registro de receita; o guard determinístico é apenas atalho de otimização e não pode ser o único caminho de compreensão.

### Edição de receita (US-02)

- RF-17: O agente deve identificar a intenção de editar receita a partir de linguagem natural, cobrindo no mínimo: `corrige aquela receita`, `corrige o lançamento`, `altera aquela entrada`, `atualiza a receita`, `o valor estava errado`, `era 150`, `na verdade foi 180`, `recebi mais`, `recebi menos`, `troca o valor`, `muda a data`, `foi ontem`, `foi semana passada`.
- RF-18: O agente deve localizar receitas compatíveis a partir dos critérios informados (valor atual e/ou termo/origem) quando o identificador do lançamento não for conhecido.
- RF-19: Havendo mais de um candidato compatível, o agente deve apresentar as opções para o usuário escolher; havendo apenas um, deve apresentá-lo para confirmação.
- RF-20: A atualização só deve ocorrer após confirmação explícita; antes disso o agente deve mostrar a nota de impacto descrevendo o que será alterado.
- RF-21: O valor novo e o critério de busca devem ser tratados como campos distintos: o valor que a receita passará a ter nunca deve ser confundido com o valor atual usado apenas para localizar.
- RF-22: A mudança de data na edição deve usar a mesma resolução determinística de RF-05/RF-06, sem fallback silencioso.
- RF-23: Após a confirmação, a atualização deve ser persistida uma única vez (idempotente por `wamid`/`itemSeq`) e o agente deve responder com a mensagem oficial de sucesso de edição seguida de frase motivacional do catálogo.
- RF-24: Se nenhuma receita compatível for encontrada, o agente deve informar isso sem alterar nada; se a correção informada for inválida (ex.: novo valor ≤ 0), o agente não deve persistir e deve orientar a correção.

### Qualidade e conformidade (transversais)

- RF-25: O gate de aceite deve incluir pelo menos um caso golden real-LLM por grupo de intenção (nove grupos de RF-01) mais casos-armadilha derivados de produção, exigindo limiar ≥ 0,90 e 0 caso de falso-sucesso; a mesma cobertura por grupo se aplica às formas de valor de RF-03/RF-04 e às datas novas de RF-06.
- RF-26: Todas as mensagens ao usuário devem seguir rigorosamente o catálogo oficial de mensagens (Tom de Voz); nenhuma mensagem verbatim pode ser reescrita fora do catálogo.
- RF-27: As métricas de observabilidade do fluxo devem manter cardinalidade controlada, sem `user_id` ou `category_id` como label.

## Experiência do Usuário

- Persona primária: usuário pagante no WhatsApp que registra entradas no dia a dia com linguagem informal e regional.
- Fluxo de lançamento: usuário descreve a receita → agente extrai valor/origem/data → pede apenas o que faltar (valor, origem ou categoria) → mostra bloco de confirmação → usuário responde `sim` → agente confirma sucesso com mensagem motivacional.
- Fluxo de edição: usuário pede a correção → agente localiza a receita (desambigua se houver mais de uma) → mostra nota de impacto → usuário confirma → agente confirma sucesso.
- Princípios de UX: nunca pedir dado já informado; nunca gravar/alterar sem confirmação; mensagens curtas, calorosas e motivacionais conforme catálogo; jamais registrar data futura para entrada já recebida.

## Restrições Técnicas de Alto Nível

- Canal único: WhatsApp (Meta) via o substrato de agente da plataforma; execução como Thread → Run auditável (governança R-AGENT-WF-001).
- Arquitetura LLM-first para compreensão semântica pela ferramenta de registro de receita; provider único OpenRouter; sem fallback chain.
- Catálogo oficial de mensagens é a fonte de verdade do Tom de Voz; adapters finos e zero comentários em Go de produção (R-ADAPTER-001).
- Modelagem de estado como tipos fechados (DMMF state-as-type) e regra de domínio isolada em funções `Decide*` puras; nenhuma regra de domínio em adapters (R-TXN-WORKFLOWS-001).
- Idempotência obrigatória por `wamid`/`itemSeq` para registro e edição; confirmação humana obrigatória antes de qualquer persistência.
- Métricas com cardinalidade controlada (sem `user_id`/`category_id` como label).
- Não introduzir captura de forma de pagamento em receita (mudança de modelo explicitamente fora de escopo — RF-15).

## Fora de Escopo

- Registro ou edição de múltiplos lançamentos em uma única mensagem (permanece a orientação de um lançamento por vez).
- Captura, confirmação ou edição de forma de pagamento em receita.
- Exclusão de receita (fluxo destrutivo próprio).
- Despesas, cartões, recorrências, orçamento e onboarding (fluxos próprios).
- Receita recorrente (mensagens com marcadores como `mensalmente`, `semanalmente`, `diariamente`).
- Alteração do gate de entitlement/assinatura do inbound WhatsApp.

## Suposições e Questões em Aberto

Nenhuma questão material em aberto. Decisões confirmadas nas rodadas de esclarecimento (2026-07-30):

- D1: Bloco de confirmação = catálogo oficial (`Posso registrar?`); linha `📅 Data` só quando informada (RF-10).
- D2: Mensagem de sucesso = catálogo oficial de receita; o texto de exemplo do briefing não substitui o catálogo (RF-12).
- D3: Compreensão ampla = LLM-first, com prova golden; guard é só atalho (RF-16).
- D4: Valor = numérico + separador BR + gíria (já suportados) + por-extenso simples e composto (RF-03, RF-04).
- D5: `semana passada` = hoje − 7 dias; `mês passado` = mesmo dia-do-mês do mês anterior (clamp ao último dia válido); `dia N` = ocorrência mais recente não-futura; suporte determinístico sem fallback silencioso (RF-06).
- D6: Receita resolve categoria/subcategoria; quando o texto não resolve, o agente pede clarify de categoria (RF-09).
- D7: Receita não captura forma de pagamento; limite explícito (RF-15).
- Faseamento: lançamento e edição na mesma iniciativa/PRD (entrega conjunta).
- Gate golden: ≥ 1 caso por grupo de intenção (nove grupos) + casos-armadilha de produção (RF-25).

## Apêndice — Rastreabilidade

- User Stories originais: `docs/us/2026-07-30-lancamento-e-edicao-de-receitas.md` (US-01 lançamento, US-02 edição).
- Evidência de código confrontada na US (ferramenta de registro de receita, guard, parser de valor/data, resolução de categoria income, bloco de confirmação e sucesso oficiais): ver seção `Evidências` de cada história.
