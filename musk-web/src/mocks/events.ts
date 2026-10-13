import type { AgentEvent } from '@/types/events'
import type { Plan } from '@/types/task'
import type { Report } from '@/types/report'
import { DEFAULT_GOAL } from './catalog'

const RUN_ID = 'run-ads-01'
const T0 = Date.parse('2026-08-26T02:10:00Z')

function at(offsetMs: number): string {
  return new Date(T0 + offsetMs).toISOString()
}

export const ADS_PLAN: Plan = {
  id: 'plan-ads-01',
  goal: DEFAULT_GOAL,
  tasks: [
    { id: 't-goal', title: 'Goal', description: 'Understand the Meta ROAS decline question', dependencies: [], priority: 0, status: 'COMPLETED' },
    { id: 't-data', title: 'Get Data', description: 'Pull Meta spend/revenue via Semantic Layer', dependencies: ['t-goal'], priority: 1, status: 'COMPLETED', agent: 'Analyst' },
    { id: 't-trend', title: 'Trend Analysis', description: 'Compare yesterday vs prior day and 7-day baseline', dependencies: ['t-data'], priority: 2, status: 'COMPLETED', agent: 'Analyst' },
    { id: 't-camp', title: 'Campaign/Country Analysis', description: 'Rank campaigns and countries by ROAS delta', dependencies: ['t-trend'], priority: 3, status: 'COMPLETED', agent: 'Analyst' },
    { id: 't-creative', title: 'Creative Analysis', description: 'Inspect creative CTR/CVR fatigue', dependencies: ['t-camp'], priority: 4, status: 'COMPLETED', agent: 'Analyst' },
    { id: 't-cause', title: 'Root Cause', description: 'Attribute ROAS drop to spend mix + creative fatigue', dependencies: ['t-creative'], priority: 5, status: 'COMPLETED', agent: 'Analyst' },
    { id: 't-review', title: 'Review', description: 'Reviewer validates evidence and logic', dependencies: ['t-cause'], priority: 6, status: 'COMPLETED', agent: 'Reviewer' },
    { id: 't-final', title: 'Final', description: 'Emit structured advertising report', dependencies: ['t-review'], priority: 7, status: 'COMPLETED', agent: 'Coordinator' },
  ],
}

export const ADS_REPORT: Report = {
  summary:
    'Yesterday Meta ROAS fell 28% (1.98 → 1.42). The drop is concentrated in US Mobile prospecting: campaign Meta US Prospecting (cmp_us_prosp) ROAS 0.71 vs 2.14, driven by creative CR-8821 CTR collapse (-61%) while spend was increased 22%.',
  metrics: {
    roas: 1.42,
    roas_baseline: 1.98,
    spend: 48200,
    spend_baseline: 41100,
    revenue: 68444,
    revenue_baseline: 81378,
    ctr: 0.0084,
    cpc: 1.62,
    cvr: 0.021,
    cpa: 77.1,
  },
  anomalies: [
    {
      entity: 'Campaign',
      name: 'Meta US Prospecting',
      metric: 'ROAS',
      current: 0.71,
      baseline: 2.14,
      changePct: -0.668,
      direction: 'down',
      evidenceId: 'ev-campaign',
    },
    {
      entity: 'Creative',
      name: 'CR-8821',
      metric: 'CTR',
      current: 0.0041,
      baseline: 0.0106,
      changePct: -0.613,
      direction: 'down',
      evidenceId: 'ev-creative',
    },
  ],
  rootCauses: [
    {
      statement: 'Creative CR-8821 fatigue collapsed CTR on the largest prospecting campaign while budget was increased.',
      evidenceIds: ['ev-creative', 'ev-campaign'],
    },
    {
      statement: 'Spend mix shifted toward US Mobile prospecting, a lower-converting slice versus retargeting.',
      evidenceIds: ['ev-country', 'ev-trend'],
    },
  ],
  evidence: [
    { id: 'ev-trend', description: 'Account Meta ROAS 1.42 vs 1.98; spend +17%, revenue -16%', period: '2026-08-25 vs 2026-08-24', dimension: 'channel=Meta' },
    { id: 'ev-campaign', description: 'cmp_us_prosp ROAS 0.71 vs 2.14; spend $18.4k', period: '2026-08-25', dimension: 'campaign' },
    { id: 'ev-creative', description: 'CR-8821 CTR 0.41% vs 1.06%; impressions stable', period: '2026-08-25', dimension: 'creative' },
    { id: 'ev-country', description: 'US Mobile contributed 61% of incremental spend and 82% of ROAS loss', period: '2026-08-25', dimension: 'country+device' },
  ],
  recommendations: [
    { action: 'Pause CR-8821 and launch 3 replacement hooks on US prospecting.', evidenceIds: ['ev-creative'] },
    { action: 'Shift ~$8k/day from US Prospecting to retargeting until new creatives stabilize CTR.', evidenceIds: ['ev-campaign', 'ev-country'] },
    { action: 'Cap day-over-day prospecting spend ramps at 10% without a creative refresh.', evidenceIds: ['ev-trend'] },
  ],
  confidence: 0.91,
}

const TREND_DSL = {
  size: 0,
  query: {
    bool: {
      filter: [
        { term: { channel: 'Meta' } },
        { range: { date: { gte: '2026-08-24', lt: '2026-08-26' } } },
      ],
    },
  },
  aggs: {
    by_date: {
      date_histogram: { field: 'date', calendar_interval: 'day' },
      aggs: { spend: { sum: { field: 'spend' } }, revenue: { sum: { field: 'revenue' } } },
    },
  },
}

export function buildAdvertisingEvents(runId = RUN_ID, goal = DEFAULT_GOAL): AgentEvent[] {
  let seq = 0
  const ev = (type: AgentEvent['type'], step: number, offset: number, payload: Record<string, unknown>): AgentEvent => ({
    id: `evt-${++seq}`,
    runId,
    step,
    type,
    timestamp: at(offset),
    payload,
  })

  return [
    ev('agent.started', 0, 0, { agentId: 'advertising-analyst', goal }),
    ev('goal.understood', 0, 800, {
      raw: goal,
      channel: 'Meta',
      metric: 'ROAS',
      timeRange: 'yesterday',
      summary: 'Investigate yesterday Meta ROAS decline vs prior day.',
    }),
    ev('memory.retrieved', 0, 1100, {
      ids: ['mem-lt-1', 'mem-lt-2'],
      titles: ['Business knowledge', 'Historical insight'],
    }),
    ev('plan.created', 1, 1800, { plan: ADS_PLAN }),
    ...ADS_PLAN.tasks.map((task, i) => ev('task.created', 1, 1900 + i * 40, { task })),
    ev('task.started', 2, 2600, { taskId: 't-data' }),
    ev('decision.created', 2, 2800, {
      decision: {
        id: 'd-1',
        runId,
        step: 2,
        type: 'CALL_TOOL',
        objective: 'Load Meta account metrics for yesterday vs prior day',
        summary: 'Query Semantic Layer for ROAS, spend, revenue.',
        reason: 'Need account-level evidence before drilling into campaigns.',
        confidence: 0.88,
        risk: 'LOW',
        nextAction: 'semantic_query',
        timestamp: at(2800),
        tool: 'semantic_query',
      },
    }),
    ev('semantic.query.created', 2, 3000, {
      query: {
        metrics: ['roas', 'spend', 'revenue', 'ctr', 'cpc', 'cvr', 'cpa'],
        dimensions: ['date'],
        filters: [{ dimension: 'channel', op: 'eq', value: 'Meta' }],
        timeRange: { start: '2026-08-24', end: '2026-08-26', label: 'yesterday vs prior day' },
      },
    }),
    ev('semantic.query.compiled', 2, 3200, { dsl: TREND_DSL, compiler: 'semantic-v1' }),
    ev('mcp.started', 2, 3400, {
      server: 'Elasticsearch MCP',
      tool: 'aggregate',
      request: { method: 'tools/call', params: { name: 'aggregate', arguments: { index: 'ads-*', body: TREND_DSL } } },
    }),
    ev('mcp.completed', 2, 3900, {
      tool: 'aggregate',
      latencyMs: 48,
      result: {
        aggregations: {
          by_date: {
            buckets: [
              { key_as_string: '2026-08-24', spend: { value: 41100 }, revenue: { value: 81378 } },
              { key_as_string: '2026-08-25', spend: { value: 48200 }, revenue: { value: 68444 } },
            ],
          },
        },
      },
    }),
    ev('tool.started', 2, 3450, {
      toolCall: {
        id: 'tc-1',
        runId,
        toolName: 'semantic_query',
        description: 'Semantic query → compiler → Elasticsearch aggregate',
        arguments: { metrics: ['roas', 'spend', 'revenue'], filters: { channel: 'Meta' } },
        status: 'RUNNING',
        duration: 0,
        risk: 'LOW',
        timestamp: at(3450),
      },
    }),
    ev('tool.completed', 2, 4000, {
      toolCallId: 'tc-1',
      duration: 550,
      result: { roas: 1.42, spend: 48200, revenue: 68444, baseline_roas: 1.98 },
    }),
    ev('observation.created', 2, 4200, {
      observation: {
        id: 'obs-1',
        runId,
        step: 2,
        summary: 'Account ROAS 1.42 vs 1.98 (−28%). Spend +17%, revenue −16%.',
        reason: 'Decline is real at account level, not a tracking gap.',
        evidence: 'ev-trend',
        confidence: 0.93,
        toolCallId: 'tc-1',
        timestamp: at(4200),
      },
    }),
    ev('task.completed', 2, 4300, { taskId: 't-data' }),
    ev('task.started', 3, 4500, { taskId: 't-trend' }),
    ev('task.completed', 3, 4700, { taskId: 't-trend' }),
    ev('task.started', 4, 4900, { taskId: 't-camp' }),
    ev('decision.created', 4, 5100, {
      decision: {
        id: 'd-2',
        runId,
        step: 4,
        type: 'CALL_TOOL',
        objective: 'Rank campaigns and countries by ROAS delta',
        summary: 'Drill into campaign and country slices.',
        reason: 'Account drop is concentrated; identify which entities moved.',
        confidence: 0.9,
        risk: 'LOW',
        nextAction: 'semantic_query by campaign, country',
        timestamp: at(5100),
        tool: 'semantic_query',
      },
    }),
    ev('semantic.query.created', 4, 5200, {
      query: {
        metrics: ['roas', 'spend', 'revenue'],
        dimensions: ['campaign', 'country'],
        filters: [{ dimension: 'channel', op: 'eq', value: 'Meta' }],
        timeRange: { start: '2026-08-25', end: '2026-08-26', label: 'yesterday' },
        orderBy: [{ field: 'spend', direction: 'desc' }],
        limit: 10,
      },
    }),
    ev('tool.started', 4, 5300, {
      toolCall: {
        id: 'tc-2',
        runId,
        toolName: 'semantic_query',
        description: 'Campaign / country ranking',
        arguments: { dimensions: ['campaign', 'country'] },
        status: 'RUNNING',
        duration: 0,
        risk: 'LOW',
        timestamp: at(5300),
      },
    }),
    ev('mcp.started', 4, 5350, { server: 'Elasticsearch MCP', tool: 'aggregate' }),
    ev('mcp.completed', 4, 5600, { tool: 'aggregate', latencyMs: 36, result: { campaigns: 8 } }),
    ev('tool.completed', 4, 5700, {
      toolCallId: 'tc-2',
      duration: 400,
      result: {
        campaigns: [
          { name: 'Meta US Prospecting', roas: 0.71, spend: 18400 },
          { name: 'Meta US Retargeting', roas: 3.12, spend: 9600 },
          { name: 'Meta EU Prospecting', roas: 1.66, spend: 7200 },
        ],
      },
    }),
    ev('observation.created', 4, 5900, {
      observation: {
        id: 'obs-2',
        runId,
        step: 4,
        summary: 'Meta US Prospecting accounts for most of the ROAS loss (0.71 vs 2.14).',
        reason: 'Need creative-level confirmation inside that campaign.',
        evidence: 'ev-campaign',
        confidence: 0.9,
        toolCallId: 'tc-2',
        timestamp: at(5900),
      },
    }),
    ev('task.completed', 4, 6000, { taskId: 't-camp' }),
    ev('task.started', 5, 6200, { taskId: 't-creative' }),
    ev('decision.created', 5, 6400, {
      decision: {
        id: 'd-3',
        runId,
        step: 5,
        type: 'DELEGATE_AGENT',
        objective: 'Creative fatigue analysis on cmp_us_prosp',
        summary: 'Delegate Analyst to inspect creative CTR/CVR.',
        reason: 'Specialist should isolate whether the campaign drop is creative-driven.',
        confidence: 0.87,
        risk: 'LOW',
        nextAction: 'Delegate Analyst',
        timestamp: at(6400),
        agent: 'Analyst',
      },
    }),
    ev('agent.delegated', 5, 6500, {
      from: 'Coordinator',
      to: 'Analyst',
      task: 'Creative Analysis',
      status: 'delegated',
    }),
    ev('tool.started', 5, 6700, {
      toolCall: {
        id: 'tc-3',
        runId,
        toolName: 'analyze_advertising',
        description: 'Creative ranking inside Meta US Prospecting',
        arguments: { campaign: 'cmp_us_prosp', metrics: ['ctr', 'cvr', 'cpc'] },
        status: 'RUNNING',
        duration: 0,
        risk: 'LOW',
        timestamp: at(6700),
      },
    }),
    ev('tool.completed', 5, 7200, {
      toolCallId: 'tc-3',
      duration: 500,
      result: { creatives: [{ id: 'CR-8821', ctr: 0.0041, baseline: 0.0106 }] },
    }),
    ev('observation.created', 5, 7400, {
      observation: {
        id: 'obs-3',
        runId,
        step: 5,
        summary: 'CR-8821 CTR −61% with stable impressions; fatigue, not delivery loss.',
        reason: 'Primary mechanism for campaign ROAS collapse.',
        evidence: 'ev-creative',
        confidence: 0.92,
        toolCallId: 'tc-3',
        timestamp: at(7400),
      },
    }),
    ev('agent.delegated', 5, 7500, {
      from: 'Analyst',
      to: 'Coordinator',
      task: 'Creative Analysis',
      status: 'completed',
    }),
    ev('task.completed', 5, 7600, { taskId: 't-creative' }),
    ev('task.started', 6, 7800, { taskId: 't-cause' }),
    ev('decision.created', 6, 8000, {
      decision: {
        id: 'd-4',
        runId,
        step: 6,
        type: 'REFLECT',
        objective: 'Check whether evidence is sufficient to explain the ROAS drop',
        summary: 'Reflect on goal satisfaction and evidence coverage.',
        reason: 'Have account, campaign, and creative evidence; validate completeness.',
        confidence: 0.84,
        risk: 'LOW',
        nextAction: 'REFLECT',
        timestamp: at(8000),
      },
    }),
    ev('reflection.created', 6, 8300, {
      reflection: {
        id: 'ref-1',
        runId,
        step: 6,
        goalSatisfied: false,
        evidenceSufficient: false,
        resultCorrect: true,
        missingInformation: ['Reviewer sign-off on causal chain', 'Device split confirmation'],
        errors: [],
        nextAction: 'CONTINUE',
        confidence: 0.78,
        timestamp: at(8300),
        summary: 'Cause is plausible; independent review still required.',
      },
    }),
    ev('task.completed', 6, 8400, { taskId: 't-cause' }),
    ev('task.started', 7, 8600, { taskId: 't-review' }),
    ev('decision.created', 7, 8800, {
      decision: {
        id: 'd-5',
        runId,
        step: 7,
        type: 'DELEGATE_AGENT',
        objective: 'Independent evidence review',
        summary: 'Delegate Reviewer to validate root causes.',
        reason: 'Reflection flagged missing reviewer sign-off.',
        confidence: 0.9,
        risk: 'LOW',
        nextAction: 'Delegate Reviewer',
        timestamp: at(8800),
        agent: 'Reviewer',
      },
    }),
    ev('agent.delegated', 7, 8900, {
      from: 'Coordinator',
      to: 'Reviewer',
      task: 'Review',
      status: 'delegated',
    }),
    ev('observation.created', 7, 9400, {
      observation: {
        id: 'obs-4',
        runId,
        step: 7,
        summary: 'Reviewer approved causal chain: creative fatigue + spend mix.',
        reason: 'Evidence IDs resolve; no contradictory slices.',
        evidence: 'ev-country',
        confidence: 0.9,
        timestamp: at(9400),
      },
    }),
    ev('agent.delegated', 7, 9500, {
      from: 'Reviewer',
      to: 'Coordinator',
      task: 'Review',
      status: 'completed',
    }),
    ev('reflection.created', 7, 9700, {
      reflection: {
        id: 'ref-2',
        runId,
        step: 7,
        goalSatisfied: true,
        evidenceSufficient: true,
        resultCorrect: true,
        missingInformation: [],
        errors: [],
        nextAction: 'FINISH',
        confidence: 0.91,
        timestamp: at(9700),
        summary: 'Ready to finish. Evidence covers trend, campaign, creative, and mix.',
      },
    }),
    ev('task.completed', 7, 9800, { taskId: 't-review' }),
    ev('task.started', 8, 10000, { taskId: 't-final' }),
    ev('decision.created', 8, 10200, {
      decision: {
        id: 'd-6',
        runId,
        step: 8,
        type: 'FINISH',
        objective: 'Publish advertising diagnosis report',
        summary: 'Finish with structured report and recommendations.',
        reason: 'Goal satisfied and evidence sufficient.',
        confidence: 0.91,
        risk: 'LOW',
        nextAction: 'Emit Final Report',
        timestamp: at(10200),
      },
    }),
    ev('memory.saved', 8, 10400, { id: 'mem-task-1', type: 'TASK_MEMORY' }),
    ev('report.created', 8, 10600, { report: ADS_REPORT }),
    ev('task.completed', 8, 10700, { taskId: 't-final' }),
    ev('agent.completed', 8, 11000, { status: 'COMPLETED', confidence: 0.91 }),
  ]
}

export function buildHitlEvents(runId = 'run-hitl-01'): AgentEvent[] {
  let seq = 0
  const ev = (type: AgentEvent['type'], step: number, payload: Record<string, unknown>): AgentEvent => ({
    id: `hitl-${++seq}`,
    runId,
    step,
    type,
    timestamp: new Date(Date.parse('2026-08-26T03:11:40Z') + seq * 400).toISOString(),
    payload,
  })
  return [
    ev('agent.started', 0, { agentId: 'coordinator', goal: 'Reallocate inefficient Meta budget to retargeting' }),
    ev('goal.understood', 0, {
      summary: 'Reallocate budget from fatigued Meta prospecting to retargeting.',
      channel: 'Meta',
    }),
    ev('plan.created', 1, {
      plan: {
        id: 'plan-hitl',
        goal: 'Reallocate inefficient Meta budget to retargeting',
        tasks: [
          { id: 'h1', title: 'Diagnose', description: 'Confirm CR-8821 fatigue', dependencies: [], priority: 1, status: 'COMPLETED' },
          { id: 'h2', title: 'Propose reallocation', description: 'Draft budget shift', dependencies: ['h1'], priority: 2, status: 'RUNNING' },
        ],
      },
    }),
    ev('decision.created', 2, {
      decision: {
        id: 'd-hitl',
        runId,
        step: 2,
        type: 'CALL_TOOL',
        objective: 'Shift $8k/day to retargeting',
        summary: 'Mutating live budget requires approval.',
        reason: 'HIGH risk spend change.',
        confidence: 0.86,
        risk: 'HIGH',
        nextAction: 'budget_reallocate (awaiting approval)',
        timestamp: new Date().toISOString(),
        tool: 'budget_reallocate',
      },
    }),
    ev('approval.required', 2, {
      approval: {
        id: 'appr-budget-1',
        runId,
        action: 'Reallocate Meta US Prospecting budget',
        description: 'Shift $8,000 daily budget from fatigued prospecting campaign to retargeting.',
        risk: 'HIGH',
        reason: 'Budget mutation is HIGH risk and requires a human checkpoint.',
        confidence: 0.86,
        status: 'PENDING',
        currentValue: 'Prospecting $18,400 / day',
        proposedValue: 'Prospecting $10,400 / day · Retargeting +$8,000',
        tool: 'budget_reallocate',
        timestamp: new Date().toISOString(),
      },
    }),
  ]
}

export const CHART_SERIES = {
  dates: ['08-19', '08-20', '08-21', '08-22', '08-23', '08-24', '08-25'],
  roas: [2.05, 1.99, 2.11, 1.94, 2.02, 1.98, 1.42],
  spend: [38, 40, 39, 41, 40, 41.1, 48.2],
  revenue: [78, 80, 82, 80, 81, 81.4, 68.4],
  ctr: [1.12, 1.08, 1.1, 1.05, 1.07, 1.06, 0.84],
  cpc: [1.21, 1.24, 1.22, 1.28, 1.26, 1.29, 1.62],
  cvr: [2.4, 2.5, 2.45, 2.38, 2.42, 2.41, 2.1],
  cpa: [50, 49, 50, 54, 52, 53, 77],
  campaigns: [
    { name: 'US Prospecting', roas: 0.71, spend: 18.4 },
    { name: 'US Retargeting', roas: 3.12, spend: 9.6 },
    { name: 'EU Prospecting', roas: 1.66, spend: 7.2 },
    { name: 'Lookalike 2%', roas: 1.21, spend: 6.1 },
    { name: 'Advantage+', roas: 1.88, spend: 6.9 },
  ],
  creatives: [
    { name: 'CR-8821', ctr: 0.41, spend: 11.2 },
    { name: 'CR-7740', ctr: 1.18, spend: 4.1 },
    { name: 'CR-9012', ctr: 1.02, spend: 3.3 },
    { name: 'CR-6618', ctr: 0.88, spend: 2.8 },
  ],
}
