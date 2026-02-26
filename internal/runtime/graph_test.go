package runtime

import (
	"context"
	"testing"

	"deeph/internal/project"
	"deeph/internal/typesys"
)

func TestParseAgentSpecGraph(t *testing.T) {
	g, err := ParseAgentSpecGraph("planner+reader>coder>reviewer+linter")
	if err != nil {
		t.Fatalf("ParseAgentSpecGraph returned error: %v", err)
	}
	if got, want := len(g.Stages), 3; got != want {
		t.Fatalf("stages=%d want=%d", got, want)
	}
	if got := g.Stages[0][0]; got != "planner" {
		t.Fatalf("stage[0][0]=%q", got)
	}
	if got := g.Stages[2][1]; got != "linter" {
		t.Fatalf("stage[2][1]=%q", got)
	}
}

func TestParseAgentSpecGraphRejectsDuplicate(t *testing.T) {
	if _, err := ParseAgentSpecGraph("a+b>a"); err == nil {
		t.Fatal("expected duplicate agent error, got nil")
	}
}

func TestInferTaskHandoffLinksByTypedPorts(t *testing.T) {
	src := Task{
		Agent: project.AgentConfig{
			Name: "coder",
			IO: project.AgentIOConfig{
				Outputs: []project.IOPortConfig{
					{Name: "patch", Produces: []string{"text/diff"}},
					{Name: "summary", Produces: []string{"summary/code"}},
				},
			},
		},
	}
	dst := Task{
		Agent: project.AgentConfig{
			Name: "reviewer",
			IO: project.AgentIOConfig{
				Inputs: []project.IOPortConfig{
					{Name: "changes", Accepts: []string{"text/diff"}, Required: true, MergePolicy: "latest", ChannelPriority: 3, MaxTokens: 40},
					{Name: "brief", Accepts: []string{"summary/code"}, MergePolicy: "append4", MaxTokens: 60},
				},
			},
		},
	}

	links := inferTaskHandoffLinks(src, dst)
	if len(links) != 2 {
		t.Fatalf("links=%d want=2 (%v)", len(links), links)
	}

	var foundDiff, foundSummary bool
	for _, l := range links {
		switch {
		case l.FromPort == "patch" && l.ToPort == "changes" && l.Kind == typesys.KindTextDiff:
			foundDiff = true
			if l.Channel == "" {
				t.Fatalf("expected channel id on handoff: %+v", l)
			}
			if !l.Required {
				t.Fatalf("expected required=true for changes handoff")
			}
			if l.MergePolicy != "latest" || l.TargetMaxTokens != 40 || l.ChannelPriority != 3 {
				t.Fatalf("expected changes port merge/latest prio=3 max_tokens=40, got merge=%q prio=%v max_tokens=%d", l.MergePolicy, l.ChannelPriority, l.TargetMaxTokens)
			}
		case l.FromPort == "summary" && l.ToPort == "brief" && l.Kind == typesys.KindSummaryCode:
			foundSummary = true
			if l.Channel == "" {
				t.Fatalf("expected channel id on handoff: %+v", l)
			}
			if l.MergePolicy != "append4" || l.TargetMaxTokens != 60 {
				t.Fatalf("expected brief port merge_policy/append4 + max_tokens=60, got merge=%q max_tokens=%d", l.MergePolicy, l.TargetMaxTokens)
			}
		}
	}
	if !foundDiff || !foundSummary {
		t.Fatalf("expected diff and summary links, got=%v", links)
	}
}

func TestInferTaskHandoffLinksFallback(t *testing.T) {
	src := Task{Agent: project.AgentConfig{Name: "a"}}
	dst := Task{Agent: project.AgentConfig{Name: "b"}}
	links := inferTaskHandoffLinks(src, dst)
	if len(links) != 1 {
		t.Fatalf("links=%d want=1", len(links))
	}
	if links[0].Kind != typesys.KindMessageAgent {
		t.Fatalf("fallback kind=%s want=%s", links[0].Kind, typesys.KindMessageAgent)
	}
	if links[0].Channel == "" {
		t.Fatalf("expected fallback channel id, got empty")
	}
}

func TestInferTaskHandoffLinksRespectsDependsOnPortsSelectors(t *testing.T) {
	srcPlanner := Task{
		Agent: project.AgentConfig{
			Name: "planner",
			IO: project.AgentIOConfig{
				Outputs: []project.IOPortConfig{
					{Name: "brief", Produces: []string{"summary/text"}},
				},
			},
		},
	}
	srcCoder := Task{
		Agent: project.AgentConfig{
			Name: "coder",
			IO: project.AgentIOConfig{
				Outputs: []project.IOPortConfig{
					{Name: "brief", Produces: []string{"summary/text"}},
				},
			},
		},
	}
	dst := Task{
		Agent: project.AgentConfig{
			Name: "reviewer",
			DependsOnPorts: map[string][]string{
				"brief": []string{"planner.brief"},
			},
			IO: project.AgentIOConfig{
				Inputs: []project.IOPortConfig{
					{Name: "brief", Accepts: []string{"summary/text"}},
				},
			},
		},
	}

	linksPlanner := inferTaskHandoffLinks(srcPlanner, dst)
	if len(linksPlanner) != 1 {
		t.Fatalf("planner links=%d want=1 (%v)", len(linksPlanner), linksPlanner)
	}
	if linksPlanner[0].FromAgent != "planner" || linksPlanner[0].ToPort != "brief" {
		t.Fatalf("unexpected planner link: %+v", linksPlanner[0])
	}

	linksCoder := inferTaskHandoffLinks(srcCoder, dst)
	if len(linksCoder) != 0 {
		t.Fatalf("coder should be filtered by depends_on_ports, got=%v", linksCoder)
	}
}

func TestPlanSpecAddsExplicitNonAdjacentDependency(t *testing.T) {
	p := &project.Project{
		Root: project.RootConfig{
			Version:         1,
			DefaultProvider: "mockp",
			Providers: []project.ProviderConfig{
				{Name: "mockp", Type: "mock", Model: "mock-small"},
			},
		},
		Agents: []project.AgentConfig{
			{Name: "planner"},
			{Name: "coder"},
			{Name: "reviewer", DependsOn: []string{"planner"}},
		},
		AgentFiles: map[string]string{},
	}
	eng, err := New(t.TempDir(), p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	plan, tasks, err := eng.PlanSpec(context.Background(), "planner>coder>reviewer", "x")
	if err != nil {
		t.Fatalf("PlanSpec: %v", err)
	}
	if len(plan.Handoffs) < 3 {
		t.Fatalf("expected >=3 handoffs (stage + explicit), got %d", len(plan.Handoffs))
	}
	if got := tasks[2].DependsOn; len(got) != 2 {
		t.Fatalf("reviewer depends_on=%v want [coder planner]", got)
	}
	foundPlannerToReviewer := false
	for _, h := range plan.Handoffs {
		if h.FromAgent == "planner" && h.ToAgent == "reviewer" {
			foundPlannerToReviewer = true
			break
		}
	}
	if !foundPlannerToReviewer {
		t.Fatalf("expected explicit handoff planner->reviewer in plan.Handoffs: %+v", plan.Handoffs)
	}
}

func TestPlanSpecAddsDependencyFromDependsOnPorts(t *testing.T) {
	p := &project.Project{
		Root: project.RootConfig{
			Version:         1,
			DefaultProvider: "mockp",
			Providers: []project.ProviderConfig{
				{Name: "mockp", Type: "mock", Model: "mock-small"},
			},
		},
		Agents: []project.AgentConfig{
			{Name: "planner"},
			{Name: "coder"},
			{
				Name: "reviewer",
				DependsOnPorts: map[string][]string{
					"brief": []string{"planner"},
				},
				IO: project.AgentIOConfig{
					Inputs: []project.IOPortConfig{{Name: "brief", Accepts: []string{"text/plain"}}},
				},
			},
		},
		AgentFiles: map[string]string{},
	}
	eng, err := New(t.TempDir(), p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	plan, tasks, err := eng.PlanSpec(context.Background(), "planner>coder>reviewer", "x")
	if err != nil {
		t.Fatalf("PlanSpec: %v", err)
	}
	if got := tasks[2].DependsOn; len(got) != 1 || got[0] != "planner" {
		t.Fatalf("reviewer depends_on=%v want [planner]", got)
	}
	foundPlannerToReviewer := false
	for _, h := range plan.Handoffs {
		if h.FromAgent == "planner" && h.ToAgent == "reviewer" && h.ToPort == "brief" {
			foundPlannerToReviewer = true
			break
		}
	}
	if !foundPlannerToReviewer {
		t.Fatalf("expected planner->reviewer brief handoff, got %+v", plan.Handoffs)
	}
	if !plan.Parallel {
		t.Fatalf("expected plan.Parallel=true due selective cross-stage wait overlap potential")
	}
}

func TestPlanSpecRejectsDependencyOnLaterStage(t *testing.T) {
	p := &project.Project{
		Root: project.RootConfig{
			Version:         1,
			DefaultProvider: "mockp",
			Providers: []project.ProviderConfig{
				{Name: "mockp", Type: "mock", Model: "mock-small"},
			},
		},
		Agents: []project.AgentConfig{
			{Name: "a", DependsOn: []string{"b"}},
			{Name: "b"},
		},
		AgentFiles: map[string]string{},
	}
	eng, err := New(t.TempDir(), p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, _, err := eng.PlanSpec(context.Background(), "a>b", "x"); err == nil {
		t.Fatal("expected later-stage dependency error, got nil")
	}
}

func TestPlanSpecRejectsDependsOnPortsAgentInLaterStage(t *testing.T) {
	p := &project.Project{
		Root: project.RootConfig{
			Version:         1,
			DefaultProvider: "mockp",
			Providers: []project.ProviderConfig{
				{Name: "mockp", Type: "mock", Model: "mock-small"},
			},
		},
		Agents: []project.AgentConfig{
			{Name: "a", DependsOnPorts: map[string][]string{"input": []string{"b"}}},
			{Name: "b"},
		},
		AgentFiles: map[string]string{},
	}
	eng, err := New(t.TempDir(), p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, _, err := eng.PlanSpec(context.Background(), "a>b", "x"); err == nil {
		t.Fatal("expected later-stage depends_on_ports error, got nil")
	}
}
