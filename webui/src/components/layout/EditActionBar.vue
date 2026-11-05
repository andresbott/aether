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
        saveIcon: 'ms-check',
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
        icon: 'ms-warning',
        acceptClass: 'p-button-danger',
        acceptLabel: 'Delete',
        rejectLabel: 'Cancel',
        accept: () => emit('delete')
    })
}

// Esc exits edit mode (same as Cancel: discard staged edits + leave). The listener
// is only active while editing. If any modal dialog is open (ConfirmDialog, Dialog,
// etc.), let it own Escape (dismiss the dialog) rather than also exiting edit mode.
// Esc is easy to fumble, so when there are unsaved changes it verifies before
// discarding — mirroring the views' navigation guard.
function onKeydown(e: KeyboardEvent): void {
    if (e.key !== 'Escape') return
    if (document.querySelector('.p-dialog')) return
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
    <!-- Read mode: [read-actions] Delete, Edit. Edit mode: Delete, Save, Cancel.
         Delete is ONE element in both modes, always left of the rest, so entering
         edit mode never drops the destructive action under the cursor: the pencil
         (rightmost in read mode) gives its spot to Cancel, the safe action. Delete
         sits in read mode too, so removing an item does not take a trip through
         edit mode; it still asks first (confirmDelete). -->
    <slot v-if="!editing" name="read-actions" />
    <Button
        v-if="canDelete"
        class="edit-action-delete"
        icon="ms-delete"
        text
        rounded
        severity="danger"
        v-tooltip.bottom="'Delete'"
        aria-label="Delete"
        @click="confirmDelete"
    />
    <Button
        v-if="!editing"
        class="edit-action-edit"
        icon="ms-edit"
        text
        rounded
        v-tooltip.bottom="'Edit'"
        aria-label="Edit"
        @click="emit('update:editing', true)"
    />
    <template v-else>
        <Button
            class="edit-action-save"
            :icon="saveIcon"
            text
            rounded
            :disabled="saveDisabled"
            :loading="saving"
            v-tooltip.bottom="saveTooltip"
            :aria-label="saveTooltip"
            @click="emit('save')"
        />
        <Button
            class="edit-action-cancel"
            icon="ms-close"
            text
            rounded
            v-tooltip.bottom="'Cancel'"
            aria-label="Cancel"
            @click="emit('cancel')"
        />
    </template>
</template>
