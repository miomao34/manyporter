package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/log"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5"
)

func main() {
	log.SetLevel(log.DebugLevel)
	// urlExample := "postgres://username:password@localhost:5432/database_name"
	vars := InitVars()
	dbURL := fmt.Sprintf("postgres://%v:%v@%v:%v/%v", vars.username, vars.password, vars.hostname, vars.port, vars.database)

	dbCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := pgx.Connect(dbCtx, dbURL)
	if err != nil {
		log.Error("failed to open connection to postgres")
		os.Exit(1)
	}
	defer conn.Close(dbCtx)

	controller := Controller{conn: conn, vars: vars}

	err = controller.ApplyMigrations()
	if err != nil {
		log.Error("failed to apply migrations", "err", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithMessageTextHandler("import", bot.MatchTypeCommandStartOnly, getImportHandler(&controller)),
		bot.WithMessageTextHandler("next", bot.MatchTypeCommandStartOnly, getNextHandler(&controller)),

		bot.WithCallbackQueryDataHandler("button_", bot.MatchTypePrefix, getCallbackHandler(&controller)),

		bot.WithDefaultHandler(getHandler(&controller)),
	}

	controller.bot, err = bot.New(controller.vars.tgApiKey, opts...)
	if err != nil {
		panic(err)
	}

	controller.SendNext(ctx)

	controller.bot.Start(ctx)
}

func getHandler(c *Controller) func(context.Context, *bot.Bot, *models.Update) {
	handler := func(ctx context.Context, b *bot.Bot, update *models.Update) {

	}

	return handler
}

func getImportHandler(c *Controller) func(context.Context, *bot.Bot, *models.Update) {
	handler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		lenOfImportCommand := 7
		argument := strings.TrimSpace(update.Message.Text[lenOfImportCommand:])

		log.Info("trying to import", "argument", argument)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: c.vars.tgUserID,
			Text:   fmt.Sprintf("trying to import \"%v\"", argument),
		})

		err := c.Import(argument)
		if err != nil {
			log.Error("failed to import", "argument", argument, "err", err)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: c.vars.tgUserID,
				Text:   fmt.Sprintf("failed to import \"%v\" - %v", argument, err),
			})
		} else {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: c.vars.tgUserID,
				Text:   "import success!",
			})

		}
	}
	return handler
}

func getNextHandler(c *Controller) func(context.Context, *bot.Bot, *models.Update) {
	handler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		c.SendNext(context.Background())
	}
	return handler
}

func getCallbackHandler(c *Controller) func(context.Context, *bot.Bot, *models.Update) {
	handler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			ShowAlert:       false,
		})

		// dirty hack but that's better than regexing for example
		// len of button_xxxx_ is 12
		lenOfCallbackDataPrefix := 12
		databaseID, err := strconv.Atoi(update.CallbackQuery.Data[lenOfCallbackDataPrefix:])
		log.Info(databaseID)
		if err != nil {
			log.Error("mangled callback message", "CallbackQuery.Data", update.CallbackQuery.Data)
			return
		}
		switch update.CallbackQuery.Data[7:11] {
		case "disc":
			c.MarkMessageByID(databaseID, MessageDiscarded)
		case "save":
			c.MarkMessageByID(databaseID, MessageSaved)
		case "post":
			c.MarkMessageByID(databaseID, MessagePostponed)
		default:
			log.Error("mangled callback message", "CallbackQuery.Data", update.CallbackQuery.Data)
		}
		c.SendNext(ctx)

	}

	return handler
}

func (c *Controller) SendNext(ctx context.Context) {
	m, err := c.GetOneUnresolvedMessage()
	if err != nil {
		log.Error("failed to get the next unresolved message", "err", err)
		c.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: c.vars.tgUserID,
			Text:   "failed to get the next unresolved message; err: " + err.Error(),
		})
		return
	}

	// constructing and sending a message with metainfo
	var metaText string
	if m.Photo != "" {
		metaText = m.Photo
	}
	if m.File != "" {
		metaText = m.File
	}
	time := time.Unix(m.DateUnixtime, 0)
	c.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: c.vars.tgUserID,
		Text:   fmt.Sprintf("no. %v: %v\n%v", m.ID, time, metaText),
	})

	// keyboard markup
	StringMessageID := strconv.Itoa(m.ID)
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Save", CallbackData: "button_save_" + StringMessageID},
				{Text: "Postpone", CallbackData: "button_post_" + StringMessageID},
			}, {
				{Text: "Discard", CallbackData: "button_disc_" + StringMessageID},
			},
		},
	}

	// reconstructing message
	messageEntities := GenerateMessageEntities(m.TextEntities)
	var builder strings.Builder
	for _, entity := range m.TextEntities {
		builder.WriteString(entity.Text)
	}
	text := builder.String()
	log.Debug("resulting text: " + text)

	if m.Photo != "" {
		fileData, errReadFile := os.ReadFile(m.SourceFolder + string(os.PathSeparator) + m.Photo)
		if errReadFile != nil {
			fmt.Printf("error reading file, %v\n", errReadFile)
			return
		}
		params := &bot.SendPhotoParams{
			ChatID:          c.vars.tgUserID,
			Photo:           &models.InputFileUpload{Filename: m.Photo, Data: bytes.NewReader(fileData)},
			Caption:         text,
			CaptionEntities: messageEntities,
			ReplyMarkup:     kb,
		}
		_, err := c.bot.SendPhoto(ctx, params)
		if err != nil {

		}
		return
	}

	if m.File != "" {
		fileData, errReadFile := os.ReadFile(m.SourceFolder + string(os.PathSeparator) + m.File)
		if errReadFile != nil {
			fmt.Printf("error reading file, %v\n", errReadFile)
			return
		}
		params := &bot.SendDocumentParams{
			ChatID:          c.vars.tgUserID,
			Document:        &models.InputFileUpload{Filename: m.File, Data: bytes.NewReader(fileData)},
			Caption:         m.File,
			CaptionEntities: messageEntities,
			ReplyMarkup:     kb,
		}
		c.bot.SendDocument(ctx, params)
		return
	}

	_, err = c.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      c.vars.tgUserID,
		Text:        text,
		Entities:    messageEntities,
		ReplyMarkup: kb,
	})
	log.Error("send message", "err", err)
}

func GenerateMessageEntities(textEntities []TextEntity) []models.MessageEntity {
	result := make([]models.MessageEntity, 0)
	offset := 0

	for _, entity := range textEntities {
		addMessageEntity := true
		length := utf8.RuneCountInString(entity.Text)

		switch entity.Type {
		case "plain":
			// do nothing
			addMessageEntity = false
		case "link":
			entity.Type = "url"
		}

		if addMessageEntity {
			messageEntity := models.MessageEntity{
				Type:   models.MessageEntityType(entity.Type),
				Offset: offset,
				Length: length,
				URL:    entity.HREF,
			}
			result = append(result, messageEntity)
		}

		offset += length
	}

	return result
}
