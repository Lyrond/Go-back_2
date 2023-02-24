ALTER TABLE games ALTER COLUMN version DROP DEFAULT,
ALTER COLUMN version TYPE uuid USING (uuid_generate_v4()), ALTER COLUMN version SET DEFAULT uuid_generate_v4();
