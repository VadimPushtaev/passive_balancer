passive_balancer:
	go install -v --tags integration ./...

.PHONY: docker
docker: passive_balancer
	docker build . -t passive_balancer

.PHONY: docker-run
docker-run: docker
	docker run -it -p 2308:2308 passive_balancer


test-integration: docker
	go test -v --tags integration  ./...

release:
	goreleaser release --rm-dist
