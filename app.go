package main

import (
	"context"
	"fmt"
	"wails-test/internal"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
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

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) SelectGoProject() ([]string, error) {
	mainFiles := make([]string, 0)
	for {
		dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
			Title: "Goプロジェクトを選択",
		})
		if err != nil {
			return nil, err
		}

		isRoot, err := internal.IsGoProject(dir)
		if err != nil {
			return nil, err
		}
		if isRoot {
			mainFiles, err = internal.SearchMainAll(dir)
			if err != nil {
				return nil, err
			}
			break
		}

		m, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.WarningDialog,
			Title:   "Go Project Not Found",
			Message: fmt.Sprintf("%s does not appear to be a Go Project. Please select a directory that contains a go.mod file.", dir),
		})
		if err != nil {
			return nil, err
		}
		runtime.LogInfo(a.ctx, m)
	}
	return mainFiles, nil
}
