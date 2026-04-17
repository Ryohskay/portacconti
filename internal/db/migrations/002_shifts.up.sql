CREATE TYPE shift_status AS ENUM ('open', 'closed');

CREATE TABLE shifts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    manager_id  UUID NOT NULL REFERENCES users(id),
    starts_at   TIMESTAMPTZ NOT NULL,
    ends_at     TIMESTAMPTZ NOT NULL,
    status      shift_status NOT NULL DEFAULT 'open',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shifts_time_order CHECK (ends_at > starts_at)
);

CREATE TABLE timeslots (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shift_id     UUID NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
    starts_at    TIMESTAMPTZ NOT NULL,
    ends_at      TIMESTAMPTZ NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT timeslots_time_order CHECK (ends_at > starts_at)
);

CREATE TABLE shift_counsellors (
    shift_id      UUID NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
    counsellor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (shift_id, counsellor_id)
);

CREATE INDEX ON shifts(starts_at);
CREATE INDEX ON timeslots(shift_id);
CREATE INDEX ON timeslots(starts_at) WHERE is_available = TRUE;
