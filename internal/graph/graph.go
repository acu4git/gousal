package graph

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"wails-test/internal/trace"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
)

const (
	STYLE_FILLED     = "filled"
	STYLE_INVIS      = "invis"
	STYLE_DASHED     = "dashed"
	COLOR_BLACK      = "black"
	COLOR_WHITE      = "white"
	COLOR_GRAY       = "gray"
	COLOR_LIGHTGREEN = "lightgreen"
	COLOR_RED        = "red"
	SHAPE_POINT      = "point"
)

type GraphState struct {
	ctx                 context.Context
	gviz                *graphviz.Graphviz
	g                   *graphviz.Graph
	steps               trace.StepHistory
	next                int
	goroutineMap        map[int64]*cgraph.Graph
	funcNodeMap         map[int64]map[string]*cgraph.Node
	callEdgeMap         map[int64]map[string]*cgraph.Edge
	goCreateEdgeMap     map[string]map[int64]*cgraph.Edge
	fnStack             map[int64][]string
	mechanismClusterMap map[string]*cgraph.Graph
	mechanismAnchorMap  map[string]*cgraph.Node
	goroutineRootFunc   map[int64]string
}

type CleanUpFunc func()

func NewGraphState(ctx context.Context, steps trace.StepHistory) (*GraphState, CleanUpFunc, error) {
	gv, err := graphviz.New(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to exec NewGraphState; %w", err)
	}
	g, err := gv.Graph()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to exec NewGraphState; %w", err)
	}
	cleanup := func() {
		if err := g.Close(); err != nil {
			panic(err)
		}
		if err := gv.Close(); err != nil {
			panic(err)
		}
	}
	return &GraphState{
		ctx:                 ctx,
		gviz:                gv,
		g:                   g,
		steps:               steps,
		goroutineMap:        make(map[int64]*cgraph.Graph),
		funcNodeMap:         make(map[int64]map[string]*cgraph.Node),
		callEdgeMap:         make(map[int64]map[string]*cgraph.Edge),
		goCreateEdgeMap:     make(map[string]map[int64]*cgraph.Edge),
		fnStack:             make(map[int64][]string),
		mechanismClusterMap: make(map[string]*cgraph.Graph),
		mechanismAnchorMap:  make(map[string]*cgraph.Node),
		goroutineRootFunc:   make(map[int64]string),
	}, cleanup, nil
}

func (gs *GraphState) Load() (string, error) {
	// 1. 事前計算：全ステップをスキャンしてGoroutineのルート関数を特定する
	for _, step := range gs.steps {
		if step.Mode == trace.MODE_FUNC_ENTER {
			if _, ok := gs.goroutineRootFunc[step.GID]; !ok {
				gs.goroutineRootFunc[step.GID] = step.Func
			}
		}
	}

	// 2. グラフ要素を構築する
	for _, step := range gs.steps {
		switch step.Mode {
		case trace.MODE_FUNC_ENTER:
			// goroutine subgraph
			goCluster, err := gs.getOrCreateCluster(step)
			if err != nil {
				return "", fmt.Errorf("failed to get or create cluster: %w", err)
			}
			goCluster.SetStyle(STYLE_DASHED)

			// func node
			fnNode, err := gs.getOrCreateNode(goCluster, step)
			if err != nil {
				return "", fmt.Errorf("failed to get or create node: %w", err)
			}
			fnNode.SetStyle(STYLE_DASHED)

			// transition edge
			if len(gs.fnStack[step.GID]) > 0 {
				callEdge, err := gs.getOrCreateEdge(step)
				if err != nil {
					return "", fmt.Errorf("failed to get or create edge: %w", err)
				}
				callEdge.SetStyle(STYLE_DASHED)
			}

			gs.fnStack[step.GID] = append(gs.fnStack[step.GID], step.Func)
		case trace.MODE_FUNC_EXIT:
			if len(gs.fnStack[step.GID]) == 0 {
				text := fmt.Sprintf("failed to Load(%s): fnStack for GID %d is empty, but received exit event for func %s", trace.MODE_FUNC_EXIT, step.GID, step.Func)
				return "", errors.New(text)
			}
			top := len(gs.fnStack[step.GID]) - 1
			gs.fnStack[step.GID] = gs.fnStack[step.GID][:top]
		case trace.MODE_GO_CREATE:
			// 親Goroutineのクラスタと関数ノードを取得
			parentGoCluster, err := gs.getOrCreateCluster(trace.StepInfo{GID: step.GID})
			if err != nil {
				return "", fmt.Errorf("failed to get parent cluster: %w", err)
			}
			// 親の関数ノードがなければ作成
			if _, ok := gs.funcNodeMap[step.GID][step.Func]; !ok {
				if _, err := gs.getOrCreateNode(parentGoCluster, trace.StepInfo{GID: step.GID, Func: step.Func}); err != nil {
					return "", fmt.Errorf("failed to create parent node for go-create: %w", err)
				}
			}

			// 子Goroutineのクラスタを作成
			childGoCluster, err := gs.getOrCreateCluster(trace.StepInfo{GID: step.ChildGID})
			if err != nil {
				return "", fmt.Errorf("failed to create child cluster: %w", err)
			}
			childGoCluster.SetStyle(STYLE_DASHED)

			// 子Goroutineの開始ノードを作成
			childStartNode, err := gs.getOrCreateNode(childGoCluster, trace.StepInfo{GID: step.ChildGID, Func: fmt.Sprintf("start_goroutine_%d", step.ChildGID)})
			if err != nil {
				return "", fmt.Errorf("failed to create child start node: %w", err)
			}
			childStartNode.SetLabel(fmt.Sprintf("start from GID %d", step.GID))
			childStartNode.SetShape(SHAPE_POINT)

			// 親関数ノードから子開始ノードへのエッジを作成
			edge, err := gs.getOrCreateGoCreateEdge(step, childStartNode)
			if err != nil {
				return "", fmt.Errorf("failed to create go-create edge: %w", err)
			}
			edge.SetStyle(STYLE_DASHED)
		}
	}

	// 全体グラフの作成に用いた関数スタックはStep用に初期化しておく．
	for k := range gs.fnStack {
		delete(gs.fnStack, k)
	}

	mainCluster := gs.mechanismClusterMap["main.main"]
	mainNode, err := mainCluster.CreateNodeByName("main.main")
	mainNode.SetShape(SHAPE_POINT)
	mainNode.SetStyle(STYLE_INVIS)
	if err != nil {
		return "", err
	}
	for k, v := range gs.mechanismClusterMap {
		if k == "main.main" {
			continue
		}
		v.SetStyle(STYLE_INVIS)
		node, err := v.CreateNodeByName(k)
		node.SetShape(SHAPE_POINT)
		node.SetStyle(STYLE_INVIS)
		if err != nil {
			return "", err
		}
		edge, err := gs.g.CreateEdgeByName(fmt.Sprintf("main.main -> %s", k), mainNode, node)
		if err != nil {
			return "", err
		}
		edge.SetStyle(STYLE_INVIS)
	}

	gs.g.SetRankDir("TB")

	var buf bytes.Buffer
	if err := gs.gviz.Render(gs.ctx, gs.g, graphviz.SVG, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (gs *GraphState) Step() (string, bool, error) {
	step := gs.steps[gs.next]

	switch step.Mode {
	case trace.MODE_FUNC_ENTER:
		// goroutine subgraph
		goCluster, ok := gs.goroutineMap[step.GID]
		if !ok {
			text := fmt.Sprintf("failed to Step(@%s): cluster_goroutine_%d is not created", trace.MODE_FUNC_ENTER, step.GID)
			return "", false, errors.New(text)
		}
		goCluster.SetStyle(STYLE_FILLED)
		goCluster.SetBackgroundColor(COLOR_LIGHTGREEN)

		// func node
		if _, ok := gs.funcNodeMap[step.GID]; !ok {
			text := fmt.Sprintf("failed to Step(@%s): funcNodeMap[%d] is not created", trace.MODE_FUNC_ENTER, step.GID)
			return "", false, errors.New(text)
		}
		funcNode, ok := gs.funcNodeMap[step.GID][step.Func]
		if !ok {
			name := fmt.Sprintf("%s-gid%d", step.Func, step.GID)
			text := fmt.Sprintf("failed to Step(@%s): funcNode(%s) is not created", trace.MODE_FUNC_ENTER, name)
			return "", false, errors.New(text)
		}
		funcNode.SetStyle(STYLE_FILLED)
		funcNode.SetColor(COLOR_WHITE)

		// transisition edge
		if len(gs.fnStack[step.GID]) > 0 {
			if _, ok := gs.callEdgeMap[step.GID]; !ok {
				text := fmt.Sprintf("failed to Step(@%s): callEdgeMap[%d] is not created", trace.MODE_FUNC_ENTER, step.GID)
				return "", false, errors.New(text)
			}
			top := len(gs.fnStack[step.GID]) - 1
			label := fmt.Sprintf("%s:%d -> %s:%d", gs.fnStack[step.GID][top], step.GID, step.Func, step.GID)
			callEdge, ok := gs.callEdgeMap[step.GID][label]
			if !ok {
				text := fmt.Sprintf("failed to Step(@%s): funcNodeMap[%d] is not created", trace.MODE_FUNC_ENTER, step.GID)
				return "", false, errors.New(text)
			}
			callEdge.SetStyle(STYLE_FILLED)
			callEdge.SetColor(COLOR_RED)
		}

		gs.fnStack[step.GID] = append(gs.fnStack[step.GID], step.Func)
	case trace.MODE_FUNC_EXIT:
		if len(gs.fnStack[step.GID]) == 0 {
			text := fmt.Sprintf("failed to Step(%s): fnStack for GID %d is empty, but received exit event for func %s", trace.MODE_FUNC_EXIT, step.GID, step.Func)
			return "", false, errors.New(text)
		}

		goCluster, ok := gs.goroutineMap[step.GID]
		if !ok {
			text := fmt.Sprintf("failed to Step(%s): goroutineMap[%d] is not created", trace.MODE_FUNC_EXIT, step.GID)
			return "", false, errors.New(text)
		}

		if _, ok := gs.funcNodeMap[step.GID]; !ok {
			text := fmt.Sprintf("failed to Step(@%s): funcNodeMap[%d] is not created", trace.MODE_FUNC_ENTER, step.GID)
			return "", false, errors.New(text)
		}
		fnNode, ok := gs.funcNodeMap[step.GID][step.Func]
		if !ok {
			text := fmt.Sprintf("failed to Step(%s): funcNode(%s) is not created is not created", trace.MODE_FUNC_EXIT, step.Func)
			return "", false, errors.New(text)
		}
		fnNode.SetStyle(STYLE_DASHED)
		fnNode.SetColor(COLOR_BLACK)

		top := len(gs.fnStack[step.GID]) - 1
		if top > 0 {
			label := fmt.Sprintf("%s:%d -> %s:%d", gs.fnStack[step.GID][top-1], step.GID, gs.fnStack[step.GID][top], step.GID)
			callEdge, ok := gs.callEdgeMap[step.GID][label]
			if !ok {
				text := fmt.Sprintf("failed to Step(%s): callEdge[%d] is not created", trace.MODE_FUNC_EXIT, step.GID)
				return "", false, errors.New(text)
			}
			callEdge.SetStyle(STYLE_DASHED)
			callEdge.SetColor(COLOR_BLACK)
		} else {
			goCluster.SetBackgroundColor(COLOR_WHITE)
			goCluster.SetStyle(STYLE_DASHED)

			// Goroutine同士を繋ぐ有向辺を点線にする
			// このGoroutineを生成したエッジを探して更新
			for _, childMap := range gs.goCreateEdgeMap {
				for child, edge := range childMap {
					if child == step.GID {
						edge.SetStyle(STYLE_DASHED)
						edge.SetColor(COLOR_BLACK)
					}
				}
			}

			// Goroutineの開始ノードも黒色にする
			startNodeKey := fmt.Sprintf("start_goroutine_%d", step.GID)
			if startNode, ok := gs.funcNodeMap[step.GID][startNodeKey]; ok {
				startNode.SetStyle(STYLE_DASHED)
				startNode.SetColor(COLOR_BLACK)
			}
		}

		gs.fnStack[step.GID] = gs.fnStack[step.GID][:top]
	case trace.MODE_GO_CREATE:
		// 子Goroutineのクラスタをハイライト
		childGoCluster, ok := gs.goroutineMap[step.ChildGID]
		if !ok {
			return "", false, fmt.Errorf("child cluster %d not found", step.ChildGID)
		}
		childGoCluster.SetStyle(STYLE_FILLED)
		childGoCluster.SetBackgroundColor(COLOR_LIGHTGREEN)

		// 子Goroutineの開始ノードを可視化
		childStartNode, ok := gs.funcNodeMap[step.ChildGID][fmt.Sprintf("start_goroutine_%d", step.ChildGID)]
		if !ok {
			return "", false, fmt.Errorf("child start node for GID %d not found", step.ChildGID)
		}
		childStartNode.SetStyle(STYLE_FILLED)
		childStartNode.SetColor(COLOR_RED)

		// go-createエッジをハイライト
		// label := fmt.Sprintf("%d:%s -> %d", step.GID, step.Func, step.ChildGID)
		from := fmt.Sprintf("%d:%s", step.GID, step.Func)
		to := step.ChildGID
		edge, ok := gs.goCreateEdgeMap[from][to]
		if !ok {
			return "", false, fmt.Errorf("go-create edge '%s' not found", fmt.Sprintf("%s -> %d", from, to))
		}
		edge.SetStyle(STYLE_FILLED)
		edge.SetColor(COLOR_RED)
	}

	gs.next++

	var buf bytes.Buffer
	if err := gs.gviz.Render(gs.ctx, gs.g, graphviz.SVG, &buf); err != nil {
		return "", false, err
	}

	if gs.next >= len(gs.steps) {
		return buf.String(), false, nil
	}

	return buf.String(), true, nil
}

// StepInfo内部のGoroutine IDに対応するサブグラフがあれば取得し，無ければ作成する．
// さらに、Goroutineのルート関数に基づいて、より大きな「メカニズム」クラスタにグループ化する．
func (gs *GraphState) getOrCreateCluster(step trace.StepInfo) (*cgraph.Graph, error) {
	// このGoroutineのルート関数名を取得
	rootFuncName, ok := gs.goroutineRootFunc[step.GID]
	if !ok {
		// 事前計算により、ここに来ることはないはず
		return nil, fmt.Errorf("root function for GID %d not found", step.GID)
	}

	// メカニズムクラスタを取得または作成
	mechanismCluster, ok := gs.mechanismClusterMap[rootFuncName]
	if !ok {
		// クラスタ名にスラッシュが含まれているとエラーになるため置換する
		safeRootFuncName := strings.ReplaceAll(rootFuncName, "/", "_")
		clusterName := fmt.Sprintf("cluster_mechanism_%s", safeRootFuncName)
		mc, err := gs.g.CreateSubGraphByName(clusterName)
		if err != nil {
			return nil, fmt.Errorf("failed to create mechanism cluster '%s': %w", clusterName, err)
		}
		// mc.SetLabel(fmt.Sprintf("Mechanism: %s", rootFuncName))
		mc.SetStyle(STYLE_INVIS)
		gs.mechanismClusterMap[rootFuncName] = mc
		mechanismCluster = mc
	}

	// Goroutineクラスタをメカニズムクラスタの内部に取得または作成
	if _, ok := gs.goroutineMap[step.GID]; !ok {
		name := fmt.Sprintf("cluster_goroutine_%d", step.GID)
		goCluster, err := mechanismCluster.CreateSubGraphByName(name)
		if err != nil {
			return nil, fmt.Errorf("failed to init goroutine subgraph: %s; %w", name, err)
		}
		goCluster.SetLabel(fmt.Sprintf("Goroutine %d", step.GID))
		gs.goroutineMap[step.GID] = goCluster
	}

	return gs.goroutineMap[step.GID], nil
}

// Goroutine IDに対応した関数ノードがあれば取得し，無ければ作成する．
func (gs *GraphState) getOrCreateNode(goCluster *cgraph.Graph, step trace.StepInfo) (*cgraph.Node, error) {
	if _, ok := gs.funcNodeMap[step.GID]; !ok {
		gs.funcNodeMap[step.GID] = make(map[string]*cgraph.Node)
	}
	if _, ok := gs.funcNodeMap[step.GID][step.Func]; !ok {
		funcNode, err := goCluster.CreateNodeByName(fmt.Sprintf("%s-gid%d", step.Func, step.GID))
		if err != nil {
			return nil, fmt.Errorf("failed to create func node: %s; %w", step.Func, err)
		}
		funcNode.SetLabel(step.Func)
		gs.funcNodeMap[step.GID][step.Func] = funcNode
	}

	return gs.funcNodeMap[step.GID][step.Func], nil
}

// Goroutine IDが同じであるような新しい関数ノードと親関数ノードの有向辺があれば取得し，無ければ有向辺で繋ぐ．
func (gs *GraphState) getOrCreateEdge(step trace.StepInfo) (*cgraph.Edge, error) {
	if _, ok := gs.callEdgeMap[step.GID]; !ok {
		gs.callEdgeMap[step.GID] = make(map[string]*cgraph.Edge)
	}

	top := len(gs.fnStack[step.GID]) - 1
	parentFunc := gs.fnStack[step.GID][top]
	label := fmt.Sprintf("%s:%d -> %s:%d", parentFunc, step.GID, step.Func, step.GID)
	if _, ok := gs.callEdgeMap[step.GID][label]; !ok {
		callEdge, err := gs.g.CreateEdgeByName(label, gs.funcNodeMap[step.GID][parentFunc], gs.funcNodeMap[step.GID][step.Func])
		if err != nil {
			return nil, fmt.Errorf("failed to create edge: %s; %w", label, err)
		}
		gs.callEdgeMap[step.GID][label] = callEdge
	}
	return gs.callEdgeMap[step.GID][label], nil
}

// Goroutineをまたぐ新しい関数ノードと親関数ノードの有向辺があれば取得し，無ければ有向辺で繋ぐ．
func (gs *GraphState) getOrCreateGoCreateEdge(step trace.StepInfo, childNode *cgraph.Node) (*cgraph.Edge, error) {
	from := fmt.Sprintf("%d:%s", step.GID, step.Func)
	to := step.ChildGID
	label := fmt.Sprintf("%s -> %d", from, to)

	if _, ok := gs.goCreateEdgeMap[from]; !ok {
		gs.goCreateEdgeMap[from] = make(map[int64]*cgraph.Edge)
	}

	if _, ok := gs.goCreateEdgeMap[from][to]; !ok {
		parentFuncNode, ok := gs.funcNodeMap[step.GID][step.Func]
		if !ok {
			return nil, fmt.Errorf("parent function node '%s' not found in GID %d", step.Func, step.GID)
		}

		edge, err := gs.g.CreateEdgeByName(label, parentFuncNode, childNode)
		if err != nil {
			return nil, fmt.Errorf("failed to create go-create edge: %s; %w", label, err)
		}
		// edge.SetLabel("go")
		gs.goCreateEdgeMap[from][to] = edge
	}
	return gs.goCreateEdgeMap[from][to], nil
}
