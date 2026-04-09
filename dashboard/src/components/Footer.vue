<template>
  <footer class="app-footer">
    <span class="footer-left">© 2026 Lobster Guard {{ appState?.version || 'v36.5' }} | 龙虾卫士 — AI Agent 安全网关</span>
    <span class="footer-right">Dashboard by zhuowater</span>
  </footer>
</template>

<script setup>
import { inject } from 'vue'
const appState = inject('appState')
</script>

<style scoped>
.app-footer {
  height: 40px;
  min-height: 40px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--space-5);
  background: rgba(15, 23, 42, 0.65);
  backdrop-filter: blur(8px);
  border-top: 1px solid var(--border-subtle);
  font-size: 12px;
  color: var(--text-disabled);
}
.footer-left, .footer-right {
  white-space: nowrap;
}
.footer-right {
  color: var(--text-tertiary);
}
</style>
