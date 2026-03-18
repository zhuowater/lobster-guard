<template>
  <div>
    <!-- 快捷生成按钮 -->
    <div class="gen-actions">
      <button class="gen-btn gen-daily" @click="generate('daily')" :disabled="generating">
        <span v-if="generating && genType==='daily'" class="spinner"></span>
        <span v-else>📊</span>
        生成日报
      </button>
      <button class="gen-btn gen-weekly" @click="generate('weekly')" :disabled="generating">
        <span v-if="generating && genType==='weekly'" class="spinner"></span>
        <span v-else>📈</span>
        生成周报
      </button>
      <button class="gen-btn gen-monthly" @click="generate('monthly')" :disabled="generating">
        <span v-if="generating && genType==='monthly'" class="spinner"></span>
        <span v-else>📋</span>
        生成月报
      </button>
    </div>

    <!-- 报告列表 -->
    <div class="card">
      <div class="card-header">
        <span class="card-icon">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
        </span>
        <span class="card-title">报告列表</span>
        <span class="card-count" v-if="reports.length">{{ reports.length }} 份</span>
      </div>
      <div class="table-wrap" v-if="reports.length">
        <table>
          <thead>
            <tr>
              <th>类型</th>
              <th>标题</th>
              <th>时间范围</th>
              <th>生成时间</th>
              <th>大小</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in reports" :key="r.id">
              <td><span class="type-badge" :class="'type-'+r.type">{{ typeLabel(r.type) }}</span></td>
              <td class="title-cell">{{ r.title }}</td>
              <td>{{ r.time_range }}</td>
              <td>{{ fmtTime(r.created_at) }}</td>
              <td>{{ fmtSize(r.file_size) }}</td>
              <td>
                <span class="status-badge" :class="'status-'+r.status">{{ statusLabel(r.status) }}</span>
              </td>
              <td class="actions-cell">
                <button class="action-btn preview-btn" @click="preview(r)" v-if="r.status==='ready'" title="预览">👁</button>
                <button class="action-btn download-btn" @click="download(r)" v-if="r.status==='ready'" title="下载">⬇️</button>
                <button class="action-btn delete-btn" @click="remove(r)" title="删除">🗑️</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="empty-state" v-else-if="loaded">
        <div class="empty-icon">📄</div>
        <div class="empty-title">暂无报告</div>
        <div class="empty-desc">点击上方按钮生成安全报告</div>
      </div>
      <div class="loading-state" v-else>加载中...</div>
    </div>

    <!-- 预览 Modal -->
    <Teleport to="body">
      <div class="preview-overlay" v-if="previewVisible" @click.self="previewVisible=false">
        <div class="preview-modal">
          <div class="preview-header">
            <span>{{ previewTitle }}</span>
            <button class="preview-close" @click="previewVisible=false">&times;</button>
          </div>
          <iframe class="preview-frame" :src="previewUrl" frameborder="0"></iframe>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { api, apiPost, apiDelete, getToken } from '../api.js'

const route = useRoute()
const reports = ref([])
const loaded = ref(false)
const generating = ref(false)
const genType = ref('')
const previewVisible = ref(false)
const previewTitle = ref('')
const previewUrl = ref('')

async function loadReports() {
  try {
    const d = await api('/api/v1/reports?limit=50')
    reports.value = d.reports || []
  } catch {
    reports.value = []
  }
  loaded.value = true
}

async function generate(type) {
  if (generating.value) return
  if (!confirm('确定要生成' + typeLabel(type) + '吗？')) return
  generating.value = true
  genType.value = type
  try {
    await apiPost('/api/v1/reports/generate', { type })
    await loadReports()
  } catch (e) {
    alert('生成失败: ' + e.message)
  }
  generating.value = false
  genType.value = ''
}

function preview(r) {
  previewTitle.value = r.title
  const base = location.origin
  const token = getToken()
  previewUrl.value = base + '/api/v1/reports/' + r.id + '/download?token=' + encodeURIComponent(token)
  previewVisible.value = true
}

function download(r) {
  const base = location.origin
  const token = getToken()
  const url = base + '/api/v1/reports/' + r.id + '/download'
  const a = document.createElement('a')
  // Use fetch with auth header
  fetch(url, { headers: { 'Authorization': 'Bearer ' + token } })
    .then(res => res.blob())
    .then(blob => {
      a.href = URL.createObjectURL(blob)
      a.download = r.id + '.html'
      a.click()
      URL.revokeObjectURL(a.href)
    })
}

async function remove(r) {
  if (!confirm('确定删除报告 "' + r.title + '"？')) return
  try {
    await apiDelete('/api/v1/reports/' + r.id)
    await loadReports()
  } catch (e) {
    alert('删除失败: ' + e.message)
  }
}

function typeLabel(t) {
  return { daily: '📊 日报', weekly: '📈 周报', monthly: '📋 月报' }[t] || t
}

function statusLabel(s) {
  return { ready: '✅ 就绪', generating: '⏳ 生成中', failed: '❌ 失败' }[s] || s
}

function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? ts : d.toLocaleString('zh-CN', { hour12: false })
}

function fmtSize(bytes) {
  if (!bytes || bytes <= 0) return '--'
  if (bytes < 1024) return bytes + ' B'
  return (bytes / 1024).toFixed(1) + ' KB'
}

onMounted(async () => {
  await loadReports()
  // Auto-generate if coming from overview
  const auto = route.query.auto
  if (auto && (auto === 'daily' || auto === 'weekly' || auto === 'monthly')) {
    generate(auto)
  }
})
</script>

<style scoped>
.gen-actions {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}
.gen-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 24px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-default);
  background: var(--bg-surface);
  color: var(--text-primary);
  font-size: var(--text-sm);
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}
.gen-btn:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}
.gen-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.gen-daily { border-color: #3B82F6; }
.gen-daily:hover:not(:disabled) { background: rgba(59,130,246,0.1); }
.gen-weekly { border-color: #10B981; }
.gen-weekly:hover:not(:disabled) { background: rgba(16,185,129,0.1); }
.gen-monthly { border-color: #8B5CF6; }
.gen-monthly:hover:not(:disabled) { background: rgba(139,92,246,0.1); }

.spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid var(--text-tertiary);
  border-top-color: var(--color-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.type-badge {
  font-size: var(--text-xs);
  font-weight: 600;
  white-space: nowrap;
}
.type-daily { color: #3B82F6; }
.type-weekly { color: #10B981; }
.type-monthly { color: #8B5CF6; }

.title-cell {
  font-weight: 600;
  color: var(--text-primary);
}

.status-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 700;
}
.status-ready { color: #10B981; background: rgba(16,185,129,0.1); }
.status-generating { color: #F59E0B; background: rgba(245,158,11,0.1); }
.status-failed { color: #EF4444; background: rgba(239,68,68,0.1); }

.actions-cell {
  display: flex;
  gap: 4px;
}
.action-btn {
  background: none;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  padding: 4px 8px;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
  line-height: 1;
}
.action-btn:hover {
  background: var(--bg-elevated);
  border-color: var(--border-default);
}
.delete-btn:hover {
  background: rgba(239,68,68,0.1);
  border-color: #EF4444;
}

.card-count {
  margin-left: auto;
  font-size: var(--text-xs);
  color: var(--text-tertiary);
  font-weight: 400;
}

.empty-state {
  text-align: center;
  padding: 48px 24px;
  color: var(--text-tertiary);
}
.empty-icon { font-size: 48px; margin-bottom: 12px; }
.empty-title { font-size: var(--text-base); font-weight: 600; color: var(--text-secondary); margin-bottom: 4px; }
.empty-desc { font-size: var(--text-sm); }

.loading-state {
  text-align: center;
  padding: 48px 24px;
  color: var(--text-tertiary);
  font-size: var(--text-sm);
}

/* 预览 Modal */
.preview-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0,0,0,0.6);
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
}
.preview-modal {
  width: 90vw;
  max-width: 800px;
  height: 85vh;
  background: #fff;
  border-radius: var(--radius-lg);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  box-shadow: var(--shadow-lg);
}
.preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  font-weight: 700;
  font-size: var(--text-sm);
  color: var(--text-primary);
}
.preview-close {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: var(--text-tertiary);
  line-height: 1;
  padding: 0 4px;
}
.preview-close:hover { color: var(--text-primary); }
.preview-frame {
  flex: 1;
  width: 100%;
  border: none;
  background: #fff;
}
</style>
