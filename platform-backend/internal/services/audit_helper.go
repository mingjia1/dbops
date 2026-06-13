package services

import "context"

// userIDFromCtx 从 gin 上下文取 user_id. auth.ValidateToken middleware
// 把 "user_id" 写到 ctx. 非请求路径 (后台任务/CLI) 返空串.
func userIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value("user_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
