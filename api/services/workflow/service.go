package workflow

import (
	"net/http"

	"workflow-code-test/api/pkg/db"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

type Service struct {
	db db.WorkFlowDB
}

func NewService(pool *pgxpool.Pool) (*Service, error) {
	// Create a standard sql.DB from the pgxpool for SQLBoiler
	sqlDB := stdlib.OpenDBFromPool(pool)

	// Create the repository
	repository := db.NewWorkflowRepository(sqlDB)

	return &Service{
		db: repository,
	}, nil
}

// jsonMiddleware sets the Content-Type header to application/json
func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (s *Service) LoadRoutes(parentRouter *mux.Router) {
	router := parentRouter.PathPrefix("/workflows").Subrouter()
	router.StrictSlash(false)
	router.Use(jsonMiddleware)

	router.HandleFunc("/{id}", s.HandleGetWorkflow).Methods("GET")
	router.HandleFunc("/{id}/execute", s.HandleExecuteWorkflow).Methods("POST")

}
