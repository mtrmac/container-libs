package main

import (
	"bytes"
	"cmp"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// SystemContextUse represents a use of types.SystemContext
type SystemContextUse struct {
	File   string // relative file path
	Line   int    // line number
	Column int    // column number
	Kind   string // kind of use
	Name   string // identifier name
}

// String returns a formatted string representation of the SystemContextUse
func (u SystemContextUse) String() string {
	return fmt.Sprintf("%s:%d:%d: %s %s", u.File, u.Line, u.Column, u.Kind, u.Name)
}

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <directory>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	dir := flag.Arg(0)
	uses, err := auditSystemContextUses(dir)
	if err != nil {
		log.Fatalf("Error auditing SystemContext uses: %v", err)
	}

	// Print results in VS Code compatible format
	for _, use := range uses {
		fmt.Println(use.String())
	}
}

// auditSystemContextUses finds all problematic uses of types.SystemContext in the given directory
// Returns all uses, error
func auditSystemContextUses(dir string) ([]SystemContextUse, error) {
	// Determine if input path is absolute
	inputIsAbsolute := filepath.IsAbs(dir)

	// Get current working directory for relative path computation
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Configure package loading
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedSyntax,
		Dir: dir,
	}

	// Load packages - use "./..." pattern to recursively load all packages
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("failed to load packages: %w", err)
	}

	// Check for errors in loaded packages - fail immediately on any errors
	var loadErrors []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", pkg.PkgPath, e))
			}
		}
	}
	if len(loadErrors) > 0 {
		return nil, fmt.Errorf("package loading errors:\n%s", strings.Join(loadErrors, "\n"))
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found in %s", dir)
	}

	var uses []SystemContextUse

	// Process each package
	for _, pkg := range pkgs {
		if pkg.Types == nil || pkg.TypesInfo == nil {
			return nil, fmt.Errorf("package %s missing type information", pkg.PkgPath)
		}

		// Walk AST of each file in the package
		for _, file := range pkg.Syntax {
			// Track which nodes we've already recorded to avoid duplicates
			// Key is combination of node and kind to allow multiple reports per node
			type nodeKey struct {
				node ast.Node
				kind string
			}
			recordedNodes := make(map[nodeKey]bool)

			// Use PreorderStack to walk with parent stack tracking
			ast.PreorderStack(file, nil, func(node ast.Node, stack []ast.Node) bool {
				foundUses := checkForSystemContextUse(node, stack, pkg)
				for _, use := range foundUses {
					// Check if we've already recorded this use
					key := nodeKey{use.node, use.kind}
					if !recordedNodes[key] {
						recordedNodes[key] = true
						uses = append(uses, recordUse(use, pkg, cwd, inputIsAbsolute))
					}
				}

				return true
			})
		}
	}

	// Sort by file, line, column for consistent output
	slices.SortFunc(uses, func(a, b SystemContextUse) int {
		return cmp.Or(
			cmp.Compare(a.File, b.File),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Column, b.Column),
		)
	})

	return uses, nil
}

// useInfo describes what use to report
type useInfo struct {
	node ast.Node // the node to report (position)
	kind string   // kind of use
	name string   // descriptive name
}

// recordUse creates a SystemContextUse record from useInfo
func recordUse(use *useInfo, pkg *packages.Package, cwd string, useAbsolute bool) SystemContextUse {
	pos := pkg.Fset.Position(use.node.Pos())

	var filePath string
	if useAbsolute {
		// Input was absolute, use absolute paths in output
		filePath = pos.Filename
	} else {
		// Input was relative, make paths relative to CWD
		relPath, err := filepath.Rel(cwd, pos.Filename)
		if err != nil {
			panic(fmt.Sprintf("failed to compute relative path for %s: %v", pos.Filename, err))
		}
		filePath = relPath
	}

	return SystemContextUse{
		File:   filePath,
		Line:   pos.Line,
		Column: pos.Column,
		Kind:   use.kind,
		Name:   use.name,
	}
}

// checkForSystemContextUse checks if a node represents a problematic SystemContext use
// Returns empty slice if not a reportable use
func checkForSystemContextUse(node ast.Node, stack []ast.Node, pkg *packages.Package) []*useInfo {
	var results []*useInfo

	// Check for struct field declarations
	if use := checkFieldDeclaration(node, stack, pkg); use != nil {
		results = append(results, use)
	}

	// Check for variable definitions (var or :=)
	if use := checkVarDefinition(node, pkg); use != nil {
		results = append(results, use)
	}

	// Check for nil assignments/uses
	if use := checkNilUse(node, stack, pkg); use != nil {
		results = append(results, use)
	}

	// Check for empty SystemContext{} literal uses
	if use := checkLiteralUse(node, pkg); use != nil {
		results = append(results, use)
	}

	// Check for struct literals with implicit nil/empty SystemContext fields
	results = append(results, checkImplicitNilInStructLiteral(node, pkg)...)

	return results
}

// checkFieldDeclaration checks if node is a struct field declaration with SystemContext type
func checkFieldDeclaration(node ast.Node, stack []ast.Node, pkg *packages.Package) *useInfo {
	field, ok := node.(*ast.Field)
	if !ok {
		return nil
	}

	// Check if this is a struct field (not a function parameter)
	// Parent is FieldList, grandparent should be StructType for struct fields
	if len(stack) < 2 {
		return nil
	}

	// Parent should be FieldList
	if _, ok := stack[len(stack)-1].(*ast.FieldList); !ok {
		return nil
	}

	// Grandparent should be StructType (not FuncType for parameters)
	if _, ok := stack[len(stack)-2].(*ast.StructType); !ok {
		return nil
	}

	// Check if the field type is SystemContext
	fieldType := pkg.TypesInfo.TypeOf(field.Type)
	if fieldType == nil || !isSystemContextType(fieldType) {
		return nil
	}

	// Get field name
	var fieldName string
	if len(field.Names) > 0 {
		fieldName = field.Names[0].Name
	} else {
		// Embedded field
		fieldName = getOriginalExprText(field.Type, pkg.Fset)
	}

	return &useInfo{
		node: field,
		kind: "field-declaration",
		name: fieldName,
	}
}

// checkVarDefinition checks if node is a variable definition with SystemContext type
// Reports: var declarations and := assignments that create new SystemContext variables
func checkVarDefinition(node ast.Node, pkg *packages.Package) *useInfo {
	switch n := node.(type) {
	case *ast.ValueSpec:
		// var declaration: var ctx types.SystemContext or var ctx *types.SystemContext
		// Check if any of the declared names have SystemContext type
		for _, name := range n.Names {
			if obj := pkg.TypesInfo.Defs[name]; obj != nil {
				if isSystemContextType(obj.Type()) {
					return &useInfo{
						node: name,
						kind: "var-definition",
						name: name.Name,
					}
				}
			}
		}

	case *ast.AssignStmt:
		// := assignment
		if n.Tok != token.DEFINE {
			return nil // Not a := assignment
		}

		// Check each LHS identifier being defined
		for _, lhs := range n.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			if obj := pkg.TypesInfo.Defs[ident]; obj != nil {
				if isSystemContextType(obj.Type()) {
					return &useInfo{
						node: ident,
						kind: "var-definition",
						name: ident.Name,
					}
				}
			}
		}
	}

	return nil
}

// checkNilUse checks if node is setting a *types.SystemContext to nil
func checkNilUse(node ast.Node, stack []ast.Node, pkg *packages.Package) *useInfo {
	ident, ok := node.(*ast.Ident)
	if !ok || ident.Name != "nil" {
		return nil
	}

	// Get parent from stack
	var parent ast.Node
	if len(stack) > 0 {
		parent = stack[len(stack)-1]
	}
	if parent == nil {
		return nil
	}

	switch p := parent.(type) {
	case *ast.AssignStmt:
		// Check if nil is on RHS being assigned to a *SystemContext
		for i, rhs := range p.Rhs {
			if rhs == ident {
				if i < len(p.Lhs) {
					lhs := p.Lhs[i]
					lhsType := pkg.TypesInfo.TypeOf(lhs)
					if isSystemContextPtrType(lhsType) {
						return &useInfo{
							node: ident,
							kind: "nil-assignment",
							name: getOriginalExprText(lhs, pkg.Fset),
						}
					}
				}
			}
		}

	case *ast.ValueSpec:
		// var ctx *types.SystemContext = nil
		for i, val := range p.Values {
			if val == ident {
				if i < len(p.Names) {
					name := p.Names[i]
					if obj := pkg.TypesInfo.Defs[name]; obj != nil {
						if isSystemContextPtrType(obj.Type()) {
							return &useInfo{
								node: ident,
								kind: "nil-init",
								name: name.Name,
							}
						}
					}
				}
			}
		}

	case *ast.CallExpr:
		// Check if this is a type conversion like (*Type)(nil) vs a function call
		funType := pkg.TypesInfo.TypeOf(p.Fun)
		if funType == nil {
			return nil
		}
		sig, isSignature := funType.(*types.Signature)
		if !isSignature {
			// Type conversion, not a function call
			convType := pkg.TypesInfo.TypeOf(p)
			if isSystemContextPtrType(convType) {
				return &useInfo{
					node: ident,
					kind: "nil-conversion",
					name: "*SystemContext",
				}
			}
			return nil
		}

		// Passing nil as argument to function expecting *SystemContext
		for i, arg := range p.Args {
			if arg == ident {
				paramType, err := getParameterType(sig, i)
				if err != nil {
					pos := pkg.Fset.Position(ident.Pos())
					fmt.Fprintf(os.Stderr, "WARNING: %s:%d:%d: failed to get parameter type: %v\n",
						pos.Filename, pos.Line, pos.Column, err)
					continue
				}
				if isSystemContextPtrType(paramType) {
					paramName, err := getParameterName(sig, i)
					if err != nil {
						pos := pkg.Fset.Position(ident.Pos())
						fmt.Fprintf(os.Stderr, "WARNING: %s:%d:%d: failed to get parameter name: %v\n",
							pos.Filename, pos.Line, pos.Column, err)
					}
					if paramName == "" {
						paramName = fmt.Sprintf("arg%d", i)
					}
					return &useInfo{
						node: ident,
						kind: "nil-argument",
						name: paramName,
					}
				}
			}
		}

	case *ast.KeyValueExpr:
		// nil in struct/map literal: Container{SysCtx: nil}
		if p.Value == ident {
			var grandparent ast.Node
			if len(stack) >= 2 {
				grandparent = stack[len(stack)-2]
			}

			if compositeLit, ok := grandparent.(*ast.CompositeLit); ok {
				litType := pkg.TypesInfo.TypeOf(compositeLit)
				if litType != nil {
					if structType, ok := litType.Underlying().(*types.Struct); ok {
						if keyIdent, ok := p.Key.(*ast.Ident); ok {
							for i := 0; i < structType.NumFields(); i++ {
								field := structType.Field(i)
								if field.Name() == keyIdent.Name {
									if isSystemContextPtrType(field.Type()) {
										return &useInfo{
											node: ident,
											kind: "nil-field-value",
											name: keyIdent.Name,
										}
									}
									break
								}
							}
						}
					}
				}
			}
		}

	case *ast.ReturnStmt:
		// return nil from function returning *SystemContext
		for i, result := range p.Results {
			if result == ident {
				encFunc := findEnclosingFunc(stack)
				if encFunc == nil {
					pos := pkg.Fset.Position(ident.Pos())
					fmt.Fprintf(os.Stderr, "WARNING: %s:%d:%d: nil in return statement but no enclosing function found\n",
						pos.Filename, pos.Line, pos.Column)
					continue
				}
				retType, err := getReturnType(encFunc, i, pkg)
				if err != nil {
					pos := pkg.Fset.Position(ident.Pos())
					fmt.Fprintf(os.Stderr, "WARNING: %s:%d:%d: failed to get return type: %v\n",
						pos.Filename, pos.Line, pos.Column, err)
					continue
				}
				if isSystemContextPtrType(retType) {
					return &useInfo{
						node: ident,
						kind: "nil-return",
						name: "return",
					}
				}
			}
		}
	}

	return nil
}

// checkLiteralUse reports all types.SystemContext{...} literals
func checkLiteralUse(node ast.Node, pkg *packages.Package) *useInfo {
	compositeLit, ok := node.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	litType := pkg.TypesInfo.TypeOf(compositeLit)
	if litType == nil {
		pos := pkg.Fset.Position(compositeLit.Pos())
		log.Fatalf("FATAL: %s:%d:%d: no type info for composite literal",
			pos.Filename, pos.Line, pos.Column)
	}
	if !isSystemContextTypeExact(litType) {
		return nil // Not a SystemContext literal
	}

	return &useInfo{
		node: compositeLit,
		kind: "literal",
		name: "SystemContext{}",
	}
}

// checkImplicitNilInStructLiteral checks if a struct literal has a *SystemContext field
// that is not explicitly set (implicitly nil)
func checkImplicitNilInStructLiteral(node ast.Node, pkg *packages.Package) []*useInfo {
	compositeLit, ok := node.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	// Get the type of the composite literal
	litType := pkg.TypesInfo.TypeOf(compositeLit)
	if litType == nil {
		return nil
	}

	// Check if this is a struct type
	structType, ok := litType.Underlying().(*types.Struct)
	if !ok {
		return nil
	}

	// Find all SystemContext fields in the struct (both pointer and non-pointer)
	var sysCtxPtrFields []string   // *types.SystemContext fields
	var sysCtxValueFields []string // types.SystemContext fields
	for field := range structType.Fields() {
		if isSystemContextPtrType(field.Type()) {
			sysCtxPtrFields = append(sysCtxPtrFields, field.Name())
		} else if isSystemContextTypeExact(field.Type()) {
			sysCtxValueFields = append(sysCtxValueFields, field.Name())
		}
	}

	if len(sysCtxPtrFields) == 0 && len(sysCtxValueFields) == 0 {
		return nil
	}

	// Find which fields are explicitly set in the literal
	explicitlySet := make(map[string]bool)
	for _, elt := range compositeLit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if keyIdent, ok := kv.Key.(*ast.Ident); ok {
				explicitlySet[keyIdent.Name] = true
			}
		}
	}

	var results []*useInfo

	// Check if any *SystemContext field is not explicitly set (implicitly nil)
	for _, fieldName := range sysCtxPtrFields {
		if !explicitlySet[fieldName] {
			results = append(results, &useInfo{
				node: compositeLit,
				kind: "implicit-nil-field",
				name: fieldName,
			})
		}
	}

	// Check if any SystemContext field is not explicitly set (implicitly empty)
	for _, fieldName := range sysCtxValueFields {
		if !explicitlySet[fieldName] {
			results = append(results, &useInfo{
				node: compositeLit,
				kind: "implicit-empty-literal-field",
				name: fieldName,
			})
		}
	}

	return results
}

// findEnclosingFunc finds the enclosing function declaration in the stack
// enclosingFunc represents either a FuncDecl or FuncLit
type enclosingFunc struct {
	decl *ast.FuncDecl // non-nil for function declarations
	lit  *ast.FuncLit  // non-nil for function literals
}

func findEnclosingFunc(stack []ast.Node) *enclosingFunc {
	for i := len(stack) - 1; i >= 0; i-- {
		switch f := stack[i].(type) {
		case *ast.FuncDecl:
			return &enclosingFunc{decl: f}
		case *ast.FuncLit:
			return &enclosingFunc{lit: f}
		}
	}
	return nil
}

// getReturnType returns the type of the i-th return value of a function
func getReturnType(ef *enclosingFunc, index int, pkg *packages.Package) (types.Type, error) {
	var sig *types.Signature
	var funcName string

	if ef.decl != nil {
		// Function declaration - get type from Defs
		obj := pkg.TypesInfo.Defs[ef.decl.Name]
		funcName = ef.decl.Name.Name
		if obj == nil {
			return nil, fmt.Errorf("no type info for function %s", funcName)
		}

		funcObj, ok := obj.(*types.Func)
		if !ok {
			return nil, fmt.Errorf("function %s is not a *types.Func: %T", funcName, obj)
		}

		sig = funcObj.Signature()
	} else if ef.lit != nil {
		// Function literal - get type from TypeOf
		funcName = "func literal"
		litType := pkg.TypesInfo.TypeOf(ef.lit)
		if litType == nil {
			return nil, fmt.Errorf("no type info for function literal")
		}

		var ok bool
		sig, ok = litType.(*types.Signature)
		if !ok {
			return nil, fmt.Errorf("function literal type is not a signature: %T", litType)
		}
	} else {
		return nil, fmt.Errorf("enclosingFunc has neither decl nor lit")
	}

	if sig == nil {
		return nil, fmt.Errorf("function %s has no signature", funcName)
	}

	results := sig.Results()
	if results == nil || index >= results.Len() {
		return nil, fmt.Errorf("return index %d out of bounds for function %s (has %d results)",
			index, funcName, results.Len())
	}

	return results.At(index).Type(), nil
}

// getParameterType returns the type of the i-th parameter of a function signature
func getParameterType(sig *types.Signature, argIndex int) (types.Type, error) {
	params := sig.Params()
	if params == nil {
		return nil, fmt.Errorf("no parameters in signature")
	}

	if argIndex >= params.Len() {
		// Handle variadic functions
		if sig.Variadic() && argIndex >= params.Len()-1 && params.Len() > 0 {
			lastParam := params.At(params.Len() - 1)
			// For variadic, the type is a slice, get element type
			if slice, ok := lastParam.Type().(*types.Slice); ok {
				return slice.Elem(), nil
			}
			return lastParam.Type(), nil
		}
		return nil, fmt.Errorf("argument index %d out of bounds (params: %d, variadic: %v)",
			argIndex, params.Len(), sig.Variadic())
	}

	return params.At(argIndex).Type(), nil
}

// getParameterName returns the parameter name at the given index for a function signature
func getParameterName(sig *types.Signature, argIndex int) (string, error) {
	params := sig.Params()
	if params == nil {
		return "", fmt.Errorf("no parameters in signature")
	}

	if argIndex >= params.Len() {
		if sig.Variadic() && argIndex >= params.Len()-1 && params.Len() > 0 {
			return params.At(params.Len() - 1).Name(), nil
		}
		return "", fmt.Errorf("argument index %d out of bounds (params: %d)", argIndex, params.Len())
	}

	return params.At(argIndex).Name(), nil
}

// isSystemContextType checks if a type is types.SystemContext or *types.SystemContext
func isSystemContextType(t types.Type) bool {
	// Handle pointer types
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	return isTypeExact(t, "go.podman.io/image/v5/types", "SystemContext")
}

// isSystemContextTypeExact checks if a type is exactly types.SystemContext (not *types.SystemContext)
func isSystemContextTypeExact(t types.Type) bool {
	return isTypeExact(t, "go.podman.io/image/v5/types", "SystemContext")
}

// isSystemContextPtrType checks if a type is *types.SystemContext specifically
func isSystemContextPtrType(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	return isTypeExact(ptr.Elem(), "go.podman.io/image/v5/types", "SystemContext")
}

// isTypeExact checks if a type exactly matches the given package path and type name
// Does NOT unwrap pointers
func isTypeExact(t types.Type, pkgPath, typeName string) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil {
		return false
	}

	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}

	return pkg.Path() == pkgPath && obj.Name() == typeName
}

// getOriginalExprText returns the original source text of an expression
func getOriginalExprText(expr ast.Expr, fset *token.FileSet) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, expr); err != nil {
		panic(fmt.Sprintf("failed to format expression: %v", err))
	}
	return buf.String()
}
