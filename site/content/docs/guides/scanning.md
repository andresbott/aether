---
title: "Scanning"
weight: 5
---

# Scanning

A scan reads the music in your [scan folders](/docs/getting-started/configuration.md#scan-folders) into the catalog you browse. Your favorites, playlists, play history and play queue all point at tracks in that catalog, so it matters what a scan keeps when your files change — most of this page is about that.

## Running a scan

**Settings → Tasks** has two scans, to run now or on a schedule:

| Task | Reads |
|---|---|
| **Catalog Scan** | the files that are new or changed since the last scan — the everyday one |
| **Full Catalog Scan** | every file, again |

Both add new files and drop the tracks whose file is gone. Run a full scan when:

- you added or replaced a cover or artist image in a folder by hand — a Catalog Scan only looks again at folders where a music file changed;
- you upgraded to a version that recognises moved files in more formats, see [After an upgrade](#after-an-upgrade).

## What gets scanned

| Format | Extensions |
|---|---|
| FLAC | `.flac` |
| MP3 | `.mp3` |
| MP4: AAC or Apple Lossless | `.m4a`, `.m4b`, `.mp4` |
| Ogg Vorbis, Opus | `.ogg`, `.oga`, `.opus` |
| WAV | `.wav` |
| AIFF | `.aiff` |
| WMA | `.wma` |
| AAC | `.aac` |

Files with any other extension are skipped — APE, WavPack and DSF, for example. So are `.cue` sheets: an album ripped to a single file with a cue sheet shows up as one long track.

> [!IMPORTANT]
> A WMA or `.aac` file that is moved **and** retagged in one go — what Picard and beets do when they rename files from their tags — loses its favorite, playlist entries and play history. Every other format keeps them, see [When files move or change](#when-files-move-or-change).

## When files move or change

A track keeps its favorite, its playlist entries, its play history and its place in the play queue as long as Aether can tell it is still the same file. It can when the file is:

- **edited in place** — retagged, or given new cover art;
- **moved or renamed**, in any format, as long as one scan sees both halves: the file gone from its old place and present at its new one;
- **moved and retagged in one go**, but only in the formats Aether recognises by their audio: FLAC, MP3, MP4 (`.m4a`, `.m4b`, `.mp4`), Ogg Vorbis, Opus, WAV and AIFF. For WMA and `.aac` files, do it in two steps with a scan in between: retag them where they are, scan, then move them.

It cannot when the file is **copied**: while the original is still there, the copy is a new track, and when the original goes, its history goes with it. To move files, move them — or delete the originals before the next scan.

A track whose file is gone when a scan runs is removed, together with all of the above.

## Reorganising your collection

- **Finish, then scan.** A move is only recognised when a single scan sees the file disappear and reappear. Move files out of your scan folders, scan, move them back, and they come back as new tracks without their favorites, playlist entries and history — even though the files never changed.
- **Pause scheduled scans** while you rearrange by hand: in **Settings → Tasks**, edit the scan's schedule and untick **Enabled**. Run a Catalog Scan when you are done.
- **Moving a whole scan folder** means moving it and changing its `Path`, see [Scan folders](/docs/getting-started/configuration.md#scan-folders): the old location must be gone by the next scan.

## Renaming artists, albums and genres

Aether knows an artist by its name, ignoring capitals and accents, and a genre by its exact name. Correct one in your tags — in the metadata editor or in another tagger — and Aether sees a new artist or genre:

- the artist's favorite is lost, and links to its old page stop working;
- an image you uploaded for the artist is lost, unless the artist has a MusicBrainz ID;
- a genre loses the cover you uploaded for it.

Albums fare better: retag all of an album's tracks together — in one save in the metadata editor, or all before the next scan — and it keeps its favorite, its cover and its place among the newest albums. Retag only some of them and they split off into a new album.

## Network shares and mounts

> [!WARNING]
> Mount a network share or another disk at a scan folder's root — the `Path` itself — never at a directory inside a scan folder.

When a scan folder's root is missing, or empty because its share is not mounted, scans refuse to run and nothing is lost. A share mounted *inside* a scan folder that is missing when a scan runs looks like deleted files instead: its tracks are removed with their favorites, playlist entries and history — and a track with an identical copy added elsewhere since the last scan can hand its history to that copy.

A share that hangs, rather than failing, stalls whatever reads from it: playing its tracks, showing their covers, browsing its folders in the library dialog or the metadata editor. A scan stuck on it cannot be cancelled, and changes saved in the metadata editor do not reach the library until the share answers again.

## Tags

- **Sort tags are not read** (`ARTISTSORT`, `ALBUMSORT` and the like): artists and albums sort by their names as written, so "The Beatles" sorts under T.
- **Text that is not valid UTF-8** — usually Windows-1252 that old taggers wrote into FLAC, Ogg or Opus files — reads as empty. Such a track shows up under "Unknown Artist" or "Unknown Album", or without a title.

> [!CAUTION]
> Saving such a file in the metadata editor deletes that text from it, even when you changed a different field. The editor shows the field empty: retype it in the same save, or repair the encoding in another tagger first.

## After an upgrade

Aether recognises a file that was moved and retagged by a fingerprint of its audio, taken when a scan reads the file. A Catalog Scan only reads files that changed, so when a new version adds formats to the ones it fingerprints, the files already in your catalog get no fingerprint until a scan reads them again — and until then, moving and retagging them loses their history.

Run one Full Catalog Scan after such an upgrade. Version 0.8.0 was one: it added WAV, AIFF, Ogg Vorbis and Opus. Starting over from an empty data directory needs nothing extra, since the first scan reads every file.
