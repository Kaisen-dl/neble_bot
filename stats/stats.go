package stats

import (
	"fmt"
	"log"
	"neble_2/database"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type StatsManager struct {
	session    *discordgo.Session
	db         *database.DB
	guildID    string
	channelID  string
	messageID  string
	mutex      sync.Mutex
	lastUpdate time.Time
}

func NewStatsManager(s *discordgo.Session, db *database.DB, guildID, channelID string) *StatsManager {
	return &StatsManager{
		session:   s,
		db:        db,
		guildID:   guildID, // ДОБАВЛЯЕМ
		channelID: channelID,
	}
}

// NotifyUpdate - вызывается при любом изменении ролей
func (sm *StatsManager) NotifyUpdate() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// // Защита от слишком частых обновлений (не чаще чем раз в 5 секунд)
	// if time.Since(sm.lastUpdate) < 5*time.Second {
	// 	return
	// }

	sm.lastUpdate = time.Now()

	// Запускаем обновление в горутине чтобы не блокировать основной поток
	go sm.updateStats()
}

func (sm *StatsManager) updateStats() {
	activeRoles, err := sm.getActiveRoles()
	if err != nil {
		log.Printf("Error getting active roles for stats: %v", err)
		return
	}

	content := sm.formatStatsMessage(activeRoles)

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.messageID == "" {
		// Первый запуск - ищем существующее сообщение или создаем новое
		messageID, err := sm.findLastStatsMessage()
		if err == nil && messageID != "" {
			sm.messageID = messageID
		}
	}

	if sm.messageID != "" {
		// Обновляем существующее сообщение
		_, err := sm.session.ChannelMessageEdit(sm.channelID, sm.messageID, content)
		if err != nil {
			log.Printf("Error updating stats message: %v", err)
			sm.messageID = "" // Сброс ID, создадим новое сообщение
		}
	}

	if sm.messageID == "" {
		// Создаем новое сообщение
		msg, err := sm.session.ChannelMessageSend(sm.channelID, content)
		if err != nil {
			log.Printf("Error sending stats message: %v", err)
			return
		}
		sm.messageID = msg.ID
	}
}

func (sm *StatsManager) getActiveRoles() ([]database.UserRole, error) {
	query := `SELECT user_id, user_name, role_name, expires_at 
              FROM user_roles 
              WHERE is_active = true 
              ORDER BY role_name, user_name`

	rows, err := sm.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []database.UserRole
	for rows.Next() {
		var role database.UserRole
		err := rows.Scan(&role.UserID, &role.UserName, &role.RoleName, &role.ExpiresAt)
		if err != nil {
			return nil, err
		}

		// ПОЛУЧАЕМ АКТУАЛЬНЫЙ СЕРВЕРНЫЙ НИК
		member, err := sm.session.GuildMember(sm.guildID, role.UserID) // Замени на cfg.GuildID
		if err == nil && member.Nick != "" {
			role.UserName = member.Nick // Обновляем на серверный ник
		}
		// Если серверного ника нет, остаётся глобальное имя

		roles = append(roles, role)
	}

	return roles, nil
}

func (sm *StatsManager) formatStatsMessage(roles []database.UserRole) string {
	if len(roles) == 0 {
		return "**📊 Активные роли:**\nНет активных ролей"
	}

	var sb strings.Builder
	sb.WriteString("**📊 Активные роли:**\n```\n")

	for _, role := range roles {
		sb.WriteString(fmt.Sprintf("%s - %s\n", role.UserName, role.RoleName))
	}

	sb.WriteString("```")
	return sb.String()
}

func (sm *StatsManager) findLastStatsMessage() (string, error) {
	messages, err := sm.session.ChannelMessages(sm.channelID, 10, "", "", "")
	if err != nil {
		return "", err
	}

	for _, msg := range messages {
		if msg.Author.ID == sm.session.State.User.ID && strings.Contains(msg.Content, "Активные роли") {
			return msg.ID, nil
		}
	}

	return "", nil
}

func (sm *StatsManager) SetDB(db *database.DB) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.db = db
}

func (sm *StatsManager) CleanupStatsMessage() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.messageID != "" {
		err := sm.session.ChannelMessageDelete(sm.channelID, sm.messageID)
		if err != nil {
			log.Printf("Error deleting stats message: %v", err)
		} else {
			log.Printf("Stats message %s deleted successfully", sm.messageID)
		}
	}
}
