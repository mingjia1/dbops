package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
)

type TopologyEventPublisher struct {
	repo *repositories.TopologyEventRepository
	bus  *MessageBus
}

func NewTopologyEventPublisher(repo *repositories.TopologyEventRepository, bus *MessageBus) *TopologyEventPublisher {
	return &TopologyEventPublisher{repo: repo, bus: bus}
}

func (p *TopologyEventPublisher) PublishTopologyChange(ctx context.Context, clusterID, eventType, oldMasterID, newMasterID, nodeID, details string) error {
	event := &models.TopologyEvent{
		ID:          uuid.New().String(),
		ClusterID:   clusterID,
		EventType:   eventType,
		OldMasterID: oldMasterID,
		NewMasterID: newMasterID,
		NodeID:      nodeID,
		Details:     details,
		CreatedAt:   time.Now(),
	}

	if p.repo != nil {
		if err := p.repo.Create(ctx, event); err != nil {
			return fmt.Errorf("persist topology event: %w", err)
		}
	}

	if p.bus != nil {
		p.bus.Publish(TaskEvent{
			TaskID:    fmt.Sprintf("topology-%s", clusterID),
			EventType: "topology_change",
			Stage:     eventType,
			Status:    "event",
			Metadata: map[string]string{
				"event_type":    eventType,
				"old_master_id": oldMasterID,
				"new_master_id": newMasterID,
				"node_id":       nodeID,
				"details":       details,
			},
		})
	}

	return nil
}

func (p *TopologyEventPublisher) PublishFailover(ctx context.Context, clusterID, oldMasterID, newMasterID string) error {
	return p.PublishTopologyChange(ctx, clusterID, "failover", oldMasterID, newMasterID, newMasterID,
		fmt.Sprintf("Failover from %s to %s", oldMasterID, newMasterID))
}

func (p *TopologyEventPublisher) PublishRoleSwitch(ctx context.Context, clusterID, oldMasterID, newMasterID string) error {
	return p.PublishTopologyChange(ctx, clusterID, "role_switch", oldMasterID, newMasterID, newMasterID,
		fmt.Sprintf("Role switch from %s to %s", oldMasterID, newMasterID))
}

func (p *TopologyEventPublisher) PublishNodeJoin(ctx context.Context, clusterID, nodeID string) error {
	return p.PublishTopologyChange(ctx, clusterID, "node_join", "", "", nodeID,
		fmt.Sprintf("Node %s joined cluster", nodeID))
}

func (p *TopologyEventPublisher) PublishNodeLeave(ctx context.Context, clusterID, nodeID string) error {
	return p.PublishTopologyChange(ctx, clusterID, "node_leave", "", "", nodeID,
		fmt.Sprintf("Node %s left cluster", nodeID))
}

func (p *TopologyEventPublisher) GetHistory(ctx context.Context, clusterID string, limit int) ([]models.TopologyEvent, error) {
	if p.repo == nil {
		return nil, fmt.Errorf("topology event repository not configured")
	}
	return p.repo.ListByCluster(ctx, clusterID, limit)
}
