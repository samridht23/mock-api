package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samridht23/mock-api/internal/apperror"
	"github.com/samridht23/mock-api/internal/middleware"
	"github.com/samridht23/mock-api/internal/utils"
)

type Course struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	OwnerID     string     `json:"owner_id"`
	Description *string    `json:"description"`
	IconKey     string     `json:"icon_key"`
	CreatedAt   *time.Time `json:"created_at"`
}

type CreateCourseRequest struct {
	CourseName        string  `json:"name"`
	CourseDescription *string `json:"description"`
	CourseIconKey     string  `json:"icon_key"`
}

func ListCourses(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		user, ok := middleware.GetContext[middleware.AuthUser](ctx, middleware.AUTH_CONTEXT_KEY)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}
		rows, err := pool.Query(ctx, `
			SELECT id, name, description, icon_key, created_at
			FROM courses
			WHERE owner_id = $1
			ORDER BY created_at ASC
		`, user.UserID)
		if err != nil {
			slog.Error("list_courses: query failed",
				"user_id", user.UserID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		defer rows.Close()
		courses := make([]Course, 0)
		for rows.Next() {
			var c Course
			if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.IconKey, &c.CreatedAt); err != nil {
				slog.Error("list_courses: scan failed",
					"user_id", user.UserID,
					"error", err,
				)
				utils.WriteError(w, apperror.ErrInternal)
				return
			}
			courses = append(courses, c)
		}
		if err := rows.Err(); err != nil {
			slog.Error("list_courses: rows error",
				"user_id", user.UserID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}
		utils.WriteSuccess(w, http.StatusOK, courses)
	}
}

func CreateCourse(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](r.Context(), middleware.AUTH_CONTEXT_KEY)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		var req CreateCourseRequest

		if err := utils.DecodeJSON(r, &req); err != nil {
			slog.Warn("create_course: invalid json", "error", err)
			utils.WriteError(w, apperror.ErrInvalidJSON)
			return
		}

		if req.CourseName == "" {
			utils.WriteErrorWithMessage(w, apperror.ErrValidation, "Course name is required")
			return
		}

		allowedIcons := map[string]bool{
			"book": true, "sigma": true, "atom": true, "brain": true,
			"globe": true, "landmark": true, "calculator": true,
			"flask": true, "pen": true, "bulb": true,
		}

		iconKey := req.CourseIconKey
		if iconKey == "" {
			iconKey = "book"
		} else if !allowedIcons[iconKey] {
			utils.WriteErrorWithMessage(w, apperror.ErrValidation, "Invalid course icon")
			return
		}

		id := utils.NewID()
		now := time.Now().UTC()

		_, err := pool.Exec(r.Context(), `
			INSERT INTO courses (id, owner_id, name, description, icon_key, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, id, user.UserID, req.CourseName, req.CourseDescription, iconKey, now)

		if err != nil {
			slog.Error("create_course: insert failed",
				"user_id", user.UserID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		utils.WriteSuccess(w, http.StatusCreated, Course{
			ID:          id,
			Name:        req.CourseName,
			Description: req.CourseDescription,
			IconKey:     iconKey,
			CreatedAt:   &now,
		})
	}
}

type UpdateCourseRequest struct {
	CourseName        *string `json:"name"`
	CourseDescription *string `json:"description"`
	CourseIconKey     *string `json:"icon_key"`
}

func UpdateCourse(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](r.Context(), middleware.AUTH_CONTEXT_KEY)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		courseID := chi.URLParam(r, "id")
		if courseID == "" {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		var req UpdateCourseRequest

		if err := utils.DecodeJSON(r, &req); err != nil {
			utils.WriteError(w, apperror.ErrInvalidJSON)
			return
		}

		if req.CourseName == nil &&
			req.CourseDescription == nil &&
			req.CourseIconKey == nil {
			utils.WriteErrorWithMessage(w, apperror.ErrBadRequest, "Nothing to update")
			return
		}

		tag, err := pool.Exec(r.Context(), `
			UPDATE courses
			SET name = COALESCE($3, name),
			    description = COALESCE($4, description),
			    icon_key = COALESCE($5, icon_key)
			WHERE id = $1 AND owner_id = $2
		`,
			courseID,
			user.UserID,
			req.CourseName,
			req.CourseDescription,
			req.CourseIconKey,
		)

		if err != nil {
			slog.Error("update_course: update failed",
				"user_id", user.UserID,
				"course_id", courseID,
				"error", err,
			)
			utils.WriteError(w, apperror.ErrInternal)
			return
		}

		if tag.RowsAffected() == 0 {
			utils.WriteError(w, apperror.ErrNotFound)
			return
		}

		utils.WriteSuccess(w, http.StatusOK, map[string]string{
			"message": "Course updated successfully",
		})
	}
}

func DeleteCourse(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, ok := middleware.GetContext[middleware.AuthUser](
			r.Context(),
			middleware.AUTH_CONTEXT_KEY,
		)
		if !ok || user == nil {
			utils.WriteError(w, apperror.ErrUnauthorized)
			return
		}

		courseID := chi.URLParam(r, "id")
		if courseID == "" {
			utils.WriteError(w, apperror.ErrBadRequest)
			return
		}

		cmd, err := pool.Exec(r.Context(), `
			DELETE FROM courses
			WHERE id = $1 AND owner_id = $2
		`, courseID, user.UserID)

		if err != nil {
			slog.Error("delete_course: delete failed",
				"user_id", user.UserID,
				"course_id", courseID,
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
			"message": "Course deleted successfully",
		})
	}
}
