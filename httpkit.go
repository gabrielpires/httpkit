package httpkit

import (
	"log/slog"
	"net/http"
	"os"
)

type GenericHandler struct{}

func (gh *GenericHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("server with not route configured."))
}

type Certificate struct {
	Certificate string
	Key         string
}

type Options struct {
	Port        string
	Certificate *Certificate
}

type Server struct {
	handlers    []http.Handler
	ServerMux   *http.ServeMux
	Port        string
	Certificate *Certificate
}

func (s *Server) SetPort(port string) string {

	if len(port) == 0 {
		port = ":8443"
	}

	s.Port = port

	return s.Port
}

func (s *Server) SetCertificate(cert *Certificate) *Certificate {

	//check if cert is present in the config
	if cert == nil || (len(cert.Certificate) == 0 || len(cert.Key) == 0) {
		s.Certificate = nil
		return nil
	}

	//check if file provided via config exists
	if !fileExists(cert.Key) || !fileExists(cert.Certificate) {

		s.Certificate = nil
		return nil
	}

	s.Certificate = cert
	return cert
}

func (s *Server) AddHandler(path string, handler http.Handler) {
	s.handlers = append(s.handlers, handler)
	s.ServerMux.Handle(path, handler)
}

func (s *Server) Start(opts *Options) {

	if opts == nil {
		opts = &Options{}
	}

	s.SetPort(opts.Port)
	s.SetCertificate(opts.Certificate)

	if len(s.handlers) == 0 {
		s.AddHandler("/", &GenericHandler{})
	}

	if s.Certificate == nil {
		slog.Info("http server started", "port", s.Port, "certificate", s.Certificate)
		err := http.ListenAndServe(s.Port, s.ServerMux)
		if err != nil {
			slog.Error("error while starting http server", "error", err)
			panic(err)
		}

		return
	}

	slog.Info("http server started", "port", s.Port, "certificate", s.Certificate)
	err := http.ListenAndServeTLS(
		s.Port,
		s.Certificate.Certificate,
		s.Certificate.Key,
		s.ServerMux,
	)

	if err != nil {
		slog.Error("error while starting https server", "error", err)
		panic(err)
	}

}

func NewServer() *Server {
	smux := http.NewServeMux()

	server := Server{
		ServerMux: smux,
	}

	return &server
}

func fileExists(filepath string) bool {

	_, err := os.Stat(filepath)

	if err == nil {
		return true
	}

	if os.IsNotExist(err) {
		slog.Debug("file does not exists", "filepath", filepath)
		return false
	}

	slog.Error("unable to load is file exists", "error", err)

	return false

}
