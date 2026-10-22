DO $$ BEGIN
    CREATE TYPE hospital_application_status AS ENUM (
        'PENDING_REVIEW',
        'REVISION_REQUIRED',
        'REJECTED',
        'APPROVED'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS hospital_applications (
    id UUID PRIMARY KEY,
    invitation_id UUID NOT NULL REFERENCES hospital_invitations(id),
    applicant_user_id UUID NOT NULL REFERENCES users(id),
    
    name VARCHAR(255) NOT NULL,
    address TEXT NOT NULL,
    phone VARCHAR(50) NOT NULL,
    email VARCHAR(255) NOT NULL,
    registration_number VARCHAR(100) NOT NULL,
    npwp VARCHAR(30),
    license_document_url TEXT,
    pic_full_name VARCHAR(255) NOT NULL,
    pic_position VARCHAR(100) NOT NULL,
    
    status hospital_application_status NOT NULL DEFAULT 'PENDING_REVIEW',
    admin_notes TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_hospital_applications_applicant ON hospital_applications(applicant_user_id);
CREATE INDEX IF NOT EXISTS idx_hospital_applications_status ON hospital_applications(status);
