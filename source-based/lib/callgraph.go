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
	"go/token"
	"go/types"
	"log"
	"os"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

type FuncDescriptor struct {
	Id       string
	DeclType string
}

func (fd FuncDescriptor) TypeHash() string {
	return fd.Id + fd.DeclType
}

const mode packages.LoadMode = packages.NeedName |
	packages.NeedTypes |
	packages.NeedSyntax |
	packages.NeedTypesInfo |
	packages.NeedFiles

func FindRootFunctions(projectPath string, packagePattern string) []FuncDescriptor {
	fset := token.NewFileSet()

	var currentFun FuncDescriptor
	var rootFunctions []FuncDescriptor

	fmt.Println("FindRootFunctions")
	cfg := &packages.Config{Fset: fset, Mode: mode, Dir: projectPath}
	pkgs, err := packages.Load(cfg, packagePattern)
	if err != nil {
		log.Fatal(err)
	}
	for _, pkg := range pkgs {
		fmt.Println("\t", pkg)

		for _, node := range pkg.Syntax {
			fmt.Println("\t\t", fset.File(node.Pos()).Name())
			ast.Inspect(node, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					selector, ok := x.Fun.(*ast.SelectorExpr)
					if ok {
						if selector.Sel.Name == "AutotelEntryPoint__" {
							rootFunctions = append(rootFunctions, currentFun)
						}
					}
				case *ast.FuncDecl:
					funId := pkg.TypesInfo.Defs[x.Name].Pkg().Path() + "." + pkg.TypesInfo.Defs[x.Name].Name()
					currentFun = FuncDescriptor{funId, pkg.TypesInfo.Defs[x.Name].Type().String()}
					fmt.Println("\t\t\tFuncDecl:", funId, pkg.TypesInfo.Defs[x.Name].Type().String())
				}
				return true
			})
		}
	}
	return rootFunctions
}

func GetMostInnerAstIdent(inSel *ast.SelectorExpr) *ast.Ident {
	var l []*ast.Ident
	var e ast.Expr
	e = inSel
	for e != nil {
		if _, ok := e.(*ast.Ident); ok {
			l = append(l, e.(*ast.Ident))
			break
		} else if _, ok := e.(*ast.SelectorExpr); ok {
			l = append(l, e.(*ast.SelectorExpr).Sel)
			e = e.(*ast.SelectorExpr).X
		} else if _, ok := e.(*ast.CallExpr); ok {
			e = e.(*ast.CallExpr).Fun
		} else if _, ok := e.(*ast.IndexExpr); ok {
			e = e.(*ast.IndexExpr).X
		}
	}
	if len(l) < 2 {
		panic("selector list should have at least 2 elems")
	}
	// caller or receiver is always
	// at position 1, function is at 0
	return l[1]
}

func GetPackagePathHashFromFunc(pkg *packages.Package, pkgs []*packages.Package, x *ast.FuncDecl) string {
	pkgPath := ""
	for _, v := range x.Recv.List {
		for _, dependentpkg := range pkgs {
			for _, defs := range dependentpkg.TypesInfo.Defs {
				if defs != nil {
					if _, ok := defs.Type().Underlying().(*types.Interface); ok {
						if len(v.Names) < 0 || pkg.TypesInfo.Defs[v.Names[0]] == nil {
							continue
						}
						funType := pkg.TypesInfo.Defs[v.Names[0]].Type()
						if types.Implements(funType, defs.Type().Underlying().(*types.Interface)) {
							pkgPath = defs.Type().String()
							break
						} else {
							pkgPath = funType.String()
							// We don't care if that's pointer, remove it from
							// type id
							if _, ok := funType.(*types.Pointer); ok {
								pkgPath = strings.TrimPrefix(pkgPath, "*")
							}
							// We don't care if called via index, remove it from
							// type id
							if _, ok := funType.(*types.Slice); ok {
								pkgPath = strings.TrimPrefix(pkgPath, "[]")
							}
						}
					}
				}
			}
		}
	}
	return pkgPath
}

func GetSelectorPkgPath(sel *ast.SelectorExpr, pkg *packages.Package, pkgPath string) string {
	caller := GetMostInnerAstIdent(sel)
	if caller != nil {
		if pkg.TypesInfo.Uses[caller] != nil {
			if !strings.Contains(pkg.TypesInfo.Uses[caller].Type().String(), "invalid") {
				pkgPath = pkg.TypesInfo.Uses[caller].Type().String()
				// We don't care if that's pointer, remove it from
				// type id
				if _, ok := pkg.TypesInfo.Uses[caller].Type().(*types.Pointer); ok {
					pkgPath = strings.TrimPrefix(pkgPath, "*")
				}
				// We don't care if called via index, remove it from
				// type id
				if _, ok := pkg.TypesInfo.Uses[caller].Type().(*types.Slice); ok {
					pkgPath = strings.TrimPrefix(pkgPath, "[]")
				}
			}
		}
	}
	return pkgPath
}

func GetPkgNameFromUsesTable(pkg *packages.Package, ident *ast.Ident) string {
	if pkg.TypesInfo.Uses[ident].Pkg() != nil {
		return pkg.TypesInfo.Uses[ident].Pkg().Path()
	}
	return ""
}

func GetPkgNameFromDefsTable(pkg *packages.Package, ident *ast.Ident) string {
	if pkg.TypesInfo.Defs[ident].Pkg() != nil {
		return pkg.TypesInfo.Defs[ident].Pkg().Path()
	}
	return ""
}

func BuildCallGraph(
	projectPath string,
	packagePattern string,
	funcDecls map[FuncDescriptor]bool) map[FuncDescriptor][]FuncDescriptor {

	fset := token.NewFileSet()
	cfg := &packages.Config{Fset: fset, Mode: mode, Dir: projectPath}
	pkgs, err := packages.Load(cfg, packagePattern)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("BuildCallGraph")
	currentFun := FuncDescriptor{"nil", ""}
	backwardCallGraph := make(map[FuncDescriptor][]FuncDescriptor)
	for _, pkg := range pkgs {

		fmt.Println("\t", pkg)
		for _, node := range pkg.Syntax {
			fmt.Println("\t\t", fset.File(node.Pos()).Name())
			ast.Inspect(node, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					if id, ok := x.Fun.(*ast.Ident); ok {
						pkgPath := GetPkgNameFromUsesTable(pkg, id)
						funId := pkgPath + "." + pkg.TypesInfo.Uses[id].Name()
						fmt.Println("\t\t\tFuncCall:", funId, pkg.TypesInfo.Uses[id].Type().String(),
							" @called : ",
							fset.File(node.Pos()).Name())
						fun := FuncDescriptor{funId, pkg.TypesInfo.Uses[id].Type().String()}
						if !Contains(backwardCallGraph[fun], currentFun) {
							if funcDecls[fun] == true {
								backwardCallGraph[fun] = append(backwardCallGraph[fun], currentFun)
							}
						}
					}
					if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
						if pkg.TypesInfo.Uses[sel.Sel] != nil {
							pkgPath := GetPkgNameFromUsesTable(pkg, sel.Sel)
							if sel.X != nil {
								pkgPath = GetSelectorPkgPath(sel, pkg, pkgPath)
							}
							funId := pkgPath + "." + pkg.TypesInfo.Uses[sel.Sel].Name()
							fmt.Println("\t\t\tFuncCall via selector:", funId, pkg.TypesInfo.Uses[sel.Sel].Type().String(),
								" @called : ",
								fset.File(node.Pos()).Name())
							fun := FuncDescriptor{funId, pkg.TypesInfo.Uses[sel.Sel].Type().String()}
							if !Contains(backwardCallGraph[fun], currentFun) {
								if funcDecls[fun] == true {
									backwardCallGraph[fun] = append(backwardCallGraph[fun], currentFun)
								}
							}
						}
					}
				case *ast.FuncDecl:
					pkgPath := ""
					if x.Recv != nil {
						pkgPath = GetPackagePathHashFromFunc(pkg, pkgs, x)
					} else {
						pkgPath = GetPkgNameFromDefsTable(pkg, x.Name)
					}
					funId := pkgPath + "." + pkg.TypesInfo.Defs[x.Name].Name()
					funcDecls[FuncDescriptor{funId, pkg.TypesInfo.Defs[x.Name].Type().String()}] = true
					currentFun = FuncDescriptor{funId, pkg.TypesInfo.Defs[x.Name].Type().String()}
					fmt.Println("\t\t\tFuncDecl:", funId, pkg.TypesInfo.Defs[x.Name].Type().String())
				}
				return true
			})
		}
	}
	return backwardCallGraph
}

func FindFuncDecls(projectPath string, packagePattern string) map[FuncDescriptor]bool {
	fset := token.NewFileSet()
	cfg := &packages.Config{Fset: fset, Mode: mode, Dir: projectPath}
	pkgs, err := packages.Load(cfg, packagePattern)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("FindFuncDecls")
	funcDecls := make(map[FuncDescriptor]bool)
	for _, pkg := range pkgs {
		fmt.Println("\t", pkg)
		for _, node := range pkg.Syntax {
			fmt.Println("\t\t", fset.File(node.Pos()).Name())
			ast.Inspect(node, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.FuncDecl:
					pkgPath := ""
					if x.Recv != nil {
						pkgPath = GetPackagePathHashFromFunc(pkg, pkgs, x)
					} else {
						pkgPath = GetPkgNameFromDefsTable(pkg, x.Name)
					}
					funId := pkgPath + "." + pkg.TypesInfo.Defs[x.Name].Name()
					fmt.Println("\t\t\tFuncDecl:", funId, pkg.TypesInfo.Defs[x.Name].Type().String())
					funcDecls[FuncDescriptor{funId, pkg.TypesInfo.Defs[x.Name].Type().String()}] = true

				}
				return true
			})
		}
	}
	return funcDecls
}

func InferRootFunctionsFromGraph(callgraph map[FuncDescriptor][]FuncDescriptor) []FuncDescriptor {
	var allFunctions map[FuncDescriptor]bool
	var rootFunctions []FuncDescriptor
	allFunctions = make(map[FuncDescriptor]bool)
	for k, v := range callgraph {
		allFunctions[k] = true
		for _, childFun := range v {
			allFunctions[childFun] = true
		}
	}
	for k, _ := range allFunctions {
		_, exists := callgraph[k]
		if !exists {
			rootFunctions = append(rootFunctions, k)
		}
	}
	return rootFunctions
}

// var callgraph = {
//     nodes: [
//         { data: { id: 'fun1' } },
//         { data: { id: 'fun2' } },
// 		],
//     edges: [
//         { data: { id: 'e1', source: 'fun1', target: 'fun2' } },
//     ]
// };

func Generatecfg(callgraph map[FuncDescriptor][]FuncDescriptor, path string) {
	functions := make(map[FuncDescriptor]bool, 0)
	for k, childFuns := range callgraph {
		if functions[k] == false {
			functions[k] = true
		}
		for _, v := range childFuns {
			if functions[v] == false {
				functions[v] = true
			}
		}
	}
	for f := range functions {
		fmt.Println(f)
	}
	out, err := os.Create(path)
	defer out.Close()
	if err != nil {
		return
	}
	out.WriteString("var callgraph = {")
	out.WriteString("\n\tnodes: [")
	for f := range functions {
		out.WriteString("\n\t\t { data: { id: '")
		out.WriteString(f.TypeHash())
		out.WriteString("' } },")
	}
	out.WriteString("\n\t],")
	out.WriteString("\n\tedges: [")
	edgeCounter := 0
	for k, children := range callgraph {
		for _, childFun := range children {
			out.WriteString("\n\t\t { data: { id: '")
			out.WriteString("e" + strconv.Itoa(edgeCounter))
			out.WriteString("', ")
			out.WriteString("source: '")

			out.WriteString(childFun.TypeHash())

			out.WriteString("', ")
			out.WriteString("target: '")
			out.WriteString(k.TypeHash())
			out.WriteString("' ")
			out.WriteString("} },")
			edgeCounter++
		}
	}
	out.WriteString("\n\t]")
	out.WriteString("\n};")
}
