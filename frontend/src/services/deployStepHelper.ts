/** Represents a single deploy step in the progress timeline */
export interface DeployStep {
  name: string
  status: string
  message?: string
  started_at?: string
  completed_at?: string
}

/** Minimal deployment state needed for step processing */
export interface DeployState {
  deployment_id: string
  steps?: DeployStep[]
  [key: string]: unknown
}

/**
 * Pure function that processes an incoming step event and returns the updated
 * DeployState (or null if the event doesn't apply to this deployment).
 *
 * This is extracted from the onStep handler in ClusterDeploy.tsx so it can be
 * unit-tested without rendering the full component.
 *
 * Rules:
 * - If deployment_id doesn't match, returns unchanged `current` (same ref → React bails out).
 * - If stepName is empty/missing, returns unchanged `current`.
 * - If step already exists by name, updates its status/message/timestamps.
 * - If step is new, appends it with status 'pending' (or the provided status).
 * - started_at is set on transition to 'running' (or if previously missing).
 * - completed_at is set on transition to 'completed' or 'failed'.
 */
export function processStepEvent(
  current: DeployState | null,
  taskID: string,
  stepName: string,
  stepStatus?: string,
  stepMessage?: string,
): DeployState | null {
  if (!current || current.deployment_id !== taskID) return current
  if (!stepName) return current

  const steps = [...(current.steps || [])]
  const idx = steps.findIndex((s) => s.name === stepName)
  const now = new Date().toISOString()

  if (idx >= 0) {
    steps[idx] = {
      ...steps[idx],
      status: stepStatus || steps[idx].status,
      message: stepMessage || steps[idx].message,
      ...((stepStatus === 'running' || !steps[idx].started_at) ? { started_at: steps[idx].started_at || now } : {}),
      ...((stepStatus === 'completed' || stepStatus === 'failed') ? { completed_at: now } : {}),
    }
  } else {
    steps.push({
      name: stepName,
      status: stepStatus || 'pending',
      message: stepMessage,
      started_at: now,
    })
  }

  return { ...current, steps }
}
