package executor

import (
	"strings"
	"testing"
)

func TestBuildPXCConfigContentOmitsRemovedSSTAuthOption(t *testing.T) {
	config := PXCConfig{
		ClusterName:   "pxc-real-test",
		Nodes:         []string{"10.1.81.21", "10.1.81.22", "10.1.81.32"},
		MySQLPort:     24410,
		WSREPPort:     4569,
		SSTMethod:     "xtrabackup-v2",
		ReplicateUser: "sstuser",
		ReplicatePass: "secret",
		DataDir:       "/data/mysql/pxc-24410",
		NodeHost:      "10.1.81.21",
	}

	content := buildPXCConfigContent(config)

	if strings.Contains(content, "wsrep_sst_auth") {
		t.Fatalf("PXC 8.0.45 rejects wsrep_sst_auth; config should omit it:\n%s", content)
	}
	for _, want := range []string{
		"wsrep_cluster_name=pxc-real-test",
		"wsrep_cluster_address=gcomm://10.1.81.21:4569,10.1.81.22:4569,10.1.81.32:4569",
		"wsrep_node_address=10.1.81.21",
		"wsrep_sst_method=xtrabackup-v2",
		"pxc_encrypt_cluster_traffic=OFF",
		"wsrep_provider_options=gmcast.listen_addr=tcp://0.0.0.0:4569;socket.ssl=NO",
		"[sst]\nencrypt=0",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected config to contain %q:\n%s", want, content)
		}
	}
}
