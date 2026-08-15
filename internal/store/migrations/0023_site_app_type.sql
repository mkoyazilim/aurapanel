-- Proje Tipi Eklemesi (PHP veya Node.js)
ALTER TABLE sites ADD COLUMN app_type TEXT NOT NULL DEFAULT 'php';
