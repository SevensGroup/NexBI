import { createRouter, createWebHistory } from 'vue-router'
import { isAuthed } from '@/auth/session'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/pages/LoginPage.vue'), meta: { public: true } },
    { path: '/bi/:code', name: 'board', component: () => import('@/pages/BoardPage.vue'), meta: { public: true } },
    { path: '/', redirect: '/dashboard' },
    { path: '/dashboard', name: 'dashboard', component: () => import('@/pages/DashboardPage.vue') },
    { path: '/agents', name: 'agents', component: () => import('@/pages/AgentsPage.vue') },
    {
      path: '/agents/advertising-analyst',
      name: 'advertising-analyst',
      component: () => import('@/pages/AdvertisingAnalystPage.vue'),
    },
    { path: '/agents/:id', name: 'agent-workspace', component: () => import('@/pages/AgentWorkspacePage.vue') },
    { path: '/runs', name: 'runs', component: () => import('@/pages/RunsPage.vue') },
    { path: '/runs/:id', name: 'run-detail', component: () => import('@/pages/RunDetailPage.vue') },
    { path: '/tasks', name: 'tasks', component: () => import('@/pages/TasksPage.vue') },
    { path: '/memory', name: 'memory', component: () => import('@/pages/MemoryPage.vue') },
    { path: '/semantic', name: 'semantic', component: () => import('@/pages/SemanticPage.vue') },
    { path: '/tools', name: 'tools', component: () => import('@/pages/ToolsPage.vue') },
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
