package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"wails-test/internal"
	"wails-test/internal/graph"
	"wails-test/internal/trace"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx         context.Context
	gs          *graph.GraphState
	cancel      graph.CleanUpFunc
	projectRoot string
	mainFile    string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) SelectGoProject() (string, error) {
	mainFiles := make([]string, 0)
	for {
		dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{})
		if err != nil {
			return "", err
		}

		isRoot, err := internal.IsGoProject(dir)
		if err != nil {
			return "", err
		}
		if isRoot {
			mainFiles, err = internal.SearchMainAll(dir)
			if err != nil {
				return "", err
			}
			if len(mainFiles) == 0 {
				_, err = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
					Type:    runtime.WarningDialog,
					Title:   "main File Not Found",
					Message: "This project cannot be traced because of not containing any main files",
				})
				if err != nil {
					return "", err
				}
				continue
			}
			a.projectRoot = dir
			break
		}

		_, err = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.WarningDialog,
			Title:   "Go Project Not Found",
			Message: fmt.Sprintf("%s does not appear to be a Go project. Please select a directory that contains a go.mod file.", dir),
		})
		if err != nil {
			return "", err
		}
	}

	if len(mainFiles) == 1 {
		a.mainFile = mainFiles[0]
		return mainFiles[0], nil
	}

	res, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Title:   "Found multiple main file",
		Message: "Select the main file to trace",
		Buttons: mainFiles,
	})
	if err != nil {
		return "", err
	}
	a.mainFile = res

	return res, nil
}

func (a *App) Trace() error {
	// プロジェクトのクローン
	tmpRoot, err := internal.MakeTmpProjectRoot(a.projectRoot)
	if err != nil {
		return err
	}
	ignoredNames := map[string]bool{
		".git":                 true, // Git履歴
		internal.TEMP_DIR_NAME: true, // コピー先自身
		".idea":                true, // JetBrains設定
		".vscode":              true, // VSCode設定
		".DS_STORE":            true, // for Mac
		"build":                true, // 実行バイナリなど
	}
	if err := internal.CopyProject(a.projectRoot, tmpRoot, ignoredNames); err != nil {
		return err
	}
	runtime.LogInfo(a.ctx, "project clone succeeded")

	// runtime/traceの挿入
	goFiles, err := internal.ListGoFiles(a.projectRoot)
	if err != nil {
		return err
	}
	for _, goFile := range goFiles {
		rel, err := filepath.Rel(a.projectRoot, goFile)
		if err != nil {
			return err
		}
		dstFile := filepath.Join(tmpRoot, rel)
		if err := os.MkdirAll(filepath.Dir(dstFile), 0755); err != nil {
			return err
		}
		if err := trace.StaticInsertTrace(a.ctx, tmpRoot, goFile, dstFile); err != nil {
			return err
		}
		runtime.LogInfof(a.ctx, "inserted trace code successfully: %s", goFile)
	}
	runtime.LogInfo(a.ctx, "inserted runtime/trace successfully")

	// トレース実行
	mainRel, err := filepath.Rel(a.projectRoot, a.mainFile)
	if err != nil {
		return err
	}
	tmpMain := filepath.Join(tmpRoot, mainRel)
	if err := trace.RunWithTrace(a.ctx, tmpRoot, tmpMain); err != nil {
		return err
	}
	runtime.LogInfof(a.ctx, "trace tmpMain: %s", tmpMain)

	// トレースデータの解析
	traceFile := filepath.Join(tmpRoot, "trace.out")
	steps, err := trace.Parse(traceFile)
	if err != nil {
		return err
	}
	runtime.LogInfo(a.ctx, "parse succeeded")

	// GraphState初期化
	gs, cancel, err := graph.NewGraphState(a.ctx, steps)
	if err != nil {
		return err
	}
	if a.cancel != nil {
		a.cancel()
	}
	a.gs = gs
	a.cancel = cancel
	runtime.LogInfo(a.ctx, "init GraphState")

	if err := a.gs.Load(); err != nil {
		return err
	}
	runtime.LogInfo(a.ctx, "load GraphState")

	return err
}

type StepResult struct {
	SVG        string `json:"svg"`
	Affordable bool   `json:"affordable"`
}

func (a *App) Step() (StepResult, error) {
	runtime.LogInfo(a.ctx, "Step")
	svg, ok, err := a.gs.Step()
	return StepResult{SVG: svg, Affordable: ok}, err
}
