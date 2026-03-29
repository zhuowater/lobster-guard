import { createApp } from 'vue'
import App from './App.vue'
import router from './router.js'
import { appStorePlugin } from './stores/app.js'
import './styles/main.css'

const app = createApp(App)
app.use(router)
app.use(appStorePlugin)
app.mount('#app')
