<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import {
    ALL_IDENTIFY_FIELD_IDS,
    IDENTIFY_FIELDS,
    type IdentifyFieldId
} from '@/lib/identifyFields'

// Which of a match's fields get staged, for both identify dialogs. Extracted
// rather than copied per dialog (unlike their track tables, which share a look
// but not a data shape): this row is the same registry, the same selection
// semantics and the same emitted list in both places, so one copy is the whole
// contract.
const props = defineProps<{
    modelValue: IdentifyFieldId[]
    // Prefixes every data-test id, so a dialog's spec queries its own row.
    testPrefix: string
}>()
const emit = defineEmits<{
    (e: 'update:modelValue', v: IdentifyFieldId[]): void
}>()

function toggleField(id: IdentifyFieldId, on: boolean) {
    const set = new Set(props.modelValue)
    if (on) set.add(id)
    else set.delete(id)
    // Keep the registry's order rather than click order, so the emitted list is
    // stable and easy to assert on.
    emit(
        'update:modelValue',
        ALL_IDENTIFY_FIELD_IDS.filter((f) => set.has(f))
    )
}
</script>

<template>
    <div class="identify-fields" :data-test="`${testPrefix}-fields`">
        <!-- Label, checkboxes and the two bulk actions on one line: All/None act
             on the checkboxes beside them, so separating them onto their own row
             would only make the connection harder to see. -->
        <div class="fields-list">
            <span class="fields-label">Stage fields</span>
            <div v-for="field in IDENTIFY_FIELDS" :key="field.id" class="field-toggle">
                <Checkbox
                    :modelValue="modelValue.includes(field.id)"
                    @update:modelValue="(v: boolean) => toggleField(field.id, v)"
                    :binary="true"
                    :inputId="`${testPrefix}-field-${field.id}`"
                    :data-test="`${testPrefix}-field-${field.id}`"
                />
                <label :for="`${testPrefix}-field-${field.id}`">{{ field.label }}</label>
            </div>
            <!-- `outlined size="small"`, matching the editor's other secondary
                 actions (EditPanel's Identify / Raw buttons). -->
            <Button
                label="All"
                size="small"
                outlined
                class="fields-action"
                :data-test="`${testPrefix}-fields-all`"
                @click="emit('update:modelValue', [...ALL_IDENTIFY_FIELD_IDS])"
            />
            <Button
                label="None"
                size="small"
                outlined
                class="fields-action"
                :data-test="`${testPrefix}-fields-none`"
                @click="emit('update:modelValue', [])"
            />
        </div>
        <small class="fields-note">
            Only the checked fields are staged; everything else keeps its current value.
        </small>
    </div>
</template>

<style scoped>
.identify-fields {
    display: flex;
    flex-direction: column;
    /* Fixed block above the scrolling list, never squeezed by it. */
    flex: 0 0 auto;
    gap: 0.4rem;
    padding-bottom: 0.8rem;
    margin-bottom: 0.8rem;
    border-bottom: 1px solid var(--app-border);
}
.fields-label {
    font-weight: 600;
    font-size: 0.85rem;
}
.fields-list {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem 1.1rem;
}
/* Pushed to the end of the row, so All/None sit together away from the
   checkboxes they act on rather than reading as another field. On a narrow
   dialog the row wraps and they simply follow the last checkbox. */
.fields-action {
    margin-left: auto;
}
.fields-action + .fields-action {
    /* Only the first of the pair claims the free space; the second hugs it. */
    margin-left: 0;
}
.field-toggle {
    display: flex;
    align-items: center;
    gap: 0.4rem;
}
.field-toggle label {
    font-size: 0.85rem;
    cursor: pointer;
}
.fields-note {
    color: var(--app-text-secondary);
}
</style>
