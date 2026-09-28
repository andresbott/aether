package metainfo

import (
	"strconv"
	"strings"
	"time"
)

func init() {
	if BuildTime == "" {
		BuildTime = time.Now().Format(time.RFC3339)
	}
}

var Version = "dev-build"
var BuildTime = ""
var ShaVer = "undefined"

// AcoustID application keys, one per release line so usage statistics can be
// told apart on acoustid.org.
const (
	acoustIDKeyDev = "DslKKEx8DO"
	acoustIDKeyV1  = "40kOX9zexq"
)

// AcoustIDAppKey returns the AcoustID application key for a version string.
// Dev builds and versions below 1.x use the dev key; 1.x releases use their
// own key. New release lines get their own key here when one is registered.
// An empty return disables audio identification.
func AcoustIDAppKey(version string) string {
	if majorVersion(version) < 1 {
		return acoustIDKeyDev
	}
	return acoustIDKeyV1
}

// majorVersion extracts the leading major number from strings like "1.2.3" or
// "v0.4.0". Anything unparsable (e.g. "dev-build") counts as 0.
func majorVersion(version string) int {
	v := strings.TrimPrefix(version, "v")
	head, _, _ := strings.Cut(v, ".")
	n, err := strconv.Atoi(head)
	if err != nil {
		return 0
	}
	return n
}
