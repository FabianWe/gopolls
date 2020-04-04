build_app:
	@pkger
	@go build cmd/poll/poll.go


run: build_app
	./poll
