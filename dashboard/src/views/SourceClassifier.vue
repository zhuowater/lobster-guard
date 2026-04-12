<template>
  <div class="source-classifier-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="globe" :size="20" /> Source Classifier</h1>
        <p class="page-subtitle">管理全局来源分类规则，并对单条 tool call 做 tenant-aware dry-run explain。</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-sm" @click="refreshAll" :disabled="loading || explainLoading">刷新</button>
        <button class="btn btn-primary btn-sm" @click="saveConfig" :disabled="saving">{{ saving ? '保存中...' : '保存全局规则' }}</button>
      </div>
    </div>

    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-label">全局规则数</div>
        <div class="stat-value">{{ rules.length }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">覆盖分类</div>
        <div class="stat-value">{{ categories.length }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Explain 租户</div>
        <div class="stat-value stat-small">{{ explainForm.tenant_id || 'global' }}</div>
      </div>
    </div>

    <div class="section two-col">
      <div class="panel">
        <div class="panel-header">
          <h3>全局规则编辑器</h3>
          <button class="btn btn-sm" @click="addRule">+ 新增规则</button>
        </div>
        <div v-if="loading" class="empty">加载中...</div>
        <div v-else-if="!rules.length" class="empty">暂无规则，点击“新增规则”开始配置。</div>
        <div v-else class="rule-list">
          <div v-for="(rule, idx) in rules" :key="rule.name || idx" class="rule-card">
            <div class="rule-row">
              <label>名称</label>
              <input v-model="rule.name" class="field-input" placeholder="corp-control-plane" />
            </div>
            <div class="rule-grid">
              <div class="rule-row"><label>Host Regex</label><input v-model="rule.host_pattern" class="field-input" placeholder="^api\.corp\.example$" /></div>
              <div class="rule-row"><label>Path Regex</label><input v-model="rule.path_pattern" class="field-input" placeholder="/v[0-9]+/admin/" /></div>
              <div class="rule-row"><label>Tool Regex</label><input v-model="rule.tool_pattern" class="field-input" placeholder="^http_request$" /></div>
              <div class="rule-row"><label>Method Regex</label><input v-model="rule.method_pattern" class="field-input" placeholder="^(GET|POST)$" /></div>
              <div class="rule-row"><label>Auth Regex</label><input v-model="rule.auth_type_pattern" class="field-input" placeholder="^(bearer|api_key)$" /></div>
              <div class="rule-row"><label>Category</label><input v-model="rule.category" class="field-input" placeholder="internal_control_plane" /></div>
              <div class="rule-row"><label>Trust</label><input v-model.number="rule.trust_score" type="number" step="0.01" min="0" max="1" class="field-input" /></div>
              <div class="rule-row"><label>Conf</label><input v-model.number="rule.confidentiality" type="number" min="0" max="3" class="field-input" /></div>
              <div class="rule-row"><label>Integ</label><input v-model.number="rule.integrity" type="number" min="0" max="3" class="field-input" /></div>
            </div>
            <div class="rule-row">
              <label>Tags（逗号分隔）</label>
              <input :value="(rule.tags || []).join(', ')" @input="rule.tags = parseTags($event.target.value)" class="field-input" placeholder="corp_override, control_plane" />
            </div>
            <div class="rule-actions">
              <button class="btn btn-danger btn-sm" @click="removeRule(idx)">删除</button>
            </div>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header"><h3>全局预览</h3></div>
        <div class="preview-block">
          <p class="hint">当前配置 JSON</p>
          <pre>{{ previewJson }}</pre>
        </div>
        <div class="preview-block">
          <p class="hint">分类列表</p>
          <div class="chip-wrap">
            <span v-for="c in categories" :key="c" class="chip">{{ c }}</span>
            <span v-if="!categories.length" class="empty-inline">暂无</span>
          </div>
        </div>
      </div>
    </div>

    <div class="panel explain-panel">
      <div class="panel-header">
        <div>
          <h3>Dry-run Explain</h3>
          <p class="hint">输入 tool call，返回 global/effective source、PathPolicy 决策和 Capability 决策。</p>
        </div>
        <button class="btn btn-primary btn-sm" @click="runExplain" :disabled="explainLoading">{{ explainLoading ? '分析中...' : '运行 Explain' }}</button>
      </div>

      <div class="explain-grid">
        <div class="rule-row">
          <label>租户</label>
          <select v-model="explainForm.tenant_id" class="field-input">
            <option value="">global (无 tenant override)</option>
            <option v-for="tenant in tenants" :key="tenant.id" :value="tenant.id">{{ tenant.name }} ({{ tenant.id }})</option>
          </select>
        </div>
        <div class="rule-row">
          <label>Tool Name</label>
          <input v-model="explainForm.tool_name" class="field-input" placeholder="web_fetch" />
        </div>
        <div class="rule-row">
          <label>Proposed Action</label>
          <input v-model="explainForm.proposed_action" class="field-input" placeholder="shell_exec" />
        </div>
        <div class="rule-row">
          <label>Capability Action</label>
          <select v-model="explainForm.capability_action" class="field-input">
            <option value="">不评估</option>
            <option value="read">read</option>
            <option value="write">write</option>
            <option value="execute">execute</option>
            <option value="admin">admin</option>
          </select>
        </div>
      </div>

      <div class="rule-row">
        <label>Tool Args JSON</label>
        <textarea v-model="explainArgsText" class="field-input code-input" rows="7" placeholder='{"url":"https://docs.python.org/3/library/json.html"}' />
      </div>

      <div v-if="explainError" class="error-box">{{ explainError }}</div>

      <div v-if="explainResult" class="explain-results two-col">
        <div class="panel result-subpanel">
          <div class="panel-header"><h4>Classification</h4></div>
          <div class="result-row"><span>Tenant Override</span><strong>{{ explainResult.tenant_override_active ? 'Yes' : 'No' }}</strong></div>
          <div class="result-row"><span>Global Category</span><strong>{{ explainResult.global_descriptor?.category || '-' }}</strong></div>
          <div class="result-row"><span>Global Rule</span><strong>{{ explainResult.global_rule?.name || 'heuristic' }}</strong></div>
          <div class="result-row"><span>Effective Category</span><strong>{{ explainResult.effective_descriptor?.category || '-' }}</strong></div>
          <div class="result-row"><span>Effective Rule</span><strong>{{ explainResult.effective_rule?.name || 'heuristic' }}</strong></div>
          <div class="preview-block">
            <p class="hint">Global Descriptor</p>
            <pre>{{ pretty(explainResult.global_descriptor) }}</pre>
          </div>
          <div class="preview-block">
            <p class="hint">Effective Descriptor</p>
            <pre>{{ pretty(explainResult.effective_descriptor) }}</pre>
          </div>
        </div>

        <div class="panel result-subpanel">
          <div class="panel-header"><h4>Governance</h4></div>
          <div class="result-row"><span>Path Decision</span><strong>{{ explainResult.path_decision?.decision || '-' }}</strong></div>
          <div class="result-row"><span>Capability</span><strong>{{ explainResult.capability_evaluation?.decision || '-' }}</strong></div>
          <div class="preview-block">
            <p class="hint">Path Decision</p>
            <pre>{{ pretty(explainResult.path_decision) }}</pre>
          </div>
          <div class="preview-block">
            <p class="hint">Path Context</p>
            <pre>{{ pretty(explainResult.path_context) }}</pre>
          </div>
          <div class="preview-block">
            <p class="hint">Capability Evaluation</p>
            <pre>{{ pretty(explainResult.capability_evaluation) }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import { api, apiPost, apiPut } from '../api.js'

const loading = ref(false)
const saving = ref(false)
const explainLoading = ref(false)
const rules = ref([])
const tenants = ref([])
const explainResult = ref(null)
const explainError = ref('')
const explainArgsText = ref(`{
  "url": "https://docs.python.org/3/library/json.html"
}`)
const explainForm = ref({
  tenant_id: '',
  tool_name: 'web_fetch',
  proposed_action: 'shell_exec',
  capability_action: 'write',
})

const categories = computed(() => [...new Set(rules.value.map(r => r.category).filter(Boolean))])
const previewJson = computed(() => JSON.stringify({ rules: rules.value }, null, 2))

function normalizeRule(rule = {}) {
  return {
    name: rule.name || '',
    tool_pattern: rule.tool_pattern || '',
    host_pattern: rule.host_pattern || '',
    path_pattern: rule.path_pattern || '',
    method_pattern: rule.method_pattern || '',
    auth_type_pattern: rule.auth_type_pattern || '',
    category: rule.category || '',
    confidentiality: Number(rule.confidentiality ?? 1),
    integrity: Number(rule.integrity ?? 1),
    trust_score: Number(rule.trust_score ?? 0.3),
    tags: Array.isArray(rule.tags) ? rule.tags : [],
  }
}

function parseTags(raw) {
  return String(raw || '').split(',').map(s => s.trim()).filter(Boolean)
}

function addRule() {
  rules.value.push(normalizeRule())
}

function removeRule(idx) {
  rules.value.splice(idx, 1)
}

function pretty(value) {
  return JSON.stringify(value || {}, null, 2)
}

async function loadConfig() {
  loading.value = true
  try {
    const res = await api('/api/v1/source-classifier')
    rules.value = (res.config?.rules || []).map(normalizeRule)
  } finally {
    loading.value = false
  }
}

async function loadTenants() {
  const res = await api('/api/v1/tenants')
  tenants.value = res.tenants || []
}

async function saveConfig() {
  saving.value = true
  try {
    await apiPut('/api/v1/source-classifier', { rules: rules.value })
    await loadConfig()
  } finally {
    saving.value = false
  }
}

async function runExplain() {
  explainLoading.value = true
  explainError.value = ''
  try {
    const toolArgs = JSON.parse(explainArgsText.value || '{}')
    explainResult.value = await apiPost('/api/v1/source-classifier/explain', {
      tenant_id: explainForm.value.tenant_id || undefined,
      tool_name: explainForm.value.tool_name,
      tool_args: toolArgs,
      proposed_action: explainForm.value.proposed_action || undefined,
      capability_action: explainForm.value.capability_action || undefined,
    })
  } catch (err) {
    explainResult.value = null
    explainError.value = err?.message || 'Explain 失败'
  } finally {
    explainLoading.value = false
  }
}

async function refreshAll() {
  await Promise.all([loadConfig(), loadTenants()])
}

onMounted(async () => {
  await refreshAll()
})
</script>

<style scoped>
.source-classifier-page { display:flex; flex-direction:column; gap:16px; }
.page-header { display:flex; justify-content:space-between; align-items:flex-start; gap:16px; }
.page-title { display:flex; align-items:center; gap:8px; margin:0; }
.page-subtitle { margin:6px 0 0; color:var(--text-secondary, #94a3b8); }
.header-actions { display:flex; gap:8px; }
.stats-grid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:12px; }
.stat-card,.panel { background:var(--panel-bg, #111827); border:1px solid var(--border-color, #243041); border-radius:12px; padding:16px; }
.stat-label,.hint { color:var(--text-secondary, #94a3b8); font-size:12px; }
.stat-value { font-size:28px; font-weight:700; margin-top:6px; }
.stat-small { font-size:18px; word-break:break-all; }
.two-col { display:grid; grid-template-columns:2fr 1fr; gap:16px; }
.panel-header { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; margin-bottom:12px; }
.rule-list { display:flex; flex-direction:column; gap:12px; }
.rule-card { border:1px solid var(--border-color, #243041); border-radius:10px; padding:12px; display:flex; flex-direction:column; gap:10px; }
.rule-grid,.explain-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:10px; }
.rule-row { display:flex; flex-direction:column; gap:6px; }
.rule-row label { font-size:12px; color:var(--text-secondary, #94a3b8); }
.field-input { width:100%; border-radius:8px; border:1px solid var(--border-color, #243041); background:var(--input-bg, #0b1220); color:inherit; padding:10px 12px; }
.code-input { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
.rule-actions { display:flex; justify-content:flex-end; }
.preview-block { margin-bottom:16px; }
pre { white-space:pre-wrap; word-break:break-word; font-size:12px; background:#0b1220; padding:12px; border-radius:8px; overflow:auto; }
.chip-wrap { display:flex; flex-wrap:wrap; gap:8px; }
.chip { padding:4px 10px; border-radius:999px; background:#1d4ed8; color:#fff; font-size:12px; }
.empty,.empty-inline { color:var(--text-secondary, #94a3b8); }
.explain-panel { display:flex; flex-direction:column; gap:12px; }
.result-subpanel { padding:0; background:transparent; border:none; }
.result-row { display:flex; justify-content:space-between; gap:12px; margin-bottom:10px; font-size:13px; }
.error-box { border:1px solid #7f1d1d; background:#450a0a; color:#fecaca; border-radius:8px; padding:12px; }
@media (max-width: 1100px) { .two-col, .rule-grid, .stats-grid, .explain-grid { grid-template-columns:1fr; } }
</style>
