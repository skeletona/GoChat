Simple CLI chat, written in 10 days to learn Go and prepare for a backend Go developer interview.

Server usage:
    1. systemctl start postgresql
    2. createdb chat
    3. psql -d chat -a -f init-db.sql
    4. ./server [port]

Client usage:
    5. ./client [ip:port]
