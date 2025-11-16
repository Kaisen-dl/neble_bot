package handlers

import (
	"log"
	"neble_2/config"

	"github.com/bwmarrin/discordgo"
)

var roleMessageID string

func CreateRoleSelectionMessage(s *discordgo.Session, cfg *config.Config) {
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Пидарас",
					Style:    discordgo.PrimaryButton,
					CustomID: "select_role_1",
				},
				discordgo.Button{
					Label:    "Хуесос",
					Style:    discordgo.SuccessButton,
					CustomID: "select_role_2",
				},
				discordgo.Button{
					Label:    "Убрать роль",
					Style:    discordgo.DangerButton,
					CustomID: "remove_role",
				},
			},
		},
	}

	msg, err := s.ChannelMessageSendComplex(cfg.RoleChannelID, &discordgo.MessageSend{
		Content:    "Выберите роль:",
		Components: components,
	})

	if err != nil {
		log.Printf("Error creating role selection message: %v", err)
		return
	}

	roleMessageID = msg.ID
	log.Printf("Role selection message created with ID: %s", roleMessageID)
}

func CleanupRoleMessage(s *discordgo.Session, cfg *config.Config) {
	if roleMessageID != "" {
		err := s.ChannelMessageDelete(cfg.RoleChannelID, roleMessageID)
		if err != nil {
			log.Printf("Error deleting role message: %v", err)
		} else {
			log.Printf("Role message %s deleted successfully", roleMessageID)
		}
	}
}

func Ready(s *discordgo.Session, r *discordgo.Ready) {
	err := s.UpdateGameStatus(0, "Управление ролями")
	if err != nil {
		log.Printf("Error updating game status: %v", err)
	}
}
