# Playtak-api 

## Description

![Unit Tests](https://github.com/USTakAssociation/playtak-api/actions/workflows/ci.yml/badge.svg)


This is the Play Tak API projects which contains multiple endpoints for serivcing data to the Play Tak UI

## Installation

```bash
$ npm install
```

## Running the app

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

- /events
- /v1/games-history/
- /v1/games-history/{id}
- /v1/games-historoy/ptn/{id}
- /v1/hames-history/db

- /v1/ratings
- /v1/ratings{player_name}

- /v1/ratings
- /v1/ratings{player_name}

## TODO

- Finish up migrating rerating to nest
- endpoints for getting rating data
- create user auth endpoints and test


## Contributing
PlayTak is an OPEN Open Source Project. This means that:

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
Play tak is only possible due to the excellent work of the following contributors:

||
:----:|
|[InvaderB](https://github.com/invaderb)|

See also the list of contributors who participated in this project.

License
MIT License © USTA see LICENSE.md file

