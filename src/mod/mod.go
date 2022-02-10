package mod

import (
	"context"
	"fmt"
)

type Mod interface {
	ModParams(ctx context.Context) (cmd string, dockerArgs string, downloadLink string, version string, requiredByClient bool, err error)
}

var ByName = map[string]func() Mod{
	"rebalanced_lts_mod": func() Mod { return &RebalancedLTS{} },
	"holo_shift_mod":     func() Mod { return &HoloShift{} },
	"parseable_logs":     func() Mod { return &ParseableLogs{} },
}

var ErrNoTagsFound = fmt.Errorf("no tags found")
