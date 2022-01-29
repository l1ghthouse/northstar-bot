package discord

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

type Notifyer struct {
	discordClient *discordgo.Session
	config        Config
}

func (d *Notifyer) Notify(message string) {
	if d.config.BotReportChannel != "" {
		_, err := d.discordClient.ChannelMessageSend(d.config.BotReportChannel, message)
		if err != nil {
			log.Println("error sending message to the bot Report Channel: ", err)
		}
	}
}
