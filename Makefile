# ZKTeco biometric gateway — operator shortcuts.
# Run from the repo root on the Raspberry Pi.

COMPOSE ?= docker compose

.PHONY: help init up down restart build logs ps status preflight register-device backup-note

help:
	@echo "Targets:"
	@echo "  make init      Scaffold .env and env/*.env from the .example files"
	@echo "  make up        Build and start the whole stack (detached)"
	@echo "  make down      Stop the stack (data volumes are preserved)"
	@echo "  make restart   Restart the gateway container only"
	@echo "  make build     Rebuild the gateway image"
	@echo "  make logs      Follow gateway logs         (S=mariadb|phpmyadmin to pick a service)"
	@echo "  make ps        Container status"
	@echo "  make preflight Sanity-check config before starting"
	@echo "  make register-device   Interactive: register a device via the API"

init:
	@[ -f .env ] || cp .env.example .env
	@[ -f env/zkbio.env ] || cp env/zkbio.env.example env/zkbio.env
	@[ -f env/db.env ] || cp env/db.env.example env/db.env
	@[ -f env/phpmyadmin.env ] || cp env/phpmyadmin.env.example env/phpmyadmin.env
	@echo "Copied .env and env/*.env — now edit them and replace every CHANGEME value."

preflight:
	@bash scripts/preflight.sh

up: preflight
	$(COMPOSE) up -d --build
	@echo "Web console: http://<pi-ip>:$${WEBUI_PORT:-8082}"
	@echo "Gateway:     http://<pi-ip>:$${ZKBIO_PORT:-8080}"
	@echo "phpMyAdmin:  http://<pi-ip>:$${PHPMYADMIN_PORT:-8081}"

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) restart zkbio

build:
	$(COMPOSE) build zkbio

logs:
	$(COMPOSE) logs -f $(or $(S),zkbio)

ps status:
	$(COMPOSE) ps

register-device:
	@bash scripts/register-device.sh
