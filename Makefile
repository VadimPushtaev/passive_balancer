.PHONY: docker
docker:
	docker build . -t passive_balancer

.PHONY: docker-run
docker-run:
	docker run -it -p 2308:2308 passive_balancer