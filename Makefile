.PHONY: all clean test install uninstall

BINDIR = $(HOME)/.local/bin

all: mkultra

mkultra: go.mod main.go dag.go parser.go expand.go executor.go
	go build -o mkultra .

clean:
	rm -f mkultra

test: mkultra
	@echo "=== test1: C compilation ==="
	cd tests/test1 && rm -f hello hello.o && ../../mkultra && ../../mkultra
	@echo "=== test2: multi-level deps ==="
	cd tests/test2 && rm -f program main.o utils.o main.c utils.c 2>/dev/null && ../../mkultra
	@echo "=== test3: simple build ==="
	cd tests/test3 && rm -f input.txt output.txt && ../../mkultra
	@echo "=== test4: circular dep detection ==="
	cd tests/test4 && ../../mkultra 2>/dev/null && echo "FAIL" || echo "PASS"

install: $(BINDIR)/mkultra

$(BINDIR)/mkultra: mkultra
	install -Dm755 mkultra $(BINDIR)/mkultra

uninstall:
	rm -f $(BINDIR)/mkultra
