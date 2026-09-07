<script setup lang="ts">
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Button from 'primevue/button'

const props = defineProps<{
    label: string
    id: string
    modelValue: string | number | null
    type: 'text' | 'number'
    placeholder?: string
    disabled?: boolean
    dirty: boolean
    undoTooltip: string
    undoTestId: string
    undoAriaLabel: string
    inputClass?: string
}>()

const emit = defineEmits<{
    (e: 'update:modelValue', value: string | number | null): void
    (e: 'undo'): void
}>()

function handleTextUpdate(value: string | undefined) {
    emit('update:modelValue', value ?? '')
}

function handleNumberUpdate(value: number | null | undefined) {
    emit('update:modelValue', value ?? null)
}
</script>

<template>
    <div class="field-row" :class="{ 'field-dirty': dirty, disabled }">
        <label :for="id">{{ label }}</label>
        <InputText
            v-if="type === 'text'"
            :id="id"
            :class="inputClass"
            :modelValue="modelValue as string"
            :placeholder="placeholder"
            :disabled="disabled"
            @update:modelValue="handleTextUpdate"
        />
        <InputNumber
            v-else
            :inputId="id"
            :class="inputClass"
            :modelValue="modelValue as number | null"
            :useGrouping="false"
            :placeholder="placeholder"
            :disabled="disabled"
            @update:modelValue="handleNumberUpdate"
        />
        <Button
            v-if="dirty"
            icon="pi pi-undo"
            text
            size="small"
            :aria-label="undoAriaLabel"
            :data-test="undoTestId"
            v-tooltip.left="undoTooltip"
            @click="emit('undo')"
        />
    </div>
</template>

<style scoped>
.field-row {
    display: grid;
    grid-template-columns: 8rem 1fr auto;
    align-items: center;
    gap: 0.5rem;
}
.field-row label {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    padding-top: 0.35rem;
}
.field-row :deep(.p-inputtext),
.field-row :deep(.p-inputnumber) {
    width: 100%;
}
.field-row :deep(.p-inputnumber-input) {
    width: 100%;
}
.field-row.field-dirty > label {
    color: var(--app-staged);
    font-weight: 600;
}
.field-row.field-dirty :deep(.p-inputtext),
.field-row.field-dirty :deep(.p-inputnumber-input) {
    border-color: var(--app-staged);
    background-color: var(--app-staged-soft);
}
.field-row.disabled label {
    color: var(--app-text-secondary);
    opacity: 0.6;
}
</style>
