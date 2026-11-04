DELETE FROM leads WHERE phone IS NULL OR btrim(phone) = '';
ALTER TABLE leads ALTER COLUMN phone SET NOT NULL;
