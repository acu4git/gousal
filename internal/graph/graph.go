package graph

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"wails-test/internal"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
)

const (
	HEADER_CLUSTER_GOROUTINE = "cluster_goroutine_"
	STYLE_FILLED             = "filled"
	STYLE_INVIS              = "invis"
	COLOR_WHITE              = "white"
)

type GraphState struct {
	ctx          context.Context
	gviz         *graphviz.Graphviz
	g            *graphviz.Graph
	cancel       CleanUpFunc
	steps        internal.StepHistory
	next         int
	goroutineMap map[int64]*cgraph.Graph
	funcNodeMap  map[int64]map[string]*cgraph.Node
	callEdgeMap  map[int64]map[string]*cgraph.Edge
}

type CleanUpFunc func()

func NewGraphState(ctx context.Context, steps internal.StepHistory) (*GraphState, CleanUpFunc, error) {
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

func (gs *GraphState) Load() error {
	var (
		err  error
		prev *internal.StepInfo
	)
	for _, step := range gs.steps {
		// goroutine subgraph
		goCluster, ok := gs.goroutineMap[step.GID]
		if !ok {
			name := fmt.Sprintf("%s%d", HEADER_CLUSTER_GOROUTINE, step.GID)
			goCluster, err = gs.g.CreateSubGraphByName(name)
			if err != nil {
				return fmt.Errorf("failed to init goroutine subgraph: %s; %w", name, err)
			}
			goCluster.SetLabel(fmt.Sprintf("Goroutine %d", step.GID))
			// goCluster.SetStyle(STYLE_FILLED)
			goCluster.SetStyle(STYLE_INVIS)
			gs.goroutineMap[step.GID] = goCluster
		}

		// func node
		if _, ok := gs.funcNodeMap[step.GID]; !ok {
			gs.funcNodeMap[step.GID] = make(map[string]*cgraph.Node)
		}
		funcNode, ok := gs.funcNodeMap[step.GID][step.Func]
		if !ok {
			funcNode, err := goCluster.CreateNodeByName(fmt.Sprintf("%s-gid%d", step.Func, step.GID))
			if err != nil {
				return fmt.Errorf("failed to create func node: %s; %w", step.Func, err)
			}
			funcNode.SetLabel(step.Func)
			// funcNode.SetStyle(STYLE_FILLED)
			funcNode.SetStyle(STYLE_INVIS)
			funcNode.SetFillColor(COLOR_WHITE)
			gs.funcNodeMap[step.GID][step.Func] = funcNode
		}

		// transisition edge
		if prev != nil {
			if _, ok := gs.callEdgeMap[step.GID]; !ok {
				gs.callEdgeMap[step.GID] = make(map[string]*cgraph.Edge)
			}
			label := fmt.Sprintf("%s:%d -> %s:%d", prev.Func, prev.GID, step.Func, step.GID)
			callEdge, ok := gs.callEdgeMap[step.GID][label]
			if !ok {
				callEdge, err = gs.g.CreateEdgeByName(label, gs.funcNodeMap[prev.GID][prev.Func], funcNode)
				if err != nil {
					return fmt.Errorf("failed to create edge: %s; %w", label, err)
				}
				callEdge.SetStyle(STYLE_INVIS)
				gs.callEdgeMap[step.GID][label] = callEdge
			}
		}
		cpy := step
		prev = &cpy
	}
	return nil
}

func (gs *GraphState) Step() (string, error) {
	step := gs.steps[gs.next]
	// goroutine subgraph
	goCluster, ok := gs.goroutineMap[step.GID]
	if !ok {
		text := fmt.Sprintf("failed to Step(): %s%d is not created", HEADER_CLUSTER_GOROUTINE, step.GID)
		return "", errors.New(text)
	}
	goCluster.SetStyle(STYLE_FILLED)

	// func node
	if _, ok := gs.funcNodeMap[step.GID]; !ok {
		text := fmt.Sprintf("failed to Step(): funcNodeMap[%d] is not created", step.GID)
		return "", errors.New(text)
	}
	funcNode, ok := gs.funcNodeMap[step.GID][step.Func]
	if !ok {
		name := fmt.Sprintf("%s-gid%d", step.Func, step.GID)
		text := fmt.Sprintf("failed to Step(): funcNode(%s) is not created", name)
		return "", errors.New(text)
	}
	funcNode.SetStyle(STYLE_FILLED)

	// transisition edge
	if gs.next > 0 {
		if _, ok := gs.callEdgeMap[step.GID]; !ok {
			text := fmt.Sprintf("failed to Step(): callEdgeMap[%d] is not created", step.GID)
			return "", errors.New(text)
		}
		label := fmt.Sprintf("%s:%d -> %s:%d", gs.steps[gs.next-1].Func, gs.steps[gs.next-1].GID, step.Func, step.GID)
		callEdge, ok := gs.callEdgeMap[step.GID][label]
		if !ok {
			text := fmt.Sprintf("failed to Step(): funcNodeMap[%d] is not created", step.GID)
			return "", errors.New(text)
		}
		callEdge.SetStyle(STYLE_FILLED)
	}
	gs.next++

	var buf bytes.Buffer
	if err := gs.gviz.Render(gs.ctx, gs.g, graphviz.SVG, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}
