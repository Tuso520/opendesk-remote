ALTER TABLE build_profiles
  ADD COLUMN source_json JSON NULL AFTER signing_json;

