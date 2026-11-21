.PHONY: all build run test setup update install clean

all: build

build:
	mkdir -p bin
	go build -o bin/felix ./cmd/felix

install: build
	mkdir -p $(HOME)/.local/bin
	install -m 755 bin/felix $(HOME)/.local/bin/felix
	@echo "Felix instalado exitosamente en $(HOME)/.local/bin/felix"

run: build
	./bin/felix

setup: build
	./bin/felix setup

update:
	./update.sh

test:
	go test -count=1 ./...

clean:
	rm -rf bin/felix
