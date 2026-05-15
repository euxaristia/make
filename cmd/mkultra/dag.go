package main

type DagNode struct {
	Target   string
	Prereqs  []string
	Recipes  []string
	IsPhony  bool
}

type Dag struct {
	Nodes         map[string]*DagNode
	Variables     map[string]string
	overridden    map[string]bool
	DefaultTarget string
}

func NewDag() *Dag {
	return &Dag{
		Nodes:      make(map[string]*DagNode),
		Variables:  make(map[string]string),
		overridden: make(map[string]bool),
	}
}

func (d *Dag) SetVariable(name, value string) {
	if !d.overridden[name] {
		d.Variables[name] = value
	}
}

func (d *Dag) SetOverride(name, value string) {
	d.Variables[name] = value
	d.overridden[name] = true
}

func (d *Dag) EnsureNode(target string, isPhony bool) *DagNode {
	if n, ok := d.Nodes[target]; ok {
		return n
	}
	n := &DagNode{Target: target, IsPhony: isPhony}
	d.Nodes[target] = n
	return n
}

func (d *Dag) AddPrereq(target, prereq string) {
	d.EnsureNode(target, false).Prereqs = append(d.EnsureNode(target, false).Prereqs, prereq)
	d.EnsureNode(prereq, false)
	if d.DefaultTarget == "" && target != "" && target[0] != '.' && target != ".PHONY" && target != ".SUFFIXES" {
		d.DefaultTarget = target
	}
}

func (d *Dag) AddRecipe(target, recipe string) {
	d.EnsureNode(target, false).Recipes = append(d.EnsureNode(target, false).Recipes, recipe)
}

func (d *Dag) SetDefault(target string) {
	if d.DefaultTarget == "" {
		d.DefaultTarget = target
	}
}

func (d *Dag) DetectCycle() []string {
	color := make(map[string]uint8)
	parent := make(map[string]string)

	var dfs func(string) []string
	dfs = func(node string) []string {
		switch color[node] {
		case 0:
			color[node] = 1
			n := d.Nodes[node]
			if n != nil {
				for _, p := range n.Prereqs {
					parent[p] = node
					if cycle := dfs(p); cycle != nil {
						return cycle
					}
				}
			}
			color[node] = 2
		case 1:
			var cycle []string
			cycle = append(cycle, node)
			cur := node
			for hops := 0; hops < len(d.Nodes); hops++ {
				p, ok := parent[cur]
				if !ok || p == node {
					break
				}
				cycle = append(cycle, p)
				cur = p
			}
			for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
				cycle[i], cycle[j] = cycle[j], cycle[i]
			}
			return cycle
		}
		return nil
	}

	for name := range d.Nodes {
		if color[name] == 0 {
			if cycle := dfs(name); cycle != nil {
				return cycle
			}
		}
	}
	return nil
}

func (d *Dag) Order(target string) []*DagNode {
	visited := make(map[string]bool)
	var out []string
	d.topo(target, visited, &out)
	result := make([]*DagNode, 0, len(out))
	for _, name := range out {
		if n, ok := d.Nodes[name]; ok {
			result = append(result, n)
		}
	}
	return result
}

func (d *Dag) topo(name string, visited map[string]bool, out *[]string) {
	if visited[name] {
		return
	}
	visited[name] = true
	if n, ok := d.Nodes[name]; ok {
		for _, prereq := range n.Prereqs {
			d.topo(prereq, visited, out)
		}
	}
	*out = append(*out, name)
}
