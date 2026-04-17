CREATE TABLE questionnaire_templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    locale      VARCHAR(10) NOT NULL,
    title       TEXT NOT NULL,
    schema_json JSONB NOT NULL DEFAULT '[]',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE questionnaire_tokens (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id UUID NOT NULL REFERENCES appointments(id),
    template_id    UUID NOT NULL REFERENCES questionnaire_templates(id),
    token_hash     TEXT NOT NULL UNIQUE,
    expires_at     TIMESTAMPTZ NOT NULL,
    used_at        TIMESTAMPTZ
);

CREATE TABLE questionnaire_responses (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id UUID NOT NULL REFERENCES appointments(id),
    template_id    UUID NOT NULL REFERENCES questionnaire_templates(id),
    answers_enc    BYTEA NOT NULL,
    submitted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON questionnaire_tokens(token_hash);
CREATE INDEX ON questionnaire_tokens(appointment_id);
CREATE INDEX ON questionnaire_responses(appointment_id);

-- Seed default templates
INSERT INTO questionnaire_templates (locale, title, schema_json) VALUES
('en', 'Pre-visit Questionnaire', '[
  {"id":"chief_complaint","type":"text","text_en":"What is your main concern for this visit?","text_ja":"今回の相談内容を教えてください。","required":true},
  {"id":"symptoms_duration","type":"text","text_en":"How long have you been experiencing these symptoms?","text_ja":"いつ頃から症状が続いていますか？","required":true},
  {"id":"previous_counselling","type":"boolean","text_en":"Have you received counselling before?","text_ja":"以前にカウンセリングを受けたことはありますか？","required":true},
  {"id":"medications","type":"text","text_en":"Are you currently taking any medications?","text_ja":"現在、服用している薬はありますか？","required":false},
  {"id":"emergency_contact","type":"text","text_en":"Emergency contact name and phone number","text_ja":"緊急連絡先（名前と電話番号）","required":false}
]'),
('ja', '事前問診票', '[
  {"id":"chief_complaint","type":"text","text_en":"What is your main concern for this visit?","text_ja":"今回の相談内容を教えてください。","required":true},
  {"id":"symptoms_duration","type":"text","text_en":"How long have you been experiencing these symptoms?","text_ja":"いつ頃から症状が続いていますか？","required":true},
  {"id":"previous_counselling","type":"boolean","text_en":"Have you received counselling before?","text_ja":"以前にカウンセリングを受けたことはありますか？","required":true},
  {"id":"medications","type":"text","text_en":"Are you currently taking any medications?","text_ja":"現在、服用している薬はありますか？","required":false},
  {"id":"emergency_contact","type":"text","text_en":"Emergency contact name and phone number","text_ja":"緊急連絡先（名前と電話番号）","required":false}
]');
