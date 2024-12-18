# PlayTak-API 

## Description

![Unit Tests](https://github.com/USTakAssociation/playtak-api/actions/workflows/ci.yml/badge.svg)


This is the Play Tak API project which contains multiple endpoints for servicing data to the Play Tak UI

## Requirements
- node v16.17.1
- sqlite3

## Installation

```bash
$ npm install
```

## Running the app


*Note: be sure to update the .env file with the correct paths to where the sqlite db files are located*
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

https://api.{env}.playtak.com

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

## TODO
- create user auth endpoints and test
- dockerize app
- automatically setup db files in the project and fill with dummy data
- setup API key registration to track usage
- add automated versioning and setup release artifacts with github
- build out more robust deploy and rollback
- add health check endpoint


## Contributing
PlayTak is an Open Source Project. This means that:

> Individuals making significant and valuable contributions are given commit-access to the project to contribute as they see fit.

Please read [CONTRIBUTING.md](docs/CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests to us.

1. Fork it!
2. Create your feature branch: git checkout -b my-new-feature
3. Add your changes: git add .
4. Commit your changes: git commit -am 'Add some feature'
5. Push to the branch: git push origin my-new-feature
6. Submit a pull request

## Versioning
We use [SemVer](http://semver.org/) for versioning. For the versions available, see the tags on this repository.

## Contributors
PlayTak is only possible due to the excellent work of the following contributors:

||
:----:|
|[InvaderB](https://github.com/invaderb)|

See also the list of contributors who participated in this project.

License
MIT License © USTA see LICENSE.md file

