pkg_app:
	pkger

build_app: pkg_app
	go build cmd/poll/poll.go


run: build_app
	./poll
