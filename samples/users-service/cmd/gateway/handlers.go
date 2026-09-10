package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	argos "github.com/jhonsferg/argos"
	argosgorillamux "github.com/jhonsferg/argos/integrations/httpserver/gorillamux"
	"github.com/jhonsferg/argos/samples/users-service/userspb"
)

// API translates HTTP requests into calls against userspb.UsersClient - the
// exact generated gRPC client interface, so no separate interface needs
// redefining. Tests use a fake implementing the same interface.
type API struct {
	client userspb.UsersClient
	logger argos.Logger
}

func NewAPI(client userspb.UsersClient, logger argos.Logger) *API {
	return &API{client: client, logger: logger}
}

func (a *API) Routes() *mux.Router {
	r := mux.NewRouter()
	r.Use(argosgorillamux.Middleware())
	r.HandleFunc("/healthz", a.handleHealth).Methods(http.MethodGet)
	r.HandleFunc("/users/{id}", a.handleGetUser).Methods(http.MethodGet)
	r.HandleFunc("/users", a.handleCreateUser).Methods(http.MethodPost)
	return r
}

func (a *API) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type createUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (a *API) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	u, err := a.client.CreateUser(r.Context(), &userspb.CreateUserRequest{Name: req.Name, Email: req.Email})
	if err != nil {
		a.writeGRPCError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(u)
}

func (a *API) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	u, err := a.client.GetUser(r.Context(), &userspb.GetUserRequest{Id: id})
	if err != nil {
		a.writeGRPCError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(u)
}

// writeGRPCError translates a gRPC status error into the matching HTTP
// status - this is the gateway's actual job, beyond just forwarding calls.
func (a *API) writeGRPCError(w http.ResponseWriter, r *http.Request, err error) {
	st, _ := status.FromError(err)
	switch st.Code() {
	case codes.NotFound:
		http.Error(w, "user not found", http.StatusNotFound)
	case codes.InvalidArgument:
		http.Error(w, st.Message(), http.StatusBadRequest)
	default:
		a.logger.Error(r.Context(), "grpc call failed", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
