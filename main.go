package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	tele "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	openai "github.com/sashabaranov/go-openai"
)

var (
	chatHistories    = make(map[int64][]string) // Хранилище истории сообщений
	chatContexts     = make(map[int64]string)   // Хранилище контекста чатов
	chatTemperatures = make(map[int64]float64)  // Хранилище температур для каждого чата
	chatMaxTokens    = make(map[int64]int)      // Хранилище максимального количества токенов
)

func main() {
	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	openaiToken := os.Getenv("OPENAI_API_KEY")
	botUsername := os.Getenv("BOT_USERNAME")

	if telegramToken == "" || openaiToken == "" || botUsername == "" {
		log.Fatal("Необходимо задать TELEGRAM_TOKEN, OPENAI_API_KEY и BOT_USERNAME в переменных окружения")
	}

	bot, err := tele.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatal(err)
	}
	bot.Debug = false

	log.Println("Бот запущен")
	updateConfig := tele.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text
		username := update.Message.From.UserName

		if text == "/clear" {
			clearHistory(chatID, bot, update.Message.Chat.ID)
			continue
		}

		if strings.HasPrefix(text, "/setcontext ") {
			setContext(chatID, bot, text[12:])
			continue
		}

		if strings.HasPrefix(text, "/settemp ") {
			setTemperature(chatID, bot, text[9:])
			continue
		}

		if strings.HasPrefix(text, "/setmaxtokens ") {
			setMaxTokens(chatID, bot, text[13:])
			continue
		}

		if strings.Contains(strings.ToLower(text), "@"+strings.ToLower(botUsername)) {
			reply := processMention(chatID, text, openaiToken)
			bot.Send(tele.NewMessage(chatID, fmt.Sprintf("@%s, %s", username, reply)))
		} else {
			storeMessage(chatID, fmt.Sprintf("%s: %s", username, text))
		}
	}
}

func processMention(chatID int64, text, openaiToken string) string {
	cHistory := chatHistories[chatID]
	cContext := chatContexts[chatID]
	cTemperature := chatTemperatures[chatID]
	cMaxTokens := chatMaxTokens[chatID]

	if cTemperature == 0 {
		cTemperature = 0.7 // Значение по умолчанию
	}
	if cMaxTokens == 0 {
		cMaxTokens = 150 // Значение по умолчанию
	}

	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: cContext},
	}

	for _, msg := range cHistory {
		messages = append(messages, openai.ChatCompletionMessage{Role: "user", Content: msg})
	}
	messages = append(messages, openai.ChatCompletionMessage{Role: "user", Content: text})

	client := openai.NewClient(openaiToken)
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:       openai.GPT3Dot5Turbo,
		Messages:    messages,
		MaxTokens:   cMaxTokens,
		Temperature: float32(cTemperature),
	})
	if err != nil {
		log.Println("Ошибка OpenAI API:", err)
		return "Произошла ошибка при обработке запроса."
	}

	reply := resp.Choices[0].Message.Content
	storeMessage(chatID, "Bot: "+reply)
	return reply
}

func storeMessage(chatID int64, message string) {
	chatHistories[chatID] = append(chatHistories[chatID], message)
	if len(chatHistories[chatID]) > 10 {
		chatHistories[chatID] = chatHistories[chatID][len(chatHistories[chatID])-10:]
	}
}

func clearHistory(chatID int64, bot *tele.BotAPI, chatIDMsg int64) {
	chatHistories[chatID] = []string{}
	bot.Send(tele.NewMessage(chatIDMsg, "История чата очищена."))
}

func setContext(chatID int64, bot *tele.BotAPI, context string) {
	chatContexts[chatID] = context
	bot.Send(tele.NewMessage(chatID, "Контекст успешно обновлен."))
}

func setTemperature(chatID int64, bot *tele.BotAPI, temp string) {
	var temperature float64
	_, err := fmt.Sscanf(temp, "%f", &temperature)
	if err != nil || temperature < 0 || temperature > 2 {
		bot.Send(tele.NewMessage(chatID, "Ошибка: Укажите температуру в диапазоне от 0 до 2."))
		return
	}
	chatTemperatures[chatID] = temperature
	bot.Send(tele.NewMessage(chatID, fmt.Sprintf("Температура успешно обновлена: %.2f", temperature)))
}

func setMaxTokens(chatID int64, bot *tele.BotAPI, tokens string) {
	var maxTokens int
	_, err := fmt.Sscanf(tokens, "%d", &maxTokens)
	if err != nil || maxTokens <= 0 {
		bot.Send(tele.NewMessage(chatID, "Ошибка: Укажите положительное число токенов."))
		return
	}
	chatMaxTokens[chatID] = maxTokens
	bot.Send(tele.NewMessage(chatID, fmt.Sprintf("Максимальное количество токенов успешно обновлено: %d", maxTokens)))
}
