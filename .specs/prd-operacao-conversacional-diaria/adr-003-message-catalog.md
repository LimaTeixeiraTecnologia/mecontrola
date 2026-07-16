# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Catálogo central de mensagens determinísticas de tom de voz
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Solicitante do produto, engenharia de plataforma
- **Relacionados:** PRD `prd.md`; techspec `techspec.md`; RF-05; guard `verbatim_relay`

## Contexto

Hoje as mensagens ao usuário são geradas por builders espalhados por workflow (`buildConfirmSummary`, `buildWriteSuccessText`, `buildConfirmQuestion`, `buildCardCreateQuestion`, `successMessage`), em formato de linha única e sem frase motivacional, divergindo do documento oficial (blocos multi-linha com emojis e fechamento motivacional). O documento referencia como obrigatórios um "Manual Mestre" e um "Documento de Tom de Voz" que não foram fornecidos; os textos concretos, porém, estão embutidos no próprio documento. Precisamos de fonte única, testável e fiel ao tom.

## Decisão

Criar um **catálogo central de mensagens** (`internal/agents/application/messages/`) com funções puras que produzem os blocos verbatim (confirmação de despesa/receita, sucesso, esclarecimento, resumo por categoria e geral, informacionais) e sorteiam a frase motivacional por cenário a partir de listas fixas (rotação determinística por seed estável, sem `time.Now`/random no caminho de decisão). As mensagens chegam ao usuário via `ResponseText` (resume) ou via guard `verbatim_relay` (fluxo LLM).

O tom é ancorado em **duas fontes combinadas**: a seção de Identidade/Tom já implementada e validada em produção em `mecontrola_agent.go` serve de base, e os blocos verbatim do documento fornecido são sobrepostos como normativos. Em conflito, o texto verbatim do documento prevalece. Um scorer de aderência (`verbatim_tone_adherence`, code-based, e opcional LLM-judged) verifica a conformidade.

## Alternativas Consideradas

- **Só os textos embutidos no documento como fonte**: rejeitada por descartar o tom já validado em produção presente no prompt/guards, que cobre nuances não embutidas no documento.
- **Manter geradores por workflow**: rejeitada por perpetuar divergência e duplicação — exatamente o legado a eliminar.
- **Aguardar Manual Mestre/Tom de Voz externos**: rejeitada por não estarem disponíveis; a combinação prompt-atual + verbatim do documento cobre o tom sem bloquear a entrega.

## Consequências

### Benefícios Esperados

- Fonte única, testável por scorer; fim da divergência de tom; frase motivacional consistente.
- Preserva o tom já validado em produção e garante fidelidade verbatim ao documento.

### Trade-offs e Custos

- Combinar duas fontes exige regra de precedência clara (documento vence) para evitar ambiguidade.

### Riscos e Mitigações

- Risco: divergência entre o tom do prompt atual e o documento. Mitigação: precedência do documento + scorer de aderência que compara a saída aos blocos verbatim.

## Plano de Implementação

1. Extrair a seção de Identidade/Tom de `mecontrola_agent.go` como base documentada.
2. Implementar as funções do catálogo com os blocos verbatim do documento.
3. Implementar a rotação motivacional determinística por seed.
4. Ligar `verbatim_relay` aos campos das tools e adicionar o scorer de aderência.

## Monitoramento e Validação

- Scorer `verbatim_tone_adherence` (AlwaysSample); testes unitários comparando cada bloco ao documento.
- Sucesso: 100% dos blocos batem o documento; scorer de tom acima do piso.

## Impacto em Documentação e Operação

- Documentar o catálogo como fonte única de mensagens; atualizar o guia de tom.

## Revisão Futura

- Revisitar ao receber Manual Mestre/Documento de Tom de Voz oficiais, incorporando regras adicionais ao catálogo.
