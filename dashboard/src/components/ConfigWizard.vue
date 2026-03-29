<template>
  <div v-if="visible" class="wizard-overlay" @click.self="emit('close')">
    <div class="wizard-modal">
      <div class="wizard-header">
        <div>
          <div class="wizard-eyebrow">快速配置向导</div>
          <h3>4 步生成新手配置模板</h3>
          <p>填写基础连接、安全选项和通道参数，最后下载 config.yaml 模板。</p>
        </div>
        <button class="wizard-close" @click="emit('close')">✕</button>
      </div>

      <div class="wizard-steps">
        <button v-for="(item, idx) in steps" :key="item.key" class="wizard-step" :class="{ active: step === idx + 1, done: step > idx + 1 }" @click="step = idx + 1">
          <span>{{ idx + 1 }}</span>
          <div>
            <strong>{{ item.title }}</strong>
            <small>{{ item.desc }}</small>
          </div>
        </button>
      </div>

      <div class="wizard-body">
        <section v-if="step === 1" class="wizard-panel">
          <h4>Step 1 · 基础连接</h4>
          <p class="wizard-tip">先把监听端口、管理 token、上游地址填好。示例值适合本机调试。</p>
          <div class="wizard-grid two">
            <label>
              <span>入站监听端口</span>
              <input v-model="form.listen_inbound" placeholder=":18443" />
            </label>
            <label>
              <span>出站监听端口</span>
              <input v-model="form.listen_outbound" placeholder=":18444" />
            </label>
            <label>
              <span>Dashboard 端口</span>
              <input v-model.number="form.management_port" type="number" placeholder="9090" />
            </label>
            <label>
              <span>管理 Token</span>
              <input v-model="form.management_token" placeholder="your-secret-token" />
            </label>
            <label>
              <span>上游地址</span>
              <input v-model="form.upstream_address" placeholder="127.0.0.1" />
            </label>
            <label>
              <span>上游端口</span>
              <input v-model.number="form.upstream_port" type="number" placeholder="19444" />
            </label>
          </div>
        </section>

        <section v-else-if="step === 2" class="wizard-panel">
          <h4>Step 2 · 通道类型</h4>
          <p class="wizard-tip">选择你的 IM 通道，并补充解密密钥。若暂时没有，可先保留示例占位符。</p>
          <div class="wizard-grid two">
            <label>
              <span>通道类型</span>
              <select v-model="form.channel_type">
                <option value="lanxin">蓝信 lanxin</option>
                <option value="feishu">飞书 feishu</option>
                <option value="dingtalk">钉钉 dingtalk</option>
                <option value="wecom">企微 wecom</option>
                <option value="generic">通用 generic</option>
              </select>
            </label>
            <label>
              <span>加密密钥</span>
              <input v-model="form.encryption_key" placeholder="your-aes-key" />
            </label>
          </div>
          <div class="wizard-example">示例：蓝信常用 channel_type=lanxin；如果你的通道不要求消息解密，可后续再细化配置。</div>
        </section>

        <section v-else-if="step === 3" class="wizard-panel">
          <h4>Step 3 · 安全策略</h4>
          <p class="wizard-tip">为新手默认开启基础防护；LLM Proxy 按需打开。</p>
          <div class="wizard-switches">
            <label class="wizard-switch-card">
              <input v-model="form.inbound_detect_enabled" type="checkbox" />
              <div>
                <strong>入站检测</strong>
                <p>对进入 Agent 的消息做规则匹配与风险检测。</p>
              </div>
            </label>
            <label class="wizard-switch-card">
              <input v-model="form.outbound_audit_enabled" type="checkbox" />
              <div>
                <strong>出站审计</strong>
                <p>对发往外部的内容执行审计与日志留存。</p>
              </div>
            </label>
            <label class="wizard-switch-card">
              <input v-model="form.llm_proxy_enabled" type="checkbox" />
              <div>
                <strong>LLM Proxy</strong>
                <p>如果你要代理 OpenAI / DeepSeek 等模型请求，再启用它。</p>
              </div>
            </label>
          </div>
          <div v-if="form.llm_proxy_enabled" class="wizard-grid two" style="margin-top: 16px;">
            <label>
              <span>LLM Proxy 监听端口</span>
              <input v-model="form.llm_proxy_listen" placeholder=":18445" />
            </label>
            <label>
              <span>说明</span>
              <input value="示例 target 会写入模板注释区" disabled />
            </label>
          </div>
        </section>

        <section v-else class="wizard-panel">
          <h4>Step 4 · 预览与下载</h4>
          <p class="wizard-tip">系统会调用后端验证 API，告诉你当前模板属于 L0 / L1 / L2 哪一级。</p>
          <div class="wizard-actions-row">
            <button class="btn btn-sm btn-primary" @click="validateConfig" :disabled="validating">{{ validating ? '验证中...' : '校验模板' }}</button>
            <button class="btn btn-sm" @click="downloadConfig">下载 config.yaml</button>
            <span v-if="validateResult" class="wizard-level">级别：{{ validateResult.level }}</span>
          </div>
          <div v-if="validateResult" class="wizard-validate-box">
            <div :class="['wizard-badge', validateResult.valid ? 'ok' : 'warn']">{{ validateResult.valid ? '校验通过' : '还有待完善项' }}</div>
            <div v-if="validateResult.missing_required?.length">
              <strong>缺失必填：</strong>
              <span>{{ validateResult.missing_required.join('、') }}</span>
            </div>
            <div v-if="validateResult.invalid_fields?.length">
              <strong>格式问题：</strong>
              <ul>
                <li v-for="item in validateResult.invalid_fields" :key="item.field + item.message">{{ item.field }}：{{ item.message }}</li>
              </ul>
            </div>
          </div>
          <textarea class="wizard-preview" :value="generatedYaml" readonly />
        </section>
      </div>

      <div class="wizard-footer">
        <button class="btn btn-sm btn-ghost" @click="emit('close')">取消</button>
        <div class="wizard-footer-right">
          <button v-if="step > 1" class="btn btn-sm" @click="step--">上一步</button>
          <button v-if="step < 4" class="btn btn-sm btn-primary" @click="step++">下一步</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { apiPost } from '../api.js'

const props = defineProps({ visible: { type: Boolean, default: false } })
const emit = defineEmits(['close'])

const step = ref(1)
const validating = ref(false)
const validateResult = ref(null)
const steps = [
  { key: 'base', title: '基础连接', desc: '地址、端口、token' },
  { key: 'channel', title: '通道类型', desc: 'IM 与加密密钥' },
  { key: 'security', title: '安全策略', desc: '三项开关' },
  { key: 'preview', title: '预览下载', desc: '校验 + 导出' },
]

const form = reactive({
  listen_inbound: ':18443',
  listen_outbound: ':18444',
  management_port: 9090,
  management_token: 'your-secret-token',
  upstream_address: '127.0.0.1',
  upstream_port: 19444,
  channel_type: 'lanxin',
  encryption_key: 'your-aes-key',
  inbound_detect_enabled: true,
  outbound_audit_enabled: true,
  llm_proxy_enabled: false,
  llm_proxy_listen: ':18445',
})

const generatedYaml = computed(() => {
  const lines = [
    '# ===== Level 0: 快速开始（5 个必填字段） =====',
    `listen_inbound: "${form.listen_inbound}"`,
    `listen_outbound: "${form.listen_outbound}"`,
    `management_port: ${form.management_port}`,
    `management_token: "${form.management_token}"`,
    'upstreams:',
    '  - id: "default"',
    `    address: "${form.upstream_address}"`,
    `    port: ${form.upstream_port}`,
    '',
    '# ===== Level 1: 基础安全（推荐配置） =====',
    `channel_type: "${form.channel_type}"`,
    `encryption_key: "${form.encryption_key}"`,
    `inbound_detect_enabled: ${form.inbound_detect_enabled}`,
    `outbound_audit_enabled: ${form.outbound_audit_enabled}`,
  ]
  if (form.llm_proxy_enabled) {
    lines.push('', '# ===== Level 2: 高级功能（按需启用） =====')
    lines.push('llm_proxy:')
    lines.push('  enabled: true')
    lines.push(`  listen: "${form.llm_proxy_listen}"`)
    lines.push('  # targets:')
    lines.push('  #   - name: "deepseek"')
    lines.push('  #     upstream: "https://api.deepseek.com"')
  }
  return lines.join('\n')
})

async function validateConfig() {
  validating.value = true
  try {
    validateResult.value = await apiPost('/api/v1/config/validate', { yaml: generatedYaml.value })
  } catch (e) {
    validateResult.value = { valid: false, level: 'L0', missing_required: [], invalid_fields: [{ field: 'request', message: e.message }] }
  }
  validating.value = false
}

function downloadConfig() {
  const blob = new Blob([generatedYaml.value], { type: 'text/yaml;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = 'config.yaml'
  a.click()
  URL.revokeObjectURL(a.href)
}
</script>

<style scoped>
.wizard-overlay { position: fixed; inset: 0; background: rgba(2, 6, 23, 0.78); display: flex; align-items: center; justify-content: center; z-index: 1200; padding: 24px; }
.wizard-modal { width: min(980px, 100%); max-height: 90vh; overflow: auto; border-radius: 24px; border: 1px solid rgba(129, 140, 248, 0.28); background: linear-gradient(180deg, rgba(15,23,42,.98), rgba(17,24,39,.98)); color: #e5e7eb; box-shadow: 0 30px 80px rgba(0,0,0,.45); }
.wizard-header, .wizard-footer { display: flex; justify-content: space-between; align-items: center; padding: 24px 28px; border-bottom: 1px solid rgba(99,102,241,.16); }
.wizard-footer { border-bottom: none; border-top: 1px solid rgba(99,102,241,.16); }
.wizard-eyebrow { color: #818cf8; font-size: 12px; letter-spacing: .08em; text-transform: uppercase; margin-bottom: 8px; }
.wizard-header h3 { margin: 0 0 6px; font-size: 24px; color: #eef2ff; }
.wizard-header p { margin: 0; color: #a5b4fc; font-size: 14px; }
.wizard-close { background: transparent; color: #c7d2fe; border: none; font-size: 20px; cursor: pointer; }
.wizard-steps { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; padding: 20px 28px 0; }
.wizard-step { display: flex; gap: 12px; align-items: center; background: rgba(30,41,59,.78); border: 1px solid rgba(99,102,241,.18); color: #cbd5e1; border-radius: 18px; padding: 14px; cursor: pointer; }
.wizard-step span { width: 28px; height: 28px; border-radius: 999px; display: inline-flex; align-items: center; justify-content: center; background: rgba(79,70,229,.18); color: #c7d2fe; font-weight: 700; }
.wizard-step.active { border-color: #6366f1; background: rgba(49,46,129,.32); }
.wizard-step.done span, .wizard-step.active span { background: #4f46e5; color: white; }
.wizard-step small { display: block; color: #94a3b8; margin-top: 4px; }
.wizard-body { padding: 24px 28px; }
.wizard-panel h4 { margin: 0 0 10px; font-size: 22px; color: #eef2ff; }
.wizard-tip, .wizard-example { color: #a5b4fc; font-size: 14px; margin-bottom: 18px; }
.wizard-grid { display: grid; gap: 16px; }
.wizard-grid.two { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.wizard-grid label, .wizard-switch-card { display: flex; flex-direction: column; gap: 8px; background: rgba(30,41,59,.72); border: 1px solid rgba(99,102,241,.16); border-radius: 18px; padding: 16px; }
.wizard-grid span { font-size: 13px; color: #c7d2fe; }
.wizard-grid input, .wizard-grid select { background: rgba(15,23,42,.8); border: 1px solid rgba(99,102,241,.24); border-radius: 12px; color: #f8fafc; padding: 12px 14px; }
.wizard-switches { display: grid; gap: 12px; }
.wizard-switch-card { flex-direction: row; align-items: flex-start; gap: 14px; }
.wizard-switch-card input { margin-top: 4px; accent-color: #6366f1; }
.wizard-switch-card p { margin: 6px 0 0; color: #94a3b8; font-size: 13px; }
.wizard-actions-row { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; }
.wizard-level { color: #818cf8; font-weight: 700; }
.wizard-validate-box { background: rgba(30,41,59,.72); border: 1px solid rgba(99,102,241,.18); border-radius: 16px; padding: 14px 16px; margin-bottom: 16px; color: #cbd5e1; }
.wizard-badge { display: inline-flex; padding: 4px 10px; border-radius: 999px; font-size: 12px; margin-bottom: 10px; }
.wizard-badge.ok { background: rgba(34,197,94,.12); color: #4ade80; }
.wizard-badge.warn { background: rgba(245,158,11,.12); color: #fbbf24; }
.wizard-preview { width: 100%; min-height: 300px; background: #020617; color: #c7d2fe; border: 1px solid rgba(99,102,241,.2); border-radius: 16px; padding: 16px; font-family: var(--font-mono, monospace); }
.wizard-footer-right { display: flex; gap: 8px; }
@media (max-width: 860px) {
  .wizard-steps, .wizard-grid.two { grid-template-columns: 1fr; }
}
</style>
