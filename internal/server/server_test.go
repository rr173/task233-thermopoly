package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task233-thermopoly/internal/model"
)

func TestHTTPCreateTrialAndProgramRoundTrip(t *testing.T) {
	srv, err := New(Config{Addr: "127.0.0.1:0", DB: t.TempDir() + "/thermopoly.db"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() {
		if err := srv.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	}()

	create := httptest.NewRequest(http.MethodPost, "/api/trials", strings.NewReader(`{"name":"round-trip","material":"FormA","unit":"C"}`))
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	srv.Handler().ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", created.Code, created.Body.String())
	}
	var trial model.Trial
	if err := json.NewDecoder(created.Body).Decode(&trial); err != nil {
		t.Fatalf("decode trial: %v", err)
	}
	if trial.ID == "" || trial.Status != model.TrialReceiving {
		t.Fatalf("created trial = %+v", trial)
	}

	program := httptest.NewRequest(http.MethodPut, "/api/trials/"+trial.ID+"/program", strings.NewReader(`{"name":"standard","start_temp":30,"end_temp":200,"rate_k_per_min":10}`))
	program.Header.Set("Content-Type", "application/json")
	programResponse := httptest.NewRecorder()
	srv.Handler().ServeHTTP(programResponse, program)
	if programResponse.Code != http.StatusCreated {
		t.Fatalf("program status = %d, body=%s", programResponse.Code, programResponse.Body.String())
	}

	getProgram := httptest.NewRequest(http.MethodGet, "/api/trials/"+trial.ID+"/program", nil)
	programRead := httptest.NewRecorder()
	srv.Handler().ServeHTTP(programRead, getProgram)
	if programRead.Code != http.StatusOK {
		t.Fatalf("get program status = %d, body=%s", programRead.Code, programRead.Body.String())
	}
	var got model.Program
	if err := json.NewDecoder(programRead.Body).Decode(&got); err != nil {
		t.Fatalf("decode program: %v", err)
	}
	if got.TrialID != trial.ID || got.Version != 1 || got.RateKPerMin != 10 {
		t.Fatalf("round-trip program = %+v", got)
	}
}
