<script setup lang="ts">
import DownloadSystem from "../components/DownloadSystem.vue"
import { managerBuilds } from "../data/downloads"
</script>

<template>
  <DownloadSystem
    title="下载 Manager"
    description="Telegram Bot 管理器客户端"
    :builds="managerBuilds"
    tag="Manager"
    num="01"
  />
</template>