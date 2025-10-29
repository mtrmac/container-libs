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

// DigestUse represents a use of a digest.Digest value
type DigestUse struct {
	File    string // relative file path
	Line    int    // line number
	Column  int    // column number
	Ignored bool   // true if this is construction/initialization, not a real use
	Kind    string // kind of use (for future filtering)
	Name    string // identifier name
}

// String returns a formatted string representation of the DigestUse
func (du DigestUse) String() string {
	kind := du.Kind
	if du.Ignored {
		kind = "ignored " + kind
	}
	return fmt.Sprintf("%s:%d:%d: %s %s", du.File, du.Line, du.Column, kind, du.Name)
}

func main() {
	var showIgnored bool
	flag.BoolVar(&showIgnored, "show-ignored", false, "include ignored uses (construction/initialization) in output")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <directory>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	dir := flag.Arg(0)
	uses, err := auditDigestUses(dir)
	if err != nil {
		log.Fatalf("Error auditing digest uses: %v", err)
	}

	// Print results in VS Code compatible format
	for _, use := range uses {
		// Skip ignored uses unless -show-ignored flag is set
		if use.Ignored && !showIgnored {
			continue
		}
		fmt.Println(use.String())
	}
}

// auditDigestUses finds all uses of digest.Digest values in the given directory
// Returns all uses (including those with Ignored=true), error
func auditDigestUses(dir string) ([]DigestUse, error) {
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

	var uses []DigestUse

	// Process each package
	for _, pkg := range pkgs {
		if pkg.Types == nil || pkg.TypesInfo == nil {
			return nil, fmt.Errorf("package %s missing type information", pkg.PkgPath)
		}

		// Walk AST of each file in the package
		for _, file := range pkg.Syntax {
			// Track which nodes we've already recorded to avoid duplicates
			// Use separate maps so we don't skip a non-ignored use when an ignored one exists
			recordedReported := make(map[ast.Node]bool)
			recordedIgnored := make(map[ast.Node]bool)

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
				use := determineUse(expr, stack, pkg)

				// Check if we've already recorded this use
				if use.ignored {
					if !recordedIgnored[use.node] {
						recordedIgnored[use.node] = true
						uses = append(uses, recordUse(use, pkg, cwd, inputIsAbsolute))
					}
				} else {
					if !recordedReported[use.node] {
						recordedReported[use.node] = true
						uses = append(uses, recordUse(use, pkg, cwd, inputIsAbsolute))
					}
				}

				return true
			})
		}
	}

	// Sort by file, line, column for consistent output
	slices.SortFunc(uses, func(a, b DigestUse) int {
		return cmp.Or(
			cmp.Compare(a.File, b.File),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Column, b.Column),
		)
	})

	return uses, nil
}

// useInfo describes what use to report for a digest.Digest value
type useInfo struct {
	node    ast.Node // the node to report (position)
	ignored bool     // true if this is construction/initialization, not a real use
	kind    string   // kind of use
	name    string   // descriptive name
}

// recordUse creates a DigestUse record from useInfo
func recordUse(use *useInfo, pkg *packages.Package, cwd string, useAbsolute bool) DigestUse {
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

	return DigestUse{
		File:    filePath,
		Line:    pos.Line,
		Column:  pos.Column,
		Ignored: use.ignored,
		Kind:    use.kind,
		Name:    use.name,
	}
}

// determineUse decides what to report for a digest.Digest expression
// Returns useInfo with ignored=true if this is construction/initialization, not a use
func determineUse(expr ast.Expr, stack []ast.Node, pkg *packages.Package) *useInfo {
	// Get parent from stack
	// PreorderStack provides stack of ancestors, NOT including current node
	// So the parent is the last element in the stack
	var parent ast.Node
	if len(stack) > 0 {
		parent = stack[len(stack)-1]
	}

	// No parent case - this is expected for top-level expressions
	if parent == nil {
		return &useInfo{
			node:    expr,
			ignored: false,
			kind:    determineExprKind(expr),
			name:    getOriginalExprText(expr, pkg.Fset),
		}
	}

	// Handle all known parent situations
	switch p := parent.(type) {
	case *ast.SelectorExpr:
		// expr.Method or expr.Field
		if p.X == expr {
			// Report the selector (method/field access), not the receiver
			return &useInfo{node: p.Sel, ignored: false, kind: "selector", name: p.Sel.Name}
		}
		// Check if expr is the field being selected (p.Sel)
		if p.Sel == expr {
			// This is a field access where the field itself is a digest
			// Pure value move (field access)
			return &useInfo{node: expr, ignored: true, kind: "field-access", name: getOriginalExprText(expr, pkg.Fset)}
		}

	case *ast.BinaryExpr:
		// expr op expr2 or expr2 op expr
		if p.X == expr || p.Y == expr {
			// Report the binary operation
			return &useInfo{node: p, ignored: false, kind: "binary-op", name: p.Op.String()}
		}

	case *ast.UnaryExpr:
		// op expr (like &expr)
		if p.X == expr {
			if p.Op == token.AND {
				// Pure value move (taking address)
				return &useInfo{node: p, ignored: true, kind: "addr-of", name: p.Op.String()}
			}
			// Other unary operations are reported
			return &useInfo{node: p, ignored: false, kind: "unary-op", name: p.Op.String()}
		}

	case *ast.CallExpr:
		// Function/method call or type conversion with expr as argument
		for i, arg := range p.Args {
			if arg == expr {
				// Check if this is a type conversion or a function/method call
				funType := pkg.TypesInfo.TypeOf(p.Fun)
				if _, isType := funType.(*types.Basic); isType {
					// Type conversion (e.g., string(digest))
					return &useInfo{node: expr, ignored: false, kind: "type-cast", name: funType.String()}
				}

				// Check if the type is a named type (also indicates type conversion)
				if _, isNamed := funType.(*types.Named); isNamed {
					// Type conversion to named type
					return &useInfo{node: expr, ignored: false, kind: "type-cast", name: funType.String()}
				}

				// It's a function or method call - get parameter name
				paramName, err := getParameterName(p.Fun, i, pkg)
				if err != nil {
					// Unexpected situation: fall through to unhandled case
					// This will report it rather than ignore it
					break
				}
				// This is a pure value move (passing argument to function)
				return &useInfo{node: expr, ignored: true, kind: "call-arg", name: paramName}
			}
		}

	case *ast.AssignStmt:
		// Assignment involving expr
		for _, rhs := range p.Rhs {
			if rhs == expr {
				// expr is on right-hand side of assignment
				// This is construction/initialization, not a use
				return &useInfo{node: expr, ignored: true, kind: "assign-rhs", name: getOriginalExprText(expr, pkg.Fset)}
			}
		}
		// Check if it's on LHS (being assigned to, or declared with :=)
		for _, lhs := range p.Lhs {
			if lhs == expr {
				// Identifier on LHS of assignment (including := declarations)
				// This is a pure value move (declaration/assignment destination)
				return &useInfo{node: expr, ignored: true, kind: "assign-lhs", name: getOriginalExprText(expr, pkg.Fset)}
			}
		}

	case *ast.ReturnStmt:
		// Return statement with expr
		for _, result := range p.Results {
			if result == expr {
				// Returning a value is not a "use" - it's passing ownership
				return &useInfo{node: expr, ignored: true, kind: "return", name: getOriginalExprText(expr, pkg.Fset)}
			}
		}

	case *ast.ValueSpec:
		// Variable declaration with initialization
		// Check if expr is the Type (type expression in a declaration)
		if p.Type == expr {
			// This is a type expression (e.g., *digest.Digest in var x *digest.Digest)
			// This is a type declaration, not a use of a value
			return &useInfo{node: expr, ignored: true, kind: "var-type", name: getOriginalExprText(expr, pkg.Fset)}
		}
		for _, val := range p.Values {
			if val == expr {
				// expr is an initializer value
				// This is initialization, not a use
				return &useInfo{node: expr, ignored: true, kind: "var-init", name: getOriginalExprText(expr, pkg.Fset)}
			}
		}
		// Check if it's a name being declared
		for _, name := range p.Names {
			if name == expr {
				// The digest is the name being declared
				// This is a pure value move (variable declaration)
				return &useInfo{node: expr, ignored: true, kind: "var-name", name: getOriginalExprText(expr, pkg.Fset)}
			}
		}

	case *ast.CompositeLit:
		// Composite literal containing expr
		for _, elt := range p.Elts {
			if elt == expr {
				// Pure value move (element in literal)
				return &useInfo{node: expr, ignored: true, kind: "composite-lit", name: getExprName(expr, pkg.Fset)}
			}
		}

	case *ast.IndexExpr:
		// Array/slice/map access
		if p.X == expr {
			// digestValue[something] - the digest is being indexed
			return &useInfo{node: expr, ignored: false, kind: "index", name: getExprName(expr, pkg.Fset)}
		}
		if p.Index == expr {
			// something[digestValue] - the digest is used as a key/index
			// Report the collection name (something), not the digest value
			return &useInfo{node: expr, ignored: false, kind: "index-key-in", name: getExprName(p.X, pkg.Fset)}
		}

	case *ast.KeyValueExpr:
		// Key or value in map/struct literal
		if p.Key == expr {
			// Check if this is in a struct literal (field name)
			// Get grandparent (should be CompositeLit)
			var grandparent ast.Node
			if len(stack) >= 2 {
				grandparent = stack[len(stack)-2]
			}

			if compositeLit, ok := grandparent.(*ast.CompositeLit); ok && compositeLit.Type != nil {
				// Check if the composite literal type is a struct
				if litType := pkg.TypesInfo.TypeOf(compositeLit.Type); litType != nil {
					if _, isStruct := litType.Underlying().(*types.Struct); isStruct {
						// Struct key (field name) is ignored
						return &useInfo{node: expr, ignored: true, kind: "struct-key", name: getExprName(expr, pkg.Fset)}
					}
				}
			}
			// Default: map key is reported (used for hashing)
			// This is fail-safe: if we can't determine, assume it's a use
			return &useInfo{node: expr, ignored: false, kind: "map-key", name: getExprName(expr, pkg.Fset)}
		}
		if p.Value == expr {
			// Value is ignored (pure storage) in both map and struct
			return &useInfo{node: expr, ignored: true, kind: "map-value", name: getExprName(expr, pkg.Fset)}
		}

	case *ast.Field:
		// Function parameter, struct field, etc.
		// Check if expr is the Type (type expression in a declaration)
		if p.Type == expr {
			// This is a type expression (e.g., *digest.Digest as parameter type)
			// This is a type declaration, not a use of a value
			return &useInfo{node: expr, ignored: true, kind: "field-type", name: getOriginalExprText(expr, pkg.Fset)}
		}
		// Check if expr is one of the Names
		for _, name := range p.Names {
			if name == expr {
				// This is a parameter/field name
				// This is a pure value move (parameter/field declaration)
				return &useInfo{node: expr, ignored: true, kind: "field-name", name: getOriginalExprText(expr, pkg.Fset)}
			}
		}

	case *ast.RangeStmt:
		// Range statement
		if p.Key == expr {
			// Range key is ignored (iteration index/key variable)
			return &useInfo{node: expr, ignored: true, kind: "range-key", name: getOriginalExprText(expr, pkg.Fset)}
		}
		if p.Value == expr {
			// Range value is ignored (iteration value variable)
			return &useInfo{node: expr, ignored: true, kind: "range-value", name: getOriginalExprText(expr, pkg.Fset)}
		}
		if p.X == expr {
			// Range expression is reported (the collection being iterated over)
			return &useInfo{node: expr, ignored: false, kind: "range-expr", name: getOriginalExprText(expr, pkg.Fset)}
		}

	case *ast.StarExpr:
		// Dereferencing a pointer: *expr
		if p.X == expr {
			// Pure value move (pointer dereference)
			return &useInfo{node: p, ignored: true, kind: "deref", name: "*" + getExprName(expr, pkg.Fset)}
		}

	case *ast.SwitchStmt:
		// Switch statement on a digest value
		if p.Tag == expr {
			// This is the switch tag, a use
			return &useInfo{node: expr, ignored: false, kind: "switch-tag", name: getOriginalExprText(expr, pkg.Fset)}
		}

	case *ast.CaseClause:
		// Case clause in a switch
		for _, caseExpr := range p.List {
			if caseExpr == expr {
				// This is a case value being compared
				return &useInfo{node: expr, ignored: false, kind: "case-value", name: getOriginalExprText(expr, pkg.Fset)}
			}
		}

	case *ast.ArrayType:
		// Array or slice type declaration
		if p.Elt == expr {
			// This is the element type of an array/slice (e.g., []digest.Digest or []*digest.Digest)
			return &useInfo{node: expr, ignored: true, kind: "array-type", name: getOriginalExprText(expr, pkg.Fset)}
		}

	case *ast.ParenExpr:
		// Parenthesized expression - ignored (transparent wrapper)
		return &useInfo{node: expr, ignored: true, kind: "paren", name: getOriginalExprText(expr, pkg.Fset)}
	}

	// Unhandled situation: warn and report the expression itself
	pos := pkg.Fset.Position(expr.Pos())
	exprText := getOriginalExprText(expr, pkg.Fset)
	fmt.Fprintf(os.Stderr, "WARNING: %s:%d:%d: unhandled digest expression %q in parent %s\n",
		pos.Filename, pos.Line, pos.Column, exprText, fmt.Sprintf("%T", parent))

	return &useInfo{node: expr, ignored: false, kind: determineExprKind(expr), name: exprText}
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
func getParameterName(fun ast.Expr, argIndex int, pkg *packages.Package) (string, error) {
	// Get the type of the function being called
	funType := pkg.TypesInfo.TypeOf(fun)
	if funType == nil {
		return "", fmt.Errorf("no type info for function")
	}

	// Extract the signature
	var sig *types.Signature
	switch t := funType.(type) {
	case *types.Signature:
		sig = t
	default:
		return "", fmt.Errorf("function type is not a signature: %T", funType)
	}

	// Get the parameter at the given index
	params := sig.Params()
	if params == nil {
		return "", fmt.Errorf("no parameters in signature")
	}

	if argIndex >= params.Len() {
		// Handle variadic functions
		if sig.Variadic() && argIndex >= params.Len()-1 && params.Len() > 0 {
			// This is a variadic argument
			lastParam := params.At(params.Len() - 1)
			name := lastParam.Name()
			if name == "" {
				return fmt.Sprintf("arg%d", argIndex), nil
			}
			return name, nil
		}
		return "", fmt.Errorf("argument index %d out of bounds (params: %d)", argIndex, params.Len())
	}

	param := params.At(argIndex)
	name := param.Name()
	if name == "" {
		return fmt.Sprintf("arg%d", argIndex), nil
	}

	return name, nil
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
