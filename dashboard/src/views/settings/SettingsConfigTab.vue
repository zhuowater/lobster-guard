<template>
  <div>
    <Skeleton v-if="configLoading" type="text" />
    <template v-else>
      <div class="card config-card" style="margin-bottom: 16px; border-color: rgba(99,102,241,.22); background: linear-gradient(135deg, rgba(49,46,129,.18), rgba(15,23,42,.96));">
        <div class="card-header" style="display:flex;justify-content:space-between;align-items:center;gap:12px;">
          <div>
            <div class="card-title" style="color:#e0e7ff;">🚀 快速配置向导</div>
            <div class="config-desc" style="margin-top:6px;color:#a5b4fc;">适合新手：4 步生成可下载的 config.yaml 模板，不会直接写入服务器。</div>
          </div>
          <button class="btn btn-sm btn-primary" style="background:#4f46e5;border-color:#6366f1;" @click="$emit('open-wizard')">打开向导</button>
        </div>
      </div>
      <div class="config-group-nav">
        <button v-for="g in configGroups" :key="g.key" class="config-group-btn" :class="{ active: activeGroup === g.key }" @click="$emit('update:activeGroup', g.key)">{{ g.icon }} {{ g.label }}</button>
      </div>
      <div v-if="hasChanges" class="changes-bar">
        <div class="changes-bar-inner">
          <span class="changes-dot">●</span>
          <span>{{ changedFields.length }} 项配置已修改</span>
          <button class="btn btn-sm" @click="$emit('toggle-changes-preview')" style="margin-left:8px">{{ showChangesPreview ? '收起' : '查看变更' }}</button>
          <button class="btn btn-sm btn-primary" @click="$emit('save-config')" :disabled="configSaving" style="margin-left:4px">{{ configSaving ? '保存中...' : '💾 保存' }}</button>
          <button class="btn btn-sm btn-ghost" @click="$emit('reset-config')" style="margin-left:4px">撤销</button>
        </div>
        <div v-if="showChangesPreview" class="changes-preview">
          <div v-for="c in changedFields" :key="c.key" class="change-row">
            <span class="change-key">{{ c.label }}</span>
            <span class="change-old">{{ c.oldVal }}</span>
            <span class="change-arrow">→</span>
            <span class="change-new">{{ c.newVal }}</span>
            <span v-if="c.restart" class="cfg-restart-tag">需重启</span>
          </div>
        </div>
      </div>

      <div v-for="group in visibleGroups" :key="group.key" v-show="activeGroup === group.key" class="config-section">
        <div class="card config-card">
          <div class="card-header"><span class="card-icon">{{ group.icon }}</span><span class="card-title">{{ group.title }}</span></div>
          <div class="config-desc">{{ group.desc }}</div>
          <div class="config-items">
            <div v-for="item in group.items" :key="item.key || item.field" class="cfg-item">
              <div class="cfg-item-head">
                <span class="cfg-item-label">{{ item.label }}</span>
                <span v-if="item.restart" class="cfg-restart-tag">需重启</span>
                <span v-if="item.field ? isRLChanged(item.field) : isChanged(item.key)" class="cfg-changed-dot">●</span>
              </div>
              <div class="cfg-item-desc">{{ item.desc }}</div>
              <select v-if="item.options" v-model="form[item.key]" class="cfg-select"><option v-for="o in item.options" :key="o.value" :value="o.value">{{ o.label }}</option></select>
              <div v-else-if="item.type === 'toggle'" class="cfg-toggle-row"><label class="toggle-switch"><input type="checkbox" v-model="form[item.key]" /><span class="toggle-slider"></span></label><span class="cfg-toggle-label">{{ form[item.key] ? '已开启' : '已关闭' }}</span></div>
              <div v-else-if="item.field" class="cfg-inline"><input type="number" :value="form.rate_limit[item.field]" @input="form.rate_limit[item.field] = Number($event.target.value)" class="cfg-input-num" :min="item.min" :max="item.max" :step="item.step" /><span v-if="item.unit" class="cfg-unit">{{ item.unit }}</span></div>
              <div v-else-if="item.type === 'number'" class="cfg-inline"><input type="number" v-model.number="form[item.key]" class="cfg-input-num" :min="item.min" :max="item.max" :step="item.step" /><span v-if="item.unit" class="cfg-unit">{{ item.unit }}</span></div>
              <input v-else type="text" v-model="form[item.key]" class="cfg-input" :class="{ 'cfg-input-wide': item.wide }" :placeholder="item.placeholder" />
              <span v-if="item.key && errors[item.key]" class="cfg-error">{{ errors[item.key] }}</span>
            </div>
          </div>
        </div>
      </div>

      <div v-show="activeGroup === 'alerts'" class="config-section" style="margin-top:0">
        <div class="card config-card" style="margin-bottom:16px">
          <div class="card-header"><span class="card-icon">🔔</span><span class="card-title">告警通知</span></div>
          <div class="config-desc">Webhook 通知与告警频率设置</div>
          <div class="config-items">
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">Webhook URL</span><span v-if="isChanged('alert_webhook')" class="cfg-changed-dot">●</span></div>
              <div class="cfg-item-desc">告警推送目标地址</div>
              <input type="text" v-model="form.alert_webhook" class="cfg-input cfg-input-wide" placeholder="https://your-webhook-url" />
            </div>
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">告警格式</span></div>
              <div class="cfg-item-desc">推送消息格式</div>
              <select v-model="form.alert_format" class="cfg-select"><option value="generic">通用 JSON</option><option value="lanxin">蓝信</option></select>
            </div>
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">最小间隔</span></div>
              <div class="cfg-item-desc">两次告警之间的最小间隔，防告警风暴</div>
              <div class="cfg-inline"><input type="number" v-model.number="form.alert_min_interval" class="cfg-input-num" min="0" max="3600" step="10" /><span class="cfg-unit">秒</span></div>
            </div>
          </div>
          <div class="cfg-actions-row">
            <button class="btn btn-sm" @click="$emit('test-alert')" :disabled="!form.alert_webhook || alertTesting">{{ alertTesting ? '发送中...' : '📤 测试告警' }}</button>
            <span v-if="alertTestResult" class="cfg-hint" :style="{ color: alertTestResult.ok ? 'var(--color-success)' : 'var(--color-danger)' }">{{ alertTestResult.msg }}</span>
          </div>
        </div>
        <div class="card config-card">
          <div class="card-header"><span class="card-icon">📋</span><span class="card-title">最近告警</span><div class="card-actions"><button class="btn btn-ghost btn-sm" @click="$emit('load-alert-history')">刷新</button></div></div>
          <Skeleton v-if="alertsLoading" type="table" />
          <div v-else-if="!alerts.length" class="empty"><div class="empty-icon">🔕</div>暂无告警记录</div>
          <div v-else class="alert-list">
            <div v-for="a in alerts" :key="a.id" class="alert-item">
              <div class="alert-meta"><span class="alert-dir" :class="'dir-' + a.direction">{{ a.direction === 'inbound' ? '⬇ 入站' : '⬆ 出站' }}</span><span class="alert-time">{{ fmtTime(a.timestamp) }}</span><span class="alert-sender">{{ a.sender_id || '--' }}</span></div>
              <div class="alert-reason">{{ a.reason }}</div>
            </div>
          </div>
        </div>
      </div>
      <!-- 人工确认配置 -->
      <div v-show="activeGroup === 'human-confirm'" class="config-section">
        <div class="card config-card">
          <div class="card-header"><span class="card-icon">🤝</span><span class="card-title">人工确认引擎</span></div>
          <div class="config-desc">当 IM 消息命中 confirm 规则时，挂起请求并向用户发送 Y/N 提示，根据回复决定放行或拦截。引擎开关在"检测引擎"Tab 管理。</div>
          <div class="config-items">
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">确认超时</span></div>
              <div class="cfg-item-desc">用户未回复时，等待多少秒后执行超时动作</div>
              <div class="cfg-inline"><input type="number" v-model.number="form.human_confirm.timeout_sec" class="cfg-input-num" min="5" max="300" step="5" /><span class="cfg-unit">秒</span></div>
            </div>
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">超时默认动作</span></div>
              <div class="cfg-item-desc">超时后对原始请求执行的全局默认动作（可被规则级 timeout_action 覆盖）</div>
              <select v-model="form.human_confirm.timeout_action" class="cfg-select">
                <option value="block">block（拦截）</option>
                <option value="pass">pass（放行）</option>
              </select>
            </div>
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">放行关键词</span></div>
              <div class="cfg-item-desc">每行一个，用户回复匹配任意一个则放行原始请求</div>
              <textarea v-model="form.human_confirm.confirm_keywords_text" rows="4" class="cfg-input cfg-input-wide cfg-textarea" placeholder="Y&#10;y&#10;是&#10;继续"></textarea>
            </div>
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">取消关键词</span></div>
              <div class="cfg-item-desc">每行一个，用户回复匹配任意一个则取消原始请求</div>
              <textarea v-model="form.human_confirm.cancel_keywords_text" rows="4" class="cfg-input cfg-input-wide cfg-textarea" placeholder="N&#10;n&#10;否&#10;取消"></textarea>
            </div>
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">确认提示消息</span></div>
              <div class="cfg-item-desc">触发确认时向用户发送的提示文本</div>
              <input type="text" v-model="form.human_confirm.confirm_msg" class="cfg-input cfg-input-wide" placeholder="⚠️ 触发安全规则，请回复 Y 放行或 N 取消（15秒内有效）" />
            </div>
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">放行反馈消息</span></div>
              <div class="cfg-item-desc">用户确认放行后发送的反馈文本</div>
              <input type="text" v-model="form.human_confirm.confirmed_msg" class="cfg-input cfg-input-wide" placeholder="✅ 已放行" />
            </div>
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">取消反馈消息</span></div>
              <div class="cfg-item-desc">用户取消请求后发送的反馈文本</div>
              <input type="text" v-model="form.human_confirm.cancelled_msg" class="cfg-input cfg-input-wide" placeholder="🚫 已取消" />
            </div>
            <div class="cfg-item">
              <div class="cfg-item-head"><span class="cfg-item-label">超时反馈消息</span></div>
              <div class="cfg-item-desc">超时后发送给用户的反馈文本</div>
              <input type="text" v-model="form.human_confirm.timeout_msg" class="cfg-input cfg-input-wide" placeholder="⏰ 超时已取消" />
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
  configLoading: Boolean,
  configGroups: Array,
  activeGroup: String,
  hasChanges: Boolean,
  changedFields: Array,
  showChangesPreview: Boolean,
  configSaving: Boolean,
  visibleGroups: Array,
  form: Object,
  errors: Object,
  alertsLoading: Boolean,
  alerts: Array,
  alertTesting: Boolean,
  alertTestResult: Object,
  isChanged: Function,
  isRLChanged: Function,
  fmtTime: Function,
})

defineEmits([
  'open-wizard',
  'update:activeGroup',
  'toggle-changes-preview',
  'save-config',
  'reset-config',
  'test-alert',
  'load-alert-history',
])
</script>

<style scoped>
.config-group-nav { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 16px; }
.config-group-btn { padding: 6px 14px; border-radius: 20px; border: 1px solid var(--border-default); background: var(--bg-elevated); color: var(--text-secondary); font-size: var(--text-sm); cursor: pointer; transition: all var(--transition-fast); white-space: nowrap; }
.config-group-btn:hover { border-color: var(--color-primary); color: var(--text-primary); }
.config-group-btn.active { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.changes-bar { background: rgba(99,102,241,0.08); border: 1px solid rgba(99,102,241,0.2); border-radius: var(--radius-md); padding: 10px 14px; margin-bottom: 16px; }
.changes-bar-inner { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); flex-wrap: wrap; }
.changes-dot { color: var(--color-warning); font-size: 14px; }
.changes-preview { margin-top: 10px; padding-top: 10px; border-top: 1px solid rgba(99,102,241,0.15); }
.change-row { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); padding: 3px 0; font-family: var(--font-mono); }
.change-key { color: var(--text-secondary); min-width: 120px; }
.change-old { color: var(--color-danger); text-decoration: line-through; }
.change-arrow { color: var(--text-tertiary); }
.change-new { color: var(--color-success); font-weight: 600; }
.config-card { margin-bottom: 16px; }
.config-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin-bottom: var(--space-3); }
.config-items { display: flex; flex-direction: column; gap: 0; }
.cfg-item { padding: 12px 0; border-bottom: 1px solid var(--border-subtle); }
.cfg-item:last-child { border-bottom: none; }
.cfg-item-head { display: flex; align-items: center; gap: var(--space-2); margin-bottom: 2px; }
.cfg-item-label { font-size: var(--text-sm); font-weight: 600; color: var(--text-primary); }
.cfg-item-desc { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: 8px; }
.cfg-restart-tag { font-size: 10px; padding: 1px 6px; border-radius: 9999px; background: rgba(245,158,11,0.15); color: var(--color-warning); font-weight: 500; }
.cfg-changed-dot { color: var(--color-primary); font-size: 10px; }
.cfg-input { background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); outline: none; width: 200px; max-width: 100%; font-family: var(--font-mono); transition: border-color var(--transition-fast); }
.cfg-input:focus { border-color: var(--color-primary); box-shadow: 0 0 0 2px var(--color-primary-dim); }
.cfg-input-wide { width: 360px; }
.cfg-textarea { resize: vertical; font-family: var(--font-mono); height: auto; }
.cfg-input-num { width: 100px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); outline: none; font-family: var(--font-mono); }
.cfg-input-num:focus { border-color: var(--color-primary); }
.cfg-select { background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); outline: none; min-width: 140px; cursor: pointer; }
.cfg-select:focus { border-color: var(--color-primary); }
.cfg-inline { display: flex; align-items: center; gap: var(--space-2); }
.cfg-unit { font-size: var(--text-xs); color: var(--text-tertiary); }
.cfg-error { display: block; font-size: var(--text-xs); color: var(--color-danger); margin-top: 4px; }
.cfg-toggle-row { display: flex; align-items: center; gap: var(--space-2); }
.cfg-toggle-label { font-size: var(--text-sm); color: var(--text-secondary); }
.cfg-actions-row { display: flex; align-items: center; gap: var(--space-2); margin-top: var(--space-3); padding-top: var(--space-3); border-top: 1px solid var(--border-subtle); }
.cfg-hint { font-size: var(--text-xs); }
.alert-list { max-height: 360px; overflow-y: auto; }
.alert-item { padding: 8px 0; border-bottom: 1px solid var(--border-subtle); }
.alert-item:last-child { border-bottom: none; }
.alert-meta { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); margin-bottom: 4px; }
.alert-dir { padding: 1px 6px; border-radius: 4px; font-weight: 500; }
.dir-inbound { background: rgba(99,102,241,0.1); color: var(--color-primary); }
.dir-outbound { background: rgba(245,158,11,0.1); color: var(--color-warning); }
.alert-time { color: var(--text-tertiary); }
.alert-sender { color: var(--text-secondary); font-family: var(--font-mono); }
.alert-reason { font-size: var(--text-sm); color: var(--text-primary); }
.toggle-switch { position: relative; display: inline-block; width: 36px; height: 20px; cursor: pointer; }
.toggle-switch input { opacity: 0; width: 0; height: 0; }
.toggle-slider { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: 20px; transition: .3s; }
.toggle-slider:before { content: ''; position: absolute; height: 14px; width: 14px; left: 2px; bottom: 2px; background: var(--text-tertiary); border-radius: 50%; transition: .3s; }
.toggle-switch input:checked + .toggle-slider { background: var(--color-primary); border-color: var(--color-primary); }
.toggle-switch input:checked + .toggle-slider:before { transform: translateX(16px); background: #fff; }
@media (max-width: 640px) {
  .cfg-input-wide { width: 100%; }
  .config-group-nav { gap: 4px; }
  .config-group-btn { padding: 4px 10px; font-size: var(--text-xs); }
  .changes-bar-inner { flex-direction: column; align-items: flex-start; }
}
</style>
