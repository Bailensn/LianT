import { createRouter, createWebHistory } from "vue-router"
import Main from "../views/Main.vue"
import Manager from "../views/Manager.vue"
import Service from "../views/Service.vue"

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: Main },
    { path: "/manager", component: Manager },
    { path: "/service", component: Service },
  ],
})

export default router