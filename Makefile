build:
	go build .

install:
	go install .

manifest:
	go run main.go -manifest host
