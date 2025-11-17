package handlers

import (
	"fmt"
	"log"
	"neble_2/config"
	"neble_2/database"
	"neble_2/scheduler"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const ChangeRoleDuration = 1 * time.Minute

func InteractionCreate(db *database.DB, cfg *config.Config) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionMessageComponent:
			data := i.MessageComponentData()

			if strings.HasPrefix(data.CustomID, "select_role_") {
				handleRoleSelection(s, i, db, cfg, data.CustomID)
			} else if strings.HasPrefix(data.CustomID, "renew_") {
				handleRenewalResponse(s, i, db, cfg, data.CustomID)
			} else if data.CustomID == "change_role" {
				handleRenewalResponse(s, i, db, cfg, data.CustomID)
			} else if data.CustomID == "remove_role" { // ДОБАВЛЯЕМ
				handleRemoveRole(s, i, db, cfg)
			}
		}
	}
}

func handleRemoveRole(s *discordgo.Session, i *discordgo.InteractionCreate, db *database.DB, cfg *config.Config) {
	// Получаем текущую активную роль пользователя
	currentRole, err := db.GetActiveRoleByUserID(i.Member.User.ID)
	if err != nil || currentRole == nil {
		respond(s, i, "У вас нет активной роли для удаления.")
		return
	}

	// Удаляем роль из Discord
	err = s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, currentRole.RoleID)
	if err != nil {
		log.Printf("Error removing role: %v", err)
		respond(s, i, "Ошибка при удалении роли")
		return
	}

	// Деактивируем роль в БД
	err = db.DeactivateRole(currentRole.ID)
	if err != nil {
		log.Printf("Error deactivating role in DB: %v", err)
		respond(s, i, "Ошибка при обновлении данных")
		return
	}

	respond(s, i, fmt.Sprintf("Роль **%s** успешно удалена!", currentRole.RoleName))
}

func handleRoleSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *database.DB, cfg *config.Config, customID string) {
	// ПРОВЕРЯЕМ ЕСТЬ ЛИ УЖЕ АКТИВНАЯ РОЛЬ
	existingRole, err := db.GetUserRole(i.Member.User.ID)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		log.Printf("Error checking existing role: %v", err)
		respond(s, i, "Ошибка при проверке ролей")
		return
	}

	if existingRole != nil && existingRole.IsActive {
		respond(s, i, fmt.Sprintf("У вас уже есть активная роль **%s**. Сначала отмените её.", existingRole.RoleName))
		return
	}

	// Маппинг кнопок на роли
	roles := map[string]struct {
		ID   string
		Name string
	}{
		"select_role_1": {ID: "1439750973861007442", Name: "Сенди-Шорс"},
		"select_role_2": {ID: "1439751278925316116", Name: "ХПалето-Бэй"},
	}

	role, exists := roles[customID]
	if !exists {
		respond(s, i, "Неизвестная роль")
		return
	}

	expiresAt := time.Now().Add(cfg.RoleDuration)

	// Добавляем роль пользователю в Discord
	err = s.GuildMemberRoleAdd(cfg.GuildID, i.Member.User.ID, role.ID)
	if err != nil {
		log.Printf("Error adding role: %v", err)
		respond(s, i, "Ошибка при выдаче роли")
		return
	}

	// ЕСЛИ УЖЕ ЕСТЬ ЗАПИСЬ - ОБНОВЛЯЕМ, ЕСЛИ НЕТ - СОЗДАЕМ
	if existingRole != nil {
		// ОБНОВЛЯЕМ СУЩЕСТВУЮЩУЮ ЗАПИСЬ
		err = db.UpdateUserRole(i.Member.User.ID, role.ID, role.Name, expiresAt)
		if err != nil {
			log.Printf("Error updating role in DB: %v", err)
			s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, role.ID)
			respond(s, i, "Ошибка при обновлении данных")
			return
		}
	} else {
		// СОЗДАЕМ НОВУЮ ЗАПИСЬ
		err = db.AddUserRole(i.Member.User.ID, i.Member.User.Username, role.ID, role.Name, expiresAt)
		if err != nil {
			log.Printf("Error saving to DB: %v", err)
			s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, role.ID)
			respond(s, i, "Ошибка при сохранении данных")
			return
		}
	}

	// sendChangeConfirmation(s, i, db, cfg, role.Name)

	respond(s, i, fmt.Sprintf("Роль **%s** успешно выдана!", role.Name))
}

// func sendChangeConfirmation(s *discordgo.Session, i *discordgo.InteractionCreate, db *database.DB, cfg *config.Config, roleName string) {
// 	components := []discordgo.MessageComponent{
// 		discordgo.ActionsRow{
// 			Components: []discordgo.MessageComponent{
// 				discordgo.Button{
// 					Label:    "Изменить выбор",
// 					Style:    discordgo.SecondaryButton,
// 					CustomID: "change_role",
// 				},
// 			},
// 		},
// 	}

// 	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
// 		Type: discordgo.InteractionResponseChannelMessageWithSource,
// 		Data: &discordgo.InteractionResponseData{
// 			Content:    fmt.Sprintf("Теперь ваша роль **%s**. Вы можете изменить выбор в течение 1 минуты.", roleName),
// 			Components: components,
// 			Flags:      discordgo.MessageFlagsEphemeral, // ДОБАВЛЯЕМ ФЛАГ
// 		},
// 	})

// 	if err != nil {
// 		log.Printf("Error sending change confirmation: %v", err)
// 		return
// 	}

// 	// ЗАПУСКАЕМ ТАЙМЕР ДЛЯ УДАЛЕНИЯ КНОПКИ ИЗМЕНЕНИЯ
// 	go startChangeTimer(s, i.ChannelID, i.Member.User.ID)
// }

func handleRenewalResponse(s *discordgo.Session, i *discordgo.InteractionCreate, db *database.DB, cfg *config.Config, customID string) {
	log.Printf("Processing customID: %s", customID)

	// if customID == "change_role" {
	// 	handleChangeRole(s, i, db, cfg)
	// 	return
	// }

	// Парсим customID чтобы извлечь действие и ID записи
	var action string
	var roleID int

	// Ожидаемый формат: "renew_yes_123" или "renew_no_123"
	parts := strings.Split(customID, "_")
	if len(parts) < 3 {
		log.Printf("Invalid customID format: %s", customID)
		respond(s, i, "Ошибка обработки запроса: неверный формат")
		return
	}

	action = parts[1] // "yes" или "no"

	// Берем последнюю часть как ID (может содержать дополнительные символы)
	idStr := parts[2]
	_, err := fmt.Sscanf(idStr, "%d", &roleID)
	if err != nil {
		log.Printf("Error parsing role ID from %s: %v", customID, err)
		respond(s, i, "Ошибка обработки запроса: неверный ID роли")
		return
	}

	// Получаем запись из базы данных
	role, err := db.GetRoleByID(roleID)
	if err != nil {
		log.Printf("Error getting role %d: %v", roleID, err)
		respond(s, i, "Ошибка: запись не найдена")
		return
	}

	// Проверяем, принадлежит ли роль пользователю, который нажал кнопку
	if i.Member.User.ID != role.UserID {
		respond(s, i, "Это действие вам недоступно!")
		return
	}

	switch action {
	case "yes":
		handleRenewalYes(s, i, db, cfg, role)
	case "no":
		handleRenewalNo(s, i, db, cfg, role)
	default:
		respond(s, i, "Неизвестное действие")
	}
}

// ДОБАВЛЯЕМ ФУНКЦИЮ ОБРАБОТКИ ИЗМЕНЕНИЯ РОЛИ
// func handleChangeRole(s *discordgo.Session, i *discordgo.InteractionCreate, db *database.DB, cfg *config.Config) {
// 	// ПОЛУЧАЕМ ID ТЕКУЩЕЙ РОЛИ ИЗ БД
// 	currentRoleID, err := db.GetActiveRoleIDByUserID(i.Member.User.ID)
// 	if err != nil {
// 		log.Printf("Error getting current role ID: %v", err)
// 		respond(s, i, "Ошибка: не найдена активная роль для изменения")
// 		return
// 	}

// 	// УДАЛЯЕМ АКТИВНУЮ РОЛЬ ИЗ БД
// 	err = db.RemoveUserRole(i.Member.User.ID)
// 	if err != nil {
// 		log.Printf("Error removing user role from DB: %v", err)
// 		respond(s, i, "Ошибка при изменении роли")
// 		return
// 	}

// 	// УДАЛЯЕМ РОЛЬ ИЗ DISCORD
// 	err = s.GuildMemberRoleRemove(i.GuildID, i.Member.User.ID, currentRoleID)
// 	if err != nil {
// 		log.Printf("Error removing role from user: %v", err)
// 		// Продолжаем, даже если ошибка удаления роли в Discord
// 	}

// 	respond(s, i, "Роль успешно удалена! Теперь вы можете выбрать новую роль.")

// 	// УДАЛЯЕМ СООБЩЕНИЕ С КНОПКОЙ ИЗМЕНЕНИЯ
// 	removeButtonsFromMessage(s, i.ChannelID, i.Message.ID)
// }

// // ДОБАВЛЯЕМ ТАЙМЕР ДЛЯ УДАЛЕНИЯ КНОПКИ ИЗМЕНЕНИЯ
// func startChangeTimer(s *discordgo.Session, channelID, userID string) {
// 	time.Sleep(ChangeRoleDuration)

// 	// Здесь нужно найти и удалить сообщение с кнопкой изменения
// 	// Пока просто логируем
// 	log.Printf("Change period expired for user %s in channel %s", userID, channelID)
// }

func handleRenewalYes(s *discordgo.Session, i *discordgo.InteractionCreate, db *database.DB, cfg *config.Config, role *database.UserRole) {
	// Продлеваем роль - добавляем еще одну неделю
	newExpiresAt := time.Now().Add(cfg.RoleDuration)

	// Обновляем дату окончания в БД
	err := db.ExtendRole(role.ID, newExpiresAt)
	if err != nil {
		log.Printf("Error extending role %d: %v", role.ID, err)
		respond(s, i, "Ошибка при продлении роли")
		return
	}

	// Убеждаемся, что роль все еще выдана пользователю
	err = s.GuildMemberRoleAdd(cfg.GuildID, role.UserID, role.RoleID)
	if err != nil {
		log.Printf("Error re-adding role: %v", err)
		// Продолжаем, так как роль могла быть уже выдана
	}

	// Отправляем подтверждение
	respond(s, i, fmt.Sprintf("Роль **%s** успешно продлена до %s!",
		role.RoleName, newExpiresAt.Format("02.01.2006 15:04")))

	// Удаляем кнопки из оригинального сообщения
	removeButtonsFromMessage(s, i.ChannelID, i.Message.ID)

	// УДАЛЯЕМ СООБЩЕНИЕ О ПРОДЛЕНИИ
	scheduler.DeleteRenewalMessage(s, cfg, role.ID, db)

	// Отправляем приватное подтверждение
	respond(s, i, fmt.Sprintf("Роль **%s** успешно продлена до %s!",
		role.RoleName, newExpiresAt.Format("02.01.2006 15:04")))
}

func handleRenewalNo(s *discordgo.Session, i *discordgo.InteractionCreate, db *database.DB, cfg *config.Config, role *database.UserRole) {
	// Убираем роль у пользователя
	err := s.GuildMemberRoleRemove(cfg.GuildID, role.UserID, role.RoleID)
	if err != nil {
		log.Printf("Error removing role: %v", err)
		// Продолжаем, чтобы обновить статус в БД
	}

	// Обновляем статус в БД
	err = db.DeactivateRole(role.ID)
	if err != nil {
		log.Printf("Error deactivating role %d: %v", role.ID, err)
		respond(s, i, "Ошибка при удалении роли")
		return
	}

	respond(s, i, fmt.Sprintf("Роль **%s** была успешно удалена.", role.RoleName))

	// Удаляем кнопки из оригинального сообщения
	removeButtonsFromMessage(s, i.ChannelID, i.Message.ID)

	scheduler.DeleteRenewalMessage(s, cfg, role.ID, db)

	respond(s, i, fmt.Sprintf("Роль **%s** была успешно удалена.", role.RoleName))
}

func removeButtonsFromMessage(s *discordgo.Session, channelID, messageID string) {
	// Создаем пустой слайс компонентов и передаем его указатель
	emptyComponents := []discordgo.MessageComponent{}

	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    channelID,
		ID:         messageID,
		Components: &emptyComponents, // Передаем указатель на пустой массив
	})
	if err != nil {
		log.Printf("Error removing buttons: %v", err)
	}
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("Error responding to interaction: %v", err)
	}
}
