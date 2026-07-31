package agents

// Registry of the ten specialized agents.

var All = []Agent{
	NewPlanner(),
	NewArchitect(),
	NewCoder(),
	NewReviewer(),
	NewDebugger(),
	NewOptimizer(),
	NewSecurityAuditor(),
	NewDocWriter(),
	NewTestEngineer(),
	NewReleaseManager(),
}

func Find(name string) (Agent, bool) {
	for _, a := range All {
		if a.Name() == name {
			return a, true
		}
	}
	return nil, false
}

func Names() []string {
	var names []string
	for _, a := range All {
		names = append(names, a.Name())
	}
	return names
}
