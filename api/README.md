# PlayTak-API

## Description

The Play Tak API is a nodejs built using the nestjs project which contains multiple endpoints for servicing data to the Play Tak UI.

## Requirements

- node v22
- pnpm
- sqlite3

## Installation

```bash
pnpm install
```

## Running the app

First, need to set up the .env file which you can copy from the .env.example file.
```
cp .env.example .env
```

There are two options to run the project: manually, or with docker (recommended).

Either way, you will need to create two databases, which can be done with the following bash script:

```bash
../scripts/development/create_databases.sh
```

This creates the players and games sqlite databases.

Optionally, you can then use the script `../scripts/development/add_user.sh` to add users to the local players database with a password of "password".

```bash
../scripts/development/add_user.sh mynewusername ../playtakdb/players.db
# See ../scripts/development/add_user.sh comments for more options.
```

### docker

In the root of the repo, run the following command:

```bash
docker compose up -d --build
```

### manual

```bash
# development
$ pnpm run start

# watch mode
$ pnpm run start:dev

# production mode
$ pnpm run start:prod
```

## Test

```bash
# unit tests
$ pnpm run test

# e2e tests
$ pnpm run test:e2e

# test coverage
$ pnpm run test:cov
```

## Endpoints

SwaggerOpenAPI Documentation

https://api.playtak.com/api
https://api.beta.playtak.com/api

Events

- GET /events

Games History

- GET /v1/games-history/
- GET /v1/games-history/{id}
- GET /v1/games-history/ptn/{id}
- GET /v1/hames-history/db

Ratings

- GET /v1/ratings
- GET /v1/ratings/{player_name}

Misc

- GET /health
