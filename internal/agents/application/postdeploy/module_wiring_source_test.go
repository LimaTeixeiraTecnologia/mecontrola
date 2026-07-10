package postdeploy

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ModuleWiringSourceSuite struct {
	suite.Suite
}

func TestModuleWiringSourceSuite(t *testing.T) {
	suite.Run(t, new(ModuleWiringSourceSuite))
}

var buildToolCallRegexp = regexp.MustCompile(`^agenttools\.Build([A-Za-z0-9]+)Tool$`)

func (s *ModuleWiringSourceSuite) TestBuildFinancialToolsCallCountMatchesInventory() {
	_, thisFile, _, ok := runtime.Caller(0)
	s.Require().True(ok, "runtime.Caller deve resolver o path deste arquivo")

	moduleGoPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "internal", "agents", "module.go")
	moduleGoPath, err := filepath.Abs(moduleGoPath)
	s.Require().NoError(err)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, moduleGoPath, nil, parser.AllErrors)
	s.Require().NoError(err, "module.go deve existir e ser parseável (RF-54/RF-55)")

	var toolBuildCalls int
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "buildFinancialTools" {
			return true
		}
		ast.Inspect(fn.Body, func(inner ast.Node) bool {
			call, ok := inner.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != "agenttools" {
				return true
			}
			if buildToolCallRegexp.MatchString("agenttools." + sel.Sel.Name) {
				toolBuildCalls++
			}
			return true
		})
		return false
	})

	s.Equal(len(RegisteredTools), toolBuildCalls, "buildFinancialTools em module.go deve registrar exatamente as tools do inventário de regressão (RF-54)")
}
