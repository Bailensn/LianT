<script setup lang="ts">
import { LOGO_PATHS } from "../logo"

const props = defineProps<{
  size?: number
}>()
</script>

<template>
  <svg
    :width="props.size ?? 30"
    :height="props.size ?? 30"
    viewBox="0 0 2000 2000"
    xmlns="http://www.w3.org/2000/svg"
    aria-hidden="true"
  >
    <path
      v-for="(p, i) in LOGO_PATHS"
      :key="i"
      :d="p.d"
      :fill="p.fill"
    />
  </svg>
</template>