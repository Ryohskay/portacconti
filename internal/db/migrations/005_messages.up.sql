CREATE TABLE messages (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id UUID NOT NULL REFERENCES appointments(id),
    sender_id      UUID NOT NULL REFERENCES users(id),
    recipient_id   UUID NOT NULL REFERENCES users(id),
    subject_enc    BYTEA,
    body_enc       BYTEA NOT NULL,
    sent_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at        TIMESTAMPTZ
);

CREATE INDEX ON messages(appointment_id);
CREATE INDEX ON messages(recipient_id);
