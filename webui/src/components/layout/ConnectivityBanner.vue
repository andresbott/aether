<script setup lang="ts">
import { useConnectivity, dismissBanner } from '@/composables/useConnectivity'
import { NETWORK_MESSAGE } from '@/lib/apiError'
import Message from 'primevue/message'

const { isOffline } = useConnectivity()

const handleDismiss = () => {
    dismissBanner()
}
</script>

<template>
    <Transition name="slide-down">
        <Message
            v-if="isOffline"
            severity="warn"
            class="connectivity-banner"
            :closable="true"
            @close="handleDismiss"
        >
            {{ NETWORK_MESSAGE }}
        </Message>
    </Transition>
</template>

<style scoped lang="scss">
.connectivity-banner {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    z-index: 9999;
    margin: 0;
    border-radius: 0;
    border-left: none;
    border-right: none;
    border-top: none;
}

.slide-down-enter-active,
.slide-down-leave-active {
    transition: transform 0.3s ease;
}

.slide-down-enter-from,
.slide-down-leave-to {
    transform: translateY(-100%);
}
</style>
