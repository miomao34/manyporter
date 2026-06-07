package main

import (
	"encoding/json"
	"os"
)

func (c *Controller) Import(rootPath string) error {
	pathToExportJson := rootPath + string(os.PathSeparator) + "result.json"
	result, err := os.ReadFile(pathToExportJson)
	if err != nil {
		return err
	}
	var export Export
	err = json.Unmarshal(result, &export)
	if err != nil {
		return err
	}

	sourceID, err := c.InsertSourceFolder(rootPath)
	if err != nil {
		return err
	}

	im := Importer{conn: c.conn}
	for _, message := range export.Messages {
		message.SourceFolderID = sourceID
		im.InsertMessage(message)
	}
	return nil
}
