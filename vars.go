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
	tgUserID int
}

func getEnvVarWithPanic(envVar string) string {
	result, ok := os.LookupEnv(envVar)
	if !ok {
		panic(fmt.Sprintf("failed to get env var %v", envVar))
	}
	return result
}

func InitVars() Vars {
	vars := Vars{}

	vars.username = getEnvVarWithPanic("PG_USERNAME")
	vars.password = getEnvVarWithPanic("PG_PASSWORD")
	vars.database = getEnvVarWithPanic("PG_DATABASE")
	vars.port = getEnvVarWithPanic("PG_PORT")
	vars.hostname = getEnvVarWithPanic("PG_HOSTNAME")

	vars.tgApiKey = getEnvVarWithPanic("TELEGRAM_BOT_TOKEN")
	var err error
	vars.tgUserID, err = strconv.Atoi(getEnvVarWithPanic("TELEGRAM_USER_ID"))
	if err != nil {
		panic("telegram id is not an int")
	}

	return vars
}
