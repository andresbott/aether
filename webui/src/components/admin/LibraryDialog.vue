<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import InputSwitch from 'primevue/inputswitch'
import Textarea from 'primevue/textarea'
import Dropdown from 'primevue/dropdown'
import Message from 'primevue/message'
import type { Library, LibraryInput } from '@/types/libraries'

const props = defineProps<{
    visible: boolean
    library: Library | null  // null = create mode
    submitting: boolean
}>()

const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'submit', input: LibraryInput): void
    (e: 'cancel'): void
}>()

const mvOptions = [
    { label: 'None', value: 'none' },
    { label: 'Multi (separate tag fields)', value: 'multi' },
    { label: 'Delim (split single tag)', value: 'delim' }
]

interface FormState {
    name: string
    path: string
    excludesText: string
    follow_symlinks: boolean
    genreMode: string
    genreDelim: string
    artistMode: string
    artistDelim: string
    albumArtistMode: string
    albumArtistDelim: string
}

function emptyForm(): FormState {
    return {
        name: '',
        path: '',
        excludesText: '',
        follow_symlinks: true,
        genreMode: 'none',
        genreDelim: '',
        artistMode: 'none',
        artistDelim: '',
        albumArtistMode: 'none',
        albumArtistDelim: ''
    }
}

function parseMV(raw: string): { mode: string; delim: string } {
    const r = (raw ?? '').trim()
    if (r.startsWith('delim ')) {
        return { mode: 'delim', delim: r.slice('delim '.length) }
    }
    if (r === 'multi') return { mode: 'multi', delim: '' }
    return { mode: 'none', delim: '' }
}

function buildMV(mode: string, delim: string): string {
    if (mode === 'delim') return `delim ${delim}`
    if (mode === 'multi') return 'multi'
    return 'none'
}

const form = ref<FormState>(emptyForm())
const initialPath = ref('')

watch(
    () => [props.visible, props.library],
    () => {
        if (!props.visible) return
        if (props.library) {
            const lib = props.library
            const g = parseMV(lib.multi_value_genre)
            const a = parseMV(lib.multi_value_artist)
            const aa = parseMV(lib.multi_value_album_artist)
            form.value = {
                name: lib.name,
                path: lib.path,
                excludesText: (lib.exclude_patterns ?? []).join('\n'),
                follow_symlinks: lib.follow_symlinks,
                genreMode: g.mode, genreDelim: g.delim,
                artistMode: a.mode, artistDelim: a.delim,
                albumArtistMode: aa.mode, albumArtistDelim: aa.delim
            }
            initialPath.value = lib.path
        } else {
            form.value = emptyForm()
            initialPath.value = ''
        }
    },
    { immediate: true }
)

const isEditMode = computed(() => props.library !== null)
const pathChanged = computed(() => isEditMode.value && form.value.path !== initialPath.value)

function buildInput(): LibraryInput {
    const excludes = form.value.excludesText
        .split('\n')
        .map((s) => s.trim())
        .filter((s) => s.length > 0)
    return {
        name: form.value.name.trim(),
        path: form.value.path.trim(),
        exclude_patterns: excludes,
        follow_symlinks: form.value.follow_symlinks,
        multi_value_genre: buildMV(form.value.genreMode, form.value.genreDelim),
        multi_value_artist: buildMV(form.value.artistMode, form.value.artistDelim),
        multi_value_album_artist: buildMV(form.value.albumArtistMode, form.value.albumArtistDelim)
    }
}

function onSubmit() {
    emit('submit', buildInput())
}

function onCancel() {
    emit('cancel')
    emit('update:visible', false)
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        modal
        :header="isEditMode ? 'Edit Library' : 'Add Library'"
        :style="{ width: '32rem' }"
    >
        <div class="form-grid">
            <label>Name</label>
            <InputText v-model="form.name" placeholder="e.g. Main" />

            <label>Path</label>
            <InputText v-model="form.path" placeholder="/srv/music" />

            <Message v-if="pathChanged" severity="warn" :closable="false">
                Changing the path will wipe existing tracks under the old path.
                The library will be empty until the next scan.
            </Message>

            <label>Follow symlinks</label>
            <InputSwitch v-model="form.follow_symlinks" />

            <label>Exclude patterns</label>
            <Textarea
                v-model="form.excludesText"
                rows="4"
                placeholder="One Go regex per line"
            />

            <label>Genre tags</label>
            <div class="mv-row">
                <Dropdown
                    v-model="form.genreMode"
                    :options="mvOptions"
                    optionLabel="label"
                    optionValue="value"
                />
                <InputText
                    v-if="form.genreMode === 'delim'"
                    v-model="form.genreDelim"
                    placeholder="separator (e.g. ;)"
                />
            </div>

            <label>Artist tags</label>
            <div class="mv-row">
                <Dropdown
                    v-model="form.artistMode"
                    :options="mvOptions"
                    optionLabel="label"
                    optionValue="value"
                />
                <InputText
                    v-if="form.artistMode === 'delim'"
                    v-model="form.artistDelim"
                    placeholder="separator"
                />
            </div>

            <label>Album-artist tags</label>
            <div class="mv-row">
                <Dropdown
                    v-model="form.albumArtistMode"
                    :options="mvOptions"
                    optionLabel="label"
                    optionValue="value"
                />
                <InputText
                    v-if="form.albumArtistMode === 'delim'"
                    v-model="form.albumArtistDelim"
                    placeholder="separator"
                />
            </div>
        </div>

        <template #footer>
            <Button label="Cancel" text @click="onCancel" />
            <Button
                :label="isEditMode ? 'Save' : 'Create'"
                :loading="submitting"
                @click="onSubmit"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.form-grid {
    display: grid;
    grid-template-columns: 10rem 1fr;
    gap: 0.75rem 1rem;
    align-items: center;
}
.form-grid label {
    font-weight: 500;
}
.form-grid > .p-message {
    grid-column: 2 / 3;
}
.mv-row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
}
.mv-row .p-dropdown {
    min-width: 12rem;
}
</style>
