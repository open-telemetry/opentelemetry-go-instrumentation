// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lib

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"log"
	"os"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

func isFunPartOfCallGraph(fun FuncDescriptor, callgraph map[FuncDescriptor][]FuncDescriptor) bool {
	// TODO this is not optimap o(n)
	for k, v := range callgraph {
		if k.TypeHash() == fun.TypeHash() {
			return true
		}
		for _, e := range v {
			if fun.TypeHash() == e.TypeHash() {
				return true
			}
		}
	}
	return false
}

func PropagateContext(projectPath string,
	packagePattern string,
	callgraph map[FuncDescriptor][]FuncDescriptor,
	rootFunctions []FuncDescriptor,
	funcDecls map[FuncDescriptor]bool,
	passFileSuffix string) {

	fset := token.NewFileSet()
	fmt.Println("PropagateContext")
	cfg := &packages.Config{Fset: fset, Mode: mode, Dir: projectPath}
	pkgs, err := packages.Load(cfg, packagePattern)
	if err != nil {
		log.Fatal(err)
	}
	for _, pkg := range pkgs {
		fmt.Println("\t", pkg)
		var node *ast.File
		for _, node = range pkg.Syntax {
			addImports := false
			var out *os.File
			fmt.Println("\t\t", fset.File(node.Pos()).Name())
			if len(passFileSuffix) > 0 {
				out, _ = os.Create(fset.File(node.Pos()).Name() + passFileSuffix)
				defer out.Close()
			} else {
				out, _ = os.Create(fset.File(node.Pos()).Name() + "ir_instr")
				defer out.Close()
			}

			if len(rootFunctions) == 0 {
				printer.Fprint(out, fset, node)
				continue
			}

			// below variable is used
			// when callexpr is inside var decl
			// instead of functiondecl
			currentFun := "nil"

			emitEmptyContext := func(x *ast.CallExpr, fun FuncDescriptor, ctxArg *ast.Ident) {
				visited := map[FuncDescriptor]bool{}
				if isPath(callgraph, fun, rootFunctions[0], visited) {
					addImports = true
					if currentFun != "nil" {
						x.Args = append([]ast.Expr{ctxArg}, x.Args...)
						return
					}
					contextTodo := &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X: &ast.Ident{
								Name: "context",
							},
							Sel: &ast.Ident{
								Name: "TODO",
							},
						},
						Lparen:   62,
						Ellipsis: 0,
					}
					x.Args = append([]ast.Expr{contextTodo}, x.Args...)
				}
			}
			emitCallExpr := func(ident *ast.Ident, n ast.Node, ctxArg *ast.Ident) {
				switch x := n.(type) {
				case *ast.CallExpr:
					if pkg.TypesInfo.Uses[ident] == nil {
						return
					}
					pkgPath := GetPkgNameFromUsesTable(pkg, ident)
					funId := pkgPath + "." + pkg.TypesInfo.Uses[ident].Name()
					fun := FuncDescriptor{funId,
						pkg.TypesInfo.Uses[ident].Type().String()}
					found := funcDecls[fun]
					// inject context parameter only
					// to these functions for which function decl
					// exists

					if found {
						emitEmptyContext(x, fun, ctxArg)
					}

				}
			}
			emitCallExprFromSelector := func(sel *ast.SelectorExpr, n ast.Node, ctxArg *ast.Ident) {
				switch x := n.(type) {
				case *ast.CallExpr:
					if pkg.TypesInfo.Uses[sel.Sel] == nil {
						return
					}
					pkgPath := GetPkgNameFromUsesTable(pkg, sel.Sel)
					if sel.X != nil {
						pkgPath = GetSelectorPkgPath(sel, pkg, pkgPath)
					}
					funId := pkgPath + "." + pkg.TypesInfo.Uses[sel.Sel].Name()
					fun := FuncDescriptor{funId,
						pkg.TypesInfo.Uses[sel.Sel].Type().String()}
					fmt.Println("\t\t\tFuncCall via selector:", funId,
						pkg.TypesInfo.Uses[sel.Sel].Type().String(),
						" @called : ", fset.File(node.Pos()).Name())
					found := funcDecls[fun]
					// inject context parameter only
					// to these functions for which function decl
					// exists

					if found {
						emitEmptyContext(x, fun, ctxArg)
					}
				}
			}
			ast.Inspect(node, func(n ast.Node) bool {
				ctxArg := &ast.Ident{
					Name: "__child_tracing_ctx",
				}
				ctxField := &ast.Field{
					Names: []*ast.Ident{
						&ast.Ident{
							Name: "__tracing_ctx",
						},
					},
					Type: &ast.SelectorExpr{
						X: &ast.Ident{
							Name: "context",
						},
						Sel: &ast.Ident{
							Name: "Context",
						},
					},
				}

				switch x := n.(type) {
				case *ast.FuncDecl:
					pkgPath := ""

					if x.Recv != nil {
						pkgPath = GetPackagePathHashFromFunc(pkg, pkgs, x)
					} else {
						pkgPath = GetPkgNameFromDefsTable(pkg, x.Name)
					}
					funId := pkgPath + "." + pkg.TypesInfo.Defs[x.Name].Name()
					fun := FuncDescriptor{funId,
						pkg.TypesInfo.Defs[x.Name].Type().String()}
					currentFun = funId
					// inject context only
					// functions available in the call graph
					if !isFunPartOfCallGraph(fun, callgraph) {
						break
					}

					if Contains(rootFunctions, fun) {
						break
					}
					visited := map[FuncDescriptor]bool{}
					fmt.Println("\t\t\tFuncDecl:", fun)
					if isPath(callgraph, fun, rootFunctions[0], visited) {
						addImports = true
						x.Type.Params.List = append([]*ast.Field{ctxField}, x.Type.Params.List...)
					}
				case *ast.CallExpr:
					if ident, ok := x.Fun.(*ast.Ident); ok {
						pkgPath := GetPkgNameFromUsesTable(pkg, ident)
						funId := pkgPath + "." + pkg.TypesInfo.Uses[ident].Name()
						fmt.Println("\t\t\tCallExpr:", funId, pkg.TypesInfo.Uses[ident].Type().String())

						emitCallExpr(ident, n, ctxArg)
					}

					if _, ok := x.Fun.(*ast.FuncLit); ok {
						addImports = true
						x.Args = append([]ast.Expr{ctxArg}, x.Args...)
					}
					if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
						emitCallExprFromSelector(sel, n, ctxArg)
					}

				case *ast.FuncLit:
					addImports = true
					x.Type.Params.List = append([]*ast.Field{ctxField}, x.Type.Params.List...)

				case *ast.TypeSpec:
					iname := x.Name
					iface, ok := x.Type.(*ast.InterfaceType)
					if !ok {
						return true
					}
					for _, method := range iface.Methods.List {
						funcType, ok := method.Type.(*ast.FuncType)
						if !ok {
							return true
						}
						visited := map[FuncDescriptor]bool{}
						pkgPath := GetPkgNameFromDefsTable(pkg, method.Names[0])
						funId := pkgPath + "." + iname.Name + "." + pkg.TypesInfo.Defs[method.Names[0]].Name()
						fun := FuncDescriptor{funId,
							pkg.TypesInfo.Defs[method.Names[0]].Type().String()}
						fmt.Println("\t\t\tInterfaceType", fun.Id, fun.DeclType)
						if isPath(callgraph, fun, rootFunctions[0], visited) {
							addImports = true
							funcType.Params.List = append([]*ast.Field{ctxField}, funcType.Params.List...)
						}
					}

				}
				return true
			})
			if addImports {
				if !astutil.UsesImport(node, "context") {
					astutil.AddImport(fset, node, "context")
				}
			}

			printer.Fprint(out, fset, node)
			if len(passFileSuffix) > 0 {
				os.Rename(fset.File(node.Pos()).Name(), fset.File(node.Pos()).Name()+".tmp")
			} else {
				os.Rename(fset.File(node.Pos()).Name()+"ir_instr", fset.File(node.Pos()).Name())
			}
		}
	}
}
