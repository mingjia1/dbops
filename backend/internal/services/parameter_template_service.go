package services

import (
	"context"
	"fmt"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"strconv"
	"strings"
	"time"
)

type ParameterTemplateService struct {
	repo            *repositories.ParameterTemplateRepository
	instanceService *InstanceService
}

func NewParameterTemplateService(repo *repositories.ParameterTemplateRepository, instanceService ...*InstanceService) *ParameterTemplateService {
	s := &ParameterTemplateService{repo: repo}
	if len(instanceService) > 0 {
		s.instanceService = instanceService[0]
	}
	return s
}

func (s *ParameterTemplateService) Create(ctx context.Context, req CreateParameterTemplateRequest) (*models.ParameterTemplate, error) {
	template := &models.ParameterTemplate{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		IsPreset:    false,
		CreatedBy:   req.CreatedBy,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to create parameter template: %w", err)
	}

	for _, param := range req.Parameters {
		p := &models.ParameterTemplateParameter{
			TemplateID:    template.ID,
			VersionID:     param.VersionID,
			ParameterName: param.ParameterName,
			Value:         param.Value,
			DataType:      param.DataType,
			MinValue:      param.MinValue,
			MaxValue:      param.MaxValue,
			Unit:          param.Unit,
			Description:   param.Description,
			IsDynamic:     param.IsDynamic,
			IsMandatory:   param.IsMandatory,
			Category:      param.Category,
		}
		if err := s.repo.CreateParameter(ctx, p); err != nil {
			return nil, fmt.Errorf("failed to create parameter: %w", err)
		}
	}

	return template, nil
}

func (s *ParameterTemplateService) GetByID(ctx context.Context, id string) (*models.ParameterTemplate, error) {
	template, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.hydrateParameters(ctx, template)
}

func (s *ParameterTemplateService) List(ctx context.Context, limit, offset int) ([]models.ParameterTemplate, error) {
	templates, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	for i := range templates {
		if hydrated, err := s.hydrateParameters(ctx, &templates[i]); err == nil {
			templates[i] = *hydrated
		}
	}
	return templates, nil
}

func (s *ParameterTemplateService) ListPresets(ctx context.Context) ([]models.ParameterTemplate, error) {
	templates, err := s.repo.ListPresetTemplates(ctx)
	if err != nil {
		return nil, err
	}
	for i := range templates {
		if hydrated, err := s.hydrateParameters(ctx, &templates[i]); err == nil {
			templates[i] = *hydrated
		}
	}
	return templates, nil
}

// SeedPresets: 启动时幂等写入内置预设模板 (is_preset=1). 原从未 seed,
// 预设列表/参数增强页/验证功能全部为空, 用户第一次打开就是空表.
func (s *ParameterTemplateService) SeedPresets(ctx context.Context) error {
	existing, err := s.repo.ListPresetTemplates(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	now := time.Now()
	for _, pt := range builtinPresetTemplates {
		t := pt.template
		t.IsPreset = true
		t.CreatedAt, t.UpdatedAt = now, now
		if err := s.repo.Create(ctx, &t); err != nil {
			return fmt.Errorf("seed preset %q: %w", t.Name, err)
		}
		for _, param := range pt.params {
			p := param
			p.TemplateID = t.ID
			if err := s.repo.CreateParameter(ctx, &p); err != nil {
				return fmt.Errorf("seed preset %q param %q: %w", t.Name, p.ParameterName, err)
			}
		}
	}
	return nil
}

// builtinPresetTemplates: 三个典型场景的参数模板, 值与 validateSingleParameter
// 的格式/范围/依赖规则兼容 (size 1G/512M, bool on/off, time 1s/3s).
var builtinPresetTemplates = []struct {
	template models.ParameterTemplate
	params   []models.ParameterTemplateParameter
}{
	{
		template: models.ParameterTemplate{
			Name:        "OLTP 高并发生产",
			Category:    "oltp",
			Description: "面向高并发事务场景: 双 1 持久化、大连接数与缓冲池, O_DIRECT 避免双写缓冲.",
		},
		params: []models.ParameterTemplateParameter{
			{ParameterName: "max_connections", Value: "1000", DataType: "int", MinValue: "10", MaxValue: "5000", IsDynamic: true, IsMandatory: true, Category: "connection", Description: "最大并发连接数"},
			{ParameterName: "innodb_buffer_pool_size", Value: "4G", DataType: "size", IsDynamic: false, IsMandatory: true, Category: "memory", Description: "InnoDB 缓冲池大小"},
			{ParameterName: "innodb_buffer_pool_instances", Value: "4", DataType: "int", IsDynamic: false, IsMandatory: true, Category: "memory", Description: "缓冲池实例数 (大于 1G 必须设置)"},
			{ParameterName: "innodb_log_file_size", Value: "1G", DataType: "size", IsDynamic: false, IsMandatory: false, Category: "memory", Description: "重做日志文件大小"},
			{ParameterName: "innodb_flush_log_at_trx_commit", Value: "1", DataType: "int", IsDynamic: true, IsMandatory: true, Category: "durability", Description: "事务提交刷盘策略 (1=每次提交刷盘)"},
			{ParameterName: "sync_binlog", Value: "1", DataType: "int", IsDynamic: true, IsMandatory: true, Category: "durability", Description: "binlog 刷盘策略 (1=每次提交刷盘)"},
			{ParameterName: "slow_query_log", Value: "on", DataType: "bool", IsDynamic: true, IsMandatory: false, Category: "diagnosis", Description: "慢查询日志开关"},
			{ParameterName: "long_query_time", Value: "1s", DataType: "time", IsDynamic: true, IsMandatory: false, Category: "diagnosis", Description: "慢查询阈值"},
			{ParameterName: "character_set_server", Value: "utf8mb4", DataType: "string", IsDynamic: true, IsMandatory: false, Category: "charset", Description: "服务端默认字符集"},
			{ParameterName: "innodb_flush_method", Value: "O_DIRECT", DataType: "string", IsDynamic: false, IsMandatory: false, Category: "io", Description: "InnoDB 刷盘方式"},
		},
	},
	{
		template: models.ParameterTemplate{
			Name:        "报表分析型",
			Category:    "olap",
			Description: "面向复杂查询/报表场景: 更大缓冲池与读线程, 适当放宽持久化与慢查询阈值.",
		},
		params: []models.ParameterTemplateParameter{
			{ParameterName: "max_connections", Value: "300", DataType: "int", MinValue: "10", MaxValue: "2000", IsDynamic: true, IsMandatory: true, Category: "connection", Description: "最大并发连接数"},
			{ParameterName: "innodb_buffer_pool_size", Value: "8G", DataType: "size", IsDynamic: false, IsMandatory: true, Category: "memory", Description: "InnoDB 缓冲池大小"},
			{ParameterName: "innodb_buffer_pool_instances", Value: "8", DataType: "int", IsDynamic: false, IsMandatory: true, Category: "memory", Description: "缓冲池实例数"},
			{ParameterName: "innodb_read_io_threads", Value: "8", DataType: "int", IsDynamic: false, IsMandatory: false, Category: "io", Description: "读 IO 线程数"},
			{ParameterName: "innodb_io_capacity", Value: "1000", DataType: "int", IsDynamic: true, IsMandatory: false, Category: "io", Description: "InnoDB 后台 IO 能力"},
			{ParameterName: "innodb_flush_log_at_trx_commit", Value: "2", DataType: "int", IsDynamic: true, IsMandatory: false, Category: "durability", Description: "事务提交刷盘策略 (2=每秒刷盘, 报表可接受)"},
			{ParameterName: "long_query_time", Value: "3s", DataType: "time", IsDynamic: true, IsMandatory: false, Category: "diagnosis", Description: "慢查询阈值 (报表场景放宽)"},
			{ParameterName: "tmp_table_size", Value: "64M", DataType: "size", IsDynamic: true, IsMandatory: false, Category: "memory", Description: "内存临时表上限"},
		},
	},
	{
		template: models.ParameterTemplate{
			Name:        "开发测试小规格",
			Category:    "dev",
			Description: "面向开发/测试环境: 小内存占用、放宽持久化, 优先节省资源.",
		},
		params: []models.ParameterTemplateParameter{
			{ParameterName: "max_connections", Value: "200", DataType: "int", MinValue: "10", MaxValue: "1000", IsDynamic: true, IsMandatory: true, Category: "connection", Description: "最大并发连接数"},
			{ParameterName: "innodb_buffer_pool_size", Value: "512M", DataType: "size", IsDynamic: false, IsMandatory: true, Category: "memory", Description: "InnoDB 缓冲池大小"},
			{ParameterName: "innodb_flush_log_at_trx_commit", Value: "2", DataType: "int", IsDynamic: true, IsMandatory: false, Category: "durability", Description: "事务提交刷盘策略 (开发环境放宽)"},
			{ParameterName: "sync_binlog", Value: "0", DataType: "int", IsDynamic: true, IsMandatory: false, Category: "durability", Description: "binlog 刷盘策略 (0=依赖操作系统)"},
			{ParameterName: "slow_query_log", Value: "off", DataType: "bool", IsDynamic: true, IsMandatory: false, Category: "diagnosis", Description: "慢查询日志开关"},
			{ParameterName: "long_query_time", Value: "1s", DataType: "time", IsDynamic: true, IsMandatory: false, Category: "diagnosis", Description: "慢查询阈值"},
		},
	},
}

func (s *ParameterTemplateService) Update(ctx context.Context, id string, req UpdateParameterTemplateRequest) (*models.ParameterTemplate, error) {
	template, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if template.IsPreset {
		return nil, fmt.Errorf("cannot modify preset template")
	}

	if req.Name != "" {
		template.Name = req.Name
	}
	if req.Description != "" {
		template.Description = req.Description
	}
	if req.Category != "" {
		template.Category = req.Category
	}
	template.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to update parameter template: %w", err)
	}

	if req.Parameters != nil {
		if err := s.repo.DeleteParametersByTemplate(ctx, id); err != nil {
			return nil, err
		}
		for _, param := range *req.Parameters {
			p := parameterInputToModel(id, param)
			if err := s.repo.CreateParameter(ctx, p); err != nil {
				return nil, fmt.Errorf("failed to create parameter: %w", err)
			}
		}
	}

	return s.hydrateParameters(ctx, template)
}

func (s *ParameterTemplateService) Delete(ctx context.Context, id string) error {
	template, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if template.IsPreset {
		return fmt.Errorf("cannot delete preset template")
	}

	return s.repo.Delete(ctx, id)
}

func (s *ParameterTemplateService) GetParameters(ctx context.Context, templateID string, versionID *string) ([]models.ParameterTemplateParameter, error) {
	return s.repo.ListParameters(ctx, templateID, versionID)
}

func (s *ParameterTemplateService) Apply(ctx context.Context, req ApplyParameterTemplateRequest) (*ApplyParameterTemplateResponse, error) {
	if s.instanceService == nil {
		return nil, fmt.Errorf("instance service is not configured")
	}
	// Templates carry MySQL system variables applied via SET GLOBAL / my.cnf.
	// Refuse engines with a different configuration model.
	if instance, err := s.instanceService.GetByID(ctx, req.InstanceID); err == nil && instance != nil {
		if err := RequireCapability(instance.Version.Flavor, CapParameterTemplate); err != nil {
			return nil, err
		}
	}
	params := req.Parameters
	if len(params) == 0 {
		template, err := s.GetByID(ctx, req.TemplateID)
		if err != nil {
			return nil, err
		}
		for _, p := range template.Parameters {
			params = append(params, ParameterInput{
				ParameterName: p.ParameterName,
				Value:         p.Value,
				DataType:      p.DataType,
				Category:      p.Category,
			})
		}
	}
	if len(params) == 0 {
		return nil, fmt.Errorf("template has no parameters to apply")
	}

	resp := &ApplyParameterTemplateResponse{
		TemplateID: req.TemplateID,
		InstanceID: req.InstanceID,
		Results:    make([]ApplyParameterResult, 0, len(params)),
	}
	for _, p := range params {
		if p.ParameterName == "" {
			continue
		}
		result, err := s.instanceService.AdminAction(ctx, req.InstanceID, InstanceAdminRequest{
			Action: "set_variable",
			Name:   p.ParameterName,
			Value:  p.Value,
		})
		row := ApplyParameterResult{
			Name:   p.ParameterName,
			Value:  p.Value,
			Status: "completed",
		}
		if err != nil {
			row.Status = "failed"
			row.Message = err.Error()
			resp.Failed++
		} else if result != nil {
			row.Status = result.Status
			row.Message = result.Message
			if result.Status == "completed" || result.Status == "success" {
				resp.Applied++
			} else {
				resp.Failed++
			}
		}
		resp.Results = append(resp.Results, row)
	}
	resp.RequireRestart = req.RequireRestart
	return resp, nil
}

func (s *ParameterTemplateService) hydrateParameters(ctx context.Context, template *models.ParameterTemplate) (*models.ParameterTemplate, error) {
	if template == nil {
		return nil, fmt.Errorf("parameter template is nil")
	}
	params, err := s.repo.ListParameters(ctx, template.ID, nil)
	if err != nil {
		return nil, err
	}
	template.Parameters = params
	return template, nil
}

func parameterInputToModel(templateID string, param ParameterInput) *models.ParameterTemplateParameter {
	return &models.ParameterTemplateParameter{
		TemplateID:    templateID,
		VersionID:     param.VersionID,
		ParameterName: param.ParameterName,
		Value:         param.Value,
		DataType:      param.DataType,
		MinValue:      param.MinValue,
		MaxValue:      param.MaxValue,
		Unit:          param.Unit,
		Description:   param.Description,
		IsDynamic:     param.IsDynamic,
		IsMandatory:   param.IsMandatory,
		Category:      param.Category,
	}
}

func (s *ParameterTemplateService) ValidateParameters(ctx context.Context, req ValidateParametersRequest) (*ValidationResult, error) {
	// parameters 未显式提供时回退到模板自身参数 (参数增强页只传 template_id/instance_id).
	if len(req.Parameters) == 0 && strings.TrimSpace(req.TemplateID) != "" {
		template, err := s.GetByID(ctx, req.TemplateID)
		if err != nil {
			return nil, err
		}
		for _, p := range template.Parameters {
			req.Parameters = append(req.Parameters, ParameterValidationInput{
				ParameterName: p.ParameterName,
				Value:         p.Value,
				DataType:      models.ParameterDataType(p.DataType),
				MinValue:      p.MinValue,
				MaxValue:      p.MaxValue,
				IsMandatory:   p.IsMandatory,
			})
		}
	}

	result := &ValidationResult{
		IsValid:  true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
	}

	for _, param := range req.Parameters {
		if err := s.validateSingleParameter(param); err != nil {
			result.IsValid = false
			result.Errors = append(result.Errors, ValidationError{
				ParameterName: param.ParameterName,
				Message:       err.Error(),
			})
		}

		if warning := s.checkParameterWarning(param); warning != "" {
			result.Warnings = append(result.Warnings, ValidationWarning{
				ParameterName: param.ParameterName,
				Message:       warning,
			})
		}
	}

	dependencyErrors := s.checkParameterDependencies(req.Parameters)
	for _, depErr := range dependencyErrors {
		result.IsValid = false
		result.Errors = append(result.Errors, depErr)
	}

	return result, nil
}

func (s *ParameterTemplateService) validateSingleParameter(param ParameterValidationInput) error {
	if param.Value == "" && param.IsMandatory {
		return fmt.Errorf("mandatory parameter '%s' is missing", param.ParameterName)
	}

	switch param.DataType {
	case models.DataTypeInt:
		if _, err := strconv.Atoi(param.Value); err != nil {
			return fmt.Errorf("parameter '%s' must be an integer", param.ParameterName)
		}
		if err := s.checkRangeConstraint(param); err != nil {
			return err
		}
	case models.DataTypeBool:
		if param.Value != "true" && param.Value != "false" && param.Value != "on" && param.Value != "off" {
			return fmt.Errorf("parameter '%s' must be a boolean value", param.ParameterName)
		}
	case models.DataTypeSize:
		if !s.isValidSizeFormat(param.Value) {
			return fmt.Errorf("parameter '%s' must be a valid size format (e.g., 1G, 512M, 1024K)", param.ParameterName)
		}
	case models.DataTypePercent:
		if !s.isValidPercentFormat(param.Value) {
			return fmt.Errorf("parameter '%s' must be a valid percentage (0-100)", param.ParameterName)
		}
	case models.DataTypeTime:
		if !s.isValidTimeFormat(param.Value) {
			return fmt.Errorf("parameter '%s' must be a valid time format (e.g., 1h, 30m, 60s)", param.ParameterName)
		}
	}

	if s.isIllegalValue(param.ParameterName, param.Value) {
		return fmt.Errorf("parameter '%s' has an illegal or dangerous value: %s", param.ParameterName, param.Value)
	}

	return nil
}

func (s *ParameterTemplateService) checkRangeConstraint(param ParameterValidationInput) error {
	if param.MinValue == "" && param.MaxValue == "" {
		return nil
	}

	value, err := strconv.Atoi(param.Value)
	if err != nil {
		return err
	}

	if param.MinValue != "" {
		min, err := strconv.Atoi(param.MinValue)
		if err == nil && value < min {
			return fmt.Errorf("parameter '%s' value %d is less than minimum %d", param.ParameterName, value, min)
		}
	}

	if param.MaxValue != "" {
		max, err := strconv.Atoi(param.MaxValue)
		if err == nil && value > max {
			return fmt.Errorf("parameter '%s' value %d is greater than maximum %d", param.ParameterName, value, max)
		}
	}

	return nil
}

func (s *ParameterTemplateService) isValidSizeFormat(value string) bool {
	value = strings.ToLower(value)
	return strings.HasSuffix(value, "k") || strings.HasSuffix(value, "m") ||
		strings.HasSuffix(value, "g") || strings.HasSuffix(value, "t")
}

func (s *ParameterTemplateService) isValidPercentFormat(value string) bool {
	num, err := strconv.Atoi(strings.TrimSuffix(value, "%"))
	if err != nil {
		return false
	}
	return num >= 0 && num <= 100
}

func (s *ParameterTemplateService) isValidTimeFormat(value string) bool {
	value = strings.ToLower(value)
	return strings.HasSuffix(value, "s") || strings.HasSuffix(value, "m") ||
		strings.HasSuffix(value, "h") || strings.HasSuffix(value, "d")
}

func (s *ParameterTemplateService) isIllegalValue(paramName, value string) bool {
	illegalValues := map[string][]string{
		"max_connections":         {"0", "-1"},
		"innodb_buffer_pool_size": {"0"},
		"innodb_log_file_size":    {"0"},
		"max_allowed_packet":      {"0"},
		"query_cache_size":        {"-1"},
	}

	if illegalList, ok := illegalValues[paramName]; ok {
		for _, illegal := range illegalList {
			if value == illegal {
				return true
			}
		}
	}

	dangerousPatterns := []string{"shell:", "file:", "exec:", "eval(", "system("}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(strings.ToLower(value), pattern) {
			return true
		}
	}

	return false
}

func (s *ParameterTemplateService) checkParameterWarning(param ParameterValidationInput) string {
	warningRules := map[string]func(string) string{
		"innodb_buffer_pool_size": func(v string) string {
			if strings.HasSuffix(strings.ToLower(v), "g") {
				numStr := strings.TrimSuffix(strings.ToLower(v), "g")
				num, err := strconv.Atoi(numStr)
				if err == nil && num > 64 {
					return "InnoDB buffer pool size larger than 64G may require adjusting innodb_buffer_pool_instances"
				}
			}
			return ""
		},
		"max_connections": func(v string) string {
			num, err := strconv.Atoi(v)
			if err == nil && num > 5000 {
				return "max_connections larger than 5000 may cause performance issues"
			}
			return ""
		},
		"open_files_limit": func(v string) string {
			num, err := strconv.Atoi(v)
			if err == nil && num > 100000 {
				return "open_files_limit higher than system ulimit may cause errors"
			}
			return ""
		},
	}

	if warningFunc, ok := warningRules[param.ParameterName]; ok {
		return warningFunc(param.Value)
	}
	return ""
}

func (s *ParameterTemplateService) checkParameterDependencies(params []ParameterValidationInput) []ValidationError {
	errors := []ValidationError{}

	dependencyRules := []struct {
		condition   func([]ParameterValidationInput) bool
		requirement func([]ParameterValidationInput) bool
		message     string
	}{
		{
			condition: func(p []ParameterValidationInput) bool {
				for _, param := range p {
					if param.ParameterName == "log_bin" && param.Value == "on" {
						return true
					}
				}
				return false
			},
			requirement: func(p []ParameterValidationInput) bool {
				for _, param := range p {
					if param.ParameterName == "server_id" && param.Value != "" && param.Value != "0" {
						return true
					}
				}
				return false
			},
			message: "log_bin requires server_id to be set",
		},
		{
			condition: func(p []ParameterValidationInput) bool {
				for _, param := range p {
					if param.ParameterName == "gtid_mode" && param.Value == "on" {
						return true
					}
				}
				return false
			},
			requirement: func(p []ParameterValidationInput) bool {
				for _, param := range p {
					if param.ParameterName == "enforce_gtid_consistency" && param.Value == "on" {
						return true
					}
				}
				return false
			},
			message: "gtid_mode requires enforce_gtid_consistency to be ON",
		},
		{
			condition: func(p []ParameterValidationInput) bool {
				for _, param := range p {
					if param.ParameterName == "innodb_buffer_pool_size" && strings.HasSuffix(strings.ToLower(param.Value), "g") {
						numStr := strings.TrimSuffix(strings.ToLower(param.Value), "g")
						num, err := strconv.Atoi(numStr)
						if err == nil && num > 1 {
							return true
						}
					}
				}
				return false
			},
			requirement: func(p []ParameterValidationInput) bool {
				for _, param := range p {
					if param.ParameterName == "innodb_buffer_pool_instances" && param.Value != "" {
						return true
					}
				}
				return false
			},
			message: "innodb_buffer_pool_size larger than 1G requires setting innodb_buffer_pool_instances",
		},
	}

	for _, rule := range dependencyRules {
		if rule.condition(params) && !rule.requirement(params) {
			errors = append(errors, ValidationError{
				ParameterName: "dependency",
				Message:       rule.message,
			})
		}
	}

	return errors
}

func (s *ParameterTemplateService) RecommendParameters(ctx context.Context, req RecommendParametersRequest) (*RecommendationResult, error) {
	recommendations := []ParameterRecommendation{}

	cpuCores := req.CPUCores
	memoryGB := req.MemoryGB
	diskGB := req.DiskGB
	isOLTP := req.WorkloadType == "oltp"
	isOLAP := req.WorkloadType == "olap"

	bufferPoolPercent := 0.7
	if isOLAP {
		bufferPoolPercent = 0.6
	}

	bufferPoolSize := int(float64(memoryGB) * bufferPoolPercent)
	recommendations = append(recommendations, ParameterRecommendation{
		ParameterName: "innodb_buffer_pool_size",
		Value:         fmt.Sprintf("%dG", bufferPoolSize),
		DataType:      models.DataTypeSize,
		Reason:        fmt.Sprintf("Based on %dGB memory, allocate %.0f%% to buffer pool for optimal performance", memoryGB, bufferPoolPercent*100),
	})

	instances := 1
	if bufferPoolSize > 1 {
		instances = (bufferPoolSize / 4) + 1
		if instances > 64 {
			instances = 64
		}
	}
	recommendations = append(recommendations, ParameterRecommendation{
		ParameterName: "innodb_buffer_pool_instances",
		Value:         fmt.Sprintf("%d", instances),
		DataType:      models.DataTypeInt,
		Reason:        fmt.Sprintf("Set %d instances for buffer pool larger than 1G to reduce contention", instances),
	})

	logFileSize := bufferPoolSize / 4
	if logFileSize > 2 {
		logFileSize = 2
	}
	if logFileSize < 1 {
		logFileSize = 1
	}
	recommendations = append(recommendations, ParameterRecommendation{
		ParameterName: "innodb_log_file_size",
		Value:         fmt.Sprintf("%dG", logFileSize),
		DataType:      models.DataTypeSize,
		Reason:        "Log file size about 25% of buffer pool size for optimal write performance",
	})

	maxConnections := 150
	if isOLTP {
		maxConnections = cpuCores * 100
		if maxConnections > 5000 {
			maxConnections = 5000
		}
	} else if isOLAP {
		maxConnections = cpuCores * 10
	}
	recommendations = append(recommendations, ParameterRecommendation{
		ParameterName: "max_connections",
		Value:         fmt.Sprintf("%d", maxConnections),
		DataType:      models.DataTypeInt,
		Reason:        fmt.Sprintf("Adjusted for workload type and %d CPU cores", cpuCores),
	})

	threadCache := maxConnections / 10
	if threadCache < 10 {
		threadCache = 10
	}
	recommendations = append(recommendations, ParameterRecommendation{
		ParameterName: "thread_cache_size",
		Value:         fmt.Sprintf("%d", threadCache),
		DataType:      models.DataTypeInt,
		Reason:        fmt.Sprintf("Cache %d threads to reduce connection overhead", threadCache),
	})

	tableCache := 400
	if isOLAP {
		tableCache = int(float64(diskGB) * 2)
		if tableCache > 2000 {
			tableCache = 2000
		}
	}
	recommendations = append(recommendations, ParameterRecommendation{
		ParameterName: "table_open_cache",
		Value:         fmt.Sprintf("%d", tableCache),
		DataType:      models.DataTypeInt,
		Reason:        fmt.Sprintf("Open %d tables concurrently based on workload", tableCache),
	})

	ioCapacity := 200
	if isOLAP {
		ioCapacity = cpuCores * 100
		if ioCapacity > 2000 {
			ioCapacity = 2000
		}
	}
	recommendations = append(recommendations, ParameterRecommendation{
		ParameterName: "innodb_io_capacity",
		Value:         fmt.Sprintf("%d", ioCapacity),
		DataType:      models.DataTypeInt,
		Reason:        fmt.Sprintf("I/O capacity %d based on CPU cores and workload", ioCapacity),
	})

	recommendations = append(recommendations, ParameterRecommendation{
		ParameterName: "innodb_flush_log_at_trx_commit",
		Value:         "1",
		DataType:      models.DataTypeInt,
		Reason:        "Set to 1 for ACID compliance (safety-first approach)",
	})

	recommendations = append(recommendations, ParameterRecommendation{
		ParameterName: "sync_binlog",
		Value:         "1",
		DataType:      models.DataTypeInt,
		Reason:        "Enable for durability in replication scenarios",
	})

	if isOLTP {
		recommendations = append(recommendations, ParameterRecommendation{
			ParameterName: "innodb_flush_method",
			Value:         "O_DIRECT",
			DataType:      models.DataTypeString,
			Reason:        "Use O_DIRECT for OLTP to avoid double buffering",
		})
	}

	result := &RecommendationResult{
		Recommendations: recommendations,
		HardwareProfile: fmt.Sprintf("%d cores, %dGB RAM, %dGB disk, %s workload",
			cpuCores, memoryGB, diskGB, req.WorkloadType),
		Summary: fmt.Sprintf("Generated %d parameter recommendations based on hardware configuration", len(recommendations)),
	}

	return result, nil
}

type CreateParameterTemplateRequest struct {
	Name        string           `json:"name" binding:"required"`
	Description string           `json:"description"`
	Category    string           `json:"category" binding:"required"`
	CreatedBy   string           `json:"created_by"`
	Parameters  []ParameterInput `json:"parameters"`
}

type ParameterInput struct {
	VersionID     string `json:"version_id"`
	ParameterName string `json:"parameter_name" binding:"required"`
	Value         string `json:"value" binding:"required"`
	DataType      string `json:"data_type" binding:"required"`
	MinValue      string `json:"min_value"`
	MaxValue      string `json:"max_value"`
	Unit          string `json:"unit"`
	Description   string `json:"description"`
	IsDynamic     bool   `json:"is_dynamic"`
	IsMandatory   bool   `json:"is_mandatory"`
	Category      string `json:"category"`
}

type UpdateParameterTemplateRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Parameters  *[]ParameterInput `json:"parameters"`
}

type ValidateParametersRequest struct {
	TemplateID string                     `json:"template_id"`
	InstanceID string                     `json:"instance_id"`
	// parameters 可空: 空时按 template_id 回退到模板自身参数做校验.
	Parameters []ParameterValidationInput `json:"parameters"`
}

type ParameterValidationInput struct {
	ParameterName string                   `json:"parameter_name" binding:"required"`
	Value         string                   `json:"value" binding:"required"`
	DataType      models.ParameterDataType `json:"data_type" binding:"required"`
	MinValue      string                   `json:"min_value"`
	MaxValue      string                   `json:"max_value"`
	IsMandatory   bool                     `json:"is_mandatory"`
}

type ValidationResult struct {
	IsValid  bool                `json:"is_valid"`
	Errors   []ValidationError   `json:"errors"`
	Warnings []ValidationWarning `json:"warnings"`
}

type ValidationError struct {
	ParameterName string `json:"parameter_name"`
	Message       string `json:"message"`
}

type ValidationWarning struct {
	ParameterName string `json:"parameter_name"`
	Message       string `json:"message"`
}

type RecommendParametersRequest struct {
	CPUCores     int    `json:"cpu_cores"`
	MemoryGB     int    `json:"memory_gb"`
	DiskGB       int    `json:"disk_gb"`
	WorkloadType string `json:"workload_type"`
}

type ApplyParameterTemplateRequest struct {
	TemplateID     string           `json:"template_id" binding:"required"`
	InstanceID     string           `json:"instance_id" binding:"required"`
	Parameters     []ParameterInput `json:"parameters"`
	RequireRestart bool             `json:"require_restart"`
}

type ApplyParameterTemplateResponse struct {
	TemplateID     string                 `json:"template_id"`
	InstanceID     string                 `json:"instance_id"`
	Applied        int                    `json:"applied"`
	Failed         int                    `json:"failed"`
	RequireRestart bool                   `json:"require_restart"`
	Results        []ApplyParameterResult `json:"results"`
}

type ApplyParameterResult struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type RecommendationResult struct {
	Recommendations []ParameterRecommendation `json:"recommendations"`
	HardwareProfile string                    `json:"hardware_profile"`
	Summary         string                    `json:"summary"`
}

type ParameterRecommendation struct {
	ParameterName string                   `json:"parameter_name"`
	Value         string                   `json:"value"`
	DataType      models.ParameterDataType `json:"data_type"`
	Reason        string                   `json:"reason"`
}
