package internal

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	TEMP_DIR_NAME = "tmp"
)

func IsGoProject(path string) (bool, error) {
	modPath := filepath.Join(path, "go.mod")
	info, err := os.Stat(modPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, nil
	}
	return true, nil
}

func SearchMainAll(root string) ([]string, error) {
	var mainFiles []string
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// 1. ディレクトリや隠しファイル(.gitなど)はスキップ
		if d.IsDir() {
			if d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		// 2. .goファイル以外は無視
		if filepath.Ext(path) != ".go" {
			return nil
		}
		// 3. テストファイル(_test.go)は除外する
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// 4. ファイルをパースしてパッケージ名を判定
		// parser.PackageClauseOnly を指定すると、冒頭の package 宣言だけ読んで止まるので高速
		f, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if err != nil {
			return nil
		}
		if f.Name.Name == "main" {
			mainFiles = append(mainFiles, path)
		}

		return nil
	})

	return mainFiles, err
}
