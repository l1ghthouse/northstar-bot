Build: `go build cmd/bot/main.go`

Configuration:
Rename `default_config.yml` to `config.cfg`
Fill in the values inside `config.cfg`

Run: `./main`


Application requires following:
Permissions:
    Read/Write in a given channel
Scope: 
    `applications.commands`
    
    
Invite example:
https://discord.com/api/oauth2/authorize?client_id=<CLIENT_BOT_ID>&permissions=3072&scope=bot%20applications.commands
