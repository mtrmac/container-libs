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
			ast.Inspect(file, func(n ast.Node) bool {
				// Look for uses of digest.Digest values
				switch node := n.(type) {
				case *ast.Ident:
					// Identifier with digest.Digest type
					if isTypeExpr(node, pkg) {
						return true
					}
					exprType := pkg.TypesInfo.TypeOf(node)
					if exprType != nil && isDigestType(exprType) {
						// Check if this identifier is being used (not just produced)
						if !isSubexprOfOperation(node, file) {
							pos := pkg.Fset.Position(node.Pos())
							relPath, err := filepath.Rel(absDir, pos.Filename)
							if err != nil {
								relPath = pos.Filename
							}
							uses = append(uses, DigestUse{
								Location: fmt.Sprintf("%s:%d:%d", relPath, pos.Line, pos.Column),
								Name:     node.Name,
								Kind:     "identifier",
							})
						}
					}
					
				case *ast.SelectorExpr:
					// Method call on digest.Digest value
					xType := pkg.TypesInfo.TypeOf(node.X)
					if xType != nil && isDigestType(xType) {
						pos := pkg.Fset.Position(node.Sel.Pos())
						relPath, err := filepath.Rel(absDir, pos.Filename)
						if err != nil {
							relPath = pos.Filename
						}
						uses = append(uses, DigestUse{
							Location: fmt.Sprintf("%s:%d:%d", relPath, pos.Line, pos.Column),
							Name:     node.Sel.Name,
							Kind:     "selector",
						})
					}
					
				case *ast.BinaryExpr:
					// Binary operation involving digest.Digest
					xType := pkg.TypesInfo.TypeOf(node.X)
					yType := pkg.TypesInfo.TypeOf(node.Y)
					if (xType != nil && isDigestType(xType)) || (yType != nil && isDigestType(yType)) {
						pos := pkg.Fset.Position(node.Pos())
						relPath, err := filepath.Rel(absDir, pos.Filename)
						if err != nil {
							relPath = pos.Filename
						}
						uses = append(uses, DigestUse{
							Location: fmt.Sprintf("%s:%d:%d", relPath, pos.Line, pos.Column),
							Name:     node.Op.String(),
							Kind:     "binary-op",
						})
					}
					
				case *ast.CallExpr:
					// Function call returning digest.Digest or taking digest.Digest as argument
					for _, arg := range node.Args {
						argType := pkg.TypesInfo.TypeOf(arg)
						if argType != nil && isDigestType(argType) {
							pos := pkg.Fset.Position(arg.Pos())
							relPath, err := filepath.Rel(absDir, pos.Filename)
							if err != nil {
								relPath = pos.Filename
							}
							// Report the argument itself
							name := getExprName(arg, pkg.Fset)
							uses = append(uses, DigestUse{
								Location: fmt.Sprintf("%s:%d:%d", relPath, pos.Line, pos.Column),
								Name:     name,
								Kind:     "call-arg",
							})
						}
					}
					
				case *ast.UnaryExpr:
					// Type conversion involving digest.Digest
					xType := pkg.TypesInfo.TypeOf(node.X)
					if xType != nil && isDigestType(xType) {
						pos := pkg.Fset.Position(node.Pos())
						relPath, err := filepath.Rel(absDir, pos.Filename)
						if err != nil {
							relPath = pos.Filename
						}
						uses = append(uses, DigestUse{
							Location: fmt.Sprintf("%s:%d:%d", relPath, pos.Line, pos.Column),
							Name:     node.Op.String(),
							Kind:     "unary-op",
						})
					}
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

// isSubexprOfOperation checks if an identifier is part of a larger operation
// that we'll report separately (like being the receiver of a method call)
func isSubexprOfOperation(ident *ast.Ident, file *ast.File) bool {
	var isSubexpr bool
	
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil || isSubexpr {
			return false
		}
		
		switch p := n.(type) {
		case *ast.SelectorExpr:
			// If ident is the X in X.Sel (receiver of method call), it's a subexpression
			if p.X == ident {
				isSubexpr = true
				return false
			}
			
		case *ast.BinaryExpr:
			// If ident is an operand in a binary operation, it's a subexpression
			if p.X == ident || p.Y == ident {
				isSubexpr = true
				return false
			}
			
		case *ast.UnaryExpr:
			// If ident is being type-converted or operated on, it's a subexpression
			if p.X == ident {
				isSubexpr = true
				return false
			}
			
		case *ast.CallExpr:
			// If ident is an argument in a call, it's a subexpression
			for _, arg := range p.Args {
				if arg == ident {
					isSubexpr = true
					return false
				}
			}
		}
		
		return true
	})
	
	return isSubexpr
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
