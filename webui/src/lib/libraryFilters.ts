import type { LibraryFilter, LibraryFilterField, LibraryFilterOptions } from '@/types/libraries'

export interface FieldMeta {
    field: LibraryFilterField
    label: string
    /** what the value control is */
    control: 'scan-folders' | 'paths' | 'options' | 'yes-no'
}

export const FILTER_FIELDS: FieldMeta[] = [
    { field: 'scan_folder', label: 'Scan folder', control: 'scan-folders' },
    { field: 'path', label: 'Folder path', control: 'paths' },
    { field: 'format', label: 'File format', control: 'options' },
    { field: 'release_type', label: 'Release type', control: 'options' },
    { field: 'compilation', label: 'Compilation', control: 'yes-no' },
    { field: 'genre', label: 'Genre', control: 'options' }
]

export const LIMITS = { filters: 20, valuesPerFilter: 100, valuesTotal: 200 } as const

export interface ValueOption {
    label: string
    value: string
    /** the value is stored on the library but is no longer offered by the server */
    missing: boolean
}

/** The label of a value: the empty release type is a real selector, not a blank. */
export function valueLabel(field: LibraryFilterField, value: string): string {
    if (field === 'release_type' && value === '') return '(none)'
    if (field === 'compilation') return value === 'true' ? 'Yes' : 'No'
    return value
}

/**
 * The options a row offers: what the server lists for the field, plus every
 * value the row already holds that the server no longer offers — flagged
 * `missing`, so it stays visible and can be removed instead of silently
 * surviving (or silently vanishing) on the next save. Values are compared and
 * kept EXACTLY as they are: the server matches them against scanned data.
 */
export function optionsFor(
    field: LibraryFilterField,
    current: string[],
    offered: LibraryFilterOptions | undefined
): ValueOption[] {
    const base: string[] =
        field === 'scan_folder' ? (offered?.scan_folders ?? [])
        : field === 'format' ? (offered?.formats ?? [])
        : field === 'genre' ? (offered?.genres ?? [])
        : field === 'release_type' ? ['', ...(offered?.release_types ?? [])]
        : []
    const out: ValueOption[] = base.map((v) => ({ label: valueLabel(field, v), value: v, missing: false }))
    for (const v of current) {
        if (!base.includes(v)) {
            const why = field === 'scan_folder' ? 'not configured' : 'not in the catalog'
            out.push({ label: `${valueLabel(field, v)} (${why})`, value: v, missing: true })
        }
    }
    return out
}

/** One line per filter for the libraries table, e.g. "Genre: Jazz, Ambient". */
export function summarize(filters: LibraryFilter[]): string[] {
    return filters.map((f) => {
        const meta = FILTER_FIELDS.find((m) => m.field === f.field)
        const shown = f.values.slice(0, 3).map((v) => valueLabel(f.field, v))
        const more = f.values.length > 3 ? ` +${f.values.length - 3}` : ''
        return `${meta?.label ?? f.field}: ${shown.join(', ')}${more}`
    })
}

export interface RowErrors {
    field?: string
    values?: string
    /** by value index */
    value: Record<number, string>
}

/**
 * Splits the server's pointer → detail map into per-row errors and the rest.
 * Pointers index the REQUEST's arrays, so row i here is filter i as it was sent.
 */
export function rowErrors(errors: Record<string, string>): { rows: Record<number, RowErrors>; general: string[] } {
    const rows: Record<number, RowErrors> = {}
    const general: string[] = []
    for (const [pointer, detail] of Object.entries(errors)) {
        const m = /^\/filters\/(\d+)\/(field|values)(?:\/(\d+))?$/.exec(pointer)
        if (!m) {
            if (pointer === '/filters') general.push(detail)
            continue
        }
        const row = (rows[Number(m[1])] ??= { value: {} })
        if (m[2] === 'field') row.field = detail
        else if (m[3] === undefined) row.values = detail
        else row.value[Number(m[3])] = detail
    }
    return { rows, general }
}
