<template>
  <div class="app-layout">
    <div class="sidebar-overlay" :class="{ show: mobileOpen }" @click="mobileOpen = false"></div>
    <Sidebar :mobile-open="mobileOpen" @close-mobile="mobileOpen = false" />
    <div class="main-area">
      <Topbar @toggle-mobile="mobileOpen = !mobileOpen" />
      <div class="content-area">
        <router-view v-slot="{ Component }">
          <transition name="page" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </div>
    </div>
    <Toast />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { api } from './api.js'
import { updateHealth, setDisconnected } from './stores/app.js'
import Sidebar from './components/Sidebar.vue'
import Topbar from './components/Topbar.vue'
import Toast from './components/Toast.vue'

const mobileOpen = ref(false)

let healthTimer = null

async function loadHealth() {
  try {
    const d = await api('/healthz')
    updateHealth(d)
  } catch {
    setDisconnected()
  }
}

onMounted(() => {
  loadHealth()
  healthTimer = setInterval(loadHealth, 10000)
})

onUnmounted(() => {
  clearInterval(healthTimer)
})
</script>

<style>
.sidebar-overlay {
  display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,.5); z-index: 199;
}
.sidebar-overlay.show { display: block; }

.page-enter-active { animation: pi .3s ease-out both; }
.page-leave-active { animation: pi .2s ease-in reverse; }
</style>
