package scheduler

import (
	"fmt"
	"log"
	"neble_2/config"
	"neble_2/database"
	"time"

	"github.com/bwmarrin/discordgo"
)

func StartScheduler(s *discordgo.Session, db *database.DB, cfg *config.Config) {
	ticker := time.NewTicker(1 * time.Second) // Проверяем каждый час

	go func() {
		for range ticker.C {
			checkExpiredRoles(s, db, cfg)
		}
	}()
}

func checkExpiredRoles(s *discordgo.Session, db *database.DB, cfg *config.Config) {
	log.Printf("Checking for expired roles...")
	expiredRoles, err := db.GetExpiredRoles()
	if err != nil {
		log.Printf("Error getting expired roles: %v", err)
		return
	}

	log.Printf("Found %d expired roles to process", len(expiredRoles))

	for _, role := range expiredRoles {
		// Отправляем сообщение с вопросом о продлении
		sendRenewalMessage(s, db, cfg, role)

		// Запускаем таймер для автоматического снятия роли
		go startRenewalTimer(s, db, cfg, role)
	}
}

func sendRenewalMessage(s *discordgo.Session, db *database.DB, cfg *config.Config, role database.UserRole) {
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Да, продлить",
					Style:    discordgo.SuccessButton,
					CustomID: fmt.Sprintf("renew_yes_%d", role.ID),
				},
				discordgo.Button{
					Label:    "Нет, убрать",
					Style:    discordgo.DangerButton,
					CustomID: fmt.Sprintf("renew_no_%d", role.ID),
				},
			},
		},
	}

	message := fmt.Sprintf("<@%s>, ты всё ещё **%s**? Ответь в течение 10 минут.", role.UserID, role.RoleName)

	msg, err := s.ChannelMessageSendComplex(cfg.NotificationChannelID, &discordgo.MessageSend{
		Content:    message,
		Components: components,
	})

	if err != nil {
		log.Printf("Error sending renewal message: %v", err)
		return
	}

	err = db.SetRenewalMessageID(role.ID, msg.ID)
	if err != nil {
		log.Printf("Error saving message ID: %v", err)
	}

	// Обновляем статус в БД на "waiting_response"
	err = db.UpdateRenewalStatus(role.ID, "waiting_response")
	if err != nil {
		log.Printf("Error updating renewal status: %v", err)
	}

	log.Printf("Successfully sent renewal message with ID: %s", msg.ID)
}

func DeleteRenewalMessage(s *discordgo.Session, cfg *config.Config, roleID int, db *database.DB) {
	messageID, err := db.GetRenewalMessageID(roleID)
	if err != nil || messageID == "" {
		log.Printf("No message ID found for role %d: %v", roleID, err)
		return
	}

	err = s.ChannelMessageDelete(cfg.NotificationChannelID, messageID)
	if err != nil {
		log.Printf("Error deleting renewal message: %v", err)
	} else {
		log.Printf("Successfully deleted renewal message %s", messageID)
	}
}

func startRenewalTimer(s *discordgo.Session, db *database.DB, cfg *config.Config, role database.UserRole) {
	time.Sleep(cfg.RenewalDuration)

	// Проверяем, ответил ли пользователь
	currentRole, err := db.GetRoleByID(role.ID)
	if err != nil {
		log.Printf("Error getting role %d: %v", role.ID, err)
		return
	}

	// Если статус все еще "waiting_response", значит пользователь не ответил
	if currentRole.RenewalStatus == "waiting_response" {

		DeleteRenewalMessage(s, cfg, role.ID, db)
		// Пользователь не ответил - снимаем роль
		err = s.GuildMemberRoleRemove(cfg.GuildID, role.UserID, role.RoleID)
		if err != nil {
			log.Printf("Error removing role from user %s: %v", role.UserID, err)
		}

		err = db.DeactivateRole(role.ID)
		if err != nil {
			log.Printf("Error deactivating role %d in DB: %v", role.ID, err)
		}

		log.Printf("Role %s automatically removed from user %s", role.RoleName, role.UserName)
	}
}
