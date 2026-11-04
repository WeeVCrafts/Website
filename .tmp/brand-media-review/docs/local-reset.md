# Local reset accounts

The local Docker workflow has one explicit reset command. It refuses to run unless the reset flag is set and DATABASE_URL points to the local wecratfs database on localhost or the Compose postgres service.

    docker compose up -d postgres redis meilisearch mailpit
    DATABASE_URL=postgres://wecratfs:wecratfs@localhost:5432/wecratfs?sslmode=disable make reset-local
    docker compose exec -T redis redis-cli FLUSHDB

The reset removes application rows, recreates the role catalogue, and creates exactly these three accounts plus one local catalog fixture used to verify seller, customer, cart, and admin flows:

| Role | Email |
| --- | --- |
| Customer | customer@weevcrafts.local |
| Seller owner | seller@weevcrafts.local |
| Super admin | superadmin@weevcrafts.local |

The default local-only password is WeeVCrafts-Local-2026!. Set WECRATFS_RESET_PASSWORD to use another password. Redis is flushed separately so old local sessions cannot survive the reset. Hosted databases, GitHub, local Garage objects, and production data are not touched.
