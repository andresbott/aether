// A station returned by the radio-browser.info search proxy (/api/v1/radiobrowser/search).
export interface RadioBrowserStation {
    name: string
    streamUrl: string
    homepage: string
    favicon: string
    tags: string
    country: string
    countryCode: string
    language: string
    codec: string
    bitrate: number
    votes: number
    uuid: string
}

// Initial values used to seed the Add Station form when importing a station
// from radio-browser. coverFile, when present, is the fetched favicon.
export interface RadioStationPrefill {
    name: string
    streamUrl: string
    homepageUrl?: string
    coverFile?: File
}
