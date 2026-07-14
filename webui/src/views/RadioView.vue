<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import RadioStationListView from '@/components/library/RadioStationListView.vue'
import RadioStationGrid from '@/components/library/RadioStationGrid.vue'
import { useRadioStations } from '@/composables/useSubsonicQueries'

type Layout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()

const layoutOptions = [
    { label: 'List', value: 'list', icon: 'pi pi-list' },
    { label: 'Grid', value: 'grid', icon: 'pi pi-th-large' }
]

const layout = computed<Layout>({
    get: () => (route.query.view === 'list' ? 'list' : 'grid'),
    set: (v) => {
        const query = { ...route.query }
        if (v === 'list') query.view = 'list'
        else delete query.view
        router.replace({ query })
    }
})

const { data: stations } = useRadioStations()

const summary = computed(() => {
    const count = stations.value?.length ?? 0
    if (count === 0) return ''
    return `${count} ${count === 1 ? 'station' : 'stations'}`
})
</script>

<template>
    <ContentScaffold title="Radio" :summary="summary">
        <template #actions>
            <SelectButton
                v-model="layout"
                :options="layoutOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                dataKey="value"
                aria-label="Layout"
            >
                <template #option="slotProps">
                    <i :class="slotProps.option.icon"></i>
                </template>
            </SelectButton>
        </template>

        <RadioStationListView v-if="layout === 'list'" />
        <RadioStationGrid v-else />
    </ContentScaffold>
</template>
