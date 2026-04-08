<template>
  <div>
    <Skeleton v-if="enginesLoading" type="table" />
    <template v-else>
      <div class="card" style="margin-bottom:16px">
        <div class="card-header">
          <span class="card-icon">🧠</span><span class="card-title">检测引擎总览</span>
          <div class="card-actions">
            <span class="engine-stats"><span class="engine-stat-on">{{ engineOnCount }} 启用</span><span class="engine-stat-off">{{ engineOffCount }} 关闭</span></span>
            <button class="btn btn-ghost btn-sm" @click="onRefresh">刷新</button>
          </div>
        </div>
        <div class="config-desc">统一管理所有安全引擎的启用/禁用状态，修改即时生效。</div>
      </div>
      <div class="engine-list">
        <div v-for="eng in engineList" :key="eng.configPath" class="engine-row card">
          <div class="engine-info">
            <div class="engine-name-row">
              <span class="engine-status-dot" :class="{ on: eng.alwaysOn || engineSettings[eng.configPath] }"></span>
              <span class="engine-name">{{ eng.name }}</span>
              <span v-if="eng.alwaysOn" class="engine-always-on-tag">始终启用</span>
            </div>
            <div class="engine-desc">{{ eng.desc }}</div>
            <div class="engine-path"><code>{{ eng.configPath }}</code></div>
          </div>
          <div class="engine-toggle">
            <label v-if="!eng.alwaysOn" class="toggle-switch toggle-switch-lg"><input type="checkbox" :checked="engineSettings[eng.configPath]" @change="onToggleEngine(eng, $event)" :disabled="eng.saving" /><span class="toggle-slider"></span></label>
            <span v-else class="engine-locked">🔒</span>
            <span v-if="eng.saving" class="engine-saving">保存中...</span>
          </div>
        </div>
      </div>
      <div class="card" style="margin-top:16px;border-color:rgba(99,102,241,.22)">
        <div class="card-header"><span class="card-icon">🔄</span><span class="card-title">污染逆转模式</span></div>
        <div style="padding:8px 20px 16px;display:flex;align-items:center;gap:12px">
          <span style="font-size:13px;color:var(--text-secondary)">请求侧 (pre-inject) + 响应侧 (soft/hard/stealth) 双模式配置</span>
          <a href="#/taint" class="btn btn-primary btn-sm" style="white-space:nowrap">前往污点追踪页配置 →</a>
        </div>
      </div>

      <div class="card" style="margin-top:16px;border-color:rgba(99,102,241,.22)">
        <div class="card-header"><span class="card-icon">🧩</span><span class="card-title">CaMeL 三引擎独立开关</span></div>
        <div class="engine-list">
          <div v-for="item in camelEngines" :key="item.name" class="engine-row card">
            <div class="engine-info">
              <div class="engine-name-row"><span class="engine-status-dot" :class="{ on: item.enabled }"></span><span class="engine-name">{{ item.title }}</span></div>
              <div class="engine-desc">{{ item.desc }}</div>
              <div class="engine-path"><code>{{ item.name }}</code></div>
              <div class="engine-desc" style="margin-top:8px">{{ item.stat1Label }}：{{ item.stat1 }} · {{ item.stat2Label }}：{{ item.stat2 }}</div>
            </div>
            <div class="engine-toggle">
              <label class="toggle-switch toggle-switch-lg"><input type="checkbox" :checked="item.enabled" @change="onToggleCamelEngine(item, $event)" /><span class="toggle-slider"></span></label>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import Skeleton from '../../components/Skeleton.vue'

defineProps({
  enginesLoading: Boolean,
  engineOnCount: Number,
  engineOffCount: Number,
  engineList: Array,
  engineSettings: Object,
  camelEngines: Array,
  onRefresh: Function,
  onToggleEngine: Function,
  onToggleCamelEngine: Function,
})
</script>

<style scoped>
.config-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin-bottom: var(--space-3); }
.engine-list { display: flex; flex-direction: column; gap: 8px; }
.engine-row { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px; }
.engine-info { flex: 1; min-width: 0; }
.engine-name-row { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.engine-status-dot { width: 10px; height: 10px; border-radius: 50%; background: var(--text-quaternary, #4a5568); flex-shrink: 0; transition: background .3s; }
.engine-status-dot.on { background: var(--color-success, #22c55e); box-shadow: 0 0 6px rgba(34,197,94,0.4); }
.engine-name { font-size: var(--text-sm); font-weight: 600; color: var(--text-primary); }
.engine-always-on-tag { font-size: 10px; padding: 1px 8px; border-radius: 9999px; background: rgba(34,197,94,0.12); color: var(--color-success); font-weight: 500; }
.engine-desc { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: 2px; padding-left: 18px; }
.engine-path { padding-left: 18px; }
.engine-path code { font-size: 11px; color: var(--text-quaternary, #64748b); font-family: var(--font-mono); }
.engine-toggle { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.engine-locked { font-size: 18px; }
.engine-saving { font-size: var(--text-xs); color: var(--color-warning); }
.engine-stats { display: flex; gap: 12px; font-size: var(--text-xs); }
.engine-stat-on { color: var(--color-success); font-weight: 600; }
.engine-stat-off { color: var(--text-tertiary); }
.toggle-switch { position: relative; display: inline-block; width: 36px; height: 20px; cursor: pointer; }
.toggle-switch input { opacity: 0; width: 0; height: 0; }
.toggle-slider { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: 20px; transition: .3s; }
.toggle-slider:before { content: ''; position: absolute; height: 14px; width: 14px; left: 2px; bottom: 2px; background: var(--text-tertiary); border-radius: 50%; transition: .3s; }
.toggle-switch input:checked + .toggle-slider { background: var(--color-primary); border-color: var(--color-primary); }
.toggle-switch input:checked + .toggle-slider:before { transform: translateX(16px); background: #fff; }
.toggle-switch-lg { width: 44px; height: 24px; }
.toggle-switch-lg .toggle-slider:before { height: 18px; width: 18px; }
.toggle-switch-lg input:checked + .toggle-slider:before { transform: translateX(20px); }
@media (max-width: 640px) {
  .engine-row { flex-direction: column; align-items: flex-start; gap: 8px; }
}
</style>
