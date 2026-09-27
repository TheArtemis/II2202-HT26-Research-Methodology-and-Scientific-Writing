package netinfra

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed specs/*.yaml
var specsFS embed.FS

// TopologySpec is the declarative YAML topology schema.
type TopologySpec struct {
	Name             string            `yaml:"name" json:"name"`
	Nodes            []string          `yaml:"nodes" json:"nodes"`
	IdentityCIDR     string            `yaml:"identity_cidr" json:"identity_cidr"`
	InitialLeader    string            `yaml:"initial_leader" json:"initial_leader"`
	Links            []LinkSpec        `yaml:"links" json:"links"`
	ForwardingRoutes []ForwardingRoute `yaml:"forwarding_routes" json:"forwarding_routes"`
}

// LinkSpec describes one bidirectional host–host link.
type LinkSpec struct {
	Endpoints []string `yaml:"endpoints" json:"endpoints"`
	Failed    bool     `yaml:"failed" json:"failed"`
}

// ForwardingRoute is a static multi-hop route installed only in Forwarding mode.
type ForwardingRoute struct {
	Node string `yaml:"node" json:"node"`
	Dest string `yaml:"dest" json:"dest"`
	Via  string `yaml:"via" json:"via"`
}

// LoadSpecFile reads and validates a topology YAML file from disk.
func LoadSpecFile(path string) (TopologySpec, error) {
	f, err := os.Open(path)
	if err != nil {
		return TopologySpec{}, err
	}
	defer f.Close()
	return LoadSpec(f)
}

// LoadSpec reads and validates a topology YAML document.
func LoadSpec(r io.Reader) (TopologySpec, error) {
	var spec TopologySpec
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&spec); err != nil {
		return TopologySpec{}, fmt.Errorf("decode topology yaml: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return TopologySpec{}, err
	}
	return spec, nil
}

// LoadNamedSpec loads an embedded specs/<name>.yaml (name like "t4" or "T4").
func LoadNamedSpec(name string) (TopologySpec, error) {
	base := strings.ToLower(strings.TrimSpace(name))
	base = strings.TrimSuffix(base, ".yaml")
	base = strings.TrimSuffix(base, ".yml")
	path := filepath.ToSlash(filepath.Join("specs", base+".yaml"))
	f, err := specsFS.Open(path)
	if err != nil {
		return TopologySpec{}, fmt.Errorf("unknown topology %q (embedded %s): %w", name, path, err)
	}
	defer f.Close()
	return LoadSpec(f)
}

// ListEmbeddedSpecs returns sorted topology names (without .yaml).
func ListEmbeddedSpecs() ([]string, error) {
	entries, err := specsFS.ReadDir("specs")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, ".yml") {
			names = append(names, strings.TrimSuffix(strings.TrimSuffix(n, ".yaml"), ".yml"))
		}
	}
	sort.Strings(names)
	return names, nil
}

// Validate checks structural consistency of the topology.
func (s TopologySpec) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("topology name is required")
	}
	if len(s.Nodes) == 0 {
		return fmt.Errorf("topology %s: nodes must be non-empty", s.Name)
	}
	if s.IdentityCIDR == "" {
		return fmt.Errorf("topology %s: identity_cidr is required", s.Name)
	}
	nodeSet := make(map[string]struct{}, len(s.Nodes))
	for _, n := range s.Nodes {
		if n == "" {
			return fmt.Errorf("topology %s: empty node name", s.Name)
		}
		if _, ok := nodeSet[n]; ok {
			return fmt.Errorf("topology %s: duplicate node %q", s.Name, n)
		}
		nodeSet[n] = struct{}{}
	}
	if s.InitialLeader != "" {
		if _, ok := nodeSet[s.InitialLeader]; !ok {
			return fmt.Errorf("topology %s: initial_leader %q not in nodes", s.Name, s.InitialLeader)
		}
	}
	seenLinks := make(map[string]struct{})
	for i, link := range s.Links {
		if len(link.Endpoints) != 2 {
			return fmt.Errorf("topology %s: link[%d] must have exactly 2 endpoints", s.Name, i)
		}
		a, b := link.Endpoints[0], link.Endpoints[1]
		if a == b {
			return fmt.Errorf("topology %s: link[%d] endpoints must differ", s.Name, i)
		}
		if _, ok := nodeSet[a]; !ok {
			return fmt.Errorf("topology %s: link[%d] unknown endpoint %q", s.Name, i, a)
		}
		if _, ok := nodeSet[b]; !ok {
			return fmt.Errorf("topology %s: link[%d] unknown endpoint %q", s.Name, i, b)
		}
		key := linkKey(a, b)
		if _, ok := seenLinks[key]; ok {
			return fmt.Errorf("topology %s: duplicate link %s", s.Name, key)
		}
		seenLinks[key] = struct{}{}
	}
	for i, r := range s.ForwardingRoutes {
		for _, name := range []string{r.Node, r.Dest, r.Via} {
			if _, ok := nodeSet[name]; !ok {
				return fmt.Errorf("topology %s: forwarding_routes[%d] unknown node %q", s.Name, i, name)
			}
		}
		if r.Node == r.Dest || r.Node == r.Via || r.Dest == r.Via {
			return fmt.Errorf("topology %s: forwarding_routes[%d] node/dest/via must be distinct", s.Name, i)
		}
	}
	return nil
}

// PlannedFailures returns endpoint pairs marked failed in the YAML.
func (s TopologySpec) PlannedFailures() [][2]string {
	var out [][2]string
	for _, link := range s.Links {
		if link.Failed {
			out = append(out, [2]string{link.Endpoints[0], link.Endpoints[1]})
		}
	}
	return out
}

// RemapLeader returns a copy of the topology with the YAML initial_leader role
// swapped onto actualLeader. Link failure marks and forwarding_routes follow the
// swap so inject cuts the same relative pattern around whoever was elected.
func (s TopologySpec) RemapLeader(actualLeader string) TopologySpec {
	out := s.clone()
	planned := strings.TrimSpace(s.InitialLeader)
	actual := strings.TrimSpace(actualLeader)
	if planned == "" || actual == "" || planned == actual {
		if actual != "" {
			out.InitialLeader = actual
		}
		return out
	}
	rename := func(id string) string {
		switch id {
		case planned:
			return actual
		case actual:
			return planned
		default:
			return id
		}
	}
	for i := range out.Links {
		out.Links[i].Endpoints[0] = rename(out.Links[i].Endpoints[0])
		out.Links[i].Endpoints[1] = rename(out.Links[i].Endpoints[1])
	}
	for i := range out.ForwardingRoutes {
		out.ForwardingRoutes[i].Node = rename(out.ForwardingRoutes[i].Node)
		out.ForwardingRoutes[i].Dest = rename(out.ForwardingRoutes[i].Dest)
		out.ForwardingRoutes[i].Via = rename(out.ForwardingRoutes[i].Via)
	}
	out.InitialLeader = actual
	return out
}

func (s TopologySpec) clone() TopologySpec {
	out := s
	out.Nodes = append([]string(nil), s.Nodes...)
	out.Links = append([]LinkSpec(nil), s.Links...)
	for i := range out.Links {
		out.Links[i].Endpoints = append([]string(nil), s.Links[i].Endpoints...)
	}
	out.ForwardingRoutes = append([]ForwardingRoute(nil), s.ForwardingRoutes...)
	return out
}

// IdentityIP returns the stable Raft address for node (A→.1, B→.2, … in Nodes order).
func (s TopologySpec) IdentityIP(node string) (string, error) {
	idx := -1
	for i, n := range s.Nodes {
		if n == node {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("unknown node %q", node)
	}
	// identity_cidr like 10.0.0.0/24 → host .1 + idx
	base, err := parseIPv4Base(s.IdentityCIDR)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.%d.%d", base[0], base[1], base[2], base[3]+byte(idx+1)), nil
}

// IdentityMap returns node → identity IP for all nodes.
func (s TopologySpec) IdentityMap() (map[string]string, error) {
	out := make(map[string]string, len(s.Nodes))
	for _, n := range s.Nodes {
		ip, err := s.IdentityIP(n)
		if err != nil {
			return nil, err
		}
		out[n] = ip
	}
	return out, nil
}

func linkKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "--" + b
}

func parseIPv4Base(cidr string) ([4]byte, error) {
	var zero [4]byte
	host := cidr
	if i := strings.IndexByte(cidr, '/'); i >= 0 {
		host = cidr[:i]
	}
	var a, b, c, d int
	n, err := fmt.Sscanf(host, "%d.%d.%d.%d", &a, &b, &c, &d)
	if err != nil || n != 4 || a < 0 || a > 255 || b < 0 || b > 255 || c < 0 || c > 255 || d < 0 || d > 255 {
		return zero, fmt.Errorf("invalid identity_cidr %q", cidr)
	}
	return [4]byte{byte(a), byte(b), byte(c), byte(d)}, nil
}
