import { createRouter, createWebHashHistory } from 'vue-router'
import { isAuthenticated } from './stores/app.js'

const routes = [
  { path: '/', redirect: '/overview' },
  { path: '/login', name: 'login', component: () => import('./views/Login.vue'), meta: { title: '登录', public: true } },
  { path: '/overview', name: 'overview', component: () => import('./views/Overview.vue'), meta: { title: '概览', icon: '📊', group: 'im' } },
  { path: '/custom', name: 'custom-dashboard', component: () => import('./views/CustomDashboard.vue'), meta: { title: '自定义大屏', icon: '🎨', group: 'dashboard' } },
  { path: '/upstream', name: 'upstream', component: () => import('./views/Upstream.vue'), meta: { title: '上游', icon: '🔗', group: 'im' } },
  { path: '/routes', name: 'routes', component: () => import('./views/Routes.vue'), meta: { title: '路由', icon: '🗺️', group: 'im' } },
  { path: '/rules', name: 'rules', component: () => import('./views/Rules.vue'), meta: { title: '规则', icon: '🛡️', group: 'im' } },
  { path: '/audit', name: 'audit', component: () => import('./views/Audit.vue'), meta: { title: '审计', icon: '📋', group: 'im' } },
  { path: '/user-profiles', name: 'user-profiles', component: () => import('./views/UserProfiles.vue'), meta: { title: '用户画像', icon: '🕵️', group: 'threat' } },
  { path: '/user-profiles/:id', name: 'user-detail', component: () => import('./views/UserDetail.vue'), meta: { title: '用户详情', icon: '🕵️', group: 'threat' } },
  { path: '/llm', name: 'llm', component: () => import('./views/LLMOverview.vue'), meta: { title: 'LLM 概览', icon: '🧠', group: 'llm' } },
  { path: '/llm-rules', name: 'llm-rules', component: () => import('./views/LLMRules.vue'), meta: { title: 'LLM 规则', icon: '🛡️', group: 'llm' } },
  { path: '/agent', name: 'agent', component: () => import('./views/AgentBehavior.vue'), meta: { title: 'Agent 行为', icon: '🤖', group: 'llm' } },
  { path: '/behavior', name: 'behavior', component: () => import('./views/BehaviorProfile.vue'), meta: { title: '行为画像', icon: '🧠', group: 'threat' } },
  { path: '/sessions', name: 'sessions', component: () => import('./views/SessionReplay.vue'), meta: { title: '会话回放', icon: '🎬', group: 'llm' } },
  { path: '/sessions/:traceId', name: 'session-detail', component: () => import('./views/SessionDetail.vue'), meta: { title: '会话详情', icon: '🎬', group: 'llm' } },
  { path: '/prompts', name: 'prompts', component: () => import('./views/PromptTracker.vue'), meta: { title: 'Prompt 追踪', icon: '📝', group: 'llm' } },
  { path: '/ab-testing', name: 'ab-testing', component: () => import('./views/ABTesting.vue'), meta: { title: 'A/B 测试', icon: '🔬', group: 'llm' } },
  { path: '/honeypot', name: 'honeypot', component: () => import('./views/Honeypot.vue'), meta: { title: 'Agent 蜜罐', icon: '🍯', group: 'threat' } },
  { path: '/attack-chains', name: 'attack-chains', component: () => import('./views/AttackChain.vue'), meta: { title: '攻击链分析', icon: '🔗', group: 'threat' } },
  { path: '/monitor', name: 'monitor', component: () => import('./views/Monitor.vue'), meta: { title: '监控', icon: '⚡', group: 'system' } },
  { path: '/anomaly', name: 'anomaly', component: () => import('./views/AnomalyDetection.vue'), meta: { title: '异常检测', icon: '📊', group: 'threat' } },
  { path: '/reports', name: 'reports', component: () => import('./views/Reports.vue'), meta: { title: '报告', icon: '📄', group: 'govern' } },
  { path: '/tenants', name: 'tenants', component: () => import('./views/Tenants.vue'), meta: { title: '租户', icon: '🏢', group: 'govern' } },
  { path: '/redteam', name: 'redteam', component: () => import('./views/RedTeam.vue'), meta: { title: '红队测试', icon: '🎯', group: 'govern' } },
  { path: '/leaderboard', name: 'leaderboard', component: () => import('./views/Leaderboard.vue'), meta: { title: '排行榜', icon: '🏆', group: 'govern' } },
  { path: '/envelopes', name: 'envelopes', component: () => import('./views/Envelopes.vue'), meta: { title: '执行信封', icon: '🔐', group: 'govern' } },
  { path: '/events', name: 'events', component: () => import('./views/EventBus.vue'), meta: { title: '事件总线', icon: '📡', group: 'govern' } },
  { path: '/evolution', name: 'evolution', component: () => import('./views/Evolution.vue'), meta: { title: '自进化', icon: '🧬', group: 'govern' } },
  { path: '/singularity', name: 'singularity', component: () => import('./views/Singularity.vue'), meta: { title: '奇点蜜罐', icon: '🌀', group: 'threat' } },
  { path: '/ops', name: 'ops', component: () => import('./views/Operations.vue'), meta: { title: '运维', icon: '🔧', group: 'system' } },
  { path: '/users', name: 'users', component: () => import('./views/Users.vue'), meta: { title: '用户管理', icon: '👥', group: 'system' } },
  { path: '/settings', name: 'settings', component: () => import('./views/Settings.vue'), meta: { title: '设置', icon: '⚙️', group: 'system' } },
  { path: '/semantic', name: 'semantic', component: () => import('./views/SemanticDetector.vue'), meta: { title: '语义检测', icon: '🔬', group: 'threat' } },
  { path: '/tools', name: 'tools', component: () => import('./views/ToolPolicy.vue'), meta: { title: '工具策略', icon: '🔧', group: 'llm' } },
  { path: '/taint', name: 'taint', component: () => import('./views/TaintTracker.vue'), meta: { title: '污染追踪', icon: '☣️', group: 'threat' } },
  { path: '/cache', name: 'cache', component: () => import('./views/LLMCache.vue'), meta: { title: '响应缓存', icon: '💾', group: 'llm' } },
  { path: '/gateway', name: 'gateway', component: () => import('./views/APIGateway.vue'), meta: { title: 'API 网关', icon: '🚪', group: 'system' } },
  { path: '/bigscreen', name: 'bigscreen', component: () => import('./views/BigScreen.vue'), meta: { title: '态势大屏', icon: '🖥', public: true, bigscreen: true } },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

// v14.1: 路由守卫 — 未认证时跳转登录页
router.beforeEach(async (to) => {
  // 公开页面不需要认证
  if (to.meta.public) return true

  // 检查本地是否有 token
  if (isAuthenticated()) return true

  // 没有 token → 检查后端 auth 是否启用
  try {
    const res = await fetch(location.origin + '/api/v1/auth/check')
    const data = await res.json()

    if (!data.auth_enabled) {
      // auth 未启用，检查旧 token
      if (data.authenticated) return true
      // 旧 token 也没有，但 auth 未启用时也放行（让页面自己处理 token 输入）
      return true
    }

    // auth 已启用但未认证 → 跳转登录
    if (!data.authenticated) {
      return { name: 'login' }
    }
    return true
  } catch {
    // 网络错误，放行让页面自己处理
    return true
  }
})

export default router
