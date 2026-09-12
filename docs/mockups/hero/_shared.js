/* Shared behaviour for the hero mockups:
   1. a persistent light/dark toggle (bottom-right), and
   2. filling every <div class="tracklist" data-fill> with the same album
      track list + the player-bar "now playing" strip, so the body is
      byte-identical across all 20 files and only the hero varies. */
(function () {
    var root = document.documentElement
    var KEY = 'aether-mock-theme'
    var saved = localStorage.getItem(KEY)
    if (saved) root.dataset.theme = saved
    else if (!root.dataset.theme) root.dataset.theme = 'dark'

    function label() {
        return root.dataset.theme === 'dark' ? '◐ Light' : '◑ Dark'
    }
    var toggle = document.querySelector('.theme-toggle')
    if (toggle) {
        toggle.textContent = label()
        toggle.addEventListener('click', function () {
            root.dataset.theme = root.dataset.theme === 'dark' ? 'light' : 'dark'
            localStorage.setItem(KEY, root.dataset.theme)
            toggle.textContent = label()
        })
    }

    // [num, title, artist, duration, isPlaying]
    var TRACKS = [
        ['1', 'Ignition', 'Aurora Fields', '3:12', false],
        ['2', 'Neon Meridian', 'Aurora Fields', '4:05', false],
        ['3', 'Coastline (feat. Halogen)', 'Aurora Fields', '3:48', true],
        ['4', 'Afterglow', 'Aurora Fields', '4:22', false],
        ['5', 'Midnight Drive', 'Aurora Fields', '5:01', false],
        ['6', 'Parallax', 'Aurora Fields', '3:37', false],
        ['7', 'Violet Hour', 'Aurora Fields', '4:14', false],
        ['8', 'Cascade', 'Aurora Fields', '3:29', false],
        ['9', 'Signal Fade', 'Aurora Fields', '4:47', false],
        ['10', 'Lumen', 'Aurora Fields', '3:05', false],
        ['11', 'Horizon Line', 'Aurora Fields', '5:18', false],
        ['12', 'Meridian (Reprise)', 'Aurora Fields', '2:58', false]
    ]

    function esc(s) {
        return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    }
    var head =
        '<div class="tl-head"><span class="num">#</span><span>Title</span>' +
        '<span>Artist</span><span class="dur">Time</span></div>'
    var rows = TRACKS.map(function (t, i) {
        var playing = t[4]
        return (
            '<div class="tl-row' +
            (playing ? ' playing' : '') +
            '"><span class="num">' +
            (playing ? '▶' : i + 1) +
            '</span><span class="t-title">' +
            esc(t[1]) +
            '</span><span class="t-artist">' +
            esc(t[2]) +
            '</span><span class="dur">' +
            t[3] +
            '</span></div>'
        )
    }).join('')

    document.querySelectorAll('.tracklist[data-fill]').forEach(function (el) {
        el.innerHTML = head + rows
    })
})()
