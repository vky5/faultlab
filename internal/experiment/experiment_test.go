package experiment

import (
	"testing"
)

func TestLoadFileSortsTimelineAndParsesCommands(t *testing.T) {
	exp, err := LoadFile("./testdata_partition_conflict.yaml")
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if exp.Name != "partition_conflict" {
		t.Fatalf("unexpected name: %q", exp.Name)
	}

	if len(exp.Cluster.Members) != 5 {
		t.Fatalf("unexpected cluster members length: %d", len(exp.Cluster.Members))
	}

	if exp.Cluster.ID != "conflict_cluster" {
		t.Fatalf("unexpected cluster id: %q", exp.Cluster.ID)
	}

	if len(exp.Timeline) != 5 {
		t.Fatalf("unexpected timeline length: %d", len(exp.Timeline))
	}

	if got := exp.Timeline[0].At.Duration().String(); got != "0s" {
		t.Fatalf("unexpected first at: %s", got)
	}
	if got := exp.Timeline[1].At.Duration().String(); got != "5s" {
		t.Fatalf("unexpected second at: %s", got)
	}
	if got := exp.Timeline[2].At.Duration().String(); got != "6s" {
		t.Fatalf("unexpected third at: %s", got)
	}
	if got := exp.Timeline[3].At.Duration().String(); got != "6s" {
		t.Fatalf("unexpected fourth at: %s", got)
	}
	if got := exp.Timeline[4].At.Duration().String(); got != "15s" {
		t.Fatalf("unexpected fifth at: %s", got)
	}

	compiled, err := exp.Compile(CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if got := compiled[0].Commands[0]; got != "cp new-cluster conflict_cluster" {
		t.Fatalf("unexpected first compiled command: %q", got)
	}

	if got := compiled[1].Commands[0]; got != "fault-partition conflict_cluster node1 node3 true" {
		t.Fatalf("unexpected partition command: %q", got)
	}

	if got := compiled[2].Commands[0]; got != "kv-put conflict_cluster node1 x 1" {
		t.Fatalf("unexpected write command: %q", got)
	}

	if got := compiled[3].Commands[0]; got != "kv-put conflict_cluster node3 x 2" {
		t.Fatalf("unexpected write command: %q", got)
	}

	if got := compiled[4].Commands[0]; got != "fault-partition conflict_cluster node1 node3 false" {
		t.Fatalf("unexpected heal command: %q", got)
	}
}

func TestCompileUsesMemberIDsAndPortsFromYAML(t *testing.T) {
	exp, err := LoadFile("./testdata_partition_conflict_members.yaml")
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if len(exp.Cluster.Members) != 3 {
		t.Fatalf("unexpected cluster members length: %d", len(exp.Cluster.Members))
	}

	if exp.Cluster.ID != "conflict_members_cluster" {
		t.Fatalf("unexpected cluster id: %q", exp.Cluster.ID)
	}

	compiled, err := exp.Compile(CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if got := compiled[0].Commands[1]; got != "start-node alpha 8101 --cluster-id conflict_members_cluster --host localhost --peers beta:8102,gamma:8103 --cp-host localhost --cp-port 9091" {
		t.Fatalf("unexpected start-node command: %q", got)
	}

	if got := compiled[1].Commands[0]; got != "fault-partition conflict_members_cluster alpha beta true" {
		t.Fatalf("unexpected partition command: %q", got)
	}

	if got := compiled[2].Commands[0]; got != "kv-put conflict_members_cluster alpha x 1" {
		t.Fatalf("unexpected write command: %q", got)
	}

	if got := compiled[3].Commands[0]; got != "fault-partition conflict_members_cluster alpha beta false" {
		t.Fatalf("unexpected heal command: %q", got)
	}
}
