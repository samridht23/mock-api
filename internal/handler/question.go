package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samridht23/mock-api/internal/apperror"
	"github.com/samridht23/mock-api/internal/middleware"
	"github.com/samridht23/mock-api/internal/utils"
)

type ImportQuestionInput struct {
	QuestionText string   `json:"question_text"`
	Options      []string `json:"options"`
	CorrectIndex int      `json:"correct_index"`
	Difficulty   string   `json:"difficulty"`
	Tags         []string `json:"tags"`
	Explanation  *string  `json:"explanation"`
	HasLatex     bool     `json:"has_latex"`
	DiagramURL   *string  `json:"diagram_url"`
}

type ImportQuestionsPayload struct {
	Questions []ImportQuestionInput `json:"questions"`
}

type Question struct {
	ID           string    `json:"id"`
	CourseID     string    `json:"course_id"`
	OwnerID      string    `json:"owner_id"`
	Scope        string    `json:"scope"`
	QuestionText string    `json:"question_text"`
	Options      []string  `json:"options"`
	CorrectIndex int       `json:"correct_index"`
	Explanation  *string   `json:"explanation"`
	HasLatex     *bool     `json:"has_latex"`
	DiagramUrl   *string   `json:"diagram_url"`
	Difficulty   string    `json:"difficulty"`
	ContentHash  string    `json:"content_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

func computeContentHash(q ImportQuestionInput) string {
	canonical := strings.TrimSpace(q.QuestionText) +
		"|" + strings.Join(q.Options, "|") +
		"|" + strconv.Itoa(q.CorrectIndex) +
		"|" + q.Difficulty

	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func validateQuestion(q ImportQuestionInput) error {
	if q.QuestionText == "" {
		return errors.New("question_text required")
	}
	if len(q.Options) != 4 {
		return errors.New("options must be array of 4")
	}
	if q.CorrectIndex < 0 || q.CorrectIndex > 3 {
		return errors.New("correct_index must be 0-3")
	}
	if q.Difficulty != "easy" && q.Difficulty != "medium" && q.Difficulty != "hard" {
		return errors.New("invalid difficulty")
	}
	return nil
}

func ListQuestionsByCourse(pool *pgxpool.Pool) http.HandlerFunc {
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
		var ownerID string
		err := pool.QueryRow(r.Context(),
			`SELECT owner_id FROM courses WHERE id=$1`,
			courseID,
		).Scan(&ownerID)
		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("list_questions_by_course: owner check failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if ownerID != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}
		rows, err := pool.Query(r.Context(), `
			SELECT
				q.id,
				q.course_id,
				q.owner_id,
				q.scope,
				q.question_text,
				q.options,
				q.correct_index,
				q.explanation,
				q.has_latex,
				q.diagram_url,
				q.difficulty,
				q.content_hash,
				q.created_at,
				COALESCE(json_agg(t.name) FILTER (WHERE t.name IS NOT NULL),'[]') AS tags
			FROM questions q
			LEFT JOIN question_tags qt ON qt.question_id = q.id
			LEFT JOIN tags t ON t.id = qt.tag_id
			WHERE q.course_id = $1
			GROUP BY q.id
			ORDER BY q.created_at DESC
		`, courseID)
		if err != nil {
			slog.Error("list_questions_by_course: query failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()
		type response struct {
			Question
			Tags []string `json:"tags"`
		}
		questions := make([]response, 0)
		for rows.Next() {
			var q response
			if err := rows.Scan(
				&q.ID,
				&q.CourseID,
				&q.OwnerID,
				&q.Scope,
				&q.QuestionText,
				&q.Options,
				&q.CorrectIndex,
				&q.Explanation,
				&q.HasLatex,
				&q.DiagramUrl,
				&q.Difficulty,
				&q.ContentHash,
				&q.CreatedAt,
				&q.Tags,
			); err != nil {
				slog.Error("list_questions_by_course: scan failed", "error", err)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}
			questions = append(questions, q)
		}
		utils.WriteSuccess(w, http.StatusOK, questions)
	}
}

func ListAllQuestions(pool *pgxpool.Pool) http.HandlerFunc {
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
				q.id,
				q.question_text,
				q.difficulty,
				q.created_at,
				q.course_id,
				c.name,
				COALESCE(
					json_agg(t.name) FILTER (WHERE t.name IS NOT NULL),
					'[]'
				)
			FROM questions q
			JOIN courses c ON c.id = q.course_id
			LEFT JOIN question_tags qt ON qt.question_id = q.id
			LEFT JOIN tags t ON t.id = qt.tag_id
			WHERE q.owner_id = $1
			GROUP BY q.id, c.name
			ORDER BY q.created_at DESC
		`, user.UserID)

		if err != nil {
			slog.Error("list_all_questions: query failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()

		type QuestionResponse struct {
			ID           string    `json:"id"`
			QuestionText string    `json:"question_text"`
			Difficulty   string    `json:"difficulty"`
			CreatedAt    time.Time `json:"created_at"`
			CourseID     string    `json:"course_id"`
			CourseName   string    `json:"course_name"`
			Tags         []string  `json:"tags"`
		}

		out := make([]QuestionResponse, 0)

		for rows.Next() {
			var q QuestionResponse

			if err := rows.Scan(
				&q.ID,
				&q.QuestionText,
				&q.Difficulty,
				&q.CreatedAt,
				&q.CourseID,
				&q.CourseName,
				&q.Tags,
			); err != nil {
				slog.Error("list_all_questions: scan failed", "error", err)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}

			out = append(out, q)
		}

		utils.WriteSuccess(w, http.StatusOK, out)
	}
}

func ImportQuestions(pool *pgxpool.Pool) http.HandlerFunc {
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

		var ownerID string
		err := pool.QueryRow(r.Context(),
			`SELECT owner_id FROM courses WHERE id=$1`,
			courseID,
		).Scan(&ownerID)

		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("import_questions: owner check failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		if ownerID != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		var payload ImportQuestionsPayload
		if errDef := utils.DecodeJSON(r, &payload); errDef != nil {
			utils.WriteError(w, *errDef)
			return
		}

		if len(payload.Questions) == 0 {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		ctx := r.Context()

		tx, err := pool.Begin(ctx)
		if err != nil {
			slog.Error("import_questions: tx begin failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer tx.Rollback(ctx)

		inserted := 0
		skipped := 0

		for i, q := range payload.Questions {

			if err := validateQuestion(q); err != nil {
				utils.WriteError(w, apperror.ErrValidation)
				return
			}

			hash := computeContentHash(q)
			qID := utils.NewID()

			cmd, err := tx.Exec(ctx, `
				INSERT INTO questions
				(id, course_id, owner_id, scope,
				 question_text, options, correct_index,
				 explanation, has_latex, diagram_url,
				 difficulty, content_hash, created_at)
				VALUES ($1,$2,$3,'PRIVATE',$4,$5,$6,$7,$8,$9,$10,$11,now())
				ON CONFLICT (course_id, content_hash) DO NOTHING
			`,
				qID, courseID, user.UserID,
				q.QuestionText, q.Options, q.CorrectIndex,
				q.Explanation, q.HasLatex, q.DiagramURL,
				q.Difficulty, hash,
			)

			if err != nil {
				slog.Error("import_questions: insert failed", "index", i, "error", err)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}

			if cmd.RowsAffected() == 0 {
				skipped++
				continue
			}

			inserted++

			for _, tagName := range q.Tags {

				tagName = strings.TrimSpace(tagName)
				if tagName == "" {
					continue
				}

				var tagID string

				err := tx.QueryRow(ctx, `
					SELECT id FROM tags
					WHERE course_id = $1 AND name = $2
				`, courseID, tagName).Scan(&tagID)

				if err == pgx.ErrNoRows {
					tagID = utils.NewID()
					_, err = tx.Exec(ctx, `
						INSERT INTO tags (id, course_id, name)
						VALUES ($1,$2,$3)
					`, tagID, courseID, tagName)

					if err != nil {
						slog.Error("import_questions: tag insert failed", "error", err)
						utils.WriteError(w, apperror.ErrInternal)
						return
					}

				} else if err != nil {
					slog.Error("import_questions: tag lookup failed", "error", err)
					utils.WriteError(w, apperror.ErrInternal)
					return
				}
				_, err = tx.Exec(ctx, `
					INSERT INTO question_tags (question_id, tag_id)
					VALUES ($1,$2)
					ON CONFLICT DO NOTHING
				`, qID, tagID)

				if err != nil {
					slog.Error("import_questions: question_tag insert failed", "error", err)
					utils.WriteError(w, apperror.ErrInternal)
					return
				}
			}
		}

		if err := tx.Commit(ctx); err != nil {
			slog.Error("import_questions: commit failed", "error", err)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		utils.WriteSuccess(w, http.StatusCreated, map[string]any{
			"course_id": courseID,
			"inserted":  inserted,
			"skipped":   skipped,
		})
	}
}

func ListQuestionsByTest(pool *pgxpool.Pool) http.HandlerFunc {
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
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		var ownerID string
		err := pool.QueryRow(
			r.Context(),
			`SELECT owner_id FROM tests WHERE id = $1`,
			testID,
		).Scan(&ownerID)

		if err == pgx.ErrNoRows {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}
		if err != nil {
			slog.Error("list_questions_by_test: owner check failed",
				"test_id", testID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		if ownerID != user.UserID {
			utils.WriteError(w, apperror.ErrForbidden)
			return
		}

		rows, err := pool.Query(r.Context(), `
			SELECT
				q.id,
				q.question_text,
				q.difficulty,
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
			slog.Error("list_questions_by_test: query failed",
				"test_id", testID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()

		type response struct {
			ID           string   `json:"id"`
			QuestionText string   `json:"question_text"`
			Difficulty   string   `json:"difficulty"`
			Tags         []string `json:"tags"`
		}

		var out []response

		for rows.Next() {
			var q response
			if err := rows.Scan(
				&q.ID,
				&q.QuestionText,
				&q.Difficulty,
				&q.Tags,
			); err != nil {
				slog.Error("list_questions_by_test: scan failed",
					"test_id", testID,
					"error", err,
				)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}
			out = append(out, q)
		}

		if err := rows.Err(); err != nil {
			slog.Error("list_questions_by_test: rows error",
				"test_id", testID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		utils.WriteSuccess(w, http.StatusOK, out)
	}
}
