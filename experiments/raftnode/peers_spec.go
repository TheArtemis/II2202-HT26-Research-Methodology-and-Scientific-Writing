package raftnode

import (
	"fmt"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/netinfra"
)

// PeersFromSpec returns node→identity-IP for Raft --peers flags.
func PeersFromSpec(spec netinfra.TopologySpec) (map[string]string, error) {
	return spec.IdentityMap()
}

// APIURLsFromSpec returns peer IDs and HTTP Apply base URLs in Nodes order.
func APIURLsFromSpec(spec netinfra.TopologySpec, apiPort int) (ids, urls []string, err error) {
	if apiPort == 0 {
		apiPort = DefaultAPIPort
	}
	ids = make([]string, 0, len(spec.Nodes))
	urls = make([]string, 0, len(spec.Nodes))
	for _, n := range spec.Nodes {
		ip, err := spec.IdentityIP(n)
		if err != nil {
			return nil, nil, err
		}
		ids = append(ids, n)
		urls = append(urls, fmt.Sprintf("http://%s:%d", ip, apiPort))
	}
	return ids, urls, nil
}
