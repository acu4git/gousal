package trace

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	xtrace "golang.org/x/exp/trace"
)

const (
	MODE_FUNC_ENTER = "func-enter"
	MODE_FUNC_EXIT  = "func-exit"
)

type StepInfo struct {
	GID  int64
	Mode string
	Func string
	File string
}

type StepHistory []StepInfo

func (sh StepHistory) hasFuncWithPrefix(args ...string) bool {
	for _, v := range sh {
		for _, arg := range args {
			if ok := strings.HasPrefix(v.Func, arg); ok {
				return true
			}
		}
	}
	return false
}

func Parse(traceFile string) ([]StepInfo, error) {
	dir := filepath.Dir(traceFile)
	logFile := filepath.Join(dir, "log.txt")
	logf, err := os.Create(logFile)
	if err != nil {
		return nil, err
	}
	defer logf.Close()

	r, cancel, err := traceReader(traceFile)
	if err != nil {
		return nil, err
	}
	defer cancel()

	stepHistory := make(StepHistory, 0)

	// re := regexp.MustCompile(`^(runtime|os)[./]`)
	for {
		ev, err := r.ReadEvent()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		gid := ev.Goroutine()
		// goroutine文脈を持たないイベントは無視
		if gid == -1 {
			continue
		}
		// stk := ev.Stack()
		// // スタックが存在しない場合もあるのでチェック"
		// if stk == xtrace.NoStack {
		// 	continue
		// }

		switch ev.Kind() {
		case xtrace.EventLog:
			log := ev.Log()
			step, err := funcDefIdToStepInfo(gid, log.Category, log.Message)
			if err != nil {
				return nil, err
			}
			stepHistory = append(stepHistory, step)
			fmt.Fprintf(logf, "Goroutine %d\n\tmode: %s\n\tfuncDefID: %s\n", gid, log.Category, log.Message)
		}
	}

	return stepHistory, nil
}

type cancelFunc func()

func traceReader(file string) (*xtrace.Reader, cancelFunc, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, nil, err
	}

	r, err := xtrace.NewReader(f)
	if err != nil {
		return nil, nil, err
	}

	return r, func() { f.Close() }, nil
}

// funcDefIDは，"<project_root>/<rel>/<go_file>:<line>:<col>#<module>/<package>.<func>"のようなファイル情報と関数情報を'#'で区切る形式のID．
//
// 例外的に，mainパッケージのmain関数は"<project_root>/<rel>/<go_file>:<line>:<col>#main.main"と表現される．
func funcDefIdToStepInfo(gid xtrace.GoID, mode, funcDefID string) (StepInfo, error) {
	fileInfo, funcInfo, ok := strings.Cut(funcDefID, "#")
	if !ok {
		return StepInfo{}, errors.New("failed to convert funcDefID to StepInfo: token '#' not found.")
	}
	return StepInfo{
		GID:  int64(gid),
		Mode: mode,
		Func: funcInfo,
		File: fileInfo,
	}, nil
}
