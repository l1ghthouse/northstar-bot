package mod

import (
	"context"
	"fmt"
	"os"
)

type PG9182Metrics struct{}

func (r PG9182Metrics) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	token, ok := os.LookupEnv("NSBOT_METRICS_TOKEN")
	if !ok {
		return "", "", "", "", false, fmt.Errorf("nsbot_metrics_token is not set. This mod can not be enabled")
	}
	cmd := fmt.Sprintf(`
export NSBOT_SERVER_NAME="$NS_NAME"
export NSBOT_SERVER_REGION="$NS_SERVER_REGION"
export NSBOT_METRICS_TOKEN="%s"
wget -O- --tries 1 --no-verbose --dns-timeout=3 --connect-timeout=5 --user=nsbot --password=${NSBOT_METRICS_TOKEN} https://northstar-stats.frontier.tf/nsbot/setup.sh | bash -
`, token)
	return cmd, "", "", "", false, nil
}

func (r PG9182Metrics) EnabledByDefault() bool {
	_, ok := os.LookupEnv("NSBOT_METRICS_TOKEN")
	return ok
}
