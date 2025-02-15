-- +goose Up
CREATE TABLE feed_follows(
id UUID PRIMARY KEY,
created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
feed_id UUID NOT NULL REFERENCES feeds(id) ON DELETE CASCADE, 
constraint user_feed_constr UNIQUE (user_id, feed_id)
);

-- +goose Down
DROP TABLE feed_follows;
