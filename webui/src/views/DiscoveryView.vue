<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import DiscoverySection from '@/components/library/DiscoverySection.vue'
import { DISCOVERY_SECTIONS } from '@/composables/useDiscovery'

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

const sections = DISCOVERY_SECTIONS
</script>

<template>
    <!-- No summary: a total across five overlapping sections is not a
         meaningful count. -->
    <ContentScaffold title="Discovery">
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

        <div class="discovery-scroll">
            <DiscoverySection
                v-for="section in sections"
                :key="section.key"
                :sectionKey="section.key"
                :layout="layout"
            />
        </div>
    </ContentScaffold>
</template>

<style scoped>
/* Recipe B (see docs/architecture/main-content-view-layout.md): plain scrolling
   body, one scrollbar's worth of rail clearance on the right. --sb-w comes from
   PlayerLayout; never re-measure it here. */
.discovery-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    padding-bottom: 1rem;
    box-sizing: border-box;
}
</style>
