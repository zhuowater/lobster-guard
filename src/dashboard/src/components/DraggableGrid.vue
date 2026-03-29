<template>
  <div class="draggable-grid" :class="{ 'edit-mode': editMode }">
    <TransitionGroup name="grid-anim" tag="div" class="grid-container">
      <div
        v-for="(panel, index) in sortedPanels"
        :key="panel.id"
        class="grid-panel"
        :class="{
          collapsed: panel.collapsed,
          dragging: dragIndex === index,
          'drag-over': dropIndex === index,
          'width-full': panel.width === 'full',
          'width-half': panel.width === 'half' || !panel.width,
          'width-third': panel.width === 'third',
          'panel-locked': panel.locked,
          'panel-hidden': !panel.visible,
        }"
        :draggable="editMode && !panel.locked"
        @dragstart="onDragStart($event, index)"
        @dragover.prevent="onDragOver($event, index)"
        @dragend="onDragEnd"
        @drop="onDrop($event, index)"
      >
        <div class="panel-header" @click="toggleCollapse(panel)">
          <div class="panel-header-left">
            <button
              v-if="editMode && !panel.locked"
              class="drag-handle"
              title="拖拽排序"
              @mousedown.stop
            >⠿</button>
            <span class="panel-title">{{ panel.title }}</span>
          </div>
          <div class="panel-actions">
            <button
              v-if="editMode"
              class="panel-width-btn"
              @click.stop="cycleWidth(panel)"
              :title="'宽度: ' + panel.width"
            >
              {{ panel.width === 'full' ? '▰▰' : panel.width === 'third' ? '▰' : '▰▰' }}
            </button>
            <button
              class="panel-collapse-btn"
              @click.stop="toggleCollapse(panel)"
              :title="panel.collapsed ? '展开' : '折叠'"
            >
              {{ panel.collapsed ? '▸' : '▾' }}
            </button>
            <button
              v-if="editMode && panel.removable"
              class="panel-remove-btn"
              @click.stop="removePanel(panel)"
              title="隐藏"
            >✕</button>
          </div>
        </div>
        <Transition name="collapse">
          <div v-show="!panel.collapsed" class="panel-body">
            <slot :name="panel.id" :panel="panel"></slot>
          </div>
        </Transition>
      </div>
    </TransitionGroup>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  panels: { type: Array, required: true },
  editMode: { type: Boolean, default: false },
})

const emit = defineEmits(['update:panels', 'remove', 'reorder'])

const dragIndex = ref(-1)
const dropIndex = ref(-1)

const sortedPanels = computed(() => {
  return [...props.panels]
    .filter(p => p.visible !== false)
    .sort((a, b) => (a.order || 0) - (b.order || 0))
})

function onDragStart(e, index) {
  if (!props.editMode) return
  dragIndex.value = index
  e.dataTransfer.effectAllowed = 'move'
  e.dataTransfer.setData('text/plain', index.toString())
}

function onDragOver(e, index) {
  if (!props.editMode || dragIndex.value === -1) return
  e.preventDefault()
  dropIndex.value = index
}

function onDrop(e, index) {
  if (!props.editMode || dragIndex.value === -1) return
  e.preventDefault()
  const from = dragIndex.value
  const to = index
  if (from !== to) {
    const visible = sortedPanels.value
    const panels = [...props.panels]
    const fromPanel = visible[from]
    const toPanel = visible[to]

    // Swap orders
    const fromIdx = panels.findIndex(p => p.id === fromPanel.id)
    const toIdx = panels.findIndex(p => p.id === toPanel.id)
    if (fromIdx >= 0 && toIdx >= 0) {
      const tmpOrder = panels[fromIdx].order
      panels[fromIdx].order = panels[toIdx].order
      panels[toIdx].order = tmpOrder
      emit('update:panels', panels)
      emit('reorder', { from: fromPanel.id, to: toPanel.id })
    }
  }
  dragIndex.value = -1
  dropIndex.value = -1
}

function onDragEnd() {
  dragIndex.value = -1
  dropIndex.value = -1
}

function toggleCollapse(panel) {
  const panels = [...props.panels]
  const idx = panels.findIndex(p => p.id === panel.id)
  if (idx >= 0) {
    panels[idx] = { ...panels[idx], collapsed: !panels[idx].collapsed }
    emit('update:panels', panels)
  }
}

function removePanel(panel) {
  const panels = [...props.panels]
  const idx = panels.findIndex(p => p.id === panel.id)
  if (idx >= 0) {
    panels[idx] = { ...panels[idx], visible: false }
    emit('update:panels', panels)
    emit('remove', panel.id)
  }
}

function cycleWidth(panel) {
  const widths = ['half', 'full', 'third']
  const curr = panel.width || 'half'
  const next = widths[(widths.indexOf(curr) + 1) % widths.length]
  const panels = [...props.panels]
  const idx = panels.findIndex(p => p.id === panel.id)
  if (idx >= 0) {
    panels[idx] = { ...panels[idx], width: next }
    emit('update:panels', panels)
  }
}
</script>

<style scoped>
.draggable-grid {
  width: 100%;
}

.grid-container {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4, 16px);
}

.grid-panel {
  background: var(--bg-surface, #1a1a2e);
  border: 1px solid var(--border-subtle, #2a2a40);
  border-radius: var(--radius-lg, 12px);
  overflow: hidden;
  transition: all 0.3s ease;
  min-height: 60px;
}

.grid-panel.width-full {
  width: 100%;
  flex: 0 0 100%;
}

.grid-panel.width-half {
  width: calc(50% - 8px);
  flex: 0 0 calc(50% - 8px);
}

.grid-panel.width-third {
  width: calc(33.333% - 11px);
  flex: 0 0 calc(33.333% - 11px);
}

/* Edit mode styles */
.edit-mode .grid-panel {
  border: 2px dashed rgba(99, 102, 241, 0.3);
  cursor: default;
}

.edit-mode .grid-panel:hover {
  border-color: rgba(99, 102, 241, 0.6);
}

.grid-panel.dragging {
  opacity: 0.5;
  border: 2px dashed #6366f1 !important;
  transform: scale(0.98);
}

.grid-panel.drag-over {
  border-top: 3px solid #6366f1 !important;
  box-shadow: 0 -2px 8px rgba(99, 102, 241, 0.3);
}

.grid-panel.panel-locked {
  border-style: solid;
}

/* Panel header */
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-3, 12px) var(--space-4, 16px);
  cursor: pointer;
  user-select: none;
  border-bottom: 1px solid var(--border-subtle, #2a2a40);
  background: var(--bg-elevated, rgba(255, 255, 255, 0.02));
  transition: background 0.2s;
}

.panel-header:hover {
  background: var(--bg-elevated, rgba(255, 255, 255, 0.04));
}

.panel-header-left {
  display: flex;
  align-items: center;
  gap: var(--space-2, 8px);
}

.panel-title {
  font-size: var(--text-sm, 14px);
  font-weight: 600;
  color: var(--text-primary, #e0e0e0);
}

.panel-actions {
  display: flex;
  align-items: center;
  gap: var(--space-1, 4px);
}

.drag-handle {
  background: none;
  border: none;
  color: var(--text-tertiary, #666);
  cursor: grab;
  font-size: 16px;
  padding: 0 4px;
  line-height: 1;
  opacity: 0.6;
  transition: opacity 0.2s;
}

.drag-handle:hover {
  opacity: 1;
  color: var(--color-primary, #6366f1);
}

.drag-handle:active {
  cursor: grabbing;
}

.panel-collapse-btn,
.panel-remove-btn,
.panel-width-btn {
  background: none;
  border: none;
  color: var(--text-tertiary, #666);
  cursor: pointer;
  font-size: 12px;
  padding: 2px 6px;
  border-radius: var(--radius-sm, 4px);
  transition: all 0.2s;
}

.panel-collapse-btn:hover,
.panel-width-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary, #e0e0e0);
}

.panel-remove-btn:hover {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

/* Panel body */
.panel-body {
  padding: var(--space-4, 16px);
  min-height: 100px;
}

/* Collapse transition */
.collapse-enter-active,
.collapse-leave-active {
  transition: max-height 0.3s ease, opacity 0.3s ease, padding 0.3s ease;
  overflow: hidden;
}

.collapse-enter-from,
.collapse-leave-to {
  max-height: 0;
  opacity: 0;
  padding-top: 0;
  padding-bottom: 0;
}

.collapse-enter-to,
.collapse-leave-from {
  max-height: 600px;
  opacity: 1;
}

/* Grid animation */
.grid-anim-enter-active {
  transition: all 0.3s ease;
}

.grid-anim-leave-active {
  transition: all 0.3s ease;
}

.grid-anim-enter-from,
.grid-anim-leave-to {
  opacity: 0;
  transform: scale(0.95);
}

.grid-anim-move {
  transition: transform 0.3s ease;
}

/* Collapsed state */
.grid-panel.collapsed .panel-header {
  border-bottom: none;
}

/* Responsive */
@media (max-width: 1200px) {
  .grid-panel.width-third {
    width: calc(50% - 8px);
    flex: 0 0 calc(50% - 8px);
  }
}

@media (max-width: 768px) {
  .grid-panel.width-half,
  .grid-panel.width-third {
    width: 100%;
    flex: 0 0 100%;
  }
}
</style>
