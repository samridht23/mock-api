package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samridht23/mock-api/internal/apperror"
	"github.com/samridht23/mock-api/internal/middleware"
	"github.com/samridht23/mock-api/internal/utils"
)

const (
	DEFAULT_TEST_MAX_TIME  = 7200 // 2 hours
	DEFAULT_TEST_QUESTIONS = 0
)

type Test struct {
	ID             string     `json:"id"`
	CourseID       string     `json:"course_id"`
	Title          string     `json:"title"`
	Description    *string    `json:"description"`
	Scope          string     `json:"scope"`
	TotalQuestions int        `json:"total_questions"`
	MaxTimeSeconds int        `json:"max_time_seconds"`
	CreatedAt      *time.Time `json:"created_at"`
}

type TestResultListItem struct {
	ResultID       string     `json:"result_id"`
	TestID         string     `json:"test_id"`
	TestTitle      string     `json:"test_title"`
	TestScope      string     `json:"test_scope"`
	StartedAt      *time.Time `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at"`
	TotalScore     *float64   `json:"total_score"`
	TotalAttempted *int       `json:"total_attempted"`
	WrongCount     int        `json:"wrong_count"`
}

type ReviewQuestion struct {
	ID            string   `json:"id"`
	QuestionText  string   `json:"question_text"`
	Options       []string `json:"options"`
	CorrectIndex  int      `json:"correct_index"`
	SelectedIndex *int     `json:"selected_index"`
	IsCorrect     *bool    `json:"is_correct"`
	TimeTakenSec  *int      `json:"time_taken_sec"`
}

type AssignQuestionsInput struct {
	QuestionIDs []string `json:"question_ids" validate:"required" example:"aB3xK9mP,xK9mPaB3"`
}

func AssignQuestionsToTest(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetContext[middleware.AuthUser](r.Context(), middleware.AUTH_CONTEXT_KEY)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}
		testID := chi.URLParam(r, "test_id")
		if testID == "" {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}
		var err error
		var ownerId string
		var scope string

		err = pool.QueryRow(r.Context(),
			`SELECT owner_id, scope FROM tests WHERE id = $1`,
			testID,
		).Scan(&ownerId, &scope)
		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("assign_questions: owner check failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if scope == "PUBLIC" {
			if user.Role != "ADMIN" && user.Role != "MODERATOR" {
				utils.WriteError(w, apperror.ErrForbidden)
				return
			}
		} else {
			if ownerId != user.UserID {
				utils.WriteError(w, apperror.ErrForbidden)
				return
			}
		}
		var in AssignQuestionsInput
		if err := utils.DecodeJSON(r, &in); err != nil {
			utils.WriteError(w, *err)
			return
		}
		ctx := r.Context()
		tx, err := pool.Begin(ctx)
		if err != nil {
			slog.Error("assign_questions: tx begin failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx,
			`DELETE FROM test_questions WHERE test_id = $1`,
			testID,
		)
		if err != nil {
			slog.Error("assign_questions: delete failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		for i, qid := range in.QuestionIDs {
			_, err := tx.Exec(ctx, `
				INSERT INTO test_questions (test_id, question_id, position)
				VALUES ($1, $2, $3)
			`, testID, qid, i+1)
			if err != nil {
				slog.Error("assign_questions: insert failed", "error", err)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}
		}
		_, err = tx.Exec(ctx, `
			UPDATE tests
			SET total_questions = $2
			WHERE id = $1
		`, testID, len(in.QuestionIDs))
		if err != nil {
			slog.Error("assign_questions: update failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			slog.Error("assign_questions: commit failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		utils.WriteSuccess(w, http.StatusOK, map[string]any{
			"test_id":         testID,
			"total_questions": len(in.QuestionIDs),
		})
	}
}

func ListTests(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](r.Context(), middleware.AUTH_CONTEXT_KEY)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}
		rows, err := pool.Query(r.Context(), `
		SELECT id, course_id, title, description, scope,
		       total_questions, max_time_seconds, created_at
		FROM tests
		WHERE owner_id = $1
		ORDER BY created_at ASC
	`, user.UserID)
		if err != nil {
			slog.Error("list_tests: query failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()

		tests := make([]Test, 0)
		for rows.Next() {
			var t Test
			if err := rows.Scan(
				&t.ID,
				&t.CourseID,
				&t.Title,
				&t.Description,
				&t.Scope,
				&t.TotalQuestions,
				&t.MaxTimeSeconds,
				&t.CreatedAt,
			); err != nil {
				slog.Error("list_tests: scan failed", "error", err)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}
			tests = append(tests, t)
		}
		utils.WriteSuccess(w, http.StatusOK, tests)
	}
}

type CreateTestRequest struct {
	CourseID    string  `json:"course_id" validate:"required" example:"123e4567"`
	Title       string  `json:"title" validate:"required,min=3,max=100" example:"Test Title"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=250" example:"This is test description"`
	Scope       string  `json:"scope" validate:"required,oneof=PRIVATE PUBLIC"`
}

func CreateTest(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetContext[middleware.AuthUser](r.Context(), middleware.AUTH_CONTEXT_KEY)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}
		var req CreateTestRequest
		if errDef := utils.DecodeJSON(r, &req); errDef != nil {
			utils.WriteError(w, *errDef)
			return
		}
		if req.Scope == "PUBLIC" {
			if user.Role != "ADMIN" && user.Role != "MODERATOR" {
				utils.WriteError(w, apperror.ErrForbidden)
				return
			}
		}
		var courseOwnerID string
		err := pool.QueryRow(
			r.Context(),
			`SELECT owner_id FROM courses WHERE id = $1`,
			req.CourseID,
		).Scan(&courseOwnerID)
		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("create_test: owner check failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if courseOwnerID != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}
		testID := utils.NewID()
		now := time.Now().UTC()
		_, err = pool.Exec(r.Context(), `
		INSERT INTO tests
		(id, course_id, owner_id, title, description, scope,
		 total_questions, max_time_seconds, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
			testID,
			req.CourseID,
			user.UserID,
			req.Title,
			req.Description,
			req.Scope,
			DEFAULT_TEST_QUESTIONS,
			DEFAULT_TEST_MAX_TIME,
			now,
		)

		if err != nil {
			slog.Error("create_test: insert failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		utils.WriteSuccess(w, http.StatusCreated, Test{
			ID:             testID,
			CourseID:       req.CourseID,
			Title:          req.Title,
			Description:    req.Description,
			Scope:          req.Scope,
			TotalQuestions: 0,
			MaxTimeSeconds: DEFAULT_TEST_MAX_TIME,
			CreatedAt:      &now,
		})
	}
}

func ListTestsByCourse(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](r.Context(), middleware.AUTH_CONTEXT_KEY)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		courseID := chi.URLParam(r, "course_id")
		if courseID == "" {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}
		var course Course
		err := pool.QueryRow(
			r.Context(),
			`SELECT id, name, description, icon_key, owner_id, created_at
					 FROM courses
					 WHERE id = $1`,
			courseID,
		).Scan(
			&course.ID,
			&course.Name,
			&course.Description,
			&course.IconKey,
			&course.OwnerID,
			&course.CreatedAt,
		)

		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("list_tests_by_course: owner check failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if course.OwnerID != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		rows, err := pool.Query(
			r.Context(),
			`SELECT id, course_id, title, description, scope, total_questions, max_time_seconds, created_at
		 FROM tests
		 WHERE course_id = $1 AND owner_id = $2
		 ORDER BY created_at ASC`,
			courseID, user.UserID,
		)
		if err != nil {
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()

		tests := make([]Test, 0)
		for rows.Next() {
			var t Test
			if err := rows.Scan(
				&t.ID,
				&t.CourseID,
				&t.Title,
				&t.Description,
				&t.Scope,
				&t.TotalQuestions,
				&t.MaxTimeSeconds,
				&t.CreatedAt,
			); err != nil {
				utils.WriteError(w, apperror.ErrInternal)
				return
			}
			tests = append(tests, t)
		}
		response := map[string]interface{}{
			"course": course,
			"tests":  tests,
		}
		utils.WriteSuccess(w, http.StatusOK, response)
	}
}

type UpdateTestRequest struct {
	Title          string  `json:"title" validate:"required,min=3,max=100" example:"Updated Test Title"`
	Description    *string `json:"description,omitempty" validate:"omitempty,max=250" example:"Update test description"`
	MaxTimeSeconds int     `json:"max_time_seconds" validate:"required,min=60" example:"9000"`
}

func UpdateTestMetadata(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](r.Context(), middleware.AUTH_CONTEXT_KEY)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}
		testID := chi.URLParam(r, "test_id")
		if testID == "" {
			utils.WriteErrorWithMessage(w, apperror.ErrBadRequest, "Test id required")
			return
		}
		var req UpdateTestRequest
		if err := utils.DecodeJSON(r, &req); err != nil {
			utils.WriteError(w, *err)
			return
		}

		var ownerID string
		var scope string

		err := pool.QueryRow(
			r.Context(),
			`SELECT owner_id, scope FROM tests WHERE id = $1`,
			testID,
		).Scan(&ownerID, &scope)

		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}

		if err != nil {
			slog.Error("update_test: lookup failed",
				"test_id", testID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		if scope == "PUBLIC" {
			if user.Role != "ADMIN" && user.Role != "MODERATOR" {
				utils.WriteError(w, apperror.ErrForbidden)
				return
			}
		} else {
			if ownerID != user.UserID {
				utils.WriteError(w, apperror.ErrForbidden)
				return
			}
		}

		cmd, err := pool.Exec(r.Context(), `
			UPDATE tests
			SET title = $1,
   				description = $2,
			    max_time_seconds = $3
			WHERE id = $4
		`, req.Title,
			req.Description,
			req.MaxTimeSeconds,
			testID,
		)
		if err != nil {
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if cmd.RowsAffected() == 0 {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		utils.WriteSuccess(w, http.StatusOK, map[string]string{
			"message": "test updated",
		})
	}
}

func GetTestById(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetContext[middleware.AuthUser](r.Context(), middleware.AUTH_CONTEXT_KEY)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}
		var err error
		testID := chi.URLParam(r, "test_id")
		if testID == "" {
			utils.WriteErrorWithMessage(w, apperror.ErrBadRequest, "Test id required")
			return
		}
		var t Test
		var ownerId string

		err = pool.QueryRow(r.Context(), `
		SELECT id, course_id, owner_id, title, description, scope,
		       total_questions, max_time_seconds, created_at
		FROM tests
		WHERE id = $1
	`, testID).Scan(
			&t.ID,
			&t.CourseID,
			&ownerId,
			&t.Title,
			&t.Description,
			&t.Scope,
			&t.TotalQuestions,
			&t.MaxTimeSeconds,
			&t.CreatedAt,
		)
		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if t.Scope == "PRIVATE" && ownerId != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}
		utils.WriteSuccess(w, http.StatusOK, t)
	}
}

func StartTest(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}
		testID := chi.URLParam(r, "test_id")
		if testID == "" {
			utils.WriteErrorWithMessage(w, apperror.ErrBadRequest, "test id required")
			return
		}
		var duration int
		var ownerId string
		var scope string

		err := pool.QueryRow(
			r.Context(),
			`
			SELECT max_time_seconds, owner_id, scope
			FROM tests
			WHERE id = $1
			`,
			testID,
		).Scan(&duration, &ownerId, &scope)
		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("start_test: query failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if scope == "PRIVATE" && ownerId != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}
		var questionCount int
		err = pool.QueryRow(
			r.Context(),
			`
			SELECT COUNT(*)
			FROM test_questions
			WHERE test_id = $1
			`,
			testID,
		).Scan(&questionCount)

		if err != nil {
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if questionCount == 0 {
			utils.WriteErrorWithMessage(w, apperror.ErrBadRequest, "Cannot start empty test")
			return
		}
		var activeResultID string
		err = pool.QueryRow(r.Context(),
			`
					SELECT id FROM test_results
					WHERE test_id = $1 AND owner_id = $2
					AND ended_at IS NULL AND max_end_at > NOW()
					LIMIT 1
			`,
			testID,
			user.UserID,
		).Scan(&activeResultID)
		if err != nil && err != pgx.ErrNoRows {
			slog.Error("start_test: active session check failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if activeResultID != "" {
			utils.WriteErrorWithMessage(w, apperror.ErrConflict, "An active session is already running, please end it before starting a new one")
			return
		}
		resultID := utils.NewID()
		startTime := time.Now().UTC()
		endTime := startTime.Add(time.Duration(duration) * time.Second)
		_, err = pool.Exec(r.Context(), `
			INSERT INTO test_results (id, test_id, owner_id, started_at, max_end_at)
			VALUES ($1, $2, $3, $4, $5)
		`, resultID, testID, user.UserID, startTime, endTime)
		if err != nil {
			slog.Error("start_test: insert failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		utils.WriteSuccess(w, http.StatusCreated, map[string]string{
			"test_result_id": resultID,
		})
	}
}

type QuestionResponse struct {
	ID           string   `json:"id" example:"aB3xK9mP"`
	QuestionText string   `json:"question_text" example:"What is 2 + 2?"`
	Options      []string `json:"options" example:"2,3,4,5"`
	Difficulty   string   `json:"difficulty" example:"easy"`
	Explanation  *string  `json:"explanation,omitempty" example:"Basic arithmetic"`
	HasLatex     bool     `json:"has_latex" example:"false"`
	DiagramURL   *string  `json:"diagram_url,omitempty" example:"false"`
	ContentHash  string   `json:"content_hash" example:"b48922ecd8a1590f08ce412a35a72480"`
	Tags         []string `json:"tags" example:"math,arithmetic"`
}

func GetTestQuestions(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}
		testID := chi.URLParam(r, "test_id")
		if testID == "" {
			utils.WriteErrorWithMessage(w, apperror.ErrBadRequest, "test id required")
			return
		}

		var ownerId string
		var scope string

		err := pool.QueryRow(
			r.Context(),
			`SELECT owner_id, scope FROM tests WHERE id = $1`,
			testID,
		).Scan(&ownerId, &scope)
		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("get_test_questions: owner check failed",
				"test_id", testID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		if scope == "PRIVATE" && ownerId != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		rows, err := pool.Query(r.Context(), `
			SELECT
				q.id,
				q.question_text,
				q.options,
				q.difficulty,
				q.explanation,
				q.has_latex,
				q.diagram_url,
				q.content_hash,
				COALESCE(
					json_agg(t.name) FILTER (WHERE t.name IS NOT NULL),
					'[]'
				) AS tags
			FROM test_questions tq
			JOIN questions q ON q.id = tq.question_id
			LEFT JOIN question_tags qt ON qt.question_id = q.id
			LEFT JOIN tags t ON t.id = qt.tag_id
			WHERE tq.test_id = $1
			GROUP BY q.id, tq.position
			ORDER BY tq.position ASC
		`, testID)

		if err != nil {
			slog.Error("get_test_questions: query failed",
				"test_id", testID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()

		questions := make([]QuestionResponse, 0)
		for rows.Next() {
			var q QuestionResponse
			if err := rows.Scan(
				&q.ID,
				&q.QuestionText,
				&q.Options,
				&q.Difficulty,
				&q.Explanation,
				&q.HasLatex,
				&q.DiagramURL,
				&q.ContentHash,
				&q.Tags,
			); err != nil {
				slog.Error("get_test_questions: scan failed",
					"test_id", testID,
					"error", err,
				)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}
			questions = append(questions, q)
		}
		if err := rows.Err(); err != nil {
			slog.Error("get_test_questions: rows error",
				"test_id", testID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		utils.WriteSuccess(w, http.StatusOK, map[string]any{
			"test_id":         testID,
			"total_questions": len(questions),
			"questions":       questions,
		})
	}
}

type SaveAttemptInput struct {
	QuestionID    string `json:"question_id"`
	SelectedIndex *int   `json:"selected_index"`
	TimeTakenSec  int    `json:"time_taken_sec"`
}

func SaveQuestionAttempt(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		resultID := chi.URLParam(r, "result_id")
		if resultID == "" {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		var in SaveAttemptInput
		if err := utils.DecodeJSON(r, &in); err != nil {
			utils.WriteError(w, *err)
			return
		}

		ctx := r.Context()

		var (
			userId   string
			maxEndAt time.Time
			endedAt  *time.Time
		)

		err := pool.QueryRow(ctx, `
				SELECT owner_id, max_end_at, ended_at
				FROM test_results
				WHERE id = $1
		`, resultID).Scan(&userId, &maxEndAt, &endedAt)
		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("save_attempt: result lookup failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		if userId != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		// test already submitted
		if endedAt != nil {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		//  test time expired
		if time.Now().UTC().After(maxEndAt) {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		var exists bool

		err = pool.QueryRow(ctx, `
				SELECT EXISTS(
					SELECT 1
					FROM test_questions tq
					JOIN test_results tr ON tr.test_id = tq.test_id
					WHERE tr.id = $1 AND tq.question_id = $2
				)
			`, resultID, in.QuestionID).Scan(&exists)

		if err != nil {
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if !exists {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		var isCorrect *bool

		//  correctness
		if in.SelectedIndex != nil {

			var correctIndex int

			err := pool.QueryRow(
				r.Context(),
				`SELECT correct_index FROM questions WHERE id = $1`,
				in.QuestionID,
			).Scan(&correctIndex)

			if err != nil {
				utils.WriteError(w, apperror.ErrInternal)
				return
			}

			v := correctIndex == *in.SelectedIndex

			isCorrect = &v

		}

		_, err = pool.Exec(ctx, `
					INSERT INTO question_attempts (
						id,
						question_id,
						test_result_id,
						selected_index,
						is_correct,
						time_taken_sec,
						attempted_at
					)
					VALUES ($1,$2,$3,$4,$5,$6,now())
					ON CONFLICT (test_result_id, question_id)
					DO UPDATE SET
						selected_index = EXCLUDED.selected_index,
						is_correct = EXCLUDED.is_correct,
						time_taken_sec = EXCLUDED.time_taken_sec,
						attempted_at = now()
				`,
			utils.NewID(),
			in.QuestionID,
			resultID,
			in.SelectedIndex,
			isCorrect,
			in.TimeTakenSec,
		)

		if err != nil {
			slog.Error("save_attempt: upsert failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		utils.WriteSuccess(w, http.StatusOK, map[string]string{
			"message": "attempt saved",
		})

	}
}

func SubmitTest(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		resultId := chi.URLParam(r, "result_id")
		if resultId == "" {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		ctx := r.Context()

		tx, err := pool.Begin(ctx)
		if err != nil {
			slog.Error("failed to begin transaction",
				slog.String("result_id", resultId),
				slog.Any("error", err),
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		defer tx.Rollback(ctx)

		var (
			userId   string
			maxEndAt time.Time
			endedAt  *time.Time
		)

		//  Lock row to prevent double submission race
		err = tx.QueryRow(ctx, `
			SELECT owner_id, max_end_at, ended_at
			FROM test_results
			WHERE id = $1
			FOR UPDATE
		`, resultId).Scan(&userId, &maxEndAt, &endedAt)

		if err == pgx.ErrNoRows {
			slog.Warn("submit attempted for non-existent result",
				slog.String("result_id", resultId),
				slog.String("user_id", user.UserID),
			)
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}

		if err != nil {
			slog.Error("failed fetching test result",
				slog.String("result_id", resultId),
				slog.Any("error", err),
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		//  Ownership check
		if userId != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		//  already submitted
		if endedAt != nil {
			utils.WriteError(w, apperror.ErrConflict)
			return
		}

		//  If time expired, still allow submit but clamp end time
		now := time.Now().UTC()
		if now.After(maxEndAt) {
			now = maxEndAt
		}

		// mark the test as ended
		_, err = tx.Exec(ctx, `
			UPDATE test_results
			SET
				ended_at = $2
			WHERE id = $1
		`,
			resultId,
			now,
		)
		if err != nil {
			slog.Error("failed updating test_results ended_at",
				slog.String("result_id", resultId),
				slog.Any("error", err),
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		// Calculate and store the result summary
		if err := calculateAndUpdateTestResult(ctx, tx, resultId); err != nil {
			slog.Error("failed to calculate test result",
				slog.String("result_id", resultId),
				slog.Any("error", err),
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			slog.Error("transaction commit failed during submit",
				slog.String("result_id", resultId),
				slog.Any("error", err),
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		utils.WriteSuccess(w, http.StatusOK, map[string]any{
			"result_id": resultId,
			"ended_at":  now,
		})
	}
}

type TestResultResponse struct {
	Test struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"test"`
	Result struct {
		TotalScore     float64    `json:"total_score"`
		TotalAttempted int        `json:"total_attempted"`
		TotalSkipped   int        `json:"total_skipped"`
		WrongCount     int        `json:"wrong_count"`
		StartedAt      *time.Time `json:"started_at"`
		EndedAt        *time.Time `json:"ended_at"`
	} `json:"result"`
}

func GetTestResult(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetContext[middleware.AuthUser](r.Context(), middleware.AUTH_CONTEXT_KEY)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		resultID := chi.URLParam(r, "result_id")
		if resultID == "" {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		var res TestResultResponse
		var ownerID string

		// NOTE: schema column is owner_id (not user_id).
		err := pool.QueryRow(r.Context(), `
			SELECT
				t.id,
				t.title,
				tr.owner_id,
				tr.total_score,
				tr.total_attempted,
				tr.total_skipped,
				tr.wrong_count,
				tr.started_at,
				tr.ended_at
			FROM  test_results tr
			JOIN  tests t ON t.id = tr.test_id
			WHERE tr.id = $1
		`, resultID).Scan(
			&res.Test.ID,
			&res.Test.Title,
			&ownerID,
			&res.Result.TotalScore,
			&res.Result.TotalAttempted,
			&res.Result.TotalSkipped,
			&res.Result.WrongCount,
			&res.Result.StartedAt,
			&res.Result.EndedAt,
		)
		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("get_test_result: query", "result_id", resultID, "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if ownerID != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		utils.WriteSuccess(w, http.StatusOK, res)
	}
}

func GetTestTimeSync(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		resultID := chi.URLParam(r, "result_id")
		if resultID == "" {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		var (
			userID    string
			testID    string
			startedAt time.Time
			maxEndAt  time.Time
			serverNow time.Time
			endedAt   *time.Time
		)

		err := pool.QueryRow(r.Context(),
			`SELECT test_id, owner_id, started_at, ended_at, max_end_at, NOW() FROM test_results WHERE id = $1`, resultID,
		).Scan(&testID, &userID, &startedAt, &endedAt, &maxEndAt, &serverNow)

		if err != nil {
			slog.Error("get_test_results: query failed", "result_id", resultID, "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		if userID != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		remaining := 0

		if endedAt == nil {
			if serverNow.Before(maxEndAt) {
				remaining = int(maxEndAt.Sub(serverNow).Seconds())
			}
		}

		resp := map[string]interface{}{
			"attempt_id":    resultID,
			"exam_started":  startedAt,
			"exam_ends_at":  maxEndAt,
			"server_time":   serverNow,
			"remaining_sec": remaining,
		}
		utils.WriteSuccess(w, http.StatusOK, resp)
	}
}

type ActiveTestSession struct {
	ResultID     string    `json:"result_id"`
	TestID       string    `json:"test_id"`
	TestTitle    string    `json:"test_title"`
	StartedAt    time.Time `json:"started_at"`
	MaxEndAt     time.Time `json:"max_end_at"`
	RemainingSec int       `json:"remaining_sec"`
}

func ListActiveTestSessions(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		rows, err := pool.Query(r.Context(), `
			SELECT
				tr.id,
				tr.test_id,
				t.title,
				tr.started_at,
				tr.max_end_at,
				GREATEST(
					EXTRACT(EPOCH FROM (tr.max_end_at - NOW())),
					0
				)::int AS remaining_sec
			FROM test_results tr
			JOIN tests t ON t.id = tr.test_id
			WHERE tr.owner_id = $1
			  AND tr.ended_at IS NULL
			  AND tr.max_end_at > NOW()
			ORDER BY tr.started_at DESC
		`, user.UserID)

		if err != nil {
			slog.Error("list_active_sessions: query failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()

		sessions := make([]ActiveTestSession, 0)

		for rows.Next() {
			var s ActiveTestSession
			if err := rows.Scan(
				&s.ResultID,
				&s.TestID,
				&s.TestTitle,
				&s.StartedAt,
				&s.MaxEndAt,
				&s.RemainingSec,
			); err != nil {
				slog.Error("list_active_sessions: scan failed", "error", err)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}
			sessions = append(sessions, s)
		}

		utils.WriteSuccess(w, http.StatusOK, map[string]any{
			"total":    len(sessions),
			"sessions": sessions,
		})
	}
}

func testExists(ctx context.Context, pool *pgxpool.Pool, courseID string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
	SELECT EXISTS(SELECT 1 FROM tests WHERE id = $1)
`, courseID).Scan(&exists)
	return exists, err
}

func calculateAndUpdateTestResult(ctx context.Context, tx pgx.Tx, resultID string) error {
	// Get the test_id and total questions for that test
	var testID string
	var totalQuestions int
	err := tx.QueryRow(ctx, `
		SELECT tr.test_id, COUNT(tq.question_id)
		FROM test_results tr
		JOIN test_questions tq ON tr.test_id = tq.test_id
		WHERE tr.id = $1
		GROUP BY tr.test_id
	`, resultID).Scan(&testID, &totalQuestions)
	if err != nil {
		return err
	}

	// Aggregate attempt statistics
	var attempted, correct, wrong int
	err = tx.QueryRow(ctx, `
		SELECT
			COUNT(*) AS attempted,
			COUNT(*) FILTER (WHERE is_correct = true) AS correct,
			COUNT(*) FILTER (WHERE is_correct = false) AS wrong
		FROM question_attempts
		WHERE test_result_id = $1
	`, resultID).Scan(&attempted, &correct, &wrong)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	// If no attempts, the scan leaves variables at zero (default)

	skipped := totalQuestions - attempted
	score := float64(correct) // each correct = 1 point

	// Update the test_results row with the computed values
	_, err = tx.Exec(ctx, `
		UPDATE test_results
		SET
			total_attempted = $2,
			total_skipped = $3,
			wrong_count = $4,
			total_score = $5
		WHERE id = $1
	`, resultID, attempted, skipped, wrong, score)
	if err != nil {
		return err
	}

	return nil
}

func GetTestReview(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		resultID := chi.URLParam(r, "result_id")
		if resultID == "" {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		ctx := r.Context()

		var (
			testID  string
			userID  string
			endedAt *time.Time
		)

		err := pool.QueryRow(ctx, `
			SELECT test_id, owner_id, ended_at
			FROM test_results
			WHERE id = $1
		`, resultID).Scan(&testID, &userID, &endedAt)

		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}

		if err != nil {
			slog.Error("review: lookup failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		if userID != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		if endedAt == nil {
			utils.WriteErrorWithMessage(
				w,
				apperror.ErrForbidden,
				"Test must be submitted before viewing review",
			)
			return
		}

		rows, err := pool.Query(ctx, `
			SELECT
				q.id,
				q.question_text,
				q.options,
				q.correct_index,
				qa.selected_index,
				qa.is_correct,
				qa.time_taken_sec
			FROM test_questions tq
			JOIN questions q
				ON q.id = tq.question_id
			LEFT JOIN question_attempts qa
				ON qa.question_id = q.id
				AND qa.test_result_id = $1
			WHERE tq.test_id = $2
			ORDER BY tq.position
		`, resultID, testID)

		if err != nil {
			slog.Error("review query failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()

		questions := make([]ReviewQuestion, 0)

		for rows.Next() {
			var q ReviewQuestion

			err := rows.Scan(
				&q.ID,
				&q.QuestionText,
				&q.Options,
				&q.CorrectIndex,
				&q.SelectedIndex,
				&q.IsCorrect,
				&q.TimeTakenSec,
			)

			if err != nil {
				slog.Error("review scan failed", "error", err)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}

			questions = append(questions, q)
		}

		utils.WriteSuccess(w, http.StatusOK, map[string]any{
			"result_id": resultID,
			"questions": questions,
		})
	}
}

func ListUserTestResults(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		rows, err := pool.Query(r.Context(), `
			SELECT
				tr.id,
				tr.test_id,
				t.title,
				t.scope,
				tr.started_at,
				tr.ended_at,
				tr.total_score,
				tr.total_attempted,
				tr.wrong_count
			FROM test_results tr
			JOIN tests t ON t.id = tr.test_id
			WHERE tr.owner_id = $1
			  AND tr.ended_at IS NOT NULL
			ORDER BY tr.ended_at DESC
		`, user.UserID)

		if err != nil {
			slog.Error("list_user_test_results: query failed",
				"user_id", user.UserID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()

		results := make([]TestResultListItem, 0)

		for rows.Next() {
			var r TestResultListItem

			if err := rows.Scan(
				&r.ResultID,
				&r.TestID,
				&r.TestTitle,
				&r.TestScope,
				&r.StartedAt,
				&r.EndedAt,
				&r.TotalScore,
				&r.TotalAttempted,
				&r.WrongCount,
			); err != nil {
				slog.Error("list_user_test_results: scan failed",
					"user_id", user.UserID,
					"error", err,
				)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}

			results = append(results, r)
		}

		if err := rows.Err(); err != nil {
			slog.Error("list_user_test_results: rows error",
				"user_id", user.UserID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		utils.WriteSuccess(w, http.StatusOK, map[string]any{
			"total":   len(results),
			"results": results,
		})
	}
}

// DELETE

func DeleteTest(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		testID := chi.URLParam(r, "test_id")
		if testID == "" {
			utils.WriteErrorWithMessage(w, apperror.ErrBadRequest, "test id required")
			return
		}

		var ownerId string
		var scope string

		err := pool.QueryRow(
			r.Context(),
			`SELECT owner_id, scope FROM tests WHERE id = $1`,
			testID,
		).Scan(&ownerId, &scope)
		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("delete_test: lookup failed",
				"test_id", testID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if scope == "PUBLIC" {
			if user.Role != "ADMIN" && user.Role != "MODERATOR" {
				utils.WriteError(w, apperror.ErrForbidden)
				return
			}
		} else {
			if ownerId != user.UserID {
				utils.WriteError(w, apperror.ErrForbidden)
				return
			}
		}
		cmd, err := pool.Exec(r.Context(), `
			DELETE FROM tests
			WHERE id = $1
		`, testID)

		if err != nil {
			slog.Error("delete_test: delete failed",
				"test_id", testID,
				"user_id", user.UserID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		if cmd.RowsAffected() == 0 {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}

		utils.WriteSuccess(w, http.StatusOK, map[string]string{
			"message": "test deleted successfully",
		})
	}
}

func ListPublicTests(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		rows, err := pool.Query(r.Context(), `
			SELECT id, course_id, title, description, scope,
			       total_questions, max_time_seconds, created_at
			FROM tests
			WHERE scope = 'PUBLIC'
			ORDER BY created_at DESC
		`)
		if err != nil {
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()

		tests := []Test{}

		for rows.Next() {
			var t Test

			if err := rows.Scan(
				&t.ID,
				&t.CourseID,
				&t.Title,
				&t.Description,
				&t.Scope,
				&t.TotalQuestions,
				&t.MaxTimeSeconds,
				&t.CreatedAt,
			); err != nil {
				utils.WriteError(w, apperror.ErrInternal)
				return
			}

			tests = append(tests, t)
		}

		if err := rows.Err(); err != nil {
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		utils.WriteSuccess(w, http.StatusOK, tests)
	}
}

// return user test attempt to reconstruct test page
func GetTestAttempts(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		resultID := chi.URLParam(r, "result_id")
		if resultID == "" {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		var ownerID string
		err := pool.QueryRow(r.Context(),
			`SELECT owner_id FROM test_results WHERE id = $1`,
			resultID,
		).Scan(&ownerID)

		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		if ownerID != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		rows, err := pool.Query(r.Context(), `
			SELECT question_id, selected_index
			FROM question_attempts
			WHERE test_result_id = $1
		`, resultID)

		if err != nil {
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()

		attempts := make(map[string]*int)

		for rows.Next() {
			var qid string
			var selected *int

			if err := rows.Scan(&qid, &selected); err != nil {
				utils.WriteError(w, apperror.ErrInternal)
				return
			}

			attempts[qid] = selected
		}

		utils.WriteSuccess(w, http.StatusOK, map[string]any{
			"result_id": resultID,
			"attempts":  attempts,
		})
	}
}
