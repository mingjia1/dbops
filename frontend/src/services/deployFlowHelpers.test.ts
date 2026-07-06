import { describe, expect, it } from 'vitest'
import {
  createDefaultFlowSpec,
  deployPlanToFlowElements,
  flowSpecToDeployPayload,
  flowSpecToGraphSnapshot,
  middlewareByID,
  validateFlowSpec,
  type DeployFlowSpec,
} from './deployFlowHelpers'

const credential = { username: 'root', password: 'Root#1234' }

const withHosts = (spec: DeployFlowSpec): DeployFlowSpec => ({
  ...spec,
  nodes: spec.nodes.map((node, index) => ({ ...node, host_id: `host-${index + 1}` })),
})

describe('deploy flow helpers', () => {
  it('converts HA flow spec to deployment payload with middleware and tools', () => {
    const spec = withHosts(createDefaultFlowSpec('ha'))
    const keepalived = middlewareByID(spec, 'keepalived')
    if (keepalived) {
      keepalived.enabled = true
      keepalived.vip = '10.1.1.100'
      keepalived.vip_interface = 'eth0'
    }
    const proxysql = middlewareByID(spec, 'proxysql')
    if (proxysql) {
      proxysql.enabled = true
      proxysql.proxy_host_id = 'host-3'
      proxysql.proxy_port = 6033
      proxysql.proxy_agent_port = 9091
    }

    const payload = flowSpecToDeployPayload(spec, credential)

    expect(payload.cluster_type).toBe('ha')
    expect(payload.nodes).toHaveLength(2)
    expect(payload.nodes[0].instance_id).toBe('master')
    expect(payload.custom.middleware.keepalived.enabled).toBe(true)
    expect(payload.custom.middleware.proxysql.agent_port).toBe(9091)
    expect(payload.custom.middleware.proxysql.proxy_agent_port).toBe(9091)
    expect(payload.custom.tools.precheck.enabled).toBe(true)
    expect(payload.custom.tools.health_check.enabled).toBe(true)
    expect(payload.custom.flow_spec.nodes.map((node: any) => node.id)).toContain('master')
    expect(payload.custom.flow_spec.edges.map((edge: any) => edge.target)).toContain('keepalived')
    expect(payload.custom.flow_spec.repl_password).toBeUndefined()
  })

  it.each([
    ['mha', ['manager', 'master', 'replica']],
    ['mgr', ['primary', 'secondary', 'secondary']],
    ['pxc', ['bootstrap', 'secondary', 'secondary']],
  ] as const)('converts %s flow spec to deployment payload', (arch, roles) => {
    const spec = withHosts(createDefaultFlowSpec(arch))
    const payload = flowSpecToDeployPayload(spec, credential)

    expect(payload.cluster_type).toBe(arch)
    expect(payload.mode).toBe('real')
    expect(payload.nodes.map((node: any) => node.role)).toEqual(roles)
    expect(payload.custom.flow_spec.arch).toBe(arch)
  })

  it('serializes flow spec as graph nodes and edges', () => {
    const spec = withHosts(createDefaultFlowSpec('mha'))
    const snapshot = flowSpecToGraphSnapshot(spec)

    expect(snapshot.arch).toBe('mha')
    expect(snapshot.nodes.find((node) => node.id === 'arch-mha')).toMatchObject({ type: 'architecture' })
    expect(snapshot.nodes.find((node) => node.id === 'manager')).toMatchObject({ type: 'database' })
    expect(snapshot.edges[0]).toMatchObject({ source: 'arch-mha', target: 'precheck' })
  })

  it('rejects Keepalived for MGR', () => {
    const spec = withHosts(createDefaultFlowSpec('mgr'))
    const keepalived = middlewareByID(spec, 'keepalived')
    if (keepalived) keepalived.enabled = true

    expect(validateFlowSpec(spec)).toContain('MGR/PXC do not support Keepalived in this version')
  })

  it('requires ProxySQL host when enabled', () => {
    const spec = withHosts(createDefaultFlowSpec('pxc'))
    const proxysql = middlewareByID(spec, 'proxysql')
    if (proxysql) proxysql.enabled = true

    expect(validateFlowSpec(spec)).toContain('ProxySQL host is required')
  })

  it('rejects duplicate host and port endpoints', () => {
    const spec = withHosts(createDefaultFlowSpec('ha'))
    spec.nodes[1].host_id = spec.nodes[0].host_id
    spec.nodes[1].mysql_port = spec.nodes[0].mysql_port

    expect(validateFlowSpec(spec)).toContain('duplicate node port: host-1:3306')
  })

  it('maps plan and live status to flow elements', () => {
    const plan = {
      steps: [
        { id: 'validate_input', name: 'Validate', type: 'validate' },
        { id: 'deploy_master', name: 'Deploy Master', type: 'bootstrap', depends_on: ['validate_input'] },
      ],
    }
    const deployment = {
      steps: [
        { id: 'validate_input', name: 'Validate', status: 'completed' },
        { id: 'deploy_master', name: 'Deploy Master', status: 'running', message: 'installing' },
      ],
    }

    const elements = deployPlanToFlowElements(plan, deployment)

    expect(elements.nodes).toHaveLength(2)
    expect(elements.edges[0]).toMatchObject({ source: 'validate_input', target: 'deploy_master' })
    expect(elements.nodes[1].data?.status).toBe('running')
    expect(elements.nodes[1].data?.message).toBe('installing')
  })
})
