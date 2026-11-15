-- +goose Up
-- A name the merchant gives a key so they can tell their keys apart.
ALTER TABLE merchant_api_keys
    ADD COLUMN label text NOT NULL DEFAULT '' CHECK (char_length(label) <= 64);

-- +goose Down
ALTER TABLE merchant_api_keys DROP COLUMN label;
