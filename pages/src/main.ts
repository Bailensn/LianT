import { createApp } from "vue"
import App from "./App.vue"
import router from "./router"
import { setupFavicon } from "./favicon"

import "./styles/main.css"

setupFavicon()

createApp(App)
  .use(router)
  .mount("#app")