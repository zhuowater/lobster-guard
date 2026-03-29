<template>
  <div>
    <!-- 快捷生成按钮 -->
    <div class="gen-actions">
      <button class="gen-btn gen-daily" @click="generate('daily')" :disabled="generating">
        <Icon v-if="generating && genType==='daily'" name="loader" :size="18" />
        <Icon v-else name="bar-chart" :size="18" />
        生成日报
      </button>
      <button class="gen-btn gen-weekly" @click="generate('weekly')" :disabled="generating">
        <Icon v-if="generating && genType==='weekly'" name="loader" :size="18" />
        <Icon v-else name="trending-up" :size="18" />
        生成周报
      </button>
      <button class="gen-btn gen-monthly" @click="generate('monthly')" :disabled="generating">
        <Icon v-if="generating && genType==='monthly'" name="loader" :size="18" />
        <Icon v-else name="clipboard" :size="18" />
        生成月报
      </button>
    </div>

    <!-- 报告列表 -->
    <div class="card">
      <div class="card-header">
        <span class="card-icon">
          <Icon name="file-text" :size="18" />
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
              <td>
                <span class="type-badge" :class="'type-'+r.type">
                  <Icon :name="typeIcon(r.type)" :size="14" />
                  {{ typeText(r.type) }}
                </span>
              </td>
              <td class="title-cell">{{ r.title }}</td>
              <td><span class="range-badge">{{ r.time_range }}</span></td>
              <td class="time-cell">{{ fmtTime(r.created_at) }}</td>
              <td>{{ fmtSize(r.file_size) }}</td>
              <td>
                <span class="status-badge" :class="'status-'+r.status">
                  <Icon :name="statusIcon(r.status)" :size="12" />
                  {{ statusText(r.status) }}
                </span>
              </td>
              <td class="actions-cell">
                <button class="action-btn preview-btn" @click="preview(r)" v-if="r.status==='ready'" title="预览">
                  <Icon name="eye" :size="14" />
                </button>
                <button class="action-btn download-btn" @click="download(r)" v-if="r.status==='ready'" title="下载">
                  <Icon name="download" :size="14" />
                </button>
                <button class="action-btn delete-btn" @click="remove(r)" title="删除">
                  <Icon name="trash" :size="14" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="empty-state" v-else-if="loaded">
        <div class="empty-icon-wrap"><Icon name="file-text" :size="48" color="var(--text-quaternary)" /></div>
        <div class="empty-title">暂无报告</div>
        <div class="empty-desc">点击上方按钮生成安全报告</div>
      </div>
      <div class="loading-state" v-else><Icon name="loader" :size="20" /> 加载中...</div>
    </div>

    <!-- 预览 Modal -->
    <Teleport to="body">
      <div class="preview-overlay" v-if="previewVisible" @click.self="previewVisible=false">
        <div class="preview-modal">
          <div class="preview-header">
            <span class="preview-title-wrap">
              <Icon name="file-text" :size="16" />
              {{ previewTitle }}
            </span>
            <button class="preview-close" @click="previewVisible=false">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
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
import Icon from '../components/Icon.vue'

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
  if (!confirm('确定要生成' + typeText(type) + '吗？')) return
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
  fetch(url, { headers: { 'Authorization': 'Bearer ' + token } })
    .then(res => res.blob())
    .then(blob => {
      const a = document.createElement('a')
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

function typeIcon(t) {
  return { daily: 'bar-chart', weekly: 'trending-up', monthly: 'clipboard' }[t] || 'file-text'
}
function typeText(t) {
  return { daily: '日报', weekly: '周报', monthly: '月报' }[t] || t
}

function statusIcon(s) {
  return { ready: 'check-circle', generating: 'loader', failed: 'x-circle' }[s] || 'info'
}
function statusText(s) {
  return { ready: '就绪', generating: '生成中', failed: '失败' }[s] || s
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
.gen-daily { border-color: #3B82F6; color: #3B82F6; }
.gen-daily:hover:not(:disabled) { background: rgba(59,130,246,0.08); }
.gen-weekly { border-color: #10B981; color: #10B981; }
.gen-weekly:hover:not(:disabled) { background: rgba(16,185,129,0.08); }
.gen-monthly { border-color: #8B5CF6; color: #8B5CF6; }
.gen-monthly:hover:not(:disabled) { background: rgba(139,92,246,0.08); }

.type-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
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
.time-cell {
  font-variant-numeric: tabular-nums;
  color: var(--text-secondary);
  font-size: var(--text-xs);
}
.range-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  background: rgba(99,102,241,0.1);
  color: var(--color-primary);
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 10px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 700;
}
.status-ready { color: #10B981; background: rgba(16,185,129,0.1); }
.status-generating { color: #F59E0B; background: rgba(245,158,11,0.1); }
.status-failed { color: #EF4444; background: rgba(239,68,68,0.1); }

.actions-cell {
  display: flex;
  gap: 6px;
}
.action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  padding: 6px;
  cursor: pointer;
  transition: all 0.15s;
  color: var(--text-tertiary);
  line-height: 1;
}
.action-btn:hover {
  color: var(--text-primary);
  background: var(--bg-elevated);
  border-color: var(--border-default);
}
.preview-btn:hover { color: #6366F1; border-color: #6366F1; background: rgba(99,102,241,0.06); }
.download-btn:hover { color: #10B981; border-color: #10B981; background: rgba(16,185,129,0.06); }
.delete-btn:hover { color: #EF4444; border-color: #EF4444; background: rgba(239,68,68,0.06); }

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
.empty-icon-wrap { margin-bottom: 12px; opacity: 0.4; }
.empty-title { font-size: var(--text-base); font-weight: 600; color: var(--text-secondary); margin-bottom: 4px; }
.empty-desc { font-size: var(--text-sm); }

.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
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
  backdrop-filter: blur(4px);
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
  box-shadow: 0 25px 50px -12px rgba(0,0,0,0.5);
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
.preview-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}
.preview-close {
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  cursor: pointer;
  color: var(--text-tertiary);
  padding: 4px;
  transition: all 0.15s;
}
.preview-close:hover {
  color: var(--text-primary);
  background: var(--bg-elevated);
  border-color: var(--border-subtle);
}
.preview-frame {
  flex: 1;
  width: 100%;
  border: none;
  background: #fff;
}
</style>
