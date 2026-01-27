package main

import (
	"context"
	"fmt"
	"wails-test/internal"
	"wails-test/internal/graph"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx         context.Context
	gs          graph.GraphState
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

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
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
