<script setup lang="ts">
import Button from 'primevue/button'
import { watch, onBeforeUnmount } from 'vue'
import { useConfirm } from 'primevue/useconfirm'

const props = withDefaults(
    defineProps<{
        editing: boolean
        saveDisabled?: boolean
        saving?: boolean
        canDelete?: boolean
        dirty?: boolean
        deleteHeader?: string
        deleteMessage?: string
        saveIcon?: string
        saveTooltip?: string
    }>(),
    {
        saveDisabled: false,
        saving: false,
        canDelete: true,
        dirty: false,
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

// Esc exits edit mode (same as Cancel: discard staged edits + leave). The listener
// is only active while editing. If a confirm dialog is open, let it own Escape
// (dismiss the dialog) rather than also exiting edit mode. Esc is easy to fumble, so
// when there are unsaved changes it verifies before discarding — mirroring the views'
// navigation guard.
function onKeydown(e: KeyboardEvent): void {
    if (e.key !== 'Escape') return
    if (document.querySelector('.p-confirmdialog')) return
    if (props.dirty && !window.confirm('You have unsaved changes. Discard them?')) return
    emit('cancel')
}

watch(
    () => props.editing,
    (editing) => {
        if (editing) document.addEventListener('keydown', onKeydown)
        else document.removeEventListener('keydown', onKeydown)
    },
    { immediate: true }
)

onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
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
        <!-- Order: Delete, Save, Cancel. Delete sits far left so it is NOT under the
             cursor when entering edit mode (the pencil is the rightmost read-mode
             button); Cancel, the safe action, takes that spot instead. -->
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
    </template>
</template>
