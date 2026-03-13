package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// osExitAnalyzer reports direct calls to os.Exit inside the main function
// of the main package.
//
// Мотивация:
// прямой вызов os.Exit из функции main усложняет тестирование и повторное
// использование логики завершения программы. Более гибкий подход — отделять
// вычисления от точки входа, использовать возвращаемые ошибки и единое место
// обработки ошибок и выхода из программы.
var osExitAnalyzer = &analysis.Analyzer{
	Name: "noosexit",
	Doc:  "reports direct calls to os.Exit in main.main",
	Run:  runOsExitAnalyzer,
}

func runOsExitAnalyzer(pass *analysis.Pass) (interface{}, error) {
	// Интересует только пакет main.
	if pass.Pkg == nil || pass.Pkg.Name() != "main" {
		return nil, nil
	}

	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			// Ищем именно функцию main.
			if fn.Name == nil || fn.Name.Name != "main" || fn.Body == nil {
				return true
			}

			ast.Inspect(fn.Body, func(nn ast.Node) bool {
				call, ok := nn.(*ast.CallExpr)
				if !ok {
					return true
				}

				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				if sel.Sel == nil || sel.Sel.Name != "Exit" {
					return true
				}

				// Убеждаемся, что это действительно os.Exit, а не любая другая Exit.
				if obj, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func); ok {
					if pkg := obj.Pkg(); pkg != nil && pkg.Path() == "os" && obj.Name() == "Exit" {
						pass.Reportf(call.Pos(), "direct call to os.Exit in main.main is forbidden")
					}
				}

				return true
			})

			// Уже обработали тело main, дальше идти не нужно.
			return false
		})
	}

	return nil, nil
}

