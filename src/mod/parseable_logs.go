package mod

import (
	"context"
)

type ParseableLogs struct{}

const ParseableLogsModName = "ParseableLogs"

func (p ParseableLogs) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	return latestThunderstoreMod(ctx, ParseableLogsModName, false)
}

func (p ParseableLogs) EnabledByDefault() bool {
	return true
}
