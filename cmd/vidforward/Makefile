# NB: The default (soft) install does not override existing installation files.
USER := $(shell whoami)
PATH := /usr/local/go/bin:$(PATH)
BIN_NAME := vidforward
BIN_DIR := /src/github.com/ausocean/cloud/cmd/$(BIN_NAME)

.SILENT:soft_copy_files
.SILENT:hard_copy_files
.SILENT:clean

rebuild:
	go build -tags debug

install: as_root soft_copy_files rebuild
	@echo "Install complete"

install_hard: as_root hard_copy_files rebuild
	@echo "Hard install complete"

as_root:
ifneq ($(USER),root)
	$(error Must run as superuser!)
endif

soft_copy_files:
	if [ -f /etc/systemd/system/$(BIN_NAME).service ] ; then \
		echo "/etc/systemd/system/$(BIN_NAME).service left unmodified" ; \
	else \
		bash create_service.sh $(BIN_DIR); \
	fi
	systemctl enable $(BIN_NAME).service

hard_copy_files:
	if [ -f /etc/systemd/system/$(BIN_NAME).service ] ; then \
		echo "/etc/systemd/system/$(BIN_NAME).service overwritten" ; \
	fi
	bash create_service.sh $(BIN_DIR)
	systemctl enable $(BIN_NAME).service

clean: as_root
	rm -rf /var/log/vidforward
	rm -rf /etc/systemd/system/$(BIN_NAME).service
	@echo "Clean complete"
