#!/usr/bin/env bash

# Script for developer purposes, generating SQL for creating a new user that can be authenticated
# with the password of literally "password". (Therefore: only for development/test setup.)
#
# Usage:
#
#   add_user.sh <username> <path-to-sqlite-db-file>
#
# Examples:
#
#   add_user.sh localtestuser playtakdb/players.db
#
#   for x in \$( seq 1 100 ) | do add_user.sh user$x playtakdb/players.db
#
# Requirements:
#
#   expects sqlite3 to be executable in shell
#
# Notes:
#
# 1. Only for use development purposes where the exact initial password doesn't matter!
#
# 2. Must start/restart server after new users have been added, since the server won't
#    be aware of manually created users.
#
# 3. Why this script is useful: email isn't usually configured locally, so you cannot get the randomly created password.
#    Also, this manual process would be laborious for a larger amount of test users or setting up a test bed.

if [ -z $1 ]; then
  echo "Error: Must provide username as first argument" 1>&2
  echo "See script source for usage notes." 1>&2
  exit 1
fi

if [ -z $2 ]; then
  echo "Error: Must provide database (as path to sqlite file) as second argument" 1>&2
  echo "See script source for usage notes." 1>&2
  exit 1
fi

existing=$( sqlite3 $2 "select COUNT(name) FROM players WHERE name='$1'" )

if [ $existing != 0 ]; then
  echo "User already exists with name $1"
  echo "Exiting."
  exit 1
fi

# Determining next player ID, since table doesn't currently autoinc with a default.
nextId=$( sqlite3 $2 "select 1 + IFNULL(MAX(id), 0) FROM players" )
echo "Using next Player ID: $nextId"

# password hash below is a precalculated bcrypt hash that authenticates to "password"
# email is empty string, which currently is allowed
# id is determined earlier and provided explicitly
sql="INSERT INTO players (id, name, password, email, rating, boost, ratedgames, maxrating, ratingage, ratingbase, unrated, isbot, fatigue, is_admin, is_mod, is_gagged, is_banned) VALUES('$nextId', '$1','\$2a\$10\$JpGWa00gDsDtJK0Ttq/dD.F2zTj9kCkdcHwO5pIZFfWbT3CfDY/C6','',1000.0,750.0,0,1000.0,0,0,0,0,'{}',0,0,0,0);"
echo $sql | sqlite3 $2
echo "Created user $1 (#$nextId) with password: password"
