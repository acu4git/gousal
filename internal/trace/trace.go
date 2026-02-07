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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	etemp "wails-test/assets/template"
	"wails-test/internal/util"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/ast/astutil"
)

const (
	KEY_PKG_RUNTIME_TRACE = "runtime/trace"
	KEY_PKG_CONTEXT       = "context"
	KEY_PKG_OS            = "os"
	KEY_PKG_PATH_FILEPATH = "path/filepath"

	ALIAS_RUNTIME_TRACE = "__rtrace__"
	ALIAS_CONTEXT       = "__context__"
	ALIAS_OS            = "__os__"
	ALIAS_PATH_FILEPATH = "__pfilepath__"
)

// 無名関数のカウンターを管理する構造体
type anonFuncCounter struct {
	counters map[ast.Node]int
}

func newAnonFuncCounter() *anonFuncCounter {
	return &anonFuncCounter{
		counters: make(map[ast.Node]int),
	}
}

func (c *anonFuncCounter) next(parent ast.Node) int {
	c.counters[parent]++
	return c.counters[parent]
}

func StaticInsertTrace(ctx context.Context, tmpRoot, src, dest string) error {
	suffix := util.HexSuffix()

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

	counter := newAnonFuncCounter()
	for _, fn := range funcs {
		if err := insertTrace(ctx, tmpRoot, suffix, fset, file, fn, counter); err != nil {
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

func insertTrace(_ context.Context, tmpRoot, suffix string, fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, counter *anonFuncCounter) error {
	funcDefID := funcDefID(fset, file, fn, "")

	// context.Background()
	ctxExpr := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(ALIAS_CONTEXT + suffix),
			Sel: ast.NewIdent("Background"),
		},
	}

	// trace.Log(context.Background(), "func-enter", funcDefID)
	enterCall := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(ALIAS_RUNTIME_TRACE + suffix),
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
				X:   ast.NewIdent(ALIAS_RUNTIME_TRACE + suffix),
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

	// 関数本体内の無名関数を処理
	if err := processAnonFuncs(fset, file, fn, fn.Body, suffix, counter, ""); err != nil {
		return err
	}

	// 必須パッケージ(key: pkgName, value: alias)
	PkgAliases := map[string]string{
		KEY_PKG_CONTEXT:       ALIAS_CONTEXT + suffix,
		KEY_PKG_RUNTIME_TRACE: ALIAS_RUNTIME_TRACE + suffix,
	}

	if fn.Name.Name == "main" && file.Name.Name == "main" {
		// runtime/trace.Start/Stopを行うために追加する処理
		PkgAliases[KEY_PKG_OS] = ALIAS_OS + suffix
		PkgAliases[KEY_PKG_PATH_FILEPATH] = ALIAS_PATH_FILEPATH + suffix

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
			"FileVar":          fileVar,
			"InitFunc":         funcNameInit,
			"StopFunc":         funcNameStop,
			"ProjectRoot":      strconv.Quote(tmpRoot),
			"PkgAliasFILEPATH": PkgAliases[KEY_PKG_PATH_FILEPATH],
			"PkgAliasOS":       PkgAliases[KEY_PKG_OS],
			"PkgAliasTRACE":    PkgAliases[KEY_PKG_RUNTIME_TRACE],
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
	}

	// 追加
	for pkg, alias := range PkgAliases {
		astutil.AddNamedImport(fset, file, alias, pkg)
	}

	return nil
}

// 無名関数を再帰的に処理する
func processAnonFuncs(fset *token.FileSet, file *ast.File, parentFunc *ast.FuncDecl, node ast.Node, suffix string, counter *anonFuncCounter, parentAnonPath string) error {
	var err error
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		// 無名関数リテラルを検出
		if funcLit, ok := n.(*ast.FuncLit); ok {
			// この無名関数のカウントを取得
			count := counter.next(node)

			// 無名関数のパスを構築
			var anonPath string
			if parentAnonPath == "" {
				anonPath = fmt.Sprintf("func%d", count)
			} else {
				anonPath = fmt.Sprintf("%s.%d", parentAnonPath, count)
			}

			// 無名関数用のfuncDefIDを生成
			anonFuncDefID := funcDefID(fset, file, parentFunc, anonPath)

			// context.Background()
			ctxExpr := &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent(ALIAS_CONTEXT + suffix),
					Sel: ast.NewIdent("Background"),
				},
			}

			// trace.Log(context.Background(), "func-enter", anonFuncDefID)
			enterCall := &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   ast.NewIdent(ALIAS_RUNTIME_TRACE + suffix),
						Sel: ast.NewIdent("Log"),
					},
					Args: []ast.Expr{
						ctxExpr,
						&ast.BasicLit{Kind: token.STRING, Value: `"func-enter"`},
						&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", anonFuncDefID)},
					},
				},
			}

			// defer trace.Log(context.Background(), "func-exit", anonFuncDefID)
			exitDefer := &ast.DeferStmt{
				Call: &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   ast.NewIdent(ALIAS_RUNTIME_TRACE + suffix),
						Sel: ast.NewIdent("Log"),
					},
					Args: []ast.Expr{
						ctxExpr,
						&ast.BasicLit{Kind: token.STRING, Value: `"func-exit"`},
						&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", anonFuncDefID)},
					},
				},
			}

			// 無名関数の本体にトレースコードを挿入
			funcLit.Body.List = append(
				[]ast.Stmt{enterCall, exitDefer},
				funcLit.Body.List...,
			)

			// この無名関数内のさらにネストした無名関数を処理
			if innerErr := processAnonFuncs(fset, file, parentFunc, funcLit.Body, suffix, counter, anonPath); innerErr != nil {
				err = innerErr
				return false
			}

			// この無名関数リテラル自体の子ノードは処理済みなのでfalseを返す
			return false
		}

		return true
	})

	return err
}

func funcDefID(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, anonPath string) string {
	pos := fset.Position(fn.Pos())
	funcPosInfo := fmt.Sprintf("%s:%d:%d", pos.Filename, pos.Line, pos.Column)
	funcName := packageID(fset, file)
	if file.Name.Name == "main" {
		funcName = "main"
	}
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv := receiverTypeString(fn.Recv.List[0].Type)
		funcName += fmt.Sprintf(".(%s)", recv)
	}
	funcName += fmt.Sprintf(".%s", fn.Name.Name)
	if params := typeParamNames(fn); len(params) > 0 {
		funcName += fmt.Sprintf("[%s]", strings.Join(params, ","))
	}
	if fn.Name.Name == "main" && file.Name.Name == "main" {
		funcName = "main.main"
	}

	// 無名関数の場合はパスを追加
	if anonPath != "" {
		funcName += fmt.Sprintf(".%s", anonPath)
	}

	return fmt.Sprintf("%s#%s", funcPosInfo, funcName)
}

func packageID(fset *token.FileSet, file *ast.File) string {
	projectRoot, _ := os.Getwd()
	modBytes, _ := os.ReadFile(filepath.Join(projectRoot, "go.mod"))
	modFile, _ := modfile.Parse("go.mod", modBytes, nil)
	pos := fset.Position(file.Pos())
	rel, _ := filepath.Rel(projectRoot, filepath.Dir(pos.Filename))
	return path.Join(modFile.Module.Mod.Path, rel)
}

func modulePath() string {
	projectRoot, _ := os.Getwd()
	modBytes, _ := os.ReadFile(filepath.Join(projectRoot, "go.mod"))
	modFile, _ := modfile.Parse("go.mod", modBytes, nil)
	return modFile.Module.Mod.Path
}

func receiverTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + receiverTypeString(t.X)
	case *ast.SelectorExpr:
		return receiverTypeString(t.X) + "." + t.Sel.Name
	case *ast.IndexExpr:
		return receiverTypeString(t.X) + "[T]"
	default:
		return fmt.Sprintf("<%T>", expr)
	}
}

func typeParamNames(f *ast.FuncDecl) []string {
	if f.Type.TypeParams == nil {
		return nil
	}

	var params []string
	for _, field := range f.Type.TypeParams.List {
		for _, name := range field.Names {
			params = append(params, name.Name)
		}
	}
	return params
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

func RunWithTrace(ctx context.Context, tmpRoot string, files ...string) error {
	args := []string{"run"}
	args = append(args, files...)
	cmd := exec.Command("go", args...)
	cmd.Dir = tmpRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runtime.LogInfo(ctx, cmd.String())
	return cmd.Run()
}
