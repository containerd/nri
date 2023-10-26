
GO_CMD     := go
GO_BUILD   := $(GO_CMD) build

BUILD_PATH   := $(shell pwd)
BIN_PATH     := $(BUILD_PATH)/bin

INSTALL_PATH := /opt/nri/bin
INSTALL_CONFIG_PATH  := /etc/nri

EXAMPLESS := \
	$(BIN_PATH)/clearcfs

ifneq ($(V),1)
  Q := @
endif

all: build

build: $(EXAMPLESS)

$(BIN_PATH)/clearcfs: $(wildcard clearcfs/*.go)
	$(Q)echo "Building $@..."; \
	cd $(dir $<) && $(GO_BUILD) -o $@ .

clean:
	$(Q)rm -f $(EXAMPLESS)

install:
	install -D -m 755 $(EXAMPLESS) $(INSTALL_PATH)/
	install -D -m 755 misc/conf.json $(INSTALL_CONFIG_PATH)/conf.json