#!/bin/bash

docker exec -i playtak-db mariadb -uroot -pmydb123 --database=playtak_db < ./init-mariadb.sql