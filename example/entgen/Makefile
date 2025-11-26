clean:
	rm -rf ./ent
	rm -rf ./mapper
	rm -rf ./bind
	rm -rf ./proto
	rm -rf ./api
	rm -rf ./render

run: clean
	go run ./cmd/ent
	buf dep update
	buf dep prune
	buf generate
	go run ./cmd/bind
	go test ./...
	go vet ./...
