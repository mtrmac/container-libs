package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// DigestUse represents a use of a digest.Digest value
type DigestUse struct {
	Location string // file:line:column format
	Kind     string // kind of use (for future filtering)
	Name     string // identifier name
}

// String returns a formatted string representation of the DigestUse
func (du DigestUse) String() string {
	return fmt.Sprintf("%s: %s %s", du.Location, du.Kind, du.Name)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <directory>\n", os.Args[0])
		os.Exit(1)
	}

	dir := os.Args[1]
	uses, err := auditDigestUses(dir)
	if err != nil {
		log.Fatalf("Error auditing digest uses: %v", err)
	}

	// Print results in VS Code compatible format
	for _, use := range uses {
		fmt.Println(use.String())
	}
}

// auditDigestUses finds all uses of digest.Digest values in the given directory
func auditDigestUses(dir string) ([]DigestUse, error) {
	// Convert to absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Configure package loading
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedSyntax,
		Dir: absDir,
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
		return nil, fmt.Errorf("no packages found in %s", absDir)
	}

	var uses []DigestUse

	// Process each package
	for _, pkg := range pkgs {
		if pkg.Types == nil || pkg.TypesInfo == nil {
			return nil, fmt.Errorf("package %s missing type information", pkg.PkgPath)
		}

		// Walk AST of each file in the package
		for _, file := range pkg.Syntax {
			// Track which nodes we've already reported to avoid duplicates
			reported := make(map[ast.Node]bool)

			// Use PreorderStack to walk with parent stack tracking
			ast.PreorderStack(file, nil, func(node ast.Node, stack []ast.Node) bool {
				expr, ok := node.(ast.Expr)
				if !ok {
					return true
				}

				// Skip type expressions - we only want values
				if isTypeExpr(expr, pkg) {
					return true
				}

				// Check if this expression has digest.Digest type
				exprType := pkg.TypesInfo.TypeOf(expr)
				if exprType == nil || !isDigestType(exprType) {
					return true
				}

				// Found a digest.Digest value expression
				// Determine if we should report it or its parent
				parent := getParentFromStack(stack)
				use := determineUse(expr, parent, pkg)

				if use != nil && !reported[use.node] {
					reported[use.node] = true

					pos := pkg.Fset.Position(use.node.Pos())
					relPath, err := filepath.Rel(absDir, pos.Filename)
					if err != nil {
						panic(fmt.Sprintf("failed to compute relative path for %s: %v", pos.Filename, err))
					}

					uses = append(uses, DigestUse{
						Location: fmt.Sprintf("%s:%d:%d", relPath, pos.Line, pos.Column),
						Kind:     use.kind,
						Name:     use.name,
					})
				}

				return true
			})
		}
	}

	// Sort by location for consistent output
	sort.Slice(uses, func(i, j int) bool {
		return uses[i].Location < uses[j].Location
	})

	return uses, nil
}

// useInfo describes what use to report for a digest.Digest value
type useInfo struct {
	node ast.Node // the node to report (position)
	kind string   // kind of use
	name string   // descriptive name
}

// getParentFromStack returns the immediate parent from the stack
func getParentFromStack(stack []ast.Node) ast.Node {
	// PreorderStack provides stack of ancestors, NOT including current node
	// So the parent is the last element in the stack
	if len(stack) == 0 {
		return nil
	}
	return stack[len(stack)-1]
}

// determineUse decides what to report for a digest.Digest expression
// Returns nil if this expression should not be reported (it's part of a larger use)
func determineUse(expr ast.Expr, parent ast.Node, pkg *packages.Package) *useInfo {
	// No parent case - this is expected for top-level expressions
	if parent == nil {
		return &useInfo{
			node: expr,
			kind: determineExprKind(expr),
			name: getOriginalExprText(expr, pkg.Fset),
		}
	}

	// Handle all known parent situations
	switch p := parent.(type) {
	case *ast.SelectorExpr:
		// expr.Method or expr.Field
		if p.X == expr {
			// Report the selector (method/field access), not the receiver
			return &useInfo{
				node: p.Sel,
				kind: "selector",
				name: p.Sel.Name,
			}
		}
		// Check if expr is the field being selected (p.Sel)
		if p.Sel == expr {
			// This is a field access where the field itself is a digest
			// Report it as a field access
			return &useInfo{
				node: expr,
				kind: "field-access",
				name: getOriginalExprText(expr, pkg.Fset),
			}
		}

	case *ast.BinaryExpr:
		// expr op expr2 or expr2 op expr
		if p.X == expr || p.Y == expr {
			// Report the binary operation
			return &useInfo{
				node: p,
				kind: "binary-op",
				name: p.Op.String(),
			}
		}

	case *ast.UnaryExpr:
		// op expr (like &expr or string(expr))
		if p.X == expr {
			// Report the unary operation
			return &useInfo{
				node: p,
				kind: "unary-op",
				name: p.Op.String(),
			}
		}

	case *ast.CallExpr:
		// Function/method call or type conversion with expr as argument
		for i, arg := range p.Args {
			if arg == expr {
				// Check if this is a type conversion or a function/method call
				funType := pkg.TypesInfo.TypeOf(p.Fun)
				if _, isType := funType.(*types.Basic); isType {
					// Type conversion (e.g., string(digest))
					return &useInfo{
						node: expr,
						kind: "type-cast",
						name: funType.String(),
					}
				}

				// Check if the type is a named type (also indicates type conversion)
				if _, isNamed := funType.(*types.Named); isNamed {
					// Type conversion to named type
					return &useInfo{
						node: expr,
						kind: "type-cast",
						name: funType.String(),
					}
				}

				// It's a function or method call - get parameter name
				paramName := getParameterName(p.Fun, i, pkg)
				return &useInfo{
					node: expr,
					kind: "call-arg",
					name: paramName,
				}
			}
		}

	case *ast.AssignStmt:
		// Assignment involving expr
		for _, rhs := range p.Rhs {
			if rhs == expr {
				// expr is on right-hand side of assignment
				// Don't report - this is just construction/initialization, not a use
				return nil
			}
		}
		// Check if it's on LHS (being assigned to, or declared with :=)
		for _, lhs := range p.Lhs {
			if lhs == expr {
				// Identifier on LHS of assignment (including := declarations)
				// Report it - it's a digest-typed identifier being declared/assigned
				return &useInfo{
					node: expr,
					kind: determineExprKind(expr),
					name: getOriginalExprText(expr, pkg.Fset),
				}
			}
		}

	case *ast.ReturnStmt:
		// Return statement with expr
		for _, result := range p.Results {
			if result == expr {
				// Don't report - returning a value is not a "use"
				return nil
			}
		}

	case *ast.ValueSpec:
		// Variable declaration with initialization
		// Check if expr is the Type (type expression in a declaration)
		if p.Type == expr {
			// This is a type expression (e.g., *digest.Digest in var x *digest.Digest)
			// Don't report - this is a type declaration, not a use of a value
			return nil
		}
		for _, val := range p.Values {
			if val == expr {
				// expr is an initializer value
				// Don't report - this is just initialization, not a use
				return nil
			}
		}
		// Check if it's a name being declared
		for _, name := range p.Names {
			if name == expr {
				// The digest is the name being declared
				// Report it - it's a digest-typed identifier being declared
				return &useInfo{
					node: expr,
					kind: determineExprKind(expr),
					name: getOriginalExprText(expr, pkg.Fset),
				}
			}
		}

	case *ast.CompositeLit:
		// Composite literal containing expr
		for _, elt := range p.Elts {
			if elt == expr {
				return &useInfo{
					node: expr,
					kind: "composite-lit",
					name: getExprName(expr, pkg.Fset),
				}
			}
		}

	case *ast.IndexExpr:
		// Array/slice/map access
		if p.Index == expr {
			return &useInfo{
				node: expr,
				kind: "index",
				name: getExprName(expr, pkg.Fset),
			}
		}

	case *ast.KeyValueExpr:
		// Key or value in map/struct literal
		if p.Key == expr || p.Value == expr {
			return &useInfo{
				node: expr,
				kind: "key-value",
				name: getExprName(expr, pkg.Fset),
			}
		}

	case *ast.Field:
		// Function parameter, struct field, etc.
		// Check if expr is the Type (type expression in a declaration)
		if p.Type == expr {
			// This is a type expression (e.g., *digest.Digest as parameter type)
			// Don't report - this is a type declaration, not a use of a value
			return nil
		}
		// Check if expr is one of the Names
		for _, name := range p.Names {
			if name == expr {
				// This is a parameter/field name
				// Report it - it's a digest-typed identifier being declared as parameter/field
				return &useInfo{
					node: expr,
					kind: determineExprKind(expr),
					name: getOriginalExprText(expr, pkg.Fset),
				}
			}
		}

	case *ast.RangeStmt:
		// Range over digest values
		if p.Key == expr || p.Value == expr {
			// This is the loop variable in a range statement
			return &useInfo{
				node: expr,
				kind: "range-var",
				name: getOriginalExprText(expr, pkg.Fset),
			}
		}

	case *ast.StarExpr:
		// Dereferencing a pointer: *expr
		if p.X == expr {
			// Report the dereference operation
			return &useInfo{
				node: p,
				kind: "deref",
				name: "*" + getExprName(expr, pkg.Fset),
			}
		}

	case *ast.SwitchStmt:
		// Switch statement on a digest value
		if p.Tag == expr {
			// Don't report - this is the switch tag, not a use
			return nil
		}

	case *ast.CaseClause:
		// Case clause in a switch
		for _, caseExpr := range p.List {
			if caseExpr == expr {
				// This is a case value being compared
				return &useInfo{
					node: expr,
					kind: "case-value",
					name: getOriginalExprText(expr, pkg.Fset),
				}
			}
		}
	}

	// Unhandled situation: warn and report the expression itself
	pos := pkg.Fset.Position(expr.Pos())
	exprText := getOriginalExprText(expr, pkg.Fset)
	fmt.Fprintf(os.Stderr, "WARNING: %s:%d:%d: unhandled digest expression %q in parent %s\n",
		pos.Filename, pos.Line, pos.Column, exprText, fmt.Sprintf("%T", parent))

	return &useInfo{
		node: expr,
		kind: determineExprKind(expr),
		name: exprText,
	}
}

// getOriginalExprText returns the original source text of an expression
func getOriginalExprText(expr ast.Expr, fset *token.FileSet) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, expr); err != nil {
		panic(fmt.Sprintf("failed to format expression: %v", err))
	}
	return buf.String()
}

// getParameterName returns the parameter name at the given index for a function/method call
func getParameterName(fun ast.Expr, argIndex int, pkg *packages.Package) string {
	// Get the type of the function being called
	funType := pkg.TypesInfo.TypeOf(fun)
	if funType == nil {
		return fmt.Sprintf("arg%d", argIndex)
	}

	// Extract the signature
	var sig *types.Signature
	switch t := funType.(type) {
	case *types.Signature:
		sig = t
	default:
		return fmt.Sprintf("arg%d", argIndex)
	}

	// Get the parameter at the given index
	params := sig.Params()
	if params == nil || argIndex >= params.Len() {
		// Handle variadic functions or out of bounds
		if sig.Variadic() && argIndex >= params.Len()-1 && params.Len() > 0 {
			// This is a variadic argument
			lastParam := params.At(params.Len() - 1)
			return lastParam.Name()
		}
		return fmt.Sprintf("arg%d", argIndex)
	}

	param := params.At(argIndex)
	if param.Name() == "" {
		return fmt.Sprintf("arg%d", argIndex)
	}

	return param.Name()
}

// isTypeExpr checks if an expression is being used as a type (not a value)
func isTypeExpr(expr ast.Expr, pkg *packages.Package) bool {
	// Check if this expression is recorded as a type use in TypesInfo
	// TypesInfo.Uses maps identifiers to their objects, but type names
	// are recorded differently - they map to TypeName objects
	if ident, ok := expr.(*ast.Ident); ok {
		if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
			_, isTypeName := obj.(*types.TypeName)
			return isTypeName
		}
		if obj := pkg.TypesInfo.Defs[ident]; obj != nil {
			_, isTypeName := obj.(*types.TypeName)
			return isTypeName
		}
	}

	// Check if it's a selector expression referring to a type
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if obj := pkg.TypesInfo.Uses[sel.Sel]; obj != nil {
			_, isTypeName := obj.(*types.TypeName)
			return isTypeName
		}
	}

	return false
}

// isDigestType checks if a type is github.com/opencontainers/go-digest.Digest
func isDigestType(t types.Type) bool {
	// Handle pointer types
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	// Get the named type
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	// Check package path and type name
	obj := named.Obj()
	if obj == nil {
		return false
	}

	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}

	return pkg.Path() == "github.com/opencontainers/go-digest" && obj.Name() == "Digest"
}

// determineExprKind determines the kind of expression for future filtering capabilities
func determineExprKind(expr ast.Expr) string {
	switch expr.(type) {
	case *ast.Ident:
		return "identifier"
	case *ast.CallExpr:
		return "call-result"
	case *ast.SelectorExpr:
		return "selector"
	case *ast.BinaryExpr:
		return "binary-op"
	case *ast.UnaryExpr:
		return "unary-op"
	case *ast.ParenExpr:
		return "paren"
	case *ast.TypeAssertExpr:
		return "type-assert"
	case *ast.IndexExpr:
		return "index"
	case *ast.StarExpr:
		return "deref"
	case *ast.BasicLit:
		return "literal"
	default:
		return "expression"
	}
}

// getExprName returns a descriptive name for an expression use
func getExprName(expr ast.Expr, fset *token.FileSet) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.CallExpr:
		// This is a call that returns digest.Digest
		// Get the function name being called
		if fun, ok := e.Fun.(*ast.SelectorExpr); ok {
			return fun.Sel.Name + "()"
		} else if fun, ok := e.Fun.(*ast.Ident); ok {
			return fun.Name + "()"
		}
		return "call()"
	case *ast.SelectorExpr:
		// Method call or field access
		return e.Sel.Name
	case *ast.BasicLit:
		return e.Value
	case *ast.BinaryExpr:
		// Binary operation involving digest.Digest
		return e.Op.String()
	case *ast.UnaryExpr:
		// Unary operation
		return e.Op.String()
	case *ast.ParenExpr:
		return getExprName(e.X, fset)
	case *ast.TypeAssertExpr:
		return "type-assert"
	case *ast.IndexExpr:
		return "index"
	case *ast.StarExpr:
		return "*"
	default:
		return "expr"
	}
}
