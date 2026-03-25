import { createRouter, createWebHashHistory } from 'vue-router'
import { isAuthenticated } from './stores/app.js'
import { navStore } from './stores/navigation.js'

const routes = [
  { path: '/', redirect: '/overview' },
  { path: '/login', name: 'login', component: () => import('./views/Login.vue'), meta: { title: '登录', public: true } },
  { path: '/overview', name: 'overview', component: () => import('./views/Overview.vue'), meta: { title: '概览', icon: 'bar-chart', group: 'im' } },
  { path: '/custom', name: 'custom-dashboard', component: () => import('./views/CustomDashboard.vue'), meta: { title: '自定义大屏', icon: 'palette', group: 'dashboard' } },
  { path: '/upstream', name: 'upstream', component: () => import('./views/Upstream.vue'), meta: { title: '上游', icon: 'link', group: 'im' } },
  { path: '/routes', name: 'routes', component: () => import('./views/Routes.vue'), meta: { title: '路由', icon: 'git-branch', group: 'im' } },
  { path: '/rules', name: 'rules', component: () => import('./views/Rules.vue'), meta: { title: '规则', icon: 'shield', group: 'im' } },
  { path: '/audit', name: 'audit', component: () => import('./views/Audit.vue'), meta: { title: '审计', icon: 'clipboard', group: 'im' } },
  { path: '/user-profiles', name: 'user-profiles', component: () => import('./views/UserProfiles.vue'), meta: { title: '用户画像', icon: 'user-scan', group: 'threat' } },
  { path: '/user-profiles/:id', name: 'user-detail', component: () => import('./views/UserDetail.vue'), meta: { title: '用户详情', icon: 'user-scan', group: 'threat' } },
  { path: '/llm', name: 'llm', component: () => import('./views/LLMOverview.vue'), meta: { title: 'LLM 概览', icon: 'brain', group: 'llm' } },
  { path: '/llm-rules', name: 'llm-rules', component: () => import('./views/LLMRules.vue'), meta: { title: 'LLM 规则', icon: 'shield', group: 'llm' } },
  { path: '/llm-targets', name: 'llm-targets', component: () => import('./views/LLMTargets.vue'), meta: { title: 'LLM 目标', icon: 'globe', group: 'llm' } },
  { path: '/agent', name: 'agent', component: () => import('./views/AgentBehavior.vue'), meta: { title: 'Agent 行为', icon: 'bot', group: 'llm' } },
  { path: '/behavior', name: 'behavior', component: () => import('./views/BehaviorProfile.vue'), meta: { title: '行为画像', icon: 'behavior', group: 'threat' } },
  { path: '/sessions', name: 'sessions', component: () => import('./views/SessionReplay.vue'), meta: { title: '会话回放', icon: 'film', group: 'llm' } },
  { path: '/sessions/:traceId', name: 'session-detail', component: () => import('./views/SessionDetail.vue'), meta: { title: '会话详情', icon: 'film', group: 'llm' } },
  { path: '/prompts', name: 'prompts', component: () => import('./views/PromptTracker.vue'), meta: { title: 'Prompt 追踪', icon: 'edit', group: 'llm' } },
  { path: '/ab-testing', name: 'ab-testing', component: () => import('./views/ABTesting.vue'), meta: { title: 'A/B 测试', icon: 'split', group: 'llm' } },
  { path: '/honeypot', name: 'honeypot', component: () => import('./views/Honeypot.vue'), meta: { title: 'Agent 蜜罐', icon: 'target', group: 'threat' } },
  { path: '/attack-chains', name: 'attack-chains', component: () => import('./views/AttackChain.vue'), meta: { title: '攻击链分析', icon: 'link', group: 'threat' } },
  { path: '/monitor', name: 'monitor', component: () => import('./views/Monitor.vue'), meta: { title: '监控', icon: 'zap', group: 'system' } },
  { path: '/anomaly', name: 'anomaly', component: () => import('./views/AnomalyDetection.vue'), meta: { title: '异常检测', icon: 'bar-chart', group: 'threat' } },
  { path: '/reports', name: 'reports', component: () => import('./views/Reports.vue'), meta: { title: '报告', icon: 'file-text', group: 'govern' } },
  { path: '/tenants', name: 'tenants', component: () => import('./views/Tenants.vue'), meta: { title: '租户', icon: 'building', group: 'govern' } },
  { path: '/redteam', name: 'redteam', component: () => import('./views/RedTeam.vue'), meta: { title: '红队测试', icon: 'crosshair', group: 'govern' } },
  { path: '/leaderboard', name: 'leaderboard', component: () => import('./views/Leaderboard.vue'), meta: { title: '排行榜', icon: 'trophy', group: 'govern' } },
  { path: '/envelopes', name: 'envelopes', component: () => import('./views/Envelopes.vue'), meta: { title: '执行信封', icon: 'lock', group: 'govern' } },
  { path: '/events', name: 'events', component: () => import('./views/EventBus.vue'), meta: { title: '事件总线', icon: 'radio', group: 'govern' } },
  { path: '/evolution', name: 'evolution', component: () => import('./views/Evolution.vue'), meta: { title: '自进化', icon: 'dna', group: 'govern' } },
  { path: '/singularity', name: 'singularity', component: () => import('./views/Singularity.vue'), meta: { title: '奇点蜜罐', icon: 'orbit', group: 'threat' } },
  { path: '/ops', name: 'ops', component: () => import('./views/Operations.vue'), meta: { title: '运维', icon: 'wrench', group: 'system' } },
  { path: '/users', name: 'users', component: () => import('./views/Users.vue'), meta: { title: '用户管理', icon: 'users', group: 'system' } },
  { path: '/settings', name: 'settings', component: () => import('./views/Settings.vue'), meta: { title: '设置', icon: 'settings', group: 'system' } },
  { path: '/semantic', name: 'semantic', component: () => import('./views/SemanticDetector.vue'), meta: { title: '语义检测', icon: 'microscope', group: 'threat' } },
  { path: '/tools', name: 'tools', component: () => import('./views/ToolPolicy.vue'), meta: { title: '工具策略', icon: 'wrench', group: 'llm' } },
  { path: '/path-policy', name: 'path-policy', component: () => import('./views/PathPolicy.vue'), meta: { title: '路径策略', icon: 'git-branch', group: 'govern' } },
  { path: '/taint', name: 'taint', component: () => import('./views/TaintTracker.vue'), meta: { title: '污染追踪', icon: 'biohazard', group: 'threat' } },
  { path: '/cache', name: 'cache', component: () => import('./views/LLMCache.vue'), meta: { title: '响应缓存', icon: 'save', group: 'llm' } },
  { path: '/gateway', name: 'gateway', component: () => import('./views/APIGateway.vue'), meta: { title: 'API 网关', icon: 'door', group: 'system' } },
  { path: '/gateway-monitor', name: 'gateway-monitor', component: () => import('./views/GatewayMonitor.vue'), meta: { title: 'Gateway 监控', icon: 'activity', group: 'system' } },
  { path: '/counterfactual', name: 'counterfactual', component: () => import('./views/Counterfactual.vue'), meta: { title: '反事实验证', icon: 'microscope', group: 'govern' } },
  { path: '/bigscreen', name: 'bigscreen', component: () => import('./views/BigScreen.vue'), meta: { title: '态势大屏', icon: 'layout', public: true, bigscreen: true } },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

// v14.1: 路由守卫 — 未认证时跳转登录页; v15.0: Tab 自动同步
router.beforeEach(async (to) => {
  // Tab 自动同步：根据目标路由自动切换 Tab
  if (to.name && !to.meta.public && !to.meta.bigscreen) {
    const tab = navStore.getTabForRoute(to.name)
    if (tab !== navStore.activeTab) {
      navStore.setTab(tab)
    }
  }

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
