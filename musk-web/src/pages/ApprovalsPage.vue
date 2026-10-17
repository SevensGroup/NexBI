<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import ApprovalCenter from '@/components/approval/ApprovalCenter.vue'
import ApprovalDetailDrawer from '@/components/approval/ApprovalDetailDrawer.vue'
import { listApprovals, approveAction, rejectAction } from '@/services/api/agent'
import { useAgentRunStore } from '@/stores/agentRun'
import type { Approval } from '@/types'
import { ElMessage } from 'element-plus'

const run = useAgentRunStore()
const items = ref<Approval[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const drawer = ref(false)
const currentId = ref<string | null>(null)
const current = computed(() => items.value.find((a) => a.id === currentId.value) ?? null)

onMounted(async () => {
  try {
    const fromRun = run.state.approvals
    const catalog = await listApprovals()
    const map = new Map<string, Approval>()
    for (const a of [...catalog, ...fromRun]) map.set(a.id, a)
    items.value = [...map.values()]
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load approvals'
  } finally {
    loading.value = false
  }
})

function open(id: string): void {
  currentId.value = id
  drawer.value = true
}

async function approve(id: string): Promise<void> {
  try {
    await approveAction(id)
    items.value = items.value.map((a) => (a.id === id ? { ...a, status: 'APPROVED' } : a))
    run.ingest({
      id: `appr-ok-${Date.now()}`,
      runId: run.state.runId || items.value.find((a) => a.id === id)?.runId || '',
      step: 0,
      type: 'approval.approved',
      timestamp: new Date().toISOString(),
      payload: { approvalId: id },
    })
    ElMessage.success('Approved')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : 'Approve failed')
  }
}

async function reject(id: string): Promise<void> {
  try {
    await rejectAction(id, 'Rejected from console')
    items.value = items.value.map((a) => (a.id === id ? { ...a, status: 'REJECTED' } : a))
    run.ingest({
      id: `appr-no-${Date.now()}`,
      runId: run.state.runId || items.value.find((a) => a.id === id)?.runId || '',
      step: 0,
      type: 'approval.rejected',
      timestamp: new Date().toISOString(),
      payload: { approvalId: id },
    })
    ElMessage.success('Rejected')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : 'Reject failed')
  }
}
</script>

<template>
  <div class="h-full overflow-auto">
    <div class="mx-auto flex max-w-5xl flex-col gap-8 px-6 py-8">
      <header>
        <h1 class="text-xl font-semibold tracking-tight">Approvals</h1>
        <p class="mt-1 text-sm text-mute">High and critical actions pause the agent until someone decides.</p>
      </header>
      <section>
        <div class="mb-3">
          <h2 class="text-sm font-medium">Queue</h2>
          <p class="mt-0.5 text-xs leading-5 text-mute">Approve to continue the run, or reject to stop that action.</p>
        </div>
        <ApprovalCenter :items="items" :loading="loading" :error="error" @approve="approve" @reject="reject" @open="open" />
      </section>
    </div>
    <ApprovalDetailDrawer v-model="drawer" :approval="current" @approve="approve" @reject="reject" />
  </div>
</template>
