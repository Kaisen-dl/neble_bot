package main

import (
	"fmt"
	"log"
	"neble_2/config"
	"neble_2/database"
	"neble_2/handlers"
	"neble_2/scheduler"
	"neble_2/stats"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Загружаем .env файл
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	cfg := config.Load()

	// Используем ваш формат строки подключения
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "123")
	dbName := getEnv("DB_NAME", "neble_2")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Создание сессии Discord ПЕРВЫМ
	discord, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		log.Fatal("Error creating Discord session:", err)
	}

	// Создаем StatsManager ВТОРЫМ (нужен discord session)
	statsManager := stats.NewStatsManager(discord, nil, cfg.GuildID, cfg.StatsChannelID) // временно nil для БД

	// Инициализация БД ТРЕТЬИМ (передаем statsUpdater)
	db, err := database.New(connStr, statsManager.NotifyUpdate)
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}
	defer db.Close()

	// Обновляем StatsManager с реальной БД
	statsManager.SetDB(db)

	// Добавление обработчиков
	discord.AddHandler(handlers.Ready)
	discord.AddHandler(handlers.InteractionCreate(db, cfg))

	// Открытие соединения
	err = discord.Open()
	if err != nil {
		log.Fatal("Error opening connection:", err)
	}
	defer discord.Close()

	// Создание сообщения с кнопками для выбора ролей
	handlers.CreateRoleSelectionMessage(discord, cfg)
	defer handlers.CleanupRoleMessage(discord, cfg)
	defer statsManager.CleanupStatsMessage()

	// Запуск планировщика для проверки expired ролей
	scheduler.StartScheduler(discord, db, cfg)
	log.Printf("Scheduler started with check interval: 1 hour")

	// Первоначальное создание сообщения со статистикой
	statsManager.NotifyUpdate()

	log.Println("Bot is now running. Press CTRL-C to exit.")

	// Ожидание сигнала завершения
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Shutting down bot...")
}
