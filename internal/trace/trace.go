package trace

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strconv"
	"text/template"
	etemp "wails-test/assets/template"
	"wails-test/internal/util"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/tools/go/ast/astutil"
)

func StaticInsertTrace(ctx context.Context, tmpRoot, src, dest string) error {
	// Go file -> AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	funcs := make([]*ast.FuncDecl, 0)

	// ASTを走査
	ast.Inspect(file, func(n ast.Node) bool {
		// 通常関数
		if fn, ok := n.(*ast.FuncDecl); ok {
			funcs = append(funcs, fn)
		}
		return true
	})

	for _, fn := range funcs {
		if err := insertTrace(ctx, tmpRoot, fset, file, fn); err != nil {
			return err
		}
	}

	var buf bytes.Buffer
	format.Node(&buf, fset, file)

	if err := os.WriteFile(dest, buf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

func insertTrace(_ context.Context, tmpRoot string, fset *token.FileSet, file *ast.File, fn *ast.FuncDecl) error {
	pos := fset.Position(fn.Pos())
	funcDefID := fmt.Sprintf("%s:%d:%d#%s", pos.Filename, pos.Line, pos.Column, fn.Name.Name)

	// context.Background()
	ctxExpr := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("context"),
			Sel: ast.NewIdent("Background"),
		},
	}

	// trace.Log(context.Background(), "func-enter", funcDefID)
	enterCall := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("trace"),
				Sel: ast.NewIdent("Log"),
			},
			Args: []ast.Expr{
				ctxExpr,
				&ast.BasicLit{Kind: token.STRING, Value: `"func-enter"`},
				&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", funcDefID)},
			},
		},
	}

	// defer trace.Log(context.Background(), "func-exit", funcDefID)
	exitDefer := &ast.DeferStmt{
		Call: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("trace"),
				Sel: ast.NewIdent("Log"),
			},
			Args: []ast.Expr{
				ctxExpr,
				&ast.BasicLit{Kind: token.STRING, Value: `"func-exit"`},
				&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", funcDefID)},
			},
		},
	}

	fn.Body.List = append(
		[]ast.Stmt{enterCall, exitDefer},
		fn.Body.List...,
	)

	// 必須パッケージ
	pkgs := map[string]bool{
		"runtime/trace": false,
		"context":       false,
	}

	if fn.Name.Name == "main" && file.Name.Name == "main" {
		// runtime/trace.Start/Stopを行うために追加する処理
		suffix := util.HexSuffix()
		prefix := "__trace_" + suffix
		funcNameInit := prefix + "_init"
		funcNameStop := prefix + "_stop"
		fileVar := prefix + "_file"

		initCall := &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: ast.NewIdent(funcNameInit),
			},
		}
		stopDefer := &ast.DeferStmt{
			Call: &ast.CallExpr{
				Fun: ast.NewIdent(funcNameStop),
			},
		}
		fn.Body.List = append(
			[]ast.Stmt{initCall, stopDefer},
			fn.Body.List...,
		)

		data := map[string]string{
			"FileVar":     fileVar,
			"InitFunc":    funcNameInit,
			"StopFunc":    funcNameStop,
			"ProjectRoot": strconv.Quote(tmpRoot),
		}
		__src, err := renderTemplate("func.tmpl", data)
		if err != nil {
			return err
		}
		src := fmt.Sprintf("package %s\n\n%s", file.Name.Name, __src)
		helperFile, err := parser.ParseFile(fset, "", src, 0)
		if err != nil {
			return err
		}
		file.Decls = append(file.Decls, helperFile.Decls...)

		pkgs["os"] = false
		pkgs["path/filepath"] = false
		pkgs["log"] = false
	}

	// 検出
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return err
		}
		pkgs[path] = true
	}
	// runtime.LogInfo(ctx, "pkg detection done")

	// 追加
	for pkg, ok := range pkgs {
		if !ok {
			astutil.AddImport(fset, file, pkg)
		}
	}
	// runtime.LogInfo(ctx, "AddImport done")

	return nil
}

func renderTemplate(pattern string, data any) (string, error) {
	t, err := template.ParseFS(etemp.FS, pattern)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err // data が足りない等
	}

	return buf.String(), nil
}

func RunWithTrace(ctx context.Context, tmpRoot, mainFile string) error {
	cmd := exec.Command("go", "run", mainFile)
	cmd.Dir = tmpRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runtime.LogInfo(ctx, cmd.String())
	return cmd.Run()
}
