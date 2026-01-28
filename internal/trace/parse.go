package trace

import (
	"errors"
	"io"
	"os"
	"strings"

	xtrace "golang.org/x/exp/trace"
)

type StepInfo struct {
	GID  int64
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

func ParseTrace(traceFile string) ([]StepInfo, error) {
	r, err := traceReader(traceFile)
	if err != nil {
		return nil, err
	}

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
		stk := ev.Stack()
		// スタックが存在しない場合もあるのでチェック"
		if stk == xtrace.NoStack {
			continue
		}

		switch ev.Kind() {
		case xtrace.EventLog:
			funcDefID := ev.Log().Message
			step, err := idToStepInfo(gid, funcDefID)
			if err != nil {
				return nil, err
			}
			stepHistory = append(stepHistory, step)
		}
	}

	return stepHistory, nil
}

func traceReader(file string) (*xtrace.Reader, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := xtrace.NewReader(f)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// funcDefIDは，"<project_root>/<rel>/<go_file>:<line>:<col>#<module>/<package>.<func>"のようなファイル情報と関数情報を'#'で区切る形式のID．
//
// 例外的に，mainパッケージのmain関数は"<project_root>/<rel>/<go_file>:<line>:<col>#main.main"と表現される．
func idToStepInfo(gid xtrace.GoID, funcDefID string) (StepInfo, error) {
	fileInfo, funcInfo, ok := strings.Cut(funcDefID, "#")
	if !ok {
		return StepInfo{}, errors.New("failed to convert funcDefID to StepInfo: token '#' not found.")
	}
	return StepInfo{
		GID:  int64(gid),
		Func: funcInfo,
		File: fileInfo,
	}, nil
}
