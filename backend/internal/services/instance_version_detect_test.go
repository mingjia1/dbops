package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShortVersionFromBanner(t *testing.T) {
	tests := []struct {
		name   string
		banner string
		want   string
	}{
		{
			name:   "openGauss banner",
			banner: "PostgreSQL 9.2.4 (openGauss 5.0.0 build 123) on x86_64-unknown-linux-gnu",
			want:   "9.2.4",
		},
		{
			name:   "plain postgres banner",
			banner: "PostgreSQL 14.5 on x86_64-pc-linux-gnu, compiled by gcc",
			want:   "14.5",
		},
		{
			name:   "kingbase banner has no dotted numeric token",
			banner: "KingbaseES V008R006C008B0014 on x86_64-pc-linux-gnu",
			want:   "KingbaseES V008R006C008B0014 on x86_64-pc-linux-gnu",
		},
		{
			name:   "empty banner",
			banner: "",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shortVersionFromBanner(tt.banner))
		})
	}
}

func TestDetectVersionRefusesDriverlessFlavors(t *testing.T) {
	// Dameng DM and GBase 8s have no Go driver. DetectVersion must say so plainly
	// instead of dispatching MySQL syntax to the agent.
	for _, flavor := range driverlessFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			hostRepo := repositories.NewHostRepository(db)
			instanceRepo := repositories.NewInstanceRepository(db)
			taskRepo := repositories.NewTaskRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-detect-"+flavor, "", flavor)

			// nil agent client: reaching the agent path would fail differently.
			service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
			version, err := service.DetectVersion(ctx, "inst-detect-"+flavor)

			require.Error(t, err)
			assert.Nil(t, version)
			assert.Contains(t, err.Error(), flavor)
			assert.Contains(t, err.Error(), "不支持自动版本检测")
		})
	}
}

func TestDetectVersionUsesConnectorForPostgresCompatibleFlavors(t *testing.T) {
	// PG-compatible engines must go through the connector, not the agent. With a
	// nil agent client, reaching the agent path would report "agent client not
	// configured"; a connection error instead proves the connector path was taken.
	for _, flavor := range []string{"kingbase", "opengauss", "highgo", "gbase8a", "shentong"} {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			hostRepo := repositories.NewHostRepository(db)
			instanceRepo := repositories.NewInstanceRepository(db)
			taskRepo := repositories.NewTaskRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-detect-"+flavor, "", flavor)

			service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
			// Point at a closed local port so the connect fails immediately instead
			// of burning the full connect timeout.
			conn, connErr := instanceRepo.GetConnection(ctx, "inst-detect-"+flavor)
			require.NoError(t, connErr)
			conn.Host = "127.0.0.1"
			conn.Port = 1
			require.NoError(t, instanceRepo.UpdateConnection(ctx, conn))

			version, err := service.DetectVersion(ctx, "inst-detect-"+flavor)

			require.Error(t, err)
			assert.Nil(t, version)
			assert.NotContains(t, err.Error(), "agent client not configured")
			assert.Contains(t, err.Error(), "version detect for "+flavor+" failed")
		})
	}
}

func TestDetectVersionKeepsAgentPathForMySQLFlavors(t *testing.T) {
	// MySQL-protocol engines must keep the existing agent-based detection. The nil
	// agent client is the marker: this error can only come from the agent path.
	for _, flavor := range []string{"mysql", "mariadb", "percona", "oceanbase", ""} {
		name := flavor
		if name == "" {
			name = "empty-legacy-flavor"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			hostRepo := repositories.NewHostRepository(db)
			instanceRepo := repositories.NewInstanceRepository(db)
			taskRepo := repositories.NewTaskRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-detect-mysqlish", "", flavor)

			service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
			version, err := service.DetectVersion(ctx, "inst-detect-mysqlish")

			require.Error(t, err)
			assert.Nil(t, version)
			assert.Contains(t, err.Error(), "agent client not configured")
		})
	}
}
