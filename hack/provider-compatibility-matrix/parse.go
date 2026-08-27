// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Constructor names in the apistatus package. Only NewUnsupportedFieldError
// counts toward the matrix — it is the "this device cannot do X" signal.
// NewInvalidArgumentError (malformed input) and NewFailedPreconditionError
// (transient device state) are universal, not capability gaps.
const (
	apistatusPkgPath    = "github.com/ironcore-dev/network-operator/internal/apistatus"
	unsupportedCtor     = "NewUnsupportedFieldError"
	invalidArgumentCtor = "NewInvalidArgumentError"
)

// collectViolations returns, per kind, the spec field paths a provider's EnsureX
// method rejects with NewUnsupportedFieldError, following same-package calls to
// any depth. No rejected fields renders as full; one or more as partial.
func collectViolations(pkg *packages.Package, kinds []string) map[string]map[string]bool {
	ensureToKind := make(map[string]string, len(kinds))
	for _, k := range kinds {
		ensureToKind["Ensure"+k] = k
	}
	// Index every function/method declaration by its *types.Func so callee
	// resolution during the walk is an O(1) lookup.
	funcDecls := indexFuncDecls(pkg)

	out := make(map[string]map[string]bool)
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			kind, ok := ensureToKind[fn.Name.Name]
			if !ok {
				continue
			}
			for _, field := range collectFieldsFrom(pkg, funcDecls, fn, map[*types.Func]bool{}) {
				if out[kind] == nil {
					out[kind] = make(map[string]bool)
				}
				out[kind][field] = true
			}
		}
	}
	return out
}

// indexFuncDecls maps each in-package function/method to its declaration.
func indexFuncDecls(pkg *packages.Package) map[*types.Func]*ast.FuncDecl {
	out := make(map[*types.Func]*ast.FuncDecl)
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func); ok {
				out[obj] = fn
			}
		}
	}
	return out
}

// collectFieldsFrom returns every unsupported spec field path reachable from fn:
// literals raised directly, plus those in same-package callees (transitively).
// visited guards against cycles.
func collectFieldsFrom(pkg *packages.Package, funcDecls map[*types.Func]*ast.FuncDecl, fn *ast.FuncDecl, visited map[*types.Func]bool) []string {
	// A STYLE 2 body collects FieldViolation literals into a slice and spreads it
	// into NewUnsupportedFieldError(violations...). Capturing every literal in the
	// body is only safe when the body raises no NewInvalidArgumentError — otherwise
	// an InvalidArgument field would be misattributed as unsupported.
	hasInvalidArgument := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false // don't descend into closures
		}
		if call, ok := n.(*ast.CallExpr); ok && isAPIStatusCall(pkg, call, invalidArgumentCtor) {
			hasInvalidArgument = true
		}
		return true
	})

	var fields []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false // don't descend into closures
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if isAPIStatusCall(pkg, call, unsupportedCtor) {
			// STYLE 1: FieldViolation literals passed directly to the constructor.
			hadDirectLit := false
			for _, arg := range call.Args {
				if lit, ok := arg.(*ast.CompositeLit); ok && isAPIStatusLit(pkg, lit, "FieldViolation") {
					hadDirectLit = true
					if field := fieldValue(lit); field != "" {
						fields = append(fields, field)
					}
				}
			}
			// STYLE 2: the constructor received a spread slice (violations...), not
			// literals. Capture every FieldViolation literal in the body, but only
			// when the body cannot also produce an InvalidArgument violation.
			if !hadDirectLit && !hasInvalidArgument {
				fields = append(fields, allFieldLiterals(pkg, fn.Body)...)
			}
			return true
		}
		// Any other call: follow it if it resolves to an in-package declaration.
		if callee, decl := calleeDecl(pkg, funcDecls, call); decl != nil && !visited[callee] {
			visited[callee] = true
			fields = append(fields, collectFieldsFrom(pkg, funcDecls, decl, visited)...)
		}
		return true
	})
	return fields
}

// allFieldLiterals returns the field path of every FieldViolation literal in body.
func allFieldLiterals(pkg *packages.Package, body *ast.BlockStmt) []string {
	var fields []string
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false // don't descend into closures
		}
		if lit, ok := n.(*ast.CompositeLit); ok && isAPIStatusLit(pkg, lit, "FieldViolation") {
			if field := fieldValue(lit); field != "" {
				fields = append(fields, field)
			}
		}
		return true
	})
	return fields
}

// calleeDecl resolves a call to its declaration, but only when that declaration
// lives in pkg. Returns (nil, nil) for calls into other packages.
func calleeDecl(pkg *packages.Package, funcDecls map[*types.Func]*ast.FuncDecl, call *ast.CallExpr) (*types.Func, *ast.FuncDecl) {
	var ident *ast.Ident
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		ident = fn
	case *ast.SelectorExpr:
		ident = fn.Sel
	default:
		return nil, nil
	}
	obj, ok := pkg.TypesInfo.Uses[ident].(*types.Func)
	if !ok {
		return nil, nil
	}
	return obj, funcDecls[obj]
}

// calleeName returns the name of a call's callee, or "" if it is neither a
// selector nor a bare identifier.
func calleeName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fn.Sel.Name
	case *ast.Ident:
		return fn.Name
	default:
		return ""
	}
}

// isAPIStatusCall reports whether call invokes a function with the given name
// from the apistatus package. For qualified calls (pkg.Func), the package is
// verified through type info; bare identifiers are trusted by name alone since
// they are either dot-imports or same-package definitions.
func isAPIStatusCall(pkg *packages.Package, call *ast.CallExpr, funcName string) bool {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		if fn.Sel.Name != funcName {
			return false
		}
		obj, ok := pkg.TypesInfo.Uses[fn.Sel].(*types.Func)
		if !ok {
			return fn.Sel.Name == funcName
		}
		return obj.Pkg() != nil && obj.Pkg().Path() == apistatusPkgPath
	case *ast.Ident:
		return fn.Name == funcName
	default:
		return false
	}
}

// isAPIStatusLit reports whether a composite literal is a struct with the given
// type name from the apistatus package. For qualified types (pkg.Type), the
// package is verified; bare identifiers are trusted by name alone.
func isAPIStatusLit(pkg *packages.Package, lit *ast.CompositeLit, typeName string) bool {
	switch t := lit.Type.(type) {
	case *ast.SelectorExpr:
		if t.Sel.Name != typeName {
			return false
		}
		obj, ok := pkg.TypesInfo.Uses[t.Sel].(*types.TypeName)
		if !ok {
			return t.Sel.Name == typeName
		}
		return obj.Pkg() != nil && obj.Pkg().Path() == apistatusPkgPath
	case *ast.Ident:
		return t.Name == typeName
	default:
		return false
	}
}

// fieldValue extracts the field path from a FieldViolation literal's Field element.
// See staticString for how dynamic values are handled.
func fieldValue(lit *ast.CompositeLit) string {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Field" {
			continue
		}
		return staticString(kv.Value)
	}
	return ""
}

// staticString resolves an expression to a static field path: a string literal
// verbatim, or a Sprintf format string with verbs stripped. "" for anything else.
func staticString(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if s, err := strconv.Unquote(v.Value); err == nil {
			return s
		}
	case *ast.CallExpr:
		if calleeName(v) == "Sprintf" && len(v.Args) > 0 {
			if lit, ok := v.Args[0].(*ast.BasicLit); ok {
				if s, err := strconv.Unquote(lit.Value); err == nil {
					return stripVerbs(s)
				}
			}
		}
	}
	return ""
}

// stripVerbs removes fmt verb runs (%s, %d, %[1]v, %%, ...) from a format string,
// turning "spec.servers[%s].vrfName" into "spec.servers[].vrfName".
func stripVerbs(format string) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			b.WriteByte(format[i])
			continue
		}
		i++ // skip '%'
		if i < len(format) && format[i] == '%' {
			b.WriteByte('%') // literal percent
			continue
		}
		// Skip flags, width, precision, arg index, and the verb letter.
		for i < len(format) && !isVerbLetter(format[i]) {
			i++
		}
		// i now points at the verb letter; the loop's i++ consumes it.
	}
	return b.String()
}

// isVerbLetter reports whether b terminates a fmt verb (an ASCII letter).
func isVerbLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
