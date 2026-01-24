package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"golang.org/x/exp/trace"
)

type StackFrameInfo struct {
	goID     trace.GoID
	fileName string
	funcName string
	line     uint64
}

type StackFrameInfoSlice []StackFrameInfo

func (ss StackFrameInfoSlice) hasFuncWithPrefix(args ...string) bool {
	for _, v := range ss {
		for _, arg := range args {
			if ok := strings.HasPrefix(v.funcName, arg); ok {
				return true
			}
		}
	}
	return false
}

func outStartLine() {
	fmt.Printf(`
		=========================================================================
		    parse start [%v]
		=========================================================================
`, time.Now())
}

func main() {
	outStartLine()
	history := make([]StackFrameInfoSlice, 0)

	// 1. trace.out ファイルを開く
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	traceFile := filepath.Join(cwd, "trace_data/trace.out")
	f, err := os.Open(traceFile)
	if err != nil {
		log.Fatalf("failed to open trace file: %v", err)
	}
	defer f.Close()

	// 2. トレースリーダーを作成
	r, err := trace.NewReader(f)
	if err != nil {
		log.Fatalf("failed to create trace reader: %v", err)
	}

	// 3. イベントを順番に読み込む
	re := regexp.MustCompile(`^(runtime|os)[./]`)
	for {
		ev, err := r.ReadEvent()
		if err == io.EOF {
			break // 読み込み終了
		}
		if err != nil {
			log.Fatalf("failed to read event: %v", err)
		}

		// 4. 必要なイベントだけをフィルタリングして表示
		// 例: Goroutineの状態遷移（作成、実行開始、ブロックなど）を表示
		gid := ev.Goroutine()
		if gid == -1 {
			// goroutine文脈を持たないイベントは無視
			continue
		}
		evTime := ev.Time()

		stk := ev.Stack()
		// スタックが存在しない場合もあるのでチェック"
		if stk == trace.NoStack {
			continue
		}

		switch ev.Kind() {
		case trace.EventLog:
			log := ev.Log()
			fmt.Printf("Event[ts%v]: Goroutine %d(%v)\n", evTime, gid, ev.Kind())
			fmt.Printf("func log: (%s, %s)\n", log.Category, log.Message)
			fmt.Println("------------------------------------------------")
			stk := StackFrameInfoSlice{StackFrameInfo{
				goID:     gid,
				funcName: log.Message,
			}}
			history = append(history, stk)
		case trace.EventStateTransition:
			st := ev.StateTransition()
			// 対象がGoroutineである場合のみ処理
			if st.Resource.Kind == trace.ResourceGoroutine {
				historyStk := make(StackFrameInfoSlice, 0)

				frames := stk.Frames()
				for v := range frames {
					info := StackFrameInfo{
						goID:     gid,
						funcName: v.Func,
						fileName: v.File,
						line:     v.Line,
					}
					historyStk = append(historyStk, info)
				}

				from, to := st.Goroutine()
				if to == trace.GoSyscall ||
					historyStk.hasFuncWithPrefix("runtime/trace", "fmt.", "sync.(*WaitGroup).Done") ||
					re.MatchString(historyStk[len(historyStk)-1].funcName) {
					break
				}
				fmt.Printf("Event[ts%v]: Goroutine %d(%v)\n", evTime, gid, ev.Kind())
				fmt.Printf("state kind: %v(%v -> %v)\n", st.Resource.Kind, from, to)
				fmt.Printf("reason: %s\n", st.Reason)

				reverse := make([]StackFrameInfo, 0, len(historyStk))
				for _, v := range slices.Backward(historyStk) {
					fmt.Printf("\tat %s (%s:%d)\n", v.funcName, v.fileName, v.line)
					reverse = append(reverse, v)
				}
				history = append(history, reverse)
				fmt.Println("------------------------------------------------")
			}
		default:
		}
	}

	// graphvizのセットアップ
	ctx := context.Background()
	g, err := graphviz.New(ctx)
	if err != nil {
		panic(err)
	}

	graph, err := g.Graph()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := graph.Close(); err != nil {
			panic(err)
		}
		g.Close()
	}()

	goroutineMap := make(map[trace.GoID]*cgraph.Graph)
	nodeMap := make(map[trace.GoID]map[string]*cgraph.Node)
	edgeMap := make(map[trace.GoID]map[string]*cgraph.Edge)

	// re := regexp.MustCompile(`hogehoge`)
	for _, stk := range history {
		var prev *StackFrameInfo
		for _, v := range stk {
			if re.MatchString(v.funcName) {
				break
			}
			gg, ok := goroutineMap[v.goID]
			if !ok {
				name := fmt.Sprintf("cluster_goroutine_%d", v.goID)
				gg, err = graph.CreateSubGraphByName(name)
				if err != nil {
					panic(err)
				}
				gg.SetLabel(name)
				gg.SetStyle("filled")
				goroutineMap[v.goID] = gg
			}

			// ノード生成条件を「funcName 単位」にする
			if _, ok := nodeMap[v.goID]; !ok {
				nodeMap[v.goID] = make(map[string]*cgraph.Node)
			}
			node, ok := nodeMap[v.goID][v.funcName]
			if !ok {
				// name := fmt.Sprintf("%s\n(%s:%d)", v.funcName, v.fileName, v.line)
				name := fmt.Sprintf("%s", v.funcName)
				node, err = gg.CreateNodeByName(fmt.Sprintf("%s-gid%d", name, v.goID))
				if err != nil {
					panic(err)
				}
				node.SetLabel(name)
				node.SetStyle("filled")
				node.SetFillColor("white")
				nodeMap[v.goID][v.funcName] = node
			}

			if prev != nil {
				// edgeMap も goroutine 単位にする
				if _, ok := edgeMap[v.goID]; !ok {
					edgeMap[v.goID] = make(map[string]*cgraph.Edge)
				}
				edgeLabel := fmt.Sprintf("%s:%d -> %s:%d", prev.funcName, prev.goID, v.funcName, v.goID)
				edge, ok := edgeMap[v.goID][edgeLabel]
				if !ok {
					edge, err = gg.CreateEdgeByName(edgeLabel, nodeMap[prev.goID][prev.funcName], node)
					if err != nil {
						panic(err)
					}
					edgeMap[v.goID][edgeLabel] = edge
				}
			}
			vCopy := v
			prev = &vCopy
		}
	}

	renderDir := filepath.Join(cwd, "graph")
	if err := os.MkdirAll(renderDir, 0755); err != nil {
		panic(err)
	}
	renderFile := filepath.Join(renderDir, fmt.Sprintf("out_%d.%s", time.Now().UnixNano(), graphviz.SVG))

	var buf bytes.Buffer
	if err := g.Render(ctx, graph, graphviz.SVG, &buf); err != nil {
		panic(err)
	}
	// fmt.Println(buf.String())
	if err := os.WriteFile(renderFile, buf.Bytes(), 0755); err != nil {
		panic(err)
	}
}
