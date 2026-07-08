-- Fixture for lint warnings on views, triggers, and non-simple indexes
PRAGMA foreign_keys=OFF;

CREATE TABLE items (
  id INTEGER PRIMARY KEY,
  email TEXT NOT NULL,
  deleted_at TEXT
);

CREATE VIEW active_items AS
  SELECT id, email FROM items WHERE deleted_at IS NULL;

CREATE TRIGGER items_updated_at
AFTER UPDATE ON items
BEGIN
  UPDATE items SET email = email WHERE id = NEW.id;
END;

CREATE INDEX idx_items_email ON items(email);

CREATE INDEX idx_items_active_email ON items(email) WHERE deleted_at IS NULL;

CREATE INDEX idx_items_lower_email ON items(lower(email));

INSERT INTO items (id, email) VALUES (1, 'a@example.com');
