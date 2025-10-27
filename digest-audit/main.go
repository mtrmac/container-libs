package main

import (
	"fmt"
	"go/ast"
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
				// Look for identifiers
				ident, ok := n.(*ast.Ident)
				if !ok {
					return true
				}

				// Get the type of this identifier
				obj := pkg.TypesInfo.ObjectOf(ident)
				if obj == nil {
					return true
				}

				// Check if the type is digest.Digest
				if isDigestType(obj.Type()) {
					pos := pkg.Fset.Position(ident.Pos())

					// Compute relative path from the input directory
					relPath, err := filepath.Rel(absDir, pos.Filename)
					if err != nil {
						// If we can't compute relative path, use absolute
						relPath = pos.Filename
					}

					// Determine kind of use for future filtering
					kind := determineUseKind(ident, file, pkg)

					uses = append(uses, DigestUse{
						Location: fmt.Sprintf("%s:%d:%d", relPath, pos.Line, pos.Column),
						Name:     ident.Name,
						Kind:     kind,
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

// determineUseKind determines the kind of use for future filtering capabilities
// This is a placeholder implementation that can be expanded
func determineUseKind(ident *ast.Ident, file *ast.File, pkg *packages.Package) string {
	// For now, return a simple classification
	// In the future, this can be expanded to distinguish between:
	// - variable declaration
	// - assignment/copy
	// - method call receiver
	// - function parameter
	// - comparison operand
	// - type conversion
	// - return value
	// etc.

	// Try to determine context by looking at parent nodes
	var parent ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		// Check if this node contains our identifier
		for _, child := range childNodes(n) {
			if child == ident {
				parent = n
				return false
			}
		}
		return true
	})

	if parent == nil {
		return "identifier"
	}

	switch p := parent.(type) {
	case *ast.SelectorExpr:
		if p.X == ident {
			return "method-receiver"
		}
		return "selector"
	case *ast.CallExpr:
		return "call-arg"
	case *ast.BinaryExpr:
		return "binary-op"
	case *ast.AssignStmt:
		return "assignment"
	case *ast.ValueSpec:
		return "declaration"
	case *ast.ReturnStmt:
		return "return"
	default:
		return "identifier"
	}
}

// childNodes returns immediate child nodes of the given node
func childNodes(n ast.Node) []ast.Node {
	var children []ast.Node

	switch node := n.(type) {
	case *ast.SelectorExpr:
		children = append(children, node.X, node.Sel)
	case *ast.CallExpr:
		children = append(children, node.Fun)
		for _, arg := range node.Args {
			children = append(children, arg)
		}
	case *ast.BinaryExpr:
		children = append(children, node.X, node.Y)
	case *ast.AssignStmt:
		for _, lhs := range node.Lhs {
			children = append(children, lhs)
		}
		for _, rhs := range node.Rhs {
			children = append(children, rhs)
		}
	case *ast.ValueSpec:
		for _, name := range node.Names {
			children = append(children, name)
		}
		for _, val := range node.Values {
			children = append(children, val)
		}
	case *ast.ReturnStmt:
		for _, res := range node.Results {
			children = append(children, res)
		}
	}

	return children
}
