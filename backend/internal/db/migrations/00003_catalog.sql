-- +goose Up

CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END;
$$;

CREATE TYPE visibility AS ENUM ('private', 'public');

-- System rows (owner_id NULL) are curated by migrations; users may add private
-- ones. Public exercises can only use system groups.
CREATE TABLE muscle_groups (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   uuid REFERENCES users(id) ON DELETE CASCADE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX muscle_groups_owner_name_uq
    ON muscle_groups (owner_id, lower(name)) NULLS NOT DISTINCT
    WHERE deleted_at IS NULL;

CREATE INDEX muscle_groups_sync_idx ON muscle_groups (owner_id, updated_at);

-- Search helpers only ("pecs" -> chest). System groups only, ships with the app.
CREATE TABLE muscle_group_aliases (
    muscle_group_id uuid NOT NULL REFERENCES muscle_groups(id) ON DELETE CASCADE,
    alias           text NOT NULL,
    PRIMARY KEY (muscle_group_id, alias)
);

-- Referenced by id from user data. Never hard-deleted; archive instead.
-- Orphans (owner deleted) stay public if public, otherwise unreachable.
CREATE TABLE exercises (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid REFERENCES users(id) ON DELETE SET NULL,
    name        text NOT NULL,
    visibility  visibility NOT NULL DEFAULT 'private',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);

CREATE UNIQUE INDEX exercises_owner_name_uq
    ON exercises (owner_id, lower(name)) NULLS NOT DISTINCT
    WHERE archived_at IS NULL;

CREATE INDEX exercises_sync_idx ON exercises (owner_id, updated_at);

CREATE TABLE exercise_muscles (
    exercise_id     uuid NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    muscle_group_id uuid NOT NULL REFERENCES muscle_groups(id) ON DELETE CASCADE,
    is_primary      boolean NOT NULL DEFAULT true,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,
    PRIMARY KEY (exercise_id, muscle_group_id)
);

CREATE INDEX exercise_muscles_muscle_idx ON exercise_muscles (muscle_group_id);

-- Rule: a public exercise may only reference system muscle groups.
CREATE FUNCTION check_public_exercise_muscles() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM exercises e
        JOIN muscle_groups m ON m.id = NEW.muscle_group_id
        WHERE e.id = NEW.exercise_id
          AND e.visibility = 'public'
          AND m.owner_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'public exercises can only use system muscle groups';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER exercise_muscles_public_check
    BEFORE INSERT OR UPDATE ON exercise_muscles
    FOR EACH ROW EXECUTE FUNCTION check_public_exercise_muscles();

-- Same rule when an existing exercise is switched to public.
CREATE FUNCTION check_exercise_made_public() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.visibility = 'public' AND EXISTS (
        SELECT 1
        FROM exercise_muscles em
        JOIN muscle_groups m ON m.id = em.muscle_group_id
        WHERE em.exercise_id = NEW.id
          AND em.deleted_at IS NULL
          AND m.owner_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'remove private muscle groups before making an exercise public';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER exercises_public_check
    BEFORE UPDATE OF visibility ON exercises
    FOR EACH ROW EXECUTE FUNCTION check_exercise_made_public();

-- +goose Down

DROP TRIGGER exercises_public_check ON exercises;
DROP FUNCTION check_exercise_made_public();
DROP TRIGGER exercise_muscles_public_check ON exercise_muscles;
DROP FUNCTION check_public_exercise_muscles();
DROP TABLE exercise_muscles;
DROP TABLE exercises;
DROP TYPE visibility;
DROP TABLE muscle_group_aliases;
DROP TABLE muscle_groups;
DROP FUNCTION set_updated_at();
