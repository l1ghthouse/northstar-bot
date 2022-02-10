package discord

import (
	"errors"
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

func sendInteractionReply(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Users: []string{},
			},
		},
	}); err != nil {
		log.Println("Error sending message: ", err)
	}
}

var ErrInvalidRole = errors.New("missing required role to use this command")

func (h *handler) handleAuthUser(member *discordgo.Member) error {
	if !isAdministrator(member.Permissions) {
		if h.dedicatedRoleID != "" && !hasRole(member.Roles, h.dedicatedRoleID) {
			return ErrInvalidRole
		}
	}
	return nil
}
