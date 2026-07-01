# Prompt Mandatório — Correção de Formatação e Emojis do MeControla Agent no WhatsApp

> Gerado via skill `prompt-enricher` em 2026-07-01. Este arquivo é o prompt enriquecido, pronto para ser usado como instrução de execução em uma tarefa futura de bugfix. **Nenhuma implementação foi realizada na criação deste arquivo.**

---

## 1. Contexto Obrigatório (ler antes de qualquer ação)

- Arquivo alvo: `internal/agents/application/agents/mecontrola_agent.go` (81 linhas), especificamente a constante `mecontrolaAgentInstructions` (linhas 14-61) consumida por `BuildMeControlaAgent`.
- Canal afetado: respostas do agente MeControla entregues via WhatsApp.
- `AGENTS.md` e `.agents/skills/agent-governance/SKILL.md` são a fonte canônica de governança deste repositório e DEVEM ser lidos antes de qualquer edição.
- Este é código Go de produção: `.agents/skills/go-implementation/SKILL.md` é obrigatório para qualquer alteração (Etapas 1-5, regras R0-R7).
- Esta é uma correção de defeito de comportamento (prompt/formatação), não uma feature nova: `.agents/skills/bugfix/SKILL.md` deve orquestrar o fluxo (causa raiz + teste de regressão + evidência).
- Zero comentários em código Go de produção é regra `[HARD]` inegociável neste repositório — não se aplica ao texto da constante de instructions (que é conteúdo de prompt, não comentário de código), mas se aplica a qualquer código Go novo escrito na correção.
- É proibido usar `_ = variavel` para silenciar parâmetro/import não utilizado (convenção registrada do repositório).

## 2. Evidência do Defeito (produção real, WhatsApp)

Duas falhas foram observadas na mesma resposta, geradas a partir das instructions atuais:

### 2.1 Emojis definidos, mas não usados
A seção `## Emojis` das instructions (linhas 26-34) define e exige 6 emojis contextuais (`✅ 💰 📊 🎯 ⚠️ 💡`). Na resposta real de WhatsApp analisada, **nenhum emoji foi usado**, nem no texto explicativo da metodologia de orçamento, nem no resumo de onboarding, nem na pergunta de confirmação final.

### 2.2 Negrito quebrado / truncamento de texto
A resposta real usa `*Custo Fixo*` (asterisco simples, correto para negrito no WhatsApp), mas o texto foi cortado no meio da explicação da categoria "Metas" (evidência literal: "...garantir que você está progred" — palavra incompleta, frase incompleta, e as categorias 5 (Liberdade Financeira) nunca chegou a ser explicada apesar de aparecer depois no resumo de onboarding).

**Texto real da resposta truncada (íntegra, para reprodução):**
```
1. *Custo Fixo*: Esta categoria abrange todas as despesas que você tem todo mês e que não podem ser evitadas, como aluguel, contas de luz, água, internet, e outras despesas essenciais. É importante garantir que esses custos estejam sempre cobertos.

2. *Conhecimento*: Aqui, você destina uma parte do seu orçamento para investir em sua educação e desenvolvimento pessoal. Isso pode incluir cursos, livros, workshops ou qualquer outra forma de aprendizado que ajude a melhorar suas habilidades e conhecimentos.

3. *Prazeres*: Esta categoria é dedicada ao seu bem-estar e lazer. É o dinheiro que você reserva para se divertir, como sair para jantar, viajar ou praticar hobbies. É fundamental para manter um equilíbrio saudável entre trabalho e vida pessoal.

4. *Metas*: Nesta categoria, você define um percentual do seu orçamento para alcançar objetivos financeiros específicos, como comprar um carro, fazer uma viagem dos sonhos ou economizar para a aposentadoria. É uma forma de garantir que você está progred[TRUNCADO]

*Resumo do Onboarding:*
- *Objetivo Financeiro:* Não foi fornecido um objetivo financeiro claro.
- *Renda Mensal:* R$8.000,00
- *Cartão de Crédito:* Não informado (dia 0)
- *Distribuição de Despesas:*
  - Conhecimento: 20%
  - Custo Fixo: 30%
  - Liberdade Financeira: 20%
  - Metas: 10%
  - Prazeres: 20%

Por favor, confirme se deseja ativar o orçamento com as informações acima.
```

### 2.3 Pista de causa raiz já localizada no código (investigar, NÃO assumir como solução fechada)
- `internal/platform/llm/openrouter.go:24` define `defaultMaxTokens = 256`.
- `internal/platform/llm/openrouter.go:341-353` e `internal/platform/llm/openrouter.go:643-648` (`resolveMaxTokens`) mostram que, quando a requisição não define `MaxTokens`, o valor cai para 256 tokens — insuficiente para uma resposta educativa de 5 categorias + resumo de onboarding + pergunta de confirmação.
- `internal/platform/agent/ports.go:24` e `internal/platform/agent/agent.go:94,209` mostram que `MaxTokens` é repassado de `agent.CompletionInput`/config até o provider, então o ponto de configuração pode estar em `BuildMeControlaAgent` (sem `agent.AgentOption` de max tokens) ou em configuração upstream do provider.
- Hipótese a validar: o truncamento em 256 tokens é a causa raiz da quebra de formatação (o corte ocorre no meio de uma frase, e não em um limite de token de emoji), e a ausência de emojis pode ser causa correlata (o modelo pode estar reservando tokens para completar o conteúdo textual e nunca chega a inserir os emojis) ou pode ser uma causa independente nas instructions/prompt-engineering.
- Esta hipótese DEVE ser confirmada ou refutada com evidência (teste reproduzindo a chamada real ao LLM, ou análise do `finish_reason`/`stop_reason` retornado pelo provider) antes de qualquer correção. Não implementar a correção assumindo a hipótese como verdadeira sem essa validação.

## 3. Objetivo da Tarefa (o que a implementação futura DEVE entregar)

1. Identificar a causa raiz exata de:
   a. Truncamento de resposta que quebra negrito e corta frases no meio.
   b. Ausência de emojis nas respostas reais, apesar de definidos nas instructions.
2. Corrigir a causa raiz (não o sintoma) com a menor mudança segura possível, preservando arquitetura, camadas (`domain`/`application`/`infrastructure`) e convenções existentes do módulo `internal/agents` e `internal/platform/{agent,llm}`.
3. Garantir que respostas do MeControla Agent no WhatsApp:
   - Nunca sejam truncadas no meio de uma frase, categoria ou lista.
   - Sempre usem os emojis definidos na seção `## Emojis` de forma contextual e consistente, conforme as regras já especificadas (✅ confirmações, 💰 valores, 📊 resumos/planos, 🎯 metas, ⚠️ alertas, 💡 dicas).
   - Preservem a sintaxe de negrito compatível com WhatsApp (`*texto*`, asterisco simples — nunca `**texto**` markdown duplo).
4. Adicionar teste de regressão que comprove a correção (ver seção 5).

## 4. Restrições Mandatórias (zero desvio, zero flexibilização)

- **NÃO implementar nada nesta etapa de enriquecimento de prompt.** Este arquivo é apenas o prompt para uma execução futura via `bugfix` ou `execute-task`.
- Quando a implementação futura ocorrer, ela DEVE:
  - Carregar `.agents/skills/agent-governance/SKILL.md`, `.agents/skills/go-implementation/SKILL.md` e `.agents/skills/bugfix/SKILL.md` antes de editar.
  - Seguir R-AGENT-WF-001 e R-ADAPTER-001 quando a correção tocar `internal/platform/agent` ou `internal/platform/llm` (zero regra de negócio em adapters, zero comentários em código Go de produção).
  - Não usar `panic` em nenhuma alteração.
  - Não usar `_ = variavel` para silenciar parâmetro ou import não utilizado.
  - Não introduzir abstrações, camadas, flags ou dependências novas sem necessidade concreta comprovada pela causa raiz identificada.
  - Não alterar o tom, identidade ou regras de comunicação já corretas da constante `mecontrolaAgentInstructions` (linhas 18-24, 36-43, 45-60) — a correção deve ser cirúrgica, restrita ao necessário para resolver truncamento e uso de emojis (ex.: ajuste de `MaxTokens`, ajuste de reforço textual da regra de emojis, ou ambos, conforme a causa raiz confirmada).
  - Validar com, no mínimo: `gofmt -w` nos arquivos alterados, `go build`, `go vet` e `go test -race -count=1 ./internal/agents/... ./internal/platform/agent/... ./internal/platform/llm/...` (escopo proporcional aos pacotes tocados).
  - Reportar explicitamente qualquer suposição, drift ou bloqueio encontrado durante a implementação.
- **Zero falso positivo**: a correção só é considerada válida se houver teste automatizado reproduzindo o cenário de resposta longa (5 categorias + resumo de onboarding) e confirmando que (a) a resposta não é truncada e (b) os emojis esperados aparecem na saída, ou evidência equivalente de que a causa raiz foi eliminada.
- **Zero lacuna**: se a causa raiz envolver múltiplos fatores (ex.: `MaxTokens` insuficiente E reforço insuficiente da regra de emojis nas instructions), ambos devem ser corrigidos na mesma entrega — não é aceitável corrigir apenas um e declarar a tarefa concluída.

## 5. Critérios de Aceitação (mensuráveis)

- [ ] Causa raiz do truncamento documentada com evidência (log, teste, ou trace do provider) — não apenas suposição.
- [ ] Causa raiz da ausência de emojis documentada com evidência.
- [ ] Resposta simulada/testada com o mesmo conteúdo do cenário real (5 categorias de orçamento + resumo de onboarding + pergunta de confirmação) é gerada sem truncamento.
- [ ] Resposta simulada/testada contém pelo menos os emojis contextualmente esperados pela regra `## Emojis` (ex.: 📊 no resumo, ✅ ou 🎯 na pergunta de confirmação, conforme o mapeamento definido).
- [ ] Todo negrito na resposta usa `*texto*` (asterisco simples), nunca `**texto**` nem negrito quebrado por corte de token.
- [ ] Teste de regressão automatizado adicionado em `internal/agents/application/agents/` ou `internal/platform/llm/` (local mais apropriado à causa raiz), cobrindo o cenário de falha original.
- [ ] `go build`, `go vet` e `go test -race -count=1` passam nos pacotes alterados.
- [ ] Nenhuma regra de tom, identidade, ou regra de comunicação pré-existente foi alterada sem necessidade.
- [ ] Nenhum comentário adicionado a código Go de produção (exceto exceções documentadas: `// Code generated`, `//go:`, `//nolint:` com justificativa).

## 6. Formato de Saída Esperado da Implementação Futura

- Diff cirúrgico em Go (e, se aplicável, ajuste textual na constante `mecontrolaAgentInstructions`).
- Teste(s) de regressão em Go, seguindo o padrão de testes já existente no pacote (`mecontrola_agent_test.go`, `mecontrola_agent_realllm_test.go`, `mecontrola_agent_e2e_test.go` como referência de estilo, sem copiar cegamente).
- Relatório final com: causa raiz confirmada, mudança aplicada, comandos de validação executados e resultado.

## 7. Ambiguidades Identificadas (a resolver antes ou durante a implementação futura)

1. Não está confirmado se o `MaxTokens` de 256 é de fato o gargalo, ou se existe outro limite (ex.: configuração do modelo, truncamento no client HTTP, ou at nível de webhook do WhatsApp) — a implementação futura deve investigar antes de alterar `defaultMaxTokens` ou `BuildMeControlaAgent`.
2. Não está confirmado se a ausência de emojis é causada pelo mesmo truncamento (modelo não chega a gerar os emojis por falta de espaço) ou se é uma falha independente de aderência às instructions pelo modelo (necessitando reforço textual, exemplos few-shot, ou ajuste de temperatura/params do provider).
3. Não há confirmação sobre qual valor de `MaxTokens` seria adequado — isso deve ser dimensionado com base no tamanho típico de respostas esperadas (ex.: resposta completa de onboarding), não arbitrado sem análise.

---

**Prompt original do usuário (preservado para rastreabilidade):**

> O internal/agents/application/agents/mecontrola_agent.go no whatsapp, não usou os emojis permitidos e quebrou *Custo Fixo* nos negritos [...] Eu quero melhorar deixar mais robusto, eficiente, sem desvios, sem flexibilidade, 0 gaps, 0 lacunas, 0 falso positivo.
