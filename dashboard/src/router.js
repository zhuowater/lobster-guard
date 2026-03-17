import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/overview' },
  { path: '/overview', name: 'overview', component: () => import('./views/Overview.vue'), meta: { title: '概览', icon: '📊' } },
  { path: '/upstream', name: 'upstream', component: () => import('./views/Upstream.vue'), meta: { title: '上游', icon: '🔗' } },
  { path: '/routes', name: 'routes', component: () => import('./views/Routes.vue'), meta: { title: '路由', icon: '🗺️' } },
  { path: '/rules', name: 'rules', component: () => import('./views/Rules.vue'), meta: { title: '规则', icon: '🛡️' } },
  { path: '/audit', name: 'audit', component: () => import('./views/Audit.vue'), meta: { title: '审计', icon: '📋' } },
  { path: '/monitor', name: 'monitor', component: () => import('./views/Monitor.vue'), meta: { title: '监控', icon: '⚡' } },
  { path: '/ops', name: 'ops', component: () => import('./views/Operations.vue'), meta: { title: '运维', icon: '🔧' } },
  { path: '/settings', name: 'settings', component: () => import('./views/Settings.vue'), meta: { title: '设置', icon: '⚙️' } },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router
