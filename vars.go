package main

import (
	"fmt"
	"os"
	"strconv"
)

type Vars struct {
	username string
	password string
	database string
	port     string
	hostname string

	tgApiKey string
	tgUserID int64
}

var (
	username string = getEnvVarWithPanic("PG_USERNAME")
	password string = getEnvVarWithPanic("PG_PASSWORD")
	database string = getEnvVarWithPanic("PG_DATABASE")
	port     string = getEnvVarWithPanic("PG_PORT")
	hostname string = getEnvVarWithPanic("PG_HOSTNAME")

	tgApiKey string = getEnvVarWithPanic("TELEGRAM_BOT_TOKEN")
	tgUserID int64
)

func getEnvVarWithPanic(envVar string) string {
	result, ok := os.LookupEnv(envVar)
	if !ok {
		panic(fmt.Sprintf("failed to get env var %v", envVar))
	}
	return result
}

func InitTgUserID() error {
	rawVar := getEnvVarWithPanic("TELEGRAM_USER_ID")
	var err error
	var tgUserIDInt int
	tgUserIDInt, err = strconv.Atoi(rawVar)
	if err != nil {
		return err
	}
	tgUserID = int64(tgUserIDInt)
	return nil
}
