<script setup lang="ts">
import markUrl from '@/assets/aether-mark.svg'

// The Aether diamond mark, for the three places the brand appears: the sidebar
// header, the mobile drawer header and the login card. The art is the cleaned
// diamond rendition of the app icon (`assets/aether-mark.svg`, derived from the
// `zarf/icon/icon.svg` Inkscape master — edit it there, not in the asset).
//
// Always decorative (empty alt + aria-hidden): every use sits right next to the
// "Aether" wordmark, so announcing the image too would only repeat the name.
// Shipped as an <img> rather than inline SVG on purpose — the gradient carries a
// document-wide id, and two inlined copies (sidebar + drawer both exist in the
// DOM at the tablet breakpoint) would collide on it.
withDefaults(
    defineProps<{
        // Any CSS length; the mark is square, so it sets both dimensions.
        size?: string
    }>(),
    { size: '1.75rem' }
)
</script>

<template>
    <img
        class="brand-mark"
        :src="markUrl"
        :style="{ width: size, height: size }"
        alt=""
        aria-hidden="true"
        draggable="false"
    />
</template>

<style scoped>
.brand-mark {
    display: block;
    flex-shrink: 0;
    /* The diamond has no flat sides to align to, so it reads as slightly high
       next to a wordmark baseline unless nudged down a hair. */
    transform: translateY(1px);
}
</style>
