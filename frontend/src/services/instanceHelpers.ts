/** Pure utility functions extracted from InstanceDetail.tsx */

// ---- Task Status Helpers ----

export const isFailedTaskStatus = (status?: string): boolean => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

export const isSuccessTaskStatus = (status?: string): boolean => {
  const normalized = (status || '').toLowerCase()
  return ['completed', 'success', 'succeeded', 'ok'].includes(normalized)
}

export const isActiveTaskStatus = (status?: string): boolean => {
  const normalized = (status || '').toLowerCase()
  return ['pending', 'running', 'submitted', 'accepted', 'queued'].includes(normalized)
}

export const formatTaskMessage = (result: any, fallback: string): string => {
  const parts = [
    result?.message || fallback,
    result?.status ? `status=${result.status}` : '',
    result?.task_id ? `task_id=${result.task_id}` : '',
  ].filter(Boolean)
  return parts.join(' | ')
}

export const formatBatchRows = (rows: any[]): string =>
  rows
    .map((row: any) => `${row?.name || row?.instance_id || '-'}:${row?.port || '-'} ${row?.status || 'unknown'}${row?.message ? ` - ${row.message}` : ''}`)
    .join('\n')

// ---- Password Helpers ----

export const checkPasswordComplexity = (pw: string): string[] => {
  const errors: string[] = []
  if (pw.length < 8) errors.push('至少8位')
  if (!/[A-Z]/.test(pw)) errors.push('大写字母')
  if (!/[a-z]/.test(pw)) errors.push('小写字母')
  if (!/[0-9]/.test(pw)) errors.push('数字')
  if (!/[^A-Za-z0-9]/.test(pw)) errors.push('特殊字符')
  return errors
}

export const getPasswordStrength = (pw: string): { level: number; label: string; color: string } => {
  if (!pw) return { level: 0, label: '', color: '' }
  let score = 0
  if (/[A-Z]/.test(pw)) score++
  if (/[a-z]/.test(pw)) score++
  if (/[0-9]/.test(pw)) score++
  if (/[^A-Za-z0-9]/.test(pw)) score++
  if (pw.length >= 8) score++
  if (score <= 2) return { level: score, label: '弱', color: 'red' }
  if (score <= 3) return { level: score, label: '中', color: 'orange' }
  if (score <= 4) return { level: score, label: '强', color: 'lime' }
  return { level: score, label: '非常强', color: 'green' }
}
