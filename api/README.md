# PlayTak-API 

## Description

![Unit Tests](https://github.com/USTakAssociation/playtak-api/actions/workflows/ci.yml/badge.svg)

This is the Play Tak API project which contains multiple endpoints for servicing data to the Play Tak UI

## Requirements
- node v20
- sqlite3

## Installation

```bash
$ npm install
```

## Running the app

There are 2 options to run the project either manually or with docker (*recommended)

Either option you go you will need to create 2 databases which can be done with the following bash script

```bash
sh ../script/development/create_databases.sh
```
This creates the players and games sqlite dbs

Optionally, you can then use the script `scripts/development/add_user.sh` to add users to the local players database with a password of "password".

```bash
./scripts/development/add_user.sh mylocalacct ./players.db
# See scripts/development/add_user.sh comments for more options.
```

### docker
In the root of the repo run the following command
```bash
docker compose up -d --build
```

### manual
```bash
# development
$ npm run start

# watch mode
$ npm run start:dev

# production mode
$ npm run start:prod
```

## Test

```bash
# unit tests
$ npm run test

# e2e tests
$ npm run test:e2e

# test coverage
$ npm run test:cov
```

## Endpoints

SwaggerOpenAPI Documentation

https://api.playtak.com/api

https://api.beta.playtak.com/api

Events
- /events

Games History
- /v1/games-history/
- /v1/games-history/{id}
- /v1/games-history/ptn/{id}
- /v1/hames-history/db

Ratings
- /v1/ratings
- /v1/ratings/{player_name}
