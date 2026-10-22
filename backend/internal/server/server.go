package server

import (
	"net/http"

	"github.com/farhan-hamzah/Akesa/backend/internal/access"
	"github.com/farhan-hamzah/Akesa/backend/internal/audit"
	"github.com/farhan-hamzah/Akesa/backend/internal/auth"
	"github.com/farhan-hamzah/Akesa/backend/internal/hospital"
	appmiddleware "github.com/farhan-hamzah/Akesa/backend/internal/middleware"
	"github.com/farhan-hamzah/Akesa/backend/internal/notification"
	"github.com/farhan-hamzah/Akesa/backend/internal/patient"
	identity "github.com/farhan-hamzah/Akesa/backend/internal/patient/identify"
	"github.com/farhan-hamzah/Akesa/backend/internal/patientqr"
	"github.com/farhan-hamzah/Akesa/backend/internal/user"
	"github.com/farhan-hamzah/Akesa/backend/internal/ws"
)

const (
	RolePatient = "PATIENT"
	RoleStaff   = "HOSPITAL_STAFF"
	RoleAdmin   = "ADMIN"
)

type Dependencies struct {
	UserLoader          auth.UserLoader
	UserHandler         *user.Handler
	PatientHandler      *patient.Handler
	IdentityHandler     *identity.Handler
	HospitalHandler     *hospital.Handler
	AccessHandler       *access.Handler
	AuditHandler        *audit.Handler
	PatientQRHandler    *patientqr.Handler
	NotificationHandler *notification.Handler
	WSHandler           *ws.Handler
}

type Server struct {
	mux *http.ServeMux
}

func New(deps Dependencies) *Server {
	mux := http.NewServeMux()

	server := &Server{mux: mux}
	server.registerRoutes(deps)

	return server
}

func (s *Server) Handler() http.Handler {
	return appmiddleware.Chain(
		appmiddleware.Recover,
		appmiddleware.Logging,
	)(s.mux)
}

func (s *Server) registerRoutes(deps Dependencies) {
	s.mux.HandleFunc("GET /{$}", rootHandler)
	s.mux.HandleFunc("GET /health", healthHandler)
	s.mux.HandleFunc("GET /api/v1/audit/verify-chain", deps.AuditHandler.VerifyChain)
	s.mux.HandleFunc("POST /api/v1/audit/sync-chain", deps.AuditHandler.SyncChain)

	s.mux.Handle("POST /api/v1/users/sync", auth.RequireAuth(http.HandlerFunc(deps.UserHandler.Sync)))
	s.mux.Handle("GET /api/v1/me", auth.RequireAuth(http.HandlerFunc(deps.UserHandler.Me)))

	// --- Real-time WebSocket connection ---
	if deps.WSHandler != nil {
		s.mux.Handle("GET /api/v1/ws",
			s.wsChain(deps, http.HandlerFunc(deps.WSHandler.ServeWS)),
		)
	}

	s.mux.Handle("GET /api/v1/hospitals",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.ListActive)),
	)
	s.mux.Handle("GET /api/v1/hospitals/invitations/validate",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.ValidateInvitation)),
	)
	s.mux.Handle("POST /api/v1/hospitals/register",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.RegisterWithKey)),
	)
	s.mux.Handle("GET /api/v1/hospitals/my-application",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.GetMyApplication)),
	)
	s.mux.Handle("PUT /api/v1/hospitals/my-application",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.ResubmitApplication)),
	)

	// --- Patient routes ---
	s.mux.Handle("GET /api/v1/patient/profile",
		s.chain(deps, http.HandlerFunc(deps.PatientHandler.GetMyProfile), RolePatient),
	)
	s.mux.Handle("POST /api/v1/patient/profile",
		s.chain(deps, http.HandlerFunc(deps.PatientHandler.CreateProfile), RolePatient),
	)
	s.mux.Handle("PUT /api/v1/patient/profile",
		s.chain(deps, http.HandlerFunc(deps.PatientHandler.UpdateProfile), RolePatient),
	)
	s.mux.Handle("POST /api/v1/patient/identity/verifications",
		s.chain(deps, http.HandlerFunc(deps.IdentityHandler.CreateVerification), RolePatient),
	)
	s.mux.Handle("GET /api/v1/patient/identity/verifications/{id}",
		s.chain(deps, http.HandlerFunc(deps.IdentityHandler.GetVerification), RolePatient),
	)
	s.mux.Handle("GET /api/v1/patient/identity/verifications/latest",
		s.chain(deps, http.HandlerFunc(deps.IdentityHandler.GetLatestVerification), RolePatient),
	)
	s.mux.Handle("POST /api/v1/patient/identity/verifications/{id}/document",
		s.chain(deps, http.HandlerFunc(deps.IdentityHandler.UploadDocument), RolePatient),
	)
	s.mux.Handle("POST /api/v1/patient/identity/verifications/{id}/selfie",
		s.chain(deps, http.HandlerFunc(deps.IdentityHandler.UploadSelfie), RolePatient),
	)
	s.mux.Handle("POST /api/v1/admin/identity-verifications/{id}/approve",
		s.chain(deps, http.HandlerFunc(deps.IdentityHandler.ApproveVerification), RoleAdmin),
	)
	s.mux.Handle("POST /api/v1/admin/identity-verifications/{id}/reject",
		s.chain(deps, http.HandlerFunc(deps.IdentityHandler.RejectVerification), RoleAdmin),
	)
	s.mux.Handle("GET /api/v1/patient/access-requests",
		s.chain(deps, http.HandlerFunc(deps.AccessHandler.ListMine), RolePatient),
	)
	s.mux.Handle("POST /api/v1/patient/access-requests/{id}/approve",
		s.chain(deps, http.HandlerFunc(deps.AccessHandler.Approve), RolePatient),
	)
	s.mux.Handle("POST /api/v1/patient/access-requests/{id}/reject",
		s.chain(deps, http.HandlerFunc(deps.AccessHandler.Reject), RolePatient),
	)
	s.mux.Handle("POST /api/v1/patient/access-requests/{id}/revoke",
		s.chain(deps, http.HandlerFunc(deps.AccessHandler.Revoke), RolePatient),
	)
	s.mux.Handle("GET /api/v1/patient/history",
		s.chain(deps, http.HandlerFunc(deps.AuditHandler.MyProfileHistory), RolePatient),
	)

	s.mux.Handle("POST /api/v1/hospital/access-requests",
		s.chain(deps, http.HandlerFunc(deps.AccessHandler.Create), RoleStaff),
	)
	s.mux.Handle("GET /api/v1/hospital/access-requests",
		s.chain(deps, http.HandlerFunc(deps.AccessHandler.ListForMyHospital), RoleStaff),
	)
	s.mux.Handle("GET /api/v1/hospital/access-requests/{id}/data",
		s.chain(deps, http.HandlerFunc(deps.AccessHandler.FetchData), RoleStaff),
	)

	// --- Admin routes ---
	s.mux.Handle("GET /api/v1/admin/hospitals",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.List), RoleAdmin),
	)
	s.mux.Handle("PATCH /api/v1/admin/hospitals/{id}/verify",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.Verify), RoleAdmin),
	)
	s.mux.Handle("PATCH /api/v1/admin/hospitals/{id}/activate",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.Activate), RoleAdmin),
	)
	s.mux.Handle("PATCH /api/v1/admin/hospitals/{id}/deactivate",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.Deactivate), RoleAdmin),
	)
	s.mux.Handle("POST /api/v1/admin/hospitals/{id}/staff",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.AddStaff), RoleAdmin),
	)
	s.mux.Handle("POST /api/v1/admin/hospitals/invitations",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.GenerateInvitation), RoleAdmin),
	)
	s.mux.Handle("GET /api/v1/admin/hospital-applications",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.ListApplications), RoleAdmin),
	)
	s.mux.Handle("GET /api/v1/admin/hospital-applications/{id}",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.GetApplication), RoleAdmin),
	)
	s.mux.Handle("POST /api/v1/admin/hospital-applications/{id}/request-revision",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.RequestRevision), RoleAdmin),
	)
	s.mux.Handle("POST /api/v1/admin/hospital-applications/{id}/reject",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.RejectApplication), RoleAdmin),
	)
	s.mux.Handle("POST /api/v1/admin/hospital-applications/{id}/approve",
		s.chain(deps, http.HandlerFunc(deps.HospitalHandler.ApproveApplication), RoleAdmin),
	)
	s.mux.Handle("GET /api/v1/admin/security-alerts",
		s.chain(deps, http.HandlerFunc(deps.AuditHandler.SecurityAlerts), RoleAdmin),
	)
	s.mux.Handle("GET /api/v1/patient/qr",
		s.chain(deps, http.HandlerFunc(deps.PatientQRHandler.Get), RolePatient),
	)
	s.mux.Handle("POST /api/v1/patient/qr/rotate",
		s.chain(deps, http.HandlerFunc(deps.PatientQRHandler.Rotate), RolePatient),
	)

	// --- Notification routes (Mobile & Web) ---
	s.mux.Handle("POST /api/v1/notifications/device-token",
		s.chain(deps, http.HandlerFunc(deps.NotificationHandler.RegisterDeviceToken)),
	)
	s.mux.Handle("DELETE /api/v1/notifications/device-token",
		s.chain(deps, http.HandlerFunc(deps.NotificationHandler.UnregisterDeviceToken)),
	)
	s.mux.Handle("GET /api/v1/notifications",
		s.chain(deps, http.HandlerFunc(deps.NotificationHandler.List)),
	)
	s.mux.Handle("GET /api/v1/notifications/unread-count",
		s.chain(deps, http.HandlerFunc(deps.NotificationHandler.GetUnreadCount)),
	)
	s.mux.Handle("PATCH /api/v1/notifications/{id}/read",
		s.chain(deps, http.HandlerFunc(deps.NotificationHandler.MarkAsRead)),
	)
	s.mux.Handle("POST /api/v1/notifications/read-all",
		s.chain(deps, http.HandlerFunc(deps.NotificationHandler.MarkAllAsRead)),
	)
}

func (s *Server) wsChain(deps Dependencies, final http.Handler) http.Handler {
	// Middleware untuk mengizinkan token dari query param ?token=...
	tokenFromQuery := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				token := r.URL.Query().Get("token")
				if token != "" {
					r.Header.Set("Authorization", "Bearer "+token)
				}
			}
			next.ServeHTTP(w, r)
		})
	}

	return appmiddleware.Chain(
		tokenFromQuery,
		auth.RequireAuth,
		auth.WithUser(deps.UserLoader),
	)(final)
}

func (s *Server) chain(deps Dependencies, final http.Handler, roles ...string) http.Handler {
	mws := []appmiddleware.Middleware{
		auth.RequireAuth,
		auth.WithUser(deps.UserLoader),
	}
	if len(roles) > 0 {
		mws = append(mws, auth.RequireRole(roles...))
	}
	return appmiddleware.Chain(mws...)(final)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"name":"Akesa API","status":"running","health":"/health"}`))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
