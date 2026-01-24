package internal

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"wails-test/internal/util"

	"golang.org/x/tools/go/ast/astutil"
)

func StaticInsertTrace(projectRoot string, filename string) error {
	// プロジェクトのコピー先の作成
	tmpRoot := filepath.Join(projectRoot, "tmp")
	os.MkdirAll(tmpRoot, 0755)
	r, err := util.RandStringBase62(8)
	traceDir := filepath.Join(tmpRoot, r)
	os.Mkdir(traceDir, 0755)

	// Go file -> AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
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
		insertTrace(fset, file, fn)
	}

	var buf bytes.Buffer
	format.Node(&buf, fset, file)

	if err := os.WriteFile(filename, buf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

func newTraceSuffix() string {
	b := make([]byte, 4) // 32bit
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func insertTrace(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl) {
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
		suffix := newTraceSuffix()
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
			"FileVar":  fileVar,
			"InitFunc": funcNameInit,
			"StopFunc": funcNameStop,
		}
		src := fmt.Sprintf("package %s\n\n%s", file.Name.Name, renderTemplate("./template/func.tmpl", data))
		helperFile, err := parser.ParseFile(fset, "", src, 0)
		if err != nil {
			panic(err)
		}
		file.Decls = append(file.Decls, helperFile.Decls...)

		pkgs["os"] = false
		pkgs["path/filepath"] = false
		pkgs["log"] = false
	}

	// 検出
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		pkgs[path] = true
	}
	// 追加
	for pkg, ok := range pkgs {
		if !ok {
			astutil.AddImport(fset, file, pkg)
		}
	}
}

func renderTemplate(filename string, data any) string {
	t, err := template.ParseFiles(filename)
	if err != nil {
		panic(err) // テンプレート自体が壊れている
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		panic(err) // data が足りない等
	}

	return buf.String()
}
