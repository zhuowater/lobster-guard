<template>
  <div class="data-table-wrapper">
    <!-- Toolbar: column visibility -->
    <div class="dt-toolbar" v-if="showToolbar">
      <div class="dt-col-toggle" v-if="columns.length > 3">
        <button class="btn btn-ghost btn-sm" @click="colMenuOpen = !colMenuOpen" title="列显隐">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
          列
        </button>
        <div class="dt-col-menu" v-show="colMenuOpen">
          <label v-for="col in columns" :key="col.key" class="dt-col-item">
            <input type="checkbox" :checked="visibleCols.has(col.key)" @change="toggleCol(col.key)" />
            <span>{{ col.label }}</span>
          </label>
        </div>
      </div>
      <slot name="toolbar"></slot>
    </div>

    <!-- Loading -->
    <Skeleton v-if="loading" type="table" />

    <!-- Empty -->
    <EmptyState v-else-if="!sortedData.length"
      :icon-svg="emptySvg"
      :title="emptyText"
      :description="emptyDesc"
    >
      <slot name="empty-hint"></slot>
    </EmptyState>

    <!-- Table -->
    <div v-else class="table-wrap">
      <table>
        <thead>
          <tr>
            <th v-if="expandable" style="width:30px"></th>
            <th
              v-for="col in visibleColumns" :key="col.key"
              :style="col.width ? { width: col.width } : {}"
              :class="{ sortable: col.sortable, sorted: sortKey === col.key }"
              @click="col.sortable && toggleSort(col.key)"
            >
              {{ col.label }}
              <span v-if="col.sortable" class="sort-icon">
                <template v-if="sortKey === col.key">{{ sortDir === 'asc' ? '▲' : '▼' }}</template>
                <template v-else>
                  <span class="sort-icon-neutral">▲▼</span>
                </template>
              </span>
            </th>
            <th v-if="$slots.actions" style="white-space:nowrap">操作</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="(row, idx) in pagedData" :key="rowKeyFn(row, idx)">
            <tr
              :class="rowClassFn(row)"
              @click="expandable && toggleExpand(idx)"
              :style="expandable ? { cursor: 'pointer' } : {}"
            >
              <td v-if="expandable" style="width:30px;text-align:center">
                <svg :class="{ 'expand-arrow': true, expanded: expandedRows.has(idx) }" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
              </td>
              <td v-for="col in visibleColumns" :key="col.key" :style="col.tdStyle || {}">
                <slot :name="'cell-' + col.key" :row="row" :value="getCellValue(row, col)">
                  <span v-html="formatCell(row, col)"></span>
                </slot>
              </td>
              <td v-if="$slots.actions">
                <slot name="actions" :row="row" :index="idx"></slot>
              </td>
            </tr>
            <!-- Expand row -->
            <tr v-if="expandable && expandedRows.has(idx)" class="expand-row">
              <td :colspan="visibleColumns.length + (expandable ? 1 : 0) + ($slots.actions ? 1 : 0)">
                <slot name="expand" :row="row" :index="idx"></slot>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div v-if="sortedData.length > 0" class="dt-pagination">
      <div class="dt-page-info">
        共 {{ sortedData.length }} 条，第 {{ currentPage }} / {{ totalPages }} 页
      </div>
      <div class="dt-page-controls">
        <select v-model.number="currentPageSize" @change="currentPage = 1" class="dt-page-select">
          <option v-for="s in pageSizes" :key="s" :value="s">{{ s }} 条/页</option>
        </select>
        <button class="btn btn-ghost btn-sm" :disabled="currentPage <= 1" @click="currentPage--">上一页</button>
        <span class="dt-page-num">{{ currentPage }}</span>
        <button class="btn btn-ghost btn-sm" :disabled="currentPage >= totalPages" @click="currentPage++">下一页</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, reactive } from 'vue'
import EmptyState from './EmptyState.vue'
import Skeleton from './Skeleton.vue'

const props = defineProps({
  columns: { type: Array, required: true },
  data: { type: Array, default: () => [] },
  pageSize: { type: Number, default: 20 },
  pageSizes: { type: Array, default: () => [20, 50, 100] },
  loading: { type: Boolean, default: false },
  emptyText: { type: String, default: '暂无数据' },
  emptyIcon: { type: String, default: '' },
  emptyDesc: { type: String, default: '' },
  emptySvg: { type: String, default: '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg>' },
  expandable: { type: Boolean, default: false },
  rowKey: { type: [String, Function], default: null },
  rowClass: { type: Function, default: null },
  showToolbar: { type: Boolean, default: true },
})

// Column visibility
const visibleCols = reactive(new Set(props.columns.map(c => c.key)))
const colMenuOpen = ref(false)

function toggleCol(key) {
  if (visibleCols.has(key)) {
    if (visibleCols.size > 1) visibleCols.delete(key)
  } else {
    visibleCols.add(key)
  }
}

const visibleColumns = computed(() => props.columns.filter(c => visibleCols.has(c.key)))

// Sorting
const sortKey = ref(null)
const sortDir = ref('asc')

function toggleSort(key) {
  if (sortKey.value === key) {
    if (sortDir.value === 'asc') sortDir.value = 'desc'
    else if (sortDir.value === 'desc') { sortKey.value = null; sortDir.value = 'asc' }
  } else {
    sortKey.value = key
    sortDir.value = 'asc'
  }
}

const sortedData = computed(() => {
  const d = [...props.data]
  if (!sortKey.value) return d
  const key = sortKey.value
  const dir = sortDir.value === 'asc' ? 1 : -1
  return d.sort((a, b) => {
    let va = a[key], vb = b[key]
    if (va == null) va = ''
    if (vb == null) vb = ''
    if (typeof va === 'number' && typeof vb === 'number') return (va - vb) * dir
    return String(va).localeCompare(String(vb)) * dir
  })
})

// Pagination
const currentPage = ref(1)
const currentPageSize = ref(props.pageSize)
const totalPages = computed(() => Math.max(1, Math.ceil(sortedData.value.length / currentPageSize.value)))
const pagedData = computed(() => {
  const start = (currentPage.value - 1) * currentPageSize.value
  return sortedData.value.slice(start, start + currentPageSize.value)
})

// Reset page on data change
watch(() => props.data, () => { currentPage.value = 1 })

// Expand
const expandedRows = reactive(new Set())
function toggleExpand(idx) {
  if (expandedRows.has(idx)) expandedRows.delete(idx)
  else expandedRows.add(idx)
}

// Row key
function rowKeyFn(row, idx) {
  if (!props.rowKey) return idx
  if (typeof props.rowKey === 'function') return props.rowKey(row)
  return row[props.rowKey] ?? idx
}

// Row class
function rowClassFn(row) {
  if (props.rowClass) return props.rowClass(row)
  return ''
}

// Cell value
function getCellValue(row, col) {
  return row[col.key]
}

function formatCell(row, col) {
  const val = row[col.key]
  if (col.format) return col.format(val, row)
  if (val == null) return '--'
  return escHtml(String(val))
}

function escHtml(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}
</script>

<style scoped>
.data-table-wrapper { position: relative; }
.dt-toolbar { display: flex; gap: var(--space-2); margin-bottom: var(--space-2); align-items: center; justify-content: flex-end; position: relative; }
.dt-col-toggle { position: relative; }
.dt-col-menu {
  position: absolute; right: 0; top: 100%; z-index: 50;
  background: var(--bg-overlay); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); padding: var(--space-2); min-width: 160px;
  box-shadow: var(--shadow-lg);
}
.dt-col-item { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-1) 0; font-size: var(--text-sm); color: var(--text-primary); cursor: pointer; }
.dt-col-item input { accent-color: var(--color-primary); }

th.sortable { cursor: pointer; user-select: none; }
th.sortable:hover { color: var(--text-primary); }
th.sorted { color: var(--text-primary); background: linear-gradient(180deg, var(--color-primary-dim), rgba(99,102,241,0.04)); box-shadow: inset 0 -1px 0 var(--color-primary); }
.sort-icon { font-size: 0.55rem; margin-left: 2px; color: var(--text-tertiary); }
th.sorted .sort-icon { color: var(--color-primary); }
.sort-icon-neutral { font-size: 0.5rem; letter-spacing: -2px; opacity: .4; }

.expand-arrow {
  transition: transform var(--transition-fast); display: inline-block;
  color: var(--text-tertiary);
}
.expand-arrow.expanded { transform: rotate(90deg); }

.expand-row td {
  background: var(--bg-elevated); padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}

.dt-pagination {
  display: flex; justify-content: space-between; align-items: center;
  margin-top: var(--space-3); padding-top: var(--spacing-sm);
  border-top: 1px solid var(--border-subtle);
  font-size: var(--text-sm); color: var(--text-secondary);
}
.dt-page-controls { display: flex; gap: var(--space-2); align-items: center; }
.dt-page-num {
  font-weight: 600; color: var(--color-primary);
  background: var(--color-primary-dim);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  min-width: 24px; text-align: center;
}
.dt-page-select {
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-1) var(--space-2);
  font-size: var(--text-xs); outline: none; font-family: var(--font-sans);
}
.dt-page-select option { background: var(--bg-elevated); }
.btn:disabled { opacity: .3; cursor: not-allowed; transform: none; }
</style>
