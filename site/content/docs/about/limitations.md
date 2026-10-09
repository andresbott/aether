---
title: "Known limitations"
weight: 20
---

# Known limitations

What Aether does not do yet, or does with a catch, and what to do about it. Most entries link to the page with the details. A few things it leaves out on purpose are under [Not planned](#not-planned).

## Apps

- **JSON only.** Apps that understand only the original Subsonic XML responses, such as DSub, cannot connect yet.
- **No transcoding, ratings, artist information, lyrics, podcasts or jukebox mode**, among other app features. The full list, and what an app does without them, is under [What apps cannot do yet](/docs/guides/connecting-apps.md#what-apps-cannot-do-yet).
- **Some apps' plays count twice:** Aether records an app's "now playing" report as a play too.
- **Plays stay in Aether.** Nothing is passed on to Last.fm or ListenBrainz.
- **No DLNA or UPnP.** TVs, receivers and other network players cannot browse Aether on their own.

## Web player

- **It plays what your browser plays.** There is no transcoding: FLAC, MP3, AAC, Ogg Vorbis, Opus and WAV play in most browsers, WMA in none, and AIFF and Apple Lossless only in some.
- **Radio stations are left out of the synced play queue.** Another browser picking up your queue does not get them, and they upset the sync: with a station queued before the track that is playing, the other browser resumes at the wrong track or the queue is not saved at all, and a queue of nothing but stations clears the saved one. Keep stations out of a queue you want to carry on elsewhere.
- **Lyrics are not shown**, even when your files carry them.

## Your files

The details are in [Scanning](/docs/guides/scanning.md) and under [Scan folders](/docs/getting-started/configuration.md#scan-folders).

- **APE, WavPack and DSF files are not scanned**, nor anything else outside the [supported formats](/docs/guides/scanning.md#what-gets-scanned). `.cue` sheets are ignored.
- **WMA and `.aac` files moved and retagged in one go** lose their favorites, playlist entries and play history.
- **A move must not straddle two scans.** Reorganise, then scan.
- **Copying a scan folder to a new disk** and pointing `Path` at the copy drops every track's favorites, playlist entries and history, unless the old copy is gone by the next scan.
- **Network shares belong at a scan folder's root**, never inside one, and a share that hangs stalls whatever reads from it.
- **Symlinks that lead outside every scan folder** give tracks that are listed but do not play.
- **Renaming an artist or a genre** in your tags makes a new one, without the old one's favorite or uploaded image.
- **Tag text that is not valid UTF-8** reads as empty, and saving the file in the metadata editor deletes it.
- **Sort tags are not read.** Artists and albums sort by their names.

## Metadata editor

- **Symlinked folders are not listed**, so the music in them cannot be edited in the browser.
- **Album identify can propose the wrong track positions.** Check the **Position** column before applying: each song's position can be changed or cleared there.
- **Lyrics, comments and sort tags have no field of their own.** Change them under **Raw**.

## Libraries and users

- **Library filters only include**, and hiding a library's artists hides them from the whole catalog only, see [Libraries](/docs/guides/libraries.md#worth-knowing).
- **Login names cannot be changed**, and a user renamed in your identity provider arrives as a new user, see [Authentication](/docs/guides/authentication.md).

## Not planned

These were considered and turned down, so don't wait for them in a later version.

- **Share links.** Some apps can ask the server for a link that plays a song or album for anyone who opens it, without signing in. Aether does not create them, because such a link gets around sign-in.
- **Chat.** Subsonic's chat is a single message board for everyone on the server, and hardly any app still offers it.
- **Managing users from an app.** Apps cannot add, change or delete users, or change a password. Do that in the web player, see [Authentication](/docs/guides/authentication.md). An app can still read your own account, which is all it needs to play music.
