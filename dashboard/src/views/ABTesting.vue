<template>
  <div class="page-container">
    <!-- 标题栏 -->
    <div class="page-header">
      <div>
        <h1 class="page-title">🔬 Prompt A/B 测试</h1>
        <p class="page-desc">同时运行两个 Prompt 版本，量化比安全性</p>
      </div>
      <button class="btn btn-primary" @click="showCreateModal = true">+ 创建测试</button>
    </div>

    <!-- 活跃测试卡片 -->
    <div v-for="test in activeTests" :key="test.id" class="ab-card" :class="'ab-card-' + test.status">
      <div class="ab-card-header">
        <div class="ab-card-title">
          <span class="ab-name">{{ test.name }}</span>
          <span class="ab-badge" :class="'badge-' + test.status">{{ statusLabel(test.status) }}</span>
          <span v-if="test.status === 'running' && test.started_at" class="ab-duration">已进行 {{ duration(test.started_at) }}</span>
          <span v-if="test.confidence > 0" class="ab-confidence">置信度 {{ test.confidence.toFixed(1) }}%</span>
        </div>
      </div>

      <div class="ab-card-body">
        <!-- 双列对比 -->
        <div class="ab-comparison">
          <div class="ab-version">
            <div class="ab-version-header">
              <span class="ab-version-label">版本 A (对照组 {{ test.traffic_a }}%)</span>
              <span v-if="test.winner === 'A'" class="ab-winner">🏆</span>
            </div>
            <div class="ab-metrics" v-if="test.result_a">
              <div class="ab-metric-row">
                <span class="ab-metric-label">请求</span>
                <span class="ab-metric-value">{{ test.result_a.total_requests }}</span>
              </div>
              <div class="ab-metric-row">
                <span class="ab-metric-label">拦截率</span>
                <span class="ab-metric-value">{{ pct(test.result_a.block_rate) }}</span>
              </div>
              <div class="ab-metric-row">
                <span class="ab-metric-label">Canary</span>
                <span class="ab-metric-value" :class="betterClass(test.result_a.canary_leak_rate, test.result_b?.canary_leak_rate, true)">{{ pct(test.result_a.canary_leak_rate) }}</span>
              </div>
              <div class="ab-metric-row">
                <span class="ab-metric-label">错误</span>
                <span class="ab-metric-value">{{ pct(test.result_a.error_rate) }}</span>
              </div>
              <div class="ab-metric-row ab-metric-score">
                <span class="ab-metric-label">安全分</span>
                <span class="ab-metric-value" :class="scoreClass(test.result_a.security_score)">{{ test.result_a.security_score.toFixed(1) }}</span>
              </div>
            </div>
            <div v-else class="ab-no-data">暂无数据</div>
          </div>

          <div class="ab-divider"></div>

          <div class="ab-version">
            <div class="ab-version-header">
              <span class="ab-version-label">版本 B (实验组 {{ test.traffic_b }}%)</span>
              <span v-if="test.winner === 'B'" class="ab-winner">🏆</span>
            </div>
            <div class="ab-metrics" v-if="test.result_b">
              <div class="ab-metric-row">
                <span class="ab-metric-label">请求</span>
                <span class="ab-metric-value">{{ test.result_b.total_requests }}</span>
              </div>
              <div class="ab-metric-row">
                <span class="ab-metric-label">拦截率</span>
                <span class="ab-metric-value">{{ pct(test.result_b.block_rate) }}</span>
              </div>
              <div class="ab-metric-row">
                <span class="ab-metric-label">Canary</span>
                <span class="ab-metric-value" :class="betterClass(test.result_b?.canary_leak_rate, test.result_a?.canary_leak_rate, true)">{{ pct(test.result_b.canary_leak_rate) }}</span>
              </div>
              <div class="ab-metric-row">
                <span class="ab-metric-label">错误</span>
                <span class="ab-metric-value">{{ pct(test.result_b.error_rate) }}</span>
              </div>
              <div class="ab-metric-row ab-metric-score">
                <span class="ab-metric-label">安全分</span>
                <span class="ab-metric-value" :class="scoreClass(test.result_b.security_score)">{{ test.result_b.security_score.toFixed(1) }}</span>
              </div>
            </div>
            <div v-else class="ab-no-data">暂无数据</div>
          </div>
        </div>

        <!-- 结论 -->
        <div class="ab-conclusion" v-if="test.confidence > 0 || test.recommendation">
          <div v-if="test.confidence > 0" class="ab-conclusion-line">
            📊 置信度: {{ test.confidence.toFixed(1) }}%
            <span v-if="test.winner === 'B'"> — 版本 B 在安全性上显著优于版本 A</span>
            <span v-else-if="test.winner === 'A'"> — 版本 A 在安全性上更优</span>
            <span v-else-if="test.winner === 'tie'"> — 两个版本差异不显著</span>
          </div>
          <div v-if="test.recommendation" class="ab-conclusion-line">💡 {{ test.recommendation }}</div>
        </div>
      </div>

      <div class="ab-card-actions">
        <button v-if="test.status === 'draft'" class="btn btn-sm btn-success" @click="startTest(test.id)">开始测试</button>
        <button v-if="test.status === 'running'" class="btn btn-sm btn-warning" @click="stopTest(test.id)">停止测试</button>
        <button class="btn btn-sm btn-danger" @click="deleteTest(test.id)">删除</button>
      </div>
    </div>

    <div v-if="activeTests.length === 0 && !loading" class="empty-state">
      <div class="empty-icon">🔬</div>
      <div class="empty-text">暂无 A/B 测试</div>
      <div class="empty-hint">创建一个测试，比较不同 Prompt 版本的安全表现</div>
    </div>

    <!-- 历史测试列表 -->
    <div v-if="historyTests.length > 0" class="history-section">
      <h2 class="section-title">📋 历史测试</h2>
      <div class="data-table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>状态</th>
              <th>版本 A</th>
              <th>版本 B</th>
              <th>赢家</th>
              <th>置信度</th>
              <th>安全分 A</th>
              <th>安全分 B</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in historyTests" :key="t.id">
              <td>{{ t.name }}</td>
              <td><span class="ab-badge" :class="'badge-' + t.status">{{ statusLabel(t.status) }}</span></td>
              <td>{{ t.version_a }}</td>
              <td>{{ t.version_b }}</td>
              <td>
                <span v-if="t.winner === 'A'" class="winner-a">🏆 A</span>
                <span v-else-if="t.winner === 'B'" class="winner-b">🏆 B</span>
                <span v-else-if="t.winner === 'tie'" class="winner-tie">⚖️ 平局</span>
                <span v-else>—</span>
              </td>
              <td>{{ t.confidence > 0 ? t.confidence.toFixed(1) + '%' : '—' }}</td>
              <td>{{ t.result_a ? t.result_a.security_score.toFixed(1) : '—' }}</td>
              <td>{{ t.result_b ? t.result_b.security_score.toFixed(1) : '—' }}</td>
              <td>{{ formatTime(t.created_at) }}</td>
              <td><button class="btn btn-sm btn-danger" @click="deleteTest(t.id)">删除</button></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 创建测试弹窗 -->
    <div v-if="showCreateModal" class="modal-overlay" @click.self="showCreateModal = false">
      <div class="modal-box">
        <h3>创建 A/B 测试</h3>
        <div class="form-group">
          <label>测试名称</label>
          <input v-model="newTest.name" placeholder="如：v4.0 安全指令优化" />
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>版本 A 标签</label>
            <input v-model="newTest.version_a" placeholder="如：v3.2-当前版" />
          </div>
          <div class="form-group">
            <label>版本 A Prompt Hash</label>
            <input v-model="newTest.prompt_hash_a" placeholder="prompt_versions 的 hash" />
          </div>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>版本 B 标签</label>
            <input v-model="newTest.version_b" placeholder="如：v4.0-新指令" />
          </div>
          <div class="form-group">
            <label>版本 B Prompt Hash</label>
            <input v-model="newTest.prompt_hash_b" placeholder="prompt_versions 的 hash" />
          </div>
        </div>
        <div class="form-group">
          <label>版本 A 流量比例: {{ newTest.traffic_a }}%</label>
          <input type="range" v-model.number="newTest.traffic_a" min="10" max="90" step="5" />
          <div class="traffic-labels">
            <span>A: {{ newTest.traffic_a }}%</span>
            <span>B: {{ 100 - newTest.traffic_a }}%</span>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showCreateModal = false">取消</button>
          <button class="btn btn-primary" @click="createTest" :disabled="!newTest.name">创建</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'

const tests = ref([])
const loading = ref(true)
const showCreateModal = ref(false)
const newTest = ref({
  name: '', version_a: 'A', prompt_hash_a: '', version_b: 'B', prompt_hash_b: '', traffic_a: 50
})

const activeTests = computed(() => tests.value.filter(t => t.status === 'running' || t.status === 'draft'))
const historyTests = computed(() => tests.value.filter(t => t.status === 'completed' || t.status === 'cancelled'))

async function loadTests() {
  loading.value = true
  try {
    const d = await api('/api/v1/ab-tests?tenant=all')
    tests.value = d.tests || []
  } catch { tests.value = [] }
  loading.value = false
}

async function createTest() {
  try {
    await apiPost('/api/v1/ab-tests', newTest.value)
    showCreateModal.value = false
    newTest.value = { name: '', version_a: 'A', prompt_hash_a: '', version_b: 'B', prompt_hash_b: '', traffic_a: 50 }
    loadTests()
  } catch (e) { alert('创建失败: ' + e.message) }
}

async function startTest(id) {
  if (!confirm('确定开始测试？')) return
  try {
    await apiPost(`/api/v1/ab-tests/${id}/start`)
    loadTests()
  } catch (e) { alert('启动失败: ' + e.message) }
}

async function stopTest(id) {
  if (!confirm('确定停止测试？将计算最终结果。')) return
  try {
    await apiPost(`/api/v1/ab-tests/${id}/stop`)
    loadTests()
  } catch (e) { alert('停止失败: ' + e.message) }
}

async function deleteTest(id) {
  if (!confirm('确定删除此测试？')) return
  try {
    await apiDelete(`/api/v1/ab-tests/${id}`)
    loadTests()
  } catch (e) { alert('删除失败: ' + e.message) }
}

function statusLabel(s) {
  return { draft: '草稿', running: '运行中', completed: '已完成', cancelled: '已取消' }[s] || s
}

function pct(v) {
  if (v == null) return '—'
  return (v * 100).toFixed(1) + '%'
}

function scoreClass(s) {
  if (s >= 80) return 'score-good'
  if (s >= 60) return 'score-warn'
  return 'score-bad'
}

function betterClass(myRate, otherRate, lowerIsBetter) {
  if (myRate == null || otherRate == null) return ''
  if (lowerIsBetter) return myRate < otherRate ? 'metric-better' : myRate > otherRate ? 'metric-worse' : ''
  return myRate > otherRate ? 'metric-better' : myRate < otherRate ? 'metric-worse' : ''
}

function duration(startedAt) {
  if (!startedAt) return ''
  const ms = Date.now() - new Date(startedAt).getTime()
  const h = Math.floor(ms / 3600000)
  const m = Math.floor((ms % 3600000) / 60000)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function formatTime(t) {
  if (!t) return '—'
  return new Date(t).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

onMounted(loadTests)
</script>

<style scoped>
.page-container { padding: var(--space-6); max-width: 1200px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: var(--space-6); }
.page-title { font-size: var(--text-2xl); font-weight: 700; color: var(--text-primary); margin: 0; }
.page-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: var(--space-1); }

/* A/B 卡片 */
.ab-card {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-5); margin-bottom: var(--space-4);
  transition: all var(--transition-fast);
}
.ab-card-running { border-left: 4px solid var(--color-primary); }
.ab-card-completed { border-left: 4px solid var(--color-success); }
.ab-card-draft { border-left: 4px solid var(--text-tertiary); }

.ab-card-header { margin-bottom: var(--space-4); }
.ab-card-title { display: flex; align-items: center; gap: var(--space-3); flex-wrap: wrap; }
.ab-name { font-size: var(--text-lg); font-weight: 600; color: var(--text-primary); }
.ab-badge {
  font-size: 11px; padding: 2px 8px; border-radius: 12px; font-weight: 600;
}
.badge-draft { background: rgba(255,255,255,0.1); color: var(--text-tertiary); }
.badge-running { background: rgba(59,130,246,0.15); color: #60A5FA; }
.badge-completed { background: rgba(34,197,94,0.15); color: #4ADE80; }
.badge-cancelled { background: rgba(239,68,68,0.15); color: #F87171; }
.ab-duration { font-size: var(--text-xs); color: var(--text-tertiary); }
.ab-confidence { font-size: var(--text-xs); color: var(--color-primary); font-weight: 600; }

/* 双列对比 */
.ab-comparison { display: flex; gap: var(--space-4); margin-bottom: var(--space-4); }
.ab-version { flex: 1; background: var(--bg-elevated); border-radius: var(--radius-md); padding: var(--space-4); }
.ab-version-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-3); }
.ab-version-label { font-size: var(--text-sm); font-weight: 600; color: var(--text-secondary); }
.ab-winner { font-size: 1.2rem; }
.ab-divider { width: 1px; background: var(--border-subtle); flex-shrink: 0; }

.ab-metrics { display: flex; flex-direction: column; gap: var(--space-2); }
.ab-metric-row { display: flex; justify-content: space-between; align-items: center; padding: var(--space-1) 0; }
.ab-metric-label { font-size: var(--text-sm); color: var(--text-tertiary); }
.ab-metric-value { font-size: var(--text-sm); font-weight: 600; color: var(--text-primary); font-family: var(--font-mono); }
.ab-metric-score { border-top: 1px solid var(--border-subtle); padding-top: var(--space-2); margin-top: var(--space-1); }
.ab-no-data { font-size: var(--text-sm); color: var(--text-tertiary); text-align: center; padding: var(--space-4); }

.metric-better { color: #4ADE80 !important; }
.metric-better::after { content: ' ← 更好'; font-size: 10px; color: #4ADE80; }
.metric-worse { color: #F87171 !important; }

.score-good { color: #4ADE80 !important; }
.score-warn { color: #FBBF24 !important; }
.score-bad { color: #F87171 !important; }

/* 结论 */
.ab-conclusion {
  background: var(--bg-elevated); border-radius: var(--radius-md); padding: var(--space-3) var(--space-4);
  margin-bottom: var(--space-3);
}
.ab-conclusion-line { font-size: var(--text-sm); color: var(--text-secondary); line-height: 1.6; }

/* 操作按钮 */
.ab-card-actions { display: flex; gap: var(--space-2); }

/* 历史表格 */
.history-section { margin-top: var(--space-8); }
.section-title { font-size: var(--text-lg); font-weight: 600; color: var(--text-primary); margin-bottom: var(--space-4); }
.data-table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-sm); }
.data-table th { text-align: left; padding: var(--space-2) var(--space-3); color: var(--text-tertiary); font-weight: 600; border-bottom: 1px solid var(--border-subtle); white-space: nowrap; }
.data-table td { padding: var(--space-2) var(--space-3); border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); white-space: nowrap; }
.winner-a { color: #60A5FA; font-weight: 600; }
.winner-b { color: #4ADE80; font-weight: 600; }
.winner-tie { color: #FBBF24; }

/* 空状态 */
.empty-state { text-align: center; padding: var(--space-12); color: var(--text-tertiary); }
.empty-icon { font-size: 3rem; margin-bottom: var(--space-3); }
.empty-text { font-size: var(--text-lg); font-weight: 600; }
.empty-hint { font-size: var(--text-sm); margin-top: var(--space-2); }

/* 弹窗 */
.modal-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.6); z-index: 1000;
  display: flex; align-items: center; justify-content: center;
}
.modal-box {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-6);
  width: 560px; max-width: 90vw; max-height: 90vh; overflow-y: auto;
}
.modal-box h3 { margin: 0 0 var(--space-5); font-size: var(--text-lg); color: var(--text-primary); }
.form-group { margin-bottom: var(--space-4); }
.form-group label { display: block; font-size: var(--text-sm); font-weight: 600; color: var(--text-secondary); margin-bottom: var(--space-1); }
.form-group input, .form-group select {
  width: 100%; padding: var(--space-2) var(--space-3);
  background: var(--bg-elevated); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm);
}
.form-group input[type="range"] { padding: 0; }
.form-row { display: flex; gap: var(--space-4); }
.form-row .form-group { flex: 1; }
.traffic-labels { display: flex; justify-content: space-between; font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }
.modal-actions { display: flex; justify-content: flex-end; gap: var(--space-2); margin-top: var(--space-5); }

/* 按钮 */
.btn {
  padding: var(--space-2) var(--space-4); border-radius: var(--radius-md);
  font-size: var(--text-sm); font-weight: 600; cursor: pointer; border: 1px solid var(--border-subtle);
  background: var(--bg-elevated); color: var(--text-secondary); transition: all var(--transition-fast);
}
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover { opacity: 0.9; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-success { background: rgba(34,197,94,0.15); color: #4ADE80; border-color: rgba(34,197,94,0.3); }
.btn-warning { background: rgba(251,191,36,0.15); color: #FBBF24; border-color: rgba(251,191,36,0.3); }
.btn-danger { background: rgba(239,68,68,0.1); color: #F87171; border-color: rgba(239,68,68,0.2); }
.btn-sm { padding: var(--space-1) var(--space-3); font-size: var(--text-xs); }

@media (max-width: 768px) {
  .ab-comparison { flex-direction: column; }
  .ab-divider { height: 1px; width: 100%; }
  .form-row { flex-direction: column; }
}
</style>
