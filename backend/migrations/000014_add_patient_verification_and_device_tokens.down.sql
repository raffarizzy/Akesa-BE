DROP TABLE IF EXISTS user_device_tokens;

ALTER TABLE patient_profiles
    DROP COLUMN IF EXISTS profile_completed,
    DROP COLUMN IF EXISTS identity_verification_status;
