CREATE TABLE IF NOT EXISTS recipient_group_members (
    group_id UUID NOT NULL REFERENCES recipient_groups(id) ON DELETE CASCADE,
    recipient_id UUID NOT NULL REFERENCES recipients(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, recipient_id)
);

CREATE INDEX IF NOT EXISTS idx_recipient_group_members_recipient_id ON recipient_group_members (recipient_id);
