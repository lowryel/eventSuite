-- +goose Up
-- +goose StatementBegin
-- create user role enum

-- CREATE DATABASE eventsdb;

-- GRANT ALL PRIVILEGES ON DATABASE eventsdb TO lotus_api;

-- GRANT ALL PRIVILEGES ON ALL TABLE IN SCHEMA PUBLIC TO lotus_api;


CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE user_roles AS ENUM('user', 'organizer');

CREATE TABLE IF NOT EXISTS organizer(
    organizer_id UUID PRIMARY KEY DEFAULT uuid_generate_v4() NULL, name TEXT, description TEXT, email TEXT, phone TEXT, password TEXT, role user_roles, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ
);


-- create Event table
CREATE TABLE IF NOT EXISTS event (
    event_id UUID PRIMARY KEY, title VARCHAR(255), description VARCHAR(255), start_date VARCHAR(255), end_date VARCHAR(255), location  VARCHAR(255), organizer_id UUID REFERENCES organizer (organizer_id) ON DELETE CASCADE, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ
);

-- update table with event foreign key
ALTER TABLE organizer ADD COLUMN events_managed text;

ALTER TABLE organizer ADD FOREIGN KEY (events_managed) REFERENCES event;

-- table containing login data
CREATE TABLE IF NOT EXISTS login_data (
    loginID TEXT PRIMARY KEY, email VARCHAR(255), username VARCHAR(255), password VARCHAR(255), phone VARCHAR(255), role user_roles
);

CREATE TABLE IF NOT EXISTS regular_user (
    loginID UUID PRIMARY KEY default uuid_generate_v4 (), email VARCHAR(255), username VARCHAR(255), password VARCHAR(255), phone VARCHAR(255), role user_roles
);

--  joint table between event and regular user
CREATE TABLE IF NOT EXISTS event_user (
    event_userID UUID PRIMARY KEY, eventID VARCHAR(255), userID VARCHAR(255)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP DATABASE eventsdb;

ALTER TABLE organizer DROP COLUMN events_managed;


drop type user_roles CASCADE;
DROP TABLE organizer CASCADE;

DROP TABLE event CASCADE;

DROP TABLE login_data CASCADE;

DROP TABLE event_user;

-- DROP TABLE regular_user;


-- DROP TABLE ticket;


-- DROP TABLE registration;


-- DROP TABLE archive_event;

-- +goose StatementEnd