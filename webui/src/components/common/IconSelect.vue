<script setup lang="ts">
import { ref, shallowRef, computed, nextTick } from 'vue'
import Popover from 'primevue/popover'
import InputText from 'primevue/inputtext'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import LibraryIcon from '@/components/common/LibraryIcon.vue'
import { searchIcons, MAX_ICON_RESULTS } from '@/lib/iconSearch'
import { DEFAULT_LIBRARY_ICON } from '@/lib/libraryIcons'

const props = withDefaults(
    defineProps<{
        modelValue?: string
    }>(),
    { modelValue: DEFAULT_LIBRARY_ICON }
)

const emit = defineEmits<{
    (e: 'update:modelValue', v: string): void
    // Whether the icon Popover is open. PrimeVue's Popover hides on Escape
    // without stopping propagation and binds its own document listener, so a
    // Dialog hosting this picker has to stop closing on Escape while it is
    // open — or one key press closes both and the form is lost.
    (e: 'update:open', open: boolean): void
}>()

const popoverRef = ref<InstanceType<typeof Popover> | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const searchInputRef = ref<{ $el?: HTMLInputElement } | null>(null)
const searchQuery = ref('')
const isOpen = ref(false)
const popoverWidth = ref('360px')

// The ~4,000 icon names are admin-only, so they are their own chunk, fetched
// the first time the picker opens.
const catalogue = shallowRef<readonly string[] | null>(null)
const catalogueFailed = ref(false)
async function loadCatalogue() {
    if (catalogue.value) return
    catalogueFailed.value = false
    try {
        catalogue.value = (await import('virtual:material-symbols/catalogue')).default
    } catch {
        catalogueFailed.value = true
    }
}

const filteredIcons = computed(() => (catalogue.value ? searchIcons(catalogue.value, searchQuery.value) : []))
const truncated = computed(() => searchQuery.value.trim() !== '' && filteredIcons.value.length === MAX_ICON_RESULTS)

const humanize = (name: string) => name.replace(/_/g, ' ')

function toggleDropdown(event: Event) {
    popoverRef.value?.toggle(event)
}

function onPopoverShow() {
    isOpen.value = true
    emit('update:open', true)
    searchQuery.value = ''
    void loadCatalogue()
    if (triggerRef.value) {
        popoverWidth.value = `${triggerRef.value.offsetWidth}px`
    }
    nextTick(() => {
        searchInputRef.value?.$el?.focus()
    })
}

function onPopoverHide() {
    isOpen.value = false
    emit('update:open', false)
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
            <LibraryIcon :name="props.modelValue" class="trigger-icon" />
            <span class="trigger-label">{{ humanize(props.modelValue) }}</span>
            <i
                class="ms-keyboard-arrow-down trigger-chevron"
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
                        <InputIcon class="ms-search" />
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
                        :title="humanize(icon)"
                        @click="selectIcon(icon)"
                    >
                        <LibraryIcon :name="icon" />
                    </button>
                </div>

                <div v-if="!catalogue && !catalogueFailed" class="icons-empty">
                    <i class="ms-progress-activity icon-spin"></i>
                </div>
                <div v-else-if="catalogueFailed" class="icons-empty">
                    <i class="ms-error"></i>
                    <p>Could not load the icon list</p>
                </div>
                <div v-else-if="filteredIcons.length === 0" class="icons-empty">
                    <i class="ms-search"></i>
                    <p>No icons found for "{{ searchQuery }}"</p>
                </div>
                <p v-else-if="truncated" class="icons-hint">Showing the first {{ MAX_ICON_RESULTS }} — type to narrow</p>
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
.icon-select-popover .icons-hint {
    margin: 0;
    padding: 0 0.75rem 0.75rem;
    font-size: 0.8rem;
    color: var(--app-text-secondary);
}
</style>
