package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func sendMessageWithFilesDM(session *discordgo.Session, userChannelID string, msg string, file []*discordgo.File) {
	directMessageChannel, err := session.UserChannelCreate(userChannelID)
	if err != nil {
		log.Println("Error creating DM channel: ", err)
		return
	}
	sendComplexMessage(session, directMessageChannel.ID, msg, file)
}

func sendComplexMessage(session *discordgo.Session, channelID string, msg string, file []*discordgo.File) {
	if _, err := session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: msg,
		Files:   file,
	}); err != nil {
		log.Println(fmt.Sprintf("failed to send message with attachment to channel id: %s. error: %v", channelID, err))
	}
}

func sendMessage(session *discordgo.Session, channelID string, msg string) {
	if _, err := session.ChannelMessageSend(channelID, msg); err != nil {
		log.Println(fmt.Sprintf("failed to send message to channel id: %s. error: %v", channelID, err))
	}
}

func editDeferredInteractionReply(session *discordgo.Session, interaction *discordgo.Interaction, msg string) {
	response := &discordgo.WebhookEdit{Content: msg}
	_, err := session.InteractionResponseEdit(interaction, response)
	if err != nil {
		log.Println(fmt.Sprintf("failed to update interaction. error: %v", err))
	}
}

func sendInteractionDeferred(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		log.Println("Error sending message: ", err)
	}
}
