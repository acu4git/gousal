package internal

import (
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// copyProject: ソースから宛先へ再帰的にコピー (除外リスト対応)
func copyProject(srcRoot, destRoot string, ignored map[string]bool) error {
	return filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// ルートディレクトリ自身はスキップしないが処理もしない
		if path == srcRoot {
			return nil
		}

		// 除外リストのチェック
		if ignored[d.Name()] {
			if d.IsDir() {
				return filepath.SkipDir // ディレクトリなら中身もスキャンしない
			}
			return nil // ファイルなら単に無視
		}

		// コピー先のパスを計算
		// 例: /project/cmd/main.go -> cmd/main.go
		relPath, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(destRoot, relPath)

		// ディレクトリの場合: 作成
		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// ファイルの場合: コピー
		return copyFile(path, destPath)
	})
}

// copyFile: 単一ファイルのコピー
func copyFile(src, dest string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)

	// パーミッション情報をコピーしたい場合は以下を追加
	// info, _ := sourceFile.Stat()
	// os.Chmod(dest, info.Mode())

	return err
}
