<template>
  <div class="envelopes-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="lock" :size="20" /> 执行信封 + Merkle Tree</h1>
        <p class="page-subtitle">不可篡改的决策信封 — 每条拦截决策都有密封凭证，Merkle Tree 保证链式完整性</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
        <button class="btn btn-primary" @click="openConfigModal"><Icon name="settings" :size="14" /> 配置</button>
      </div>
    </div>

    <div class="stats-grid">
      <StatCard :iconSvg="svgEnvelope" label="总信封数" :value="stats.total ?? 0" color="indigo" />
      <StatCard :iconSvg="svgTree" label="Merkle 批次" :value="stats.merkle_batches ?? 0" color="blue" />
      <StatCard :iconSvg="svgLeaf" label="待处理叶子" :value="stats.pending_leaves ?? 0" color="yellow" :badge="stats.batch_size ? '批次: '+stats.batch_size : ''" />
      <StatCard :iconSvg="svgCheck" label="验证通过" :value="verifiedCount" color="green" />
    </div>

    <div class="tab-bar">
      <button class="tab-btn" :class="{active:activeTab==='envelopes'}" @click="activeTab='envelopes'"><Icon name="file-text" :size="14" /> 信封列表 <span class="tab-count">{{filteredEnvelopes.length}}</span></button>
      <button class="tab-btn" :class="{active:activeTab==='batches'}" @click="activeTab='batches'">🌲 Merkle 批次 <span class="tab-count">{{batches.length}}</span></button>
      <button class="tab-btn" :class="{active:activeTab==='chain'}" @click="activeTab='chain'"><Icon name="check-circle" :size="14" /> 链验证</button>
    </div>

    <div v-if="activeTab==='envelopes'" class="section">
      <div class="section-toolbar">
        <div class="search-box"><Icon name="search" :size="14" /><input v-model="searchQuery" placeholder="搜索信封 ID / TraceID / 域..." /></div>
        <select v-model="filterDecision" class="filter-select"><option value="">全部决策</option><option value="block">阻断</option><option value="warn">告警</option><option value="pass">放行</option><option value="allow">允许</option><option value="deny">拒绝</option><option value="log">记录</option></select>
      </div>
      <DataTable :columns="envelopeColumns" :data="filteredEnvelopes" :loading="loading" :expandable="true" rowKey="id" emptyText="暂无信封记录" emptyDesc="系统运行后会自动生成决策信封">
        <template #cell-id="{value}"><code class="mono-cell">{{truncate(value,12)}}</code></template>
        <template #cell-decision="{value}"><span class="decision-badge" :class="'decision-'+value">{{({block:'阻断',warn:'告警',pass:'放行',allow:'允许',deny:'拒绝',log:'记录'})[value]||value||'-'}}</span></template>
        <template #cell-trace_id="{value}"><code class="mono-cell">{{truncate(value,14)}}</code></template>
        <template #cell-created_at="{row}"><span class="mono-cell tc">{{fmtTime(row.created_at||row.timestamp)}}</span></template>
        <template #actions="{row}"><div class="act-row"><button class="btn btn-sm btn-ghost" @click.stop="verifyEnvelope(row.id)" :disabled="verifying[row.id]">{{verifying[row.id]?'...':'🔍 验证'}}</button><button class="btn btn-sm btn-ghost" @click.stop="showProof(row.id)">📜 证明</button><span v-if="verifyResults[row.id]!==undefined" class="vt" :class="verifyResults[row.id]?'vt-ok':'vt-fail'">{{verifyResults[row.id]?'✅':'❌'}}</span></div></template>
      </DataTable>
    </div>

    <div v-if="activeTab==='batches'" class="section">
      <DataTable :columns="batchColumns" :data="batches" rowKey="id" emptyText="暂无批次">
        <template #cell-id="{value}"><code class="mono-cell">{{ truncate(value, 12) }}</code></template>
        <template #cell-root="{value}"><code class="mono-cell root-hash">{{ truncate(value, 20) }}</code></template>
        <template #cell-leaf_count="{value}"><span class="leaf-badge">{{ value }}</span></template>
        <template #cell-created_at="{row}"><span class="mono-cell tc">{{ fmtTime(row.created_at) }}</span></template>
        <template #actions="{row}"><div class="act-row"><button class="btn btn-sm btn-ghost" @click.stop="verifyBatch(row.id)" :disabled="batchVerifying[row.id]">{{ batchVerifying[row.id] ? '...' : '🧪 验证' }}</button><span v-if="batchVerifyResults[row.id] !== undefined" class="vt" :class="batchVerifyResults[row.id] ? 'vt-ok' : 'vt-fail'">{{ batchVerifyResults[row.id] ? '✅' : '❌' }}</span></div></template>
      </DataTable>
    </div>

    <div v-if="activeTab==='chain'" class="section">
      <div class="card" style="border-color: rgba(99,102,241,.22); margin-bottom:16px;">
        <div class="card-header"><div class="card-title">Merkle 审计验证</div></div>
        <div style="display:flex; gap:12px; flex-wrap:wrap; margin-bottom:12px;">
          <input v-model="verifyRange.start" type="datetime-local" class="filter-select" />
          <input v-model="verifyRange.end" type="datetime-local" class="filter-select" />
          <button class="btn btn-primary" @click="runBatchVerify">开始验证</button>
          <button class="btn" @click="exportVerifyReport" :disabled="!verifyResult">导出报告</button>
        </div>
        <div v-if="verifyResult" class="stats-grid" style="margin-bottom:12px;">
          <StatCard :iconSvg="svgTree" label="总批次数" :value="verifyResult.total_batches" color="indigo" />
          <StatCard :iconSvg="svgCheck" label="通过数" :value="verifyResult.passed" color="green" />
          <StatCard :iconSvg="svgLeaf" label="失败数" :value="verifyResult.failed" color="red" />
        </div>
        <div v-if="verifyResult?.results?.length" class="table-wrap">
          <table><thead><tr><th>批次</th><th>时间</th><th>状态</th><th>叶子数</th><th>失败原因</th></tr></thead><tbody>
            <tr v-for="row in verifyResult.results" :key="row.batch_id" :style="!row.valid ? 'background: rgba(127,29,29,.35)' : ''">
              <td>{{ row.batch_id }}</td><td>{{ fmtTime(row.created_at) }}</td><td>{{ row.valid ? '通过' : '失败' }}</td><td>{{ row.leaf_count }}</td><td>{{ (row.failure_reasons || []).join('；') || '-' }}</td>
            </tr>
          </tbody></table>
        </div>
      </div>

      <div class="chain-panel">
        <div class="chain-hd"><Icon name="check-circle" :size="16" /> Trace 链验证</div>
        <div class="chain-desc">输入 Trace ID 验证完整调用链是否被篡改</div>
        <div class="chain-row"><input v-model="chainTraceId" class="chain-input" placeholder="输入 Trace ID..." /><button class="btn btn-primary" @click="verifyChain" :disabled="chainVerifying || !chainTraceId.trim()">{{ chainVerifying ? '验证中...' : '验证调用链' }}</button></div>
        <div v-if="chainResult" class="chain-result" :class="chainResult.valid ? 'cr-ok' : 'cr-fail'"><div class="cr-icon">{{ chainResult.valid ? '✅' : '❌' }}</div><div class="cr-body"><div class="cr-title">{{ chainResult.valid ? '链完整性验证通过' : '链完整性验证失败' }}</div><div class="cr-meta"><span>Trace: <code>{{ chainTraceId }}</code></span></div><div class="dc">{{ JSON.stringify(chainResult, null, 2) }}</div></div></div>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="proofVisible" class="modal-overlay" @click.self="proofVisible=false"><div class="modal-box modal-lg"><div class="modal-header"><span><Icon name="shield" :size="16" /> Merkle Proof — <code>{{ proofEnvelopeId }}</code></span><button class="btn-close" @click="proofVisible=false">✕</button></div><div class="modal-body"><div v-if="proofLoading" class="loading-hint">加载证明中...</div><div v-else-if="proofError" class="error-banner" style="margin-top:0">{{ proofError }}</div><template v-else><div class="dg"><div class="di"><span class="dl">批次 ID</span><code>{{ proofData?.batch_id || '-' }}</code></div><div class="di"><span class="dl">内容哈希</span><code>{{ proofData?.content_hash || '-' }}</code></div><div class="di"><span class="dl">Merkle Root</span><code class="root-hash">{{ proofData?.root || '-' }}</code></div><div class="di"><span class="dl">验证结果</span><span :class="proofData?.verified ? 'vt vt-ok' : 'vt vt-fail'">{{ proofData?.verified ? '✅ 有效' : '❌ 无效' }}</span></div></div><div class="dp"><div class="dl">Proof 路径（自底向上）</div><pre class="dc">{{ JSON.stringify(proofData?.path || [], null, 2) }}</pre></div></template></div><div class="modal-footer"><button class="btn btn-sm" @click="proofVisible=false">关闭</button></div></div></div>
    </Teleport>

    <Teleport to="body">
      <div v-if="configVisible" class="modal-overlay" @click.self="configVisible=false"><div class="modal-box"><div class="modal-header"><span><Icon name="settings" :size="16" /> 信封系统配置</span><button class="btn-close" @click="configVisible=false">✕</button></div><div class="modal-body"><div class="fg"><label class="fl">启用信封系统</label><label class="toggle"><input type="checkbox" v-model="configForm.enabled" /><span class="toggle-track"><span class="toggle-thumb"></span></span><span class="toggle-txt">{{configForm.enabled?'已启用':'已关闭'}}</span></label></div><div class="fg"><label class="fl">HMAC 密钥</label><input v-model="configForm.secret_key" type="password" class="fi" placeholder="输入 HMAC 签名密钥..." /><span class="fh">用于信封签名和验证的密钥，留空保持现有</span></div></div><div class="modal-footer"><button class="btn btn-sm" @click="configVisible=false">取消</button><button class="btn btn-sm btn-primary" @click="saveConfig" :disabled="configSaving">{{configSaving?'保存中...':'保存'}}</button></div></div></div>
    </Teleport>

    <div v-if="error" class="error-banner" @click="error=''">⚠️ {{error}} <span class="err-x">✕</span></div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import DataTable from '../components/DataTable.vue'
import { api, apiPut, apiPost } from '../api.js'
import { showToast } from '../stores/app.js'

const activeTab = ref('envelopes')
const stats = ref({})
const envelopes = ref([])
const batches = ref([])
const error = ref('')
const loading = ref(false)
const searchQuery = ref('')
const filterDecision = ref('')
const verifying = reactive({})
const verifyResults = reactive({})
const batchVerifying = reactive({})
const batchVerifyResults = reactive({})
const chainTraceId = ref('')
const chainVerifying = ref(false)
const chainResult = ref(null)
const proofVisible = ref(false)
const proofEnvelopeId = ref('')
const proofData = ref(null)
const proofLoading = ref(false)
const proofError = ref('')
const configVisible = ref(false)
const configSaving = ref(false)
const configForm = reactive({ enabled: true, secret_key: '' })
const verifyRange = ref({ start: '', end: '' })
const verifyResult = ref(null)

const verifiedCount = computed(() => Object.values(verifyResults).filter(v => v === true).length)
const filteredEnvelopes = computed(() => {
  let list = envelopes.value
  if (filterDecision.value) list = list.filter(e => e.decision === filterDecision.value)
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase().trim()
    list = list.filter(e => (e.id||'').toLowerCase().includes(q) || (e.trace_id||'').toLowerCase().includes(q) || (e.domain||'').toLowerCase().includes(q))
  }
  return list
})
const envelopeColumns = [{ key: 'id', label: 'ID', sortable: true, width: '140px' },{ key: 'domain', label: '域', sortable: true },{ key: 'decision', label: '决策', sortable: true, width: '90px' },{ key: 'trace_id', label: 'TraceID', width: '160px' },{ key: 'created_at', label: '时间', sortable: true, width: '140px' }]
const batchColumns = [{ key: 'id', label: '批次ID', sortable: true, width: '140px' },{ key: 'root', label: 'Merkle Root', width: '200px' },{ key: 'leaf_count', label: '叶子数', sortable: true, width: '90px' },{ key: 'created_at', label: '创建时间', sortable: true, width: '150px' }]
const svgEnvelope = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M22 7l-10 7L2 7"/></svg>'
const svgTree = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L7 7h3v4H7l5 5 5-5h-3V7h3L12 2z"/></svg>'
const svgLeaf = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 20A7 7 0 0 1 9.8 6.9C15.5 4.9 17 3.5 19 2c1 2 2 4.5 2 8 0 5.5-3.8 10-10 10z"/></svg>'
const svgCheck = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>'

async function loadStats() { try { stats.value = await api('/api/v1/envelopes/stats') } catch(e) { error.value = '统计加载失败: ' + e.message } }
async function loadEnvelopes() { loading.value = true; try { const d = await api('/api/v1/envelopes/list?limit=100'); envelopes.value = d.envelopes || d || [] } catch(e) { error.value = '信封加载失败: ' + e.message } finally { loading.value = false } }
async function loadBatches() { try { const d = await api('/api/v1/envelopes/batches'); batches.value = d.batches || d || [] } catch(e) { error.value = '批次加载失败: ' + e.message } }
async function verifyEnvelope(id) { verifying[id] = true; try { const d = await api('/api/v1/envelopes/verify/' + id); verifyResults[id] = d.valid !== false; showToast(d.valid !== false ? '信封验证通过 ✅' : '信封验证失败 ❌', d.valid !== false ? 'success' : 'error') } catch { verifyResults[id] = false; showToast('验证失败', 'error') } finally { verifying[id] = false } }
async function verifyBatch(id) { batchVerifying[id] = true; try { const d = await api('/api/v1/envelopes/batch/' + id); batchVerifyResults[id] = d.valid !== false; showToast(d.valid !== false ? 'Merkle 批次完整 ✅' : '批次损坏 ❌', d.valid !== false ? 'success' : 'error') } catch { batchVerifyResults[id] = false; showToast('批次验证失败', 'error') } finally { batchVerifying[id] = false } }
async function verifyChain() { if (!chainTraceId.value.trim()) return; chainVerifying.value = true; chainResult.value = null; try { const d = await api('/api/v1/envelopes/chain/' + encodeURIComponent(chainTraceId.value.trim())); chainResult.value = d; showToast(d.valid !== false ? '链验证通过 ✅' : '链验证失败 ❌', d.valid !== false ? 'success' : 'error') } catch(e) { chainResult.value = { valid: false, error: e.message }; showToast('链验证失败', 'error') } finally { chainVerifying.value = false } }
async function showProof(id) { proofEnvelopeId.value = id; proofVisible.value = true; proofLoading.value = true; proofError.value = ''; proofData.value = null; try { proofData.value = await api('/api/v1/envelopes/proof/' + id) } catch(e) { proofError.value = e.message } finally { proofLoading.value = false } }
function openConfigModal() { configForm.enabled = stats.value.enabled !== false; configForm.secret_key = ''; configVisible.value = true }
async function saveConfig() { configSaving.value = true; try { await apiPut('/api/v1/envelopes/config', { enabled: configForm.enabled, secret_key: configForm.secret_key || undefined }); showToast('配置已更新', 'success'); configVisible.value = false; loadStats() } catch(e) { showToast('保存失败: ' + e.message, 'error') } finally { configSaving.value = false } }
async function runBatchVerify() { try { const payload = { start: verifyRange.value.start ? new Date(verifyRange.value.start).toISOString() : '', end: verifyRange.value.end ? new Date(verifyRange.value.end).toISOString() : '' }; verifyResult.value = await apiPost('/api/v1/envelopes/verify', payload); showToast('批次验证完成', 'success') } catch (e) { showToast('验证失败: ' + e.message, 'error') } }
function exportVerifyReport() { if (!verifyResult.value) return; const rows = (verifyResult.value.results || []).map(r => `<tr style="background:${r.valid ? '#0f172a' : '#7f1d1d'}"><td>${r.batch_id}</td><td>${r.created_at}</td><td>${r.valid ? '通过' : '失败'}</td><td>${r.leaf_count}</td><td>${(r.failure_reasons || []).join('<br/>')}</td></tr>`).join(''); const html = `<!doctype html><html><body style="background:#0f172a;color:#e5e7eb;font-family:sans-serif"><h1>Merkle 审计验证报告</h1><p>总批次 ${verifyResult.value.total_batches}，通过 ${verifyResult.value.passed}，失败 ${verifyResult.value.failed}</p><table border="1" cellspacing="0" cellpadding="8">${rows}</table></body></html>`; const blob = new Blob([html], { type: 'text/html' }); const a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = 'merkle-verify-report.html'; a.click(); URL.revokeObjectURL(a.href) }
function loadAll() { error.value = ''; loadStats(); loadEnvelopes(); loadBatches() }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function fmtTime(ts) { if (!ts) return '-'; try { const d = new Date(ts); return d.toLocaleDateString('zh-CN',{month:'2-digit',day:'2-digit'})+' '+d.toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit',second:'2-digit'}) } catch { return ts } }
onMounted(loadAll)
</script>

<style scoped>
.envelopes-page { padding: var(--space-4); max-width: 1200px; }.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; display: flex; align-items: center; gap: 8px; }.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }.header-actions { display: flex; gap: var(--space-2); align-items: center; }.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }.tab-bar { display: flex; gap: var(--space-1); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); }.tab-btn { display: inline-flex; align-items: center; gap: 6px; background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; border-bottom: 2px solid transparent; }.tab-btn.active { color: var(--color-primary); border-bottom-color: var(--color-primary); font-weight: 600; }.tab-count { padding: 0 6px; border-radius: 9999px; font-size: 10px; font-weight: 600; background: rgba(99,102,241,.12); color: var(--color-primary); line-height: 1.6; }.section { margin-bottom: var(--space-4); }.section-toolbar { display: flex; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; align-items: center; }.search-box { display: flex; align-items: center; gap: 8px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 6px 12px; flex: 1; min-width: 200px; max-width: 400px; }.search-box input,.chain-input,.fi{background:none;border:none;outline:none;color:var(--text-primary);font-size:var(--text-sm);width:100%}.filter-select,.fi,.chain-input{background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);padding:8px 10px}.mono-cell{font-family:var(--font-mono);font-size:11px;color:var(--text-secondary)}.tc{color:var(--text-tertiary)}.root-hash{color:#a78bfa}.decision-badge{display:inline-block;padding:2px 8px;border-radius:4px;font-size:10px;font-weight:700}.decision-block,.decision-deny{background:#EF4444;color:#fff}.decision-warn{background:#F59E0B;color:#1a1a2e}.decision-pass,.decision-allow{background:#10B981;color:#fff}.decision-log{background:#6B7280;color:#fff}.leaf-badge{display:inline-block;padding:2px 8px;border-radius:9999px;font-size:11px;font-weight:700;background:rgba(99,102,241,.12);color:var(--color-primary)}.act-row{display:flex;gap:var(--space-1);align-items:center}.vt-ok{color:#10B981}.vt-fail{color:#EF4444}.chain-panel{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:var(--space-5)}.chain-row{display:flex;gap:var(--space-2);align-items:center}.chain-result{display:flex;gap:var(--space-3);margin-top:var(--space-4);padding:var(--space-4);border-radius:var(--radius-lg)}.cr-ok{background:rgba(16,185,129,.08);border:1px solid rgba(16,185,129,.3)}.cr-fail{background:rgba(239,68,68,.08);border:1px solid rgba(239,68,68,.3)}.dc{background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);padding:var(--space-3);font-size:11px;font-family:var(--font-mono);overflow-x:auto;max-height:300px;white-space:pre-wrap}.modal-overlay{position:fixed;inset:0;background:rgba(0,0,0,.5);z-index:1000;display:flex;align-items:center;justify-content:center}.modal-box{background:var(--bg-surface);border:1px solid var(--border-default);border-radius:var(--radius-lg);padding:24px;min-width:360px;max-width:700px}.modal-lg{min-width:500px}.modal-header,.modal-footer{display:flex;align-items:center;justify-content:space-between;margin-bottom:16px}.btn{display:inline-flex;align-items:center;gap:6px;padding:8px 16px;border-radius:var(--radius-md);font-weight:600;font-size:var(--text-sm);cursor:pointer;border:1px solid var(--border-subtle);background:var(--bg-elevated);color:var(--text-secondary)}.btn-primary{background:var(--color-primary);color:#fff;border-color:var(--color-primary)}.btn-sm{padding:4px 10px;font-size:var(--text-xs)}.btn-ghost{background:transparent;border-color:transparent}.error-banner{margin-top:var(--space-3);padding:var(--space-3);background:rgba(239,68,68,.1);border:1px solid rgba(239,68,68,.3);border-radius:var(--radius-md);color:#FCA5A5;font-size:var(--text-sm);cursor:pointer;display:flex;justify-content:space-between}@media (max-width:768px){.stats-grid{grid-template-columns:repeat(2,1fr)}.chain-row{flex-direction:column}}
</style>