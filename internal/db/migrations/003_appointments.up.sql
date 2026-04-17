CREATE TYPE appointment_status AS ENUM (
    'pending_payment',
    'confirmed',
    'in_progress',
    'completed',
    'cancelled',
    'no_show'
);

CREATE TABLE appointments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timeslot_id         UUID NOT NULL REFERENCES timeslots(id),
    patient_id          UUID NOT NULL REFERENCES users(id),
    counsellor_id       UUID REFERENCES users(id),
    status              appointment_status NOT NULL DEFAULT 'pending_payment',
    meeting_url         TEXT,
    cancellation_reason TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE TABLE patient_records (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id UUID NOT NULL REFERENCES appointments(id),
    author_id      UUID NOT NULL REFERENCES users(id),
    content_enc    BYTEA NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON appointments(patient_id);
CREATE INDEX ON appointments(counsellor_id);
CREATE INDEX ON appointments(timeslot_id);
CREATE INDEX ON appointments(status);
CREATE INDEX ON patient_records(appointment_id);
