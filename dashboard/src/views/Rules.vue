<template>
  <div>
    <!-- Rule Hits -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon">🎯</span><span class="card-title">规则命中率</span>
        <div class="card-actions">
          <button class="btn btn-sm" @click="loadRuleHits">🔄 刷新</button>
          <button class="btn btn-sm btn-red" @click="confirmResetHits">🔄 重置</button>
        </div>
      </div>
      <DataTable :columns="hitsColumns" :data="ruleHits" :loading="hitsLoading" empty-text="暂无命中数据" empty-icon="🎯" :expandable="false">
        <template #cell-hits="{ value }"><span style="font-weight:700;color:var(--neon-blue)">{{ value }}</span></template>
        <template #cell-action="{ value }"><span class="tag" :class="actTag(value)">{{ value }}</span></template>
        <template #cell-direction="{ value }">{{ value === 'inbound' ? '入站' : '出站' }}</template>
        <template #cell-last_hit="{ value }">{{ fmtTime(value) }}</template>
      </DataTable>
    </div>

    <!-- Rules Management -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header"><span class="card-icon">⚙️</span><span class="card-title">规则管理</span></div>
      <div class="tab-header">
        <button class="tab-btn" :class="{ active: activeTab === 'inbound' }" @click="activeTab = 'inbound'">入站规则</button>
        <button class="tab-btn" :class="{ active: activeTab === 'outbound' }" @click="activeTab = 'outbound'">出站规则</button>
      </div>

      <!-- Inbound -->
      <div v-show="activeTab === 'inbound'">
        <DataTable :columns="inboundColumns" :data="inboundRules" :loading="inboundLoading" empty-text="暂无入站规则" :expandable="true">
          <template #cell-action="{ value }"><span class="tag" :class="actTag(value)">{{ value }}</span></template>
          <template #cell-type="{ value }"><span class="tag tag-info">{{ value || 'keyword' }}</span></template>
          <template #cell-group="{ value }">
            <span v-if="value" class="tag" :style="{ background: groupColor(value), color: '#fff' }">{{ value }}</span>
            <span v-else>--</span>
          </template>
          <template #expand="{ row }">
            <div style="font-size:.82rem">
              <div><b style="color:var(--neon-blue)">名称:</b> {{ row.name }}</div>
              <div><b style="color:var(--neon-blue)">类型:</b> {{ row.type || 'keyword' }} | <b style="color:var(--neon-blue)">动作:</b> {{ row.action }} | <b style="color:var(--neon-blue)">优先级:</b> {{ row.priority ?? '--' }}</div>
              <div v-if="row.patterns && row.patterns.length"><b style="color:var(--neon-blue)">模式:</b>
                <pre style="background:rgba(0,0,0,.3);padding:8px;border-radius:6px;margin-top:4px;font-size:.75rem;overflow-x:auto;color:var(--neon-green)">{{ row.patterns.join('\n') }}</pre>
              </div>
            </div>
          </template>
        </DataTable>
        <div class="rule-meta" v-if="inboundMeta">
          版本: {{ inboundMeta.version }} 来源: {{ inboundMeta.source }} 加载: {{ fmtTime(inboundMeta.loaded_at) }}
        </div>
        <div style="margin-top:12px"><button class="btn btn-green" @click="reloadInbound">🔄 热更新入站规则</button></div>
      </div>

      <!-- Outbound -->
      <div v-show="activeTab === 'outbound'">
        <DataTable :columns="outboundColumns" :data="outboundRules" :loading="outboundLoading" empty-text="暂无出站规则" :expandable="false">
          <template #cell-action="{ value }"><span class="tag" :class="actTag(value)">{{ value }}</span></template>
        </DataTable>
        <div style="margin-top:12px"><button class="btn btn-green" @click="reloadOutbound">🔄 热更新出站规则</button></div>
      </div>
    </div>

    <!-- Rule Templates -->
    <div class="card">
      <div class="card-header">
        <span class="card-icon">📖</span><span class="card-title">规则模板</span>
        <div class="card-actions"><button class="btn btn-sm" @click="loadTemplates">🔄 刷新</button></div>
      </div>
      <DataTable :columns="templateColumns" :data="templates" :loading="templatesLoading" empty-text="暂无规则模板" empty-icon="📖" />
    </div>

    <!-- Confirm modal -->
    <ConfirmModal :visible="confirmVisible" :title="confirmTitle" :message="confirmMessage" :type="confirmType" @confirm="doConfirm" @cancel="confirmVisible = false" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api, apiPost } from '../api.js'
import { showToast } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const activeTab = ref('inbound')

// Rule hits
const ruleHits = ref([])
const hitsLoading = ref(false)
const hitsColumns = [
  { key: 'name', label: '规则', sortable: true },
  { key: 'hits', label: '命中次数', sortable: true },
  { key: 'action', label: '动作', sortable: true },
  { key: 'direction', label: '方向', sortable: true },
  { key: 'last_hit', label: '最后命中', sortable: true },
]

// Inbound rules
const inboundRules = ref([])
const inboundLoading = ref(false)
const inboundMeta = ref(null)
const inboundColumns = [
  { key: 'name', label: '名称', sortable: true },
  { key: 'action', label: '动作', sortable: true },
  { key: 'type', label: '类型', sortable: true },
  { key: 'priority', label: '优先级', sortable: true },
  { key: 'patterns_count', label: '模式数', sortable: true },
  { key: 'group', label: '分组', sortable: true },
]

// Outbound rules
const outboundRules = ref([])
const outboundLoading = ref(false)
const outboundColumns = [
  { key: 'name', label: '名称', sortable: true },
  { key: 'action', label: '动作', sortable: true },
  { key: 'patterns_count', label: '模式数', sortable: true },
]

// Templates
const templates = ref([])
const templatesLoading = ref(false)
const templateColumns = [
  { key: 'name', label: '名称', sortable: true },
  { key: 'description', label: '描述', sortable: false },
  { key: 'rule_count', label: '规则数', sortable: true },
  { key: 'category', label: '分类', sortable: true },
]

// Confirm
const confirmVisible = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmType = ref('warning')
let confirmAction = null

const groupColors = { jailbreak: '#ff6b6b', injection: '#ffa94d', social_engineering: '#69db7c', pii: '#74c0fc', sensitive: '#b197fc', roleplay: '#e599f7', command_injection: '#ff8787' }
function groupColor(g) { return groupColors[g] || '#868e96' }
function actTag(a) { a = (a || '').toLowerCase(); return a === 'block' ? 'tag-block' : a === 'warn' ? 'tag-warn' : a === 'log' ? 'tag-log' : 'tag-pass' }
function fmtTime(ts) { if (!ts) return '--'; const d = new Date(ts); return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false }) }

async function loadRuleHits() {
  hitsLoading.value = true
  try { const d = await api('/api/v1/rules/hits'); ruleHits.value = Array.isArray(d) ? d : (d.hits || []) } catch { ruleHits.value = [] }
  hitsLoading.value = false
}

async function loadInbound() {
  inboundLoading.value = true
  try {
    const d = await api('/api/v1/inbound-rules')
    const list = d.rules || []
    inboundRules.value = list.map(r => ({ ...r, patterns_count: r.patterns_count ?? r.pattern_count ?? '--' }))
    if (d.version && typeof d.version === 'object') inboundMeta.value = d.version
    else inboundMeta.value = null
  } catch { inboundRules.value = [] }
  inboundLoading.value = false
}

async function loadOutbound() {
  outboundLoading.value = true
  try { const d = await api('/api/v1/outbound-rules'); outboundRules.value = d.rules || [] } catch { outboundRules.value = [] }
  outboundLoading.value = false
}

async function loadTemplates() {
  templatesLoading.value = true
  try { const d = await api('/api/v1/rule-templates'); templates.value = d.templates || [] } catch { templates.value = [] }
  templatesLoading.value = false
}

function confirmResetHits() {
  confirmTitle.value = '重置命中统计'
  confirmMessage.value = '确认重置所有规则命中统计？此操作不可恢复。'
  confirmType.value = 'danger'
  confirmAction = async () => {
    try { await apiPost('/api/v1/rules/hits/reset', {}); showToast('命中统计已重置', 'success'); loadRuleHits() } catch (e) { showToast('重置失败: ' + e.message, 'error') }
  }
  confirmVisible.value = true
}

async function reloadInbound() {
  try { await apiPost('/api/v1/inbound-rules/reload', {}); showToast('入站规则已热更新', 'success'); loadInbound() } catch (e) { showToast('更新失败: ' + e.message, 'error') }
}

async function reloadOutbound() {
  try { await apiPost('/api/v1/rules/reload', {}); showToast('出站规则已热更新', 'success'); loadOutbound() } catch (e) { showToast('更新失败: ' + e.message, 'error') }
}

function doConfirm() {
  confirmVisible.value = false
  if (confirmAction) confirmAction()
}

onMounted(() => { loadRuleHits(); loadInbound(); loadOutbound(); loadTemplates() })
</script>
