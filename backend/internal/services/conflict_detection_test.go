package services

import (
	"testing"
)

func TestFilterActiveEndpoints(t *testing.T) {
	tests := []struct {
		name           string
		endpoints      []instanceEndpoint
		activeClusters map[string]bool
		expected       int
		desc           string
	}{
		{
			name:           "empty endpoints",
			endpoints:      []instanceEndpoint{},
			activeClusters: map[string]bool{},
			expected:       0,
			desc:           "no endpoints returns empty",
		},
		{
			name:           "empty activeClusters returns all endpoints",
			activeClusters: map[string]bool{},
			endpoints: []instanceEndpoint{
				{clusterID: "cluster-1"},
				{clusterID: "cluster-2"},
				{clusterID: ""},
			},
			expected: 3,
			desc:     "when len(activeClusters)==0, keep all endpoints (safe default)",
		},
		{
			name: "filters out inactive cluster endpoints",
			activeClusters: map[string]bool{
				"cluster-1": true,
			},
			endpoints: []instanceEndpoint{
				{clusterID: "cluster-1", host: "10.0.0.1", port: 3306, instName: "inst-1"},
				{clusterID: "cluster-2", host: "10.0.0.2", port: 3306, instName: "inst-2"},
				{clusterID: "cluster-3", host: "10.0.0.3", port: 3306, instName: "inst-3"},
			},
			expected: 1,
			desc:     "only cluster-1 should remain",
		},
		{
			name: "keeps endpoints with empty clusterID",
			activeClusters: map[string]bool{
				"cluster-1": true,
			},
			endpoints: []instanceEndpoint{
				{clusterID: "cluster-1", host: "10.0.0.1", port: 3306, instName: "inst-1"},
				{clusterID: "", host: "10.0.0.4", port: 3306, instName: "orphan"},
			},
			expected: 2,
			desc:     "orphan instances (no clusterID) are always kept",
		},
		{
			name: "multiple active clusters",
			activeClusters: map[string]bool{
				"cluster-a": true,
				"cluster-c": true,
			},
			endpoints: []instanceEndpoint{
				{clusterID: "cluster-a", host: "10.0.0.1", port: 3306},
				{clusterID: "cluster-b", host: "10.0.0.2", port: 3306},
				{clusterID: "cluster-c", host: "10.0.0.3", port: 3306},
				{clusterID: "cluster-d", host: "10.0.0.4", port: 3306},
			},
			expected: 2,
			desc:     "only cluster-a and cluster-c remain",
		},
	}

	svc := &ClusterDeployService{} // filterActiveEndpoints doesn't use any fields

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.filterActiveEndpoints(tt.endpoints, tt.activeClusters)
			if len(got) != tt.expected {
				t.Errorf("filterActiveEndpoints returned %d endpoints, want %d\n  input: %+v\n  active: %v\n  desc: %s",
					len(got), tt.expected, tt.endpoints, tt.activeClusters, tt.desc)
			}
			// When activeClusters is empty, all endpoints are kept (safe default).
			// When non-empty, verify no inactive-cluster endpoints leak through.
			if len(tt.activeClusters) > 0 {
				for _, ep := range got {
					if ep.clusterID != "" && !tt.activeClusters[ep.clusterID] {
						t.Errorf("filterActiveEndpoints returned endpoint with clusterID=%q which is not in activeClusters", ep.clusterID)
					}
				}
			}
		})
	}
}

func TestFindManagedPortConflictInEndpoints(t *testing.T) {
	endpoints := []instanceEndpoint{
		{host: "10.0.0.1", port: 3306, instName: "inst-master", clusterID: "cluster-active"},
		{host: "10.0.0.2", port: 3306, instName: "inst-replica", clusterID: "cluster-active"},
		{host: "10.0.0.1", port: 3307, instName: "inst-old", clusterID: "cluster-destroyed"},
		{host: "10.0.0.3", port: 3306, instName: "inst-orphan", clusterID: ""},
	}

	activeClusters := map[string]bool{
		"cluster-active": true,
	}

	tests := []struct {
		name             string
		host             string
		port             int
		currentClusterID string
		endpoints        []instanceEndpoint
		activeClusters   map[string]bool
		wantConflict     bool
	}{
		{
			name:           "port conflict with active cluster",
			host:           "10.0.0.1",
			port:           3306,
			endpoints:      endpoints,
			activeClusters: activeClusters,
			wantConflict:   true,
		},
		{
			name:             "same cluster ID — no conflict",
			host:             "10.0.0.1",
			port:             3306,
			currentClusterID: "cluster-active",
			endpoints:        endpoints,
			activeClusters:   activeClusters,
			wantConflict:     false,
		},
		{
			name:           "port from destroyed cluster — no conflict",
			host:           "10.0.0.1",
			port:           3307,
			endpoints:      endpoints,
			activeClusters: activeClusters,
			wantConflict:   false,
		},
		{
			name:           "orphan instance — no conflict (empty clusterID)",
			host:           "10.0.0.3",
			port:           3306,
			endpoints:      endpoints,
			activeClusters: activeClusters,
			wantConflict:   false,
		},
		{
			name:           "no matching host — no conflict",
			host:           "10.0.0.99",
			port:           3306,
			endpoints:      endpoints,
			activeClusters: activeClusters,
			wantConflict:   false,
		},
		{
			name:           "port not matching — no conflict",
			host:           "10.0.0.1",
			port:           3400,
			endpoints:      endpoints,
			activeClusters: activeClusters,
			wantConflict:   false,
		},
		{
			name:           "empty activeClusters skips all cluster endpoints (no conflict)",
			host:           "10.0.0.1",
			port:           3306,
			endpoints:      endpoints,
			activeClusters: map[string]bool{},
			wantConflict:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findManagedPortConflictInEndpoints(tt.host, tt.port, tt.currentClusterID, tt.endpoints, tt.activeClusters)
			if tt.wantConflict && got == "" {
				t.Errorf("expected conflict but got none (host=%s, port=%d, currentCluster=%q)", tt.host, tt.port, tt.currentClusterID)
			}
			if !tt.wantConflict && got != "" {
				t.Errorf("unexpected conflict: %s (host=%s, port=%d, currentCluster=%q)", got, tt.host, tt.port, tt.currentClusterID)
			}
		})
	}
}

func TestFilterActiveEndpoints_SameHostDifferentPort(t *testing.T) {
	svc := &ClusterDeployService{}
	endpoints := []instanceEndpoint{
		{host: "10.0.0.1", port: 3306, instName: "mysql-3306-active", clusterID: "cluster-a"},
		{host: "10.0.0.1", port: 3307, instName: "mysql-3307-destroyed", clusterID: "cluster-dead"},
		{host: "10.0.0.1", port: 3308, instName: "mysql-3308-orphan", clusterID: ""},
	}
	active := map[string]bool{"cluster-a": true}

	got := svc.filterActiveEndpoints(endpoints, active)
	if len(got) != 2 {
		t.Errorf("expected 2 endpoints (cluster-a + orphan), got %d", len(got))
	}
	for _, ep := range got {
		if ep.clusterID == "cluster-dead" {
			t.Errorf("cluster-dead should have been filtered out, but got %+v", ep)
		}
	}
}

func TestFindManagedPortConflict_SameHostDifferentPort(t *testing.T) {
	// Confirm that only an exact host+port match triggers a conflict.
	endpoints := []instanceEndpoint{
		{host: "10.0.0.1", port: 3306, instName: "mysql-3306", clusterID: "cluster-a"},
		{host: "10.0.0.1", port: 3307, instName: "mysql-3307", clusterID: "cluster-a"},
	}
	active := map[string]bool{"cluster-a": true}

	// 3306 on 10.0.0.1 should conflict
	if got := findManagedPortConflictInEndpoints("10.0.0.1", 3306, "", endpoints, active); got == "" {
		t.Error("expected conflict for 10.0.0.1:3306 but got none")
	}
	// 3308 (no match) should not conflict
	if got := findManagedPortConflictInEndpoints("10.0.0.1", 3308, "", endpoints, active); got != "" {
		t.Errorf("unexpected conflict for 10.0.0.1:3308: %s", got)
	}
}

func TestFilterActiveEndpoints_NilActiveClusters(t *testing.T) {
	// nil activeClusters should be treated like empty (len == 0 → return all)
	svc := &ClusterDeployService{}
	endpoints := []instanceEndpoint{
		{clusterID: "cluster-1"},
		{clusterID: ""},
	}
	got := svc.filterActiveEndpoints(endpoints, nil)
	if len(got) != 2 {
		t.Errorf("nil activeClusters should return all endpoints, got %d", len(got))
	}
}

func TestFindManagedPortConflictInEndpoints_NilActiveClusters(t *testing.T) {
	// nil activeClusters: accessing nil map[key] returns zero value (false),
	// so all non-orphan endpoints are skipped — no conflict.
	endpoints := []instanceEndpoint{
		{host: "10.0.0.1", port: 3306, instName: "inst", clusterID: "cluster-a"},
	}
	if got := findManagedPortConflictInEndpoints("10.0.0.1", 3306, "", endpoints, nil); got != "" {
		t.Errorf("nil activeClusters should skip all cluster endpoints, got conflict: %s", got)
	}
}
