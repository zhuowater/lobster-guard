<template>
  <div class="narrative-placeholder">
    <div class="narrative-content">
      <div class="narrative-icon">🔮</div>
      <h2 class="narrative-title">叙事模式开发中...</h2>
      <p class="narrative-desc">基于安全叙事的全新交互体验，敬请期待</p>
      <button class="narrative-back-btn" @click="switchBack">
        📋 切换回经典模式
      </button>
    </div>
  </div>
</template>

<script setup>
import { navStore } from '../stores/navigation.js'

function switchBack() {
  navStore.setMode('classic')
}
</script>

<style scoped>
.narrative-placeholder {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: linear-gradient(135deg, #0f0b1e 0%, #1a1333 50%, #0d1117 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 500;
}

.narrative-content {
  text-align: center;
  animation: narrativeFadeIn 0.5s ease-out;
}

.narrative-icon {
  font-size: 4rem;
  margin-bottom: 1.5rem;
  animation: narrativeFloat 3s ease-in-out infinite;
}

.narrative-title {
  font-size: 1.75rem;
  font-weight: 700;
  color: #c4b5fd;
  margin: 0 0 0.75rem 0;
  letter-spacing: 0.02em;
}

.narrative-desc {
  font-size: 0.875rem;
  color: rgba(196, 181, 253, 0.5);
  margin: 0 0 2.5rem 0;
}

.narrative-back-btn {
  background: rgba(99, 102, 241, 0.15);
  border: 1px solid rgba(99, 102, 241, 0.3);
  color: #a5b4fc;
  padding: 10px 24px;
  border-radius: 8px;
  font-size: 0.875rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  font-family: inherit;
}

.narrative-back-btn:hover {
  background: rgba(99, 102, 241, 0.25);
  border-color: rgba(99, 102, 241, 0.5);
  color: #c7d2fe;
  box-shadow: 0 0 20px rgba(99, 102, 241, 0.15);
}

@keyframes narrativeFadeIn {
  from { opacity: 0; transform: translateY(20px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes narrativeFloat {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-10px); }
}
</style>
