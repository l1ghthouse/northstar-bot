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
	"holo_shift_mod": func() Mod {
		return &ThunderstoreMod{
			enabledByDefault: false,
			name:             "HoloShift",
			requiredByClient: false,
		}
	},
	"parseable_logs": func() Mod {
		return &ThunderstoreMod{
			enabledByDefault: false,
			name:             "ParseableLogs",
			requiredByClient: false,
		}
	},
	"ramp_water": func() Mod {
		return &ThunderstoreMod{
			enabledByDefault: true,
			name:             "RampWater",
			requiredByClient: false,
		}
	},
	"better_homestead": func() Mod {
		return &ThunderstoreMod{
			enabledByDefault: true,
			name:             "BetterHomestead",
			requiredByClient: false,
		}
	},
}

type ThunderstoreMod struct {
	enabledByDefault bool
	name             string
	requiredByClient bool
}

func (h ThunderstoreMod) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	return latestThunderstoreMod(ctx, h.name, h.requiredByClient)
}

func (h ThunderstoreMod) EnabledByDefault() bool {
	return h.enabledByDefault
}

var ErrNoTagsFound = fmt.Errorf("no tags found")
