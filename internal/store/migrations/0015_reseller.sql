-- Add parent_id to users for reseller logic
ALTER TABLE users ADD COLUMN parent_id INTEGER REFERENCES users(id) ON DELETE SET NULL;

-- Add user_id to sites to track owner
ALTER TABLE sites ADD COLUMN user_id INTEGER REFERENCES users(id);

-- Insert reseller role (id=3 typically, assuming admin=1, user=2)
INSERT INTO roles (name, permissions) VALUES ('reseller', '["*"]');
