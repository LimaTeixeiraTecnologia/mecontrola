# Generated: 2026-08-04T14:07:33Z

# Relatório de Execução — Correção de Regressão de Dependência (fora do numeramento de tasks.md)

## Contexto

- Escopo: correção cirúrgica de `go.mod`/`go.sum` para destravar `go build ./...` no repositório
  inteiro, bloqueando as tarefas pendentes 4.0, 6.0 e 8.0 do PRD
  `.specs/prd-observabilidade-golden-signals-otel` (que dependem de build limpo).
- Esta correção NÃO é uma tarefa numerada do PRD; `tasks.md` não foi alterado.
- Nenhum arquivo `.go` foi tocado — apenas `go.mod` e `go.sum`.

## Comandos Executados

- Preflight de profundidade/skills:
  ```
  export AI_INVOCATION_DEPTH=0
  source .agents/lib/check-invocation-depth.sh
  export AI_PREFLIGHT_DONE=1
  ```
- `git status` / `git log --oneline -5` -> estado inicial confirmado (branch `main`, múltiplos
  arquivos de trabalho pré-existente das tarefas 1.0–7.0 não relacionados a esta correção).
- `git diff HEAD -- go.mod` (estado ANTES da correção) -> confirmou:
  - `go.opentelemetry.io/otel v1.45.0` no bloco `require` DIRETO do módulo raiz.
  - `go.opentelemetry.io/otel/sdk/metric v1.45.0` também direto.
  - `go.opentelemetry.io/otel/log v0.21.0 // indirect`, `sdk/log v0.21.0 // indirect`,
    `otlplog/otlploggrpc v0.21.0 // indirect`, `otlplog/otlploghttp v0.21.0 // indirect` no bloco
    indirect (o comentário `// indirect` já estava presente no go.mod antes da correção, ao
    contrário do que a hipótese inicial supunha — ver seção "Achado sobre o Diagnóstico" abaixo).
  - Família `otel*` core (v1.44.0 -> v1.45.0), `otlpmetric*`, `otlptrace*` (v1.44.0 -> v1.45.0)
    também bumpados.
- `go mod graph | grep 'otel/log@v0.21.0' | grep '^github.com/LimaTeixeiraTecnologia/mecontrola '`
  -> retornou `github.com/LimaTeixeiraTecnologia/mecontrola go.opentelemetry.io/otel/log@v0.21.0`,
  confirmando que o módulo raiz é a origem direta da seleção de versão v0.21.0 no grafo MVS (Minimal
  Version Selection do Go modules), independentemente do rótulo `// indirect` no comentário do
  go.mod (esse rótulo reflete apenas se o pacote é importado diretamente pelo código Go do módulo,
  não se a *versão* foi escolhida diretamente pelo `require` do módulo raiz).
- `go list -m -f '{{.Version}} {{.GoVersion}}' go.opentelemetry.io/contrib/bridges/otelslog`
  -> `v0.19.0 1.25.0` — confirmado que v0.19.0 é a versão resolvida (e, por pesquisa do proxy
  Go, a mais recente publicada) e que ela depende de `go.opentelemetry.io/otel/log` em uma API
  anterior a `log.KeyValue`/`log.Value`/`log.BoolValue`/etc. introduzidos apenas em v0.21.0.
- `go build ./...` (ANTES da correção) -> **FALHOU** com:
  ```
  # go.opentelemetry.io/contrib/bridges/otelslog
  .../otelslog@v0.19.0/handler.go:380:37: undefined: log.KeyValue
  .../otelslog@v0.19.0/handler.go:413:13: undefined: log.KeyValue
  .../otelslog@v0.19.0/handler.go:438:41: undefined: log.KeyValue
  .../otelslog@v0.19.0/handler.go:490:32: undefined: log.Value
  .../otelslog@v0.19.0/convert.go:21:30: undefined: log.Value
  .../otelslog@v0.19.0/convert.go:25:14: undefined: log.BoolValue
  .../otelslog@v0.19.0/convert.go:27:14: undefined: log.StringValue
  .../otelslog@v0.19.0/convert.go:29:14: undefined: log.Int64Value
  .../otelslog@v0.19.0/convert.go:31:14: undefined: log.Int64Value
  .../otelslog@v0.19.0/convert.go:123:37: undefined: log.Value
  ```
- `go get go.opentelemetry.io/otel/log@v0.20.0 go.opentelemetry.io/otel/sdk/log@v0.20.0 go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc@v0.20.0 go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp@v0.20.0`
  -> saída:
  ```
  go: downgraded go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.21.0 => v0.20.0
  go: downgraded go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.21.0 => v0.20.0
  go: downgraded go.opentelemetry.io/otel/log v0.21.0 => v0.20.0
  go: downgraded go.opentelemetry.io/otel/sdk/log v0.20.0
  ```
- `go build ./...` (APÓS o downgrade) -> saída vazia (PASS, repositório inteiro).
- `go vet ./...` -> saída vazia, `EXIT_VET=0` (PASS, repositório inteiro).
- Avaliação da família `otel` core v1.45.0 (Etapa 3 do mandato): como `go build ./...` e
  `go vet ./...` ficaram limpos SEM tocar a família core (`otel`, `otel/sdk`, `otel/sdk/metric`,
  `otel/metric`, `otel/trace`, `otlpmetric*`, `otlptrace*` permaneceram em v1.45.0), **não foi
  aplicado downgrade adicional** para v1.44.0 — decisão baseada em evidência real de build/vet
  limpos, conforme mandato explícito de não supor incompatibilidade sem prova.
- `go mod tidy` -> saída vazia, `EXIT_TIDY=0`. `git diff HEAD -- go.mod` após o tidy confirmou que
  as versões v0.20.0 dos 4 pacotes de log foram preservadas (não re-bumpadas pelo tidy), e que a
  família core permaneceu em v1.45.0 (nenhuma mudança adicional introduzida pelo tidy além de
  atualizações menores de dependências indiretas de terceiros já presentes no diff original —
  `getkin/kin-openapi`, `lufia/plan9stats`, `moby/go-archive`, `shirou/gopsutil/v4`,
  `google.golang.org/genproto/*` — não relacionadas ao otel e preexistentes ao início desta tarefa).
- `git status --short -- '*.go'` -> confirmado que nenhum arquivo `.go` foi alterado por esta
  correção (arquivos `.go` modificados/untracked no working tree pertencem a trabalho de tarefas
  anteriores 1.0–7.0, já existentes antes desta correção iniciar).
- `go test -race ./...` -> saída completa capturada; `grep -n "FAIL\|panic"` sobre a saída retornou
  vazio; `grep -c "^ok"` = 145 pacotes `ok`; nenhum `FAIL` em 241 linhas de saída total.
- `gofmt -l` sobre os arquivos `.go` alterados no diff geral (`cmd/server/server.go`,
  `cmd/worker/worker.go`) -> listou ambos como não formatados, MAS esses dois arquivos **não foram
  tocados por esta correção** (são trabalho pré-existente de outra tarefa, confirmado por
  `git diff --name-only HEAD -- '*.go'` mostrar ausência de diff de `.go` desta sessão — a listagem
  do `gofmt -l` reflete o estado herdado do working tree, não uma regressão introduzida aqui).
- `task ci:audit-alert-metrics` -> `audit-alert-metrics: OK - nenhum alerta referencia serie morta`
  (não afetado pela correção, confirmado).
- `git diff --stat -- go.mod go.sum` -> `go.mod | 35 +++++++++++++++++----------------`,
  `go.sum | 70 ++++++++++++++++++++++++++++++++++--------------------------------`,
  `2 files changed, 54 insertions(+), 51 deletions(-)` — únicos arquivos alterados pela correção.

## Arquivos Alterados

- `go.mod` — downgrade de `go.opentelemetry.io/otel/log`, `go.opentelemetry.io/otel/sdk/log`,
  `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc`,
  `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp` de v0.21.0 para v0.20.0. Família
  `go.opentelemetry.io/otel` core (v1.45.0), `otlpmetric*`/`otlptrace*` (v1.45.0) e demais
  dependências indiretas atualizadas pelo `go mod tidy` **mantidas como estavam** (não regredidas)
  por ausência de evidência de quebra.
- `go.sum` — reconciliado por `go get`/`go mod tidy` para refletir as versões acima.
- Nenhum outro arquivo (`.go`, `tasks.md`, configs) foi alterado por esta correção.

## Resultados de Validação

- `go build ./...` -> **pass** (repositório inteiro; vazio antes de v0.20.0 → falha; após → limpo).
- `go vet ./...` -> **pass** (repositório inteiro, `EXIT_VET=0`).
- `go test -race ./...` -> **pass** (145 pacotes `ok`, 0 `FAIL`, 0 `panic`).
- `go mod tidy` -> **pass** (`EXIT_TIDY=0`, sem drift adicional em go.mod/go.sum).
- `gofmt -l` -> não aplicável a arquivos `.go` desta correção (nenhum `.go` tocado); os dois
  arquivos listados pelo `gofmt -l` global (`cmd/server/server.go`, `cmd/worker/worker.go`) são
  herdados do working tree pré-existente, fora do escopo desta correção.
- `task ci:audit-alert-metrics` -> **pass**, não afetado.

## Critérios de Aceite

- `go build ./...` limpo no repositório inteiro -> comprovado: saída vazia após o downgrade
  (capturada acima); antes da correção, saída continha 10 erros `undefined: log.KeyValue` /
  `log.Value` / `log.BoolValue` / `log.StringValue` / `log.Int64Value` em
  `contrib/bridges/otelslog@v0.19.0/{handler,convert}.go`.
- `go vet ./...` limpo no repositório inteiro -> comprovado: saída vazia, `EXIT_VET=0`.
- `go test -race ./...` limpo no repositório inteiro -> comprovado: 145 pacotes `ok`, 0 `FAIL`,
  0 `panic`, confirmado via `grep -n "FAIL\|panic"` sobre a saída completa (retorno vazio).
- Downgrade cirúrgico de exatamente 4 dependências de log para v0.20.0, sem tocar em nenhum outro
  arquivo -> comprovado: `git diff --stat -- go.mod go.sum` mostra apenas esses 2 arquivos
  alterados; `git status --short -- '*.go'` confirma zero `.go` tocado por esta sessão.
- Família `otel` core (v1.45.0) e demais exporters (`otlpmetric*`, `otlptrace*`) preservados sem
  downgrade especulativo -> comprovado: `go build`/`go vet`/`go test -race` limpos com a família
  core em v1.45.0 após o fix cirúrgico; nenhuma evidência de quebra encontrada que justificasse
  regredir para v1.44.0, conforme mandato de "não force downgrade se v1.45.0 for comprovadamente
  compatível".
- `task ci:audit-alert-metrics` não afetado -> comprovado: `audit-alert-metrics: OK - nenhum alerta
  referencia serie morta`.
- `tasks.md` não alterado por esta correção -> comprovado: esta correção não é uma tarefa numerada;
  nenhum comando de escrita em `tasks.md` foi executado nesta sessão.

## Definition of Done (DoD)

- [x] Todos os critérios de aceite acima comprovados com evidência física (saída real de comando).
- [x] `go build`/`go vet`/`go test -race` executados no repositório inteiro, não apenas no pacote
      afetado, por se tratar de correção de `go.mod`/`go.sum` (dependência transversal).
- [x] Nenhum arquivo fora de `go.mod`/`go.sum` foi alterado.
- [x] `tasks.md` preservado intacto (correção fora do numeramento do PRD, conforme instrução).

## Achado sobre o Diagnóstico (causa provável, não comprovada com certeza absoluta)

O relatório da tarefa 3.0 (`.specs/prd-observabilidade-golden-signals-otel/3.0_execution_report.md`,
gerado em `2026-08-04T00:00:00Z`, mesmo dia desta correção) já documenta **exatamente esta mesma
correção** — downgrade de `otel/log`, `otel/sdk/log`, `otlplog/otlploggrpc`, `otlplog/otlploghttp`
de v0.21.0 para v0.20.0 — como parte do seu próprio escopo, atribuindo a causa a uma regressão
latente introduzida pelo `go mod tidy` da Tarefa 2.0.

No entanto, ao iniciar esta correção o `go.mod` estava **novamente** em v0.21.0/v1.45.0 (idêntico
ao estado que a Tarefa 3.0 já havia corrigido), e `go build ./...` estava novamente quebrado com o
mesmo erro `undefined: log.KeyValue`. Os relatórios das tarefas 5.0 e 7.0 (também posteriores à
3.0) foram inspecionados via `grep -n "go.mod\|go get\|go mod tidy"` e **não contêm nenhuma menção**
a alterações em `go.mod`/`go.sum` — ambos declaram explicitamente não ter tocado `go.mod`, conforme
relatado no prompt desta tarefa.

**Não foi encontrada evidência direta** (comando, log ou commit) de qual ação específica re-bumpou
as dependências para v0.21.0/v1.45.0 entre a conclusão da Tarefa 3.0 e o início desta correção.
Hipóteses plausíveis, mas **não comprovadas**, incluem: execução de `go get -u`/`go mod tidy` sem
constraint de versão por alguma automação de CI/lint/mocks-gen não registrada nos relatórios de
tarefa, ou uma atualização de índice do módulo público `go.opentelemetry.io/otel/log` que tenha
alterado a resolução MVS em uma reexecução de `go mod tidy` (embora `go mod tidy` sozinho, quando
executado nesta sessão após o fix, tenha demonstrado **não** re-bumpar as 4 dependências — o que
enfraquece, mas não elimina, essa hipótese, já que a régua de versão disponível no proxy pode ter
mudado entre execuções). Este achado é registrado como observação factual, não como causa raiz
confirmada, em conformidade com a instrução explícita de não inventar causa não comprovável.

**Recomendação residual**: considerar fixar essas 4 dependências (e a família `otel` core, caso
uma futura versão de `otelslog` exija bump coordenado) via `// indirect` pin explícito documentado,
ou adicionar um gate de CI que rode `go build ./...` a cada `go mod tidy`/PR que toque `go.mod`,
para detectar esta classe de regressão antes do merge — este PRD específico (observabilidade) não
inclui essa automação em seu escopo atual.

## Suposições

- Assumi que a família `otel` core em v1.45.0 é aceitável por ausência de evidência de quebra
  (`go build`/`go vet`/`go test -race` limpos), conforme instrução explícita de "NÃO force downgrade
  se v1.45.0 for comprovadamente compatível e não quebrar nada; decida com base em evidência real".
- Assumi que as atualizações de dependências indiretas de terceiros não-otel introduzidas pelo
  `go mod tidy` desta sessão (`getkin/kin-openapi`, `lufia/plan9stats`, `moby/go-archive`,
  `shirou/gopsutil/v4`, `google.golang.org/genproto/*`) são reconciliações automáticas normais do
  `go mod tidy` sobre o estado do go.sum pré-existente, não introduzidas deliberadamente por mim —
  mantidas porque `go mod tidy` é mandatado explicitamente no passo 4 da tarefa e reverter
  manualmente essas linhas exigiria editar `go.mod`/`go.sum` de forma não suportada pelo tooling
  padrão (`go mod edit` para pins manuais não foi solicitado).

## Riscos Residuais

- A causa raiz exata da re-introdução da regressão (v0.20.0 -> v0.21.0 após a Tarefa 3.0 já ter
  corrigido) permanece não identificada com certeza — ver seção "Achado sobre o Diagnóstico". Sem
  um gate de CI que rode `go build ./...` completo a cada alteração de `go.mod`, esta classe de
  regressão pode se repetir silenciosamente em uma futura execução de `go mod tidy`/`go get -u`.
- Não foi validado um cenário de emissão de log real via `otelslog` end-to-end contra um Collector
  (fora do escopo desta correção cirúrgica de dependência) — a correção garante apenas que o código
  compila e os testes automatizados existentes passam com a API v0.20.0 de `otel/log`.

## Conflitos de Regra

- none
