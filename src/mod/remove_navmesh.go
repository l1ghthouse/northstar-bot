package mod

import (
	"context"
	"fmt"
)

type RemoveNavmesh struct{}

func (r RemoveNavmesh) ModParams(ctx context.Context) (string, string, string, string, bool, error) {

	fileContainerOrigin := "/usr/lib/northstar/R2Northstar/mods/"
	folders := []string{
		"Northstar.CustomServers/mod/maps/graphs",
		"Northstar.CustomServers/mod/maps/navmesh",
	}

	emptyDir := "/empty_dir"
	cmd := fmt.Sprintf("mkdir %s", emptyDir)
	dockerArgs := ""

	for _, path := range folders {
		dockerArgs += fmt.Sprintf("--mount \"type=bind,source=%s,target=%s,readonly\"", emptyDir, fileContainerOrigin+path)
		dockerArgs += " "
	}

	return cmd, dockerArgs, "", "latest", false, nil
}

func (r RemoveNavmesh) EnabledByDefault() bool {
	return false
}
