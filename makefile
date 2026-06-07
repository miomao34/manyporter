include .env
export

all: list-targets

.PHONY: all list-targets
list-targets:
	@$(MAKE) -pRrq -f $(lastword $(MAKEFILE_LIST)) : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | egrep -v -e '^[^[:alnum:]]'

up:
	docker compose up --build -d
	
down:
	docker compose down

build:
	go build .

send:
	docker build .
	docker save manyporter-manyporter:latest -o manyporter.docker
	rsync manyporter.docker cyan:/app

run:
	go run .