import { ref, onMounted } from 'vue'
import type { Ref } from 'vue'

/**
 * Measures the OS/browser native scrollbar width (0 for overlay scrollbars)
 * by probing a hidden scrolling element. Reusable anywhere a layout needs to
 * account for the scrollbar's footprint. Measured once on mount.
 */
export function useScrollbarWidth(): Ref<number> {
    const width = ref(0)

    onMounted(() => {
        const probe = document.createElement('div')
        probe.style.position = 'absolute'
        probe.style.top = '-9999px'
        probe.style.width = '100px'
        probe.style.height = '100px'
        probe.style.overflow = 'scroll'
        document.body.appendChild(probe)
        width.value = probe.offsetWidth - probe.clientWidth
        probe.remove()
    })

    return width
}
