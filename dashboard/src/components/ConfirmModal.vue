<template>
  <Teleport to="body">
    <div v-if="visible" class="modal-overlay" @click.self="$emit('cancel')">
      <div class="modal-box" :class="type">
        <div class="modal-header">
          <span class="modal-icon">{{ icon }}</span>
          <span class="modal-title">{{ title }}</span>
        </div>
        <div class="modal-body">{{ message }}</div>
        <div class="modal-footer">
          <button class="btn btn-sm" @click="$emit('cancel')">{{ cancelText }}</button>
          <button class="btn btn-sm" :class="confirmBtnClass" @click="$emit('confirm')">{{ confirmText }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  visible: { type: Boolean, default: false },
  title: { type: String, default: '确认操作' },
  message: { type: String, default: '确认执行此操作？' },
  type: { type: String, default: 'warning' }, // danger | warning | info
  confirmText: { type: String, default: '确认' },
  cancelText: { type: String, default: '取消' },
})

defineEmits(['confirm', 'cancel'])

const icon = computed(() => {
  if (props.type === 'danger') return '🚨'
  if (props.type === 'info') return 'ℹ️'
  return '⚠️'
})

const confirmBtnClass = computed(() => {
  if (props.type === 'danger') return 'btn-red'
  if (props.type === 'info') return ''
  return 'btn-red'
})
</script>

<style scoped>
.modal-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,.6); z-index: 1000;
  display: flex; align-items: center; justify-content: center;
  animation: fadeIn .2s;
}
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
.modal-box {
  background: var(--bg-card); border: 1px solid rgba(0,212,255,.2);
  border-radius: var(--radius); padding: 24px; min-width: 360px; max-width: 480px;
  box-shadow: 0 16px 64px rgba(0,0,0,.5);
  animation: slideUp .2s ease-out;
}
@keyframes slideUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
.modal-box.danger { border-color: rgba(255,68,102,.3); }
.modal-header { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.modal-icon { font-size: 1.4rem; }
.modal-title { font-size: 1.1rem; font-weight: 600; color: var(--text); }
.modal-body { color: var(--text-dim); font-size: .9rem; margin-bottom: 20px; line-height: 1.6; }
.modal-footer { display: flex; justify-content: flex-end; gap: 8px; }
</style>
