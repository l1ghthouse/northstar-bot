package mod

import (
	"context"
	"fmt"
)

type Mod interface {
	ModParams(ctx context.Context) (cmd string, dockerArgs string, downloadLink string, version string, err error)
}

var ModByName = map[string]func() Mod{
	"rebalanced_lts_mod": func() Mod { return &RebalancedLTSMod{} },
	"holo_shift_mod":     func() Mod { return &HoloShift{} },
}

var ErrNoTagsFound = fmt.Errorf("no tags found")
