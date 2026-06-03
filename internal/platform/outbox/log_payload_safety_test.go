package outbox_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogPayloadSafety garante que nenhum arquivo .go do pacote outbox
// passa payload/Payload/json.RawMessage/evt.Payload como argumento em chamadas slog.*.
//
// Implementa o critério de aceite de RF-24 e R-SEC-001:
// payload bruto do evento NUNCA aparece em logs (proteção contra regressão).
func TestLogPayloadSafety(t *testing.T) {
	t.Parallel()

	// Determina o diretório do pacote (mesmo diretório do arquivo de teste).
	dir := filepath.Join(".") // pacote outbox

	// Lista todos os arquivos .go no diretório (sem recursão).
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "falha ao listar diretório do pacote outbox")

	fset := token.NewFileSet()
	var violations []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		// Pula o próprio arquivo de teste para evitar falso positivo no nome do arquivo.
		if entry.Name() == "log_payload_safety_test.go" {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		src, readErr := os.ReadFile(filePath)
		require.NoError(t, readErr, "falha ao ler arquivo %s", entry.Name())

		f, parseErr := parser.ParseFile(fset, filePath, src, 0)
		require.NoError(t, parseErr, "falha ao parsear arquivo %s", entry.Name())

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Verifica se é uma chamada slog.*.
			if !isSlogCall(call) {
				return true
			}

			// Verifica cada argumento da chamada slog.*.
			for _, arg := range call.Args {
				if containsPayloadRef(arg) {
					pos := fset.Position(call.Pos())
					violations = append(violations,
						"arquivo "+entry.Name()+" linha "+pos.String()+
							": chamada slog.* com referência a payload — violação de RF-24/R-SEC-001",
					)
				}
			}
			return true
		})
	}

	assert.Empty(t, violations,
		"encontradas %d violação(ões) de payload em slog.*:\n%s",
		len(violations), strings.Join(violations, "\n"),
	)
}

// isSlogCall retorna true se a chamada for para slog.* (slog.Info, slog.Warn, etc.)
// ou para métodos de *slog.Logger (logger.InfoContext, logger.WarnContext, etc.).
func isSlogCall(call *ast.CallExpr) bool {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		// Caso: slog.Info, slog.Warn, slog.Error, slog.Debug, slog.Log, slog.LogAttrs
		if ident, ok := fn.X.(*ast.Ident); ok && ident.Name == "slog" {
			return true
		}
		// Caso: logger.InfoContext, logger.WarnContext, logger.ErrorContext, logger.LogAttrs
		methodName := fn.Sel.Name
		return methodName == "InfoContext" ||
			methodName == "WarnContext" ||
			methodName == "ErrorContext" ||
			methodName == "DebugContext" ||
			methodName == "LogAttrs" ||
			methodName == "Log" ||
			methodName == "Info" ||
			methodName == "Warn" ||
			methodName == "Error" ||
			methodName == "Debug"
	}
	return false
}

// containsPayloadRef retorna true se a expressão contiver referência a payload.
// Detecta: identificadores "payload", "Payload"; seletor ".Payload()"; literal "payload".
func containsPayloadRef(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		switch v := n.(type) {
		case *ast.Ident:
			low := strings.ToLower(v.Name)
			if low == "payload" {
				found = true
				return false
			}
		case *ast.SelectorExpr:
			low := strings.ToLower(v.Sel.Name)
			if low == "payload" {
				found = true
				return false
			}
		case *ast.BasicLit:
			// Detecta literal de string "payload"
			if strings.Contains(strings.ToLower(v.Value), "payload") {
				found = true
				return false
			}
		}
		return true
	})
	return found
}
