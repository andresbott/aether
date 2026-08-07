<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import ToggleSwitch from 'primevue/toggleswitch'
import SelectButton from 'primevue/selectbutton'
import type { User, CreateUserInput, UpdateUserInput, UserRole } from '@/types/users'

const props = defineProps<{
    visible: boolean
    user: User | null // null = create mode
    submitting: boolean
}>()

const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'create', input: CreateUserInput): void
    (e: 'update', payload: { id: string; input: UpdateUserInput }): void
    (e: 'cancel'): void
}>()

interface FormState {
    login: string
    password: string
    enabled: boolean
    role: UserRole
}

const roleOptions: { label: string; value: UserRole }[] = [
    { label: 'Regular user', value: 'user' },
    { label: 'Admin', value: 'admin' }
]

function emptyForm(): FormState {
    return { login: '', password: '', enabled: true, role: 'user' }
}

const form = ref<FormState>(emptyForm())

watch(
    () => [props.visible, props.user],
    () => {
        if (!props.visible) return
        if (props.user) {
            // Edit mode: password empty means "keep current".
            form.value = {
                login: props.user.login,
                password: '',
                enabled: props.user.enabled,
                role: props.user.role
            }
        } else {
            form.value = emptyForm()
        }
    },
    { immediate: true }
)

const isEditMode = computed(() => props.user !== null)

const canSubmit = computed(() => {
    // Edit mode always has a login (read-only, prefilled) and an empty password
    // means "keep current", so nothing can be missing.
    if (isEditMode.value) return true
    return form.value.login.trim().length > 0 && form.value.password.length > 0
})

function onSubmit() {
    if (isEditMode.value && props.user) {
        // The login is never sent on an update: per-user data (play queue,
        // stars, playlists, history) is keyed on the login STRING, so a rename
        // would orphan it. The backend rejects a changed login with 400; the
        // field is shown read-only so the UI does not offer a doomed edit.
        const input: UpdateUserInput = { enabled: form.value.enabled }
        if (form.value.password.length > 0) input.password = form.value.password
        if (form.value.role !== props.user.role) input.role = form.value.role
        emit('update', { id: props.user.id, input })
    } else {
        emit('create', {
            login: form.value.login.trim(),
            password: form.value.password,
            enabled: form.value.enabled,
            role: form.value.role
        })
    }
}

function onCancel() {
    emit('cancel')
    emit('update:visible', false)
}
</script>

<template>
    <Dialog
        :visible="visible"
        modal
        :header="isEditMode ? `Edit user ${user?.login}` : 'Add user'"
        :style="{ width: '26rem' }"
        @update:visible="(v: boolean) => emit('update:visible', v)"
    >
        <div class="form">
            <div class="field">
                <label for="user-login">Login</label>
                <InputText
                    id="user-login"
                    v-model="form.login"
                    autocomplete="off"
                    placeholder="username"
                    :readonly="isEditMode"
                    :aria-describedby="isEditMode ? 'user-login-hint' : undefined"
                />
                <small v-if="isEditMode" id="user-login-hint" class="hint">
                    The login cannot be changed after the user is created.
                </small>
            </div>

            <div class="field">
                <label for="user-password">
                    {{ isEditMode ? 'New password' : 'Password' }}
                </label>
                <Password
                    v-model="form.password"
                    input-id="user-password"
                    :feedback="false"
                    toggle-mask
                    autocomplete="new-password"
                    :placeholder="isEditMode ? 'leave empty to keep current' : ''"
                    fluid
                />
            </div>

            <div class="field">
                <label id="user-role-label">Role</label>
                <SelectButton
                    v-model="form.role"
                    :options="roleOptions"
                    optionLabel="label"
                    optionValue="value"
                    :allowEmpty="false"
                    aria-labelledby="user-role-label"
                />
            </div>

            <div class="field field-inline">
                <label for="user-enabled">Enabled</label>
                <ToggleSwitch v-model="form.enabled" input-id="user-enabled" />
            </div>
        </div>

        <template #footer>
            <Button label="Cancel" text :disabled="submitting" @click="onCancel" />
            <Button
                :label="isEditMode ? 'Save' : 'Create'"
                :loading="submitting"
                :disabled="!canSubmit"
                @click="onSubmit"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
}
.field {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
}
.field label {
    font-size: 0.85rem;
    font-weight: 600;
}
.hint {
    color: var(--p-text-muted-color);
    font-size: 0.78rem;
}
.field-inline {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
}
</style>
