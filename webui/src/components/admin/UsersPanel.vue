<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import { useConfirm } from 'primevue/useconfirm'
import ConfirmDialog from 'primevue/confirmdialog'
import {
    useUsers,
    useCreateUser,
    useUpdateUser,
    useDeleteUser
} from '@/composables/useUsers'
import type { User, CreateUserInput, UpdateUserInput } from '@/types/users'
import UserDialog from './UserDialog.vue'
import { useViewport } from '@/composables/useViewport'

const { data: users, isLoading } = useUsers()
const createMutation = useCreateUser()
const updateMutation = useUpdateUser()
const deleteMutation = useDeleteUser()
const confirm = useConfirm()

const dialogVisible = ref(false)
const editing = ref<User | null>(null)

function openCreate() {
    editing.value = null
    dialogVisible.value = true
}
function openEdit(user: User) {
    editing.value = user
    dialogVisible.value = true
}
function onCreate(input: CreateUserInput) {
    createMutation.mutate(input, {
        onSuccess: () => (dialogVisible.value = false)
    })
}
function onUpdate(payload: { id: string; input: UpdateUserInput }) {
    updateMutation.mutate(payload, {
        onSuccess: () => (dialogVisible.value = false)
    })
}
function onDelete(user: User) {
    confirm.require({
        message: `Delete user "${user.login}"? This cannot be undone.`,
        header: 'Delete user?',
        icon: 'pi pi-exclamation-triangle',
        rejectLabel: 'Cancel',
        acceptLabel: 'Delete',
        acceptClass: 'p-button-danger',
        accept: () => deleteMutation.mutate(user.id)
    })
}

const submitting = computed(
    () => createMutation.isPending.value || updateMutation.isPending.value
)

const { tier } = useViewport()
// Spec §5: settings tables must not overflow a phone; these columns are the
// ones a phone admin can live without (the row's dialog still shows all).
const phoneCols = computed(() => tier.value === 'phone')
</script>

<template>
    <section class="section">
        <div class="section-header">
            <h2>Users</h2>
            <Button label="Add user" icon="pi pi-plus" @click="openCreate" />
        </div>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
        </div>

        <div v-else-if="!users || users.length === 0" class="empty-state">
            <p>No users yet.</p>
            <p>Add a user to allow logging in.</p>
        </div>

        <div v-else class="table-fit">
            <DataTable :value="users" responsiveLayout="scroll">
                <Column field="login" header="Login">
                    <template #body="{ data }">
                        <span class="user-login">
                            <i :class="data.role === 'admin' ? 'pi pi-shield' : 'pi pi-user'"></i>
                            {{ data.login }}
                        </span>
                    </template>
                </Column>
                <Column header="Role" style="width: 8rem">
                    <template #body="{ data }">
                        <Tag
                            :severity="data.role === 'admin' ? 'warn' : 'secondary'"
                            :value="data.role === 'admin' ? 'admin' : 'user'"
                        />
                    </template>
                </Column>
                <Column header="Status" :hidden="phoneCols" style="width: 8rem">
                    <template #body="{ data }">
                        <Tag
                            :severity="data.enabled ? 'success' : 'secondary'"
                            :value="data.enabled ? 'enabled' : 'disabled'"
                        />
                    </template>
                </Column>
                <Column header="" style="width: 11rem; text-align: right">
                    <template #body="{ data }">
                        <Button icon="pi pi-pencil" text rounded @click="openEdit(data)" />
                        <Button
                            icon="pi pi-trash"
                            text
                            rounded
                            severity="danger"
                            @click="onDelete(data)"
                        />
                    </template>
                </Column>
            </DataTable>
        </div>

        <UserDialog
            v-model:visible="dialogVisible"
            :user="editing"
            :submitting="submitting"
            @create="onCreate"
            @update="onUpdate"
            @cancel="dialogVisible = false"
        />

        <ConfirmDialog />
    </section>
</template>

<style scoped>
.section {
    margin-bottom: 2.5rem;
}
.section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
}
.section h2 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0;
}
.loading {
    display: flex;
    justify-content: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}
.empty-state {
    text-align: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}
.user-login {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
}
.table-fit {
    overflow-x: auto;
}
</style>
