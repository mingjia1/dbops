package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type VersionController struct {
	catalog *services.VersionCatalog
}

func NewVersionController(catalog *services.VersionCatalog) *VersionController {
	return &VersionController{catalog: catalog}
}

// List returns the full version catalog. Optional ?flavor= filter.
func (c *VersionController) List(ctx *gin.Context) {
	flavor := ctx.Query("flavor")
	if flavor != "" {
		utils.SuccessResponse(ctx, c.catalog.ByFlavor(flavor))
		return
	}
	utils.SuccessResponse(ctx, c.catalog.List())
}

// GetOne returns a single version entry by id "mysql-8.0.36" or "mysql/8.0.36".
func (c *VersionController) GetOne(ctx *gin.Context) {
	id := ctx.Param("id")
	v, err := c.catalog.Get(id)
	if err != nil {
		utils.NotFoundResponse(ctx, err.Error())
		return
	}
	utils.SuccessResponse(ctx, v)
}

// ListSupported returns only versions available for deployment (active status + usable package URL).
// Used by ClusterDeploy and UpgradeManage to populate version dropdowns.
func (c *VersionController) ListSupported(ctx *gin.Context) {
	flavor := ctx.Query("flavor")
	var entries []services.VersionEntry
	if flavor != "" {
		entries = c.catalog.ByFlavor(flavor)
	} else {
		entries = c.catalog.List()
	}
	out := make([]services.VersionEntry, 0, len(entries))
	for _, e := range entries {
		if e.Status == "active" && e.PackageURL != "" {
			out = append(out, e)
		}
	}
	utils.SuccessResponse(ctx, out)
}

// ValidatePath validates an upgrade path. Body: {source_flavor, source_version, target_flavor, target_version}.
func (c *VersionController) ValidatePath(ctx *gin.Context) {
	var req struct {
		SourceFlavor  string `json:"source_flavor"`
		SourceVersion string `json:"source_version"`
		TargetFlavor  string `json:"target_flavor"`
		TargetVersion string `json:"target_version"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "invalid request")
		return
	}
	// Accept from_version/to_version as aliases
	if req.SourceVersion == "" {
		var alt struct {
			SourceVersion string `json:"from_version"`
			TargetVersion string `json:"to_version"`
		}
		if err := ctx.ShouldBindJSON(&alt); err == nil {
			req.SourceVersion = alt.SourceVersion
			if req.TargetVersion == "" {
				req.TargetVersion = alt.TargetVersion
			}
		}
	}
	if req.SourceVersion == "" || req.TargetVersion == "" {
		utils.BadRequestResponse(ctx, "source_version and target_version are required")
		return
	}
	if req.SourceFlavor == "" {
		req.SourceFlavor = "mysql"
	}
	if req.TargetFlavor == "" {
		req.TargetFlavor = "mysql"
	}
	allowed, reason := services.IsValidUpgradePath(req.SourceFlavor, req.SourceVersion, req.TargetFlavor, req.TargetVersion)
	utils.SuccessResponse(ctx, gin.H{
		"allowed": allowed,
		"reason":  reason,
	})
}
