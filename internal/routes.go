package internal

import (
	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samridht23/mock-api/internal/core"
	"github.com/samridht23/mock-api/internal/handler"
	"github.com/samridht23/mock-api/internal/middleware"
)

func InitRoutes(r chi.Router, conn *pgxpool.Pool, auth *core.AuthService, googleHTTP *core.GoogleHTTPService) {

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Group(func(r chi.Router) {
		r.Get("/auth/google", handler.GoogleLogin(auth))
		r.Get("/auth/google/callback", handler.GoogleCallback(auth, googleHTTP))
		r.Post("/auth/logout", handler.Logout())
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(auth))

		r.Get("/auth-status", handler.AuthStatus(auth))

		r.Route("/courses", func(r chi.Router) {
			r.Get("/", handler.ListCourses(conn))
			r.Post("/", handler.CreateCourse(conn))
			r.Put("/{id}", handler.UpdateCourse(conn))
			r.Get("/{course_id}/tests", handler.ListTestsByCourse(conn))
			r.Post("/{course_id}/questions/import", handler.ImportQuestions(conn))
			r.Get("/{course_id}/questions", handler.ListQuestionsByCourse(conn))

			// delete
			r.Delete("/{id}", handler.DeleteCourse(conn))
		})

		r.Route("/tests", func(r chi.Router) {
			r.Get("/", handler.ListTests(conn))
			r.Post("/", handler.CreateTest(conn))
			r.Get("/sessions/active", handler.ListActiveTestSessions(conn))
			r.Route("/{test_id}", func(r chi.Router) {
				r.Get("/", handler.GetTestById(conn))
				r.Put("/", handler.UpdateTestMetadata(conn))
				r.Post("/start", handler.StartTest(conn))
				r.Get("/questions", handler.GetTestQuestions(conn))
				r.Get("/questions/preview", handler.ListQuestionsByTest(conn))
				r.Post("/questions", handler.AssignQuestionsToTest(conn))

				// delete
				r.Delete("/", handler.DeleteTest(conn))

			})

			// list public tests
			r.Get("/public", handler.ListPublicTests(conn))

		})

		r.Route("/results", func(r chi.Router) {

			r.Get("/sync/{result_id}", handler.GetTestTimeSync(conn))
			r.Get("/{result_id}", handler.GetTestResult(conn))
			r.Get("/{result_id}/review", handler.GetTestReview(conn))
			r.Get("/history", handler.ListUserTestResults(conn))

			// return user test attempts to reconstruct
			r.Get("/{result_id}/attempt", handler.GetTestAttempts(conn))

			r.Post("/{result_id}/attempt", handler.SaveQuestionAttempt(conn))
			r.Post("/{result_id}/submit", handler.SubmitTest(conn))

		})

		r.Get("/questions", handler.ListAllQuestions(conn))
	})
}
