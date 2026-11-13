package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"sojourn/game"
)

func stubBinaryCalls(t *testing.T, running, golden []byte, pokes []string, pokeErr error, writes int, metered bool) {
	t.Helper()
	prev := [4]any{runningImage, goldenImage, patchPokes, remainingWrites}
	runningImage = func(*game.State) ([]byte, error) { return running, nil }
	goldenImage = func(*game.State) ([]byte, error) { return golden, nil }
	patchPokes = func(*game.State, []byte) ([]string, error) { return pokes, pokeErr }
	remainingWrites = func(*game.State) (int, bool) { return writes, metered }
	t.Cleanup(func() {
		runningImage = prev[0].(func(*game.State) ([]byte, error))
		goldenImage = prev[1].(func(*game.State) ([]byte, error))
		patchPokes = prev[2].(func(*game.State, []byte) ([]string, error))
		remainingWrites = prev[3].(func(*game.State) (int, bool))
	})
}

func TestDownloadRunningAndGoldenImages(t *testing.T) {
	stubBinaryCalls(t, []byte{1, 2, 3}, []byte{9, 9}, nil, nil, 0, false)
	s := newTestServer(t)

	for _, tc := range []struct {
		handler  http.HandlerFunc
		body     []byte
		filename string
	}{
		{s.downloadBinary, []byte{1, 2, 3}, `attachment; filename="probe.bin"`},
		{s.downloadGoldenBinary, []byte{9, 9}, `attachment; filename="probe_golden.bin"`},
	} {
		rec := httptest.NewRecorder()
		tc.handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), tc.body) ||
			rec.Header().Get("Content-Disposition") != tc.filename {
			t.Errorf("status %d, body % x, disposition %q; want 200, % x, %q",
				rec.Code, rec.Body.Bytes(), rec.Header().Get("Content-Disposition"), tc.body, tc.filename)
		}
	}
}

func TestDownloadErrorStatuses(t *testing.T) {
	s := newTestServer(t)
	for _, tc := range []struct {
		err  error
		want int
	}{
		{game.ErrNoGameInProgress, http.StatusBadRequest},
		{game.ErrProbeMemoryUnavailable, http.StatusServiceUnavailable},
		{errors.New("broken ELF"), http.StatusInternalServerError},
	} {
		prev := runningImage
		runningImage = func(*game.State) ([]byte, error) { return nil, tc.err }
		rec := httptest.NewRecorder()
		s.downloadBinary(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		runningImage = prev

		if rec.Code != tc.want {
			t.Errorf("%v: status = %d, want %d", tc.err, rec.Code, tc.want)
		}
	}

	// Without stubs: no game in progress.
	rec := httptest.NewRecorder()
	s.downloadGoldenBinary(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("golden without a game: status = %d, want 400", rec.Code)
	}
}

// openUplink gives the server a game uplink to queue onto.
func openUplink(s *server) {
	s.uplinkChan = make(chan []byte, 16)
	s.uplinkOpen = true
}

func drain(ch chan []byte) []string {
	var lines []string
	for {
		select {
		case line := <-ch:
			lines = append(lines, string(line))
		default:
			return lines
		}
	}
}

func upload(t *testing.T, s *server, url string, body []byte, multipartFile bool) (int, uploadPatchResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if multipartFile {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		part, _ := mw.CreateFormFile("file", "probe.bin")
		part.Write(body)
		mw.Close()
		req = httptest.NewRequest(http.MethodPost, url, &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
	}
	rec := httptest.NewRecorder()
	s.uploadPatch(rec, req)

	var resp uploadPatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return rec.Code, resp
}

func TestUploadPatchQueuesPokes(t *testing.T) {
	pokes := []string{"POKE 0x20001040 AB", "POKE 0x20001200 46C0"}
	var received []byte
	stubBinaryCalls(t, nil, nil, pokes, nil, 10, true)
	patchPokes = func(_ *game.State, patched []byte) ([]string, error) {
		received = patched
		return pokes, nil
	}

	for _, multipartFile := range []bool{false, true} {
		s := newTestServer(t)
		openUplink(s)

		code, resp := upload(t, s, "/api/upload-patch", []byte{1, 2, 3}, multipartFile)
		if code != http.StatusOK || resp.Writes != 2 || !slices.Equal(resp.Pokes, pokes) {
			t.Errorf("multipart=%v: %d %+v", multipartFile, code, resp)
		}
		if !bytes.Equal(received, []byte{1, 2, 3}) {
			t.Errorf("multipart=%v: patch received as % x", multipartFile, received)
		}
		if got := drain(s.uplinkChan); !slices.Equal(got, pokes) {
			t.Errorf("multipart=%v: uplink got %q, want %q", multipartFile, got, pokes)
		}
	}
}

func TestUploadPatchDryRunSendsNothing(t *testing.T) {
	stubBinaryCalls(t, nil, nil, []string{"POKE 0x20001040 AB"}, nil, 0, false)
	s := newTestServer(t)
	openUplink(s)

	code, resp := upload(t, s, "/api/upload-patch?dry-run=1", []byte{1}, false)
	if code != http.StatusOK || !resp.DryRun || resp.Writes != 1 {
		t.Errorf("%d %+v", code, resp)
	}
	if got := drain(s.uplinkChan); len(got) != 0 {
		t.Errorf("dry run sent %q", got)
	}
}

func TestUploadPatchIsAllOrNothing(t *testing.T) {
	stubBinaryCalls(t, nil, nil, []string{"POKE 0x20001040 AB", "POKE 0x20001080 CD", "POKE 0x200010C0 EF"}, nil, 2, true)
	s := newTestServer(t)
	openUplink(s)

	code, resp := upload(t, s, "/api/upload-patch", []byte{1}, false)
	if code != http.StatusConflict || resp.Writes != 3 || resp.Message == "" {
		t.Errorf("%d %+v, want 409 explaining 3 writes needed", code, resp)
	}
	if got := drain(s.uplinkChan); len(got) != 0 {
		t.Errorf("sent %q from a patch that doesn't fit the allowance", got)
	}
}

func TestUploadPatchErrors(t *testing.T) {
	s := newTestServer(t)

	rec := httptest.NewRecorder()
	s.uploadPatch(rec, httptest.NewRequest(http.MethodGet, "/api/upload-patch", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: status = %d, want 405", rec.Code)
	}

	for _, tc := range []struct {
		err  error
		want int
	}{
		{game.ErrNoGameInProgress, http.StatusBadRequest},
		{game.ErrPatchTooLarge, http.StatusBadRequest},
		{game.ErrProbeMemoryUnavailable, http.StatusServiceUnavailable},
	} {
		stubBinaryCalls(t, nil, nil, nil, tc.err, 0, false)
		if code, _ := upload(t, s, "/api/upload-patch", []byte{1}, false); code != tc.want {
			t.Errorf("%v: status = %d, want %d", tc.err, code, tc.want)
		}
	}

	// The websocket has gone and the game's uplink is closed.
	stubBinaryCalls(t, nil, nil, []string{"POKE 0x20001040 AB"}, nil, 0, false)
	s.uplinkOpen = false
	if code, _ := upload(t, s, "/api/upload-patch", []byte{1}, false); code != http.StatusBadRequest {
		t.Errorf("closed uplink: status = %d, want 400", code)
	}
}
