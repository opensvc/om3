package helptest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	// sourceTree reads the packages of the module, for the checks that
	// need to see what a command does with its options.
	sourceTree struct {
		modulePath string
		moduleDir  string
		fset       *token.FileSet
		pkgs       map[string][]*ast.File
	}

	// typeRef names a type of the module: the import path of its package
	// and its name.
	typeRef struct {
		pkg  string
		name string
	}
)

// knownUnrouted are the options types of the commands offering --node that
// do not submit their action to the nodes, with what is wrong. The check
// fails on any other type, and on a type of this list that is routed now,
// for it to leave the list.
var knownUnrouted = map[string]string{}

// nodeSelectorUsage is the usage of the --node option of a command submitting
// its action to the selected nodes, as commoncmd.FlagNodeSelector and its
// variants declare it.
const nodeSelectorUsage = "submit the action to the selected nodes"

// NodeSelectorRouted checks that a command offering --node as the nodes to
// submit the action to does submit it to them.
//
// The help of "om node scsi prkey" offered --node, and the command answered
// "RemoteFunc is nil" with it: its nodeaction had no remote function. The
// "om node compliance" commands offered it too, and never read it, so they
// ran on the local node whatever node was named.
//
// The function a command runs is found by its name, and its source is read:
// the options it declares --node into, the methods of their type, and the
// actions these methods build. A command passes when one of the methods reads
// the node selector, and none builds an action routed to the nodes without
// a function running it there.
func NodeSelectorRouted(t *testing.T, root *cobra.Command) {
	t.Helper()
	src, err := newSourceTree()
	if err != nil {
		t.Fatalf("read the sources of the module: %s", err)
	}
	var (
		visited = make(map[string]bool)
		failed  = make(map[string]bool)
	)
	commoncmd.WalkCommands(root, func(cmd *cobra.Command) {
		flag := cmd.LocalNonPersistentFlags().Lookup("node")
		if flag == nil || flag.Usage != nodeSelectorUsage {
			return
		}
		opts, err := src.checkNodeSelectorRouted(cmd)
		key := opts.key()
		if key != "" {
			visited[key] = true
		}
		if err == nil {
			return
		}
		if _, ok := knownUnrouted[key]; ok {
			failed[key] = true
			return
		}
		t.Errorf("%q offers --node: %s", cmd.CommandPath(), err)
	})
	for key := range knownUnrouted {
		if visited[key] && !failed[key] {
			t.Errorf("%s routes --node now: drop it from knownUnrouted", key)
		}
	}
}

// key names the type as knownUnrouted does: its package name and its name.
func (t typeRef) key() string {
	if t.name == "" {
		return ""
	}
	return filepath.Base(t.pkg) + "." + t.name
}

func newSourceTree() (*sourceTree, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if s, ok := strings.CutPrefix(line, "module "); ok {
					return &sourceTree{
						modulePath: strings.TrimSpace(s),
						moduleDir:  dir,
						fset:       token.NewFileSet(),
						pkgs:       make(map[string][]*ast.File),
					}, nil
				}
			}
			return nil, fmt.Errorf("%s/go.mod declares no module", dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no go.mod above the working directory")
		}
		dir = parent
	}
}

// files returns the parsed non-test files of the package of the module at
// the import path.
func (s *sourceTree) files(pkg string) ([]*ast.File, error) {
	if l, ok := s.pkgs[pkg]; ok {
		return l, nil
	}
	rel, ok := strings.CutPrefix(pkg, s.modulePath)
	if !ok {
		return nil, fmt.Errorf("package %s is not of the module %s", pkg, s.modulePath)
	}
	dir := filepath.Join(s.moduleDir, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var l []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(s.fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		l = append(l, f)
	}
	s.pkgs[pkg] = l
	return l, nil
}

// checkNodeSelectorRouted reads what the command does with the node
// selector it declares, and returns the type of the options holding it.
func (s *sourceTree) checkNodeSelectorRouted(cmd *cobra.Command) (typeRef, error) {
	var fn any
	switch {
	case cmd.RunE != nil:
		fn = cmd.RunE
	case cmd.Run != nil:
		fn = cmd.Run
	default:
		return typeRef{}, fmt.Errorf("the command runs nothing")
	}
	pkg, factory, err := funcOrigin(fn)
	if err != nil {
		return typeRef{}, err
	}
	decl, file, err := s.funcDecl(pkg, factory)
	if err != nil {
		return typeRef{}, err
	}
	opts, local, err := s.optionsType(pkg, decl, file)
	if err != nil {
		return typeRef{}, fmt.Errorf("%s.%s: %w", pkg, factory, err)
	}
	if local != "" {
		// The selector is a variable of the function building the
		// command: declared, given to the flag, and read by the run
		// function, or by nothing.
		if countIdent(decl.Body, local) < 3 {
			return typeRef{}, fmt.Errorf("%s.%s never reads %s", pkg, factory, local)
		}
		return typeRef{}, nil
	}
	return opts, s.checkMethods(opts)
}

// funcOrigin returns the package and the name of the function declaring fn,
// a function literal of a function building a command.
func funcOrigin(fn any) (string, string, error) {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	i := strings.LastIndex(name, "/")
	if i < 0 {
		return "", "", fmt.Errorf("unexpected run function name %s", name)
	}
	pkgName, rest, ok := strings.Cut(name[i+1:], ".")
	if !ok {
		return "", "", fmt.Errorf("unexpected run function name %s", name)
	}
	outer, _, _ := strings.Cut(rest, ".")
	if outer == "" || strings.HasPrefix(outer, "(") {
		return "", "", fmt.Errorf("run function %s is not declared in a function building the command", name)
	}
	return name[:i+1] + pkgName, outer, nil
}

func (s *sourceTree) funcDecl(pkg, name string) (*ast.FuncDecl, *ast.File, error) {
	files, err := s.files(pkg)
	if err != nil {
		return nil, nil, err
	}
	for _, f := range files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == name {
				return fd, f, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("function %s.%s not found", pkg, name)
}

// optionsType returns the type of the variable --node is declared into, or
// the name of that variable when it is a plain string of the function.
func (s *sourceTree) optionsType(pkg string, decl *ast.FuncDecl, file *ast.File) (typeRef, string, error) {
	var target ast.Expr
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || target != nil {
			return target == nil
		}
		if !strings.Contains(callName(call), "NodeSelector") {
			return true
		}
		for _, arg := range call.Args {
			if u, ok := arg.(*ast.UnaryExpr); ok && u.Op == token.AND {
				target = u.X
				return false
			}
		}
		return true
	})
	var varName string
	switch v := target.(type) {
	case nil:
		// Declared by a helper adding the options of a family of
		// commands, as commoncmd.AddFlagsNodeGlobal: the options are
		// the variable of the function by convention.
		varName = "options"
	case *ast.SelectorExpr:
		id, ok := v.X.(*ast.Ident)
		if !ok {
			return typeRef{}, "", fmt.Errorf("unexpected --node target")
		}
		varName = id.Name
	case *ast.Ident:
		if typ := localVarType(decl, v.Name); typ != nil {
			if id, ok := typ.(*ast.Ident); ok && id.Name == "string" {
				return typeRef{}, v.Name, nil
			}
		}
		varName = v.Name
	default:
		return typeRef{}, "", fmt.Errorf("unexpected --node target")
	}
	typ := localVarType(decl, varName)
	if typ == nil {
		return typeRef{}, "", fmt.Errorf("the type of %s is not found", varName)
	}
	return resolveType(pkg, file, typ)
}

// callName returns the name of the function called, without its package.
func callName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// localVarType returns the type expression of the variable declared in the
// function, by a var declaration or a composite literal assignment.
func localVarType(decl *ast.FuncDecl, name string) ast.Expr {
	var typ ast.Expr
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		if typ != nil {
			return false
		}
		switch v := n.(type) {
		case *ast.ValueSpec:
			for i, id := range v.Names {
				if id.Name != name {
					continue
				}
				if v.Type != nil {
					typ = v.Type
				} else if i < len(v.Values) {
					typ = literalType(v.Values[i])
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range v.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name && i < len(v.Rhs) {
					typ = literalType(v.Rhs[i])
				}
			}
		}
		return true
	})
	return typ
}

func literalType(e ast.Expr) ast.Expr {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	if c, ok := e.(*ast.CompositeLit); ok {
		return c.Type
	}
	return nil
}

func resolveType(pkg string, file *ast.File, typ ast.Expr) (typeRef, string, error) {
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	switch v := typ.(type) {
	case *ast.Ident:
		return typeRef{pkg: pkg, name: v.Name}, "", nil
	case *ast.SelectorExpr:
		alias, ok := v.X.(*ast.Ident)
		if !ok {
			return typeRef{}, "", fmt.Errorf("unexpected options type")
		}
		for _, imp := range file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			name := filepath.Base(path)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			if name == alias.Name {
				return typeRef{pkg: path, name: v.Sel.Name}, "", nil
			}
		}
		return typeRef{}, "", fmt.Errorf("package %s not imported", alias.Name)
	}
	return typeRef{}, "", fmt.Errorf("unexpected options type")
}

// checkMethods checks the methods of the options type, and of the types it
// embeds, read the node selector, and build no action routed to nodes
// without a function to run there.
func (s *sourceTree) checkMethods(opts typeRef) error {
	var (
		reads    bool
		problems []string
	)
	found, err := s.walkMethods(opts, func(ref typeRef, fd *ast.FuncDecl) {
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if x.Sel.Name == "NodeSelector" {
					reads = true
				}
			case *ast.CallExpr:
				if p := unroutedAction(x); p != "" {
					problems = append(problems, fmt.Sprintf("%s.%s builds a %s", ref.name, fd.Name.Name, p))
				}
			}
			return true
		})
	})
	switch {
	case err != nil:
		return err
	case !found:
		return fmt.Errorf("no method of %s.%s found", opts.pkg, opts.name)
	case len(problems) > 0:
		return fmt.Errorf("%s", strings.Join(problems, ", "))
	case !reads:
		return fmt.Errorf("no method of %s reads the node selector", opts.name)
	}
	return nil
}

// walkMethods calls fn with each method of the type, and of the types it
// embeds, and says whether there was one.
func (s *sourceTree) walkMethods(opts typeRef, fn func(typeRef, *ast.FuncDecl)) (bool, error) {
	var found bool
	todo := []typeRef{opts}
	seen := make(map[typeRef]bool)
	for len(todo) > 0 {
		ref := todo[0]
		todo = todo[1:]
		if seen[ref] {
			continue
		}
		seen[ref] = true
		files, err := s.files(ref.pkg)
		if err != nil {
			return found, err
		}
		for _, f := range files {
			for _, d := range f.Decls {
				switch v := d.(type) {
				case *ast.GenDecl:
					todo = append(todo, embeddedTypes(ref, f, v)...)
				case *ast.FuncDecl:
					if v.Body == nil || !isMethodOf(v, ref.name) {
						continue
					}
					found = true
					fn(ref, v)
				}
			}
		}
	}
	return found, nil
}

// embeddedTypes returns the types embedded in the struct of ref, when the
// declaration is the one of ref.
func embeddedTypes(ref typeRef, file *ast.File, decl *ast.GenDecl) []typeRef {
	var l []typeRef
	for _, spec := range decl.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || ts.Name.Name != ref.name {
			continue
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			continue
		}
		for _, field := range st.Fields.List {
			if len(field.Names) > 0 {
				continue
			}
			if r, _, err := resolveType(ref.pkg, file, field.Type); err == nil {
				l = append(l, r)
			}
		}
	}
	return l
}

func isMethodOf(fd *ast.FuncDecl, name string) bool {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return false
	}
	typ := fd.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	id, ok := typ.(*ast.Ident)
	return ok && id.Name == name
}

// unroutedAction describes the call when it builds a node or object action
// routed to the selected nodes with no function to run there, and returns
// empty otherwise.
func unroutedAction(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "New" {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || (pkg.Name != "nodeaction" && pkg.Name != "objectaction") {
		return ""
	}
	var routed, remote bool
	for _, arg := range call.Args {
		c, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		switch callName(c) {
		case "WithRemoteNodes":
			routed = true
		case "WithRemoteFunc":
			remote = true
		}
	}
	if routed && !remote {
		return pkg.Name + " routed to the nodes with no remote function"
	}
	return ""
}

// countIdent counts the identifiers of the name in the node.
func countIdent(node ast.Node, name string) int {
	n := 0
	ast.Inspect(node, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && id.Name == name {
			n++
		}
		return true
	})
	return n
}

// ActionsRunnable checks that no command builds a node or object action
// with no function to run it: no local function, no remote one, and no
// orchestration to ask for. Such an action does nothing whatever its options,
// as the ox node compliance commands did, which answered rc 0 having done
// nothing, and then an error saying the command has no remote form.
//
// The options of a command are the variable its run function calls the Run
// method of, in the function building the command. A command built another
// way is not checked.
func ActionsRunnable(t *testing.T, root *cobra.Command) {
	t.Helper()
	src, err := newSourceTree()
	if err != nil {
		t.Fatalf("read the sources of the module: %s", err)
	}
	checked := make(map[typeRef]error)
	commoncmd.WalkCommands(root, func(cmd *cobra.Command) {
		opts, ok := src.runOptionsType(cmd)
		if !ok {
			return
		}
		err, done := checked[opts]
		if !done {
			err = src.checkActionsRunnable(opts)
			checked[opts] = err
		}
		if err != nil {
			t.Errorf("%q: %s", cmd.CommandPath(), err)
		}
	})
}

// runOptionsType returns the type of the options the run function of the
// command calls the Run method of.
func (s *sourceTree) runOptionsType(cmd *cobra.Command) (typeRef, bool) {
	if cmd.RunE == nil {
		return typeRef{}, false
	}
	pkg, factory, err := funcOrigin(cmd.RunE)
	if err != nil {
		return typeRef{}, false
	}
	decl, file, err := s.funcDecl(pkg, factory)
	if err != nil {
		return typeRef{}, false
	}
	var varName string
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || varName != "" {
			return varName == ""
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			varName = id.Name
		}
		return true
	})
	if varName == "" {
		return typeRef{}, false
	}
	typ := localVarType(decl, varName)
	if typ == nil {
		return typeRef{}, false
	}
	opts, _, err := resolveType(pkg, file, typ)
	if err != nil {
		return typeRef{}, false
	}
	return opts, true
}

func (s *sourceTree) checkActionsRunnable(opts typeRef) error {
	var problems []string
	if _, err := s.walkMethods(opts, func(ref typeRef, fd *ast.FuncDecl) {
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if p := unrunnableAction(call); p != "" {
					problems = append(problems, fmt.Sprintf("%s.%s builds a %s", ref.name, fd.Name.Name, p))
				}
			}
			return true
		})
	}); err != nil {
		return err
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, ", "))
	}
	return nil
}

// unrunnableAction describes the call when it builds a node or object action
// with no function to run it, and returns empty otherwise.
func unrunnableAction(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "New" {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || (pkg.Name != "nodeaction" && pkg.Name != "objectaction") {
		return ""
	}
	for _, arg := range call.Args {
		c, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		switch callName(c) {
		case "WithLocalFunc", "WithRemoteFunc", "WithAsyncTarget", "WithAsyncFunc":
			return ""
		}
	}
	return pkg.Name + " with no function to run it"
}
