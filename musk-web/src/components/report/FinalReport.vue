<script setup lang="ts">
import type { Report } from '@/types'
import ConfidenceBar from '@/components/decision/ConfidenceBar.vue'

defineProps<{ report: Report }>()
const emit = defineEmits<{ evidence: [id: string] }>()
</script>

<template>
  <article class="space-y-4 text-sm">
    <section>
      <h3 class="text-xs uppercase text-mute">Executive Summary</h3>
      <p class="mt-1 leading-relaxed">{{ report.summary }}</p>
    </section>
    <section>
      <h3 class="text-xs uppercase text-mute">Key Metrics</h3>
      <div class="mt-1 grid grid-cols-2 gap-2 md:grid-cols-4">
        <div v-for="(v, k) in report.metrics" :key="k" class="rounded bg-raised px-2 py-1">
          <div class="text-[11px] uppercase text-mute">{{ k }}</div>
          <div class="tabular-nums">{{ v }}</div>
        </div>
      </div>
    </section>
    <section>
      <h3 class="text-xs uppercase text-mute">Anomalies</h3>
      <ul class="mt-1 space-y-1">
        <li v-for="(a, i) in report.anomalies" :key="i">
          <button class="text-left hover:underline" type="button" @click="emit('evidence', a.evidenceId)">
            {{ a.entity }} {{ a.name }} {{ a.metric }} {{ (a.changePct * 100).toFixed(1) }}%
          </button>
        </li>
      </ul>
    </section>
    <section>
      <h3 class="text-xs uppercase text-mute">Root Causes</h3>
      <ul class="mt-1 space-y-2">
        <li v-for="(c, i) in report.rootCauses" :key="i">
          {{ c.statement }}
          <div class="text-[11px] text-accent">
            <button v-for="id in c.evidenceIds" :key="id" class="mr-2 underline" type="button" @click="emit('evidence', id)">{{ id }}</button>
          </div>
        </li>
      </ul>
    </section>
    <section>
      <h3 class="text-xs uppercase text-mute">Evidence</h3>
      <ul class="mt-1 space-y-1">
        <li v-for="e in report.evidence" :key="e.id">
          <button class="text-left text-xs hover:underline" type="button" @click="emit('evidence', e.id)">
            {{ e.id }} — {{ e.description }}
          </button>
        </li>
      </ul>
    </section>
    <section>
      <h3 class="text-xs uppercase text-mute">Recommendations</h3>
      <ul class="mt-1 list-disc space-y-1 pl-4">
        <li v-for="(r, i) in report.recommendations" :key="i">
          {{ r.action }}
          <button
            v-for="id in r.evidenceIds"
            :key="id"
            class="ml-2 text-[11px] text-accent underline"
            type="button"
            @click="emit('evidence', id)"
          >
            {{ id }}
          </button>
        </li>
      </ul>
    </section>
    <section>
      <h3 class="mb-1 text-xs uppercase text-mute">Confidence</h3>
      <ConfidenceBar :value="report.confidence" />
    </section>
  </article>
</template>
