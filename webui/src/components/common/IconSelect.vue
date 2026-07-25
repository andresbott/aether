<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import Popover from 'primevue/popover'
import InputText from 'primevue/inputtext'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import { PRIME_ICONS } from '@/utils/primeIcons'

const props = withDefaults(
    defineProps<{
        modelValue?: string
    }>(),
    { modelValue: 'folder' }
)

const emit = defineEmits<{
    (e: 'update:modelValue', v: string): void
}>()

const popoverRef = ref<InstanceType<typeof Popover> | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const searchInputRef = ref<{ $el?: HTMLInputElement } | null>(null)
const searchQuery = ref('')
const isOpen = ref(false)
const popoverWidth = ref('360px')

const filteredIcons = computed(() => {
    if (!searchQuery.value) return PRIME_ICONS
    const query = searchQuery.value.toLowerCase()
    return PRIME_ICONS.filter((icon) => icon.includes(query))
})

function toggleDropdown(event: Event) {
    popoverRef.value?.toggle(event)
}

function onPopoverShow() {
    isOpen.value = true
    searchQuery.value = ''
    if (triggerRef.value) {
        popoverWidth.value = `${triggerRef.value.offsetWidth}px`
    }
    nextTick(() => {
        searchInputRef.value?.$el?.focus()
    })
}

function onPopoverHide() {
    isOpen.value = false
}

function selectIcon(icon: string) {
    emit('update:modelValue', icon)
    popoverRef.value?.hide()
}
</script>

<template>
    <div class="icon-select">
        <button
            ref="triggerRef"
            type="button"
            class="icon-select-trigger p-inputtext"
            :class="{ 'icon-select-trigger--open': isOpen }"
            @click="toggleDropdown"
        >
            <i :class="`pi pi-${props.modelValue}`" class="trigger-icon"></i>
            <span class="trigger-label">{{ props.modelValue }}</span>
            <i
                class="pi pi-chevron-down trigger-chevron"
                :class="{ 'trigger-chevron--open': isOpen }"
            ></i>
        </button>

        <Popover
            ref="popoverRef"
            class="icon-select-popover"
            @show="onPopoverShow"
            @hide="onPopoverHide"
        >
            <div class="icon-picker-content" :style="{ width: popoverWidth }">
                <div class="icon-search">
                    <IconField>
                        <InputIcon class="pi pi-search" />
                        <InputText
                            ref="searchInputRef"
                            v-model="searchQuery"
                            placeholder="Search icons..."
                            fluid
                        />
                    </IconField>
                </div>

                <div class="icons-grid">
                    <button
                        v-for="icon in filteredIcons"
                        :key="icon"
                        type="button"
                        class="icon-item"
                        :class="{ 'icon-item--selected': props.modelValue === icon }"
                        :title="icon"
                        @click="selectIcon(icon)"
                    >
                        <i :class="`pi pi-${icon}`"></i>
                    </button>
                </div>

                <div v-if="filteredIcons.length === 0" class="icons-empty">
                    <i class="pi pi-search"></i>
                    <p>No icons found for "{{ searchQuery }}"</p>
                </div>
            </div>
        </Popover>
    </div>
</template>

<style scoped>
.icon-select-trigger {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
    cursor: pointer;
    text-align: left;
    line-height: 1.5rem;
}
.trigger-icon {
    font-size: 1.1rem;
    color: var(--app-accent);
    flex-shrink: 0;
}
.trigger-label {
    flex: 1;
}
.trigger-chevron {
    font-size: 0.75rem;
    opacity: 0.6;
    flex-shrink: 0;
    transition: transform 0.2s;
}
.trigger-chevron--open {
    transform: rotate(180deg);
}
</style>

<style>
.icon-select-popover.p-popover .p-popover-content {
    padding: 0;
}
.icon-select-popover .icon-picker-content {
    min-width: 280px;
    max-width: calc(100vw - 2rem);
}
.icon-select-popover .icon-search {
    padding: 0.75rem 0.75rem 0.25rem;
}
.icon-select-popover .icons-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(44px, 1fr));
    gap: 0.5rem;
    max-height: 300px;
    overflow-y: auto;
    padding: 0.75rem;
}
.icon-select-popover .icon-item {
    display: flex;
    align-items: center;
    justify-content: center;
    aspect-ratio: 1;
    font-size: 1.25rem;
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--app-radius);
    cursor: pointer;
    color: var(--app-text-primary);
}
.icon-select-popover .icon-item:hover {
    background: var(--app-hover);
}
.icon-select-popover .icon-item--selected {
    background: var(--app-accent-soft);
    color: var(--app-accent);
    border-color: var(--app-accent);
}
.icon-select-popover .icons-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 1.5rem;
    color: var(--app-text-secondary);
}
</style>
