/* Augments a track list that _shared.js has already filled, turning the # cell
   into a checkbox and marking the rows named in data-select (1-based, comma
   separated) as selected. Loaded after _shared.js and only by the 07 Duotone
   flow mocks, so the base hero files are unaffected. */
(function () {
    document.querySelectorAll('.tracklist[data-select]').forEach(function (tl) {
        var sel = (tl.getAttribute('data-select') || '')
            .split(',')
            .map(function (s) {
                return s.trim()
            })
            .filter(Boolean)

        var headNum = tl.querySelector('.tl-head .num')
        if (headNum) headNum.innerHTML = '<span class="cbx some" title="Select all"></span>'

        tl.querySelectorAll('.tl-row').forEach(function (row, i) {
            var on = sel.indexOf(String(i + 1)) >= 0
            if (on) row.classList.add('selected')
            var num = row.querySelector('.num')
            if (num) num.innerHTML = '<span class="cbx' + (on ? ' on' : '') + '"></span>'
        })
    })
})()
