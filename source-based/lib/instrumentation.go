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

func Instrument(projectPath string,
	packagePattern string,
	callgraph map[FuncDescriptor][]FuncDescriptor,
	rootFunctions []FuncDescriptor,
	passFileSuffix string) {

	fset := token.NewFileSet()
	fmt.Println("Instrumentation")
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
			addContext := false

			var out *os.File
			fmt.Println("\t\t", fset.File(node.Pos()).Name())
			if len(passFileSuffix) > 0 {
				out, _ = os.Create(fset.File(node.Pos()).Name() + passFileSuffix)
				defer out.Close()
			} else {
				out, _ = os.Create(fset.File(node.Pos()).Name() + "ir_context")
				defer out.Close()
			}
			if len(rootFunctions) == 0 {
				printer.Fprint(out, fset, node)
				continue
			}

			childTracingTodo := &ast.AssignStmt{
				Lhs: []ast.Expr{
					&ast.Ident{
						Name: "__child_tracing_ctx",
					},
				},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{
					&ast.CallExpr{
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
					},
				},
			}
			childTracingSupress := &ast.AssignStmt{
				Lhs: []ast.Expr{
					&ast.Ident{
						Name: "_",
					},
				},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{
					&ast.Ident{
						Name: "__child_tracing_ctx",
					},
				},
			}

			ast.Inspect(node, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.FuncDecl:
					pkgPath := ""

					if x.Recv != nil {
						pkgPath = GetPackagePathHashFromFunc(pkg, pkgs, x)
					} else {
						pkgPath = GetPkgNameFromDefsTable(pkg, x.Name)
					}
					fundId := pkgPath + "." + pkg.TypesInfo.Defs[x.Name].Name()
					fun := FuncDescriptor{fundId, pkg.TypesInfo.Defs[x.Name].Type().String()}
					// check if it's root function or
					// one of function in call graph
					// and emit proper ast nodes
					_, exists := callgraph[fun]
					if !exists {
						if !Contains(rootFunctions, fun) {
							addContext = true
							x.Body.List = append([]ast.Stmt{childTracingTodo, childTracingSupress}, x.Body.List...)
							return false
						}
					}

					for _, root := range rootFunctions {
						visited := map[FuncDescriptor]bool{}
						fmt.Println("\t\t\tFuncDecl:", fundId, pkg.TypesInfo.Defs[x.Name].Type().String())
						if isPath(callgraph, fun, root, visited) && fun.TypeHash() != root.TypeHash() {
							s1 := &ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X: &ast.Ident{
											Name: "fmt",
										},
										Sel: &ast.Ident{
											Name: "Println",
										},
									},
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"child instrumentation"`,
										},
									},
								},
							}
							s2 := &ast.AssignStmt{
								Lhs: []ast.Expr{
									&ast.Ident{
										Name: "__child_tracing_ctx",
									},
									&ast.Ident{
										Name: "span",
									},
								},
								Tok: token.DEFINE,
								Rhs: []ast.Expr{
									&ast.CallExpr{
										Fun: &ast.SelectorExpr{
											X: &ast.CallExpr{
												Fun: &ast.SelectorExpr{
													X: &ast.Ident{
														Name: "otel",
													},
													Sel: &ast.Ident{
														Name: "Tracer",
													},
												},
												Lparen: 50,
												Args: []ast.Expr{
													&ast.Ident{
														Name: `"` + x.Name.Name + `"`,
													},
												},
												Ellipsis: 0,
											},
											Sel: &ast.Ident{
												Name: "Start",
											},
										},
										Lparen: 62,
										Args: []ast.Expr{
											&ast.Ident{
												Name: "__tracing_ctx",
											},
											&ast.Ident{
												Name: `"` + x.Name.Name + `"`,
											},
										},
										Ellipsis: 0,
									},
								},
							}

							s3 := &ast.AssignStmt{
								Lhs: []ast.Expr{
									&ast.Ident{
										Name: "_",
									},
								},
								Tok: token.ASSIGN,
								Rhs: []ast.Expr{
									&ast.Ident{
										Name: "__child_tracing_ctx",
									},
								},
							}

							s4 := &ast.DeferStmt{
								Defer: 27,
								Call: &ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X: &ast.Ident{
											Name: "span",
										},
										Sel: &ast.Ident{
											Name: "End",
										},
									},
									Lparen:   41,
									Ellipsis: 0,
								},
							}
							_ = s1
							x.Body.List = append([]ast.Stmt{s2, s3, s4}, x.Body.List...)
							addContext = true
							addImports = true
						} else {
							// check whether this function is root function
							if !Contains(rootFunctions, fun) {
								x.Body.List = append([]ast.Stmt{childTracingTodo, childTracingSupress}, x.Body.List...)
								addContext = true
								return false
							}
							s1 := &ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X: &ast.Ident{
											Name: "fmt",
										},
										Sel: &ast.Ident{
											Name: "Println",
										},
									},
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"root instrumentation"`,
										},
									},
								},
							}

							s2 :=
								&ast.AssignStmt{
									Lhs: []ast.Expr{
										&ast.Ident{
											Name: "ts",
										},
									},
									Tok: token.DEFINE,

									Rhs: []ast.Expr{
										&ast.CallExpr{
											Fun: &ast.SelectorExpr{
												X: &ast.Ident{
													Name: "rtlib",
												},
												Sel: &ast.Ident{
													Name: "NewTracingState",
												},
											},
											Lparen:   54,
											Ellipsis: 0,
										},
									},
								}
							s3 := &ast.DeferStmt{
								Defer: 27,
								Call: &ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X: &ast.Ident{
											Name: "rtlib",
										},
										Sel: &ast.Ident{
											Name: "Shutdown",
										},
									},
									Lparen: 48,
									Args: []ast.Expr{
										&ast.Ident{
											Name: "ts",
										},
									},
									Ellipsis: 0,
								},
							}

							s4 := &ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X: &ast.Ident{
											Name: "otel",
										},
										Sel: &ast.Ident{
											Name: "SetTracerProvider",
										},
									},
									Lparen: 49,
									Args: []ast.Expr{
										&ast.SelectorExpr{
											X: &ast.Ident{
												Name: "ts",
											},
											Sel: &ast.Ident{
												Name: "Tp",
											},
										},
									},
									Ellipsis: 0,
								},
							}
							s5 := &ast.AssignStmt{
								Lhs: []ast.Expr{
									&ast.Ident{
										Name: "ctx",
									},
								},
								Tok: token.DEFINE,
								Rhs: []ast.Expr{
									&ast.CallExpr{
										Fun: &ast.SelectorExpr{
											X: &ast.Ident{
												Name: "context",
											},
											Sel: &ast.Ident{
												Name: "Background",
											},
										},
										Lparen:   52,
										Ellipsis: 0,
									},
								},
							}
							s6 := &ast.AssignStmt{
								Lhs: []ast.Expr{
									&ast.Ident{
										Name: "__child_tracing_ctx",
									},
									&ast.Ident{
										Name: "span",
									},
								},
								Tok: token.DEFINE,
								Rhs: []ast.Expr{
									&ast.CallExpr{
										Fun: &ast.SelectorExpr{
											X: &ast.CallExpr{
												Fun: &ast.SelectorExpr{
													X: &ast.Ident{
														Name: "otel",
													},
													Sel: &ast.Ident{
														Name: "Tracer",
													},
												},
												Lparen: 50,
												Args: []ast.Expr{
													&ast.Ident{
														Name: `"` + x.Name.Name + `"`,
													},
												},
												Ellipsis: 0,
											},
											Sel: &ast.Ident{
												Name: "Start",
											},
										},
										Lparen: 62,
										Args: []ast.Expr{
											&ast.Ident{
												Name: "ctx",
											},
											&ast.Ident{
												Name: `"` + x.Name.Name + `"`,
											},
										},
										Ellipsis: 0,
									},
								},
							}

							s8 := &ast.DeferStmt{
								Defer: 27,
								Call: &ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X: &ast.Ident{
											Name: "span",
										},
										Sel: &ast.Ident{
											Name: "End",
										},
									},
									Lparen:   41,
									Ellipsis: 0,
								},
							}
							_ = s1
							x.Body.List = append([]ast.Stmt{s2, s3, s4, s5, s6, s8}, x.Body.List...)
							x.Body.List = append([]ast.Stmt{childTracingTodo, childTracingSupress}, x.Body.List...)
							addContext = true
							addImports = true

						}
					}
				case *ast.FuncLit:
					s1 := &ast.ExprStmt{
						X: &ast.CallExpr{
							Fun: &ast.SelectorExpr{
								X: &ast.Ident{
									Name: "fmt",
								},
								Sel: &ast.Ident{
									Name: "Println",
								},
							},
							Args: []ast.Expr{
								&ast.BasicLit{
									Kind:  token.STRING,
									Value: `"child instrumentation"`,
								},
							},
						},
					}
					s2 := &ast.AssignStmt{
						Lhs: []ast.Expr{
							&ast.Ident{
								Name: "__child_tracing_ctx",
							},
							&ast.Ident{
								Name: "span",
							},
						},
						Tok: token.DEFINE,
						Rhs: []ast.Expr{
							&ast.CallExpr{
								Fun: &ast.SelectorExpr{
									X: &ast.CallExpr{
										Fun: &ast.SelectorExpr{
											X: &ast.Ident{
												Name: "otel",
											},
											Sel: &ast.Ident{
												Name: "Tracer",
											},
										},
										Lparen: 50,
										Args: []ast.Expr{
											&ast.Ident{
												Name: `"` + "anonymous" + `"`,
											},
										},
										Ellipsis: 0,
									},
									Sel: &ast.Ident{
										Name: "Start",
									},
								},
								Lparen: 62,
								Args: []ast.Expr{
									&ast.Ident{
										Name: "__tracing_ctx",
									},
									&ast.Ident{
										Name: `"` + "anonymous" + `"`,
									},
								},
								Ellipsis: 0,
							},
						},
					}

					s3 := &ast.AssignStmt{
						Lhs: []ast.Expr{
							&ast.Ident{
								Name: "_",
							},
						},
						Tok: token.ASSIGN,
						Rhs: []ast.Expr{
							&ast.Ident{
								Name: "__child_tracing_ctx",
							},
						},
					}

					s4 := &ast.DeferStmt{
						Defer: 27,
						Call: &ast.CallExpr{
							Fun: &ast.SelectorExpr{
								X: &ast.Ident{
									Name: "span",
								},
								Sel: &ast.Ident{
									Name: "End",
								},
							},
							Lparen:   41,
							Ellipsis: 0,
						},
					}
					_ = s1
					x.Body.List = append([]ast.Stmt{s2, s3, s4}, x.Body.List...)
					addImports = true
					addContext = true
				}

				return true
			})
			if addContext {
				if !astutil.UsesImport(node, "context") {
					astutil.AddImport(fset, node, "context")
				}
			}
			if addImports {
				if !astutil.UsesImport(node, "go.opentelemetry.io/otel") {
					astutil.AddNamedImport(fset, node, "otel", "go.opentelemetry.io/otel")
				}
			}
			printer.Fprint(out, fset, node)
			if len(passFileSuffix) > 0 {
				os.Rename(fset.File(node.Pos()).Name(), fset.File(node.Pos()).Name()+".original")
			} else {
				os.Rename(fset.File(node.Pos()).Name()+"ir_context", fset.File(node.Pos()).Name())
			}
		}

	}
}
