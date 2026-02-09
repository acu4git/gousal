package main

import (
	"context"
	"errors"
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
	mainFiles   []string
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

func (a *App) SelectGoProject() (map[string][]string, error) {
	mainDirs := make(map[string][]string, 0)
	for {
		dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{})
		if err != nil {
			return nil, err
		}

		isRoot, err := internal.IsGoProject(dir)
		if err != nil {
			return nil, err
		}
		if isRoot {
			mainDirs, err = internal.SearchMainAll(dir)
			if err != nil {
				return nil, err
			}
			if len(mainDirs) == 0 {
				_, err = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
					Type:    runtime.WarningDialog,
					Title:   "main File Not Found",
					Message: "This project cannot be traced because of not containing any main files",
				})
				if err != nil {
					return nil, err
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
			return nil, err
		}
	}

	// runtime.LogInfof(a.ctx, "num of mainDirs: %d", len(mainDirs))
	runtime.LogInfo(a.ctx, "num of main files:")
	for dir, files := range mainDirs {
		runtime.LogInfof(a.ctx, "\t%s: %d", dir, len(files))
	}

	return mainDirs, nil
}

func (a *App) Trace(files []string) (string, error) {
	a.mainFiles = files

	// プロジェクトのクローン
	tmpRoot, err := internal.MakeTmpProjectRoot(a.projectRoot)
	if err != nil {
		return "", err
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
		return "", err
	}
	runtime.LogInfo(a.ctx, "project clone succeeded")

	// runtime/traceの挿入
	goFiles, err := internal.ListGoFiles(a.projectRoot)
	if err != nil {
		return "", err
	}
	for _, goFile := range goFiles {
		rel, err := filepath.Rel(a.projectRoot, goFile)
		if err != nil {
			return "", err
		}
		dstFile := filepath.Join(tmpRoot, rel)
		if err := os.MkdirAll(filepath.Dir(dstFile), 0755); err != nil {
			return "", err
		}
		if err := trace.StaticInsertTrace(a.ctx, a.projectRoot, tmpRoot, goFile, dstFile); err != nil {
			return "", err
		}
		runtime.LogInfof(a.ctx, "inserted trace code successfully: %s", goFile)
	}
	runtime.LogInfo(a.ctx, "inserted runtime/trace successfully")

	// トレース実行
	tmpMains := make([]string, 0)
	for _, m := range a.mainFiles {
		mainRel, err := filepath.Rel(a.projectRoot, m)
		if err != nil {
			return "", err
		}
		tmpMain := filepath.Join(tmpRoot, mainRel)
		tmpMains = append(tmpMains, tmpMain)
	}
	// mainRel, err := filepath.Rel(a.projectRoot, a.mainFile)
	// if err != nil {
	// 	return "", err
	// }
	// tmpMain := filepath.Join(tmpRoot, mainRel)
	if err := trace.RunWithTrace(a.ctx, tmpRoot, tmpMains...); err != nil {
		return "", err
	}
	runtime.LogInfof(a.ctx, "trace tmpMain: %v", tmpMains)

	// トレースデータの解析
	traceFile := filepath.Join(tmpRoot, "trace.out")
	steps, err := trace.Parse(traceFile)
	if err != nil {
		return "", err
	}
	runtime.LogInfo(a.ctx, "parse succeeded")

	// GraphState初期化
	gs, cancel, err := graph.NewGraphState(a.ctx, steps)
	if err != nil {
		return "", err
	}
	if a.cancel != nil {
		a.cancel()
	}
	a.gs = gs
	a.cancel = cancel
	runtime.LogInfo(a.ctx, "init GraphState")

	svg, err := a.gs.Load()
	if err != nil {
		return "", err
	}
	runtime.LogInfo(a.ctx, "load GraphState")

	runtime.EventsEmit(a.ctx, "clearLogs", nil)

	return svg, nil
}

type StepResult struct {
	SVG     string `json:"svg"`
	CanStep bool   `json:"canStep"`
}

func (a *App) Step() (StepResult, error) {
	runtime.LogInfo(a.ctx, "Step")
	if a.gs == nil {
		return StepResult{}, errors.New("(*App).gs is nil")
	}
	svg, ok, err := a.gs.Step()
	return StepResult{SVG: svg, CanStep: ok}, err
}

func (a *App) Rel(basepath string, targpath string) (string, error) {
	return filepath.Rel(basepath, targpath)
}
