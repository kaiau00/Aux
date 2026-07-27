package dashboard

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aux-ai/aux-cli/internal/logging"
	"github.com/aux-ai/aux-cli/internal/viewmodel"
)

//go:embed all:assets
var assets embed.FS

type Server struct {
	options  Options
	services Services
	redactor redactor
	token    string
	url      string

	httpServer *http.Server
	listener   net.Listener

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	clientsMu sync.Mutex
	clients   map[chan DashboardEvent]struct{}
}

func Start(parent context.Context, services Services, options Options) (*Server, error) {
	if !options.Enabled {
		return nil, nil
	}
	if options.Host == "" {
		options.Host = "127.0.0.1"
	}
	if options.Redaction == "" {
		options.Redaction = RedactionRedacted
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", options.Host, options.Port))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	server := &Server{
		options:  options,
		services: services,
		redactor: newRedactor(options),
		token:    token,
		listener: listener,
		ctx:      ctx,
		cancel:   cancel,
		clients:  make(map[chan DashboardEvent]struct{}),
	}
	server.url = fmt.Sprintf("http://%s/?token=%s", listener.Addr().String(), token)
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("GET /tasks", server.handleTasksPage)
	mux.HandleFunc("GET /css/", server.handleStaticAsset)
	mux.HandleFunc("GET /js/", server.handleStaticAsset)
	mux.HandleFunc("/api/snapshot", server.handleSnapshot)
	mux.HandleFunc("/api/sessions/", server.handleSessionMessages)
	mux.HandleFunc("GET /api/v1/tasks", server.handleTasksList)
	mux.HandleFunc("GET /api/v1/tasks/{id}", server.handleTaskView)
	mux.HandleFunc("/events", server.handleEvents)
	server.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	server.startSubscribers()
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		err := server.httpServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Warn("dashboard server stopped unexpectedly", "error", err)
		}
	}()
	logging.InfoPersist("Aux dashboard available", "url", server.url)
	return server, nil
}

func (s *Server) URL() string {
	if s == nil {
		return ""
	}
	return s.url
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.cancel()
	s.clientsMu.Lock()
	for client := range s.clients {
		delete(s.clients, client)
		close(client)
	}
	s.clientsMu.Unlock()
	err := s.httpServer.Shutdown(ctx)
	s.wg.Wait()
	return err
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "dashboard token required", http.StatusUnauthorized)
		return
	}
	data, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "dashboard asset missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// handleTasksPage serves the active-task workspace (roadmapplan.md §13.12), an
// active-work-first view backed entirely by the read-only /api/v1 projections.
// Token-gated like the rest of the dashboard.
func (s *Server) handleTasksPage(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "dashboard token required", http.StatusUnauthorized)
		return
	}
	data, err := assets.ReadFile("assets/tasks.html")
	if err != nil {
		http.Error(w, "dashboard asset missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// handleStaticAsset serves the split css/js sub-resources (roadmapplan.md
// §13.13) from the embedded asset tree. These carry no sensitive data — the
// token protects the /api data endpoints — so they are not token-gated, since a
// browser cannot attach the token to <link>/<script> sub-resource requests.
func (s *Server) handleStaticAsset(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	// Only css/ and js/ trees are served, and Clean removes any traversal.
	if !strings.HasPrefix(clean, "css/") && !strings.HasPrefix(clean, "js/") {
		http.NotFound(w, r)
		return
	}
	data, err := assets.ReadFile("assets/" + clean)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(clean, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(clean, ".js"):
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	}
	_, _ = w.Write(data)
}

// handleTasksList serves recent task summaries for the dashboard's active-work
// navigation (roadmapplan.md §13.12, §18). Read-only and token-gated.
func (s *Server) handleTasksList(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "dashboard token required", http.StatusUnauthorized)
		return
	}
	if s.services.Tasks == nil {
		http.Error(w, "task read models unavailable", http.StatusNotFound)
		return
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	tasks, err := s.services.Tasks.RecentTasks(r.Context(), limit)
	if err != nil {
		http.Error(w, "failed to list tasks", http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []viewmodel.TaskSummaryVM{}
	}
	writeJSON(w, tasks)
}

// handleTaskView serves a task's assembled, read-only view model (roadmapplan.md
// §5.6, §18). It is read-only and token-gated like the rest of the dashboard.
func (s *Server) handleTaskView(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "dashboard token required", http.StatusUnauthorized)
		return
	}
	if s.services.Tasks == nil {
		http.Error(w, "task read models unavailable", http.StatusNotFound)
		return
	}
	id := r.PathValue("id")
	view, err := s.services.Tasks.TaskView(r.Context(), id)
	if err != nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "dashboard token required", http.StatusUnauthorized)
		return
	}
	snapshot, err := s.snapshot(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, snapshot)
}

func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "dashboard token required", http.StatusUnauthorized)
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	sessionID = strings.TrimSuffix(sessionID, "/messages")
	if sessionID == "" || !strings.HasSuffix(r.URL.Path, "/messages") {
		http.NotFound(w, r)
		return
	}
	messages, err := s.services.Messages.List(r.Context(), sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dtos := make([]MessageDTO, 0, len(messages))
	for _, msg := range messages {
		dtos = append(dtos, s.redactor.message(msg))
	}
	writeJSON(w, dtos)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "dashboard token required", http.StatusUnauthorized)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	client := make(chan DashboardEvent, 128)
	s.clientsMu.Lock()
	s.clients[client] = struct{}{}
	s.clientsMu.Unlock()
	defer func() {
		s.clientsMu.Lock()
		if _, ok := s.clients[client]; ok {
			delete(s.clients, client)
			close(client)
		}
		s.clientsMu.Unlock()
	}()

	for {
		select {
		case event, ok := <-client:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Server) authorized(r *http.Request) bool {
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Aux-Dashboard-Token")
	}
	return token != "" && token == s.token
}

func (s *Server) snapshot(ctx context.Context) (Snapshot, error) {
	sessions, err := s.services.Sessions.List(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	dto := make([]SessionDTO, 0, len(sessions))
	stats := StatsDTO{SessionCount: len(sessions)}
	for _, sess := range sessions {
		dto = append(dto, sessionDTO(sess))
		stats.MessageCount += sess.MessageCount
		stats.PromptTokens += sess.PromptTokens
		stats.CompletionTokens += sess.CompletionTokens
		stats.Cost += sess.Cost
	}
	logs := logging.List()
	logDTOs := make([]LogDTO, 0, len(logs))
	for _, log := range logs {
		logDTOs = append(logDTOs, logDTO(log))
	}
	return Snapshot{
		Sessions: dto,
		Logs:     logDTOs,
		Stats:    stats,
		Mode:     string(s.redactor.mode),
	}, nil
}

func (s *Server) startSubscribers() {
	s.wg.Add(5)
	go s.pipeSessions()
	go s.pipeMessages()
	go s.pipeHistory()
	go s.pipeAgent()
	go s.pipeLogs()
}

func (s *Server) pipeSessions() {
	defer s.wg.Done()
	ch := s.services.Sessions.Subscribe(s.ctx)
	for event := range ch {
		s.broadcast(DashboardEvent{Type: eventType("session", event), Data: sessionDTO(event.Payload), Time: nowUnix()})
	}
}

func (s *Server) pipeMessages() {
	defer s.wg.Done()
	ch := s.services.Messages.Subscribe(s.ctx)
	for event := range ch {
		s.broadcast(DashboardEvent{Type: eventType("message", event), Data: s.redactor.message(event.Payload), Time: nowUnix()})
	}
}

func (s *Server) pipeHistory() {
	defer s.wg.Done()
	ch := s.services.History.Subscribe(s.ctx)
	for event := range ch {
		s.broadcast(DashboardEvent{Type: eventType("history", event), Data: s.redactor.file(event.Payload), Time: nowUnix()})
	}
}

func (s *Server) pipeAgent() {
	defer s.wg.Done()
	ch := s.services.Agent.Subscribe(s.ctx)
	for event := range ch {
		dto := AgentDTO{
			Type:      string(event.Payload.Type),
			SessionID: event.Payload.SessionID,
			Progress:  event.Payload.Progress,
			Done:      event.Payload.Done,
		}
		if event.Payload.Error != nil {
			dto.Error = event.Payload.Error.Error()
		}
		s.broadcast(DashboardEvent{Type: eventType("agent", event), Data: dto, Time: nowUnix()})
	}
}

func (s *Server) pipeLogs() {
	defer s.wg.Done()
	ch := logging.Subscribe(s.ctx)
	for event := range ch {
		s.broadcast(DashboardEvent{Type: eventType("log", event), Data: logDTO(event.Payload), Time: nowUnix()})
	}
}

func (s *Server) broadcast(event DashboardEvent) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for client := range s.clients {
		select {
		case client <- event:
		default:
			select {
			case <-client:
			default:
			}
			select {
			case client <- DashboardEvent{Type: "dashboard.resync", Data: "event buffer overflow", Time: nowUnix()}:
			default:
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	_ = encoder.Encode(value)
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
