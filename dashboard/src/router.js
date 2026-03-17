import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/overview' },
  { path: '/overview', name: 'overview', component: () => import('./views/Overview.vue'), meta: { title: '概览', icon: '📊', group: 'im' } },
  { path: '/upstream', name: 'upstream', component: () => import('./views/Upstream.vue'), meta: { title: '上游', icon: '🔗', group: 'im' } },
  { path: '/routes', name: 'routes', component: () => import('./views/Routes.vue'), meta: { title: '路由', icon: '🗺️', group: 'im' } },
  { path: '/rules', name: 'rules', component: () => import('./views/Rules.vue'), meta: { title: '规则', icon: '🛡️', group: 'im' } },
  { path: '/audit', name: 'audit', component: () => import('./views/Audit.vue'), meta: { title: '审计', icon: '📋', group: 'im' } },
  { path: '/user-profiles', name: 'user-profiles', component: () => import('./views/UserProfiles.vue'), meta: { title: '用户画像', icon: '🕵️', group: 'im' } },
  { path: '/user-profiles/:id', name: 'user-detail', component: () => import('./views/UserDetail.vue'), meta: { title: '用户详情', icon: '🕵️', group: 'im' } },
  { path: '/llm', name: 'llm', component: () => import('./views/LLMOverview.vue'), meta: { title: 'LLM 概览', icon: '🧠', group: 'llm' } },
  { path: '/llm-rules', name: 'llm-rules', component: () => import('./views/LLMRules.vue'), meta: { title: 'LLM 规则', icon: '🛡️', group: 'llm' } },
  { path: '/agent', name: 'agent', component: () => import('./views/AgentBehavior.vue'), meta: { title: 'Agent 行为', icon: '🤖', group: 'llm' } },
  { path: '/monitor', name: 'monitor', component: () => import('./views/Monitor.vue'), meta: { title: '监控', icon: '⚡', group: 'system' } },
  { path: '/ops', name: 'ops', component: () => import('./views/Operations.vue'), meta: { title: '运维', icon: '🔧', group: 'system' } },
  { path: '/settings', name: 'settings', component: () => import('./views/Settings.vue'), meta: { title: '设置', icon: '⚙️', group: 'system' } },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router
