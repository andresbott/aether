<script setup lang="ts">
import Button from 'primevue/button'
import { useConfirm } from 'primevue/useconfirm'

const props = withDefaults(
    defineProps<{
        editing: boolean
        saveDisabled?: boolean
        saving?: boolean
        canDelete?: boolean
        deleteHeader?: string
        deleteMessage?: string
        saveIcon?: string
        saveTooltip?: string
    }>(),
    {
        saveDisabled: false,
        saving: false,
        canDelete: true,
        deleteHeader: 'Delete?',
        deleteMessage: 'This cannot be undone.',
        saveIcon: 'pi pi-check',
        saveTooltip: 'Save'
    }
)

const emit = defineEmits<{
    (e: 'update:editing', value: boolean): void
    (e: 'save'): void
    (e: 'cancel'): void
    (e: 'delete'): void
}>()

const confirm = useConfirm()

function confirmDelete(): void {
    confirm.require({
        header: props.deleteHeader,
        message: props.deleteMessage,
        icon: 'pi pi-exclamation-triangle',
        acceptClass: 'p-button-danger',
        acceptLabel: 'Delete',
        rejectLabel: 'Cancel',
        accept: () => emit('delete')
    })
}
</script>

<template>
    <template v-if="!editing">
        <slot name="read-actions" />
        <Button
            class="edit-action-edit"
            icon="pi pi-pencil"
            text
            rounded
            v-tooltip.bottom="'Edit'"
            @click="emit('update:editing', true)"
        />
    </template>
    <template v-else>
        <Button
            class="edit-action-save"
            :icon="saveIcon"
            text
            rounded
            :disabled="saveDisabled"
            :loading="saving"
            v-tooltip.bottom="saveTooltip"
            @click="emit('save')"
        />
        <Button
            class="edit-action-cancel"
            icon="pi pi-times"
            text
            rounded
            v-tooltip.bottom="'Cancel'"
            @click="emit('cancel')"
        />
        <Button
            v-if="canDelete"
            class="edit-action-delete"
            icon="pi pi-trash"
            text
            rounded
            severity="danger"
            v-tooltip.bottom="'Delete'"
            @click="confirmDelete"
        />
    </template>
</template>
