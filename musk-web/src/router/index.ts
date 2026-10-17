import { createRouter, createWebHistory, type RouteLocationRaw } from 'vue-router'
import { isAuthed } from '@/auth/session'
import { useAgentRunStore } from '@/stores/agentRun'

function openRun(view: 'board' | 'agents' | 'tasks'): RouteLocationRaw {
  const run = useAgentRunStore()
  const id = run.state.runId || run.history[0]?.id
  if (!id) return { path: '/dashboard' }
  if (view === 'board') return { path: `/runs/${id}` }
  return { path: `/runs/${id}`, query: { view } }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/pages/LoginPage.vue'), meta: { public: true } },
    { path: '/bi/:code', name: 'board', component: () => import('@/pages/BoardPage.vue'), meta: { public: true } },
    { path: '/', redirect: '/dashboard' },
    { path: '/dashboard', name: 'dashboard', component: () => import('@/pages/DashboardPage.vue') },
    { path: '/run', name: 'run', component: () => import('@/pages/RunPage.vue') },
    { path: '/agents', name: 'agents', redirect: () => openRun('agents') },
    {
      path: '/agents/advertising-analyst',
      name: 'advertising-analyst',
      component: () => import('@/pages/AdvertisingAnalystPage.vue'),
    },
    { path: '/agents/:id', name: 'agent-workspace', component: () => import('@/pages/AgentWorkspacePage.vue') },
    { path: '/runs', redirect: '/run' },
    { path: '/runs/:id', name: 'run-detail', component: () => import('@/pages/RunDetailPage.vue') },
    { path: '/tasks', name: 'tasks', redirect: () => openRun('tasks') },
    { path: '/memory', redirect: { path: '/settings', query: { section: 'memory' } } },
    { path: '/semantic', redirect: { path: '/settings', query: { section: 'semantic' } } },
    { path: '/tools', redirect: { path: '/settings', query: { section: 'tools' } } },
    { path: '/approvals', name: 'approvals', component: () => import('@/pages/ApprovalsPage.vue') },
    { path: '/settings', name: 'settings', component: () => import('@/pages/SettingsPage.vue') },
  ],
})

router.beforeEach((to) => {
  if (to.meta.public) {
    if (to.name === 'login' && isAuthed()) return { path: '/dashboard' }
    return true
  }
  if (!isAuthed()) return { name: 'login', query: { redirect: to.fullPath } }
  return true
})

export default router
