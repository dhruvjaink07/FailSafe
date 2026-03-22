# FailSafe Welcomes you

## Why this structure?

> cmd/ → entry point binaries

> internal/ → private app logic

> models/ → structs

> orchestrator/ → experiment lifecycle logic

> monitoring/ → metrics collectors

> handlers/ → HTTP handlers

### Command to install Postgres
```
docker run -d --name failsafe-postgres -e POSTGRES_USER=failsafe -e POSTGRES_PASSWORD=failsafe -e POSTGRES_DB=failsafe -p 5432:5432 postgres:15
```
### Command to initiate local postgres
```
docker exec -it failsafe-postgres psql -U failsafe -d failsafe
```

### to build the docker image
```
docker-compose up --build
```