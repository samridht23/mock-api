include .env
# up the sql migration verison
migration_up:
	@migrate -database ${DATABASE_URL}?sslmode=disable -path internal/database/migrations up

# down the sql migration verison
migration_down:
	@migrate -database ${DATABASE_URL}?sslmode=disable -path internal/database/migrations down

# for generating new up and down migration file
new_migration:
	@migrate create -ext sql -dir internal/database/migrations -seq schema
