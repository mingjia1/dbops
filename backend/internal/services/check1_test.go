package services

import (
	"context"
	"testing"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTopoPublisherRepo() *repositories.TopologyEventRepository {
	return repositories.NewTopologyEventRepository(newTestDB())
}

func TestTopologyEventPublisher_PublishTopologyChange(t *testing.T) {
	repo := newTestTopoPublisherRepo()
	bus := NewMessageBus()
	pub := NewTopologyEventPublisher(repo, bus)

	ch := bus.Subscribe("topology-tpub-001")
	defer bus.Unsubscribe("topology-tpub-001", ch)

	err := pub.PublishTopologyChange(context.Background(), "tpub-001", "failover", "old-001", "new-001", "new-001", "test failover")
	require.NoError(t, err)

	select {
	case event := <-ch:
		assert.Equal(t, "topology_change", event.EventType)
		assert.Equal(t, "failover", event.Stage)
		assert.Equal(t, "old-001", event.Metadata["old_master_id"])
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for topology event")
	}

	events, err := repo.ListByCluster(context.Background(), "tpub-001", 10)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "failover", events[0].EventType)
}

func TestTopologyEventPublisher_PublishFailover(t *testing.T) {
	repo := newTestTopoPublisherRepo()
	pub := NewTopologyEventPublisher(repo, nil)

	err := pub.PublishFailover(context.Background(), "tpub-002", "master-001", "master-002")
	require.NoError(t, err)

	events, err := repo.ListByCluster(context.Background(), "tpub-002", 10)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "failover", events[0].EventType)
}

func TestTopologyEventPublisher_PublishRoleSwitch(t *testing.T) {
	repo := newTestTopoPublisherRepo()
	pub := NewTopologyEventPublisher(repo, nil)

	err := pub.PublishRoleSwitch(context.Background(), "tpub-003", "old-master", "new-master")
	require.NoError(t, err)
}

func TestTopologyEventPublisher_PublishNodeJoin(t *testing.T) {
	repo := newTestTopoPublisherRepo()
	pub := NewTopologyEventPublisher(repo, nil)

	err := pub.PublishNodeJoin(context.Background(), "tpub-004", "node-005")
	require.NoError(t, err)

	events, err := repo.ListByCluster(context.Background(), "tpub-004", 10)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "node_join", events[0].EventType)
}

func TestTopologyEventPublisher_PublishNodeLeave(t *testing.T) {
	repo := newTestTopoPublisherRepo()
	pub := NewTopologyEventPublisher(repo, nil)

	err := pub.PublishNodeLeave(context.Background(), "tpub-005", "node-003")
	require.NoError(t, err)
}

func TestTopologyEventPublisher_GetHistory(t *testing.T) {
	repo := newTestTopoPublisherRepo()
	pub := NewTopologyEventPublisher(repo, nil)

	_ = pub.PublishNodeJoin(context.Background(), "tpub-006", "n1")
	_ = pub.PublishNodeJoin(context.Background(), "tpub-006", "n2")
	_ = pub.PublishNodeLeave(context.Background(), "tpub-006", "n1")

	events, err := pub.GetHistory(context.Background(), "tpub-006", 10)
	require.NoError(t, err)
	assert.Len(t, events, 3)
}

func TestTopologyEventPublisher_NilRepo(t *testing.T) {
	pub := NewTopologyEventPublisher(nil, nil)
	err := pub.PublishTopologyChange(context.Background(), "tpub-nil", "type", "", "", "", "")
	assert.NoError(t, err, "nil repo should not error, just skip DB persistence")
}

func TestTopologyEventPublisher_GetHistory_NilRepo(t *testing.T) {
	pub := NewTopologyEventPublisher(nil, nil)
	_, err := pub.GetHistory(context.Background(), "tpub-nil2", 10)
	assert.Error(t, err)
}

func TestRedisPubSubBackend_InMemory(t *testing.T) {
	r := NewRedisPubSubBackend("")
	assert.False(t, r.IsEnabled())

	ch := r.Subscribe("task-001")
	r.Publish(TaskEvent{TaskID: "task-001", EventType: "progress", Progress: 50})

	select {
	case event := <-ch:
		assert.Equal(t, 50, event.Progress)
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	r.Unsubscribe("task-001", ch)
	assert.Equal(t, 0, r.SubscriberCount("task-001"))
	r.Close()
}

func TestRedisPubSubBackend_Enabled(t *testing.T) {
	r := NewRedisPubSubBackend("localhost:6379")
	assert.True(t, r.IsEnabled())
	r.Close()
}

func TestRebuildService_New(t *testing.T) {
	vault := NewCredentialVault(newTestCredentialRepo(), "test-key")
	svc := NewRebuildService(nil, vault, nil)
	assert.NotNil(t, svc)
}

func TestRebuildService_RebuildNode_MissingKey(t *testing.T) {
	vault := NewCredentialVault(newTestCredentialRepo(), "test-key")
	svc := NewRebuildService(nil, vault, nil)
	_, err := svc.RebuildNode(context.Background(), RebuildServiceRequest{
		ClusterID: "c", Flavor: "mysql",
	})
	assert.Error(t, err)
}

func mustCreateCredential(t *testing.T, vault *CredentialVault, clusterID, accountType, user, pass string) {
	t.Helper()
	ctx := context.Background()
	err := vault.SetCredential(ctx, clusterID, accountType, user, pass)
	require.NoError(t, err)
}
