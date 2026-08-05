package migrations_test

import (
	"strings"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

func TestAudioMigrationDoesNotPersistRawMedia(t *testing.T) {
	up, err := migrations.FS.ReadFile("000015_agents_whatsapp_audio_messages.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	upSQL := strings.ToLower(string(up))

	forbidden := []string{
		"bytea",
		"audio_bytes",
		"audio_data",
		"audio_blob",
		"raw_audio",
		"audio_base64",
		"media_url",
		"media_bytes",
	}
	for _, term := range forbidden {
		if strings.Contains(upSQL, term) {
			t.Fatalf("migration 000015 must not persist raw audio: found forbidden term %q", term)
		}
	}

	if !strings.Contains(upSQL, "media_sha256") {
		t.Fatalf("migration 000015 must persist media_sha256 for audit without raw audio")
	}
}
