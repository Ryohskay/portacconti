CREATE TYPE payment_status AS ENUM (
    'created',
    'processing',
    'succeeded',
    'failed',
    'refunded'
);

CREATE TABLE payments (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id        UUID NOT NULL REFERENCES appointments(id),
    patient_id            UUID NOT NULL REFERENCES users(id),
    stripe_payment_intent TEXT NOT NULL UNIQUE,
    amount_jpy            BIGINT NOT NULL,
    currency              VARCHAR(3) NOT NULL DEFAULT 'jpy',
    status                payment_status NOT NULL DEFAULT 'created',
    stripe_event_id       TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON payments(appointment_id);
CREATE INDEX ON payments(stripe_payment_intent);
