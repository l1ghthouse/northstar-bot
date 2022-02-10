package discord

import (
	"io"

	"github.com/bwmarrin/discordgo"
)

type Notifyer struct {
	discordClient *discordgo.Session
	reportChannel string
}

func (d *Notifyer) Notify(message string) {
	sendMessage(d.discordClient, d.reportChannel, message)
}

func (d *Notifyer) NotifyAndAttach(message string, filename string, file io.Reader) {
	if file != nil {
		sendComplexMessage(d.discordClient, d.reportChannel, message, []*discordgo.File{{
			Name:        filename,
			ContentType: "application/octet-stream",
			Reader:      file,
		}})
	} else {
		d.Notify(message)
	}
}
