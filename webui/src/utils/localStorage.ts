export function saveToLocalStorage<T>(key: string, value: T): boolean {
    try {
        const serialized = JSON.stringify(value)
        localStorage.setItem(key, serialized)
        return true
    } catch (error) {
        console.error(`Failed to save to localStorage (key: ${key}):`, error)
        return false
    }
}

export function loadFromLocalStorage<T>(key: string, defaultValue: T): T {
    try {
        const item = localStorage.getItem(key)
        if (item === null) {
            return defaultValue
        }
        return JSON.parse(item) as T
    } catch (error) {
        console.error(`Failed to load from localStorage (key: ${key}):`, error)
        return defaultValue
    }
}
