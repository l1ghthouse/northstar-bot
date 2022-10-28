package mod

import (
	"context"
	"fmt"
	"strings"
)

type TestCTFSpawns struct{}

func (r TestCTFSpawns) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	linkOrigin := "https://raw.githubusercontent.com/R2Northstar/NorthstarMods/maybe-better-ctf-spawns-pr/"
	fileContainerOrigin := "/usr/lib/northstar/R2Northstar/mods/"
	files := []string{
		"Northstar.CustomServers/mod/scripts/vscripts/gamemodes/_gamemode_ctf.nut",
		"Northstar.CustomServers/mod/scripts/vscripts/mp/spawn.nut",
	}

	wget := ""
	dockerArgs := ""

	for _, link := range files {
		f := strings.Split(link, "/")
		filePath := "/" + f[len(f)-1]
		wget += fmt.Sprintf("wget %s -O %s \n", linkOrigin+link, filePath)
		dockerArgs += fmt.Sprintf("--mount \"type=bind,source=%s,target=%s,readonly\"", filePath, fileContainerOrigin+link)
		dockerArgs += " "
	}

	return wget, dockerArgs, "", "latest", false, nil
}

func (r TestCTFSpawns) Validate(otherMods []Mod) error {
	return nil
}

func (r TestCTFSpawns) EnabledByDefault() bool {
	return false
}
