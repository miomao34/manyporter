package main

import (
	"context"
	"os"
	"path"

	"github.com/charmbracelet/log"
	"github.com/jackc/pgx/v5"
)

func (c *Controller) ApplyMigrations() error {
	migrationsFolder := "migrations"
	result, err := os.ReadDir(migrationsFolder)
	if err != nil {
		return err
	}

	for _, entry := range result {
		if !entry.IsDir() {
			continue
		}
		migrationFilePath := path.Join(migrationsFolder, entry.Name(), "up.sql")
		log.Info("applying migration", "migration", migrationFilePath)

		migrationContents, err := os.ReadFile(migrationFilePath)
		if err != nil {
			return err
		}

		_, err = c.conn.Exec(context.Background(), string(migrationContents))
		if err != nil {
			return err
		}
	}
	return nil
}

// the photos, files and text entities are all attached to messages.
// messages can have parent messages - this allows to implement telegram grouping
// not ideal, but eh
func (im *Importer) InsertMessage(m Message) (int, error) {
	tx, err := im.conn.Begin(context.Background())
	if err != nil {
		return 0, err
	}

	isMergeable := arePhotoMessagesMergeable(im.groupPrevMessage, &m)
	if !isMergeable {
		im.groupPrevMessage = &m
	}

	var result pgx.Row

	if !isMergeable {
		result = tx.QueryRow(
			context.Background(),
			`INSERT INTO messages (
					telegram_id,
					message_type,
					message_date,
					message_from,
					from_id,
					forwarded_from,
					forwarded_from_id,
					source_id
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				RETURNING id;`,
			m.ID,
			m.Type,
			m.DateUnixtime,
			m.From,
			m.FromID,
			m.ForwardedFrom,
			m.ForwardedFromID,
			m.SourceFolderID,
		)
	} else {
		result = tx.QueryRow(
			context.Background(),
			`INSERT INTO messages (
					telegram_id,
					message_type,
					message_date,
					message_from,
					from_id,
					forwarded_from,
					forwarded_from_id,
					source_id,
					parent_id
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				RETURNING id;`,
			m.ID,
			m.Type,
			m.DateUnixtime,
			m.From,
			m.FromID,
			m.ForwardedFrom,
			m.ForwardedFromID,
			m.SourceFolderID,
			im.rootID,
		)
	}
	var messageID int
	err = result.Scan(&messageID)
	if err != nil {
		log.Error("failed to insert message", "messageID", messageID, "err", err)
		return 0, err
	}

	if !isMergeable {
		im.rootID = messageID
	}

	if len(m.TextEntities) != 0 {
		// todo: make it a single insert with string builder
		for _, entry := range m.TextEntities {
			_, err := tx.Exec(
				context.Background(),
				`INSERT INTO message_texts (
					message_id, 
					text_type,
					text_text,
					href
				) VALUES ($1, $2, $3, $4) 
				RETURNING id;`,
				messageID,
				entry.Type,
				entry.Text,
				entry.HREF,
			)
			if err != nil {
				log.Error("failed to insert message text", "messageID", messageID, "err", err)
			}
		}
	}

	if m.Photo != "" {
		_, err = tx.Exec(
			context.Background(),
			`INSERT INTO photos (
				message_id, 
				photo
			) VALUES ($1, $2) RETURNING id;`,
			messageID,
			m.Photo,
		)
		if err != nil {
			log.Error("failed to insert photo", "messageID", messageID, "err", err)
		}
	}

	if m.File != "" {
		_, err = tx.Exec(
			context.Background(),
			`INSERT INTO files (
				message_id,
				file_file,
				media_type,
				mime_type
			) VALUES ($1, $2, $3, $4) RETURNING id;`,
			messageID,
			m.File,
			m.MediaType,
			m.MimeType,
		)
		if err != nil {
			log.Error("failed to insert file", "messageID", messageID, "err", err)
		}
	}

	if isMergeable {
		im.groupPrevMessage = &m
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return 0, err
	}

	return messageID, nil
}

func (c *Controller) InsertSourceFolder(folder string) (int, error) {
	// not doing returning here bc on conflict do nothing doesn't return a id

	tx, err := c.conn.Begin(context.Background())
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(context.Background(),
		`INSERT INTO source_folders (folder)
	VALUES ($1)
	ON CONFLICT DO NOTHING;`, folder)
	if err != nil {
		return 0, err
	}

	result := tx.QueryRow(context.Background(),
		"SELECT id FROM source_folders WHERE folder = $1;", folder)

	var id int
	err = result.Scan(&id)
	if err != nil {
		log.Error("failed to insert source folder", "err", err)
		return 0, err
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (c *Controller) GetUnresolvedMessagesGroup() ([]*Message, error) {
	m, err := c.GetOneUnresolvedMessage()
	if err != nil {
		return nil, err
	}

	tx, err := c.conn.Begin(context.Background())
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(context.Background(),
		`SELECT 
			messages.id,
			telegram_id,
			message_type,
			message_date,
			message_from,
			from_id,
			forwarded_from,
			forwarded_from_id,
			photos.photo,
			files.file_file,
			source_id,
			folder
		FROM messages 
		LEFT JOIN photos ON messages.id = photos.message_id
		LEFT JOIN files ON messages.id = files.message_id
		LEFT JOIN source_folders ON messages.source_id = source_folders.id
		WHERE resolution = $1 AND parent_id = $2;`,
		MessageUnprocessed,
		m.ID,
	)
	if err != nil {
		return nil, err
	}

	results := make([]*Message, 1)
	results = append(results, m)

	for rows.Next() {
		var resultMessage Message
		err := rows.Scan(
			&resultMessage.ID,
			&resultMessage.TelegramID,
			&resultMessage.Type,
			&resultMessage.DateUnixtime,
			&resultMessage.From,
			&resultMessage.FromID,
			&resultMessage.ForwardedFrom,
			&resultMessage.ForwardedFromID,
			&resultMessage.Photo,
			&resultMessage.File,
			&resultMessage.SourceFolderID,
			&resultMessage.SourceFolder,
		)
		if err != nil {
			return results, err
		}
		results = append(results, &resultMessage)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (c *Controller) GetOneUnresolvedMessage() (*Message, error) {
	tx, err := c.conn.Begin(context.Background())
	if err != nil {
		return nil, err
	}

	// getting the message
	result := tx.QueryRow(context.Background(),
		`SELECT 
			messages.id,
			telegram_id,
			message_type,
			message_date,
			message_from,
			from_id,
			forwarded_from,
			forwarded_from_id,
			COALESCE(photos.photo, ''),
			COALESCE(files.file_file, ''),
			source_id,
			folder
		FROM messages 
		LEFT JOIN photos ON messages.id = photos.message_id
		LEFT JOIN files ON messages.id = files.message_id
		LEFT JOIN source_folders ON messages.source_id = source_folders.id
		WHERE resolution = $1
		ORDER BY messages.id
		LIMIT 1;`,
		MessageUnprocessed,
	)
	var m Message
	err = result.Scan(
		&m.ID,
		&m.TelegramID,
		&m.Type,
		&m.DateUnixtime,
		&m.From,
		&m.FromID,
		&m.ForwardedFrom,
		&m.ForwardedFromID,
		&m.Photo,
		&m.File,
		&m.SourceFolderID,
		&m.SourceFolder,
	)
	if err != nil {
		return nil, err
	}

	textEntriesResult, err := tx.Query(context.Background(), `SELECT
		text_type,
		COALESCE(text_text, ''),
		COALESCE(href, '')
	FROM message_texts
	WHERE message_id = $1
	ORDER BY id;
	`, m.ID)
	if err != nil {
		return nil, err
	}

	m.TextEntities = make([]TextEntity, 0)
	for textEntriesResult.Next() {
		var te TextEntity
		err = textEntriesResult.Scan(&te.Type, &te.Text, &te.HREF)
		if err != nil {
			return nil, err
		}
		m.TextEntities = append(m.TextEntities, te)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return nil, err
	}

	return &m, nil
}

func (c *Controller) MarkMessageByID(id int, resolution MessageResolution) error {
	_, err := c.conn.Exec(context.Background(), "UPDATE messages SET resolution = $1 WHERE id = $2;", resolution, id)
	if err != nil {
		log.Error("failed to mark message by id", "err", err)
	}

	return err
}

func (c *Controller) GetMessageEntities(id int) ([]TextEntity, error) {
	result, err := c.conn.Query(context.Background(), `SELECT
		text_type,
		text_text,
		COALESCE(href, '')
	FROM message_texts
	WHERE message_id = $1
	ORDER BY id;
	`, id)
	if err != nil {
		return nil, err
	}

	textEntities := make([]TextEntity, 0)
	for result.Next() {
		var te TextEntity
		err = result.Scan(&te.Type, &te.Text, &te.HREF)
		if err != nil {
			return nil, err
		}
		textEntities = append(textEntities, te)
	}
	return textEntities, nil
}

func (c *Controller) TagMessage(messageID int, tagName string) error {
	// log.Debug("trying to apply a tag", "tag_name", tag_name)
	tx, err := c.conn.Begin(context.Background())
	if err != nil {
		return err
	}

	_, err = tx.Exec(context.Background(),
		`INSERT INTO tags_list (name)
	VALUES ($1)
	ON CONFLICT DO NOTHING;`, tagName)

	row := tx.QueryRow(context.Background(),
		`SELECT id FROM tags_list WHERE name = $1;`, tagName)
	var tagID int
	row.Scan(&tagID)

	_, err = tx.Exec(context.Background(), "INSERT INTO tags (message_id, tag_id) VALUES ($1, $2);", messageID, tagID)

	err = tx.Commit(context.Background())

	return err
}

// in telegram, every message contains at most 1 photo or file,
// so posts with multiple images or files are actually multiple posts
// with sequential ids and similar properties. this function checks if
// the messages are likely to be from one group
func arePhotoMessagesMergeable(m1, m2 *Message) bool {
	if m1 == nil || m2 == nil {
		return false
	}
	hasSameMedia := (m1.Photo != "" &&
		m2.Photo != "") || (m1.File != "" &&
		m2.File != "")

	return (m1.ID+1 == m2.ID &&
		hasSameMedia &&
		m1.DateUnixtime == m2.DateUnixtime &&
		m1.FromID == m2.FromID &&
		m1.ForwardedFromID == m2.ForwardedFromID)
}
