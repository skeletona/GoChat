CREATE TABLE IF NOT EXISTS
users (
    username        TEXT NOT NULL,
    password    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS
history (
    user1   TEXT NOT NULL,
    user2   TEXT NOT NULL,
    chat    TEXT
);
