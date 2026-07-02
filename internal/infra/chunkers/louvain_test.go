package chunkers

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────
// These helpers mirror the ones that lived in the now-deleted
// louvain_compare_test.go. They are test-only utilities for building weighted
// graphs from a compact edge-spec notation and for computing modularity.

// buildTestGraph creates a weighted undirected graph from a list of edge specs.
// Each spec is "node1 node2 weight". Both endpoints are added as graph nodes.
func buildTestGraph(specs []string) map[string]map[string]int {
	graph := make(map[string]map[string]int)
	for _, s := range specs {
		parts := strings.Fields(s)
		if len(parts) < 3 {
			continue
		}
		u, v := parts[0], parts[1]
		w := 0
		fmt.Sscanf(parts[2], "%d", &w)
		if graph[u] == nil {
			graph[u] = make(map[string]int)
		}
		if graph[v] == nil {
			graph[v] = make(map[string]int)
		}
		graph[u][v] = w
		graph[v][u] = w
	}
	return graph
}

// modularity computes the standard Newman-Girvan modularity for a partition.
// Q = (1/2m) * Σ_ij [A_ij - (k_i * k_j / 2m)] * δ(c_i, c_j)
// Returns 0 for graphs with no edges.
func modularity(graph map[string]map[string]int, clusters [][]string) float64 {
	// Build node -> cluster index
	nodeCluster := make(map[string]int)
	for ci, cluster := range clusters {
		for _, n := range cluster {
			nodeCluster[n] = ci
		}
	}

	// Total edge weight (2m)
	totalWeight := 0
	for u := range graph {
		for _, w := range graph[u] {
			totalWeight += w
		}
	}
	if totalWeight == 0 {
		return 0
	}
	m := float64(totalWeight) / 2.0

	// Weighted degree of each node
	k := make(map[string]float64)
	for u := range graph {
		var sum int
		for _, w := range graph[u] {
			sum += w
		}
		k[u] = float64(sum)
	}

	var q float64
	for u := range graph {
		for v, w := range graph[u] {
			if nodeCluster[u] == nodeCluster[v] {
				aij := float64(w)
				q += aij - (k[u]*k[v])/(2.0*m)
			}
		}
	}
	q /= 2.0 * m

	return q
}

// ─── 3.1 louvainClusters ─────────────────────────────────────────────────────

func TestLouvainClusters(t *testing.T) {
	tests := []struct {
		name           string
		graph          map[string]map[string]int
		wantClusters   int     // expected number of clusters
		wantModularity float64 // minimum modularity (0 if trivial)
	}{
		{
			name:           "0 nodes",
			graph:          map[string]map[string]int{},
			wantClusters:   0,
			wantModularity: 0,
		},
		{
			name:           "1 node",
			graph:          map[string]map[string]int{"a.go": {}},
			wantClusters:   1,
			wantModularity: 0,
		},
		{
			name:           "no edges",
			graph:          map[string]map[string]int{"a.go": {}, "b.go": {}},
			wantClusters:   2,
			wantModularity: 0,
		},
		{
			name: "chain strong-weak-strong",
			graph: buildTestGraph([]string{
				"a.go b.go 1000",
				"b.go c.go 1",
				"c.go d.go 1000",
			}),
			wantClusters:   2,
			wantModularity: 0.7,
		},
		{
			name: "dense groups",
			graph: buildTestGraph([]string{
				"a1.go a2.go 100", "a1.go a3.go 100", "a2.go a3.go 100",
				"b1.go b2.go 100", "b1.go b3.go 100", "b2.go b3.go 100",
			}),
			wantClusters:   2,
			wantModularity: 0.3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := louvainClusters(tt.graph)
			if len(got) != tt.wantClusters {
				t.Errorf("louvainClusters() got %d clusters, want %d", len(got), tt.wantClusters)
			}
			if tt.wantModularity > 0 {
				q := modularity(tt.graph, got)
				if q < tt.wantModularity {
					t.Errorf("modularity = %.4f, want >= %.4f", q, tt.wantModularity)
				}
			}
		})
	}
}

func TestLouvainClusters_Deterministic(t *testing.T) {
	graph := buildTestGraph([]string{
		"a.go b.go 1000",
		"b.go c.go 1",
		"c.go d.go 1000",
		"d.go e.go 500",
		"e.go f.go 100",
	})
	first := louvainClusters(graph)
	for i := 0; i < 10; i++ {
		got := louvainClusters(graph)
		if !reflect.DeepEqual(first, got) {
			t.Errorf("iteration %d: different result", i)
		}
	}
}

// ─── 3.2 mergeBridgeClusters ─────────────────────────────────────────────────

func TestMergeBridgeClusters(t *testing.T) {
	tests := []struct {
		name     string
		clusters [][]string
		graph    map[string]map[string]int
		want     int // expected number of clusters after merge
	}{
		{
			name:     "strong bridge 600",
			clusters: [][]string{{"a.go"}, {"b.go"}},
			graph:    buildTestGraph([]string{"a.go b.go 600"}),
			want:     1,
		},
		{
			name:     "weak bridge 400",
			clusters: [][]string{{"a.go"}, {"b.go"}},
			graph:    buildTestGraph([]string{"a.go b.go 400"}),
			want:     2,
		},
		{
			name:     "single cluster",
			clusters: [][]string{{"a.go", "b.go"}},
			graph:    buildTestGraph([]string{"a.go b.go 600"}),
			want:     1,
		},
		{
			name:     "no edges",
			clusters: [][]string{{"a.go"}, {"b.go"}},
			graph:    map[string]map[string]int{"a.go": {}, "b.go": {}},
			want:     2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeBridgeClusters(tt.clusters, tt.graph)
			if len(got) != tt.want {
				t.Errorf("mergeBridgeClusters() got %d clusters, want %d", len(got), tt.want)
			}
		})
	}
}

// ─── 3.3 splitOversizedClusters ──────────────────────────────────────────────

func TestSplitOversizedClusters(t *testing.T) {
	graph := buildTestGraph([]string{
		"a.go b.go 100", "a.go c.go 100", "a.go d.go 100",
		"b.go c.go 100", "b.go d.go 100", "c.go d.go 100",
	})
	// Add more files with weaker connections to node a.go.
	for _, f := range []string{"e.go", "f.go", "g.go", "h.go", "i.go", "j.go", "k.go", "l.go"} {
		graph[f] = map[string]int{"a.go": 10}
		graph["a.go"][f] = 10
	}

	tests := []struct {
		name     string
		clusters [][]string
		maxFiles int
		want     int // expected number of clusters after split
	}{
		{
			name: "12 files max 8",
			clusters: [][]string{{
				"a.go", "b.go", "c.go", "d.go", "e.go", "f.go",
				"g.go", "h.go", "i.go", "j.go", "k.go", "l.go",
			}},
			maxFiles: 8,
			want:     2,
		},
		{
			name: "8 files max 8",
			clusters: [][]string{{
				"a.go", "b.go", "c.go", "d.go", "e.go", "f.go", "g.go", "h.go",
			}},
			maxFiles: 8,
			want:     1,
		},
		{
			name:     "0 files",
			clusters: [][]string{},
			maxFiles: 8,
			want:     0,
		},
		{
			name:     "maxFiles 0 (disabled)",
			clusters: [][]string{{"a.go", "b.go"}},
			maxFiles: 0,
			want:     1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitOversizedClusters(tt.clusters, graph, tt.maxFiles)
			if len(got) != tt.want {
				t.Errorf("splitOversizedClusters() got %d clusters, want %d", len(got), tt.want)
			}
		})
	}
}

// ─── 3.4 createClusters end-to-end integration ───────────────────────────────

func TestCreateClusters_EndToEnd(t *testing.T) {
	// Build a realistic graph using buildGraph + calculateForce, then run
	// createClusters. Files come from different directories so the directory
	// affinity (+100) creates intra-package edges without any cross-package
	// edges. Every file must end up in exactly one cluster.
	files := []*gitdiff.File{
		{NewName: "auth/login.go", OldName: "auth/login.go"},
		{NewName: "auth/session.go", OldName: "auth/session.go"},
		{NewName: "db/query.go", OldName: "db/query.go"},
		{NewName: "db/migrate.go", OldName: "db/migrate.go"},
		{NewName: "ui/button.go", OldName: "ui/button.go"},
	}

	catalog := NewLanguageCatalog()
	u := NewUnifiedASTPass(catalog)
	u.maxFilesPerChunk = 10

	filenames := []string{
		"auth/login.go", "auth/session.go",
		"db/query.go", "db/migrate.go",
		"ui/button.go",
	}
	symbols := make(map[string]FileSymbols)
	graph := u.buildGraph(files, symbols)
	clusters := u.createClusters(graph, filenames)

	// Verify every file appears in exactly one cluster.
	seen := make(map[string]bool)
	for _, cluster := range clusters {
		for _, f := range cluster {
			if seen[f] {
				t.Errorf("file %s appears in multiple clusters", f)
			}
			seen[f] = true
		}
	}
	for _, f := range filenames {
		if !seen[f] {
			t.Errorf("file %s missing from clusters", f)
		}
	}

	// Sanity: the two auth files (same dir → +100 force) must be together.
	authTogether := false
	for _, cluster := range clusters {
		if containsStr(cluster, "auth/login.go") && containsStr(cluster, "auth/session.go") {
			authTogether = true
		}
	}
	if !authTogether {
		t.Errorf("auth/login.go and auth/session.go should be in the same cluster (dir affinity +100)")
	}
}

// ─── 3.5 Regression safety ───────────────────────────────────────────────────
// The louvain_compare_test.go file was deleted (it served its design-validation
// purpose). The full `go test ./internal/infra/chunkers/` run is the regression
// gate (task 3.6).
