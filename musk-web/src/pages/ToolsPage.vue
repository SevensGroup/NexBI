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

defineProps<{ embedded?: boolean }>()

const run = useAgentRunStore()
const last = computed(() => run.state.mcp.at(-1))
</script>

<template>
  <div class="h-full overflow-auto">
    <div class="mx-auto flex max-w-5xl flex-col gap-8 px-6 py-8">
      <header v-if="!embedded">
        <h1 class="text-xl font-semibold tracking-tight">Tools</h1>
        <p class="mt-1 text-sm text-mute">Agent, semantic query, compiler, MCP, then the result.</p>
      </header>

      <section>
        <div class="mb-3">
          <h2 class="text-sm font-medium">Server</h2>
          <p class="mt-0.5 text-xs leading-5 text-mute">The MCP endpoint and the tools it exposes.</p>
        </div>
        <div class="grid gap-3 md:grid-cols-2">
          <MCPServerCard :server="MOCK_MCP" />
          <div class="grid grid-cols-2 gap-3">
            <MCPToolCard v-for="t in MOCK_MCP.tools" :key="t.name" :tool="t" />
          </div>
        </div>
      </section>

      <section>
        <div class="mb-3">
          <h2 class="text-sm font-medium">Last call</h2>
          <p class="mt-0.5 text-xs leading-5 text-mute">Query, compiled DSL, and the MCP request.</p>
        </div>
        <div class="rounded-xl border border-line bg-panel p-4">
          <MCPQueryViewer
            :query="last?.semanticQuery || run.state.queries.at(-1) || { metrics: ['roas'], filters: [{ dimension: 'channel', op: 'eq', value: 'Meta' }] }"
            :dsl="last?.generatedDsl || run.state.compiledDsl || { size: 0 }"
            :request="last?.mcpRequest || { method: 'tools/call', params: { name: 'aggregate' } }"
          />
        </div>
      </section>

      <section>
        <div class="mb-3">
          <h2 class="text-sm font-medium">Result</h2>
          <p class="mt-0.5 text-xs leading-5 text-mute">What came back from the last tool call.</p>
        </div>
        <div class="rounded-xl border border-line bg-panel p-4">
          <MCPResultViewer :result="last?.elasticsearchResult ?? null" :latency-ms="last?.latencyMs" :status="last?.status || 'idle'" />
        </div>
      </section>

      <section class="grid gap-8 md:grid-cols-2">
        <div>
          <div class="mb-3">
            <h2 class="text-sm font-medium">Tool calls</h2>
            <p class="mt-0.5 text-xs leading-5 text-mute">Calls from the current run.</p>
          </div>
          <div class="space-y-3">
            <ToolExecutionCard v-for="t in run.state.toolCalls" :key="t.id" :tool="t" />
            <p v-if="!run.state.toolCalls.length" class="rounded-xl border border-line bg-panel px-4 py-6 text-sm text-mute">
              Run the advertising analyst to populate tool calls.
            </p>
          </div>
        </div>
        <div>
          <div class="mb-3">
            <h2 class="text-sm font-medium">Timeline</h2>
            <p class="mt-0.5 text-xs leading-5 text-mute">Decision, tool, then observation.</p>
          </div>
          <div class="rounded-xl border border-line bg-panel p-4">
            <ToolTimeline />
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
