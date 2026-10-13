<script setup lang="ts">
import { computed } from 'vue'
import MCPServerCard from '@/components/tool/MCPServerCard.vue'
import MCPToolCard from '@/components/tool/MCPToolCard.vue'
import MCPQueryViewer from '@/components/tool/MCPQueryViewer.vue'
import MCPResultViewer from '@/components/tool/MCPResultViewer.vue'
import ToolExecutionCard from '@/components/tool/ToolExecutionCard.vue'
import ToolTimeline from '@/components/tool/ToolTimeline.vue'
import { MOCK_MCP } from '@/mocks/catalog'
import { useAgentRunStore } from '@/stores/agentRun'

const run = useAgentRunStore()
const last = computed(() => run.state.mcp.at(-1))
</script>

<template>
  <div class="h-full overflow-auto p-6">
    <h1 class="text-lg font-medium">Elasticsearch MCP Console</h1>
    <p class="mt-1 text-sm text-mute">
      Agent → Semantic Layer → Query Compiler → MCP → Elasticsearch → Result
    </p>
    <div class="mt-4 grid gap-3 md:grid-cols-2">
      <MCPServerCard :server="MOCK_MCP" />
      <div class="grid grid-cols-2 gap-2">
        <MCPToolCard v-for="t in MOCK_MCP.tools" :key="t.name" :tool="t" />
      </div>
    </div>
    <div class="mt-6">
      <MCPQueryViewer
        :query="last?.semanticQuery || run.state.queries.at(-1) || { metrics: ['roas'], filters: [{ dimension: 'channel', op: 'eq', value: 'Meta' }] }"
        :dsl="last?.generatedDsl || run.state.compiledDsl || { size: 0 }"
        :request="last?.mcpRequest || { method: 'tools/call', params: { name: 'aggregate' } }"
      />
    </div>
    <div class="mt-4">
      <MCPResultViewer :result="last?.elasticsearchResult ?? null" :latency-ms="last?.latencyMs" :status="last?.status || 'idle'" />
    </div>
    <div class="mt-6 grid gap-4 md:grid-cols-2">
      <div class="space-y-2">
        <h2 class="text-xs uppercase text-mute">Tool Calls</h2>
        <ToolExecutionCard v-for="t in run.state.toolCalls" :key="t.id" :tool="t" />
        <div v-if="!run.state.toolCalls.length" class="text-xs text-mute">Run the advertising analyst to populate tool calls.</div>
      </div>
      <div>
        <h2 class="mb-2 text-xs uppercase text-mute">Decision → Tool → Observation</h2>
        <ToolTimeline />
      </div>
    </div>
  </div>
</template>
