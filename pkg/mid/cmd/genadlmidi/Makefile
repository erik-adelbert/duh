# Lifecycle of the embedded bank database ../../adlmidi.dat.
#
#   make            same as 'make build'
#   make build      regenerate adlmidi.dat from the local banks (incremental)
#   make rebuild    regenerate from scratch, ignoring the instrument cache
#   make config     convert fm_banks_new/banks.ini into banks.json
#   make download   fetch banks and banks.ini from the libADLMIDI repository
#   make check      verify that the database loads and the tests pass
#   make install    rebuild bin/adlmid so it embeds the current database
#   make clean      remove the generator binary
#   make distclean  also remove the instrument cache and the database

GO     ?= go
ROOT   := ../../../..
DAT    := ../../adlmidi.dat
CONFIG := fm_banks_new/banks.json
INI    := fm_banks_new/banks.ini
CACHE  := genadlmidi.cache
GEN    := genadlmidi

SRC := $(filter-out %_test.go,$(wildcard *.go))

.PHONY: all build rebuild config download check install clean distclean

all: build

$(GEN): $(SRC)
	$(GO) build -o $@ .

# The database depends on the generator, its config and the bank files.
$(DAT): $(GEN) $(CONFIG) $(wildcard fm_banks_new/*/*)
	./$(GEN) -c $(CONFIG) -cache $(CACHE) -o $@

build: $(DAT)

rebuild: $(GEN)
	rm -f $(CACHE)
	./$(GEN) -c $(CONFIG) -cache $(CACHE) -o $(DAT)

$(CONFIG): $(INI) | $(GEN)
	./$(GEN) -ini $(INI) -c $@

config: $(CONFIG)

download: $(GEN)
	./$(GEN) -download
	$(MAKE) config

check: $(DAT)
	cd $(ROOT) && $(GO) vet ./pkg/adlmid/... && $(GO) test ./pkg/adlmid/...

install: $(DAT)
	cd $(ROOT) && $(GO) build -o bin/adlmid ./cmd/adlmid

clean:
	rm -f $(GEN)

distclean: clean
	rm -f $(CACHE) $(DAT)
