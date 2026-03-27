package orchestrator

import "github.com/dhruvjaink07/failsafe/internal/models"

func (o *Orchestrator) computeGraphImpactScore(
	exp *models.Experiment,
	degraded map[string]bool,
) (float64, int) {
	total := exp.GraphMetadata.TotalNodes
	if total == 0 {
		return 0, 0
	}

	affected := len(degraded)
	blast := float64(affected) / float64(total) * 100
	depth := o.computeCascadeDepth(exp, degraded)

	return blast, depth
}

func (o *Orchestrator) computeCascadeDepth(exp *models.Experiment, degraded map[string]bool) int {
	maxDepth := 0

	var dfs func(node string, depth int, visited map[string]bool)

	dfs = func(node string, depth int, visited map[string]bool) {
		if visited[node] {
			return
		}

		visited[node] = true

		if depth > maxDepth {
			maxDepth = depth
		}

		for _, dep := range exp.DependencyGraph[node] {
			if degraded[dep] {
				dfs(dep, depth+1, visited)
			}
		}
	}

	for node := range degraded {
		visited := make(map[string]bool)
		dfs(node, 1, visited)
	}

	return maxDepth
}

func computeGraphMeta(graph models.DependencyGraph) models.GraphMetadata {
	visited := make(map[string]bool)
	maxDepth := 0

	var dfs func(string, int)
	dfs = func(node string, depth int) {
		if visited[node] {
			return
		}
		visited[node] = true

		if depth > maxDepth {
			maxDepth = depth
		}

		for _, n := range graph[node] {
			dfs(n, depth+1)
		}
	}

	for node := range graph {
		dfs(node, 1)
	}

	return models.GraphMetadata{
		TotalNodes: len(visited),
		MaxDepth:   maxDepth,
	}
}
