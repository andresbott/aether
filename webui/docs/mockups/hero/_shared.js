/* ==========================================================================
   Shared behaviour + sample data for the hero-header mockups.
   Loaded by every hero-NN.html. Keeps each mockup focused on its hero markup:
   the song list / album grid below the hero are rendered from here, and generic
   edit interactions (edit-mode, drawers, modals, popovers, toasts) are wired via
   data-attributes so individual files stay lean.
   ========================================================================== */
(function () {
    'use strict';

    // --- Theme toggle --------------------------------------------------------
    function initTheme() {
        var btn = document.getElementById('themeToggle');
        if (!btn) return;
        var saved = localStorage.getItem('mock-theme');
        if (saved === 'dark') document.body.classList.add('dark-mode');
        syncIcon();
        btn.addEventListener('click', function () {
            document.body.classList.toggle('dark-mode');
            localStorage.setItem(
                'mock-theme',
                document.body.classList.contains('dark-mode') ? 'dark' : 'light'
            );
            syncIcon();
        });
        function syncIcon() {
            var dark = document.body.classList.contains('dark-mode');
            btn.innerHTML = '<i class="pi ' + (dark ? 'pi-sun' : 'pi-moon') + '"></i>';
        }
    }

    // --- Sample data ---------------------------------------------------------
    var img = function (seed, size) {
        return 'https://picsum.photos/seed/' + seed + '/' + (size || 300);
    };

    var SONGS = [
        { t: 'Nightshade', a: 'Aurora Vale', d: '3:42', s: 'trk1' },
        { t: 'Paper Lanterns', a: 'The Hollow Coast', d: '4:15', s: 'trk2' },
        { t: 'Glass Horizon', a: 'Mira Sol', d: '2:58', s: 'trk3' },
        { t: 'Undertow', a: 'Aurora Vale', d: '5:03', s: 'trk4' },
        { t: 'Cinder & Smoke', a: 'Field of Reeds', d: '3:27', s: 'trk5' },
        { t: 'Long Way Down', a: 'The Hollow Coast', d: '4:41', s: 'trk6' },
        { t: 'Static Bloom', a: 'Mira Sol', d: '3:12', s: 'trk7' },
        { t: 'Evergreen', a: 'Field of Reeds', d: '4:08', s: 'trk8' }
    ];

    var ALBUMS = [
        { n: 'Distant Signals', y: 2023, s: 'alb1' },
        { n: 'The Quiet Hour', y: 2021, s: 'alb2' },
        { n: 'Meridian', y: 2019, s: 'alb3' },
        { n: 'Paper Lanterns', y: 2017, s: 'alb4' },
        { n: 'First Light', y: 2015, s: 'alb5' },
        { n: 'Debut', y: 2013, s: 'alb6' }
    ];

    // Metadata exposed to the hero markup via window.MOCK for files that want it.
    window.MOCK = {
        img: img,
        playlist: {
            name: 'Late Night Drives',
            owner: 'andres',
            comment: 'Hazy synths and slow guitars for the long way home.',
            songCount: SONGS.length,
            duration: '31 min',
            created: 'Mar 2024',
            updated: '2 days ago',
            visibility: 'Private',
            tags: ['Chill', 'Synthwave', 'Night'],
            cover: img('late-night-playlist', 300)
        },
        artist: {
            name: 'Aurora Vale',
            albumCount: ALBUMS.length,
            songCount: 74,
            plays: '1.2M',
            followers: '48.3k',
            formed: 2011,
            country: 'Reykjavík, IS',
            genres: ['Dream Pop', 'Ambient', 'Shoegaze'],
            bio: 'Aurora Vale builds cavernous, reverb-soaked soundscapes that drift between ambient and dream pop.',
            starred: true,
            cover: img('aurora-vale-artist', 300)
        }
    };

    // --- List renderers ------------------------------------------------------
    function renderSongs(el) {
        el.innerHTML = SONGS.map(function (s, i) {
            return (
                '<div class="track-row">' +
                '<span class="t-idx">' + (i + 1) + '</span>' +
                '<span class="t-art"><img src="' + img(s.s, 80) + '" alt=""></span>' +
                '<span><span class="t-title">' + s.t + '</span><br><span class="t-artist">' + s.a + '</span></span>' +
                '<span class="t-dur">' + s.d + '</span>' +
                '</div>'
            );
        }).join('');
    }

    function renderAlbums(el) {
        el.innerHTML = ALBUMS.map(function (a) {
            return (
                '<a class="album-card" href="#" onclick="return false">' +
                '<span class="cc"><img src="' + img(a.s, 200) + '" alt=""></span>' +
                '<span class="ci"><span class="ct">' + a.n + '</span>' +
                '<span class="cs">' + a.y + '</span></span>' +
                '</a>'
            );
        }).join('');
    }

    function renderLists() {
        document.querySelectorAll('[data-render="songs"]').forEach(renderSongs);
        document.querySelectorAll('[data-render="albums"]').forEach(renderAlbums);
    }

    // --- Generic edit interactions ------------------------------------------
    // data-edit-toggle="<selector>"  -> toggles `.editing` on the matched root
    // data-open="<id>"               -> toggles `.open` on element #id (scrim/drawer/modal/menu)
    // data-close="<id>"              -> removes `.open` from element #id
    // data-toast="<msg>"             -> flashes a toast
    function closest(el, sel) { return el.closest(sel); }

    function initInteractions() {
        document.addEventListener('click', function (e) {
            var t = e.target.closest('[data-edit-toggle],[data-open],[data-close],[data-toast],[data-flip],[data-tab]');
            if (!t) return;

            if (t.hasAttribute('data-edit-toggle')) {
                var root = document.querySelector(t.getAttribute('data-edit-toggle'));
                if (root) root.classList.toggle('editing');
            }
            if (t.hasAttribute('data-open')) {
                var o = document.getElementById(t.getAttribute('data-open'));
                if (o) o.classList.add('open');
            }
            if (t.hasAttribute('data-close')) {
                var c = document.getElementById(t.getAttribute('data-close'));
                if (c) c.classList.remove('open');
            }
            if (t.hasAttribute('data-flip')) {
                var f = document.querySelector(t.getAttribute('data-flip'));
                if (f) {
                    f.classList.toggle('flipped');
                    // Mark the element as mid-flip so styles can suppress controls
                    // (e.g. the edit badge) until the rotation actually finishes.
                    f.classList.add('flipping');
                    var clearFlip = function () {
                        f.classList.remove('flipping');
                        f.removeEventListener('transitionend', clearFlip);
                        clearTimeout(f._flipTimer);
                    };
                    f.addEventListener('transitionend', clearFlip);
                    clearTimeout(f._flipTimer);
                    f._flipTimer = setTimeout(clearFlip, 700); // fallback if transitionend is missed
                }
            }
            if (t.hasAttribute('data-tab')) {
                var group = t.closest('[data-tabgroup]');
                if (group) {
                    group.querySelectorAll('[data-tab]').forEach(function (b) { b.classList.remove('is-active'); });
                    t.classList.add('is-active');
                    var panelSel = t.getAttribute('data-tab');
                    group.querySelectorAll('[data-panel]').forEach(function (p) {
                        p.classList.toggle('is-active', '#' + p.id === panelSel);
                    });
                }
            }
            if (t.hasAttribute('data-toast')) {
                showToast(t.getAttribute('data-toast'));
            }
        });
    }

    var toastTimer;
    function showToast(msg) {
        var el = document.getElementById('mockToast');
        if (!el) {
            el = document.createElement('div');
            el.id = 'mockToast';
            el.className = 'toast';
            document.body.appendChild(el);
        }
        el.textContent = msg || 'Saved';
        el.classList.add('show');
        clearTimeout(toastTimer);
        toastTimer = setTimeout(function () { el.classList.remove('show'); }, 1600);
    }

    document.addEventListener('DOMContentLoaded', function () {
        initTheme();
        renderLists();
        initInteractions();
    });
})();
