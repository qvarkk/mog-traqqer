-- +goose Up

-- System muscle groups (owner_id NULL) with stable UUIDs so the client can map
-- ids to localized names. Names are stored in English; RU display names are
-- resolved client-side against these stable ids.
INSERT INTO muscle_groups (id, owner_id, name) VALUES
    ('00000000-0000-4000-8000-000000000001', NULL, 'Chest'),
    ('00000000-0000-4000-8000-000000000002', NULL, 'Back'),
    ('00000000-0000-4000-8000-000000000003', NULL, 'Shoulders'),
    ('00000000-0000-4000-8000-000000000004', NULL, 'Biceps'),
    ('00000000-0000-4000-8000-000000000005', NULL, 'Triceps'),
    ('00000000-0000-4000-8000-000000000006', NULL, 'Forearms'),
    ('00000000-0000-4000-8000-000000000007', NULL, 'Quadriceps'),
    ('00000000-0000-4000-8000-000000000008', NULL, 'Hamstrings'),
    ('00000000-0000-4000-8000-000000000009', NULL, 'Glutes'),
    ('00000000-0000-4000-8000-000000000010', NULL, 'Calves'),
    ('00000000-0000-4000-8000-000000000011', NULL, 'Abs'),
    ('00000000-0000-4000-8000-000000000012', NULL, 'Traps'),
    ('00000000-0000-4000-8000-000000000013', NULL, 'Lower Back'),
    ('00000000-0000-4000-8000-000000000014', NULL, 'Obliques'),
    ('00000000-0000-4000-8000-000000000015', NULL, 'Adductors'),
    ('00000000-0000-4000-8000-000000000016', NULL, 'Abductors');

-- Search helpers: common shorthands for a few system groups.
INSERT INTO muscle_group_aliases (muscle_group_id, alias) VALUES
    ('00000000-0000-4000-8000-000000000001', 'pecs'),
    ('00000000-0000-4000-8000-000000000002', 'lats'),
    ('00000000-0000-4000-8000-000000000007', 'quads'),
    ('00000000-0000-4000-8000-000000000008', 'hams'),
    ('00000000-0000-4000-8000-000000000011', 'core');

-- +goose Down

DELETE FROM muscle_group_aliases
    WHERE muscle_group_id IN ('00000000-0000-4000-8000-000000000001',
                             '00000000-0000-4000-8000-000000000002',
                             '00000000-0000-4000-8000-000000000007',
                             '00000000-0000-4000-8000-000000000008',
                             '00000000-0000-4000-8000-000000000011');

DELETE FROM muscle_groups
    WHERE owner_id IS NULL
      AND id BETWEEN '00000000-0000-4000-8000-000000000001'
                AND '00000000-0000-4000-8000-000000000016';
