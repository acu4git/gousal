package trace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	xtrace "golang.org/x/exp/trace"
)

const (
	MODE_FUNC_ENTER = "func-enter"
	MODE_FUNC_EXIT  = "func-exit"
	MODE_GO_CREATE  = "go-create"
)

// イベントログがもつスタックトレースのフレーム情報
type callRecord struct {
	GID      xtrace.GoID
	ChildGID int64
	Func     string
	File     string
	Line     uint64
	PC       uint64
}

type callStack []callRecord

func (stk callStack) hasFuncWithPrefix(args ...string) bool {
	for _, v := range stk {
		for _, arg := range args {
			if ok := strings.HasPrefix(v.Func, arg); ok {
				return true
			}
		}
	}
	return false
}

// ステップ実行に必要な情報
type StepInfo struct {
	GID      int64  // parent goroutine id
	ChildGID int64  // child goroutine id(when go-create or go-)
	Mode     string // event mode
	Func     string // function name
	File     string // filepath where function is written
	Line     uint64 // line where function is located
}

type StepHistory []StepInfo

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

	re := regexp.MustCompile(`^(runtime|os)[./]`)
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
			log := ev.Log()
			step, err := funcDefIdToStepInfo(gid, log.Category, log.Message)
			if err != nil {
				return nil, err
			}
			stepHistory = append(stepHistory, step)
			fmt.Fprintf(logf, "Goroutine %d\n", gid)
			fmt.Fprintf(logf, "\tkind: %s\n", ev.Kind())
			fmt.Fprintf(logf, "\tmode: %s\n", log.Category)
			fmt.Fprintf(logf, "\tfuncDefID: %s\n", log.Message)
		case xtrace.EventStateTransition:
			st := ev.StateTransition()
			switch st.Resource.Kind {
			case xtrace.ResourceGoroutine:
				// コールスタックをスライスに整形
				callStk := make(callStack, 0)

				frames := stk.Frames()
				for v := range frames {
					info := callRecord{
						GID:  gid,
						Func: v.Func,
						File: v.File,
						Line: v.Line,
						PC:   v.PC,
					}
					callStk = append(callStk, info)
				}

				// 標準ライブラリ由来のGoroutineイベントは除外
				from, to := st.Goroutine()
				if to == xtrace.GoSyscall ||
					callStk.hasFuncWithPrefix("runtime/trace", "fmt.", "sync.(*WaitGroup).Done") ||
					re.MatchString(callStk[0].Func) {
					break
				}

				// EvGoCreate
				if from == xtrace.GoNotExist && to == xtrace.GoRunnable {
					// 非同期処理を呼び出した関数の特定
					var parentFunc, file string
					var line uint64
					for _, v := range callStk {
						if v.Func != "sync.(*WaitGroup).Go" {
							parentFunc = v.Func
							file = v.File
							line = v.Line
							break
						}
					}
					childGID := st.Resource.Goroutine()

					info := StepInfo{
						GID:      int64(gid),
						ChildGID: int64(childGID),
						Mode:     MODE_GO_CREATE,
						Func:     parentFunc,
						File:     file,
						Line:     line,
					}
					stepHistory = append(stepHistory, info)
				}

				fmt.Fprintf(logf, "Goroutine %d\n", gid)
				fmt.Fprintf(logf, "\tkind: %s\n", ev.Kind())
				fmt.Fprintf(logf, "\ttransistion: %s -> %s\n", from, to)
				fmt.Fprintf(logf, "\treason: %s\n", st.Reason)
				fmt.Fprintf(logf, "\tev.Goroutine(): %d\n", gid)
				fmt.Fprintf(logf, "\tst.Resource.Goroutine(): %d\n", st.Resource.Goroutine())
				fmt.Fprintln(logf, "\tstack trace:")
				for _, v := range callStk {
					fmt.Fprintf(logf, "\t\t(PC=%d) %s (%s:%d)\n", v.PC, v.Func, v.File, v.Line)
				}
			}
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

type funcInfo struct {
	filepath string
	line     uint64
	col      uint64
	fullname string
}

func idToFuncInfo(id string) (funcInfo, error) {
	l, r, ok := strings.Cut(id, "#")
	if !ok {
		return funcInfo{}, fmt.Errorf("failed to convert %s to funcInfo: token '#' not found.", id)
	}
	tok := strings.Split(l, ":")
	if len(tok) != 3 {
		return funcInfo{}, fmt.Errorf("failed to convert %s to funcInfo: token ':' appear twice.", id)
	}
	path := tok[0]
	line, err := strconv.ParseUint(tok[1], 10, 64)
	if err != nil {
		return funcInfo{}, fmt.Errorf("failed to convert %s to funcInfo: %w", id, err)
	}
	col, err := strconv.ParseUint(tok[2], 10, 64)
	if err != nil {
		return funcInfo{}, fmt.Errorf("failed to convert %s to funcInfo: %w", id, err)
	}
	return funcInfo{
		filepath: path,
		line:     line,
		col:      col,
		fullname: r,
	}, nil
}

// funcDefIDは，"<project_root>/<rel>/<go_file>:<line>:<col>#<module>/<package>.<func>"のようなファイル情報と関数情報を'#'で区切る形式のID．
//
// 例外的に，mainパッケージのmain関数は"<project_root>/<rel>/<go_file>:<line>:<col>#main.main"と表現される．
func funcDefIdToStepInfo(gid xtrace.GoID, mode, funcDefID string) (StepInfo, error) {
	fn, err := idToFuncInfo(funcDefID)
	if err != nil {
		return StepInfo{}, err
	}
	return StepInfo{
		GID:  int64(gid),
		Mode: mode,
		Func: fn.fullname,
		File: fn.filepath,
		Line: fn.line,
	}, nil
}
