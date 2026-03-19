<template>
  <nav class="sidebar" :class="{ collapsed: appState.sidebarCollapsed, 'mobile-open': mobileOpen }">
    <!-- 严格模式横幅 (v11.1) -->
    <div class="strict-banner" v-if="strictMode && !appState.sidebarCollapsed">⚠️ 严格模式已启用</div>
    <div class="sidebar-brand">
      <span class="sidebar-logo" :class="{'strict-logo': strictMode}">🦞</span>
      <div class="sidebar-brand-text">
        <div class="sidebar-brand-title">龙虾卫士</div>
        <div class="sidebar-brand-sub">Lobster Guard</div>
      </div>
    </div>
    <div class="sidebar-nav">
      <!-- IM 安全 -->
      <div class="nav-group-label" v-if="!appState.sidebarCollapsed">IM 安全</div>
      <router-link
        v-for="item in imItems" :key="item.path"
        :to="item.path"
        class="nav-item"
        :class="{ active: $route.path === item.path || ($route.path.startsWith(item.path + '/') && item.path.length > 1) }"
        @click="$emit('closeMobile')"
        :title="appState.sidebarCollapsed ? item.label : ''"
      >
        <Icon :name="item.icon" :size="20" class="nav-icon" />
        <span class="nav-label">{{ item.label }}</span>
      </router-link>

      <!-- LLM 安全 (仅启用时显示) -->
      <template v-if="llmEnabled">
        <div class="nav-divider"></div>
        <div class="nav-group-label" v-if="!appState.sidebarCollapsed">LLM 安全</div>
        <router-link
          v-for="item in llmItems" :key="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ active: $route.path === item.path || $route.path.startsWith(item.path + '/') }"
          @click="$emit('closeMobile')"
          :title="appState.sidebarCollapsed ? item.label : ''"
        >
          <Icon :name="item.icon" :size="20" class="nav-icon" />
          <span class="nav-label">{{ item.label }}</span>
        </router-link>
      </template>

      <!-- 威胁分析 -->
      <template v-if="true">
        <div class="nav-divider"></div>
        <div class="nav-group-label" v-if="!appState.sidebarCollapsed">威胁分析</div>
        <router-link
          v-for="item in threatItems" :key="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ active: $route.path === item.path || $route.path.startsWith(item.path + '/') }"
          @click="$emit('closeMobile')"
          :title="appState.sidebarCollapsed ? item.label : ''"
        >
          <Icon :name="item.icon" :size="20" class="nav-icon" />
          <span class="nav-label">{{ item.label }}</span>
        </router-link>
      </template>

      <!-- 安全治理 -->
      <template v-if="true">
        <div class="nav-divider"></div>
        <div class="nav-group-label" v-if="!appState.sidebarCollapsed">安全治理</div>
        <router-link
          v-for="item in governItems" :key="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ active: $route.path === item.path || $route.path.startsWith(item.path + '/') }"
          @click="$emit('closeMobile')"
          :title="appState.sidebarCollapsed ? item.label : ''"
        >
          <Icon :name="item.icon" :size="20" class="nav-icon" />
          <span class="nav-label">{{ item.label }}</span>
        </router-link>
      </template>

      <!-- 系统 -->
      <div class="nav-divider"></div>
      <div class="nav-group-label" v-if="!appState.sidebarCollapsed">系统</div>
      <router-link
        v-for="item in sysItems" :key="item.path"
        :to="item.path"
        class="nav-item"
        :class="{ active: $route.path === item.path }"
        @click="$emit('closeMobile')"
        :title="appState.sidebarCollapsed ? item.label : ''"
      >
        <Icon :name="item.icon" :size="20" class="nav-icon" />
        <span class="nav-label">{{ item.label }}</span>
      </router-link>
    </div>
    <div class="sidebar-footer">
      <!-- 态势大屏 + 自定义大屏 -->
      <div class="bigscreen-btns" v-if="!appState.sidebarCollapsed">
        <router-link to="/bigscreen" class="bigscreen-btn" title="态势大屏">🖥 大屏</router-link>
        <router-link to="/custom" class="bigscreen-btn" title="自定义大屏">🎨 自定义</router-link>
      </div>
      <div class="bigscreen-btns" v-else>
        <router-link to="/bigscreen" class="bigscreen-btn bigscreen-btn-icon" title="态势大屏">🖥</router-link>
        <router-link to="/custom" class="bigscreen-btn bigscreen-btn-icon" title="自定义大屏">🎨</router-link>
      </div>
      <!-- 严格模式开关 (v11.1) -->
      <div class="strict-toggle" v-if="!appState.sidebarCollapsed">
        <span class="strict-label">🛡️ 严格模式</span>
        <label class="toggle-switch">
          <input type="checkbox" :checked="strictMode" @change="toggleStrictMode">
          <span class="toggle-slider" :class="{'toggle-active': strictMode}"></span>
        </label>
      </div>
      <div class="sidebar-version">{{ appState.version }}</div>
      <div class="sidebar-status">
        <span class="dot dot-sm" :class="dotClass"></span>
        <span class="sidebar-status-text">{{ statusText }}</span>
      </div>
    </div>
    <!-- 严格模式切换 toast -->
    <Transition name="toast-fade">
      <div class="strict-toast" v-if="strictToastVisible">{{ strictToast }}</div>
    </Transition>
    <button class="sidebar-toggle" @click="toggleSidebar" :title="appState.sidebarCollapsed ? '展开' : '折叠'">
      <svg v-if="appState.sidebarCollapsed" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
      <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
    </button>
  </nav>
</template>

<script setup>
import { inject, computed, ref, onMounted } from 'vue'
import { toggleSidebar } from '../stores/app.js'
import { api, apiPost } from '../api.js'
import Icon from './Icon.vue'

defineProps({ mobileOpen: Boolean })
defineEmits(['closeMobile'])

const appState = inject('appState')

const llmEnabled = ref(false)
const strictMode = ref(false)

async function checkLLMStatus() {
  try { const d = await api('/api/v1/llm/status'); llmEnabled.value = d.enabled === true } catch { llmEnabled.value = false }
}

async function loadStrictMode() {
  try { const d = await api('/api/v1/system/strict-mode'); strictMode.value = d.enabled === true } catch { strictMode.value = false }
}

const strictToast = ref('')
const strictToastVisible = ref(false)

async function toggleStrictMode() {
  const newVal = !strictMode.value
  const msg = newVal ? '确定要启用严格模式吗？\n\n所有规则将切换为"拦截"模式，影子模式规则也将生效。' : '确定要关闭严格模式吗？\n\n所有规则将恢复到之前的状态。'
  if (!confirm(msg)) return
  try {
    const res = await apiPost('/api/v1/system/strict-mode', { enabled: newVal })
    strictMode.value = newVal
    const imCount = res.affected_im_rules || 0
    const llmCount = res.affected_llm_rules || 0
    if (newVal) {
      strictToast.value = `已切换为严格模式，${imCount} 条 IM 规则 + ${llmCount} 条 LLM 规则已设为拦截`
    } else {
      strictToast.value = `已关闭严格模式，${imCount} 条 IM 规则 + ${llmCount} 条 LLM 规则已恢复`
    }
    strictToastVisible.value = true
    setTimeout(() => { strictToastVisible.value = false }, 4000)
  } catch { alert('切换失败') }
}

onMounted(() => { checkLLMStatus(); loadStrictMode() })

// IM 安全导航项 (5项)
const imItems = [
  { path: '/overview', label: '概览', icon: 'grid' },
  { path: '/upstream', label: '上游', icon: 'server' },
  { path: '/routes', label: '路由', icon: 'git-branch' },
  { path: '/rules', label: '规则', icon: 'shield' },
  { path: '/audit', label: '审计', icon: 'file-text' },
]

// LLM 安全导航项 (6项)
const llmItems = [
  { path: '/llm', label: 'LLM 概览', icon: 'brain' },
  { path: '/llm-rules', label: 'LLM 规则', icon: 'shield-check' },
  { path: '/agent', label: 'Agent 行为', icon: 'bot' },
  { path: '/sessions', label: '会话回放', icon: 'clapperboard' },
  { path: '/prompts', label: 'Prompt 追踪', icon: 'file-check' },
  { path: '/ab-testing', label: 'A/B 测试', icon: 'split' },
]

// 威胁分析导航项 (6项)
const threatItems = [
  { path: '/user-profiles', label: '用户画像', icon: 'user-scan' },
  { path: '/behavior', label: '行为画像', icon: 'behavior' },
  { path: '/attack-chains', label: '攻击链', icon: 'link' },
  { path: '/honeypot', label: 'Agent 蜜罐', icon: 'flame' },
  { path: '/anomaly', label: '异常检测', icon: 'chart-line' },
  { path: '/singularity', label: '奇点蜜罐', icon: 'orbit' },
]

// 安全治理导航项 (7项)
const governItems = [
  { path: '/redteam', label: '红队测试', icon: 'crosshair' },
  { path: '/leaderboard', label: '排行榜', icon: 'trophy' },
  { path: '/tenants', label: '租户', icon: 'building' },
  { path: '/reports', label: '报告', icon: 'file-up' },
  { path: '/envelopes', label: '执行信封', icon: 'lock' },
  { path: '/events', label: '事件总线', icon: 'radio' },
  { path: '/evolution', label: '自进化', icon: 'dna' },
]

// 系统导航项 (4项)
const sysItems = [
  { path: '/monitor', label: '监控', icon: 'activity' },
  { path: '/users', label: '用户管理', icon: 'users' },
  { path: '/ops', label: '运维', icon: 'wrench' },
  { path: '/settings', label: '设置', icon: 'settings' },
]

const dotClass = computed(() => appState.connectionStatus === 'connected' ? 'dot-healthy' : 'dot-unhealthy')
const statusText = computed(() => {
  const s = appState.connectionStatus
  return s === 'connected' ? '在线' : (s === 'degraded' ? '降级' : '断开')
})
</script>

<style scoped>
.sidebar {
  width: var(--sidebar-w); min-width: var(--sidebar-w); height: 100vh;
  background: var(--bg-surface);
  border-right: 1px solid var(--border-subtle);
  display: flex; flex-direction: column; transition: width var(--transition-normal), min-width var(--transition-normal);
  z-index: 200; position: relative; overflow: hidden;
}
.sidebar.collapsed { width: var(--sidebar-collapsed); min-width: var(--sidebar-collapsed); }
.sidebar-brand {
  padding: var(--space-4) var(--space-4) var(--space-3); display: flex; align-items: center; gap: var(--space-3);
  border-bottom: 1px solid var(--border-subtle); min-height: 60px;
  overflow: hidden; white-space: nowrap;
  background: var(--gradient-surface);
}
.sidebar-logo { font-size: 1.75rem; flex-shrink: 0; }
.sidebar-brand-text { display: flex; flex-direction: column; overflow: hidden; transition: opacity var(--transition-normal), width var(--transition-normal); }
.sidebar.collapsed .sidebar-brand-text { opacity: 0; width: 0; }
.sidebar-brand-title {
  font-size: var(--text-base); font-weight: 700;
  color: var(--text-primary);
  white-space: nowrap;
}
.sidebar-brand-sub { font-size: var(--text-xs); color: var(--text-tertiary); white-space: nowrap; }
.sidebar-nav { flex: 1; padding: var(--space-2) 0; overflow-y: auto; overflow-x: hidden; min-height: 0; }
.sidebar-nav::-webkit-scrollbar { width: 4px; }
.sidebar-nav::-webkit-scrollbar-track { background: transparent; }
.sidebar-nav::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.15); border-radius: 4px; }
.sidebar-nav::-webkit-scrollbar-thumb:hover { background: rgba(255,255,255,0.3); }

.nav-group-label {
  padding: var(--space-2) var(--space-4) var(--space-1);
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-tertiary);
  white-space: nowrap;
  overflow: hidden;
}
.nav-divider {
  height: 1px;
  background: var(--border-subtle);
  margin: var(--space-2) var(--space-3);
}

.nav-item {
  display: flex; align-items: center; gap: var(--space-3); padding: var(--space-2) var(--space-4); margin: var(--space-1) var(--space-2);
  border-radius: var(--radius-md); cursor: pointer; color: var(--text-secondary); font-size: var(--text-sm);
  transition: all var(--transition-fast); position: relative; white-space: nowrap; overflow: hidden; text-decoration: none;
}
.nav-item:hover { background: var(--bg-elevated); color: var(--text-primary); }
.nav-item.active { background: var(--color-primary-dim); color: var(--color-primary); }
.nav-item.active::before {
  content: ''; position: absolute; left: 0; top: var(--space-1); bottom: var(--space-1); width: 3px;
  background: var(--color-primary); border-radius: 0 3px 3px 0;
}
.nav-icon { flex-shrink: 0; width: 20px; height: 20px; display: flex; align-items: center; justify-content: center; }
.nav-label { transition: opacity var(--transition-normal); overflow: hidden; }
.sidebar.collapsed .nav-label { opacity: 0; width: 0; }
.sidebar.collapsed .nav-item { justify-content: center; padding: var(--space-2) 0; margin: var(--space-1) var(--space-1); }
.sidebar.collapsed .nav-group-label,
.sidebar.collapsed .nav-divider { display: none; }
.sidebar-footer {
  padding: var(--space-3) var(--space-4); border-top: 1px solid var(--border-subtle);
  display: flex; flex-direction: column; gap: var(--space-1); overflow: hidden;
}
.sidebar-version { font-size: var(--text-xs); color: var(--text-tertiary); font-family: var(--font-mono); white-space: nowrap; transition: opacity var(--transition-normal); }
.sidebar-status { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); color: var(--text-secondary); white-space: nowrap; }
.sidebar.collapsed .sidebar-version,
.sidebar.collapsed .sidebar-status-text { opacity: 0; width: 0; overflow: hidden; }
.sidebar-toggle {
  display: flex; align-items: center; justify-content: center; padding: var(--space-2);
  margin: 0 var(--space-2) var(--space-2); border-radius: var(--radius-md); cursor: pointer; color: var(--text-tertiary);
  transition: all var(--transition-fast); border: 1px solid var(--border-subtle); background: transparent;
}
.sidebar-toggle:hover { background: var(--bg-elevated); color: var(--text-primary); }

/* 态势大屏按钮 */
.bigscreen-btns {
  display: flex; gap: 4px; margin-bottom: var(--space-2);
}
.bigscreen-btns .bigscreen-btn { flex: 1; margin: 0; }
.bigscreen-btn {
  display: flex; align-items: center; justify-content: center;
  padding: 8px 12px; margin: 0 0 var(--space-2) 0;
  background: linear-gradient(135deg, rgba(99,102,241,0.15), rgba(139,92,246,0.15));
  border: 1px solid rgba(99,102,241,0.3);
  border-radius: var(--radius-md); color: #a5b4fc;
  font-size: var(--text-xs); font-weight: 600;
  cursor: pointer; transition: all var(--transition-fast);
  text-decoration: none; white-space: nowrap;
  letter-spacing: 0.03em;
}
.bigscreen-btn:hover {
  background: linear-gradient(135deg, rgba(99,102,241,0.25), rgba(139,92,246,0.25));
  color: #c7d2fe; box-shadow: 0 0 12px rgba(99,102,241,0.2);
}
.bigscreen-btn-icon { padding: 8px 0; font-size: 1.2rem; }

/* 严格模式 */
.strict-banner {
  background: linear-gradient(90deg, #DC2626, #EF4444);
  color: #fff; text-align: center; font-size: 11px; font-weight: 700;
  padding: 4px 8px; letter-spacing: 0.05em;
}
.strict-logo { filter: hue-rotate(0deg) saturate(3) brightness(1.2); }
.strict-toggle {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 0 var(--space-2) 0; margin-bottom: var(--space-2);
  border-bottom: 1px solid var(--border-subtle);
}
.strict-label { font-size: var(--text-xs); color: var(--text-secondary); white-space: nowrap; }
.toggle-switch { position: relative; display: inline-block; width: 36px; height: 20px; cursor: pointer; }
.toggle-switch input { opacity: 0; width: 0; height: 0; }
.toggle-slider {
  position: absolute; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(255,255,255,0.1); border-radius: 20px; transition: all .3s;
}
.toggle-slider::before {
  content: ''; position: absolute; height: 16px; width: 16px; left: 2px; bottom: 2px;
  background: #fff; border-radius: 50%; transition: all .3s;
}
.toggle-active { background: #EF4444; }
.toggle-active::before { transform: translateX(16px); }

/* 严格模式 toast */
.strict-toast {
  position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%);
  background: var(--bg-surface); border: 1px solid var(--color-primary);
  color: var(--text-primary); padding: 10px 20px; border-radius: var(--radius-md);
  font-size: var(--text-xs); font-weight: 600; box-shadow: var(--shadow-lg);
  z-index: 9999; white-space: nowrap; max-width: 90vw; text-align: center;
}
.toast-fade-enter-active { transition: all .3s ease; }
.toast-fade-leave-active { transition: all .3s ease; }
.toast-fade-enter-from, .toast-fade-leave-to { opacity: 0; transform: translateX(-50%) translateY(10px); }
@media(max-width:768px) {
  .sidebar {
    position: fixed; left: -280px; top: 0; bottom: 0; z-index: 201;
    transition: left .3s; box-shadow: 4px 0 20px rgba(0,0,0,.5);
  }
  .sidebar.mobile-open { left: 0; }
}
</style>
