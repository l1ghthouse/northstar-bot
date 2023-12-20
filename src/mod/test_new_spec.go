package mod

import (
	"context"
	"fmt"
	"strings"
)

type TestNewSpec struct{}

func (r TestNewSpec) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	linkOrigin := "https://raw.githubusercontent.com/l1ghthouse/NorthstarMods/main/"
	fileContainerOrigin := "/usr/lib/northstar/R2Northstar/mods/"
	files := []string{
		"Northstar.CustomServers/mod/scripts/vscripts/mp/_gamestate_mp.nut",
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

func (r TestNewSpec) Validate(otherMods []Mod) error {
	return nil
}

func (r TestNewSpec) EnabledByDefault() bool {
	return false
}
