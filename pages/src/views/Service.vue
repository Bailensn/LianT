<script setup lang="ts">
import DownloadSystem from "../components/DownloadSystem.vue"
import { serviceBuilds } from "../data/downloads"
</script>

<template>
  <DownloadSystem
    title="下载 Service"
    description="服务端组件"
    :builds="serviceBuilds"
    tag="Service"
    num="02"
  />
</template>