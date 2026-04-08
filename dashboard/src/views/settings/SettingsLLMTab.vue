<template>
  <div>
    <div class="card" style="margin-bottom:20px">
      <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44"/><path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44"/></svg></span><span class="card-title">LLM 代理配置</span></div>
      <Skeleton v-if="llmConfigLoading" type="text" />
      <div v-else-if="!llmConfig" style="color:var(--text-tertiary);font-size:var(--text-sm)">LLM 代理未启用</div>
      <div v-else>
        <div class="llm-row"><span class="llm-label">启用状态</span><span class="llm-val"><label class="toggle-switch"><input type="checkbox" v-model="llmConfig.enabled" /><span class="toggle-slider"></span></label><span style="margin-left:8px;font-size:var(--text-xs);color:var(--text-tertiary)">{{ llmConfig.enabled ? '已启用' : '未启用' }} · 修改需重启</span></span></div>
        <div class="llm-row"><span class="llm-label">监听端口</span><input type="text" v-model="llmConfig.listen" class="llm-input-sm" style="width:140px;font-family:var(--font-mono)" placeholder=":8445" /><span class="llm-hint">修改需重启</span></div>
        <div id="section-security" class="llm-section-title">安全策略</div>
        <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.security.scan_pii_in_response" /> 扫描响应中的 PII</label>
        <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.security.prompt_injection_scan" /> 扫描 Prompt Injection</label>
        <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.security.block_high_risk_tools" /> 拦截高危工具调用</label>
        <div class="llm-row" style="margin-top:8px"><span class="llm-label">高危工具</span><input type="text" v-model="highRiskToolsStr" class="llm-input" placeholder="exec, shell, bash" /></div>
        <div id="section-canary" class="llm-section-title">🐤 Canary Token</div>
        <label class="llm-checkbox"><input type="checkbox" v-model="canaryEnabled" /> 启用 Canary Token</label>
        <template v-if="canaryEnabled">
          <div class="llm-row"><span class="llm-label">Token</span><span class="llm-val" style="font-family:var(--font-mono);font-size:var(--text-xs)">{{ canaryStatus.token || '(未配置)' }}</span><button class="btn btn-sm" @click="rotateCanary" style="margin-left:8px;font-size:11px;padding:2px 8px">轮换</button></div>
          <div class="llm-row"><span class="llm-label">泄露动作</span><select v-model="canaryAlertAction" class="llm-input-sm" style="width:100px"><option value="log">log</option><option value="warn">warn</option><option value="block">block</option></select></div>
          <label class="llm-checkbox"><input type="checkbox" v-model="canaryAutoRotate" /> 每24h自动轮换</label>
          <div class="llm-row"><span class="llm-label">最近泄露</span><span class="llm-val" :style="{ color: (canaryStatus.leak_count||0) > 0 ? 'var(--color-danger)' : 'var(--text-secondary)' }">{{ canaryStatus.leak_count || 0 }} 次</span></div>
        </template>
        <div id="section-budget" class="llm-section-title"><Icon name="bar-chart" :size="16" /> Response Budget</div>
        <label class="llm-checkbox"><input type="checkbox" v-model="budgetEnabled" /> 启用预算控制</label>
        <div v-if="budgetEnabled">
          <div class="llm-row"><span class="llm-label">最大工具调用</span><input type="number" v-model.number="budgetMaxTools" class="llm-input-sm" min="1" max="100" /><span class="llm-hint">次/请求</span></div>
          <div class="llm-row"><span class="llm-label">单类工具</span><input type="number" v-model.number="budgetMaxSingle" class="llm-input-sm" min="1" max="50" /><span class="llm-hint">次/请求</span></div>
          <div class="llm-row"><span class="llm-label">最大 Token</span><input type="number" v-model.number="budgetMaxTokens" class="llm-input-sm" style="width:100px" min="1000" max="10000000" step="10000" /><span class="llm-hint">Token/请求</span></div>
          <div class="llm-row"><span class="llm-label">超限动作</span><select v-model="budgetAction" class="llm-input-sm" style="width:100px"><option value="warn">warn</option><option value="block">block</option></select></div>
          <div class="llm-row"><span class="llm-label">工具限制</span><input type="text" v-model="budgetToolLimitsStr" class="llm-input" placeholder="exec=3, shell=2" /></div>
          <div class="llm-row"><span class="llm-label">24h超限</span><span class="llm-val" :style="{ color: (budgetStatus.violations_24h||0) > 0 ? 'var(--color-warning)' : 'var(--text-secondary)' }">{{ budgetStatus.violations_24h || 0 }} 次</span></div>
        </div>
        <div class="llm-section-title">审计配置</div>
        <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.audit.log_tool_input" /> 记录工具调用输入</label>
        <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.audit.log_tool_result" /> 记录工具调用结果</label>
        <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.audit.log_system_prompt" /> 记录 System Prompt</label>
        <div class="llm-row" style="margin-top:8px"><span class="llm-label">摘要长度</span><input type="number" v-model.number="llmConfig.audit.max_preview_len" class="llm-input-sm" min="100" max="5000" /><span class="llm-hint">字符</span></div>
        <div id="section-cost" class="llm-section-title">成本预警</div>
        <div class="llm-row"><span class="llm-label">日限额</span><span class="llm-hint" style="margin-right:4px">$</span><input type="number" v-model.number="llmConfig.cost_alert.daily_limit_usd" class="llm-input-sm" min="0" step="5" /><span class="llm-hint">USD</span></div>
        <div class="llm-row"><span class="llm-label">Webhook</span><input type="text" v-model="llmConfig.cost_alert.webhook_url" class="llm-input" placeholder="https://..." /></div>
        <div class="llm-section-title llm-advanced-toggle" @click="toggleShowAdvanced" style="cursor:pointer;user-select:none"><span>{{ showAdvanced ? '▾' : '▸' }} 高级配置</span></div>
        <div v-show="showAdvanced">
          <div class="llm-row"><span class="llm-label">超时</span><input type="number" v-model.number="llmConfig.timeout_sec" class="llm-input-sm" min="5" max="300" /><span class="llm-hint">秒</span></div>
          <div class="llm-row"><span class="llm-label">请求体限制</span><input type="number" v-model.number="llmConfig.max_body_bytes" class="llm-input-sm" style="width:120px" min="0" step="1048576" /><span class="llm-hint">字节</span></div>
          <div v-if="llmConfig.targets && llmConfig.targets.length" class="llm-targets"><div v-for="t in llmConfig.targets" :key="t.name" class="llm-target-row"><code>{{ t.name }}</code><span style="color:var(--text-tertiary)">{{ t.upstream }}</span></div></div>
        </div>
        <div style="margin-top:var(--space-4);display:flex;align-items:center;gap:var(--space-3)">
          <button class="btn btn-sm" @click="saveLLMConfig" :disabled="llmSaving">{{ llmSaving ? '保存中...' : '保存配置' }}</button>
          <span class="llm-restart-hint">⚠️ 部分变更需重启生效</span>
        </div>
      </div>
    </div>

    <div class="card" style="margin-top:16px;border-color:rgba(99,102,241,.22)">
      <div class="card-header"><span class="card-icon">🐤</span><span class="card-title">金丝雀令牌管理</span></div>
      <div class="config-desc">创建时间：{{ canaryRotationStatus.token_created_at || '--' }} · 下次自动轮换：{{ canaryRotationStatus.next_rotation_at || '--' }}</div>
      <div style="margin-top:12px"><button class="btn btn-sm btn-primary" @click="confirmCanaryRotateNow">立即轮换</button></div>
      <div style="margin-top:16px" v-if="canaryRotationHistory.length">
        <div v-for="item in canaryRotationHistory" :key="item.rotated_at" class="status-row"><span class="status-key">{{ fmtTime(item.rotated_at) }}</span><span class="status-val">{{ item.old_token_hash }} → {{ item.new_token_hash }}</span></div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import Icon from '../../components/Icon.vue'
import Skeleton from '../../components/Skeleton.vue'

const props = defineProps({
  llmConfigLoading: Boolean,
  llmConfig: Object,
  llmSaving: Boolean,
  showAdvanced: Boolean,
  canaryStatus: Object,
  canaryRotationStatus: Object,
  canaryRotationHistory: Array,
  budgetStatus: Object,
  fmtTime: Function,
  rotateCanary: Function,
  saveLLMConfig: Function,
  confirmCanaryRotateNow: Function,
  toggleShowAdvanced: Function,
})

const highRiskToolsStr = computed({
  get: () => props.llmConfig?.security?.high_risk_tool_list?.join(', ') || '',
  set: (value) => {
    if (props.llmConfig?.security) {
      props.llmConfig.security.high_risk_tool_list = value.split(',').map((s) => s.trim()).filter(Boolean)
    }
  },
})

const canaryEnabled = computed({
  get: () => props.llmConfig?.security?.canary_token?.enabled ?? true,
  set: (value) => {
    if (props.llmConfig?.security?.canary_token) props.llmConfig.security.canary_token.enabled = value
  },
})

const canaryAlertAction = computed({
  get: () => props.llmConfig?.security?.canary_token?.alert_action || 'warn',
  set: (value) => {
    if (props.llmConfig?.security?.canary_token) props.llmConfig.security.canary_token.alert_action = value
  },
})

const canaryAutoRotate = computed({
  get: () => props.llmConfig?.security?.canary_token?.auto_rotate ?? false,
  set: (value) => {
    if (props.llmConfig?.security?.canary_token) props.llmConfig.security.canary_token.auto_rotate = value
  },
})

const budgetEnabled = computed({
  get: () => props.llmConfig?.security?.response_budget?.enabled ?? false,
  set: (value) => {
    if (props.llmConfig?.security?.response_budget) props.llmConfig.security.response_budget.enabled = value
  },
})

const budgetMaxTools = computed({
  get: () => props.llmConfig?.security?.response_budget?.max_tool_calls_per_req || 20,
  set: (value) => {
    if (props.llmConfig?.security?.response_budget) props.llmConfig.security.response_budget.max_tool_calls_per_req = value
  },
})

const budgetMaxSingle = computed({
  get: () => props.llmConfig?.security?.response_budget?.max_single_tool_per_req || 5,
  set: (value) => {
    if (props.llmConfig?.security?.response_budget) props.llmConfig.security.response_budget.max_single_tool_per_req = value
  },
})

const budgetMaxTokens = computed({
  get: () => props.llmConfig?.security?.response_budget?.max_tokens_per_req || 100000,
  set: (value) => {
    if (props.llmConfig?.security?.response_budget) props.llmConfig.security.response_budget.max_tokens_per_req = value
  },
})

const budgetAction = computed({
  get: () => props.llmConfig?.security?.response_budget?.over_budget_action || 'warn',
  set: (value) => {
    if (props.llmConfig?.security?.response_budget) props.llmConfig.security.response_budget.over_budget_action = value
  },
})

const budgetToolLimitsStr = computed({
  get: () => {
    const limits = props.llmConfig?.security?.response_budget?.tool_limits
    if (!limits || typeof limits !== 'object') return ''
    return Object.entries(limits).map(([k, v]) => `${k}=${v}`).join(', ')
  },
  set: (value) => {
    if (!props.llmConfig?.security?.response_budget) return
    const limits = {}
    value.split(',').map((s) => s.trim()).filter(Boolean).forEach((pair) => {
      const [k, v] = pair.split('=').map((s) => s.trim())
      if (k && v) limits[k] = parseInt(v, 10) || 5
    })
    props.llmConfig.security.response_budget.tool_limits = limits
  },
})
</script>

<style scoped>
.config-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin-bottom: var(--space-3); }
.status-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 0; border-bottom: 1px solid var(--border-subtle); }
.status-row:last-child { border-bottom: none; }
.status-key { color: var(--text-secondary); font-size: var(--text-sm); }
.status-val { color: var(--text-primary); font-size: var(--text-sm); }
.llm-row { display: flex; align-items: center; gap: var(--space-2); margin-bottom: 6px; font-size: var(--text-sm); }
.llm-label { color: var(--text-secondary); min-width: 80px; flex-shrink: 0; }
.llm-val { color: var(--text-primary); display: flex; align-items: center; gap: var(--space-1); }
.llm-section-title { font-size: var(--text-xs); font-weight: 600; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text-tertiary); margin: 16px 0 8px; padding-top: 12px; border-top: 1px solid var(--border-subtle); }
.llm-targets { display: flex; flex-direction: column; gap: 4px; }
.llm-target-row { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); padding: 4px 8px; background: var(--bg-elevated); border-radius: var(--radius-sm); }
.llm-target-row code { background: var(--bg-base); padding: 1px 6px; border-radius: 3px; font-size: var(--text-xs); font-family: var(--font-mono); color: var(--color-primary); }
.llm-checkbox { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); color: var(--text-primary); cursor: pointer; margin-bottom: 4px; }
.llm-checkbox input[type="checkbox"] { accent-color: var(--color-primary); width: 16px; height: 16px; cursor: pointer; }
.llm-input-sm { width: 80px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 4px 8px; font-size: var(--text-sm); outline: none; font-family: var(--font-mono); }
.llm-input-sm:focus { border-color: var(--color-primary); }
.llm-input { flex: 1; max-width: 320px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 4px 8px; font-size: var(--text-sm); outline: none; }
.llm-input:focus { border-color: var(--color-primary); }
.llm-hint { font-size: var(--text-xs); color: var(--text-tertiary); }
.llm-restart-hint { font-size: var(--text-xs); color: var(--color-warning); }
.llm-advanced-toggle { display: flex; align-items: center; }
.toggle-switch { position: relative; display: inline-block; width: 36px; height: 20px; cursor: pointer; }
.toggle-switch input { opacity: 0; width: 0; height: 0; }
.toggle-slider { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: 20px; transition: .3s; }
.toggle-slider:before { content: ''; position: absolute; height: 14px; width: 14px; left: 2px; bottom: 2px; background: var(--text-tertiary); border-radius: 50%; transition: .3s; }
.toggle-switch input:checked + .toggle-slider { background: var(--color-primary); border-color: var(--color-primary); }
.toggle-switch input:checked + .toggle-slider:before { transform: translateX(16px); background: #fff; }
</style>
