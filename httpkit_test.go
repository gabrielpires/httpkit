package httpkit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateDefaultRoute(t *testing.T) {

	server := Server{
		ServerMux: http.NewServeMux(),
	}

	expectedStatusCode := 200

	server.AddHandler("/", &GenericHandler{})

	statusCode, err := mockGetRequest(&server)

	if statusCode != expectedStatusCode {
		t.Errorf("status code is incorrect. expected %v got %v", expectedStatusCode, statusCode)
	}

	if err != nil {
		t.Errorf("unexpected error while requesting. got %v", err)
	}

}

func TestSetPort_Default(t *testing.T) {
	server := Server{
		ServerMux: http.NewServeMux(),
	}

	server.SetPort("")
	expected := ":8443"

	if server.Port != expected {
		t.Errorf("default port is incorrect expected %s got %s", expected, server.Port)
	}
}

func TestSetPort_Setted(t *testing.T) {
	server := Server{}
	expected := ":3333"
	server.SetPort(expected)

	if server.Port != expected {
		t.Errorf("default port is incorrect expected %s got %s", expected, server.Port)
	}
}

func TestSetCertificate_Default(t *testing.T) {
	server := Server{}

	server.SetCertificate(&Certificate{
		Certificate: "",
		Key:         "testing.pem",
	})

	if server.Certificate != nil {
		t.Errorf("default certificate is incorrect. expected %v got %v", nil, server.Certificate)
	}

	server.SetCertificate(&Certificate{
		Certificate: "testing.pem",
		Key:         "",
	})

	if server.Certificate != nil {
		t.Errorf("default certificate is incorrect. expected %v got %v", nil, server.Certificate)
	}
}

func TestSetCertificate_Setted(t *testing.T) {
	server := Server{}

	certificate := &Certificate{
		Certificate: "testing.pem",
		Key:         "testing.pem",
	}

	server.SetCertificate(certificate)

	if server.Certificate != certificate {
		t.Errorf("setted certificate is incorrect. expected %v got %v", certificate, server.Certificate)
	}
}

func TestSetCertificate_Nil(t *testing.T) {
	server := Server{}

	server.SetCertificate(nil)

	if server.Certificate != nil {
		t.Errorf("setted certificate is incorrect. expected %v got %v", nil, server.Certificate)
	}
}

func TestStartAndRouting(t *testing.T) {

	server := NewServer()
	server.AddHandler("/", &GenericHandler{})

}

func mockGetRequest(server *Server) (int, error) {

	runner := httptest.NewServer(server.ServerMux)
	defer runner.Close()

	resp, err := http.Get(runner.URL)

	if err != nil {
		return 500, err
	}

	defer resp.Body.Close()
	return resp.StatusCode, err
}
