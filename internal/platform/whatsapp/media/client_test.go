package media_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	devkitfake "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/media"
)

type ClientSuite struct {
	suite.Suite
	ctx context.Context
	cfg media.Config
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupTest() {
	s.ctx = context.Background()
	s.cfg = media.Config{
		AccessToken: "test-access-token",
		HTTPTimeout: 2 * time.Second,
	}
}

func (s *ClientSuite) newClientWithServer(baseURL string) *media.Client {
	cfg := s.cfg
	cfg.BaseURL = baseURL
	c, err := media.NewClient(devkitfake.NewProvider(), cfg)
	s.Require().NoError(err)
	return c
}

func (s *ClientSuite) TestNewClient_RequiresObservabilityAndToken() {
	_, err := media.NewClient(nil, s.cfg)
	s.Error(err)

	_, err = media.NewClient(devkitfake.NewProvider(), media.Config{})
	s.Error(err)
}

func (s *ClientSuite) TestResolve_Scenarios() {
	scenarios := []struct {
		name          string
		mediaID       string
		serverHandler http.HandlerFunc
		expectErr     error
		expectURL     string
		expectMime    string
		expectSHA     string
		expectSize    int64
		noServerCall  bool
	}{
		{
			name:    "resolve com sucesso",
			mediaID: "1081986107489646",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				s.Equal(http.MethodGet, r.Method)
				s.Equal("/1081986107489646", r.URL.Path)
				s.Equal("Bearer test-access-token", r.Header.Get("Authorization"))

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"url":       "https://media.example.com/audio.ogg?token=abc",
					"mime_type": "audio/ogg",
					"sha256":    "a3a064090479e03e4e84a372ecc07c500312530b0a051a57186725a5a69d165c",
					"file_size": 11044,
					"id":        "1081986107489646",
				})
			},
			expectURL:  "https://media.example.com/audio.ogg?token=abc",
			expectMime: "audio/ogg",
			expectSHA:  "a3a064090479e03e4e84a372ecc07c500312530b0a051a57186725a5a69d165c",
			expectSize: 11044,
		},
		{
			name:         "media id vazio nao chama servidor",
			mediaID:      "  ",
			noServerCall: true,
			expectErr:    media.ErrMediaIDEmpty,
		},
		{
			name:    "resposta sem url",
			mediaID: "media-sem-url",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"mime_type": "audio/ogg",
				})
			},
			expectErr: media.ErrMediaURLMissing,
		},
		{
			name:    "autenticacao invalida",
			mediaID: "media-401",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"message": "invalid token", "code": 190},
				})
			},
			expectErr: media.ErrMediaAuth,
		},
		{
			name:    "erro de servidor meta",
			mediaID: "media-500",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"message": "internal error", "code": 1},
				})
			},
			expectErr: media.ErrMediaServer,
		},
		{
			name:    "requisicao invalida",
			mediaID: "media-400",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"message": "bad request", "code": 100},
				})
			},
			expectErr: media.ErrMediaBadRequest,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var server *httptest.Server
			if !scenario.noServerCall {
				server = httptest.NewServer(scenario.serverHandler)
				defer server.Close()
			}

			baseURL := "https://unused.invalid"
			if server != nil {
				baseURL = server.URL
			}

			client := s.newClientWithServer(baseURL)
			descriptor, err := client.Resolve(s.ctx, scenario.mediaID)

			if scenario.expectErr != nil {
				s.Error(err)
				s.True(errors.Is(err, scenario.expectErr), "esperado erro %v, obtido %v", scenario.expectErr, err)
				return
			}

			s.NoError(err)
			s.Equal(scenario.expectURL, descriptor.URL)
			s.Equal(scenario.expectMime, descriptor.MimeType)
			s.Equal(scenario.expectSHA, descriptor.SHA256)
			s.Equal(scenario.expectSize, descriptor.Size)
		})
	}
}

func (s *ClientSuite) TestResolve_Timeout() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := s.cfg
	cfg.BaseURL = server.URL
	cfg.HTTPTimeout = 20 * time.Millisecond
	client, err := media.NewClient(devkitfake.NewProvider(), cfg)
	s.Require().NoError(err)

	_, err = client.Resolve(s.ctx, "media-timeout")
	s.Error(err)
}

func (s *ClientSuite) TestDownload_Scenarios() {
	body := strings.Repeat("a", 100)
	sum := sha256.Sum256([]byte(body))
	expectedSHA := hex.EncodeToString(sum[:])

	scenarios := []struct {
		name          string
		url           string
		maxBytes      int64
		serverHandler http.HandlerFunc
		expectErr     error
		expectSHA     string
		expectSize    int64
	}{
		{
			name:     "download com sucesso dentro do limite",
			maxBytes: 1000,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				s.Equal("Bearer test-access-token", r.Header.Get("Authorization"))
				_, _ = w.Write([]byte(body))
			},
			expectSHA:  expectedSHA,
			expectSize: int64(len(body)),
		},
		{
			name:     "download acima do limite de bytes",
			maxBytes: int64(len(body) - 1),
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			},
			expectErr: media.ErrDownloadTooLarge,
		},
		{
			name:     "rejeita por content-length antes de bufferizar o corpo",
			maxBytes: 10,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "5000")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(make([]byte, 5000))
			},
			expectErr: media.ErrDownloadTooLarge,
		},
		{
			name:     "autenticacao invalida no download",
			maxBytes: 1000,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("forbidden"))
			},
			expectErr: media.ErrMediaAuth,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			server := httptest.NewServer(scenario.serverHandler)
			defer server.Close()

			client := s.newClientWithServer(server.URL)
			downloaded, err := client.Download(s.ctx, server.URL+"/audio", scenario.maxBytes)

			if scenario.expectErr != nil {
				s.Error(err)
				s.True(errors.Is(err, scenario.expectErr), "esperado erro %v, obtido %v", scenario.expectErr, err)
				return
			}

			s.NoError(err)
			s.Equal(scenario.expectSHA, downloaded.SHA256)
			s.Equal(scenario.expectSize, downloaded.Size)
		})
	}
}

func (s *ClientSuite) TestDownload_URLVazia() {
	client := s.newClientWithServer("https://unused.invalid")
	_, err := client.Download(s.ctx, "  ", 1000)
	s.Error(err)
	s.True(errors.Is(err, media.ErrDownloadURLEmpty))
}

func (s *ClientSuite) TestDownload_MaxBytesInvalido() {
	client := s.newClientWithServer("https://unused.invalid")
	_, err := client.Download(s.ctx, "https://unused.invalid/audio", 0)
	s.Error(err)
}

func (s *ClientSuite) TestDownload_Timeout() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("data"))
	}))
	defer server.Close()

	cfg := s.cfg
	cfg.BaseURL = server.URL
	cfg.HTTPTimeout = 20 * time.Millisecond
	client, err := media.NewClient(devkitfake.NewProvider(), cfg)
	s.Require().NoError(err)

	_, err = client.Download(s.ctx, server.URL+"/audio", 1000)
	s.Error(err)
}

func (s *ClientSuite) TestVerifyChecksum() {
	descriptor := media.MediaDescriptor{SHA256: "ABCDEF"}
	matching := media.DownloadedMedia{SHA256: "abcdef"}
	mismatching := media.DownloadedMedia{SHA256: "000000"}

	s.NoError(media.VerifyChecksum(descriptor, matching))

	err := media.VerifyChecksum(descriptor, mismatching)
	s.Error(err)
	s.True(errors.Is(err, media.ErrSHA256Mismatch))

	err = media.VerifyChecksum(media.MediaDescriptor{}, matching)
	s.Error(err)
	s.True(errors.Is(err, media.ErrSHA256Mismatch))
}
