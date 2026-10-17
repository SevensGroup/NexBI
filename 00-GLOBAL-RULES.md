# NexBI

Project: NexBI
Backend: musk-back
Frontend: musk-web

Frontend stack:
Vue 3 + TypeScript + Vite + Vue Router + Pinia + VueUse + Tailwind CSS + Element Plus + ECharts + Vue Flow + Axios + SSE

Product positioning:
Autonomous Agent Operating Console, rather than a traditional Chat UI.

Agent lifecycle:
Goal Understanding → Planning → Decision → Tool → Observation → Reflection → Re-Decision → Multi-Agent → Evidence → Final Report

Do not show the full Chain of Thought. Show only:
Reasoning Summary / Decision / Evidence / Confidence / Next Action / Observation / Reflection Summary

Code conventions:
- TypeScript strict
- Composition API + script setup
- Put APIs in services
- Put state in Pinia
- Put types in types
- Encapsulate SSE on its own
- Agent events must pass through the Store/Reducer
- Components must not mutate core Agent state directly
- Every page handles loading, empty, and error states
- Keep mocks isolated so they can be swapped for the real API
- Do not hardcode production data
- Keep the UI professional, restrained, and information-dense
- Do not add gamification, heavy gradients, or heavy animation

Pages:
/dashboard /agents /agents/:id /runs /runs/:id /tasks /memory /semantic /approvals /settings

Core workspace:
Left: Agents/Tasks
Center: Execution Timeline
Right: Context/Plan/Evidence/Memory
Bottom: Agent Input

States:
UNDERSTANDING / PLANNING / DECIDING / EXECUTING / OBSERVING / REFLECTING / WAITING_APPROVAL / COMPLETED / FAILED
