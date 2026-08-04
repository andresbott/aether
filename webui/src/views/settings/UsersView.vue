<script setup lang="ts">
import UsersPanel from '@/components/admin/UsersPanel.vue'
import { useNativeAuth } from '@/composables/useUsers'

// The route stays reachable by URL even when the nav entry is hidden, so the
// view itself explains the situation instead of erroring on the users query.
const nativeAuth = useNativeAuth()
</script>

<template>
    <div class="users-view">
        <UsersPanel v-if="nativeAuth" />
        <div v-else class="auth-disabled">
            <p>User management requires native authentication.</p>
            <p>Set <code>Auth.Method: "native"</code> in the server configuration to enable it.</p>
        </div>
    </div>
</template>

<style scoped>
.users-view {
    padding: 2rem;
    overflow-y: auto;
}
.auth-disabled {
    text-align: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}
</style>
