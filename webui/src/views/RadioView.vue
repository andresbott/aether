<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import Button from 'primevue/button'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import RadioStationListView from '@/components/library/RadioStationListView.vue'
import RadioStationGrid from '@/components/library/RadioStationGrid.vue'
import StationSearchDialog from '@/components/library/StationSearchDialog.vue'
import { useRadioStations } from '@/composables/useSubsonicQueries'
import type { RadioBrowserStation } from '@/types/radiobrowser'

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

const searchVisible = ref(false)

function openAdd() {
    router.push({ name: 'radio-station-new' })
}

function onDiscoverSelect(station: RadioBrowserStation) {
    const query: Record<string, string> = {
        name: station.name,
        streamUrl: station.streamUrl
    }
    if (station.homepage) query.homepage = station.homepage
    if (station.favicon) query.favicon = station.favicon
    router.push({ name: 'radio-station-new', query })
}
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
            <Button
                class="discover-station"
                label="Discover"
                icon="pi pi-globe"
                outlined
                @click="searchVisible = true"
            />
            <Button
                class="add-station"
                label="Add Station"
                icon="pi pi-plus"
                @click="openAdd"
            />
        </template>

        <RadioStationListView v-if="layout === 'list'" />
        <RadioStationGrid v-else />

        <StationSearchDialog v-model:visible="searchVisible" @select="onDiscoverSelect" />
    </ContentScaffold>
</template>
