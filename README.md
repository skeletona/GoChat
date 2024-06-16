    +---------------------------------------+
    |    ____        ____ _           _     |
    |   / ___| ___  / ___| |__   ____| |_   |
    |  | |  _ / _ \| |   | '_ \ / _  | __|  |
    |  | |_| | (_) | |___| | | | (_| | |_   |
    |   \____|\___/ \____|_| |_|\__,_|\__|  |
    |                                       |
    +---------------------------------------+

Simple CLI chat, written in 10 days to learn Go and prepare for a backend Go developer interview.

Server usage: <br>
1 systemctl start postgresql <br>
2 createdb chat <br>
3 psql -d chat -a -f ./init-db.sql <br>
4 ./server [port] <br>

Client usage: <br>
1 ./client [ip:port]
