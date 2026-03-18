CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ENUMS 

CREATE TYPE USER_ROLE AS ENUM (
    'USER',
    'MODERATOR',
    'ADMIN'
);

CREATE TYPE TEST_SCOPE AS ENUM (
  'PRIVATE',
  'PUBLIC'
);

CREATE TYPE QUESTION_SCOPE AS ENUM (
    'PRIVATE',
    'PUBLIC'
);


-- USERS

CREATE TABLE users (
    id varchar(255) PRIMARY KEY,
    name varchar(255) NOT NULL,
    email varchar(255) UNIQUE NOT NULL,
    picture varchar(255),
    role USER_ROLE NOT NULL DEFAULT 'USER',
    updated_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL
);

-- SESSIONS

CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    user_id varchar(255) NOT NULL,
    session_token varchar(255) NOT NULL,
    ip_address varchar(255) NOT NULL,
    user_agent varchar(2000) NOT NULL,
    revoked boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);

CREATE INDEX idx_session_user_id_revoked
   ON sessions (user_id, revoked);

-- COURSES

CREATE TABLE courses (
  id varchar(32) PRIMARY KEY,
  owner_id varchar(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  description text,
  icon_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);


-- TESTS

CREATE TABLE tests (
  id varchar(32) PRIMARY KEY,
  course_id varchar(32) NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
  owner_id varchar(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title text NOT NULL,
  description text,
  scope TEST_SCOPE NOT NULL DEFAULT 'PRIVATE',
  total_questions int NOT NULL DEFAULT 0,
  max_time_seconds int NOT NULL DEFAULT 7200,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX test_course_idx ON tests(course_id);

-- QUESTIONS

CREATE TABLE questions (
  id varchar(32) PRIMARY KEY,
  course_id varchar(32) NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
  owner_id varchar(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  scope QUESTION_SCOPE NOT NULL DEFAULT 'PRIVATE',
  question_text text NOT NULL,
  options jsonb NOT NULL,
  correct_index int NOT NULL,
  explanation text,
  has_latex boolean DEFAULT false,
  diagram_url text,
  difficulty text NOT NULL,
  content_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT questions_course_hash_unique UNIQUE (course_id, content_hash)
);

CREATE INDEX questions_course_idx ON questions(course_id);
CREATE INDEX questions_difficulty_idx ON questions(difficulty);

-- TAGS

CREATE TABLE tags (
  id varchar(32) PRIMARY KEY,
  course_id varchar(32) NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
  name text NOT NULL,

  CONSTRAINT tags_course_name_unique UNIQUE(course_id, name)
);

-- QUESTION TAGS

CREATE TABLE question_tags (
  question_id varchar(32) NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  tag_id varchar(32) NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (question_id, tag_id)
);

CREATE INDEX question_tags_tag_idx ON question_tags(tag_id);


-- TEST QUESTIONS

CREATE TABLE test_questions (
  test_id varchar(32) NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
  question_id varchar(32) NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  position int NOT NULL,

  PRIMARY KEY (test_id, question_id),
  CONSTRAINT test_questions_position_unique UNIQUE(test_id, position)
);


-- TEST RESULTS

CREATE TABLE test_results (
  id varchar(32) PRIMARY KEY,
  test_id varchar(32) NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
  owner_id varchar(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,

  started_at timestamptz NOT NULL,
  max_end_at timestamptz NOT NULL,
  ended_at timestamptz,

  total_score float,
  total_attempted int,
  total_skipped int,
  wrong_count int NOT NULL DEFAULT 0
);

CREATE INDEX test_results_owner_idx ON test_results(owner_id);

-- QUESTION ATTEMPTS

CREATE TABLE question_attempts (
  id varchar(32) PRIMARY KEY,
  question_id varchar(32) NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  test_result_id varchar(32) NOT NULL REFERENCES test_results(id) ON DELETE CASCADE,

  selected_index int,
  is_correct boolean,
  time_taken_sec int,
  attempted_at timestamptz,

  CONSTRAINT question_attempt_unique UNIQUE (test_result_id, question_id)
);

CREATE INDEX question_attempt_result_idx ON question_attempts(test_result_id);


-- COMMENTS 

COMMENT ON COLUMN courses.name IS 'Maths, English, Science, GS';
COMMENT ON COLUMN tests.title IS 'Maths Speed Test 1';
COMMENT ON COLUMN questions.options IS '4 options';
COMMENT ON COLUMN questions.correct_index IS '0-3';
COMMENT ON COLUMN tags.name IS 'algebra, speed, grammar, reasoning';
COMMENT ON COLUMN test_questions.position IS 'order of questions in test';

