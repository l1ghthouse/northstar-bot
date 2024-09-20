package mod

import (
	"context"
	"fmt"
)

type TestCTFSpawns struct{}

func (r TestCTFSpawns) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	// mount Northstar.Client  Northstar.Custom  Northstar.CustomServers to respective directories in `/usr/lib/northstar/R2Northstar/mods/`
	cmd := "mkdir /ctf_experimental\n"
	cmd += "git clone --depth 1 -b gamemode_fd_experimental https://github.com/Zanieon/NorthstarMods.git /ctf_experimental\n"
	fileContainerOrigin := "/usr/lib/northstar/R2Northstar/mods/"
	files := []string{
		"Northstar.Client",
		"Northstar.Custom",
		"Northstar.CustomServers",
	}

	dockerArgs := ""
	for _, link := range files {
		filePath := "/ctf_experimental/" + link
		dockerArgs += fmt.Sprintf("--mount \"type=bind,source=%s,target=%s,readonly\"", filePath, fileContainerOrigin+link)
		dockerArgs += " "
	}

	return cmd, dockerArgs, "", "latest", false, nil
}

func (r TestCTFSpawns) Validate(otherMods []Mod) error {
	return nil
}

func (r TestCTFSpawns) EnabledByDefault() bool {
	return false
}
