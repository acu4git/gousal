package graph

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"wails-test/internal/trace"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
)

const (
	HEADER_CLUSTER_GOROUTINE = "cluster_goroutine_"
	STYLE_FILLED             = "filled"
	STYLE_INVIS              = "invis"
	STYLE_DASHED             = "dashed"
	COLOR_WHITE              = "white"
	COLOR_GRAY               = "gray"
	COLOR_LIGHTGREEN         = "lightgreen"
	COLOR_RED                = "red"
)

type GraphState struct {
	ctx          context.Context
	gviz         *graphviz.Graphviz
	g            *graphviz.Graph
	cancel       CleanUpFunc
	steps        trace.StepHistory
	next         int
	goroutineMap map[int64]*cgraph.Graph
	funcNodeMap  map[int64]map[string]*cgraph.Node
	callEdgeMap  map[int64]map[string]*cgraph.Edge
	fnStack      map[int64][]string
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
		ctx:          ctx,
		gviz:         gv,
		g:            g,
		steps:        steps,
		goroutineMap: make(map[int64]*cgraph.Graph),
		funcNodeMap:  make(map[int64]map[string]*cgraph.Node),
		callEdgeMap:  make(map[int64]map[string]*cgraph.Edge),
	}, cleanup, nil
}

func (gs *GraphState) Load() (string, error) {
	// var prev *trace.StepInfo

	for _, step := range gs.steps {
		// 	// goroutine subgraph
		// 	goCluster, ok := gs.goroutineMap[step.GID]
		// 	if !ok {
		// 		name := fmt.Sprintf("%s%d", HEADER_CLUSTER_GOROUTINE, step.GID)
		// 		goCluster, err = gs.g.CreateSubGraphByName(name)
		// 		if err != nil {
		// 			return "", fmt.Errorf("failed to init goroutine subgraph: %s; %w", name, err)
		// 		}
		// 		goCluster.SetLabel(fmt.Sprintf("Goroutine %d", step.GID))
		// 		// goCluster.SetStyle(STYLE_INVIS)
		// 		goCluster.SetStyle(STYLE_DASHED)
		// 		// goCluster.SetBackgroundColor(COLOR_GRAY)
		// 		gs.goroutineMap[step.GID] = goCluster
		// 	}

		// 	// func node
		// 	if _, ok := gs.funcNodeMap[step.GID]; !ok {
		// 		gs.funcNodeMap[step.GID] = make(map[string]*cgraph.Node)
		// 	}
		// 	if _, ok := gs.funcNodeMap[step.GID][step.Func]; !ok {
		// 		funcNode, err := goCluster.CreateNodeByName(fmt.Sprintf("%s-gid%d", step.Func, step.GID))
		// 		if err != nil {
		// 			return "", fmt.Errorf("failed to create func node: %s; %w", step.Func, err)
		// 		}
		// 		funcNode.SetLabel(step.Func)
		// 		// funcNode.SetStyle(STYLE_INVIS)
		// 		funcNode.SetStyle(STYLE_DASHED)
		// 		// funcNode.SetFillColor(COLOR_WHITE)
		// 		gs.funcNodeMap[step.GID][step.Func] = funcNode
		// 	}

		// 	// transisition edge
		// 	if prev != nil && prev.GID == step.GID {
		// 		if _, ok := gs.callEdgeMap[step.GID]; !ok {
		// 			gs.callEdgeMap[step.GID] = make(map[string]*cgraph.Edge)
		// 		}
		// 		label := fmt.Sprintf("%s:%d -> %s:%d", prev.Func, prev.GID, step.Func, step.GID)
		// 		if _, ok := gs.callEdgeMap[step.GID][label]; !ok {
		// 			callEdge, err := gs.g.CreateEdgeByName(label, gs.funcNodeMap[prev.GID][prev.Func], gs.funcNodeMap[step.GID][step.Func])
		// 			if err != nil {
		// 				return "", fmt.Errorf("failed to create edge: %s; %w", label, err)
		// 			}
		// 			// callEdge.SetStyle(STYLE_INVIS)
		// 			callEdge.SetStyle(STYLE_DASHED)
		// 			gs.callEdgeMap[step.GID][label] = callEdge
		// 		}
		// 	}
		// 	cpy := step
		// 	prev = &cpy

		switch step.Mode {
		case trace.MODE_FUNC_ENTER:
			// goroutine subgraph
			goCluster, err := gs.getOrCreateCluster(step)
			if err != nil {
				return "", nil
			}

			// func node
			if _, err := gs.getOrCreateNode(goCluster, step); err != nil {
				return "", err
			}

			// transisition edge
			if len(gs.fnStack[step.GID]) > 0 {
				if _, err := gs.getOrCreateEdge(step); err != nil {
					return "", nil
				}
			}

			gs.fnStack[step.GID] = append(gs.fnStack[step.GID], step.Func)
		case trace.MODE_FUNC_EXIT:
			// 	goCluster, err := gs.getOrCreateCluster(step)
			// 	if err != nil {
			// 		return "", err
			// 	}
			// 	fnNode, err := gs.getOrCreateNode(goCluster, step)
			// 	if err != nil {
			// 		return "", err
			// 	}
			// 	// remove node
			// 	fnNode.SetStyle(STYLE_DASHED)

			// 	top := len(gs.fnStack[step.GID]) - 1
			// 	if top > 0 {
			// 		// remove edge
			// 		gs.fnStack[step.GID] = gs.fnStack[step.GID][:top]
			// 		callEdge, err := gs.getOrCreateEdge(step)
			// 		if err != nil {
			// 			return "", err
			// 		}
			// 		callEdge.SetStyle(STYLE_DASHED)
			// 	} else {
			// 		// remove subgraph
			// 		goCluster.SetBackgroundColor(COLOR_GRAY)
			// 	}
		}
	}

	var buf bytes.Buffer
	if err := gs.gviz.Render(gs.ctx, gs.g, graphviz.SVG, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (gs *GraphState) Step() (string, bool, error) {
	step := gs.steps[gs.next]
	// goroutine subgraph
	goCluster, ok := gs.goroutineMap[step.GID]
	if !ok {
		text := fmt.Sprintf("failed to Step(): %s%d is not created", HEADER_CLUSTER_GOROUTINE, step.GID)
		return "", false, errors.New(text)
	}
	goCluster.SetStyle(STYLE_FILLED)

	// func node
	if _, ok := gs.funcNodeMap[step.GID]; !ok {
		text := fmt.Sprintf("failed to Step(): funcNodeMap[%d] is not created", step.GID)
		return "", false, errors.New(text)
	}
	funcNode, ok := gs.funcNodeMap[step.GID][step.Func]
	if !ok {
		name := fmt.Sprintf("%s-gid%d", step.Func, step.GID)
		text := fmt.Sprintf("failed to Step(): funcNode(%s) is not created", name)
		return "", false, errors.New(text)
	}
	funcNode.SetStyle(STYLE_FILLED)

	// transisition edge
	if gs.next > 0 {
		if _, ok := gs.callEdgeMap[step.GID]; !ok {
			text := fmt.Sprintf("failed to Step(): callEdgeMap[%d] is not created", step.GID)
			return "", false, errors.New(text)
		}
		label := fmt.Sprintf("%s:%d -> %s:%d", gs.steps[gs.next-1].Func, gs.steps[gs.next-1].GID, step.Func, step.GID)
		callEdge, ok := gs.callEdgeMap[step.GID][label]
		if !ok {
			text := fmt.Sprintf("failed to Step(): funcNodeMap[%d] is not created", step.GID)
			return "", false, errors.New(text)
		}
		callEdge.SetStyle(STYLE_FILLED)
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

// StepInfo内部のGoroutine IDに対応するサブグラフがあれば取得し，無ければ"style=dashed"で作成する．
func (gs *GraphState) getOrCreateCluster(step trace.StepInfo) (*cgraph.Graph, error) {
	if _, ok := gs.goroutineMap[step.GID]; !ok {
		name := fmt.Sprintf("%s%d", HEADER_CLUSTER_GOROUTINE, step.GID)
		goCluster, err := gs.g.CreateSubGraphByName(name)
		if err != nil {
			return nil, fmt.Errorf("failed to init goroutine subgraph: %s; %w", name, err)
		}
		goCluster.SetLabel(fmt.Sprintf("Goroutine %d", step.GID))
		goCluster.SetStyle(STYLE_DASHED)
		gs.goroutineMap[step.GID] = goCluster
	}

	return gs.goroutineMap[step.GID], nil
}

// Goroutine IDに対応した関数ノードがあれば取得し，無ければ"style=dashed"で作成する．s
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
		// funcNode.SetStyle(STYLE_INVIS)
		funcNode.SetStyle(STYLE_DASHED)
		// funcNode.SetFillColor(COLOR_WHITE)
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
		// callEdge.SetStyle(STYLE_INVIS)
		callEdge.SetStyle(STYLE_DASHED)
		gs.callEdgeMap[step.GID][label] = callEdge
	}
	return gs.callEdgeMap[step.GID][label], nil
}
