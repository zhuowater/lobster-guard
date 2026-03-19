import { reactive } from 'vue'

const TABS = {
  overview: {
    label: '安全总览',
    icon: '🛡️',
    routes: ['overview', 'custom-dashboard', 'anomaly', 'monitor']
  },
  threat: {
    label: '威胁中心',
    icon: '🔍',
    routes: ['audit', 'sessions', 'session-detail', 'attack-chains', 'user-profiles', 'user-detail', 'behavior', 'honeypot', 'singularity', 'prompts', 'taint', 'redteam', 'semantic']
  },
  policy: {
    label: '策略引擎',
    icon: '⚙️',
    routes: ['rules', 'llm-rules', 'tools', 'evolution', 'cache', 'gateway', 'routes', 'envelopes', 'events', 'ab-testing', 'upstream']
  },
  ops: {
    label: '运营管理',
    icon: '📊',
    routes: ['reports', 'leaderboard', 'tenants', 'users', 'llm', 'ops', 'settings']
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
      if (config.routes.includes(routeName)) return tab
    }
    return 'overview'
  },

  getCurrentRoutes() {
    return TABS[this.activeTab]?.routes || []
  },

  get tabs() { return TABS }
})
