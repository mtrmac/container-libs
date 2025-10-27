package main

import (
	"fmt"
	"go/ast"
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
	Name     string // identifier name
	Kind     string // kind of use (for future filtering)
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
		fmt.Printf("%s: %s\n", use.Location, use.Name)
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

	// Check for errors in loaded packages
	var loadErrors []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", pkg.PkgPath, e))
			}
		}
	}
	// Only fail if we have errors and no packages loaded successfully
	if len(loadErrors) > 0 && len(pkgs) == 0 {
		return nil, fmt.Errorf("failed to load any packages:\n%s", strings.Join(loadErrors, "\n"))
	}

	var uses []DigestUse

	// Process each package
	for _, pkg := range pkgs {
		if pkg.Types == nil || pkg.TypesInfo == nil {
			continue
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
						relPath = pos.Filename
					}

					uses = append(uses, DigestUse{
						Location: fmt.Sprintf("%s:%d:%d", relPath, pos.Line, pos.Column),
						Name:     use.name,
						Kind:     use.kind,
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
	name string   // descriptive name
	kind string   // kind of use
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
	if parent == nil {
		// No parent, report the expression itself
		return &useInfo{
			node: expr,
			name: getExprName(expr, pkg.Fset),
			kind: determineExprKind(expr),
		}
	}

	switch p := parent.(type) {
	case *ast.SelectorExpr:
		// expr.Method or expr.Field
		if p.X == expr {
			// Report the selector (method/field access), not the receiver
			return &useInfo{
				node: p.Sel,
				name: p.Sel.Name,
				kind: "selector",
			}
		}

	case *ast.BinaryExpr:
		// expr op expr2 or expr2 op expr
		if p.X == expr || p.Y == expr {
			// Report the binary operation
			return &useInfo{
				node: p,
				name: p.Op.String(),
				kind: "binary-op",
			}
		}

	case *ast.UnaryExpr:
		// op expr (like &expr or string(expr))
		if p.X == expr {
			// Report the unary operation
			return &useInfo{
				node: p,
				name: p.Op.String(),
				kind: "unary-op",
			}
		}

	case *ast.CallExpr:
		// Function call with expr as argument
		for _, arg := range p.Args {
			if arg == expr {
				// Report the argument
				return &useInfo{
					node: expr,
					name: getExprName(expr, pkg.Fset),
					kind: "call-arg",
				}
			}
		}
		// expr is not an argument, might be the function being called (shouldn't happen for digest.Digest)

	case *ast.AssignStmt:
		// Assignment involving expr
		for _, rhs := range p.Rhs {
			if rhs == expr {
				// expr is on right-hand side of assignment
				// Don't report - this is just construction/initialization, not a use
				return nil
			}
		}
		// If expr is on LHS, fall through to default handling

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
		for _, val := range p.Values {
			if val == expr {
				// expr is an initializer value
				// Don't report - this is just initialization, not a use
				return nil
			}
		}

	case *ast.CompositeLit:
		// Composite literal containing expr
		for _, elt := range p.Elts {
			if elt == expr {
				return &useInfo{
					node: expr,
					name: getExprName(expr, pkg.Fset),
					kind: "composite-lit",
				}
			}
		}

	case *ast.IndexExpr:
		// Array/slice/map access
		if p.Index == expr {
			return &useInfo{
				node: expr,
				name: getExprName(expr, pkg.Fset),
				kind: "index",
			}
		}

	case *ast.KeyValueExpr:
		// Key or value in map/struct literal
		if p.Key == expr || p.Value == expr {
			return &useInfo{
				node: expr,
				name: getExprName(expr, pkg.Fset),
				kind: "key-value",
			}
		}
	}

	// Default: report the expression itself
	return &useInfo{
		node: expr,
		name: getExprName(expr, pkg.Fset),
		kind: determineExprKind(expr),
	}
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
