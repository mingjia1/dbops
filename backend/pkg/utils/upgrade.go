package utils

import (
	"fmt"
	"net/http"
)

// UpgradeErrorCode は upgrade バージョン検証で返す構造化エラー種別.
// フロントエンドは code を見てローカライズ可能なメッセージを表示できる.
type UpgradeErrorCode string

const (
	// CodeAgentUnavailable は agent に到達できずバージョン取得に失敗したことを示す.
	CodeAgentUnavailable UpgradeErrorCode = "agent_unavailable"
	// CodeAgentVersionEmpty は agent が空バージョンを返したことを示す.
	CodeAgentVersionEmpty UpgradeErrorCode = "agent_version_empty"
	// CodeAgentVersionTooOld は agent のバージョンが対象バージョンの最小要件を下回ることを示す.
	CodeAgentVersionTooOld UpgradeErrorCode = "agent_version_too_old"
	// CodeAgentVersionTooNew は agent のバージョンが対象バージョンの最大許容を上回ることを示す.
	CodeAgentVersionTooNew UpgradeErrorCode = "agent_version_too_new"
	// CodeUpgradeIncompatible はソース/ターゲット組合せがカタログで許容されないことを示す.
	CodeUpgradeIncompatible UpgradeErrorCode = "upgrade_incompatible"
)

// UpgradeError は upgrade 関連の構造化エラー.
type UpgradeError struct {
	Code    UpgradeErrorCode `json:"code"`
	Message string           `json:"message"`
	Details string           `json:"details,omitempty"`
}

func (e *UpgradeError) Error() string {
	return e.Message
}

// IsUpgradeError は error が *UpgradeError なのかを判定する.
func IsUpgradeError(err error) (*UpgradeError, bool) {
	e, ok := err.(*UpgradeError)
	return e, ok
}

// AgentVersionError は agent 側バーション検証の失敗を UpgradeError に変換するヘルパー.
func AgentVersionError(code UpgradeErrorCode, agentVer, targetVersion, requirement string) *UpgradeError {
	details := ""
	if agentVer != "" {
		details = fmt.Sprintf("agent_version=%s target=%s", agentVer, targetVersion)
	} else {
		details = fmt.Sprintf("target=%s", targetVersion)
	}
	if requirement != "" {
		details += " requirement=" + requirement
	}
	return &UpgradeError{
		Code:    code,
		Message: versionErrorMessage(code, agentVer, targetVersion, requirement),
		Details: details,
	}
}

func versionErrorMessage(code UpgradeErrorCode, agentVer, targetVersion, requirement string) string {
	switch code {
	case CodeAgentUnavailable:
		return "agent is unavailable; cannot verify upgrade compatibility"
	case CodeAgentVersionEmpty:
		return "agent version is empty; cannot verify upgrade compatibility"
	case CodeAgentVersionTooOld:
		return fmt.Sprintf("agent version %s is too old for target version %s (requires >= %s)", agentVer, targetVersion, requirement)
	case CodeAgentVersionTooNew:
		return fmt.Sprintf("agent version %s is newer than supported by target version %s (requires <= %s)", agentVer, targetVersion, requirement)
	default:
		return "upgrade compatibility check failed"
	}
}

// UpgradeHTTPStatus は UpgradeError の code に対応する HTTP ステータスを返す.
// バージョン検証失敗は客户端問題なので 400 を返し、agent 障害は 502 を返す.
func UpgradeHTTPStatus(err *UpgradeError) int {
	switch err.Code {
	case CodeAgentUnavailable:
		return http.StatusBadGateway
	case CodeAgentVersionEmpty, CodeAgentVersionTooOld, CodeAgentVersionTooNew, CodeUpgradeIncompatible:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
