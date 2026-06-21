<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'

defineProps<{ saving?: boolean }>()
const visible = defineModel<boolean>('visible', { required: true })
const name = defineModel<string>('name', { required: true })
const emit = defineEmits<{ save: [] }>()
</script>

<template>
    <Dialog
        v-model:visible="visible"
        header="Save Queue as Playlist"
        :modal="true"
        :style="{ width: '400px' }"
    >
        <div class="save-form">
            <InputText
                v-model="name"
                placeholder="Playlist name"
                class="w-full"
                autofocus
                @keyup.enter="emit('save')"
            />
        </div>
        <template #footer>
            <Button label="Cancel" text @click="visible = false" />
            <Button
                label="Save"
                :loading="saving"
                :disabled="!name.trim()"
                @click="emit('save')"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.save-form {
    padding: 1rem 0;
}
</style>
