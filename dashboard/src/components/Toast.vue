<template>
  <Teleport to="body">
    <div v-if="appState.toast" class="toast show" :class="appState.toast.type">
      {{ appState.toast.message }}
    </div>
  </Teleport>
</template>

<script setup>
import { inject } from 'vue'
const appState = inject('appState')
</script>

<style scoped>
.toast {
  position: fixed; bottom: 24px; right: 24px;
  background: var(--bg-card-2); border: 1px solid var(--neon-blue);
  border-radius: 8px; padding: 12px 20px; color: var(--text); font-size: .85rem;
  z-index: 999; max-width: 360px;
  animation: toastIn .3s ease-out;
}
@keyframes toastIn { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
.toast.error { border-color: var(--neon-red); }
.toast.success { border-color: var(--neon-green); }
</style>
