#!/usr/bin/env bash

# Script for development purposes, generating SQL for creating a new user that can be authenticated
# with the password of literally "password". (Therefore: only for development/test setup.)
#
# Usage:
#
#   add_user.sh <username>
#
# Examples:
#
#   add_user.sh localtestuser
#
#   for x in \$( seq 1 100 ) | do add_user.sh user$x
#
# Requirements:
#
#   expects containerized mariadb to be running
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

# docker exec -i playtak-db mariadb -uroot -pmydb123 --database=playtak_db < ./init-mariadb.sql
existing=$( docker exec -i playtak-db mariadb -uroot -pmydb123 --database=playtak_db <<< "SELECT COUNT(name) FROM players WHERE name = '$1'" | tail -1)

if [ $existing != 0 ]; then
  echo "User already exists with name $1"
  echo "Exiting."
  exit 1
fi

# password hash below is a precalculated bcrypt hash that authenticates to "password"
# email is just set as test@test.com
sql="INSERT INTO players (name, password, email, rating, boost, ratedgames, maxrating, ratingage, ratingbase, unrated, isbot, fatigue, is_admin, is_mod, is_gagged, is_banned) VALUES('$1','\$2a\$10\$JpGWa00gDsDtJK0Ttq/dD.F2zTj9kCkdcHwO5pIZFfWbT3CfDY/C6','test@test.com',1000.0,750.0,0,1000.0,0,0,0,0,'{}',0,0,0,0);"
docker exec -i playtak-db mariadb -uroot -pmydb123 --database=playtak_db <<< $sql
echo "Created user $1 with password: password"
