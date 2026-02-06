.PHONY: build run dev clean

build:
	go build -o otter .

run: build
	./otter

dev:
	go run .

clean:
	rm -f otter
