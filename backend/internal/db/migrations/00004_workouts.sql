-- +goose Up

CREATE TYPE schedule_kind AS ENUM ('rotation', 'weekly');

-- Per-user data about an exercise. PBs are NOT stored: derive them from
-- session_sets. work_weight is a cache recomputed on session save.
CREATE TABLE user_exercise_stats (
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_id uuid NOT NULL REFERENCES exercises(id),
    work_weight numeric(6,2),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, exercise_id)
);

CREATE INDEX user_exercise_stats_sync_idx ON user_exercise_stats (user_id, updated_at);

-- Routines (templates). rotation: ordered days; weekly: fixed weekdays over up
-- to 4 weeks.
CREATE TABLE routines (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        text NOT NULL,
    kind        schedule_kind NOT NULL DEFAULT 'rotation',
    cycle_weeks int NOT NULL DEFAULT 1 CHECK (cycle_weeks BETWEEN 1 AND 4),
    start_date  date,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);

CREATE INDEX routines_sync_idx ON routines (user_id, updated_at);

CREATE TABLE routine_days (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    routine_id uuid NOT NULL REFERENCES routines(id) ON DELETE CASCADE,
    position   int NOT NULL,
    name       text NOT NULL,
    week_index int CHECK (week_index BETWEEN 0 AND 3),
    weekday    int CHECK (weekday BETWEEN 1 AND 7),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CHECK ((week_index IS NULL) = (weekday IS NULL))
);

CREATE UNIQUE INDEX routine_days_position_uq
    ON routine_days (routine_id, position) WHERE deleted_at IS NULL;

CREATE TABLE routine_day_exercises (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    routine_day_id uuid NOT NULL REFERENCES routine_days(id) ON DELETE CASCADE,
    position       int NOT NULL,
    exercise_id    uuid NOT NULL REFERENCES exercises(id),
    target_sets    int CHECK (target_sets > 0),
    target_reps    int4range,
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);

CREATE UNIQUE INDEX routine_day_exercises_position_uq
    ON routine_day_exercises (routine_day_id, position) WHERE deleted_at IS NULL;

CREATE INDEX routine_day_exercises_exercise_idx ON routine_day_exercises (exercise_id);

-- Sessions (what actually happened). Survive routine edits/deletes.
CREATE TABLE workout_sessions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    routine_id     uuid REFERENCES routines(id) ON DELETE SET NULL,
    routine_day_id uuid REFERENCES routine_days(id) ON DELETE SET NULL,
    day_name       text,
    started_at     timestamptz NOT NULL,
    ended_at       timestamptz,
    notes          text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);

CREATE INDEX workout_sessions_sync_idx    ON workout_sessions (user_id, updated_at);
CREATE INDEX workout_sessions_history_idx ON workout_sessions (user_id, started_at DESC);

CREATE TABLE session_sets (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  uuid NOT NULL REFERENCES workout_sessions(id) ON DELETE CASCADE,
    exercise_id uuid NOT NULL REFERENCES exercises(id),
    set_number  int NOT NULL,
    weight_kg   numeric(6,2) CHECK (weight_kg >= 0),
    reps        int CHECK (reps >= 0),
    rpe         numeric(3,1) CHECK (rpe BETWEEN 1 AND 10),
    is_warmup   boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);

CREATE INDEX session_sets_session_idx  ON session_sets (session_id);
CREATE INDEX session_sets_exercise_idx ON session_sets (exercise_id);

-- updated_at triggers for every table that has the column. BEFORE UPDATE does
-- not fire on INSERT, so clients (or column defaults) provide the initial value.
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'users', 'refresh_tokens',
        'muscle_groups', 'exercises', 'exercise_muscles',
        'user_exercise_stats',
        'routines', 'routine_days', 'routine_day_exercises',
        'workout_sessions', 'session_sets'
    ] LOOP
        EXECUTE format(
            'CREATE TRIGGER %I BEFORE UPDATE ON %I
               FOR EACH ROW EXECUTE FUNCTION set_updated_at()',
            t || '_set_updated_at', t);
    END LOOP;
END $$;

-- +goose Down

DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'users', 'refresh_tokens',
        'muscle_groups', 'exercises', 'exercise_muscles',
        'user_exercise_stats',
        'routines', 'routine_days', 'routine_day_exercises',
        'workout_sessions', 'session_sets'
    ] LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS %I ON %I', t || '_set_updated_at', t);
    END LOOP;
END $$;

DROP TABLE session_sets;
DROP TABLE workout_sessions;
DROP TABLE routine_day_exercises;
DROP TABLE routine_days;
DROP TABLE routines;
DROP TABLE user_exercise_stats;
DROP TYPE schedule_kind;
