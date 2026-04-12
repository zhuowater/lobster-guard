import { reactive } from 'vue'

const TABS = {
  overview: {
    label: '安全总览',
    icon: 'shield',
    routes: ['overview', 'custom-dashboard', 'anomaly', 'monitor']
  },
  threat: {
    label: '威胁中心',
    icon: 'search',
    groups: [
      { label: 'IM 安全', routes: ['audit', 'sessions', 'session-detail', 'attack-chains', 'user-profiles', 'user-detail', 'behavior'] },
      { label: 'LLM 安全', routes: ['agent-profiles', 'semantic', 'prompts', 'taint'] },
      { label: '对抗测试', routes: ['honeypot', 'singularity', 'redteam'] }
    ]
  },
  policy: {
    label: '策略引擎',
    icon: 'settings',
    groups: [
      { label: 'IM 策略', routes: ['rules', 'routes', 'upstream'] },
      { label: 'LLM 策略', routes: ['llm-rules', 'llm-targets', 'tools', 'cache', 'plan-compiler', 'agent', 'ab-testing'] },
      { label: '治理引擎', routes: ['path-policy', 'source-classifier', 'counterfactual', 'capability', 'deviations', 'ifc'] },
      { label: '系统策略', routes: ['evolution', 'envelopes', 'events'] }
    ]
  },
  ops: {
    label: '运营管理',
    icon: 'bar-chart',
    routes: ['reports', 'leaderboard', 'tenants', 'apikeys', 'users', 'llm', 'ops', 'settings', 'gateway', 'gateway-monitor']
  }
}

export const navStore = reactive({
  activeTab: localStorage.getItem('lg-nav-tab') || 'overview',
  mode: localStorage.getItem('lg-nav-mode') || 'classic', // 'classic' | 'narrative'

  setTab(tab) {
    this.activeTab = tab
    localStorage.setItem('lg-nav-tab', tab)
  },

  setMode(mode) {
    this.mode = mode
    localStorage.setItem('lg-nav-mode', mode)
  },

  getTabForRoute(routeName) {
    for (const [tab, config] of Object.entries(TABS)) {
      if (config.routes && config.routes.includes(routeName)) return tab
      if (config.groups) {
        for (const group of config.groups) {
          if (group.routes.includes(routeName)) return tab
        }
      }
    }
    return 'overview'
  },

  getCurrentRoutes() {
    const tab = TABS[this.activeTab]
    if (!tab) return []
    if (tab.routes) return tab.routes
    if (tab.groups) return tab.groups.flatMap(g => g.routes)
    return []
  },

  getCurrentGroups() {
    const tab = TABS[this.activeTab]
    if (!tab) return null
    if (tab.groups) return tab.groups
    return null
  },

  get tabs() { return TABS }
})
