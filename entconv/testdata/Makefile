test:
	buf dep update
	buf dep prune
	buf generate
	go run ./cmd/entgen
	go run ./cmd/entconv
	go run ./cmd/conv
