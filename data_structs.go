package main

import (
	"github.com/go-telegram/bot"
	"github.com/jackc/pgx/v5"
)

type TextEntity struct {
	Type string `json:"type"`
	Text string `json:"text"`
	HREF string `json:"href"`
}

// id is database id, sourcefolder is the id from the source_folders table
type Message struct {
	ID              int
	TelegramID      int          `json:"id"`
	Type            string       `json:"type"`
	DateUnixtime    int64        `json:"date_unixtime,string"`
	From            string       `json:"from"`
	FromID          string       `json:"from_id"`
	ForwardedFrom   string       `json:"forwarded_from"`
	ForwardedFromID string       `json:"forwarded_from_id"`
	Photo           string       `json:"photo"`
	File            string       `json:"file"`
	MediaType       string       `json:"media_type"`
	MimeType        string       `json:"mime_type"`
	TextEntities    []TextEntity `json:"text_entities"`
	SourceFolderID  int
	SourceFolder    string
}

type Export struct {
	Type     string    `json:"type"`
	ID       int       `json:"id"`
	Messages []Message `json:"messages"`
}

type Controller struct {
	conn             *pgx.Conn
	vars             Vars
	bot              *bot.Bot
	groupPrevMessage *Message
	rootID           int
}

type MessageResolution int

// MessageUnprocessed is 0; this is used in up.sql
// i should be using a migration tool, oh bother
const (
	MessageUnprocessed MessageResolution = iota
	MessageDiscarded
	MessageSaved
	MessagePostponed
)
