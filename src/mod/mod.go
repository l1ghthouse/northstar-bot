package mod

import (
	"context"
	"fmt"
)

type Mod interface {
	ModParams(ctx context.Context) (cmd string, dockerArgs string, downloadLink string, version string, requiredByClient bool, err error)
	EnabledByDefault() bool
}

var ByName = map[string]func() Mod{
	"rebalanced_lts_mod": func() Mod { return &RebalancedLTS{} },
	"holo_shift_mod":     func() Mod { return &HoloShift{} },
	"parseable_logs":     func() Mod { return &ParseableLogs{} },
	"ramp_water":         func() Mod { return &RampWater{} },
}

var ErrNoTagsFound = fmt.Errorf("no tags found")
