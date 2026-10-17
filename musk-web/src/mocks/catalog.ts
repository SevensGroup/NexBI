import type { AgentInfo } from '@/types/agent'
import type { SemanticMetric, SemanticDimension, SemanticEntity, SemanticDataSource } from '@/types/semantic'
import type { MCPServer } from '@/types/mcp'
import type { MemoryItem } from '@/types/memory'
import type { Approval } from '@/types/approval'
import type { RunSummary } from '@/types/run'

export const MOCK_AGENTS: AgentInfo[] = [
  {
    id: 'coordinator',
    name: 'Coordinator',
    role: 'Coordinator',
    description: 'Plans, delegates, and finishes the user goal',
    status: 'IDLE',
    progress: 0,
    confidence: 0,
    tools: ['semantic_query', 'analyze_advertising', 'search_memory', 'save_memory'],
  },
  {
    id: 'analyst',
    name: 'Analyst',
    role: 'Analyst',
    description: 'Metric, anomaly, and campaign analysis',
    status: 'IDLE',
    progress: 0,
    confidence: 0,
    tools: ['semantic_query', 'analyze_advertising'],
  },
  {
    id: 'researcher',
    name: 'Researcher',
    role: 'Researcher',
    description: 'External search, industry context, background',
    status: 'IDLE',
    progress: 0,
    confidence: 0,
    tools: ['search_memory'],
  },
  {
    id: 'reviewer',
    name: 'Reviewer',
    role: 'Reviewer',
    description: 'Validates conclusions, evidence, and logic',
    status: 'IDLE',
    progress: 0,
    confidence: 0,
    tools: ['search_memory'],
  },
]

export const MOCK_METRICS: SemanticMetric[] = [
  { key: 'roas', name: 'ROAS', description: 'Return on Advertising Spend', formula: 'revenue / spend', aliases: ['ROI'] },
  { key: 'ctr', name: 'CTR', description: 'Click-through rate', formula: 'clicks / impressions' },
  { key: 'cpc', name: 'CPC', description: 'Cost per click', formula: 'spend / clicks' },
  { key: 'cpm', name: 'CPM', description: 'Cost per thousand impressions', formula: 'spend / impressions * 1000' },
  { key: 'cpa', name: 'CPA', description: 'Cost per acquisition', formula: 'spend / conversions' },
  { key: 'cvr', name: 'CVR', description: 'Conversion rate', formula: 'conversions / clicks' },
  { key: 'spend', name: 'Spend', description: 'Advertising spend', field: 'spend', aggregation: 'sum' },
  { key: 'revenue', name: 'Revenue', description: 'Attributed advertising revenue', field: 'revenue', aggregation: 'sum' },
]

export const MOCK_DIMENSIONS: SemanticDimension[] = [
  { key: 'campaign', name: 'Campaign', description: 'Advertising campaign', field: 'campaign_id', type: 'keyword', entity: 'Campaign' },
  { key: 'adgroup', name: 'AdGroup', description: 'Ad group', field: 'ad_group_id', type: 'keyword', entity: 'AdGroup' },
  { key: 'creative', name: 'Creative', description: 'Ad creative', field: 'creative_id', type: 'keyword', entity: 'Creative' },
  { key: 'country', name: 'Country', description: 'Country or region', field: 'country', type: 'keyword' },
  { key: 'device', name: 'Device', description: 'Device type', field: 'device', type: 'keyword' },
  { key: 'channel', name: 'Channel', description: 'Advertising channel / platform', field: 'channel', type: 'keyword' },
  { key: 'date', name: 'Date', description: 'Event date', field: 'date', type: 'date' },
]

export const MOCK_ENTITIES: SemanticEntity[] = [
  { key: 'campaign', name: 'Campaign', description: 'Advertising campaign', keyField: 'campaign_id', dimensions: ['Campaign', 'Channel'] },
  { key: 'adgroup', name: 'AdGroup', description: 'Ad group within a campaign', keyField: 'ad_group_id', dimensions: ['AdGroup', 'Campaign'] },
  { key: 'creative', name: 'Creative', description: 'Ad creative', keyField: 'creative_id', dimensions: ['Creative', 'AdGroup', 'Campaign'] },
]

export const MOCK_DATASOURCES: SemanticDataSource[] = [
  { key: 'ads', name: 'advertising', description: 'Advertising performance events', index: 'ads-*', dateField: 'date', default: true },
]

export const MOCK_MCP: MCPServer = {
  id: 'elasticsearch-mcp',
  name: 'Elasticsearch MCP',
  description: 'Read-only search / aggregate / count / get via MCP. Agent never talks to ES SDK.',
  health: 'healthy',
  latencyMs: 42,
  tools: [
    { name: 'search', description: 'Search documents with validated query DSL', status: 'ok' },
    { name: 'aggregate', description: 'Metric aggregations with bucket limits', status: 'ok' },
    { name: 'count', description: 'Count matching documents', status: 'ok' },
    { name: 'get', description: 'Get a document by id', status: 'idle' },
  ],
}

export const MOCK_MEMORIES: MemoryItem[] = [
  {
    id: 'mem-st-1',
    type: 'SHORT_TERM',
    agent: 'Coordinator',
    title: 'Current goal',
    content: 'Why did Meta ad ROI decline yesterday? Channel=Meta, metric=ROAS, window=yesterday.',
    tags: ['goal', 'meta', 'roas'],
    timestamp: '2026-08-26T02:10:01Z',
    used: true,
  },
  {
    id: 'mem-st-2',
    type: 'SHORT_TERM',
    agent: 'Coordinator',
    title: 'Active plan',
    content: 'Get Data → Trend → Campaign/Country → Creative → Root Cause → Review → Final',
    tags: ['plan'],
    timestamp: '2026-08-26T02:10:04Z',
    used: true,
  },
  {
    id: 'mem-st-3',
    type: 'SHORT_TERM',
    agent: 'Analyst',
    title: 'Recent decision',
    content: 'CALL_TOOL semantic_query on ROAS by campaign for Meta yesterday vs prior day.',
    tags: ['decision'],
    timestamp: '2026-08-26T02:10:08Z',
  },
  {
    id: 'mem-st-4',
    type: 'SHORT_TERM',
    agent: 'Analyst',
    title: 'Observation',
    content: 'Account ROAS 1.42 vs 1.98 (-28%). Spend up 17%, revenue down 16%.',
    tags: ['observation'],
    timestamp: '2026-08-26T02:10:14Z',
    used: true,
  },
  {
    id: 'mem-task-1',
    type: 'TASK_MEMORY',
    agent: 'Analyst',
    title: 'Meta US Prospecting diagnosis',
    content: 'Campaign cmp_us_prosp ROAS 0.71 vs 2.14. Creative CR-8821 CTR collapse is the primary driver.',
    tags: ['campaign', 'creative'],
    timestamp: '2026-08-25T18:40:00Z',
  },
  {
    id: 'mem-task-2',
    type: 'TASK_MEMORY',
    agent: 'Reviewer',
    title: 'Evidence gap closed',
    content: 'Creative ranking + country split confirmed US+Mobile concentration. Reviewer approved.',
    tags: ['review'],
    timestamp: '2026-08-25T19:02:00Z',
  },
  {
    id: 'mem-lt-1',
    type: 'LONG_TERM',
    agent: 'Coordinator',
    title: 'Business knowledge',
    content: 'Meta prospecting creatives older than 14 days typically lose 20–40% CTR; refresh before scaling spend.',
    tags: ['knowledge', 'creative'],
    timestamp: '2026-07-12T09:00:00Z',
    used: true,
  },
  {
    id: 'mem-lt-2',
    type: 'LONG_TERM',
    agent: 'Analyst',
    title: 'Historical insight',
    content: 'When Meta US spend ramps >15% day-over-day without creative refresh, ROAS usually drops before CPA recovers.',
    tags: ['insight', 'meta'],
    timestamp: '2026-06-03T11:20:00Z',
  },
  {
    id: 'mem-lt-3',
    type: 'LONG_TERM',
    agent: 'Coordinator',
    title: 'Successful strategy',
    content: 'Pause fatigued creatives, shift 20% budget to retargeting, then re-test 3 new hooks.',
    tags: ['strategy'],
    timestamp: '2026-05-18T08:00:00Z',
  },
]

export const MOCK_APPROVALS: Approval[] = [
  {
    id: 'appr-budget-1',
    runId: 'run-hitl-01',
    action: 'Reallocate Meta US Prospecting budget',
    description: 'Shift $8,000 daily budget from fatigued prospecting campaign to retargeting.',
    risk: 'HIGH',
    reason: 'Budget mutation is HIGH risk and requires a human checkpoint.',
    confidence: 0.86,
    status: 'PENDING',
    currentValue: 'Prospecting $18,400 / day',
    proposedValue: 'Prospecting $10,400 / day · Retargeting +$8,000',
    tool: 'budget_reallocate',
    timestamp: '2026-08-26T03:12:00Z',
  },
  {
    id: 'appr-pause-1',
    runId: 'run-hitl-01',
    action: 'Pause creative CR-8821',
    description: 'Pause the fatigued prospecting creative that lost 61% CTR.',
    risk: 'HIGH',
    reason: 'Pausing live ads changes delivery immediately.',
    confidence: 0.91,
    status: 'PENDING',
    currentValue: 'CR-8821 ACTIVE',
    proposedValue: 'CR-8821 PAUSED',
    tool: 'pause_creative',
    timestamp: '2026-08-26T03:12:20Z',
  },
  {
    id: 'appr-done-1',
    runId: 'run-ads-01',
    action: 'Export diagnosis report',
    description: 'Write the final report to shared memory (LOW risk, auto-approved).',
    risk: 'LOW',
    reason: 'Read/write memory only; no spend change.',
    confidence: 0.97,
    status: 'APPROVED',
    currentValue: 'not saved',
    proposedValue: 'saved to TASK_MEMORY',
    tool: 'save_memory',
    timestamp: '2026-08-26T02:11:40Z',
  },
]

export const MOCK_RUNS: RunSummary[] = [
  {
    id: 'run-ads-01',
    goal: 'Why did Meta ad ROI decline yesterday?',
    status: 'COMPLETED',
    agentId: 'advertising-analyst',
    startedAt: '2026-08-26T02:10:00Z',
    endedAt: '2026-08-26T02:11:52Z',
    durationMs: 112_000,
    steps: 12,
    llmCalls: 18,
    tokens: 42_860,
    costUsd: 0.184,
    confidence: 0.91,
  },
  {
    id: 'run-hitl-01',
    goal: 'Reallocate inefficient Meta budget to retargeting',
    status: 'WAITING_APPROVAL',
    agentId: 'coordinator',
    startedAt: '2026-08-26T03:11:40Z',
    durationMs: 48_000,
    steps: 6,
    llmCalls: 8,
    tokens: 12_440,
    costUsd: 0.052,
    confidence: 0.86,
  },
  {
    id: 'run-fail-01',
    goal: 'Pull the TikTok creative fatigue report for last week',
    status: 'FAILED',
    agentId: 'analyst',
    startedAt: '2026-08-25T16:02:00Z',
    endedAt: '2026-08-25T16:02:18Z',
    durationMs: 18_000,
    steps: 3,
    llmCalls: 4,
    tokens: 3_210,
    costUsd: 0.011,
  },
]

export const MOCK_DASHBOARD = {
  activeAgents: 2,
  completedToday: 11,
  successRate: 0.91,
  averageRuntimeMs: 86_400,
  llmCalls: 214,
  tokens: 612_440,
  costUsd: 2.37,
}

export const DEFAULT_GOAL = 'Build me an advertising data board'
