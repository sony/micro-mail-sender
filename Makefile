.PHONY: test e2etest cover

test:
	docker compose exec -u testuser app go test -cover -p 1 -tags=integration ./... --count 1

e2etest:
	docker compose exec app go test -p 1 -tags=e2e ./e2e/... -v --count 1

cover:
	docker compose exec -u testuser app go test -cover -coverprofile=./tmp/coverage.out -tags=integration ./...
	go tool cover -html=./tmp/coverage.out
