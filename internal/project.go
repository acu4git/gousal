package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	TEMP_DIR_NAME = ".tmp"
)

func MakeTmpProjectRoot(projectRoot string) (string, error) {
	// suffix := util.HexSuffix()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	tmpRoot := filepath.Join(projectRoot, TEMP_DIR_NAME, "trace-"+suffix)

	if err := os.MkdirAll(tmpRoot, 0755); err != nil {
		return "", err
	}
	return tmpRoot, nil
}

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

func SearchMainAll(root string) (map[string][]string, error) {
	dirs := make(map[string][]string)
	result := make(map[string][]string)

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

		dir := filepath.Dir(path)
		dirs[dir] = append(dirs[dir], path)

		return nil
	})
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	// mainDirs := make([]string, 0)

	for dir, files := range dirs {

		hasMainPkg := false
		hasMainFunc := false

		mainFiles := make([]string, 0)

		for _, file := range files {
			f, err := parser.ParseFile(fset, file, nil, 0)
			if err != nil {
				continue
			}

			if f.Name.Name != "main" {
				continue
			}

			hasMainPkg = true
			mainFiles = append(mainFiles, file)

			for _, decl := range f.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					if fn.Name.Name == "main" && fn.Recv == nil {
						hasMainFunc = true
					}
				}
			}
		}

		if hasMainPkg && hasMainFunc {
			result[dir] = mainFiles
		}
	}

	return result, err
}

// CopyProject: ソースから宛先へ再帰的にコピー (除外リスト対応)
func CopyProject(srcRoot, destRoot string, ignored map[string]bool) error {
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

type goListPkg struct {
	Dir     string
	GoFiles []string
}

func ListGoFiles(projectRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// ディレクトリや隠しファイル(.gitなど)はスキップ
		if d.IsDir() {
			// .tmp ディレクトリは一時的なプロジェクトのコピーなので除外
			if d.Name() == TEMP_DIR_NAME || (d.Name() != "." && strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		// .goファイル以外は無視
		if filepath.Ext(path) != ".go" {
			return nil
		}
		// テストファイル(_test.go)は除外する
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
