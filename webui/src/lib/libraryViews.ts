import type { LibraryView } from '@/types/libraries'

/**
 * The ways a library can be browsed, in the order every surface lists them —
 * the library dialog's checkboxes, `LibraryView`'s view switcher — and the order
 * the server stores a library's `views` in. The sidebar's root Library block
 * lists the same three, in the same order.
 */
export const LIBRARY_VIEWS: readonly { value: LibraryView; label: string; icon: string }[] = [
    { value: 'discover', label: 'Discover', icon: 'pi pi-compass' },
    { value: 'artists', label: 'Artists', icon: 'pi pi-users' },
    { value: 'releases', label: 'Releases', icon: 'pi pi-images' }
]

/** Every view, in display order. */
export const ALL_LIBRARY_VIEWS: readonly LibraryView[] = LIBRARY_VIEWS.map((v) => v.value)

/** The views in `views`, in display order and each once. */
export function inDisplayOrder(views: readonly LibraryView[]): LibraryView[] {
    return ALL_LIBRARY_VIEWS.filter((v) => views.includes(v))
}

/**
 * The view a library opens on: its default when it offers that view, else the
 * first view it offers; undefined when it offers none.
 */
export function openingView(
    views: readonly LibraryView[],
    defaultView?: LibraryView
): LibraryView | undefined {
    if (defaultView && views.includes(defaultView)) return defaultView
    return inDisplayOrder(views)[0]
}
