.PHONY: test e2etest

test:
	docker compose exec app go test -p 1 -tags=integration ./... -v

e2etest:
	docker compose exec app go test -p 1 -tags=e2e ./e2e/... -v --count 1
