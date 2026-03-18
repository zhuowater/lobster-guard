<template>
  <!-- v14.1: 登录页不显示侧边栏和顶栏 -->
  <div v-if="isLoginPage" class="login-layout">
    <router-view />
  </div>
  <div v-else class="app-layout">
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
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { api } from './api.js'
import { updateHealth, setDisconnected } from './stores/app.js'
import Sidebar from './components/Sidebar.vue'
import Topbar from './components/Topbar.vue'
import Toast from './components/Toast.vue'

const route = useRoute()
const mobileOpen = ref(false)

// v14.1: 判断是否在登录页
const isLoginPage = computed(() => route.name === 'login')

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
.login-layout { min-height: 100vh; }

.sidebar-overlay {
  display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,.5); z-index: 199;
}
.sidebar-overlay.show { display: block; }

.page-enter-active { animation: page-in .15s ease-out both; }
.page-leave-active { animation: page-in .1s ease-in reverse; }
@keyframes page-in { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }
</style>
