# PlayTak 

This is the new monorepo for the PlayTak services 

## Description

![Unit Tests](https://github.com/USTakAssociation/playtak-api/actions/workflows/ci.yml/badge.svg)


This is repo contains the api and Tak server for servicing data to the Play Tak UI client and games ui

## Requirements
- docker
- nvm
- node v20
- sqlite3

## Setup 

Run the following commands to setup the api for local development

```bash
nvm use
cd ./api
npm ci
cd ..
```

setup the local databases


```bash
sh ./script/development/create_databases.sh
```
This creates the players and games sqlite dbs in the playersdb folder

Optionally, you can then use the script `./scripts/development/add_user.sh` to add users to the local players database with a password of "password".

```bash
./scripts/development/add_user.sh mylocalacct ./players.db
# See scripts/development/add_user.sh comments for more options.
```

## Running the services

Run the following docker command to start all the needed services

```bash
docker compose up -d --build
```

## Test

You can run tests for each of the apps 

```bash
cd ./api
# unit test
npm run test

# e2e tests
npm run test:e2e

# test coverage
npm run test:cov
```

You can also run integration tests using [Bruno](https://www.usebruno.com/)

open the collections from each of the projects folders either under test/bruno or src/test/bruno

or you can install the bruno cli and run them like so

```bash
npm install -g @usebruno/cli
```

```bash
cd ./api/test/bruno/playtak-api
bru run test-suite --env local
```

## TODO
- migrate to mariadb
- create user auth endpoints and test
- setup API key registration to track usage
- add automated versioning and setup release artifacts with github
- build out more robust deploy and rollback
- java tests


## Contributing
PlayTak is an Open Source Project. This means that:

> Individuals making significant and valuable contributions are given commit-access to the project to contribute as they see fit.

Please read [CONTRIBUTING.md](docs/CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests to us.

## Versioning
We use [SemVer](http://semver.org/) for versioning. For the versions available, see the tags on this repository.

## Contributors
PlayTak is only possible due to the excellent work of the following contributors:

||
:----:|
|[chaitu](https://github.com/chaitu236)|
|[Nohat](https://github.com/NoHatCoder)|
|[Nitzel](https://github.com/nitzel)|
|[InvaderB](https://github.com/invaderb)|

See also the list of contributors who participated in this project.

License
MIT License © USTA see LICENSE.md file

