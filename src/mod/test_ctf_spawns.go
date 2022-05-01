package mod

import (
	"context"
	"fmt"
	"strings"
)

type TestCTFSpawns struct{}

func (r TestCTFSpawns) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	link := "https://raw.githubusercontent.com/R2Northstar/NorthstarMods/maybe-better-ctf-spawns-pr/Northstar.CustomServers/mod/scripts/vscripts/gamemodes/_gamemode_ctf.nut"
	f := strings.Split(link, "/")
	file_path := "/" + f[len(f)-1]
	wget := fmt.Sprintf("wget %s -O %s \n", link, file_path)
	dockerArgs := fmt.Sprintf("--mount \"type=bind,source=%s,target=%s,readonly\"", file_path, "/usr/lib/northstar/R2Northstar/mods/Northstar.CustomServers/mod/scripts/vscripts/gamemodes/_gamemode_ctf.nut")
	return wget, dockerArgs, link, "latest", false, nil
}

func (r TestCTFSpawns) EnabledByDefault() bool {
	return false
}
