<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { apiClient } from '@/lib/api/client'

const { data, isLoading, isError } = useQuery({
  queryKey: ['health'],
  queryFn: async () => {
    const response = await apiClient.get('/health')
    return response.data
  }
})
</script>

<template>
  <div class="p-4">
    <h1>Aether</h1>
    <p>Music Server</p>
    <div class="mt-4">
      <h3>API Health</h3>
      <p v-if="isLoading">Checking...</p>
      <p v-else-if="isError" class="text-red-500">API unreachable</p>
      <p v-else class="text-green-500">{{ data?.status }}</p>
    </div>
  </div>
</template>
