package utils

import (
	"encoding/json"
	"fmt"
	"unicode"
)

func ValidatePasswordComplexity(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("password must contain at least one digit")
	}
	if !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}
	return nil
}

// PasswordPolicy 表示 SecuritySettings 中 password_policy 的校验策略.
// 前端表单字段参见 frontend/src/pages/SecuritySettings.tsx ::PasswordPolicyForm.
type PasswordPolicy struct {
	MinLength        int  `json:"min_length"`
	RequireUppercase bool `json:"require_uppercase"`
	RequireLowercase bool `json:"require_lowercase"`
	RequireDigit     bool `json:"require_digit"`
	RequireSpecial   bool `json:"require_special"`
	MaxAgeDays       int  `json:"max_age_days"`
	HistoryCount     int  `json:"history_count"`
}

// defaultPasswordPolicy 是策略未配置时使用的服务端兜底策略.
// 与前端默认值保持一致 (SecuritySettings.tsx ::defaultPolicy).
var defaultPasswordPolicy = PasswordPolicy{
	MinLength:        8,
	RequireUppercase: true,
	RequireLowercase: true,
	RequireDigit:     true,
	RequireSpecial:   true,
}

// ValidatePasswordPolicy 根据给定的策略校验密码.
// policy 为零值时 fallback 到 defaultPasswordPolicy (等同于要求最短 8 位 + 四类字符).
// MaxAgeDays / HistoryCount 属于生命周期策略, 本次不强制实现 (不阻塞密码修改); 字段保留供将来使用.
func ValidatePasswordPolicy(password string, policy PasswordPolicy) error {
	if policy.MinLength == 0 {
		policy = defaultPasswordPolicy
	}
	if policy.MinLength > 0 && len(password) < policy.MinLength {
		return fmt.Errorf("password must be at least %d characters", policy.MinLength)
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if policy.RequireUppercase && !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if policy.RequireLowercase && !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if policy.RequireDigit && !hasDigit {
		return fmt.Errorf("password must contain at least one digit")
	}
	if policy.RequireSpecial && !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}
	return nil
}

// parsePasswordPolicy 从 settings 表中存储的 JSON string 解析策略.
// 若 value 为空 / 解析失败, 返回零值策略 (调用方应 fallback 到默认策略).
func ParsePasswordPolicy(value string) PasswordPolicy {
	if value == "" {
		return PasswordPolicy{}
	}
	var p PasswordPolicy
	if err := json.Unmarshal([]byte(value), &p); err != nil {
		return PasswordPolicy{}
	}
	return p
}
