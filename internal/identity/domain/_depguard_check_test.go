//go:build depguard_test

// Este arquivo é um teste de meta para validar que golangci-lint bloqueia
// imports inválidos de fronteira hexagonal.
//
// INSTRUÇÃO DE USO (CI/revisão manual):
//
//  1. Remova o underscore do nome do arquivo para ativar: renomeie para
//     depguard_check_test.go e adicione uma linha de import ilegal abaixo.
//
//  2. Execute:
//     golangci-lint run --build-tags depguard_test ./internal/identity/domain/...
//
//  3. Resultado esperado: ERRO de depguard indicando violação de fronteira.
//
//  4. Restore: renomeie de volta para _depguard_check_test.go (com underscore)
//     para que o arquivo seja ignorado pelo build normal.
//
// Exemplo de import ilegal que DEVE ser bloqueado pelo depguard:
//
//	import _ "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure"
//
// Regras validadas:
//   - domain NÃO importa infrastructure   (Regra 1 — RF-09a)
//   - domain NÃO importa application      (Regra 1 — RF-09a)
//   - domain NÃO importa infrastructure/* (Regra 3 — RF-09c)
//   - domain NÃO importa configs/*        (Regra 4 — RF-09d)
//   - identity/* NÃO importa finance/*    (Regra 5 — RF-09 cross-module)
//
// Validação automatizada em CI: task 10.0 configura job que executa
// golangci-lint com o build tag depguard_test para confirmar que as
// regras acima são enforçadas antes do merge para main.

package domain
